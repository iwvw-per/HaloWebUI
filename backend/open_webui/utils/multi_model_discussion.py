import asyncio
import json
import logging
import time
from copy import deepcopy
from typing import Any

from fastapi import HTTPException, Request, status
from starlette.responses import Response

from open_webui.env import BYPASS_MODEL_ACCESS_CONTROL, GLOBAL_LOG_LEVEL, SRC_LOG_LEVELS
from open_webui.models.chats import Chats
from open_webui.socket.main import get_event_emitter
from open_webui.tasks import create_task
from open_webui.utils.chat import generate_chat_completion
from open_webui.utils.model_identity import resolve_model_from_lookup
from open_webui.utils.models import check_model_access


log = logging.getLogger(__name__)
log.setLevel(SRC_LOG_LEVELS.get("MAIN", GLOBAL_LOG_LEVEL))

MAX_DISCUSSION_MODELS = 5
MAX_DISCUSSION_ROUNDS = 5
DEFAULT_DISCUSSION_ROUNDS = 2

USAGE_NUMBER_KEYS = (
    "prompt_tokens",
    "completion_tokens",
    "total_tokens",
    "prompt_eval_count",
    "eval_count",
    "prompt_eval_duration",
    "eval_duration",
    "total_duration",
    "load_duration",
)


def is_multi_model_discussion_enabled(discussion: Any) -> bool:
    return isinstance(discussion, dict) and discussion.get("enabled") is True


def _model_display_name(model: dict | None, fallback: str = "") -> str:
    if not isinstance(model, dict):
        return fallback
    return str(
        model.get("name")
        or model.get("model")
        or model.get("id")
        or fallback
    )


def _model_request_id(model: dict | None, fallback: str = "") -> str:
    if not isinstance(model, dict):
        return fallback
    return str(
        model.get("selection_id")
        or model.get("id")
        or model.get("model_id")
        or fallback
    ).strip()


def _extract_response_payload(response: Any) -> dict | None:
    if isinstance(response, dict):
        return response

    if isinstance(response, Response):
        body = getattr(response, "body", None)
        if body is None:
            return None
        if isinstance(body, memoryview):
            body = body.tobytes()
        if isinstance(body, bytes):
            raw = body.decode("utf-8", errors="replace")
        else:
            raw = str(body)
        if not raw:
            return None
        try:
            parsed = json.loads(raw)
        except Exception:
            return {"detail": raw}
        return parsed if isinstance(parsed, dict) else None

    return None


def extract_generation_content(response: Any) -> str:
    payload = _extract_response_payload(response)
    if not isinstance(payload, dict):
        return ""

    error = payload.get("error")
    if isinstance(error, dict):
        message = error.get("message") or error.get("detail") or error.get("content")
        if message:
            raise ValueError(str(message))
    elif isinstance(error, str) and error.strip():
        raise ValueError(error.strip())

    detail = payload.get("detail")
    if isinstance(detail, str) and detail.strip() and not payload.get("choices"):
        raise ValueError(detail.strip())

    choices = payload.get("choices")
    if not isinstance(choices, list) or not choices:
        return ""

    first = choices[0]
    if not isinstance(first, dict):
        return ""

    message = first.get("message")
    if isinstance(message, dict):
        content = message.get("content")
        if isinstance(content, str):
            return content
        if isinstance(content, list):
            return "".join(
                str(item.get("text") or item.get("content") or "")
                for item in content
                if isinstance(item, dict)
            )

    text = first.get("text")
    return text if isinstance(text, str) else ""


def _number_or_none(value: Any) -> float | int | None:
    if isinstance(value, bool) or value is None:
        return None
    if isinstance(value, (int, float)):
        return value
    try:
        number = float(str(value).strip())
    except Exception:
        return None
    if not number.is_integer():
        return number
    return int(number)


def _merge_usage_candidate(target: dict, candidate: Any) -> None:
    if not isinstance(candidate, dict):
        return

    for key in USAGE_NUMBER_KEYS:
        value = _number_or_none(candidate.get(key))
        if value is not None:
            target[key] = value


