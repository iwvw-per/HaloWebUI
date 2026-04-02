import pathlib
import sys


_BACKEND_DIR = pathlib.Path(__file__).resolve().parents[3]
if str(_BACKEND_DIR) not in sys.path:
    sys.path.insert(0, str(_BACKEND_DIR))

from open_webui.routers.images import (  # noqa: E402
    _classify_gemini_image_model,
    _classify_openai_image_model,
    load_b64_image_data,
)


def test_openai_output_image_metadata_prefers_chat_image_mode_for_relays():
    classified = _classify_openai_image_model(
        {
            "id": "openai/gpt-image-1",
            "name": "OpenAI GPT Image 1",
            "architecture": {"output_modalities": ["text", "image"]},
        },
        base_url="https://openrouter.ai/api/v1",
        api_config={},
        source={"effective_source": "personal"},
    )

    assert classified is not None
    assert classified["generation_mode"] == "openai_chat_image"
    assert classified["detection_method"] == "metadata"


def test_openai_official_gpt_image_family_prefers_images_endpoint_mode():
    classified = _classify_openai_image_model(
        {
            "id": "gpt-image-1",
            "name": "GPT Image 1",
        },
        base_url="https://api.openai.com/v1",
        api_config={},
        source={"effective_source": "shared"},
    )

    assert classified is not None
    assert classified["generation_mode"] == "openai_images"
    assert classified["supports_background"] is True


def test_openai_seedream_description_is_detected_as_image_model():
    classified = _classify_openai_image_model(
        {
            "id": "doubao-seedream-5.0-lite",
            "name": "Doubao Seedream 5.0 Lite",
            "description": "Doubao-Seedream-5.0-lite is ByteDance's latest image generation model.",
        },
        base_url="https://api.example.com/v1",
        api_config={},
        source={"effective_source": "shared"},
    )

    assert classified is not None
    assert classified["generation_mode"] == "openai_chat_image"


def test_non_image_model_is_filtered_out():
    classified = _classify_openai_image_model(
        {
            "id": "text-embedding-3-small",
            "name": "text-embedding-3-small",
        },
        base_url="https://api.openai.com/v1",
        api_config={},
        source={"effective_source": "shared"},
    )

    assert classified is None


def test_gemini_predict_imagen_model_is_detected():
    classified = _classify_gemini_image_model(
        {
            "id": "imagen-3.0-generate-002",
            "displayName": "imagen-3.0-generate-002",
            "supportedGenerationMethods": ["predict"],
        },
        source={"effective_source": "shared"},
    )

    assert classified is not None
    assert classified["generation_mode"] == "gemini_predict"


def test_gemini_generate_content_image_preview_is_detected():
    classified = _classify_gemini_image_model(
        {
            "id": "gemini-2.5-flash-image-preview",
            "displayName": "gemini-2.5-flash-image-preview",
            "supportedGenerationMethods": ["generateContent"],
        },
        source={"effective_source": "personal"},
    )

    assert classified is not None
    assert classified["generation_mode"] == "gemini_generate_content_image"


def test_gemini_camelcase_modalities_are_detected_as_image_model():
    classified = _classify_gemini_image_model(
        {
            "name": "google/gemini-2.5-flash-image",
            "displayName": "Google: Gemini 2.5 Flash Image (Nano Banana)",
            "inputModalities": ["image", "text"],
            "outputModalities": ["image", "text"],
        },
        source={"effective_source": "settings"},
    )

    assert classified is not None
    assert classified["generation_mode"] == "gemini_generate_content_image"


def test_load_b64_image_data_normalizes_data_url_mime_type():
    loaded = load_b64_image_data("data:image/png;base64,YWJj")

    assert loaded == (b"abc", "image/png")
