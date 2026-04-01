import asyncio
import json
import pathlib
import sys
import textwrap
from types import SimpleNamespace

import pytest


# Ensure `open_webui` is importable when running tests from repo root.
_BACKEND_DIR = pathlib.Path(__file__).resolve().parents[3]
if str(_BACKEND_DIR) not in sys.path:
    sys.path.insert(0, str(_BACKEND_DIR))


def test_mcp_streamable_http_client_json_and_sse():
    from aiohttp import web

    from open_webui.utils.mcp import MCPStreamableHttpClient

    seen_session_headers = []

    async def handler(request: web.Request):
        payload = await request.json()
        method = payload.get("method")

        # Record session header usage across requests.
        seen_session_headers.append(request.headers.get("Mcp-Session-Id"))

        if method == "initialize":
            return web.json_response(
                {
                    "jsonrpc": "2.0",
                    "id": payload.get("id"),
                    "result": {
                        "serverInfo": {"name": "TestMCP", "version": "0.0.1"},
                        "capabilities": {"tools": {}},
                    },
                },
                headers={"Mcp-Session-Id": "sess_123"},
            )

        if method == "notifications/initialized":
            # JSON-RPC notification: no response body required.
            return web.Response(status=200, headers={"Mcp-Session-Id": "sess_123"})

        if method == "tools/list":
            cursor = (payload.get("params") or {}).get("cursor")
            if not cursor:
                tools = [{"name": "foo/bar", "description": "t1", "inputSchema": {"type": "object"}}]
                result = {"tools": tools, "nextCursor": "c2"}
            else:
                tools = [{"name": "echo", "description": "t2", "inputSchema": {"type": "object"}}]
                result = {"tools": tools, "nextCursor": None}

            return web.json_response(
                {"jsonrpc": "2.0", "id": payload.get("id"), "result": result},
                headers={"Mcp-Session-Id": "sess_123"},
            )

        if method == "tools/call":
            # Return SSE to exercise the Streamable HTTP parsing branch.
            resp = web.StreamResponse(
                status=200,
                headers={
                    "Content-Type": "text/event-stream",
                    "Mcp-Session-Id": "sess_123",
                },
            )
            await resp.prepare(request)

            msg = {
                "jsonrpc": "2.0",
                "id": payload.get("id"),
                "result": {
                    "content": [{"type": "text", "text": "ok"}],
                    "name": (payload.get("params") or {}).get("name"),
                    "arguments": (payload.get("params") or {}).get("arguments"),
                },
            }

            await resp.write(f"data: {json.dumps(msg)}\n\n".encode("utf-8"))
            await resp.write(b"data: [DONE]\n\n")
            await resp.write_eof()
            return resp

        return web.json_response(
            {"jsonrpc": "2.0", "id": payload.get("id"), "error": {"message": "unknown method"}},
            status=400,
        )

    async def run():
        app = web.Application()
        app.router.add_post("/", handler)
        runner = web.AppRunner(app)
        await runner.setup()
        site = web.TCPSite(runner, "127.0.0.1", 0)
        await site.start()

        # Determine the allocated port.
        port = site._server.sockets[0].getsockname()[1]
        url = f"http://127.0.0.1:{port}/"

        try:
            client = MCPStreamableHttpClient(url)
            init = await client.initialize()
            assert init.get("serverInfo", {}).get("name") == "TestMCP"

            await client.notify_initialized()
            tools = await client.list_tools()
            assert [t["name"] for t in tools] == ["foo/bar", "echo"]

            result = await client.call_tool("echo", {"x": 1})
            assert result.get("name") == "echo"
            assert result.get("arguments") == {"x": 1}
        finally:
            await runner.cleanup()

    asyncio.run(run())

    # First request (initialize) has no session id; subsequent ones should.
    assert seen_session_headers[0] in (None, "")
    assert any(h == "sess_123" for h in seen_session_headers[1:])


