import type { MermaidConfig } from 'mermaid';

export type ChatTransitionMode = 'none' | 'fadeIn' | 'smooth';

type ChatTransitionModeSettingsLike = {
	transitionMode?: unknown;
	chatFadeStreamingText?: boolean | null;
};

export const DEFAULT_CHAT_TRANSITION_MODE: ChatTransitionMode = 'fadeIn';

export type HighlighterThemeOption = {
	displayName: string;
	id: string;
	type?: 'light' | 'dark';
};

export type MermaidThemeOption = {
	background?: string;
	displayName: string;
	id: MermaidThemeId;
};

export type MermaidThemeId = 'lobe-theme' | 'default' | 'base' | 'dark' | 'forest' | 'neutral';

export const DEFAULT_HIGHLIGHTER_THEME = 'lobe-theme';
export const DEFAULT_MERMAID_THEME: MermaidThemeId = 'lobe-theme';
const LOBE_SHIKI_LIGHT_THEME_NAME = 'lobe-theme-light';
const LOBE_SHIKI_DARK_THEME_NAME = 'lobe-theme-dark';

// Synced from LobeHub's @lobehub/ui theme selectors and shiki@3.21.x bundled themes.
export const LOBE_HIGHLIGHTER_THEMES: HighlighterThemeOption[] = [
	{
		displayName: 'Lobe Theme',
		id: 'lobe-theme'
	},
	{
		displayName: 'Andromeeda',
		id: 'andromeeda',
		type: 'dark'
	},
	{
		displayName: 'Aurora X',
		id: 'aurora-x',
		type: 'dark'
	},
	{
		displayName: 'Ayu Dark',
		id: 'ayu-dark',
		type: 'dark'
	},
	{
		displayName: 'Catppuccin Frappé',
		id: 'catppuccin-frappe',
		type: 'dark'
	},
	{
		displayName: 'Catppuccin Latte',
		id: 'catppuccin-latte',
		type: 'light'
	},
	{
		displayName: 'Catppuccin Macchiato',
		id: 'catppuccin-macchiato',
		type: 'dark'
	},
	{
		displayName: 'Catppuccin Mocha',
		id: 'catppuccin-mocha',
		type: 'dark'
	},
	{
		displayName: 'Dark Plus',
		id: 'dark-plus',
		type: 'dark'
	},
	{
		displayName: 'Dracula Theme',
		id: 'dracula',
		type: 'dark'
	},
	{
		displayName: 'Dracula Theme Soft',
		id: 'dracula-soft',
		type: 'dark'
	},
	{
		displayName: 'Everforest Dark',
		id: 'everforest-dark',
		type: 'dark'
	},
	{
		displayName: 'Everforest Light',
		id: 'everforest-light',
		type: 'light'
	},
	{
		displayName: 'GitHub Dark',
		id: 'github-dark',
		type: 'dark'
	},
	{
		displayName: 'GitHub Dark Default',
		id: 'github-dark-default',
		type: 'dark'
	},
	{
		displayName: 'GitHub Dark Dimmed',
		id: 'github-dark-dimmed',
		type: 'dark'
	},
	{
		displayName: 'GitHub Dark High Contrast',
		id: 'github-dark-high-contrast',
		type: 'dark'
	},
	{
		displayName: 'GitHub Light',
		id: 'github-light',
		type: 'light'
	},
	{
		displayName: 'GitHub Light Default',
		id: 'github-light-default',
		type: 'light'
	},
	{
		displayName: 'GitHub Light High Contrast',
		id: 'github-light-high-contrast',
		type: 'light'
	},
	{
		displayName: 'Gruvbox Dark Hard',
		id: 'gruvbox-dark-hard',
		type: 'dark'
	},
	{
		displayName: 'Gruvbox Dark Medium',
		id: 'gruvbox-dark-medium',
		type: 'dark'
	},
	{
		displayName: 'Gruvbox Dark Soft',
		id: 'gruvbox-dark-soft',
		type: 'dark'
	},
	{
		displayName: 'Gruvbox Light Hard',
		id: 'gruvbox-light-hard',
		type: 'light'
	},
	{
		displayName: 'Gruvbox Light Medium',
		id: 'gruvbox-light-medium',
		type: 'light'
	},
	{
		displayName: 'Gruvbox Light Soft',
		id: 'gruvbox-light-soft',
		type: 'light'
	},
	{
		displayName: 'Houston',
		id: 'houston',
		type: 'dark'
	},
	{
		displayName: 'Kanagawa Dragon',
		id: 'kanagawa-dragon',
		type: 'dark'
	},
	{
		displayName: 'Kanagawa Lotus',
		id: 'kanagawa-lotus',
		type: 'light'
	},
	{
		displayName: 'Kanagawa Wave',
		id: 'kanagawa-wave',
		type: 'dark'
	},
	{
		displayName: 'LaserWave',
		id: 'laserwave',
		type: 'dark'
	},
	{
		displayName: 'Light Plus',
		id: 'light-plus',
		type: 'light'
	},
	{
		displayName: 'Material Theme',
		id: 'material-theme',
		type: 'dark'
	},
	{
		displayName: 'Material Theme Darker',
		id: 'material-theme-darker',
		type: 'dark'
	},
	{
		displayName: 'Material Theme Lighter',
		id: 'material-theme-lighter',
		type: 'light'
	},
	{
		displayName: 'Material Theme Ocean',
		id: 'material-theme-ocean',
		type: 'dark'
	},
	{
		displayName: 'Material Theme Palenight',
		id: 'material-theme-palenight',
		type: 'dark'
	},
	{
		displayName: 'Min Dark',
		id: 'min-dark',
		type: 'dark'
	},
	{
		displayName: 'Min Light',
		id: 'min-light',
		type: 'light'
	},
	{
		displayName: 'Monokai',
		id: 'monokai',
		type: 'dark'
	},
	{
		displayName: 'Night Owl',
		id: 'night-owl',
		type: 'dark'
	},
	{
		displayName: 'Nord',
		id: 'nord',
		type: 'dark'
	},
	{
		displayName: 'One Dark Pro',
		id: 'one-dark-pro',
		type: 'dark'
	},
	{
		displayName: 'One Light',
		id: 'one-light',
		type: 'light'
	},
	{
		displayName: 'Plastic',
		id: 'plastic',
		type: 'dark'
	},
	{
		displayName: 'Poimandres',
		id: 'poimandres',
		type: 'dark'
	},
	{
		displayName: 'Red',
		id: 'red',
		type: 'dark'
	},
	{
		displayName: 'Rosé Pine',
		id: 'rose-pine',
		type: 'dark'
	},
	{
		displayName: 'Rosé Pine Dawn',
		id: 'rose-pine-dawn',
		type: 'light'
	},
	{
		displayName: 'Rosé Pine Moon',
		id: 'rose-pine-moon',
		type: 'dark'
	},
	{
		displayName: 'Slack Dark',
		id: 'slack-dark',
		type: 'dark'
	},
	{
		displayName: 'Slack Ochin',
		id: 'slack-ochin',
		type: 'light'
	},
	{
		displayName: 'Snazzy Light',
		id: 'snazzy-light',
		type: 'light'
	},
	{
		displayName: 'Solarized Dark',
		id: 'solarized-dark',
		type: 'dark'
	},
	{
		displayName: 'Solarized Light',
		id: 'solarized-light',
		type: 'light'
	},
	{
		displayName: "Synthwave '84",
		id: 'synthwave-84',
		type: 'dark'
	},
	{
		displayName: 'Tokyo Night',
		id: 'tokyo-night',
		type: 'dark'
	},
	{
		displayName: 'Vesper',
		id: 'vesper',
		type: 'dark'
	},
	{
		displayName: 'Vitesse Black',
		id: 'vitesse-black',
		type: 'dark'
	},
	{
		displayName: 'Vitesse Dark',
		id: 'vitesse-dark',
		type: 'dark'
	},
	{
		displayName: 'Vitesse Light',
		id: 'vitesse-light',
		type: 'light'
	}
];

