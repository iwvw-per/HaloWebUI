from __future__ import annotations

from dataclasses import dataclass
import hashlib
import json
import random
import threading
from typing import Any, Optional


API_KEY_POOL_CONFIG_KEY = "api_key_pool"
API_KEY_POOL_MODES = {"round_robin", "random", "priority"}
DEFAULT_API_KEY_POOL_MODE = "round_robin"
DEFAULT_RETRY_PRESET = "rate_limit_transient"
DEFAULT_RETRY_STATUS_CODES = [429, 500, 502, 503, 504]
DEFAULT_RETRY_ERROR_KEYWORDS = [
    "rate limit",
    "rate_limit",
    "too many requests",
    "quota",
    "insufficient_quota",
    "over quota",
    "temporarily unavailable",
    "timeout",
    "timed out",
    "overloaded",
    "server error",
    "internal server error",
    "bad gateway",
    "service unavailable",
    "gateway timeout",
    "限流",
    "额度",
    "配额",
    "超时",
    "暂时不可用",
    "服务繁忙",
]

_COUNTER_LOCK = threading.Lock()
_ROUND_ROBIN_COUNTERS: dict[str, int] = {}


@dataclass(frozen=True)
class ApiKeyAttempt:
    key: str
    id: str
    label: str
    index: int
    attempt: int
    total: int

    @property
    def safe_label(self) -> str:
        label = (self.label or "").strip()
        if label:
            return label
        return f"Key {self.index + 1}"


def _clean_str(value: Any) -> str:
    return str(value or "").strip()


def _as_dict(value: Any) -> dict:
    return value if isinstance(value, dict) else {}


def _coerce_bool(value: Any, default: bool = False) -> bool:
    if isinstance(value, bool):
        return value
    if value is None:
        return default
    if isinstance(value, (int, float)):
        return bool(value)
    if isinstance(value, str):
        normalized = value.strip().lower()
        if normalized in {"1", "true", "yes", "on"}:
            return True
        if normalized in {"0", "false", "no", "off"}:
            return False
    return default


def _stable_key_id(*, provider: str, connection_key: str, key: str, label: str, idx: int) -> str:
    seed = json.dumps(
        {
            "provider": provider or "",
            "connection": connection_key or "",
            "key": key or "",
            "label": label or "",
            "idx": idx,
        },
        sort_keys=True,
        ensure_ascii=False,
    )
    return f"k_{hashlib.sha256(seed.encode('utf-8')).hexdigest()[:12]}"


def _normalize_status_codes(value: Any) -> list[int]:
    values = value if isinstance(value, list) else DEFAULT_RETRY_STATUS_CODES
    result: list[int] = []
    for raw in values:
        try:
            code = int(raw)
        except Exception:
            continue
        if 100 <= code <= 599 and code not in result:
            result.append(code)
    return result or list(DEFAULT_RETRY_STATUS_CODES)


def _normalize_error_keywords(value: Any) -> list[str]:
    values = value if isinstance(value, list) else DEFAULT_RETRY_ERROR_KEYWORDS
    result: list[str] = []
    for raw in values:
        keyword = _clean_str(raw)
        if keyword and keyword not in result:
            result.append(keyword)
    return result or list(DEFAULT_RETRY_ERROR_KEYWORDS)