def test_get_tools_exposes_mcp_tool_and_routes_call(monkeypatch):
    # Avoid touching the tool DB layer.
    import open_webui.utils.tools as tools_mod

    monkeypatch.setattr(tools_mod.Tools, "get_tool_by_id", lambda _id: None)

    called = {}

    async def fake_execute_mcp_tool(connection, *, name, arguments, session_token=None, **_kwargs):
        called["connection"] = connection
        called["name"] = name
        called["arguments"] = arguments
        called["session_token"] = session_token
        return {"ok": True}

    monkeypatch.setattr(tools_mod, "execute_mcp_tool", fake_execute_mcp_tool)

    request = SimpleNamespace(
        app=SimpleNamespace(
            state=SimpleNamespace(
                config=SimpleNamespace(
                    MCP_SERVER_CONNECTIONS=[{"url": "http://mcp.local", "auth_type": "none"}],
                    TOOL_SERVER_CONNECTIONS=[],
                ),
                MCP_SERVERS=[
                    {
                        "idx": 0,
                        "url": "http://mcp.local",
                        "server_info": {"name": "TestMCP"},
                        "tools": [
                            {
                                "name": "foo/bar",
                                "description": "desc",
                                "inputSchema": {
                                    "type": "object",
                                    "properties": {"a": {"type": "string"}},
                                    "required": ["a"],
                                },
                            }
                        ],
                    }
                ],
                TOOL_SERVERS=[],
            )
        ),
        state=SimpleNamespace(token=SimpleNamespace(credentials="tok_abc")),
    )
    user = SimpleNamespace(id="u1", role="admin")

    tools = tools_mod.get_tools(request, ["mcp:0"], user, extra_params={})
    assert "mcp_0__foo_bar" in tools
    spec = tools["mcp_0__foo_bar"]["spec"]
    assert spec["name"] == "mcp_0__foo_bar"
    assert spec["parameters"]["type"] == "object"

    async def run():
        out = await tools["mcp_0__foo_bar"]["callable"](a="x")
        return out

    out = asyncio.run(run())
    assert out == {"ok": True}
    assert called["name"] == "foo/bar"
    assert called["arguments"] == {"a": "x"}
    assert called["session_token"] == "tok_abc"


def test_tools_route_prefers_custom_mcp_title_and_description(monkeypatch):
    from open_webui.routers import tools as tools_router

    monkeypatch.setattr(tools_router, "get_user_tool_server_connections", lambda _request, _user: [])
    monkeypatch.setattr(
        tools_router,
        "get_user_mcp_server_connections",
        lambda _request, _user: [
            {
                "transport_type": "stdio",
                "command": "uvx mcp-server-fetch",
                "name": "网页内容抓取",
                "description": "把网页正文提取成适合模型阅读的文本",
                "server_info": {"name": "mcp-fetch", "version": "1.2.3"},
                "verified_at": "2026-04-02T12:00:00Z",
                "config": {"enable": True},
            }
        ],
    )
    monkeypatch.setattr(tools_router.Tools, "get_tools_list_by_user_id", lambda *_args, **_kwargs: [])

    async def fake_get_tool_servers_data(*_args, **_kwargs):
        return []

    monkeypatch.setattr(tools_router, "get_tool_servers_data", fake_get_tool_servers_data)

    request = SimpleNamespace(
        state=SimpleNamespace(token=SimpleNamespace(credentials="tok_abc")),
        app=SimpleNamespace(state=SimpleNamespace()),
    )
    user = SimpleNamespace(id="u1", role="admin")

    async def run():
        return await tools_router.get_tools(request, user)

    tools = asyncio.run(run())
    mcp_entry = next(tool for tool in tools if tool.id == "mcp:0")

    assert mcp_entry.name == "网页内容抓取"
    assert mcp_entry.meta.description == "把网页正文提取成适合模型阅读的文本"


def test_get_mcp_server_display_metadata_falls_back_when_custom_values_missing():
    from open_webui.utils.mcp import get_mcp_server_display_metadata

    title, description = get_mcp_server_display_metadata(
        {
            "transport_type": "http",
            "url": "https://mcp.example.com/v1/mcp",
            "server_info": {"name": "mcp-fetch"},
        },
        index=0,
        default_description="MCP (HTTP) - 未验证",
        prefer_hostname_for_http=True,
    )

    assert title == "mcp-fetch"
    assert description == "MCP (HTTP) - 未验证"


