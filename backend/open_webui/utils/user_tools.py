"""
Per-user tool configuration (account-level).

This repo historically stored some tool-related settings globally in app.state.config:
- TOOL_SERVER_CONNECTIONS (OpenAPI tool servers)
- MCP_SERVER_CONNECTIONS (MCP servers)
- Native/Builtin tool gates (TOOL_CALLING_MODE, ENABLE_WEB_SEARCH_TOOL, ...)

The requirement for HaloWebUI is that *each account* (admins and regular users) owns
their own tool configuration. This module provides a single place to:
- Read/write per-user tool settings in user.settings["tools"]
- Provide safe defaults from the global config
- Provide best-effort migration for admins from legacy global configs

Storage layout (within user.settings JSON):
  {
    "tools": {
      "valves": {...},  # existing per-user tool valves (kept intact)
      "native_tools": { ...NativeToolsConfigForm fields... },
      "tool_server_connections": [ ...OpenAPI tool server connections... ],
      "mcp_server_connections": [ ...MCP server connections... ]
    }
  }
"""

from __future__ import annotations

from copy import deepcopy
from typing import Any, Optional

from open_webui.models.users import Users, UserModel


TOOLS_KEY = "tools"
NATIVE_TOOLS_KEY = "native_tools"
TOOL_SERVER_CONNECTIONS_KEY = "tool_server_connections"
MCP_SERVER_CONNECTIONS_KEY = "mcp_server_connections"
TOOL_CALLING_MODE_KEY = "TOOL_CALLING_MODE"
TOOL_CALLING_MODE_DEFAULT = "default"
TOOL_CALLING_MODE_NATIVE = "native"
TOOL_CALLING_MODE_ALLOWED = {TOOL_CALLING_MODE_DEFAULT, TOOL_CALLING_MODE_NATIVE}
MAX_TOOL_CALL_ROUNDS_KEY = "MAX_TOOL_CALL_ROUNDS"
MAX_TOOL_CALL_ROUNDS_DEFAULT = 15
MAX_TOOL_CALL_ROUNDS_MIN = 1
MAX_TOOL_CALL_ROUNDS_MAX = 30


def _clamp_int(value: Any, default: int, min_value: int, max_value: int) -> int:
    try:
        n = int(value)
    except Exception:
        n = int(default)
    if n < min_value:
        return min_value
    if n > max_value:
        return max_value
    return n


def normalize_max_tool_call_rounds(
    value: Any,
    default: int = MAX_TOOL_CALL_ROUNDS_DEFAULT,
) -> int:
    return _clamp_int(
        value,
        default=default,
        min_value=MAX_TOOL_CALL_ROUNDS_MIN,
        max_value=MAX_TOOL_CALL_ROUNDS_MAX,
    )


def normalize_tool_calling_mode(
    value: Any,
    default: str = TOOL_CALLING_MODE_DEFAULT,
) -> str:
    mode = str(value or "").strip().lower()
    return mode if mode in TOOL_CALLING_MODE_ALLOWED else default


def _as_dict(v: Any) -> dict:
    return v if isinstance(v, dict) else {}


def _as_list(v: Any) -> list:
    return v if isinstance(v, list) else []


def _get_settings_dict(user: Optional[UserModel]) -> dict:
    if not user or not getattr(user, "settings", None):
        return {}
    try:
        # Pydantic model
        return _as_dict(user.settings.model_dump())
    except Exception:
        # Best-effort fallback
        return _as_dict(getattr(user, "settings", None))


def _get_tools_settings(user: Optional[UserModel]) -> dict:
    settings = _get_settings_dict(user)
    return _as_dict(settings.get(TOOLS_KEY))


def _update_tools_settings(user_id: str, patch: dict) -> Optional[UserModel]:
    """
    Merge a partial patch into user.settings["tools"] without clobbering sibling keys
    like tools.valves.
    """
    user = Users.get_user_by_id(user_id)
    if not user:
        return None

    settings = _get_settings_dict(user)
    tools = _as_dict(settings.get(TOOLS_KEY))

    # Shallow merge at the tools dict level; callers set full values per key.
    tools.update(patch)
    settings[TOOLS_KEY] = tools

    # update_user_settings_by_id does a root-level shallow merge; we already merged "tools".
    updated = Users.update_user_settings_by_id(user_id, {TOOLS_KEY: tools})
    return updated


