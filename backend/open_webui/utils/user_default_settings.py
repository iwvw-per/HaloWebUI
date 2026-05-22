import copy
from typing import Any


DEFAULT_NEW_USER_DEFAULT_SETTINGS = {
    "configured": False,
    "enabled": False,
    "roles": ["user", "pending"],
    "ui": {},
    "tools": {"native_tools": {}},
}

ALLOWED_NEW_USER_DEFAULT_ROLES = {"user", "pending"}

TOOL_CALLING_MODE_ALLOWED = {"default", "native", "off"}
MAX_TOOL_CALL_ROUNDS_DEFAULT = 15
MAX_TOOL_CALL_ROUNDS_MIN = 1
MAX_TOOL_CALL_ROUNDS_MAX = 30

ALLOWED_NATIVE_TOOL_BOOL_KEYS = {
    "ENABLE_INTERLEAVED_THINKING",
    "ENABLE_WEB_SEARCH_TOOL",
    "ENABLE_URL_FETCH",
    "ENABLE_URL_FETCH_RENDERED",
    "ENABLE_LIST_KNOWLEDGE_BASES",
    "ENABLE_SEARCH_KNOWLEDGE_BASES",
    "ENABLE_QUERY_KNOWLEDGE_FILES",
    "ENABLE_VIEW_KNOWLEDGE_FILE",
    "ENABLE_IMAGE_GENERATION_TOOL",
    "ENABLE_IMAGE_EDIT",
    "ENABLE_MEMORY_TOOLS",
    "ENABLE_NOTES",
    "ENABLE_CHAT_HISTORY_TOOLS",
    "ENABLE_TIME_TOOLS",
    "ENABLE_CHANNEL_TOOLS",
    "ENABLE_TERMINAL_TOOL",
}

ALLOWED_UI_BOOL_KEYS = {
    "showFeaturedAssistantsOnHome",
    "showChatTitleInTab",
    "chatBubble",
    "showUsername",
    "widescreenMode",
    "notificationSound",
    "enableAutoScrollOnStreaming",
    "richTextInput",
    "promptAutocomplete",
    "showFormattingToolbar",
    "insertPromptAsRichText",
    "largeTextAsFile",
    "copyFormatted",
    "ctrlEnterToSend",
    "autoTags",
    "autoFollowUps",
    "detectArtifacts",
    "svgPreviewAutoOpen",
    "responseAutoCopy",
    "scrollOnBranchChange",
    "enableMessageQueue",
    "temporaryChatByDefault",
    "newChatInheritsPreviousState",
    "collapseCodeBlocks",
    "collapseHistoricalLongResponses",
    "showInlineCitations",
    "showMessageOutline",
    "showFormulaQuickCopyButton",
    "expandDetails",
    "insertSuggestionPrompt",
    "keepFollowUpPrompts",
    "insertFollowUpPrompt",
    "regenerateMenu",
    "renderMarkdownInPreviews",
    "displayMultiModelResponsesInTabs",
    "stylizedPdfExport",
    "showFloatingActionButtons",
    "memory",
    "imageCompression",
    "imageCompressionInChannels",
}

ALLOWED_UI_STRING_KEYS = {
    "highlighterTheme": 120,
    "mermaidTheme": 40,
    "chatDirection": 10,
    "transitionMode": 20,
    "system": 12000,
}

ALLOWED_STRING_ARRAY_KEYS = {
    "models",
}

ALLOWED_CHAT_DIRECTIONS = {"auto", "LTR", "RTL"}
ALLOWED_TRANSITION_MODES = {"none", "fadeIn", "smooth"}
ALLOWED_MERMAID_THEMES = {"lobe-theme", "default", "base", "dark", "forest", "neutral"}


def _as_dict(value: Any) -> dict:
    return value if isinstance(value, dict) else {}


def _is_bool(value: Any) -> bool:
    return isinstance(value, bool)


def _clean_string(value: Any, *, max_length: int = 400) -> str | None:
    if not isinstance(value, str):
        return None
    return value[:max_length]


def _clean_string_array(value: Any, *, max_items: int = 200, max_length: int = 400):
    if not isinstance(value, list):
        return None

    cleaned = []
    for item in value[:max_items]:
        if isinstance(item, str):
            cleaned.append(item[:max_length])
    return cleaned


def _clean_number(value: Any, *, min_value: float, max_value: float):
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        return None

    if value < min_value or value > max_value:
        return None

    return value


def _clean_int(value: Any, *, min_value: int, max_value: int, default: int) -> int:
    if isinstance(value, bool):
        return default

    try:
        parsed = int(value)
    except (TypeError, ValueError):
        return default

    return max(min_value, min(max_value, parsed))


def _clean_dimension(value: Any) -> str | None:
    if value is None:
        return None
    text = str(value).strip()
    if text == "":
        return ""
    if not text.isdigit():
        return None

    parsed = int(text)
    if parsed < 1 or parsed > 10000:
        return None
    return str(parsed)


def _clean_roles(value: Any) -> list[str]:
    if not isinstance(value, list):
        return copy.deepcopy(DEFAULT_NEW_USER_DEFAULT_SETTINGS["roles"])

    cleaned = []
    for role in value:
        if role in ALLOWED_NEW_USER_DEFAULT_ROLES and role not in cleaned:
            cleaned.append(role)

    return cleaned


