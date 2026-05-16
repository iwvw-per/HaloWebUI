import asyncio
from types import SimpleNamespace

from fastapi import HTTPException
import pytest

from open_webui import functions as functions_module
from open_webui.functions import (
    get_function_models,
    resolve_function_model_runtime_id,
)
from open_webui.utils.model_identity import (
    build_model_lookup,
    resolve_model_from_lookup,
)


def _pipe_row(pipe_id: str, name: str):
    return SimpleNamespace(
        id=pipe_id,
        name=name,
        type="pipe",
        created_at=123,
    )


def test_manifold_pipe_models_get_connection_identity_and_display_suffix(monkeypatch):
    pipe = _pipe_row("relay_a", "Relay A")
    module = SimpleNamespace(
        name="Relay A | ",
        pipes=[
            {
                "id": "gpt-4o",
                "name": "GPT-4o",
            }
        ],
    )

    monkeypatch.setattr(
        functions_module.Functions,
        "get_functions_by_type",
        lambda type, active_only=False: [pipe],
    )
    monkeypatch.setattr(
        functions_module,
        "get_function_module_by_id",
        lambda request, pipe_id: module,
    )

    models = asyncio.run(get_function_models(SimpleNamespace()))
    model = models[0]

    assert model["id"] == "relay_a.gpt-4o"
    assert model["name"] == "GPT-4o"
    assert model["connection_name"] == "Relay A"
    assert model["connection_id"] == "relay_a"
    assert model["model_id"] == "gpt-4o"
    assert model["original_id"] == "gpt-4o"
    assert model["model_ref"] == {
        "provider": "pipe",
        "source": "function",
        "connection_id": "relay_a",
    }
    assert model["selection_id"] == "modelref::pipe::function::id:relay_a::gpt-4o"
    assert "relay_a.gpt-4o" in model["legacy_ids"]
    assert "gpt-4o" in model["legacy_ids"]
    assert "Relay A | GPT-4o" in model["legacy_ids"]
    assert "Relay AGPT-4o" in model["legacy_ids"]


def test_duplicate_pipe_submodels_keep_unique_selection_ids_and_ambiguous_bare_name(
    monkeypatch,
):
    pipes = [_pipe_row("relay_a", "Relay A"), _pipe_row("relay_b", "Relay B")]
    modules = {
        "relay_a": SimpleNamespace(
            name="Relay A", pipes=[{"id": "gpt-4o", "name": "GPT-4o"}]
        ),
        "relay_b": SimpleNamespace(
            name="Relay B", pipes=[{"id": "gpt-4o", "name": "GPT-4o"}]
        ),
    }

    monkeypatch.setattr(
        functions_module.Functions,
        "get_functions_by_type",
        lambda type, active_only=False: pipes,
    )
    monkeypatch.setattr(
        functions_module,
        "get_function_module_by_id",
        lambda request, pipe_id: modules[pipe_id],
    )

    models = asyncio.run(get_function_models(SimpleNamespace()))
    lookup, ambiguous = build_model_lookup(models)

    assert models[0]["selection_id"] in lookup
    assert models[1]["selection_id"] in lookup
    assert "relay_a.gpt-4o" in lookup
    assert "relay_b.gpt-4o" in lookup
    assert "gpt-4o" in ambiguous

    with pytest.raises(HTTPException):
        resolve_model_from_lookup(lookup, ambiguous, "gpt-4o")


def test_pipe_selection_id_maps_back_to_executable_pipe_model_id():
    model = {
        "id": "relay_a.gpt-4o",
        "selection_id": "modelref::pipe::function::id:relay_a::gpt-4o",
        "pipe": {"type": "pipe", "id": "relay_a"},
    }

    assert (
        resolve_function_model_runtime_id(
            model["selection_id"], {model["selection_id"]: model}
        )
        == "relay_a.gpt-4o"
    )
    assert resolve_function_model_runtime_id(model["selection_id"]) == "relay_a.gpt-4o"


def test_single_pipe_gets_stable_identity_without_duplicate_display_suffix(monkeypatch):
    pipe = _pipe_row("rag_pipe", "RAG Pipe")
    module = SimpleNamespace()

    monkeypatch.setattr(
        functions_module.Functions,
        "get_functions_by_type",
        lambda type, active_only=False: [pipe],
    )
    monkeypatch.setattr(
        functions_module,
        "get_function_module_by_id",
        lambda request, pipe_id: module,
    )

    models = asyncio.run(get_function_models(SimpleNamespace()))
    model = models[0]

    assert model["id"] == "rag_pipe"
    assert model["selection_id"] == "modelref::pipe::function::id:rag_pipe::rag_pipe"
    assert model["model_ref"] == {
        "provider": "pipe",
        "source": "function",
        "connection_id": "rag_pipe",
    }
    assert "connection_name" not in model
