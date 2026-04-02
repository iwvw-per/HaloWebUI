import json
import re
from functools import lru_cache
from pathlib import Path
from typing import Optional
from urllib.parse import urlparse


NATIVE_WEB_SEARCH_STATUS_SUPPORTED = "supported"
NATIVE_WEB_SEARCH_STATUS_UNKNOWN = "unknown"
NATIVE_WEB_SEARCH_STATUS_UNSUPPORTED = "unsupported"
NATIVE_WEB_SEARCH_EFFECTIVE_SCOPE = "connection+model"

RULE_MATCH_REGEX = "regex"
RULE_MATCH_CONTAINS = "contains"
RULE_MATCH_EQUALS = "equals"
RULE_MATCH_PREFIX = "prefix"

PROJECT_ROOT = Path(__file__).resolve().parents[3]
NATIVE_WEB_SEARCH_RULES_PATH = (
    PROJECT_ROOT / "src" / "lib" / "data" / "native-web-search-rules.json"
)


def _get_hostname(value: str) -> str:
    try:
        return (urlparse(value).hostname or "").strip().lower()
    except Exception:
        return ""


def is_official_openai_connection(url: str) -> bool:
    host = _get_hostname(url)
    return host == "api.openai.com" or host.endswith(".openai.com")


def is_official_gemini_connection(url: str) -> bool:
    host = _get_hostname(url)
    return host == "generativelanguage.googleapis.com"


def _normalize_rule_provider(provider: str) -> str:
    normalized = str(provider or "").strip().lower()
    if normalized in {"google", "gemini"}:
        return "gemini"
    return normalized or "unknown"


def _normalize_model_lookup_value(value: str) -> str:
    normalized = str(value or "").strip().lower()
    if not normalized:
        return ""
    if normalized.startswith("models/"):
        normalized = normalized[len("models/") :]
    return normalized


def strip_model_prefix(model_id: str, prefix_id: Optional[str]) -> str:
    normalized_id = str(model_id or "").strip()
    prefix = str(prefix_id or "").strip()
    if prefix and normalized_id.startswith(f"{prefix}."):
        return normalized_id[len(prefix) + 1 :]
    return normalized_id


@lru_cache(maxsize=1)
def load_native_web_search_rules() -> dict:
    try:
        with NATIVE_WEB_SEARCH_RULES_PATH.open("r", encoding="utf-8") as file:
            data = json.load(file)
        if isinstance(data, dict):
            return data
    except Exception:
        pass
    return {"providers": {}}


def _get_provider_rules(provider: str) -> dict:
    rules = load_native_web_search_rules()
    providers = rules.get("providers", {})
    if not isinstance(providers, dict):
        return {}
    value = providers.get(_normalize_rule_provider(provider), {})
    return value if isinstance(value, dict) else {}


def _match_rule(rule: dict, candidate: str) -> bool:
    if not candidate or not isinstance(rule, dict):
        return False

    match_type = str(rule.get("type") or "").strip().lower()
    rule_value = str(rule.get("value") or "").strip()
    if not match_type or not rule_value:
        return False

    if match_type == RULE_MATCH_REGEX:
        return re.search(rule_value, candidate, flags=re.IGNORECASE) is not None
    if match_type == RULE_MATCH_CONTAINS:
        return rule_value.lower() in candidate
    if match_type == RULE_MATCH_EQUALS:
        return candidate == rule_value.lower()
    if match_type == RULE_MATCH_PREFIX:
        return candidate.startswith(rule_value.lower())
    return False