def extract_generation_usage(response: Any, elapsed_seconds: float) -> dict:
    payload = _extract_response_payload(response)
    if not isinstance(payload, dict):
        return {"duration_seconds": round(max(elapsed_seconds, 0), 3)}

    usage: dict[str, Any] = {}
    _merge_usage_candidate(usage, payload.get("usage"))
    _merge_usage_candidate(usage, payload.get("usage_metadata"))
    _merge_usage_candidate(usage, payload)

    choices = payload.get("choices")
    if isinstance(choices, list) and choices and isinstance(choices[0], dict):
        first = choices[0]
        _merge_usage_candidate(usage, first.get("usage"))
        _merge_usage_candidate(usage, first)
        message = first.get("message")
        if isinstance(message, dict):
            _merge_usage_candidate(usage, message.get("usage"))

    prompt_tokens = _number_or_none(usage.get("prompt_tokens"))
    completion_tokens = _number_or_none(usage.get("completion_tokens"))
    if completion_tokens is None:
        completion_tokens = _number_or_none(usage.get("eval_count"))
    if prompt_tokens is None:
        prompt_tokens = _number_or_none(usage.get("prompt_eval_count"))
    if usage.get("total_tokens") is None and (
        prompt_tokens is not None or completion_tokens is not None
    ):
        usage["total_tokens"] = (prompt_tokens or 0) + (completion_tokens or 0)

    duration_seconds = max(elapsed_seconds, 0)
    usage["duration_seconds"] = round(duration_seconds, 3)
    eval_duration = _number_or_none(usage.get("eval_duration"))
    speed_duration = duration_seconds
    if eval_duration and eval_duration > 0:
        # Ollama-compatible APIs report eval_duration in nanoseconds.
        speed_duration = float(eval_duration) / 1_000_000_000
    if completion_tokens and speed_duration > 0:
        usage["tokens_per_second"] = round(float(completion_tokens) / speed_duration, 2)

    return usage


def aggregate_discussion_usage(items: list[dict]) -> dict:
    aggregate: dict[str, Any] = {}
    per_model = []
    for item in items:
        if not isinstance(item, dict):
            continue
        usage = item.get("usage")
        if not isinstance(usage, dict):
            continue

        per_model.append(
            {
                "model": item.get("model"),
                "modelName": item.get("modelName"),
                "role": item.get("role"),
                "round": item.get("round"),
                "usage": usage,
            }
        )
        for key in (*USAGE_NUMBER_KEYS, "duration_seconds"):
            value = _number_or_none(usage.get(key))
            if value is not None:
                aggregate[key] = aggregate.get(key, 0) + value

    completion_tokens = _number_or_none(aggregate.get("completion_tokens"))
    if completion_tokens is None:
        completion_tokens = _number_or_none(aggregate.get("eval_count"))
    duration_seconds = _number_or_none(aggregate.get("duration_seconds"))
    if completion_tokens and duration_seconds and duration_seconds > 0:
        aggregate["tokens_per_second"] = round(
            float(completion_tokens) / float(duration_seconds), 2
        )
    if per_model:
        aggregate["per_model"] = per_model
    return aggregate


def normalize_discussion_config(
    discussion: dict,
    models_map: dict,
    ambiguous_model_aliases: set,
    final_model_id: str,
    user: Any,
) -> dict:
    raw_participants = discussion.get("participants")
    if not isinstance(raw_participants, list):
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="discussion.participants must be a list",
        )

    participants = []
    seen = set()
    for item in raw_participants:
        requested_id = str(item or "").strip()
        if not requested_id or requested_id in seen:
            continue
        seen.add(requested_id)
        model = resolve_model_from_lookup(
            models_map,
            ambiguous_model_aliases,
            requested_id,
        )
        if not model:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail=f"Discussion model not found: {requested_id}",
            )
        if not BYPASS_MODEL_ACCESS_CONTROL and getattr(user, "role", None) == "user":
            check_model_access(user, model)
        participants.append(
            {
                "request_id": requested_id,
                "id": _model_request_id(model, requested_id),
                "name": _model_display_name(model, requested_id),
                "model": model,
            }
        )

    if len(participants) < 2:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Multi-model discussion requires at least 2 models",
        )
    if len(participants) > MAX_DISCUSSION_MODELS:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"Multi-model discussion supports at most {MAX_DISCUSSION_MODELS} models",
        )

    try:
        rounds = int(discussion.get("rounds") or DEFAULT_DISCUSSION_ROUNDS)
    except Exception:
        rounds = DEFAULT_DISCUSSION_ROUNDS
    rounds = max(1, min(rounds, MAX_DISCUSSION_ROUNDS))

    requested_final_model = str(
        discussion.get("final_model") or discussion.get("finalModel") or final_model_id or ""
    ).strip()
    final_model = resolve_model_from_lookup(
        models_map,
        ambiguous_model_aliases,
        requested_final_model,
    )
    if not final_model:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"Discussion final model not found: {requested_final_model}",
        )
    if not BYPASS_MODEL_ACCESS_CONTROL and getattr(user, "role", None) == "user":
        check_model_access(user, final_model)

    return {
        "participants": participants,
        "rounds": rounds,
        "final_model": {
            "request_id": requested_final_model,
            "id": _model_request_id(final_model, requested_final_model),
            "name": _model_display_name(final_model, requested_final_model),
            "model": final_model,
        },
        "strategy": str(discussion.get("strategy") or "debate_then_summarize"),
    }