def sanitize_native_tools_config(value: Any) -> dict:
    raw = _as_dict(value)
    cleaned: dict[str, Any] = {}

    mode = raw.get("TOOL_CALLING_MODE")
    if isinstance(mode, str) and mode in TOOL_CALLING_MODE_ALLOWED:
        cleaned["TOOL_CALLING_MODE"] = mode

    if "MAX_TOOL_CALL_ROUNDS" in raw:
        cleaned["MAX_TOOL_CALL_ROUNDS"] = _clean_int(
            raw.get("MAX_TOOL_CALL_ROUNDS", MAX_TOOL_CALL_ROUNDS_DEFAULT),
            min_value=MAX_TOOL_CALL_ROUNDS_MIN,
            max_value=MAX_TOOL_CALL_ROUNDS_MAX,
            default=MAX_TOOL_CALL_ROUNDS_DEFAULT,
        )

    for key in ALLOWED_NATIVE_TOOL_BOOL_KEYS:
        if _is_bool(raw.get(key)):
            cleaned[key] = raw[key]

    return cleaned


def _sanitize_title(value: Any) -> dict:
    raw = _as_dict(value)
    cleaned = {}
    if _is_bool(raw.get("auto")):
        cleaned["auto"] = raw["auto"]
    return cleaned


def _sanitize_image_compression_size(value: Any) -> dict:
    raw = _as_dict(value)
    cleaned = {}
    width = _clean_dimension(raw.get("width"))
    height = _clean_dimension(raw.get("height"))
    if width is not None:
        cleaned["width"] = width
    if height is not None:
        cleaned["height"] = height
    return cleaned


def _sanitize_floating_action_buttons(value: Any):
    if value is None:
        return None
    if not isinstance(value, list):
        return None

    cleaned = []
    for item in value[:20]:
        raw = _as_dict(item)
        button = {
            "id": _clean_string(raw.get("id"), max_length=80),
            "label": _clean_string(raw.get("label"), max_length=80),
            "input": raw.get("input"),
            "prompt": _clean_string(raw.get("prompt"), max_length=8000),
        }
        if (
            button["id"]
            and button["label"]
            and _is_bool(button["input"])
            and button["prompt"] is not None
        ):
            cleaned.append(button)

    return cleaned


def sanitize_user_default_ui(value: Any) -> dict:
    raw = _as_dict(value)
    cleaned: dict[str, Any] = {}

    for key in ALLOWED_UI_BOOL_KEYS:
        if _is_bool(raw.get(key)):
            cleaned[key] = raw[key]

    for key, max_length in ALLOWED_UI_STRING_KEYS.items():
        next_value = _clean_string(raw.get(key), max_length=max_length)
        if next_value is None:
            continue
        if key == "chatDirection" and next_value not in ALLOWED_CHAT_DIRECTIONS:
            continue
        if key == "transitionMode" and next_value not in ALLOWED_TRANSITION_MODES:
            continue
        if key == "mermaidTheme" and next_value not in ALLOWED_MERMAID_THEMES:
            continue
        cleaned[key] = next_value

    for key in ALLOWED_STRING_ARRAY_KEYS:
        if key in raw:
            cleaned[key] = _clean_string_array(raw.get(key)) or []

    if "textScale" in raw:
        if raw.get("textScale") is None:
            cleaned["textScale"] = None
        else:
            text_scale = _clean_number(raw.get("textScale"), min_value=0.8, max_value=1.5)
            if text_scale is not None:
                cleaned["textScale"] = text_scale

    title = _sanitize_title(raw.get("title"))
    if title:
        cleaned["title"] = title

    image_compression_size = _sanitize_image_compression_size(
        raw.get("imageCompressionSize")
    )
    if image_compression_size:
        cleaned["imageCompressionSize"] = image_compression_size

    if "floatingActionButtons" in raw:
        buttons = _sanitize_floating_action_buttons(raw.get("floatingActionButtons"))
        if buttons is not None:
            cleaned["floatingActionButtons"] = buttons

    return cleaned


def sanitize_new_user_default_settings(value: Any) -> dict:
    raw = _as_dict(value)
    tools = _as_dict(raw.get("tools"))
    ui = sanitize_user_default_ui(raw.get("ui"))
    native_tools = sanitize_native_tools_config(tools.get("native_tools"))

    configured = (_is_bool(raw.get("configured")) and raw["configured"]) or bool(
        ui or native_tools
    )

    return {
        "configured": configured,
        "enabled": bool(ui or native_tools),
        "roles": copy.deepcopy(DEFAULT_NEW_USER_DEFAULT_SETTINGS["roles"]),
        "ui": ui,
        "tools": {"native_tools": native_tools},
    }


def build_new_user_settings_from_template(value: Any, role: str) -> dict | None:
    template = sanitize_new_user_default_settings(value)

    if not template["enabled"] or role not in template["roles"]:
        return None

    settings: dict[str, Any] = {}
    ui = _as_dict(template.get("ui"))
    native_tools = _as_dict(_as_dict(template.get("tools")).get("native_tools"))

    if ui:
        settings["ui"] = copy.deepcopy(ui)
    if native_tools:
        settings["tools"] = {"native_tools": copy.deepcopy(native_tools)}

    if not settings:
        return None

    settings["revision"] = 0
    return settings
