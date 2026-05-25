import asyncio
import base64
import json
import pathlib
import sys
from types import SimpleNamespace

import pytest


_BACKEND_DIR = pathlib.Path(__file__).resolve().parents[3]
if str(_BACKEND_DIR) not in sys.path:
    sys.path.insert(0, str(_BACKEND_DIR))

from fastapi import HTTPException  # noqa: E402

from open_webui.routers import audio as audio_router  # noqa: E402


def test_build_mimo_tts_payload_uses_assistant_message_and_wav():
    payload = audio_router._build_mimo_tts_payload(
        {"input": "Hello MiMo", "voice": "Chloe"},
        model="mimo-v2.5-tts",
        voice="mimo_default",
    )

    assert payload == {
        "model": "mimo-v2.5-tts",
        "messages": [{"role": "assistant", "content": "Hello MiMo"}],
        "audio": {"format": "wav", "voice": "Chloe"},
    }


def test_build_mimo_tts_payload_falls_back_from_non_mimo_defaults():
    payload = audio_router._build_mimo_tts_payload(
        {"input": "Hello MiMo", "voice": "alloy"},
        model="tts-1",
        voice="mimo_default",
    )

    assert payload["model"] == "mimo-v2.5-tts"
    assert payload["audio"]["voice"] == "mimo_default"


def test_extract_mimo_tts_audio_bytes_decodes_message_audio_data():
    audio_bytes = b"fake-wav-data"
    response = {
        "choices": [
            {
                "message": {
                    "audio": {
                        "data": base64.b64encode(audio_bytes).decode("utf-8"),
                    }
                }
            }
        ]
    }

    assert audio_router._extract_mimo_tts_audio_bytes(response) == audio_bytes


def test_extract_mimo_tts_audio_bytes_rejects_missing_audio_data():
    with pytest.raises(ValueError, match="audio data"):
        audio_router._extract_mimo_tts_audio_bytes({"choices": [{"message": {}}]})


def test_speech_mimo_posts_to_xiaomi_and_writes_audio(monkeypatch, tmp_path):
    captured = {}
    audio_bytes = b"RIFF....WAVEfmt fake"

    class FakeResponse:
        status = 200

        async def __aenter__(self):
            return self

        async def __aexit__(self, exc_type, exc, tb):
            return None

        def raise_for_status(self):
            return None

        async def json(self, content_type=None):
            return {
                "choices": [
                    {
                        "message": {
                            "audio": {
                                "data": base64.b64encode(audio_bytes).decode("utf-8"),
                            }
                        }
                    }
                ]
            }

    class FakeSession:
        def __init__(self, **kwargs):
            captured["session_kwargs"] = kwargs

        async def __aenter__(self):
            return self

        async def __aexit__(self, exc_type, exc, tb):
            return None

        def post(self, url, **kwargs):
            captured["url"] = url
            captured["post_kwargs"] = kwargs
            return FakeResponse()

    class FakeRequest:
        app = SimpleNamespace(
            state=SimpleNamespace(
                config=SimpleNamespace(
                    TTS_ENGINE="mimo",
                    TTS_MODEL="mimo-v2.5-tts",
                    TTS_VOICE="mimo_default",
                    TTS_API_KEY="test-key",
                )
            )
        )

        async def body(self):
            return json.dumps({"input": "Read this", "voice": "Mia"}).encode("utf-8")

    monkeypatch.setattr(audio_router, "SPEECH_CACHE_DIR", tmp_path)
    monkeypatch.setattr(audio_router.aiohttp, "ClientSession", FakeSession)

    result = asyncio.run(
        audio_router.speech(
            FakeRequest(),
            user=SimpleNamespace(
                id="user-1",
                name="Test User",
                email="user@example.com",
                role="user",
            ),
        )
    )

    assert result.media_type == "audio/wav"
    assert captured["url"] == "https://api.xiaomimimo.com/v1/chat/completions"
    assert captured["post_kwargs"]["headers"]["api-key"] == "test-key"
    assert "Authorization" not in captured["post_kwargs"]["headers"]
    assert captured["post_kwargs"]["json"] == {
        "model": "mimo-v2.5-tts",
        "messages": [{"role": "assistant", "content": "Read this"}],
        "audio": {"format": "wav", "voice": "Mia"},
    }

    output_path = pathlib.Path(result.path)
    assert output_path.suffix == ".wav"
    assert output_path.read_bytes() == audio_bytes


def test_speech_mimo_requires_api_key(tmp_path, monkeypatch):
    class FakeRequest:
        app = SimpleNamespace(
            state=SimpleNamespace(
                config=SimpleNamespace(
                    TTS_ENGINE="mimo",
                    TTS_MODEL="mimo-v2.5-tts",
                    TTS_VOICE="mimo_default",
                    TTS_API_KEY="",
                )
            )
        )

        async def body(self):
            return json.dumps({"input": "Read this"}).encode("utf-8")

    monkeypatch.setattr(audio_router, "SPEECH_CACHE_DIR", tmp_path)

    with pytest.raises(HTTPException) as exc_info:
        asyncio.run(
            audio_router.speech(
                FakeRequest(),
                user=SimpleNamespace(id="user-1", name="Test User", email="", role="user"),
            )
        )

    assert exc_info.value.status_code == 400
    assert "API key" in exc_info.value.detail
