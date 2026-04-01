import pathlib
import sys
from types import SimpleNamespace


_BACKEND_DIR = pathlib.Path(__file__).resolve().parents[3]
if str(_BACKEND_DIR) not in sys.path:
    sys.path.insert(0, str(_BACKEND_DIR))

from open_webui.routers.images import _normalize_image_provider_base_url  # noqa: E402
from open_webui.routers.images import _resolve_image_provider_source  # noqa: E402
from open_webui.routers.images import _sync_image_provider_config_state  # noqa: E402


def test_openai_image_settings_auto_append_v1():
    normalized, force_mode = _normalize_image_provider_base_url(
        "https://api.example.com",
        "/v1",
    )

    assert normalized == "https://api.example.com/v1"
    assert force_mode is False


def test_openai_image_settings_preserve_irregular_version_path():
    normalized, force_mode = _normalize_image_provider_base_url(
        "https://relay.example.com/api/v3",
        "/v1",
    )

    assert normalized == "https://relay.example.com/api/v3"
    assert force_mode is False


def test_openai_image_settings_strip_known_endpoint_suffixes():
    normalized, force_mode = _normalize_image_provider_base_url(
        "https://api.example.com/v1/chat/completions",
        "/v1",
    )

    assert normalized == "https://api.example.com/v1"
    assert force_mode is False


def test_openai_image_settings_hash_enables_exact_mode():
    normalized, force_mode = _normalize_image_provider_base_url(
        "https://relay.example.com/custom/path#",
        "/v1",
    )

    assert normalized == "https://relay.example.com/custom/path"
    assert force_mode is True


def test_gemini_image_settings_force_mode_is_preserved_from_payload():
    normalized, force_mode = _normalize_image_provider_base_url(
        "https://generativelanguage.googleapis.com/custom",
        "/v1beta",
        force_mode=True,
    )

    assert normalized == "https://generativelanguage.googleapis.com/custom"
    assert force_mode is True


def test_image_settings_source_does_not_inherit_global_openai_auth_config_when_key_is_explicit():
    cfg = SimpleNamespace(
        IMAGES_OPENAI_API_BASE_URL="https://api.example.com/v1",
        IMAGES_OPENAI_API_KEY="image-key",
        IMAGES_OPENAI_API_FORCE_MODE=False,
        OPENAI_API_BASE_URLS=["https://api.example.com/v1"],
        OPENAI_API_KEYS=["global-key"],
        OPENAI_API_CONFIGS={"0": {"auth_type": "api-key", "force_mode": True}},
        IMAGES_GEMINI_API_BASE_URL="",
        IMAGES_GEMINI_API_KEY="",
        IMAGES_GEMINI_API_FORCE_MODE=False,
        GEMINI_API_BASE_URLS=[],
        GEMINI_API_KEYS=[],
        GEMINI_API_CONFIGS={},
    )
    request = SimpleNamespace(app=SimpleNamespace(state=SimpleNamespace(config=cfg)))

    source = _resolve_image_provider_source(
        request,
        user=None,
        provider="openai",
        context="settings",
    )

    assert source is not None
    assert source["key"] == "image-key"
    assert source["api_config"] == {}


def test_image_runtime_shared_source_keeps_image_force_mode_while_merging_global_config():
    cfg = SimpleNamespace(
        IMAGES_OPENAI_API_BASE_URL="https://api.example.com/v1",
        IMAGES_OPENAI_API_KEY="image-key",
        IMAGES_OPENAI_API_FORCE_MODE=True,
        OPENAI_API_BASE_URLS=["https://api.example.com/v1"],
        OPENAI_API_KEYS=["global-key"],
        OPENAI_API_CONFIGS={"0": {"auth_type": "bearer"}},
        ENABLE_IMAGE_GENERATION_SHARED_KEY=True,
        IMAGES_GEMINI_API_BASE_URL="",
        IMAGES_GEMINI_API_KEY="",
        IMAGES_GEMINI_API_FORCE_MODE=False,
        GEMINI_API_BASE_URLS=[],
        GEMINI_API_KEYS=[],
        GEMINI_API_CONFIGS={},
    )
    request = SimpleNamespace(app=SimpleNamespace(state=SimpleNamespace(config=cfg)))

    source = _resolve_image_provider_source(
        request,
        user=None,
        provider="openai",
        context="runtime",
        credential_source="shared",
    )

    assert source is not None
    assert source["api_config"]["auth_type"] == "bearer"
    assert source["api_config"]["force_mode"] is True


def test_image_settings_source_normalizes_legacy_openai_base_url_without_v1():
    cfg = SimpleNamespace(
        IMAGES_OPENAI_API_BASE_URL="https://api.example.com",
        IMAGES_OPENAI_API_KEY="image-key",
        IMAGES_OPENAI_API_FORCE_MODE=False,
        OPENAI_API_BASE_URLS=[],
        OPENAI_API_KEYS=[],
        OPENAI_API_CONFIGS={},
        IMAGES_GEMINI_API_BASE_URL="",
        IMAGES_GEMINI_API_KEY="",
        IMAGES_GEMINI_API_FORCE_MODE=False,
        GEMINI_API_BASE_URLS=[],
        GEMINI_API_KEYS=[],
        GEMINI_API_CONFIGS={},
    )
    request = SimpleNamespace(app=SimpleNamespace(state=SimpleNamespace(config=cfg)))

    source = _resolve_image_provider_source(
        request,
        user=None,
        provider="openai",
        context="settings",
    )

    assert source is not None
    assert source["base_url"] == "https://api.example.com/v1"
    assert source["api_config"] == {}


def test_sync_image_provider_config_state_persists_normalized_legacy_urls():
    class DummyConfig(SimpleNamespace):
        def __setattr__(self, key, value):
            super().__setattr__(key, value)

    cfg = DummyConfig(
        IMAGES_OPENAI_API_BASE_URL="https://api.example.com",
        IMAGES_OPENAI_API_KEY="image-key",
        IMAGES_OPENAI_API_FORCE_MODE=False,
        IMAGES_GEMINI_API_BASE_URL="https://generativelanguage.googleapis.com",
        IMAGES_GEMINI_API_KEY="gemini-key",
        IMAGES_GEMINI_API_FORCE_MODE=False,
    )
    request = SimpleNamespace(app=SimpleNamespace(state=SimpleNamespace(config=cfg)))

    _sync_image_provider_config_state(request)

    assert cfg.IMAGES_OPENAI_API_BASE_URL == "https://api.example.com/v1"
    assert cfg.IMAGES_OPENAI_API_FORCE_MODE is False
    assert cfg.IMAGES_GEMINI_API_BASE_URL == "https://generativelanguage.googleapis.com/v1beta"
    assert cfg.IMAGES_GEMINI_API_FORCE_MODE is False