def resolve_model_native_web_search_rule(
    provider: str,
    *,
    model_id: str = "",
    model_name: str = "",
) -> dict:
    normalized_provider = _normalize_rule_provider(provider)
    provider_rules = _get_provider_rules(normalized_provider)

    values = []
    for raw in (model_id, model_name):
        normalized = _normalize_model_lookup_value(raw)
        if normalized and normalized not in values:
            values.append(normalized)

    if not provider_rules:
        return {
            "provider": normalized_provider,
            "status": NATIVE_WEB_SEARCH_STATUS_UNKNOWN,
            "reason": "model_rule_unknown",
            "source": "model_rules",
        }

    for group_name, status in (
        ("deny", NATIVE_WEB_SEARCH_STATUS_UNSUPPORTED),
        ("allow", NATIVE_WEB_SEARCH_STATUS_SUPPORTED),
    ):
        group = provider_rules.get(group_name, [])
        if not isinstance(group, list):
            continue
        for rule in group:
            if not isinstance(rule, dict):
                continue
            for candidate in values:
                if _match_rule(rule, candidate):
                    return {
                        "provider": normalized_provider,
                        "status": status,
                        "reason": str(rule.get("reason") or "model_rule_unknown"),
                        "source": "model_rules",
                        "match_type": rule.get("type"),
                        "match_value": rule.get("value"),
                        "matched_on": candidate,
                    }

    default_status = str(provider_rules.get("default_status") or "").strip().lower()
    if default_status not in {
        NATIVE_WEB_SEARCH_STATUS_SUPPORTED,
        NATIVE_WEB_SEARCH_STATUS_UNKNOWN,
        NATIVE_WEB_SEARCH_STATUS_UNSUPPORTED,
    }:
        default_status = NATIVE_WEB_SEARCH_STATUS_UNKNOWN

    return {
        "provider": normalized_provider,
        "status": default_status,
        "reason": "model_rule_unknown",
        "source": "model_rules",
    }


def resolve_effective_native_web_search_support(
    connection_support: dict,
    *,
    provider: str,
    model_id: str = "",
    model_name: str = "",
) -> dict:
    connection = dict(connection_support or {})
    normalized_provider = _normalize_rule_provider(
        connection.get("provider") or provider
    )
    model_rule = resolve_model_native_web_search_rule(
        normalized_provider, model_id=model_id, model_name=model_name
    )

    connection_status = connection.get("status") or NATIVE_WEB_SEARCH_STATUS_UNKNOWN
    model_status = model_rule.get("status") or NATIVE_WEB_SEARCH_STATUS_UNKNOWN

    if connection_status == NATIVE_WEB_SEARCH_STATUS_UNSUPPORTED:
        effective_status = NATIVE_WEB_SEARCH_STATUS_UNSUPPORTED
        reason = connection.get("reason") or "connection_not_found"
        source = connection.get("source") or "connection_support"
    elif model_status == NATIVE_WEB_SEARCH_STATUS_UNSUPPORTED:
        effective_status = NATIVE_WEB_SEARCH_STATUS_UNSUPPORTED
        reason = model_rule.get("reason") or "model_rule_unknown"
        source = model_rule.get("source") or "model_rules"
    elif (
        connection_status == NATIVE_WEB_SEARCH_STATUS_SUPPORTED
        and model_status == NATIVE_WEB_SEARCH_STATUS_SUPPORTED
    ):
        effective_status = NATIVE_WEB_SEARCH_STATUS_SUPPORTED
        reason = model_rule.get("reason") or connection.get("reason") or "official_connection"
        source = NATIVE_WEB_SEARCH_EFFECTIVE_SCOPE
    else:
        effective_status = NATIVE_WEB_SEARCH_STATUS_UNKNOWN
        reason = model_rule.get("reason") or connection.get("reason") or "model_rule_unknown"
        source = NATIVE_WEB_SEARCH_EFFECTIVE_SCOPE

    can_attempt = (
        effective_status in {
            NATIVE_WEB_SEARCH_STATUS_SUPPORTED,
            NATIVE_WEB_SEARCH_STATUS_UNKNOWN,
        }
        and connection_status != NATIVE_WEB_SEARCH_STATUS_UNSUPPORTED
        and model_status != NATIVE_WEB_SEARCH_STATUS_UNSUPPORTED
    )

    return {
        **{
            k: v
            for k, v in connection.items()
            if k not in {"supported", "can_attempt"}
        },
        "provider": normalized_provider,
        "status": effective_status,
        "reason": reason,
        "source": source,
        "supported": effective_status == NATIVE_WEB_SEARCH_STATUS_SUPPORTED,
        "can_attempt": can_attempt,
        "connection_support": {
            k: v
            for k, v in connection.items()
            if k not in {"url", "api_config", "connection_support", "model_rule"}
        },
        "model_rule": model_rule,
        "effective_scope": NATIVE_WEB_SEARCH_EFFECTIVE_SCOPE,
    }


