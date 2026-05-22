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
                "pinnedModels": ["gpt-4.1"],
                "modelSelectorTagOrder": ["OpenAI", "Anthropic"],
                "landingPageMode": "chat",
                "connections": {"openai": {"OPENAI_API_KEYS": ["secret"]}},
                "notifications": {"webhook_url": "https://example.test/hook"},
                "userLocation": True,
                "notificationEnabled": True,
                "speechAutoSend": True,
                "responseAutoPlayback": True,
                "showEmojiInCall": True,
                "voiceInterruption": True,
                "iframeSandboxAllowSameOrigin": True,
                "iframeSandboxAllowForms": True,
                "hapticFeedback": True,
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

    assert sanitized["configured"] is True
    assert sanitized["enabled"] is True
    assert sanitized["roles"] == ["user", "pending"]
    assert sanitized["ui"]["models"] == ["gpt-4o"]
    assert "pinnedModels" not in sanitized["ui"]
    assert "modelSelectorTagOrder" not in sanitized["ui"]
    assert "landingPageMode" not in sanitized["ui"]
    assert sanitized["ui"]["mermaidTheme"] == "lobe-theme"
    assert sanitized["ui"]["title"] == {"auto": False}
    assert "connections" not in sanitized["ui"]
    assert "notifications" not in sanitized["ui"]
    assert "userLocation" not in sanitized["ui"]
    assert "notificationEnabled" not in sanitized["ui"]
    assert "speechAutoSend" not in sanitized["ui"]
    assert "responseAutoPlayback" not in sanitized["ui"]
    assert "showEmojiInCall" not in sanitized["ui"]
    assert "voiceInterruption" not in sanitized["ui"]
    assert "iframeSandboxAllowSameOrigin" not in sanitized["ui"]
    assert "iframeSandboxAllowForms" not in sanitized["ui"]
    assert "hapticFeedback" not in sanitized["ui"]
    assert "audio" not in sanitized["ui"]
    assert sanitized["tools"]["native_tools"]["TOOL_CALLING_MODE"] == "native"
    assert sanitized["tools"]["native_tools"]["MAX_TOOL_CALL_ROUNDS"] == 30
    assert sanitized["tools"]["native_tools"]["ENABLE_WEB_SEARCH_TOOL"] is False
    assert "mcp_server_connections" not in sanitized["tools"]["native_tools"]


def test_new_user_default_settings_auto_enables_when_template_has_content():
    sanitized = sanitize_new_user_default_settings(
        {
            "enabled": False,
            "roles": [],
            "ui": {"models": ["gpt-4o"]},
            "tools": {"native_tools": {}},
        }
    )

    assert sanitized["configured"] is True
    assert sanitized["enabled"] is True
    assert sanitized["roles"] == ["user", "pending"]


def test_new_user_default_settings_disables_when_template_is_empty():
    sanitized = sanitize_new_user_default_settings(
        {
            "enabled": True,
            "roles": ["user"],
            "ui": {"connections": {"openai": {"OPENAI_API_KEYS": ["secret"]}}},
            "tools": {"native_tools": {"mcp_server_connections": [{"url": "https://secret"}]}},
        }
    )

    assert sanitized == {
        "configured": False,
        "enabled": False,
        "roles": ["user", "pending"],
        "ui": {},
        "tools": {"native_tools": {}},
    }


def test_new_user_default_settings_preserves_configured_empty_template():
    sanitized = sanitize_new_user_default_settings(
        {
            "configured": True,
            "enabled": True,
            "roles": ["user"],
            "ui": {"connections": {"openai": {"OPENAI_API_KEYS": ["secret"]}}},
            "tools": {"native_tools": {}},
        }
    )

    assert sanitized == {
        "configured": True,
        "enabled": False,
        "roles": ["user", "pending"],
        "ui": {},
        "tools": {"native_tools": {}},
    }


def test_new_user_default_settings_builds_for_new_non_admin_accounts():
    template = {
        "enabled": False,
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
    pending_settings = build_new_user_settings_from_template(template, "pending")

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
    assert pending_settings == settings
    assert build_new_user_settings_from_template(template, "admin") is None