export const LOBE_MERMAID_THEMES: MermaidThemeOption[] = [
	{
		displayName: 'Lobe Theme',
		id: 'lobe-theme'
	},
	{
		background: '#fbf9ff',
		displayName: 'Mermaid Default',
		id: 'default'
	},
	{
		background: '#fffcf8',
		displayName: 'Mermaid Base',
		id: 'base'
	},
	{
		background: '#000',
		displayName: 'Mermaid Dark',
		id: 'dark'
	},
	{
		background: '#f9ffeb',
		displayName: 'Mermaid Forest',
		id: 'forest'
	},
	{
		background: '#fff',
		displayName: 'Mermaid Neutral',
		id: 'neutral'
	}
];

const HIGHLIGHTER_THEME_IDS = new Set(LOBE_HIGHLIGHTER_THEMES.map((item) => item.id));
const MERMAID_THEME_IDS = new Set(LOBE_MERMAID_THEMES.map((item) => item.id));

type Palette = {
	background: string;
	border: string;
	comment: string;
	error: string;
	info: string;
	infoBg: string;
	infoBorder: string;
	infoText: string;
	primaryText: string;
	purple: string;
	secondaryText: string;
	success: string;
	successBg: string;
	successBorder: string;
	successText: string;
	text: string;
	warning: string;
};

