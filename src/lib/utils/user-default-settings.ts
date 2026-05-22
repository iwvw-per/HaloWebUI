import type { NewUserDefaultSettingsPayload } from '$lib/apis/users';
import {
	DEFAULT_CHAT_TRANSITION_MODE,
	DEFAULT_HIGHLIGHTER_THEME,
	DEFAULT_MERMAID_THEME
} from '$lib/utils/lobehub-chat-appearance';

type UserDefaultUiTemplate = {
	models: string[];
	pinnedModels: string[];
	modelSelectorTagOrder: string[];
	showFeaturedAssistantsOnHome: boolean;
	showChatTitleInTab: boolean;
	landingPageMode: string;
	chatBubble: boolean;
	showUsername: boolean;
	widescreenMode: boolean;
	chatDirection: 'auto' | 'LTR' | 'RTL';
	notificationSound: boolean;
	highlighterTheme: string;
	mermaidTheme: string;
	textScale: number | null;
	transitionMode: 'none' | 'fadeIn' | 'smooth';
	enableAutoScrollOnStreaming: boolean;
	richTextInput: boolean;
	promptAutocomplete: boolean;
	showFormattingToolbar: boolean;
	insertPromptAsRichText: boolean;
	largeTextAsFile: boolean;
	copyFormatted: boolean;
	ctrlEnterToSend: boolean;
	system: string;
	title: { auto: boolean };
	autoTags: boolean;
	autoFollowUps: boolean;
	detectArtifacts: boolean;
	svgPreviewAutoOpen: boolean;
	responseAutoCopy: boolean;
	scrollOnBranchChange: boolean;
	enableMessageQueue: boolean;
	temporaryChatByDefault: boolean;
	newChatInheritsPreviousState: boolean;
	collapseCodeBlocks: boolean;
	collapseHistoricalLongResponses: boolean;
	showInlineCitations: boolean;
	showMessageOutline: boolean;
	showFormulaQuickCopyButton: boolean;
	expandDetails: boolean;
	insertSuggestionPrompt: boolean;
	keepFollowUpPrompts: boolean;
	insertFollowUpPrompt: boolean;
	regenerateMenu: boolean;
	renderMarkdownInPreviews: boolean;
	displayMultiModelResponsesInTabs: boolean;
	stylizedPdfExport: boolean;
	showFloatingActionButtons: boolean;
	floatingActionButtons:
		| {
				id: string;
				label: string;
				input: boolean;
				prompt: string;
		  }[]
		| null;
	memory: boolean;
	showEmojiInCall: boolean;
	voiceInterruption: boolean;
	imageCompression: boolean;
	imageCompressionSize: { width: string; height: string };
	imageCompressionInChannels: boolean;
	audio: {
		stt: { engine: string; language: string };
		tts: { engine: string; playbackRate: number };
	};
	speechAutoSend: boolean;
	responseAutoPlayback: boolean;
	iframeSandboxAllowSameOrigin: boolean;
	iframeSandboxAllowForms: boolean;
	hapticFeedback: boolean;
};

export type UserDefaultUiBoolKey = {
	[K in keyof UserDefaultUiTemplate]: UserDefaultUiTemplate[K] extends boolean ? K : never;
}[keyof UserDefaultUiTemplate];

export type NativeToolBoolKey =
	| 'ENABLE_INTERLEAVED_THINKING'
	| 'ENABLE_WEB_SEARCH_TOOL'
	| 'ENABLE_URL_FETCH'
	| 'ENABLE_URL_FETCH_RENDERED'
	| 'ENABLE_LIST_KNOWLEDGE_BASES'
	| 'ENABLE_SEARCH_KNOWLEDGE_BASES'
	| 'ENABLE_QUERY_KNOWLEDGE_FILES'
	| 'ENABLE_VIEW_KNOWLEDGE_FILE'
	| 'ENABLE_IMAGE_GENERATION_TOOL'
	| 'ENABLE_IMAGE_EDIT'
	| 'ENABLE_MEMORY_TOOLS'
	| 'ENABLE_NOTES'
	| 'ENABLE_CHAT_HISTORY_TOOLS'
	| 'ENABLE_TIME_TOOLS'
	| 'ENABLE_CHANNEL_TOOLS'
	| 'ENABLE_TERMINAL_TOOL';

type NativeToolsTemplate = Record<NativeToolBoolKey, boolean> & {
	TOOL_CALLING_MODE: 'default' | 'native' | 'off';
	MAX_TOOL_CALL_ROUNDS: number;
};