def _stringify_message_content(content: Any) -> str:
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        parts = []
        for item in content:
            if isinstance(item, str):
                parts.append(item)
            elif isinstance(item, dict):
                parts.append(str(item.get("text") or item.get("content") or ""))
        return "\n".join(part for part in parts if part)
    return str(content or "")


def _last_user_prompt(messages: list[dict]) -> str:
    for message in reversed(messages if isinstance(messages, list) else []):
        if isinstance(message, dict) and message.get("role") == "user":
            return _stringify_message_content(message.get("content")).strip()
    return ""


def _successful_turns(rounds: list[dict]) -> list[dict]:
    turns = []
    for round_item in rounds:
        round_index = round_item.get("index")
        for turn in round_item.get("turns", []):
            if turn.get("status") != "completed" or turn.get("error"):
                continue
            content = str(turn.get("content") or "").strip()
            if not content:
                continue
            turns.append({**turn, "round": turn.get("round") or round_index})
    return turns


def _collect_failed_turns(rounds: list[dict]) -> list[dict]:
    failures = []
    for round_item in rounds:
        round_index = round_item.get("index")
        for turn in round_item.get("turns", []):
            error = str(turn.get("error") or "").strip()
            if turn.get("status") != "error" and not error:
                continue
            failures.append(
                {
                    "round": turn.get("round") or round_index,
                    "model": turn.get("model"),
                    "modelName": turn.get("modelName") or turn.get("model") or "Model",
                    "error": error,
                }
            )
    return failures


def _format_transcript(rounds: list[dict]) -> str:
    lines = []
    for round_item in rounds:
        index = round_item.get("index")
        round_lines = []
        for turn in _successful_turns([round_item]):
            name = turn.get("modelName") or turn.get("model") or "Model"
            content = str(turn.get("content") or "").strip()
            round_lines.append(f"- {name}: {content}")

        if round_lines:
            lines.append(f"Round {index}")
            lines.extend(round_lines)
            lines.append("")
    return "\n".join(lines).strip()


def _format_failure_summary(failures: list[dict]) -> str:
    lines = []
    for failure in failures:
        name = failure.get("modelName") or failure.get("model") or "Model"
        round_index = failure.get("round") or "?"
        lines.append(f"- Round {round_index} - {name}: failed")
    return "\n".join(lines).strip()


def _refresh_discussion_counts(discussion_state: dict) -> None:
    rounds = discussion_state.get("rounds", [])
    failures = _collect_failed_turns(rounds)
    discussion_state["successCount"] = len(_successful_turns(rounds))
    discussion_state["failureCount"] = len(failures)
    discussion_state["failures"] = failures


def _build_participant_prompt(
    *,
    user_prompt: str,
    participant_name: str,
    round_index: int,
    transcript: str,
) -> str:
    if round_index <= 1:
        return (
            f"You are {participant_name}, one participant in a multi-model discussion.\n"
            "Give your independent answer to the user's question. Be concise, concrete, and point out assumptions or risks.\n\n"
            f"User question:\n{user_prompt}"
        )

    if not transcript:
        return (
            f"You are {participant_name}, one participant in round {round_index} of a multi-model discussion.\n"
            "No previous successful viewpoints are available yet. Continue from the user's question directly, "
            "and do not infer anything from failed or empty turns.\n\n"
            f"User question:\n{user_prompt}"
        )

    return (
        f"You are {participant_name}, one participant in round {round_index} of a multi-model discussion.\n"
        "Read the previous viewpoints, then add missing points, correct mistakes, or challenge weak reasoning. Do not repeat what is already sufficient.\n"
        "The transcript below includes only successful turns; failed or empty turns are omitted and must not be treated as evidence.\n\n"
        f"User question:\n{user_prompt}\n\n"
        f"Previous discussion:\n{transcript}"
    )