def test_http_client_protocol_negotiation_retries_on_http_error():
    from aiohttp import web

    from open_webui.utils.mcp import MCPStreamableHttpClient

    seen_protocol_versions = []

    async def handler(request: web.Request):
        payload = await request.json()
        method = payload.get("method")
        protocol_version = request.headers.get("MCP-Protocol-Version")
        seen_protocol_versions.append((method, protocol_version))

        if method == "initialize":
            requested = (payload.get("params") or {}).get("protocolVersion")
            if requested == "2025-06-18":
                return web.json_response(
                    {
                        "jsonrpc": "2.0",
                        "id": payload.get("id"),
                        "error": {
                            "code": -32000,
                            "message": "Unsupported protocol version (supported versions: 2024-11-05)",
                        },
                    },
                    status=400,
                )

            return web.json_response(
                {
                    "jsonrpc": "2.0",
                    "id": payload.get("id"),
                    "result": {
                        "protocolVersion": "2024-11-05",
                        "serverInfo": {"name": "LegacyMCP"},
                        "capabilities": {"tools": {}},
                    },
                },
                headers={"Mcp-Session-Id": "legacy_sess"},
            )

        if method == "notifications/initialized":
            return web.Response(status=200, headers={"Mcp-Session-Id": "legacy_sess"})

        if method == "tools/list":
            return web.json_response(
                {
                    "jsonrpc": "2.0",
                    "id": payload.get("id"),
                    "result": {
                        "tools": [
                            {
                                "name": "legacy_tool",
                                "description": "legacy",
                                "inputSchema": {"type": "object"},
                            }
                        ]
                    },
                },
                headers={"Mcp-Session-Id": "legacy_sess"},
            )

        return web.json_response({"jsonrpc": "2.0", "id": payload.get("id"), "result": {}})

    async def run():
        app = web.Application()
        app.router.add_post("/", handler)
        runner = web.AppRunner(app)
        await runner.setup()
        site = web.TCPSite(runner, "127.0.0.1", 0)
        await site.start()
        port = site._server.sockets[0].getsockname()[1]
        url = f"http://127.0.0.1:{port}/"

        try:
            client = MCPStreamableHttpClient(url)
            result = await client.initialize()
            assert result.get("serverInfo", {}).get("name") == "LegacyMCP"
            assert client.protocol_version == "2024-11-05"

            await client.notify_initialized()
            tools = await client.list_tools()
            assert tools[0]["name"] == "legacy_tool"
        finally:
            await runner.cleanup()

    asyncio.run(run())

    assert ("initialize", "2025-06-18") in seen_protocol_versions
    assert ("initialize", "2024-11-05") in seen_protocol_versions
    assert ("tools/list", "2024-11-05") in seen_protocol_versions


def _write_stdio_server(tmp_path, script_name: str, body: str) -> str:
    script_path = tmp_path / script_name
    script_path.write_text(textwrap.dedent(body), encoding="utf-8")
    return str(script_path)


def test_mcp_stdio_client_lifecycle_and_call(tmp_path, monkeypatch):
    from open_webui.utils import mcp as mcp_mod

    monkeypatch.setattr(
        mcp_mod,
        "DEFAULT_STDIO_ALLOWED_COMMANDS",
        mcp_mod.DEFAULT_STDIO_ALLOWED_COMMANDS | {pathlib.Path(sys.executable).name.lower()},
    )

    script_path = _write_stdio_server(
        tmp_path,
        "stdio_server.py",
        """
        import json
        import sys

        for raw in sys.stdin:
            msg = json.loads(raw)
            method = msg.get("method")
            if method == "initialize":
                print(json.dumps({
                    "jsonrpc": "2.0",
                    "id": msg["id"],
                    "result": {
                        "protocolVersion": "2025-06-18",
                        "serverInfo": {"name": "stdio-test", "version": "1.0.0"},
                        "capabilities": {"tools": {}},
                    },
                }), flush=True)
            elif method == "notifications/initialized":
                continue
            elif method == "tools/list":
                print(json.dumps({
                    "jsonrpc": "2.0",
                    "id": msg["id"],
                    "result": {
                        "tools": [
                            {
                                "name": "echo",
                                "description": "Echo tool",
                                "inputSchema": {"type": "object"},
                            }
                        ]
                    },
                }), flush=True)
            elif method == "tools/call":
                print(json.dumps({
                    "jsonrpc": "2.0",
                    "method": "notifications/message",
                    "params": {"level": "info", "data": "calling"},
                }), flush=True)
                print(json.dumps({
                    "jsonrpc": "2.0",
                    "id": msg["id"],
                    "result": {"content": [{"type": "text", "text": "ok"}]},
                }), flush=True)
        """,
    )

    async def run():
        client = mcp_mod.MCPStdioClient(
            {"transport_type": "stdio", "command": sys.executable, "args": [script_path]}
        )
        try:
            await client.start()
            tools = await client.list_tools()
            assert client.server_info["name"] == "stdio-test"
            assert tools[0]["name"] == "echo"

            notifications = []

            async def on_notification(msg):
                notifications.append(msg)

            result = await client.call_tool("echo", {"hello": "world"}, on_notification=on_notification)
            assert result["content"][0]["text"] == "ok"
            assert notifications[0]["method"] == "notifications/message"
        finally:
            await client.stop()

    asyncio.run(run())