export const DEFAULT_NATIVE_TOOLS_TEMPLATE: NativeToolsTemplate = {
	TOOL_CALLING_MODE: 'default',
	ENABLE_INTERLEAVED_THINKING: false,
	MAX_TOOL_CALL_ROUNDS: 15,
	ENABLE_WEB_SEARCH_TOOL: true,
	ENABLE_URL_FETCH: true,
	ENABLE_URL_FETCH_RENDERED: false,
	ENABLE_LIST_KNOWLEDGE_BASES: true,
	ENABLE_SEARCH_KNOWLEDGE_BASES: true,
	ENABLE_QUERY_KNOWLEDGE_FILES: true,
	ENABLE_VIEW_KNOWLEDGE_FILE: true,
	ENABLE_IMAGE_GENERATION_TOOL: true,
	ENABLE_IMAGE_EDIT: false,
	ENABLE_MEMORY_TOOLS: true,
	ENABLE_NOTES: false,
	ENABLE_CHAT_HISTORY_TOOLS: true,
	ENABLE_TIME_TOOLS: true,
	ENABLE_CHANNEL_TOOLS: true,
	ENABLE_TERMINAL_TOOL: false
};

export const DEFAULT_USER_DEFAULT_UI_TEMPLATE: UserDefaultUiTemplate = {
	models: [],
	pinnedModels: [],
	modelSelectorTagOrder: [],
	showFeaturedAssistantsOnHome: true,
	showChatTitleInTab: true,
	landingPageMode: '',
	chatBubble: true,
	showUsername: false,
	widescreenMode: false,
	chatDirection: 'auto',
	notificationSound: true,
	highlighterTheme: DEFAULT_HIGHLIGHTER_THEME,
	mermaidTheme: DEFAULT_MERMAID_THEME,
	textScale: null,
	transitionMode: DEFAULT_CHAT_TRANSITION_MODE,
	enableAutoScrollOnStreaming: true,
	richTextInput: true,
	promptAutocomplete: false,
	showFormattingToolbar: false,
	insertPromptAsRichText: false,
	largeTextAsFile: false,
	copyFormatted: false,
	ctrlEnterToSend: false,
	system: '',
	title: { auto: true },
	autoTags: true,
	autoFollowUps: true,
	detectArtifacts: true,
	svgPreviewAutoOpen: true,
	responseAutoCopy: false,
	scrollOnBranchChange: true,
	enableMessageQueue: true,
	temporaryChatByDefault: false,
	newChatInheritsPreviousState: false,
	collapseCodeBlocks: false,
	collapseHistoricalLongResponses: true,
	showInlineCitations: true,
	showMessageOutline: true,
	showFormulaQuickCopyButton: true,
	expandDetails: false,
	insertSuggestionPrompt: false,
	keepFollowUpPrompts: false,
	insertFollowUpPrompt: false,
	regenerateMenu: true,
	renderMarkdownInPreviews: true,
	displayMultiModelResponsesInTabs: false,
	stylizedPdfExport: true,
	showFloatingActionButtons: true,
	floatingActionButtons: null,
	memory: false,
	showEmojiInCall: false,
	voiceInterruption: false,
	imageCompression: false,
	imageCompressionSize: { width: '', height: '' },
	imageCompressionInChannels: true,
	audio: {
		stt: { engine: '', language: '' },
		tts: { engine: '', playbackRate: 1 }
	},
	speechAutoSend: false,
	responseAutoPlayback: false,
	iframeSandboxAllowSameOrigin: false,
	iframeSandboxAllowForms: false,
	hapticFeedback: false
};

const UI_BOOL_KEYS: UserDefaultUiBoolKey[] = [
	'showFeaturedAssistantsOnHome',
	'showChatTitleInTab',
	'chatBubble',
	'showUsername',
	'widescreenMode',
	'notificationSound',
	'enableAutoScrollOnStreaming',
	'richTextInput',
	'promptAutocomplete',
	'showFormattingToolbar',
	'insertPromptAsRichText',
	'largeTextAsFile',
	'copyFormatted',
	'ctrlEnterToSend',
	'autoTags',
	'autoFollowUps',
	'detectArtifacts',
	'svgPreviewAutoOpen',
	'responseAutoCopy',
	'scrollOnBranchChange',
	'enableMessageQueue',
	'temporaryChatByDefault',
	'newChatInheritsPreviousState',
	'collapseCodeBlocks',
	'collapseHistoricalLongResponses',
	'showInlineCitations',
	'showMessageOutline',
	'showFormulaQuickCopyButton',
	'expandDetails',
	'insertSuggestionPrompt',
	'keepFollowUpPrompts',
	'insertFollowUpPrompt',
	'regenerateMenu',
	'renderMarkdownInPreviews',
	'displayMultiModelResponsesInTabs',
	'stylizedPdfExport',
	'showFloatingActionButtons',
	'memory',
	'showEmojiInCall',
	'voiceInterruption',
	'imageCompression',
	'imageCompressionInChannels',
	'speechAutoSend',
	'responseAutoPlayback',
	'iframeSandboxAllowSameOrigin',
	'iframeSandboxAllowForms',
	'hapticFeedback'
];