def maybe_migrate_user_tool_settings(request, user: UserModel) -> UserModel:
    """
    Best-effort migration for legacy global tool configs to per-user settings.

    Rules:
    - Only admins are seeded from global configs.
    - Only seed missing keys (do not overwrite an existing per-user value).
    - Never delete legacy global configs.
    """
    if not user or getattr(user, "role", None) != "admin":
        return user

    tools = _get_tools_settings(user)
    changed = False

    cfg = getattr(getattr(request, "app", None), "state", None)
    cfg = getattr(cfg, "config", None)
    if cfg is None:
        return user

    legacy_tool_servers = deepcopy(getattr(cfg, "TOOL_SERVER_CONNECTIONS", []) or [])
    legacy_mcp = deepcopy(getattr(cfg, "MCP_SERVER_CONNECTIONS", []) or [])

    if TOOL_SERVER_CONNECTIONS_KEY not in tools and legacy_tool_servers:
        tools[TOOL_SERVER_CONNECTIONS_KEY] = legacy_tool_servers
        changed = True

    if MCP_SERVER_CONNECTIONS_KEY not in tools and legacy_mcp:
        tools[MCP_SERVER_CONNECTIONS_KEY] = legacy_mcp
        changed = True

    if not changed:
        return user

    updated = Users.update_user_settings_by_id(user.id, {TOOLS_KEY: tools})
    return updated or user


def get_user_tool_server_connections(request, user: Optional[UserModel]) -> list[dict]:
    tools = _get_tools_settings(user)
    if TOOL_SERVER_CONNECTIONS_KEY in tools:
        return _as_list(tools.get(TOOL_SERVER_CONNECTIONS_KEY))

    # Admin migration fallback (read-only): preserve legacy behavior for existing installs
    if user and getattr(user, "role", None) == "admin":
        cfg = getattr(getattr(request, "app", None), "state", None)
        cfg = getattr(cfg, "config", None)
        legacy = getattr(cfg, "TOOL_SERVER_CONNECTIONS", None) if cfg is not None else None
        legacy = legacy if isinstance(legacy, list) else []
        if legacy:
            return deepcopy(legacy)

    return []


def set_user_tool_server_connections(user: UserModel, connections: list[dict]) -> Optional[UserModel]:
    return _update_tools_settings(user.id, {TOOL_SERVER_CONNECTIONS_KEY: connections})


def get_user_mcp_server_connections(request, user: Optional[UserModel]) -> list[dict]:
    tools = _get_tools_settings(user)
    role = getattr(user, "role", None) if user else None

    def _filter_stdio(connections: list[dict]) -> list[dict]:
        if role == "admin":
            return connections
        return [
            connection
            for connection in connections
            if str(connection.get("transport_type") or "http").lower() != "stdio"
        ]

    if MCP_SERVER_CONNECTIONS_KEY in tools:
        return _filter_stdio(_as_list(tools.get(MCP_SERVER_CONNECTIONS_KEY)))

    # Admin migration fallback (read-only)
    if user and role == "admin":
        cfg = getattr(getattr(request, "app", None), "state", None)
        cfg = getattr(cfg, "config", None)
        legacy = getattr(cfg, "MCP_SERVER_CONNECTIONS", None) if cfg is not None else None
        legacy = legacy if isinstance(legacy, list) else []
        if legacy:
            return _filter_stdio(deepcopy(legacy))

    return []


def set_user_mcp_server_connections(user: UserModel, connections: list[dict]) -> Optional[UserModel]:
    return _update_tools_settings(user.id, {MCP_SERVER_CONNECTIONS_KEY: connections})


