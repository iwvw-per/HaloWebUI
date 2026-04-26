import pathlib
import sys


_BACKEND_DIR = pathlib.Path(__file__).resolve().parents[3]
if str(_BACKEND_DIR) not in sys.path:
    sys.path.insert(0, str(_BACKEND_DIR))

from open_webui.routers.images import (  # noqa: E402
    _classify_gemini_image_model,
    _classify_grok_image_model,
    _classify_openai_image_model,
    _extract_generated_images_from_openai_response,
    load_b64_image_data,
)


def test_openai_dedicated_image_model_uses_images_mode_for_relays():
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
    assert classified["generation_mode"] == "openai_images"
    assert classified["text_output_supported"] is False
    assert classified["detection_method"] == "metadata"


def test_openai_output_image_chat_model_uses_chat_image_mode_for_relays():
    classified = _classify_openai_image_model(
        {
            "id": "relay-image-preview",
            "name": "Relay Image Preview",
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


def test_official_xai_openai_compat_models_use_xai_images_mode():
    classified = _classify_openai_image_model(
        {
            "id": "grok-imagine-image",
            "name": "Grok Imagine Image",
        },
        base_url="https://api.x.ai/v1",
        api_config={},
        source={"effective_source": "shared", "provider": "openai"},
    )

    assert classified is not None
    assert classified["generation_mode"] == "xai_images"
    assert classified["size_mode"] == "aspect_ratio"
    assert classified["supports_resolution"] is True


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
    assert classified["generation_mode"] == "openai_images"


def test_openai_flux_name_is_detected_as_dedicated_image_model():
    classified = _classify_openai_image_model(
        {
            "id": "black-forest-labs/flux-kontext-pro",
            "name": "FLUX Kontext Pro",
        },
        base_url="https://api.example.com/v1",
        api_config={},
        source={"effective_source": "shared"},
    )

    assert classified is not None
    assert classified["generation_mode"] == "openai_images"


def test_volcengine_seedream_prefers_images_endpoint_mode():
    classified = _classify_openai_image_model(
        {
            "id": "doubao-seedream-4-5-251128",
            "name": "Doubao Seedream 4.5",
        },
        base_url="https://ark.cn-beijing.volces.com/api/v3",
        api_config={},
        source={"effective_source": "personal"},
    )

    assert classified is not None
    assert classified["generation_mode"] == "openai_images"
    assert classified["detection_method"] == "heuristic"


def test_volcengine_seededit_prefers_images_endpoint_mode():
    classified = _classify_openai_image_model(
        {
            "id": "doubao-seededit-3-0-i2i-250628",
            "name": "Doubao SeedEdit 3.0",
        },
        base_url="https://ark.cn-beijing.volces.com/api/v3",
        api_config={},
        source={"effective_source": "personal"},
    )

    assert classified is not None
    assert classified["generation_mode"] == "openai_images"
    assert classified["detection_method"] == "heuristic"


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


def test_grok_image_model_is_detected():
    classified = _classify_grok_image_model(
        {
            "id": "grok-imagine-image",
            "name": "Grok Imagine Image",
        },
        source={"effective_source": "personal", "provider": "grok"},
    )

    assert classified is not None
    assert classified["generation_mode"] == "xai_images"
    assert classified["supports_resolution"] is True


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


def test_openai_response_extracts_markdown_embedded_image_data():
    images = _extract_generated_images_from_openai_response(
        {
            "choices": [
                {
                    "message": {
                        "content": "Here is your image: ![Generated](data:image/png;base64,YWJj)"
                    }
                }
            ]
        }
    )

    assert images == [(b"abc", "image/png")]


def test_openai_response_extracts_stream_delta_images():
    images = _extract_generated_images_from_openai_response(
        {
            "choices": [
                {
                    "delta": {
                        "images": [
                            {
                                "type": "image_url",
                                "image_url": {"url": "data:image/png;base64,YWJj"},
                            }
                        ]
                    }
                }
            ]
        }
    )

    assert images == [(b"abc", "image/png")]


def test_openai_response_extracts_top_level_message_images():
    images = _extract_generated_images_from_openai_response(
        {
            "message": {
                "images": [
                    {
                        "type": "image_url",
                        "image_url": {"url": "data:image/png;base64,YWJj"},
                    }
                ]
            }
        }
    )

    assert images == [(b"abc", "image/png")]


def test_openai_response_extracts_file_image_event():
    images = _extract_generated_images_from_openai_response(
        {
            "type": "file",
            "file": {
                "mediaType": "image/jpeg",
                "base64": "YWJj",
            },
        }
    )

    assert images == [(b"abc", "image/jpeg")]