def test_mcp_stdio_timeout_marks_client_tainted_and_manager_rebuilds(tmp_path, monkeypatch):
    from open_webui.utils import mcp as mcp_mod

    monkeypatch.setattr(
        mcp_mod,
        "DEFAULT_STDIO_ALLOWED_COMMANDS",
        mcp_mod.DEFAULT_STDIO_ALLOWED_COMMANDS | {pathlib.Path(sys.executable).name.lower()},
    )
    monkeypatch.setattr(mcp_mod, "MCP_TOOL_CALL_TIMEOUT", 1)

    script_path = _write_stdio_server(
        tmp_path,
        "slow_stdio_server.py",
        """
        import json
        import sys
        import time

        for raw in sys.stdin:
            msg = json.loads(raw)
            method = msg.get("method")
            if method == "initialize":
                print(json.dumps({
                    "jsonrpc": "2.0",
                    "id": msg["id"],
                    "result": {
                        "protocolVersion": "2025-06-18",
                        "serverInfo": {"name": "slow-stdio"},
                        "capabilities": {"tools": {}},
                    },
                }), flush=True)
            elif method == "notifications/initialized":
                continue
            elif method == "tools/list":
                print(json.dumps({
                    "jsonrpc": "2.0",
                    "id": msg["id"],
                    "result": {
                        "tools": [
                            {
                                "name": "sleep",
                                "description": "Sleep tool",
                                "inputSchema": {"type": "object"},
                            }
                        ]
                    },
                }), flush=True)
            elif method == "tools/call":
                time.sleep(2)
                print(json.dumps({
                    "jsonrpc": "2.0",
                    "id": msg["id"],
                    "result": {"content": [{"type": "text", "text": "done"}]},
                }), flush=True)
        """,
    )

    async def run():
        manager = mcp_mod.MCPStdioProcessManager.instance()
        connection = {
            "transport_type": "stdio",
            "command": sys.executable,
            "args": [script_path],
        }

        try:
            client1 = await manager.get_or_start(connection, "user-1")
            with pytest.raises(RuntimeError, match="timed out"):
                await client1.call_tool("sleep", {})
            assert client1.tainted is True

            client2 = await manager.get_or_start(connection, "user-1")
            assert client2 is not client1
        finally:
            await manager.stop_all()

    asyncio.run(run())


def test_get_mcp_servers_data_only_fetches_selected_indices(monkeypatch):
    from open_webui.utils import mcp as mcp_mod

    seen = []

    async def fake_get_mcp_server_data(connection, **_kwargs):
        seen.append(connection["url"])
        return {
            "server_info": {"name": connection["url"]},
            "capabilities": {},
            "tools": [],
        }

    monkeypatch.setattr(mcp_mod, "get_mcp_server_data", fake_get_mcp_server_data)

    async def run():
        results = await mcp_mod.get_mcp_servers_data(
            [
                {"url": "http://one", "config": {"enable": True}},
                {"url": "http://two", "config": {"enable": True}},
                {"url": "http://three", "config": {"enable": True}},
            ],
            selected_indices={1},
            strict_selected=True,
        )
        assert [result["idx"] for result in results] == [1]

    asyncio.run(run())
    assert seen == ["http://two"]


def test_get_mcp_servers_data_strict_selected_rejects_invalid_index():
    from open_webui.utils import mcp as mcp_mod

    async def run():
        with pytest.raises(RuntimeError, match="所选 MCP 服务器当前不可用"):
            await mcp_mod.get_mcp_servers_data(
                [{"url": "http://one", "config": {"enable": True}}],
                selected_indices={2},
                strict_selected=True,
            )

    asyncio.run(run())


def test_mcp_server_connection_validates_transport_fields():
    from pydantic import ValidationError

    from open_webui.routers.configs import MCPServerConnection

    with pytest.raises(ValidationError):
        MCPServerConnection(transport_type="http")

    with pytest.raises(ValidationError):
        MCPServerConnection(transport_type="stdio")

    http_conn = MCPServerConnection(transport_type="http", url="http://example.com")
    assert http_conn.transport_type == "http"
    assert http_conn.url == "http://example.com"

    stdio_conn = MCPServerConnection(
        transport_type="stdio",
        command="python",
        args=["server.py"],
        env={"TOKEN": "abc"},
    )
    assert stdio_conn.transport_type == "stdio"
    assert stdio_conn.command == "python"