def _native_defaults_from_global(request) -> dict:
    """
    Default per-user native tool config.

    These are not secrets; it is safe to default from global config (env/config file).
    Global config continues to act as an upper bound for server-side features elsewhere.
    """
    cfg = getattr(getattr(request, "app", None), "state", None)
    cfg = getattr(cfg, "config", None)

    def _get_bool(name: str, default: bool) -> bool:
        try:
            return bool(getattr(cfg, name)) if cfg is not None else default
        except Exception:
            return default

    def _get_str(name: str, default: str) -> str:
        try:
            v = getattr(cfg, name) if cfg is not None else None
            return str(v) if v is not None else default
        except Exception:
            return default

    mode = normalize_tool_calling_mode(
        _get_str(TOOL_CALLING_MODE_KEY, TOOL_CALLING_MODE_DEFAULT),
        default=TOOL_CALLING_MODE_DEFAULT,
    )

    return {
        TOOL_CALLING_MODE_KEY: mode,
        "ENABLE_INTERLEAVED_THINKING": _get_bool("ENABLE_INTERLEAVED_THINKING", False),
        MAX_TOOL_CALL_ROUNDS_KEY: MAX_TOOL_CALL_ROUNDS_DEFAULT,
        "ENABLE_WEB_SEARCH_TOOL": _get_bool("ENABLE_WEB_SEARCH_TOOL", True),
        "ENABLE_URL_FETCH": _get_bool("ENABLE_URL_FETCH", True),
        "ENABLE_URL_FETCH_RENDERED": _get_bool("ENABLE_URL_FETCH_RENDERED", False),
        "ENABLE_LIST_KNOWLEDGE_BASES": _get_bool("ENABLE_LIST_KNOWLEDGE_BASES", True),
        "ENABLE_SEARCH_KNOWLEDGE_BASES": _get_bool("ENABLE_SEARCH_KNOWLEDGE_BASES", True),
        "ENABLE_QUERY_KNOWLEDGE_FILES": _get_bool("ENABLE_QUERY_KNOWLEDGE_FILES", True),
        "ENABLE_VIEW_KNOWLEDGE_FILE": _get_bool("ENABLE_VIEW_KNOWLEDGE_FILE", True),
        "ENABLE_IMAGE_GENERATION_TOOL": _get_bool("ENABLE_IMAGE_GENERATION_TOOL", True),
        "ENABLE_IMAGE_EDIT": _get_bool("ENABLE_IMAGE_EDIT", False),
        "ENABLE_MEMORY_TOOLS": _get_bool("ENABLE_MEMORY_TOOLS", True),
        "ENABLE_NOTES": _get_bool("ENABLE_NOTES", False),
        "ENABLE_CHAT_HISTORY_TOOLS": _get_bool("ENABLE_CHAT_HISTORY_TOOLS", True),
        "ENABLE_TIME_TOOLS": _get_bool("ENABLE_TIME_TOOLS", True),
        "ENABLE_CHANNEL_TOOLS": _get_bool("ENABLE_CHANNEL_TOOLS", True),
        "ENABLE_TERMINAL_TOOL": _get_bool("ENABLE_TERMINAL_TOOL", False),
    }


def get_user_native_tools_config(request, user: Optional[UserModel]) -> dict:
    """
    Return effective native tool config for a user.

    Merge strategy:
    - Start with global defaults.
    - Overlay user.settings.tools.native_tools (when present).
    """
    defaults = _native_defaults_from_global(request)
    tools = _get_tools_settings(user)
    override = _as_dict(tools.get(NATIVE_TOOLS_KEY))

    # Only apply known keys to avoid persisting/echoing garbage.
    effective = dict(defaults)
    for k in defaults.keys():
        if k in override:
            effective[k] = override[k]

    # Normalize types.
    default_mode = normalize_tool_calling_mode(
        defaults.get(TOOL_CALLING_MODE_KEY),
        default=TOOL_CALLING_MODE_DEFAULT,
    )
    effective[TOOL_CALLING_MODE_KEY] = normalize_tool_calling_mode(
        effective.get(TOOL_CALLING_MODE_KEY),
        default=default_mode,
    )

    default_rounds = normalize_max_tool_call_rounds(
        defaults.get(MAX_TOOL_CALL_ROUNDS_KEY),
        default=MAX_TOOL_CALL_ROUNDS_DEFAULT,
    )
    effective[MAX_TOOL_CALL_ROUNDS_KEY] = normalize_max_tool_call_rounds(
        effective.get(MAX_TOOL_CALL_ROUNDS_KEY),
        default=default_rounds,
    )

    for k, v in list(effective.items()):
        if k in {TOOL_CALLING_MODE_KEY, MAX_TOOL_CALL_ROUNDS_KEY}:
            continue
        effective[k] = bool(v)

    return effective


def set_user_native_tools_config(user: UserModel, native_cfg: dict) -> Optional[UserModel]:
    """
    Persist a full NativeToolsConfigForm payload under user.settings.tools.native_tools.
    """
    # Store as-is; the API layer validates/sanitizes.
    return _update_tools_settings(user.id, {NATIVE_TOOLS_KEY: native_cfg})
