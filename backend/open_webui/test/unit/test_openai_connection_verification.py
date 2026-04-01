import pathlib
import sys


_BACKEND_DIR = pathlib.Path(__file__).resolve().parents[3]
if str(_BACKEND_DIR) not in sys.path:
    sys.path.insert(0, str(_BACKEND_DIR))

from open_webui.routers.openai import (  # noqa: E402
    _build_chat_completion_request_attempts,
    _build_upstream_headers,
    _get_openai_chat_completions_url,
    _get_openai_models_url,
    _is_dashscope_compatible_connection,
    _looks_like_models_listing_unsupported,
    _normalize_openai_models_response,
)


def test_normalize_openai_models_response_accepts_models_array():
    normalized = _normalize_openai_models_response(
        {
            "models": [
                {"name": "qwen3-coder-plus"},
                "qwen3-coder-turbo",
            ]
        }
    )

    assert normalized == {
        "object": "list",
        "models": [
            {"name": "qwen3-coder-plus"},
            "qwen3-coder-turbo",
        ],
        "data": [
            {"name": "qwen3-coder-plus", "id": "qwen3-coder-plus", "object": "model"},
            {"id": "qwen3-coder-turbo", "object": "model"},
        ],
    }


def test_looks_like_models_listing_unsupported_accepts_empty_404():
    assert _looks_like_models_listing_unsupported(404, "") is True
    assert _looks_like_models_listing_unsupported(405, "<html>not found</html>") is True
    assert (
        _looks_like_models_listing_unsupported(
            400,
            {"error": {"message": "invalid request", "type": "invalid_request_error"}},
        )
        is False
    )


def test_is_dashscope_compatible_connection_matches_official_hosts_only():
    assert _is_dashscope_compatible_connection("https://dashscope.aliyuncs.com/compatible-mode/v1") is True
    assert _is_dashscope_compatible_connection("https://coding.dashscope.aliyuncs.com/v1") is True
    assert _is_dashscope_compatible_connection("https://dashscope.aliyuncs.com/v1") is False
    assert _is_dashscope_compatible_connection("https://api.openai.com/v1") is False


def test_azure_models_and_chat_urls_normalize_to_openai_v1():
    cfg = {"azure": True, "api_version": "2025-01-01-preview"}

    assert (
        _get_openai_models_url("https://example-resource.openai.azure.com/v1", cfg)
        == "https://example-resource.openai.azure.com/openai/v1/models"
    )
    assert (
        _get_openai_chat_completions_url("https://example-resource.openai.azure.com", cfg)
        == "https://example-resource.openai.azure.com/openai/v1/chat/completions"
    )


def test_azure_headers_default_to_api_key_auth():
    headers = _build_upstream_headers(
        "https://example-resource.openai.azure.com/openai/v1",
        "test-key",
        {"azure": True},
    )

    assert headers["api-key"] == "test-key"
    assert "Authorization" not in headers


def test_azure_chat_attempts_add_legacy_deployment_fallback_without_model_field():
    attempts = _build_chat_completion_request_attempts(
        url="https://example-resource.openai.azure.com",
        api_config={"azure": True, "api_version": "2025-01-01-preview"},
        model_id="gpt-4.1",
        payload_dict={"model": "gpt-4.1", "messages": [{"role": "user", "content": "ping"}]},
    )

    assert attempts[0][0] == "https://example-resource.openai.azure.com/openai/v1/chat/completions"
    assert attempts[0][1]["model"] == "gpt-4.1"
    assert (
        attempts[1][0]
        == "https://example-resource.openai.azure.com/openai/deployments/gpt-4.1/chat/completions?api-version=2025-01-01-preview"
    )
    assert "model" not in attempts[1][1]