def normalize_api_key_pool_config(
    api_config: Optional[dict],
    fallback_key: Any = "",
    *,
    provider: str = "",
    connection_key: str = "",
) -> tuple[dict, str]:
    """Return a config with a normalized api_key_pool and its primary enabled key."""

    cfg = dict(api_config or {})
    fallback_key = _clean_str(fallback_key)
    raw_pool = _as_dict(cfg.get(API_KEY_POOL_CONFIG_KEY))
    raw_keys = raw_pool.get("keys") if isinstance(raw_pool.get("keys"), list) else []

    normalized_keys: list[dict] = []
    used_ids: set[str] = set()

    for idx, raw_entry in enumerate(raw_keys):
        if isinstance(raw_entry, dict):
            key = _clean_str(raw_entry.get("key"))
            label = _clean_str(raw_entry.get("label")) or f"Key {idx + 1}"
            enabled = _coerce_bool(raw_entry.get("enabled"), True)
            entry_id = _clean_str(raw_entry.get("id"))
        else:
            key = _clean_str(raw_entry)
            label = f"Key {idx + 1}"
            enabled = True
            entry_id = ""

        if not key:
            continue

        if not entry_id:
            entry_id = _stable_key_id(
                provider=provider,
                connection_key=connection_key,
                key=key,
                label=label,
                idx=idx,
            )

        base_id = entry_id
        suffix = 2
        while entry_id in used_ids:
            entry_id = f"{base_id}_{suffix}"
            suffix += 1
        used_ids.add(entry_id)

        normalized_keys.append(
            {
                "id": entry_id,
                "label": label,
                "key": key,
                "enabled": enabled,
            }
        )

    if not normalized_keys and fallback_key:
        normalized_keys.append(
            {
                "id": _stable_key_id(
                    provider=provider,
                    connection_key=connection_key,
                    key=fallback_key,
                    label="Key 1",
                    idx=0,
                ),
                "label": "Key 1",
                "key": fallback_key,
                "enabled": True,
            }
        )

    mode = _clean_str(raw_pool.get("mode")).lower()
    if mode not in API_KEY_POOL_MODES:
        mode = DEFAULT_API_KEY_POOL_MODE

    raw_retry = _as_dict(raw_pool.get("retry"))
    retry = {
        "enabled": _coerce_bool(raw_retry.get("enabled"), True),
        "preset": _clean_str(raw_retry.get("preset")) or DEFAULT_RETRY_PRESET,
        "status_codes": _normalize_status_codes(raw_retry.get("status_codes")),
        "error_keywords": _normalize_error_keywords(raw_retry.get("error_keywords")),
    }

    cfg[API_KEY_POOL_CONFIG_KEY] = {
        "keys": normalized_keys,
        "mode": mode,
        "retry": retry,
    }

    primary_key = ""
    for entry in normalized_keys:
        if entry.get("enabled", True):
            primary_key = _clean_str(entry.get("key"))
            if primary_key:
                break

    return cfg, primary_key


def normalize_indexed_api_key_pools(
    *,
    provider: str,
    urls: list[str],
    keys: list[str],
    configs: dict,
) -> tuple[list[str], dict]:
    normalized_keys = list(keys)
    if len(normalized_keys) < len(urls):
        normalized_keys += [""] * (len(urls) - len(normalized_keys))
    elif len(normalized_keys) > len(urls):
        normalized_keys = normalized_keys[: len(urls)]

    normalized_configs: dict[str, dict] = {}
    for idx, url in enumerate(urls):
        key = str(idx)
        cfg = _as_dict(configs.get(key, configs.get(url, {})))
        prefix_id = _clean_str(cfg.get("prefix_id") or cfg.get("_resolved_prefix_id"))
        connection_key = prefix_id or f"idx:{idx}:{url}"
        normalized_cfg, primary_key = normalize_api_key_pool_config(
            cfg,
            normalized_keys[idx] if idx < len(normalized_keys) else "",
            provider=provider,
            connection_key=connection_key,
        )
        normalized_configs[key] = normalized_cfg
        if idx < len(normalized_keys):
            normalized_keys[idx] = primary_key

    return normalized_keys, normalized_configs


def _enabled_entries(api_config: Optional[dict], fallback_key: Any = "") -> list[dict]:
    cfg, primary = normalize_api_key_pool_config(api_config, fallback_key)
    pool = _as_dict(cfg.get(API_KEY_POOL_CONFIG_KEY))
    entries = [
        entry
        for entry in pool.get("keys", [])
        if isinstance(entry, dict)
        and _coerce_bool(entry.get("enabled"), True)
        and _clean_str(entry.get("key"))
    ]
    if not entries and primary:
        entries = [{"id": "legacy", "label": "Key 1", "key": primary, "enabled": True}]
    return entries


