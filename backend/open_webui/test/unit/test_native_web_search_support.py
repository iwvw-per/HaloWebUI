import pathlib
import sys


_BACKEND_DIR = pathlib.Path(__file__).resolve().parents[3]
if str(_BACKEND_DIR) not in sys.path:
    sys.path.insert(0, str(_BACKEND_DIR))

from open_webui.utils.native_web_search import (  # noqa: E402
    NATIVE_WEB_SEARCH_STATUS_SUPPORTED,
    NATIVE_WEB_SEARCH_STATUS_UNKNOWN,
    NATIVE_WEB_SEARCH_STATUS_UNSUPPORTED,
    build_native_web_search_support,
    load_native_web_search_rules,
    resolve_effective_native_web_search_support,
    resolve_model_native_web_search_rule,
    strip_model_prefix,
)


def test_native_web_search_rules_file_loads():
    rules = load_native_web_search_rules()
    assert isinstance(rules, dict)
    assert "providers" in rules
    assert "openai" in rules["providers"]
    assert "gemini" in rules["providers"]


def test_openai_model_rule_distinguishes_supported_unknown_and_denied_models():
    supported = resolve_model_native_web_search_rule(
        "openai", model_id="gpt-4o", model_name="GPT-4o"
    )
    assert supported["status"] == NATIVE_WEB_SEARCH_STATUS_SUPPORTED

    denied = resolve_model_native_web_search_rule(
        "openai", model_id="gpt-4.1-nano", model_name="GPT-4.1 Nano"
    )
    assert denied["status"] == NATIVE_WEB_SEARCH_STATUS_UNSUPPORTED

    unknown = resolve_model_native_web_search_rule(
        "openai", model_id="gpt-future-x", model_name="GPT Future X"
    )
    assert unknown["status"] == NATIVE_WEB_SEARCH_STATUS_UNKNOWN


def test_effective_support_is_model_specific_on_same_connection():
    connection_support = build_native_web_search_support(
        "openai",
        url="https://api.openai.com/v1",
        api_config={},
    )

    gpt_4o = resolve_effective_native_web_search_support(
        connection_support,
        provider="openai",
        model_id="gpt-4o",
        model_name="GPT-4o",
    )
    nano = resolve_effective_native_web_search_support(
        connection_support,
        provider="openai",
        model_id="gpt-4.1-nano",
        model_name="GPT-4.1 Nano",
    )
    future = resolve_effective_native_web_search_support(
        connection_support,
        provider="openai",
        model_id="gpt-future-x",
        model_name="GPT Future X",
    )

    assert gpt_4o["status"] == NATIVE_WEB_SEARCH_STATUS_SUPPORTED
    assert gpt_4o["supported"] is True

    assert nano["status"] == NATIVE_WEB_SEARCH_STATUS_UNSUPPORTED
    assert nano["supported"] is False
    assert nano["can_attempt"] is False

    assert future["status"] == NATIVE_WEB_SEARCH_STATUS_UNKNOWN
    assert future["supported"] is False
    assert future["can_attempt"] is True


def test_connection_level_disable_overrides_supported_model():
    connection_support = build_native_web_search_support(
        "openai",
        url="https://api.openai.com/v1",
        api_config={"native_web_search_enabled": False},
    )

    support = resolve_effective_native_web_search_support(
        connection_support,
        provider="openai",
        model_id="gpt-4o",
        model_name="GPT-4o",
    )

    assert support["status"] == NATIVE_WEB_SEARCH_STATUS_UNSUPPORTED
    assert support["reason"] == "connection_disabled"
    assert support["can_attempt"] is False


def test_gemini_rules_allow_supported_family_and_deny_non_chat_models():
    connection_support = build_native_web_search_support(
        "gemini",
        url="https://generativelanguage.googleapis.com/v1beta",
        api_config={},
    )

    supported = resolve_effective_native_web_search_support(
        connection_support,
        provider="gemini",
        model_id="gemini-2.5-flash",
        model_name="Gemini 2.5 Flash",
    )
    denied = resolve_effective_native_web_search_support(
        connection_support,
        provider="gemini",
        model_id="gemini-embedding-001",
        model_name="Gemini Embedding 001",
    )

    assert supported["status"] == NATIVE_WEB_SEARCH_STATUS_SUPPORTED
    assert denied["status"] == NATIVE_WEB_SEARCH_STATUS_UNSUPPORTED


def test_strip_model_prefix_only_removes_resolved_connection_prefix():
    assert strip_model_prefix("prod.gpt-4o", "prod") == "gpt-4o"
    assert strip_model_prefix("gpt-4.1", "prod") == "gpt-4.1"
    assert strip_model_prefix("gpt-4.1", "") == "gpt-4.1"