def _build_final_prompt(
    user_prompt: str,
    transcript: str,
    failure_summary: str = "",
) -> str:
    successful_context = transcript or "No successful participant turns are available."
    failed_context = failure_summary or "No failed participant turns were reported."
    return (
        "Several models were asked to discuss the user's question. Produce the final answer for the user.\n"
        "Use only the successful discussion transcript as source material. Failed turns are operational failures, not viewpoints or evidence.\n"
        "If failed turns are listed, briefly disclose that the final answer is based on the successful contributions only. "
        "If no successful participant turns are available, answer directly from the user's question and state that the discussion could not be synthesized from participant viewpoints.\n\n"
        f"User question:\n{user_prompt}\n\n"
        f"Discussion transcript:\n{successful_context}\n\n"
        f"Failed discussion turns (not evidence):\n{failed_context}"
    )


async def _run_model_once(
    request: Request,
    base_form_data: dict,
    user: Any,
    model_id: str,
    messages: list[dict],
) -> dict:
    payload = deepcopy(base_form_data)
    payload.pop("discussion", None)
    payload.pop("stream_options", None)
    payload.pop("background_tasks", None)
    payload.pop("model_item", None)
    payload["model"] = model_id
    payload["messages"] = messages
    payload["stream"] = False
    started_at = time.monotonic()
    response = await generate_chat_completion(request, payload, user)
    elapsed_seconds = time.monotonic() - started_at
    return {
        "content": extract_generation_content(response).strip(),
        "usage": extract_generation_usage(response, elapsed_seconds),
    }