def build_native_web_search_support(
    provider: str,
    *,
    url: str = "",
    api_config: Optional[dict] = None,
    connection_name: Optional[str] = None,
) -> dict:
    normalized_provider = str(provider or "").strip().lower()
    config = api_config if isinstance(api_config, dict) else {}
    explicit = config.get("native_web_search_enabled")
    configured = explicit if isinstance(explicit, bool) else None

    if normalized_provider == "openai":
        official = is_official_openai_connection(url)
        if configured is True:
            status = NATIVE_WEB_SEARCH_STATUS_SUPPORTED
            reason = "connection_enabled"
            source = "connection_config"
        elif configured is False:
            status = NATIVE_WEB_SEARCH_STATUS_UNSUPPORTED
            reason = "connection_disabled"
            source = "connection_config"
        elif official:
            status = NATIVE_WEB_SEARCH_STATUS_SUPPORTED
            reason = "official_connection"
            source = "official_connection"
        else:
            status = NATIVE_WEB_SEARCH_STATUS_UNKNOWN
            reason = "compat_connection_unverified"
            source = "connection_inference"

        return {
            "provider": normalized_provider,
            "status": status,
            "reason": reason,
            "source": source,
            "official": official,
            "configured": configured,
            "supported": status == NATIVE_WEB_SEARCH_STATUS_SUPPORTED,
            "can_attempt": status
            in {
                NATIVE_WEB_SEARCH_STATUS_SUPPORTED,
                NATIVE_WEB_SEARCH_STATUS_UNKNOWN,
            },
            **({"connection_name": connection_name} if connection_name else {}),
        }

    if normalized_provider in {"google", "gemini"}:
        official = is_official_gemini_connection(url)
        if configured is True:
            status = NATIVE_WEB_SEARCH_STATUS_SUPPORTED
            reason = "connection_enabled"
            source = "connection_config"
        elif configured is False:
            status = NATIVE_WEB_SEARCH_STATUS_UNSUPPORTED
            reason = "connection_disabled"
            source = "connection_config"
        elif official:
            status = NATIVE_WEB_SEARCH_STATUS_SUPPORTED
            reason = "official_connection"
            source = "official_connection"
        else:
            status = NATIVE_WEB_SEARCH_STATUS_UNKNOWN
            reason = "compat_connection_unverified"
            source = "connection_inference"

        return {
            "provider": "gemini",
            "status": status,
            "reason": reason,
            "source": source,
            "official": official,
            "configured": configured,
            "supported": status == NATIVE_WEB_SEARCH_STATUS_SUPPORTED,
            "can_attempt": status
            in {
                NATIVE_WEB_SEARCH_STATUS_SUPPORTED,
                NATIVE_WEB_SEARCH_STATUS_UNKNOWN,
            },
            **({"connection_name": connection_name} if connection_name else {}),
        }

    return {
        "provider": normalized_provider or "unknown",
        "status": NATIVE_WEB_SEARCH_STATUS_UNSUPPORTED,
        "reason": "provider_not_supported",
        "source": "provider_capability",
        "official": False,
        "configured": configured,
        "supported": False,
        "can_attempt": False,
        **({"connection_name": connection_name} if connection_name else {}),
    }