const getPalette = (isDark: boolean): Palette =>
	isDark
		? {
				background: '#111827',
				border: '#374151',
				comment: '#6b7280',
				error: '#f87171',
				info: '#60a5fa',
				infoBg: '#0f1f3a',
				infoBorder: '#1d4ed8',
				infoText: '#93c5fd',
				primaryText: '#e5e7eb',
				purple: '#c084fc',
				secondaryText: '#9ca3af',
				success: '#4ade80',
				successBg: '#052e16',
				successBorder: '#15803d',
				successText: '#86efac',
				text: '#e5e7eb',
				warning: '#fbbf24'
			}
		: {
				background: '#ffffff',
				border: '#e5e7eb',
				comment: '#9ca3af',
				error: '#dc2626',
				info: '#2563eb',
				infoBg: '#eff6ff',
				infoBorder: '#bfdbfe',
				infoText: '#1d4ed8',
				primaryText: '#111827',
				purple: '#7c3aed',
				secondaryText: '#4b5563',
				success: '#16a34a',
				successBg: '#ecfdf5',
				successBorder: '#a7f3d0',
				successText: '#047857',
				text: '#111827',
				warning: '#d97706'
			};

const FONT_FAMILY =
	"'HarmonyOS Sans SC', -apple-system, BlinkMacSystemFont, 'PingFang SC', 'Inter', ui-sans-serif, system-ui, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, 'Noto Sans', 'Hiragino Sans GB', 'Microsoft YaHei', 'WenQuanYi Micro Hei', sans-serif";
const MONO_FONT_FAMILY =
	"'JetBrains Mono', 'Fira Code', 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace";

const getRuntimeHighlighterThemeId = (themeId: string, isDark: boolean) =>
	normalizeHighlighterTheme(themeId) === DEFAULT_HIGHLIGHTER_THEME
		? isDark
			? LOBE_SHIKI_DARK_THEME_NAME
			: LOBE_SHIKI_LIGHT_THEME_NAME
		: normalizeHighlighterTheme(themeId);

export const resolveRuntimeHighlighterThemeId = getRuntimeHighlighterThemeId;

const createLobeShikiTheme = (isDark: boolean) => {
	const palette = getPalette(isDark);

	return {
		colors: {
			'editor.background': palette.background,
			'editor.foreground': palette.primaryText
		},
		displayName: 'Lobe Theme',
		name: isDark ? LOBE_SHIKI_DARK_THEME_NAME : LOBE_SHIKI_LIGHT_THEME_NAME,
		semanticHighlighting: true,
		tokenColors: [
			{ settings: { background: palette.background, foreground: palette.text } },
			{ scope: 'string', settings: { foreground: palette.success } },
			{
				scope: 'punctuation, constant.other.symbol',
				settings: { foreground: palette.info }
			},
			{
				scope: 'constant.character.escape, text.html constant.character.entity.named',
				settings: { foreground: palette.text }
			},
			{
				scope: 'constant.language.boolean, storage.type, storage.modifier, storage.control',
				settings: { foreground: palette.purple }
			},
			{
				scope: 'constant.numeric, keyword.other, variable.object.property, meta.field.declaration entity.name.function, meta.definition.method entity.name.function, entity.name.tag.yaml, meta.object-literal.key, meta.object-literal.key string, support.type.property-name.json, entity.other.attribute-name.id, entity.name.tag',
				settings: { foreground: palette.warning }
			},
			{
				scope: 'keyword, modifier, variable.language.this, support.type.object, constant.language, punctuation, constant.language.json, markup.heading, text.html.markdown beginning.punctuation.definition.list, template.expression.begin, template.expression.end, punctuation.definition.template-expression.begin, punctuation.definition.template-expression.end',
				settings: { foreground: palette.info }
			},
			{
				scope: 'entity.name.function, support.function, meta.function entity.name.function, source.cs meta.method-call meta.method, source.cs entity.name.function, source.python meta.function-call.python, meta.function-call.arguments, entity.name.function.call',
				settings: { foreground: isDark ? '#93c5fd' : '#1d4ed8' }
			},
			{
				scope: 'support.type, constant.other.key, entity.name.type, entity.other.inherited-class, entity.other, entity.name, entity.name.type.class, support.class, meta.use, entity.other.attribute-name.class, source.css entity.name.tag, source.cs meta.class.identifier storage.type, source.cs storage.type, source.cs meta.method.return-type, support.class.component',
				settings: { foreground: palette.warning }
			},
			{
				scope: 'comment, comment punctuation.definition.comment, string.quoted.docstring, source.cs meta.preprocessor',
				settings: { fontStyle: 'italic', foreground: palette.comment }
			},
			{
				scope: 'support.module, support.node, constant.keyword, keyword.control, markup.quote',
				settings: { fontStyle: 'italic', foreground: palette.info }
			},
			{
				scope: 'meta.embedded, source.groovy.embedded, meta.template.expression, meta.jsx.children, SXNested, source.cpp meta.block variable.other, source.php support.other.namespace, source.php meta.use support.class, variable, variable.parameter, support.variable, variable.language, support.constant, meta.definition.variable entity.name.function',
				settings: { foreground: palette.text }
			},
			{
				scope: 'entity.other.attribute-name',
				settings: { foreground: palette.purple }
			},
			{
				scope: 'support.type.property-name.css',
				settings: { foreground: palette.secondaryText }
			},
			{
				scope: 'markup.inline.raw.string.markdown, markup.fenced_code.block.markdown punctuation.definition.markdown',
				settings: { foreground: palette.success }
			},
			{
				scope: 'markup.italic',
				settings: { fontStyle: 'italic', foreground: palette.warning }
			},
			{
				scope: 'markup.bold',
				settings: { fontStyle: 'bold', foreground: palette.warning }
			},
			{
				scope: 'markup.bold markup.italic, markup.italic markup.bold',
				settings: { fontStyle: 'italic bold', foreground: palette.warning }
			},
			{
				scope: 'variable.parameter, variable.parameter.function.language.special.self.python',
				settings: { fontStyle: 'italic', foreground: palette.text }
			},
			{
				scope: 'markup.inserted',
				settings: { foreground: palette.success }
			},
			{
				scope: 'markup.deleted',
				settings: { foreground: palette.error }
			},
			{
				scope: 'markup.underline',
				settings: { fontStyle: 'underline' }
			}
		],
		type: isDark ? 'dark' : 'light'
	};
};