const UI_STRING_KEYS = [
	'highlighterTheme',
	'mermaidTheme',
	'landingPageMode',
	'chatDirection',
	'transitionMode',
	'system'
];

const UI_ARRAY_KEYS = ['models', 'pinnedModels', 'modelSelectorTagOrder'];
const NATIVE_TOOL_BOOL_KEYS: NativeToolBoolKey[] = [
	'ENABLE_INTERLEAVED_THINKING',
	'ENABLE_WEB_SEARCH_TOOL',
	'ENABLE_URL_FETCH',
	'ENABLE_URL_FETCH_RENDERED',
	'ENABLE_LIST_KNOWLEDGE_BASES',
	'ENABLE_SEARCH_KNOWLEDGE_BASES',
	'ENABLE_QUERY_KNOWLEDGE_FILES',
	'ENABLE_VIEW_KNOWLEDGE_FILE',
	'ENABLE_IMAGE_GENERATION_TOOL',
	'ENABLE_IMAGE_EDIT',
	'ENABLE_MEMORY_TOOLS',
	'ENABLE_NOTES',
	'ENABLE_CHAT_HISTORY_TOOLS',
	'ENABLE_TIME_TOOLS',
	'ENABLE_CHANNEL_TOOLS',
	'ENABLE_TERMINAL_TOOL'
];

const clone = <T>(value: T): T => JSON.parse(JSON.stringify(value));
const asRecord = (value: unknown): Record<string, any> =>
	value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, any>) : {};

const clampToolRounds = (value: unknown) => {
	const parsed = Number(value);
	if (!Number.isFinite(parsed)) return 15;
	return Math.min(30, Math.max(1, Math.round(parsed)));
};

const isEqual = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b);

export const createEmptyNewUserDefaultSettings = (): NewUserDefaultSettingsPayload => ({
	enabled: false,
	roles: ['user', 'pending'],
	ui: {},
	tools: { native_tools: {} }
});

export const normalizeNewUserDefaultSettings = (
	value: Partial<NewUserDefaultSettingsPayload> | null | undefined
) => {
	const raw = asRecord(value);
	const rawUi = asRecord(raw.ui);
	const rawTools = asRecord(raw.tools);
	const rawNativeTools = asRecord(rawTools.native_tools);
	const defaults = clone(DEFAULT_USER_DEFAULT_UI_TEMPLATE);
	const nativeDefaults = clone(DEFAULT_NATIVE_TOOLS_TEMPLATE);

	return {
		enabled: Boolean(raw.enabled),
		roles: Array.isArray(raw.roles)
			? raw.roles.filter((role: unknown) => role === 'user' || role === 'pending')
			: ['user', 'pending'],
		ui: {
			...defaults,
			...rawUi,
			models: Array.isArray(rawUi.models) ? rawUi.models : [],
			pinnedModels: Array.isArray(rawUi.pinnedModels) ? rawUi.pinnedModels : [],
			modelSelectorTagOrder: Array.isArray(rawUi.modelSelectorTagOrder)
				? rawUi.modelSelectorTagOrder
				: [],
			title: {
				...defaults.title,
				...asRecord(rawUi.title)
			},
			imageCompressionSize: {
				...defaults.imageCompressionSize,
				...asRecord(rawUi.imageCompressionSize)
			},
			audio: {
				stt: {
					...defaults.audio.stt,
					...asRecord(asRecord(rawUi.audio).stt)
				},
				tts: {
					...defaults.audio.tts,
					...asRecord(asRecord(rawUi.audio).tts)
				}
			}
		},
		tools: {
			native_tools: {
				...nativeDefaults,
				...rawNativeTools,
				TOOL_CALLING_MODE: ['default', 'native', 'off'].includes(rawNativeTools.TOOL_CALLING_MODE)
					? rawNativeTools.TOOL_CALLING_MODE
					: nativeDefaults.TOOL_CALLING_MODE,
				MAX_TOOL_CALL_ROUNDS: clampToolRounds(
					rawNativeTools.MAX_TOOL_CALL_ROUNDS ?? nativeDefaults.MAX_TOOL_CALL_ROUNDS
				)
			}
		}
	};
};

