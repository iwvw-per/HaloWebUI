import pathlib
import sys


_BACKEND_DIR = pathlib.Path(__file__).resolve().parents[3]
if str(_BACKEND_DIR) not in sys.path:
    sys.path.insert(0, str(_BACKEND_DIR))

from open_webui.routers.files import _cleanup_failed_uploaded_file  # noqa: E402
from open_webui.utils.file_upload_diagnostics import (  # noqa: E402
    classify_file_upload_error,
    is_archive_file,
)


class _AdminUser:
    role = "admin"


def test_is_archive_file_detects_common_archive_types():
    assert is_archive_file("demo.rar", "application/octet-stream") is True
    assert is_archive_file("demo.txt", "application/x-rar-compressed") is True
    assert is_archive_file("demo.txt", "text/plain") is False


def test_classify_file_upload_error_returns_archive_diagnostic():
    diagnostic = classify_file_upload_error(
        None,
        filename="SHUN-logo-pack.rar",
        content_type="application/vnd.rar",
    )

    assert diagnostic["code"] == "unsupported_archive"
    assert diagnostic["blocking"] is True


def test_classify_file_upload_error_handles_missing_chardet_dependency():
    diagnostic = classify_file_upload_error(
        RuntimeError("No module named 'chardet'"),
        filename="notes.txt",
        content_type="text/plain",
    )

    assert diagnostic["code"] == "missing_text_encoding_dependency"


def test_classify_file_upload_error_handles_embedding_auth_failure():
    diagnostic = classify_file_upload_error(
        RuntimeError(
            "Embedding generation failed for batch starting at index 0: 401 Unauthorized"
        ),
        filename="report.html",
        content_type="text/html",
        user=_AdminUser(),
    )

    assert diagnostic["code"] == "embedding_provider_unauthorized"
    assert "/settings/documents" in diagnostic["hint"]


def test_classify_file_upload_error_handles_embedding_connection_failure():
    diagnostic = classify_file_upload_error(
        RuntimeError(
            "Embedding generation failed for batch starting at index 0: "
            "Failed to establish a new connection"
        ),
        filename="report.html",
        content_type="text/html",
    )

    assert diagnostic["code"] == "embedding_provider_unreachable"


def test_classify_file_upload_error_handles_embedding_timeout():
    diagnostic = classify_file_upload_error(
        RuntimeError(
            "Embedding generation failed for batch starting at index 0: read timed out"
        ),
        filename="report.html",
        content_type="text/html",
    )

    assert diagnostic["code"] == "embedding_provider_unreachable"


def test_cleanup_failed_uploaded_file_removes_collection_record_and_storage(monkeypatch):
    events: list[tuple[str, str]] = []

    monkeypatch.setattr(
        "open_webui.routers.files.VECTOR_DB_CLIENT.has_collection",
        lambda collection_name: collection_name == "file-file-123",
    )
    monkeypatch.setattr(
        "open_webui.routers.files.VECTOR_DB_CLIENT.delete_collection",
        lambda collection_name: events.append(("collection", collection_name)),
    )
    monkeypatch.setattr(
        "open_webui.routers.files.Files.delete_file_by_id",
        lambda file_id: events.append(("record", file_id)),
    )
    monkeypatch.setattr(
        "open_webui.routers.files.Storage.delete_file",
        lambda file_path: events.append(("storage", file_path)),
    )

    _cleanup_failed_uploaded_file("file-123", "/tmp/file-123_demo.txt")

    assert ("collection", "file-file-123") in events
    assert ("record", "file-123") in events
    assert ("storage", "/tmp/file-123_demo.txt") in events