const FALLBACK_SURFACE = {
	background: '#ffffff',
	foreground: '#111827'
};

let shikiModulePromise: Promise<typeof import('shiki')> | null = null;
const SHIKI_LANGUAGE_ALIASES: Record<string, string> = {
	svg: 'xml'
};

const loadShikiModule = async () => {
	if (!shikiModulePromise) {
		shikiModulePromise = import('shiki');
	}

	return await shikiModulePromise;
};

export const normalizeHighlighterTheme = (value: string | null | undefined) =>
	value && HIGHLIGHTER_THEME_IDS.has(value) ? value : DEFAULT_HIGHLIGHTER_THEME;

export const normalizeMermaidTheme = (value: string | null | undefined): MermaidThemeId =>
	value && MERMAID_THEME_IDS.has(value as MermaidThemeId)
		? (value as MermaidThemeId)
		: DEFAULT_MERMAID_THEME;

export const resolveShikiLanguage = async (input: string | null | undefined) => {
	const shiki = await loadShikiModule();
	const normalized = String(input ?? '').trim().toLowerCase();
	if (!normalized) return 'plaintext';
	const canonical = SHIKI_LANGUAGE_ALIASES[normalized] ?? normalized;

	const matched = shiki.bundledLanguagesInfo.find(
		(item) => item.id === canonical || item.aliases?.includes(canonical)
	);

	return matched?.id ?? 'plaintext';
};

const resolveThemeRegistration = (themeId: string, isDark: boolean) =>
	normalizeHighlighterTheme(themeId) === DEFAULT_HIGHLIGHTER_THEME
		? createLobeShikiTheme(isDark)
		: normalizeHighlighterTheme(themeId);

export const ensureShikiHighlighter = async (language: string, themeId: string, isDark: boolean) => {
	const shiki = await loadShikiModule();
	const resolvedLanguage = await resolveShikiLanguage(language);
	const resolvedTheme = resolveThemeRegistration(themeId, isDark);

	return await shiki.getSingletonHighlighter({
		langs: [resolvedLanguage],
		themes: [resolvedTheme as any]
	});
};

export const getShikiSurfaceColors = async (themeId: string, isDark: boolean) => {
	try {
		const highlighter = await ensureShikiHighlighter('plaintext', themeId, isDark);
		const resolvedThemeId = getRuntimeHighlighterThemeId(themeId, isDark);
		const theme = highlighter.getTheme(resolvedThemeId);

		return {
			background: theme?.bg ?? getPalette(isDark).background,
			foreground: theme?.fg ?? getPalette(isDark).primaryText
		};
	} catch {
		return {
			background: getPalette(isDark).background,
			foreground: getPalette(isDark).primaryText
		};
	}
};

