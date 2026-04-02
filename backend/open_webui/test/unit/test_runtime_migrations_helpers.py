import pytest

from open_webui.runtime_migrations import (
    RuntimeMigrationError,
    _choose_pg_dump_binary_path,
    _extract_note_content,
    _extract_oauth_sub,
    _extract_text_content,
    _extract_usage_tokens,
    _merge_meta,
    _parse_postgres_major_version,
)


def test_extract_oauth_sub_prefers_oidc():
    oauth = {
        "github": {"sub": "gh-1"},
        "oidc": {"sub": "oidc-1"},
    }
    assert _extract_oauth_sub(oauth) == "oidc@oidc-1"


def test_extract_note_content_prefers_markdown():
    data = {"content": {"md": "hello markdown"}}
    assert _extract_note_content(data) == "hello markdown"


def test_extract_text_content_flattens_nested_blocks():
    content = [
        {"type": "text", "text": "hello"},
        {"content": {"md": "world"}},
    ]
    assert _extract_text_content(content) == "hello\nworld"


def test_extract_usage_tokens_supports_multiple_shapes():
    usage = {"input_tokens": "12", "output_tokens": 34}
    assert _extract_usage_tokens(usage) == (12, 34)


def test_merge_meta_keeps_existing_and_adds_source_payload():
    merged = _merge_meta({"foo": "bar"}, {"raw_content": {"text": "hello"}})
    assert merged["foo"] == "bar"
    assert merged["halo_migrated_from_openwebui"]["raw_content"] == {"text": "hello"}


def test_parse_postgres_major_version_handles_release_and_beta_strings():
    assert _parse_postgres_major_version("17.5 (Debian 17.5-1.pgdg12+1)") == 17
    assert _parse_postgres_major_version("18beta1") == 18
    assert _parse_postgres_major_version("unknown") is None


def test_choose_pg_dump_binary_prefers_exact_server_major():
    binary, major = _choose_pg_dump_binary_path(
        server_major=17,
        versioned_binaries={16: "/pg/16", 17: "/pg/17", 18: "/pg/18"},
        fallback_binary="/usr/bin/pg_dump",
        fallback_major=18,
    )
    assert binary == "/pg/17"
    assert major == 17


def test_choose_pg_dump_binary_uses_nearest_newer_version():
    binary, major = _choose_pg_dump_binary_path(
        server_major=17,
        versioned_binaries={16: "/pg/16", 18: "/pg/18"},
        fallback_binary="/usr/bin/pg_dump",
        fallback_major=18,
    )
    assert binary == "/pg/18"
    assert major == 18


def test_choose_pg_dump_binary_uses_compatible_fallback_when_no_versioned_binary():
    binary, major = _choose_pg_dump_binary_path(
        server_major=17,
        versioned_binaries={},
        fallback_binary="/custom/pg_dump",
        fallback_major=18,
    )
    assert binary == "/custom/pg_dump"
    assert major == 18


def test_choose_pg_dump_binary_raises_when_only_older_versions_are_available():
    with pytest.raises(RuntimeMigrationError, match="服务端主版本为 17"):
        _choose_pg_dump_binary_path(
            server_major=17,
            versioned_binaries={14: "/pg/14", 15: "/pg/15", 16: "/pg/16"},
            fallback_binary="/usr/bin/pg_dump",
            fallback_major=16,
        )
