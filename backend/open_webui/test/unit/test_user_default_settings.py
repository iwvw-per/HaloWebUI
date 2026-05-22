from open_webui.utils.user_default_settings import (
    build_new_user_settings_from_template,
    sanitize_new_user_default_settings,
)


def test_new_user_default_settings_sanitizes_unsafe_fields():
    sanitized = sanitize_new_user_default_settings(
        {
            "enabled": True,
            "roles": ["user", "admin", "pending"],
            "ui": {
                "models": ["gpt-4o"],
                "connections": {"openai": {"OPENAI_API_KEYS": ["secret"]}},
                "notifications": {"webhook_url": "https://example.test/hook"},
                "userLocation": True,
                "notificationEnabled": True,
                "mermaidTheme": "lobe-theme",
                "title": {"auto": False, "prompt": "private prompt"},
                "audio": {
                    "stt": {"engine": "web", "language": "zh-CN"},
                    "tts": {
                        "engine": "browser-kokoro",
                        "playbackRate": 1.25,
                        "voice": "personal",
                    },
                },
            },
            "tools": {
                "native_tools": {
                    "TOOL_CALLING_MODE": "native",
                    "MAX_TOOL_CALL_ROUNDS": 99,
                    "ENABLE_WEB_SEARCH_TOOL": False,
                    "mcp_server_connections": [{"url": "https://secret"}],
                }
            },
        }
    )

    assert sanitized["enabled"] is True
    assert sanitized["roles"] == ["user", "pending"]
    assert sanitized["ui"]["models"] == ["gpt-4o"]
    assert sanitized["ui"]["mermaidTheme"] == "lobe-theme"
    assert sanitized["ui"]["title"] == {"auto": False}
    assert sanitized["ui"]["audio"]["stt"] == {"engine": "web", "language": "zh-CN"}
    assert sanitized["ui"]["audio"]["tts"] == {"playbackRate": 1.25}
    assert "connections" not in sanitized["ui"]
    assert "notifications" not in sanitized["ui"]
    assert "userLocation" not in sanitized["ui"]
    assert "notificationEnabled" not in sanitized["ui"]
    assert sanitized["tools"]["native_tools"]["TOOL_CALLING_MODE"] == "native"
    assert sanitized["tools"]["native_tools"]["MAX_TOOL_CALL_ROUNDS"] == 30
    assert sanitized["tools"]["native_tools"]["ENABLE_WEB_SEARCH_TOOL"] is False
    assert "mcp_server_connections" not in sanitized["tools"]["native_tools"]


def test_new_user_default_settings_builds_only_for_enabled_target_roles():
    template = {
        "enabled": True,
        "roles": ["user"],
        "ui": {
            "models": ["gpt-4o"],
            "temporaryChatByDefault": True,
        },
        "tools": {
            "native_tools": {
                "TOOL_CALLING_MODE": "native",
                "ENABLE_WEB_SEARCH_TOOL": False,
            }
        },
    }

    settings = build_new_user_settings_from_template(template, "user")

    assert settings == {
        "ui": {
            "models": ["gpt-4o"],
            "temporaryChatByDefault": True,
        },
        "tools": {
            "native_tools": {
                "TOOL_CALLING_MODE": "native",
                "ENABLE_WEB_SEARCH_TOOL": False,
            }
        },
        "revision": 0,
    }
    assert build_new_user_settings_from_template(template, "pending") is None
    assert build_new_user_settings_from_template(template, "admin") is None
    assert (
        build_new_user_settings_from_template({**template, "enabled": False}, "user")
        is None
    )