def get_api_key_attempts(
    *,
    provider: str,
    connection_key: str,
    api_config: Optional[dict],
    fallback_key: Any = "",
    include_retry: bool = True,
) -> list[ApiKeyAttempt]:
    entries = _enabled_entries(api_config, fallback_key)
    if not entries:
        return [
            ApiKeyAttempt(
                key="",
                id="no-key",
                label="No API key",
                index=0,
                attempt=1,
                total=1,
            )
        ]

    cfg = _as_dict(api_config)
    pool = _as_dict(cfg.get(API_KEY_POOL_CONFIG_KEY))
    mode = _clean_str(pool.get("mode")).lower() or DEFAULT_API_KEY_POOL_MODE
    if mode not in API_KEY_POOL_MODES:
        mode = DEFAULT_API_KEY_POOL_MODE

    ordered = list(entries)
    counter_key = f"{provider}:{connection_key or _clean_str(cfg.get('prefix_id') or cfg.get('_resolved_prefix_id'))}"

    if len(ordered) > 1 and mode == "round_robin":
        with _COUNTER_LOCK:
            start = _ROUND_ROBIN_COUNTERS.get(counter_key, 0) % len(ordered)
            _ROUND_ROBIN_COUNTERS[counter_key] = (start + 1) % len(ordered)
        ordered = ordered[start:] + ordered[:start]
    elif len(ordered) > 1 and mode == "random":
        random.shuffle(ordered)

    if not include_retry:
        ordered = ordered[:1]
    else:
        retry = _as_dict(pool.get("retry"))
        if not _coerce_bool(retry.get("enabled"), True):
            ordered = ordered[:1]

    total = len(ordered)
    attempts: list[ApiKeyAttempt] = []
    for attempt_idx, entry in enumerate(ordered):
        attempts.append(
            ApiKeyAttempt(
                key=_clean_str(entry.get("key")),
                id=_clean_str(entry.get("id")) or f"key-{attempt_idx + 1}",
                label=_clean_str(entry.get("label")) or f"Key {attempt_idx + 1}",
                index=attempt_idx,
                attempt=attempt_idx + 1,
                total=total,
            )
        )
    return attempts


def get_first_api_key(
    *,
    provider: str,
    connection_key: str,
    api_config: Optional[dict],
    fallback_key: Any = "",
) -> str:
    attempts = get_api_key_attempts(
        provider=provider,
        connection_key=connection_key,
        api_config=api_config,
        fallback_key=fallback_key,
        include_retry=False,
    )
    return attempts[0].key if attempts else _clean_str(fallback_key)


def _stringify_error_body(body: Any) -> str:
    if body is None:
        return ""
    if isinstance(body, str):
        return body
    try:
        return json.dumps(body, ensure_ascii=False, default=str)
    except Exception:
        return str(body)


def should_retry_api_key(
    api_config: Optional[dict],
    *,
    status_code: Optional[int] = None,
    body: Any = None,
    exception: Optional[BaseException] = None,
) -> bool:
    cfg, _primary = normalize_api_key_pool_config(api_config, "")
    retry = _as_dict(_as_dict(cfg.get(API_KEY_POOL_CONFIG_KEY)).get("retry"))
    if not _coerce_bool(retry.get("enabled"), True):
        return False

    status_codes = set(_normalize_status_codes(retry.get("status_codes")))
    if status_code is not None:
        try:
            status = int(status_code)
        except Exception:
            status = None
        if status is not None and status in status_codes:
            return True
        # Do not hide common configuration errors with keyword matching.
        if status in {400, 401, 403, 404}:
            return False

    if exception is not None:
        name = type(exception).__name__.lower()
        text = str(exception).lower()
        if any(token in name or token in text for token in ("timeout", "client", "connect", "network")):
            return True

    text = _stringify_error_body(body).lower()
    if not text:
        return False

    for keyword in _normalize_error_keywords(retry.get("error_keywords")):
        if keyword.lower() in text:
            return True

    return False


def describe_api_key_attempt(attempt: ApiKeyAttempt) -> dict:
    return {
        "id": attempt.id,
        "label": attempt.safe_label,
        "attempt": attempt.attempt,
        "total": attempt.total,
    }
