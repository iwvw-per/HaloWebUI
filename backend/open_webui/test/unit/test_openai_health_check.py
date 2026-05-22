import asyncio
import json
import pathlib
import sys

import pytest
from fastapi import HTTPException


_BACKEND_DIR = pathlib.Path(__file__).resolve().parents[3]
if str(_BACKEND_DIR) not in sys.path:
    sys.path.insert(0, str(_BACKEND_DIR))

from open_webui.models.users import UserModel  # noqa: E402
from open_webui.routers import openai as openai_router  # noqa: E402


def _make_user() -> UserModel:
    return UserModel(
        id="user-1",
        name="Test User",
        email="user@example.com",
        role="user",
        profile_image_url="/user.png",
        last_active_at=0,
        updated_at=0,
        created_at=0,
    )


class _FakeResponse:
    def __init__(self, status: int, body, headers: dict | None = None):
        self.status = status
        self._body = body
        self.headers = headers or {"Content-Type": "application/json"}

    async def __aenter__(self):
        return self

    async def __aexit__(self, exc_type, exc, tb):
        return False

    async def json(self):
        if isinstance(self._body, Exception):
            raise self._body
        if isinstance(self._body, (dict, list)):
            return self._body
        raise ValueError("not json")

    async def text(self):
        if isinstance(self._body, str):
            return self._body
        return json.dumps(self._body, ensure_ascii=False)


class _FakeSession:
    def __init__(self, planned_responses, requests_log):
        self._planned_responses = list(planned_responses)
        self._requests_log = requests_log

    async def __aenter__(self):
        return self

    async def __aexit__(self, exc_type, exc, tb):
        return False

    def get(self, url, headers=None, **kwargs):
        self._requests_log.append(("GET", url, headers, None))
        return self._planned_responses.pop(0)

    def post(self, url, headers=None, data=None, **kwargs):
        self._requests_log.append(("POST", url, headers, data))
        return self._planned_responses.pop(0)


def _install_fake_session(monkeypatch, planned_responses, requests_log):
    monkeypatch.setattr(
        openai_router.aiohttp,
        "ClientSession",
        lambda *args, **kwargs: _FakeSession(planned_responses, requests_log),
    )


def test_health_check_uses_responses_api_when_enabled(monkeypatch):
    requests_log = []
    _install_fake_session(
        monkeypatch,
        [
            _FakeResponse(
                200,
                {"id": "resp_123", "output": [], "status": "completed"},
            )
        ],
        requests_log,
    )

    result = asyncio.run(
        openai_router.health_check_connection(
            openai_router.HealthCheckForm(
                url="https://api.openai.com/v1",
                key="sk-test",
                config={"use_responses_api": True},
                model="gpt-4o-mini",
            ),
            user=_make_user(),
        )
    )

    assert result["ok"] is True
    assert result["model"] == "gpt-4o-mini"
    assert result["response_time_ms"] >= 1
    assert requests_log[0][0] == "POST"
    assert requests_log[0][1] == "https://api.openai.com/v1/responses"

    payload = json.loads(requests_log[0][3])
    assert payload["model"] == "gpt-4o-mini"
    assert payload["stream"] is False
    assert payload["max_output_tokens"] == 16


def test_health_check_retries_azure_deployment_fallback(monkeypatch):
    requests_log = []
    _install_fake_session(
        monkeypatch,
        [
            _FakeResponse(404, {"error": {"message": "chat/completions not found"}}),
            _FakeResponse(200, {"id": "chatcmpl_123", "choices": [{"message": {"content": "ok"}}]}),
        ],
        requests_log,
    )

    result = asyncio.run(
        openai_router.health_check_connection(
            openai_router.HealthCheckForm(
                url="https://example-resource.openai.azure.com",
                key="azure-key",
                config={"azure": True, "api_version": "2025-01-01-preview"},
                model="gpt-4.1",
            ),
            user=_make_user(),
        )
    )

    assert result["ok"] is True
    assert requests_log[0][1] == "https://example-resource.openai.azure.com/openai/v1/chat/completions"
    assert (
        requests_log[1][1]
        == "https://example-resource.openai.azure.com/openai/deployments/gpt-4.1/chat/completions?api-version=2025-01-01-preview"
    )

    first_payload = json.loads(requests_log[0][3])
    second_payload = json.loads(requests_log[1][3])
    assert first_payload["model"] == "gpt-4.1"
    assert "model" not in second_payload


def test_health_check_converts_o1_max_tokens(monkeypatch):
    requests_log = []
    _install_fake_session(
        monkeypatch,
        [
            _FakeResponse(200, {"id": "chatcmpl_456", "choices": [{"message": {"content": "ok"}}]}),
        ],
        requests_log,
    )

    result = asyncio.run(
        openai_router.health_check_connection(
            openai_router.HealthCheckForm(
                url="https://api.openai.com/v1",
                key="sk-test",
                model="o1-mini",
            ),
            user=_make_user(),
        )
    )

    assert result["ok"] is True

    payload = json.loads(requests_log[0][3])
    assert payload["model"] == "o1-mini"
    assert payload["max_completion_tokens"] == 1
    assert "max_tokens" not in payload


def test_health_check_surfaces_upstream_error(monkeypatch):
    requests_log = []
    _install_fake_session(
        monkeypatch,
        [
            _FakeResponse(401, {"error": {"message": "invalid_api_key"}}),
        ],
        requests_log,
    )

    with pytest.raises(HTTPException) as exc_info:
        asyncio.run(
            openai_router.health_check_connection(
                openai_router.HealthCheckForm(
                    url="https://api.openai.com/v1",
                    key="bad-key",
                    model="gpt-4o-mini",
                ),
                user=_make_user(),
            )
        )

    assert exc_info.value.status_code == 401
    assert "invalid_api_key" in str(exc_info.value.detail)


def test_health_check_retries_next_api_key_on_rate_limit(monkeypatch):
    requests_log = []
    _install_fake_session(
        monkeypatch,
        [
            _FakeResponse(429, {"error": {"message": "rate limit"}}),
            _FakeResponse(200, {"id": "chatcmpl_789", "choices": [{"message": {"content": "ok"}}]}),
        ],
        requests_log,
    )

    result = asyncio.run(
        openai_router.health_check_connection(
            openai_router.HealthCheckForm(
                url="https://api.openai.com/v1",
                key="sk-a",
                config={
                    "api_key_pool": {
                        "keys": [
                            {"id": "a", "label": "A", "key": "sk-a", "enabled": True},
                            {"id": "b", "label": "B", "key": "sk-b", "enabled": True},
                        ],
                        "mode": "priority",
                        "retry": {"enabled": True},
                    }
                },
                model="gpt-4o-mini",
            ),
            user=_make_user(),
        )
    )

    assert result["ok"] is True
    assert len(requests_log) == 2
    assert requests_log[0][2]["Authorization"] == "Bearer sk-a"
    assert requests_log[1][2]["Authorization"] == "Bearer sk-b"