async def generate_multi_model_discussion_completion(
    request: Request,
    form_data: dict,
    user: Any,
    metadata: dict,
    model: dict,
    discussion: dict,
):
    models_map = getattr(request.state, "MODELS", None) or request.app.state.MODELS
    ambiguous_model_aliases = getattr(request.state, "MODELS_AMBIGUOUS", set()) or set()
    settings = normalize_discussion_config(
        discussion,
        models_map,
        ambiguous_model_aliases,
        form_data.get("model", ""),
        user,
    )

    event_emitter = get_event_emitter(metadata)
    if not event_emitter:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Multi-model discussion requires an active chat session",
        )

    chat_id = metadata.get("chat_id")
    message_id = metadata.get("message_id")
    if not chat_id or not message_id:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Multi-model discussion requires chat_id and message_id",
        )

    base_messages = deepcopy(form_data.get("messages") or [])
    user_prompt = _last_user_prompt(base_messages)
    participants_for_ui = [
        {"id": item["id"], "name": item["name"]} for item in settings["participants"]
    ]
    final_model = settings["final_model"]

    discussion_state = {
        "enabled": True,
        "status": "running",
        "participants": participants_for_ui,
        "rounds": [],
        "finalModel": {"id": final_model["id"], "name": final_model["name"]},
        "strategy": settings["strategy"],
        "successCount": 0,
        "failureCount": 0,
        "failures": [],
        "updatedAt": int(time.time()),
    }

    async def emit_discussion(event_type: str, **data):
        discussion_state["updatedAt"] = int(time.time())
        await event_emitter(
            {
                "type": "discussion",
                "data": {
                    "type": event_type,
                    **data,
                    "state": deepcopy(discussion_state),
                },
            }
        )

    async def discussion_task():
        final_content = ""
        usage_items = []
        try:
            await emit_discussion(
                "start",
                status="running",
                participants=participants_for_ui,
                finalModel=discussion_state["finalModel"],
                rounds=settings["rounds"],
                strategy=settings["strategy"],
            )

            for round_index in range(1, settings["rounds"] + 1):
                round_state = {"index": round_index, "turns": []}
                discussion_state["rounds"].append(round_state)
                await emit_discussion("round_start", round=round_index, status="running")

                for participant in settings["participants"]:
                    turn = {
                        "round": round_index,
                        "model": participant["id"],
                        "modelName": participant["name"],
                        "role": "participant",
                        "status": "running",
                        "content": "",
                    }
                    round_state["turns"].append(turn)
                    await emit_discussion("turn_start", **turn)

                    transcript = _format_transcript(discussion_state["rounds"])
                    prompt = _build_participant_prompt(
                        user_prompt=user_prompt,
                        participant_name=participant["name"],
                        round_index=round_index,
                        transcript=transcript,
                    )
                    try:
                        result = await _run_model_once(
                            request,
                            form_data,
                            user,
                            participant["request_id"],
                            [*base_messages, {"role": "user", "content": prompt}],
                        )
                        turn["content"] = result.get("content", "")
                        turn["usage"] = result.get("usage") or {}
                        turn["status"] = "completed"
                        usage_items.append(deepcopy(turn))
                        _refresh_discussion_counts(discussion_state)
                        await emit_discussion("turn_done", **turn)
                    except asyncio.CancelledError:
                        raise
                    except Exception as exc:
                        turn["status"] = "error"
                        turn["error"] = str(exc)
                        _refresh_discussion_counts(discussion_state)
                        await emit_discussion("turn_error", **turn)

                _refresh_discussion_counts(discussion_state)
                await emit_discussion("round_done", round=round_index, status="running")

            transcript = _format_transcript(discussion_state["rounds"])
            failures = _collect_failed_turns(discussion_state["rounds"])
            failure_summary = _format_failure_summary(failures)
            _refresh_discussion_counts(discussion_state)
            discussion_state["status"] = "summarizing"
            await emit_discussion(
                "final_start",
                status="summarizing",
                model=final_model["id"],
                modelName=final_model["name"],
            )
            final_result = await _run_model_once(
                request,
                form_data,
                user,
                final_model["request_id"],
                [
                    *base_messages,
                    {
                        "role": "user",
                        "content": _build_final_prompt(
                            user_prompt,
                            transcript,
                            failure_summary,
                        ),
                    },
                ],
            )
            final_content = final_result.get("content", "")
            final_usage_item = {
                "round": "final",
                "model": final_model["id"],
                "modelName": final_model["name"],
                "role": "final",
                "status": "completed",
                "content": final_content,
                "usage": final_result.get("usage") or {},
            }
            discussion_state["finalUsage"] = final_usage_item["usage"]
            usage_items.append(final_usage_item)
            discussion_usage = aggregate_discussion_usage(usage_items)
            if discussion_usage:
                discussion_state["usage"] = discussion_usage

            if final_content:
                await event_emitter(
                    {"type": "chat:message:delta", "data": {"content": final_content}}
                )

            completed_at = int(time.time())
            discussion_state["status"] = "completed"
            await emit_discussion("done", status="completed", usage=discussion_usage)
            await event_emitter(
                {
                    "type": "chat:completion",
                    "data": {
                        "done": True,
                        "content": final_content,
                        "completedAt": completed_at,
                        "usage": discussion_usage,
                        "discussion": discussion_state,
                    },
                }
            )
            Chats.upsert_message_to_chat_by_id_and_message_id(
                chat_id,
                message_id,
                {
                    "content": final_content,
                    "done": True,
                    "completedAt": completed_at,
                    "usage": discussion_usage,
                    "discussion": discussion_state,
                },
                guard_stopped=True,
            )
        except asyncio.CancelledError:
            completed_at = int(time.time())
            discussion_state["status"] = "stopped"
            await emit_discussion("stopped", status="stopped")
            Chats.upsert_message_to_chat_by_id_and_message_id(
                chat_id,
                message_id,
                {
                    "done": True,
                    "stopped": True,
                    "stoppedByUser": True,
                    "completedAt": completed_at,
                    "discussion": discussion_state,
                },
                guard_stopped=True,
            )
            raise
        except Exception as exc:
            log.exception("Multi-model discussion failed: %s", exc)
            completed_at = int(time.time())
            discussion_state["status"] = "error"
            error = {
                "type": "multi_model_discussion_error",
                "content": str(exc),
                "raw_message": str(exc),
            }
            await emit_discussion("error", status="error", error=error)
            await event_emitter(
                {
                    "type": "chat:completion",
                    "data": {
                        "done": True,
                        "content": final_content,
                        "completedAt": completed_at,
                        "error": error,
                        "discussion": discussion_state,
                    },
                }
            )
            Chats.upsert_message_to_chat_by_id_and_message_id(
                chat_id,
                message_id,
                {
                    "content": final_content,
                    "done": True,
                    "completedAt": completed_at,
                    "error": error,
                    "discussion": discussion_state,
                },
                guard_stopped=True,
            )

    task_id, _task = create_task(discussion_task(), id=chat_id)
    Chats.upsert_message_to_chat_by_id_and_message_id(
        chat_id,
        message_id,
        {
            "model": form_data.get("model", ""),
            "discussion": discussion_state,
        },
        guard_stopped=True,
    )
    return {"status": True, "task_id": task_id}
