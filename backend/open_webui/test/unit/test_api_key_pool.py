from open_webui.utils import api_key_pool
from open_webui.utils.api_key_pool import (
    normalize_api_key_pool_config,
    normalize_indexed_api_key_pools,
    get_api_key_attempts,
    should_retry_api_key,
)


def test_legacy_single_key_normalizes_to_single_key_pool():
    cfg, primary = normalize_api_key_pool_config(
        {"remark": "Relay"},
        "sk-legacy",
        provider="openai",
        connection_key="relay",
    )

    assert primary == "sk-legacy"
    pool = cfg["api_key_pool"]
    assert pool["mode"] == "round_robin"
    assert pool["retry"]["enabled"] is True
    assert pool["retry"]["status_codes"] == [429, 500, 502, 503, 504]
    assert pool["keys"] == [
        {
            "id": pool["keys"][0]["id"],
            "label": "Key 1",
            "key": "sk-legacy",
            "enabled": True,
        }
    ]


def test_indexed_key_pool_mirrors_first_enabled_key_to_legacy_keys():
    keys, configs = normalize_indexed_api_key_pools(
        provider="openai",
        urls=["https://relay.example.com/v1"],
        keys=["sk-old"],
        configs={
            "0": {
                "prefix_id": "relay",
                "api_key_pool": {
                    "keys": [
                        {"id": "a", "label": "A", "key": "sk-a", "enabled": False},
                        {"id": "b", "label": "B", "key": "sk-b", "enabled": True},
                    ],
                    "mode": "priority",
                    "retry": {"enabled": True},
                },
            }
        },
    )

    assert keys == ["sk-b"]
    assert configs["0"]["api_key_pool"]["keys"][0]["key"] == "sk-a"
    assert configs["0"]["api_key_pool"]["keys"][1]["key"] == "sk-b"


def test_disabled_api_key_pool_does_not_fall_back_to_disabled_or_legacy_key():
    config = {
        "api_key_pool": {
            "keys": [
                {"id": "a", "label": "A", "key": "sk-disabled", "enabled": False},
            ],
            "mode": "priority",
            "retry": {"enabled": True},
        }
    }
    cfg, primary = normalize_api_key_pool_config(
        config,
        "sk-legacy",
        provider="openai",
        connection_key="disabled-test",
    )

    assert primary == ""
    attempts = get_api_key_attempts(
        provider="openai",
        connection_key="disabled-test",
        api_config=cfg,
        fallback_key="sk-legacy",
    )
    assert [attempt.key for attempt in attempts] == [""]

    keys, _configs = normalize_indexed_api_key_pools(
        provider="openai",
        urls=["https://relay.example.com/v1"],
        keys=["sk-legacy"],
        configs={"0": config},
    )
    assert keys == [""]


def test_round_robin_returns_next_enabled_key_per_request():
    config = {
        "api_key_pool": {
            "keys": [
                {"id": "a", "label": "A", "key": "sk-a", "enabled": True},
                {"id": "b", "label": "B", "key": "sk-b", "enabled": True},
            ],
            "mode": "round_robin",
            "retry": {"enabled": True},
        }
    }
    connection_key = "round-robin-test"
    api_key_pool._ROUND_ROBIN_COUNTERS.pop(f"openai:{connection_key}", None)

    first = get_api_key_attempts(
        provider="openai",
        connection_key=connection_key,
        api_config=config,
        include_retry=False,
    )
    second = get_api_key_attempts(
        provider="openai",
        connection_key=connection_key,
        api_config=config,
        include_retry=False,
    )

    assert [attempt.key for attempt in first] == ["sk-a"]
    assert [attempt.key for attempt in second] == ["sk-b"]


def test_random_and_priority_modes_preserve_expected_attempt_order(monkeypatch):
    config = {
        "api_key_pool": {
            "keys": [
                {"id": "a", "label": "A", "key": "sk-a", "enabled": True},
                {"id": "b", "label": "B", "key": "sk-b", "enabled": True},
            ],
            "mode": "random",
            "retry": {"enabled": True},
        }
    }
    monkeypatch.setattr(api_key_pool.random, "shuffle", lambda values: values.reverse())

    attempts = get_api_key_attempts(
        provider="gemini",
        connection_key="random-test",
        api_config=config,
    )
    assert [attempt.key for attempt in attempts] == ["sk-b", "sk-a"]

    config["api_key_pool"]["mode"] = "priority"
    attempts = get_api_key_attempts(
        provider="gemini",
        connection_key="priority-test",
        api_config=config,
    )
    assert [attempt.key for attempt in attempts] == ["sk-a", "sk-b"]


def test_default_retry_rules_are_transient_only():
    config = {
        "api_key_pool": {
            "keys": [{"id": "a", "label": "A", "key": "sk-a", "enabled": True}],
            "retry": {"enabled": True},
        }
    }

    assert should_retry_api_key(config, status_code=429, body="rate limit") is True
    assert should_retry_api_key(config, status_code=503, body="service unavailable") is True
    assert should_retry_api_key(config, body={"error": {"message": "quota exceeded"}}) is True
    assert should_retry_api_key(config, exception=TimeoutError("timed out")) is True

    assert should_retry_api_key(config, status_code=400, body="rate limit") is False
    assert should_retry_api_key(config, status_code=401, body="rate limit") is False
    assert should_retry_api_key(config, status_code=403, body="quota exceeded") is False
    assert should_retry_api_key(config, status_code=404, body="timeout") is False