export const pickUserDefaultUiFields = (ui: Record<string, any>) => {
	const source = asRecord(ui);
	const picked: Record<string, any> = {};

	for (const key of UI_BOOL_KEYS) {
		if (typeof source[key] === 'boolean') picked[key] = source[key];
	}

	for (const key of UI_STRING_KEYS) {
		if (typeof source[key] === 'string') picked[key] = source[key];
	}

	for (const key of UI_ARRAY_KEYS) {
		if (Array.isArray(source[key]))
			picked[key] = source[key].filter((item) => typeof item === 'string');
	}

	if ('textScale' in source) {
		picked.textScale = source.textScale === null ? null : Number(source.textScale);
	}

	if (source.title && typeof source.title.auto === 'boolean') {
		picked.title = { auto: source.title.auto };
	}

	if (source.audio) {
		const audio = asRecord(source.audio);
		const stt = asRecord(audio.stt);
		const tts = asRecord(audio.tts);
		const pickedAudio: Record<string, any> = {};

		if (typeof stt.engine === 'string' || typeof stt.language === 'string') {
			pickedAudio.stt = {};
			if (typeof stt.engine === 'string') pickedAudio.stt.engine = stt.engine;
			if (typeof stt.language === 'string') pickedAudio.stt.language = stt.language;
		}

		if (typeof tts.engine === 'string' || typeof tts.playbackRate === 'number') {
			pickedAudio.tts = {};
			if (typeof tts.engine === 'string' && tts.engine !== 'browser-kokoro') {
				pickedAudio.tts.engine = tts.engine;
			}
			if (typeof tts.playbackRate === 'number') pickedAudio.tts.playbackRate = tts.playbackRate;
		}

		if (Object.keys(pickedAudio).length > 0) picked.audio = pickedAudio;
	}

	if (source.imageCompressionSize) {
		const size = asRecord(source.imageCompressionSize);
		picked.imageCompressionSize = {
			width: size.width === undefined || size.width === null ? '' : String(size.width),
			height: size.height === undefined || size.height === null ? '' : String(size.height)
		};
	}

	if ('floatingActionButtons' in source) {
		picked.floatingActionButtons =
			source.floatingActionButtons === null
				? null
				: Array.isArray(source.floatingActionButtons)
					? source.floatingActionButtons
					: null;
	}

	return picked;
};

export const buildNewUserDefaultSettingsPayload = (
	value: ReturnType<typeof normalizeNewUserDefaultSettings>
): NewUserDefaultSettingsPayload => {
	const pickedUi = pickUserDefaultUiFields(value.ui);
	const defaultUi = pickUserDefaultUiFields(clone(DEFAULT_USER_DEFAULT_UI_TEMPLATE));
	const ui: Record<string, any> = {};

	for (const [key, nextValue] of Object.entries(pickedUi)) {
		if (!isEqual(nextValue, defaultUi[key])) {
			ui[key] = nextValue;
		}
	}

	const native = asRecord(value.tools.native_tools);
	const nativeTools: Record<string, any> = {};

	if (
		['default', 'native', 'off'].includes(native.TOOL_CALLING_MODE) &&
		native.TOOL_CALLING_MODE !== DEFAULT_NATIVE_TOOLS_TEMPLATE.TOOL_CALLING_MODE
	) {
		nativeTools.TOOL_CALLING_MODE = native.TOOL_CALLING_MODE;
	}
	if (
		clampToolRounds(native.MAX_TOOL_CALL_ROUNDS) !==
		DEFAULT_NATIVE_TOOLS_TEMPLATE.MAX_TOOL_CALL_ROUNDS
	) {
		nativeTools.MAX_TOOL_CALL_ROUNDS = clampToolRounds(native.MAX_TOOL_CALL_ROUNDS);
	}
	for (const key of NATIVE_TOOL_BOOL_KEYS) {
		if (Boolean(native[key]) !== DEFAULT_NATIVE_TOOLS_TEMPLATE[key]) {
			nativeTools[key] = Boolean(native[key]);
		}
	}

	return {
		enabled: Boolean(value.enabled),
		roles: value.roles.filter((role) => role === 'user' || role === 'pending'),
		ui,
		tools: { native_tools: nativeTools }
	};
};