export const renderCodeToHtml = async (params: {
	code: string;
	isDark: boolean;
	language: string;
	themeId: string;
}) => {
	const { code, isDark, language, themeId } = params;
	const highlighter = await ensureShikiHighlighter(language, themeId, isDark);
	const resolvedLanguage = await resolveShikiLanguage(language);
	const resolvedThemeId = getRuntimeHighlighterThemeId(themeId, isDark);

	return await highlighter.codeToHtml(code, {
		lang: resolvedLanguage,
		theme: resolvedThemeId
	});
};

export const createMermaidConfig = (
	themeId: MermaidThemeId,
	isDark: boolean
): MermaidConfig => {
	const palette = getPalette(isDark);
	const normalizedTheme = normalizeMermaidTheme(themeId);
	const isLobeTheme = normalizedTheme === DEFAULT_MERMAID_THEME;

	return {
		fontFamily: FONT_FAMILY,
		gantt: { useWidth: 1920 },
		securityLevel: 'loose',
		startOnLoad: false,
		theme: isLobeTheme ? (isDark ? 'dark' : 'neutral') : normalizedTheme,
		themeVariables: isLobeTheme
			? {
					errorBkgColor: palette.comment,
					errorTextColor: palette.comment,
					fontFamily: FONT_FAMILY,
					lineColor: palette.secondaryText,
					mainBkg: palette.background,
					noteBkgColor: palette.infoBg,
					noteTextColor: palette.infoText,
					pie1: palette.info,
					pie2: palette.warning,
					pie3: palette.success,
					pie4: palette.error,
					primaryBorderColor: palette.border,
					primaryColor: palette.background,
					primaryTextColor: palette.text,
					secondaryBorderColor: palette.infoBorder,
					secondaryColor: palette.infoBg,
					secondaryTextColor: palette.infoText,
					tertiaryBorderColor: palette.successBorder,
					tertiaryColor: palette.successBg,
					tertiaryTextColor: palette.successText,
					textColor: palette.text
				}
			: undefined
	};
};

export const renderMermaidSvg = async (params: {
	code: string;
	id: string;
	isDark: boolean;
	themeId: MermaidThemeId;
}) => {
	const { code, id, isDark, themeId } = params;
	const mermaidModule = await import('mermaid');
	const mermaid = mermaidModule.default;

	mermaid.initialize(createMermaidConfig(themeId, isDark));

	if (await mermaid.parse(code)) {
		const { svg } = await mermaid.render(id, code);
		return svg;
	}

	return '';
};

export const getEditorChromeTheme = async (themeId: string, isDark: boolean) => {
	const palette = getPalette(isDark);
	const surface =
		normalizeHighlighterTheme(themeId) === DEFAULT_HIGHLIGHTER_THEME
			? {
					background: palette.background,
					foreground: palette.primaryText
				}
			: await getShikiSurfaceColors(themeId, isDark).catch(() => FALLBACK_SURFACE);

	return {
		caret: surface.foreground,
		fontFamily: MONO_FONT_FAMILY,
		gutterForeground: isDark ? '#9ca3af' : '#6b7280',
		lineHighlight: isDark ? 'rgba(255,255,255,0.04)' : 'rgba(15,23,42,0.04)',
		selection: isDark ? 'rgba(96,165,250,0.22)' : 'rgba(37,99,235,0.16)',
		surface
	};
};

export const normalizeChatTransitionMode = (
	mode: unknown,
	fallback: ChatTransitionMode = DEFAULT_CHAT_TRANSITION_MODE
): ChatTransitionMode => {
	return mode === 'none' || mode === 'fadeIn' || mode === 'smooth' ? mode : fallback;
};

export const resolveChatTransitionMode = (
	settingsLike?: ChatTransitionModeSettingsLike | null,
	fallback: ChatTransitionMode = DEFAULT_CHAT_TRANSITION_MODE
): ChatTransitionMode => {
	if (!settingsLike) {
		return fallback;
	}

	if (settingsLike.transitionMode !== undefined && settingsLike.transitionMode !== null) {
		return normalizeChatTransitionMode(settingsLike.transitionMode, fallback);
	}

	if (settingsLike.chatFadeStreamingText === true) {
		return 'fadeIn';
	}

	if (settingsLike.chatFadeStreamingText === false) {
		return 'none';
	}

	return fallback;
};
