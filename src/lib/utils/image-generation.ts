import { parseModelSelectionId } from '$lib/utils/model-identity';

export type GeminiImageSizeOption = {
	value: string;
	label: string;
	pixels: string;
};

export type AspectRatioOption = {
	value: string;
	label: string;
};

export type NativeResolutionOption = {
	value: string;
	label: string;
};

export const GEMINI_IMAGE_SIZE_OPTIONS: GeminiImageSizeOption[] = [
	{ value: '512', label: '512', pixels: '512x512' },
	{ value: '1K', label: '1K', pixels: '1024x1024' },
	{ value: '2K', label: '2K', pixels: '2048x2048' },
	{ value: '4K', label: '4K', pixels: '4096x4096' }
];

export const IMAGE_ASPECT_RATIO_OPTIONS: AspectRatioOption[] = [
	{ value: '1:1', label: '1:1' },
	{ value: '2:3', label: '2:3' },
	{ value: '3:2', label: '3:2' },
	{ value: '3:4', label: '3:4' },
	{ value: '4:3', label: '4:3' },
	{ value: '4:5', label: '4:5' },
	{ value: '5:4', label: '5:4' },
	{ value: '9:16', label: '9:16' },
	{ value: '16:9', label: '16:9' },
	{ value: '21:9', label: '21:9' }
];

export const GROK_IMAGE_ASPECT_RATIO_OPTIONS: AspectRatioOption[] = [
	{ value: '1:1', label: '1:1' },
	{ value: '16:9', label: '16:9' },
	{ value: '9:16', label: '9:16' },
	{ value: '4:3', label: '4:3' },
	{ value: '3:4', label: '3:4' },
	{ value: '3:2', label: '3:2' },
	{ value: '2:3', label: '2:3' },
	{ value: '2:1', label: '2:1' },
	{ value: '1:2', label: '1:2' },
	{ value: '19.5:9', label: '19.5:9' },
	{ value: '9:19.5', label: '9:19.5' },
	{ value: '20:9', label: '20:9' },
	{ value: '9:20', label: '9:20' },
	{ value: 'auto', label: 'auto' }
];

export const GROK_IMAGE_RESOLUTION_OPTIONS: NativeResolutionOption[] = [
	{ value: '1k', label: '1K' },
	{ value: '2k', label: '2K' }
];

const LEGACY_SIZE_TO_GEMINI_IMAGE_SIZE: Record<string, string> = {
	'512x512': '512',
	'1024x1024': '1K',
	'2048x2048': '2K',
	'4096x4096': '4K'
};

const LEGACY_SIZE_TO_ASPECT_RATIO: Record<string, string> = {
	'512x512': '1:1',
	'1024x1024': '1:1',
	'1024x1536': '2:3',
	'1536x1024': '3:2'
};

export const normalizeGeminiImageSize = (value: unknown): string | null => {
	const normalized = `${value ?? ''}`.trim().toUpperCase();
	return GEMINI_IMAGE_SIZE_OPTIONS.some((option) => option.value === normalized) ? normalized : null;
};

export const normalizeAspectRatio = (value: unknown): string | null => {
	const normalized = `${value ?? ''}`.trim();
	return IMAGE_ASPECT_RATIO_OPTIONS.some((option) => option.value === normalized) ? normalized : null;
};

export const normalizeGrokAspectRatio = (value: unknown): string | null => {
	const normalized = `${value ?? ''}`.trim();
	return GROK_IMAGE_ASPECT_RATIO_OPTIONS.some((option) => option.value === normalized)
		? normalized
		: null;
};

export const normalizeGrokResolution = (value: unknown): string | null => {
	const normalized = `${value ?? ''}`.trim().toLowerCase();
	return GROK_IMAGE_RESOLUTION_OPTIONS.some((option) => option.value === normalized)
		? normalized
		: null;
};

export const mapLegacySizeToGeminiParams = (size: unknown): {
	imageSize: string | null;
	aspectRatio: string | null;
} => {
	const normalized = `${size ?? ''}`.trim().toLowerCase();
	if (!normalized) {
		return { imageSize: null, aspectRatio: null };
	}

	const imageSize = LEGACY_SIZE_TO_GEMINI_IMAGE_SIZE[normalized] ?? null;
	const presetAspectRatio = LEGACY_SIZE_TO_ASPECT_RATIO[normalized];
	if (presetAspectRatio) {
		return { imageSize, aspectRatio: presetAspectRatio };
	}

	const match = normalized.match(/^(\d+)x(\d+)$/);
	if (!match) {
		return { imageSize, aspectRatio: null };
	}

	const width = Number(match[1]);
	const height = Number(match[2]);
	if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) {
		return { imageSize, aspectRatio: null };
	}

	const gcd = (a: number, b: number): number => (b === 0 ? a : gcd(b, a % b));
	const divisor = gcd(width, height);
	return {
		imageSize,
		aspectRatio: divisor > 0 ? `${width / divisor}:${height / divisor}` : null
	};
};

export const getFunctionPipeRootId = (modelId: unknown): string => {
	const raw = `${modelId ?? ''}`.trim();
	if (!raw) return '';
	const parsed = parseModelSelectionId(raw);
	if (parsed?.provider === 'pipe') {
		const connectionId =
			`${parsed.modelRef?.connection_id ?? parsed.modelRef?.prefix_id ?? ''}`.trim();
		if (connectionId) return connectionId;
	}
	return raw.includes('.') ? raw.split('.', 1)[0] : raw;
};

export const looksLikeImageValveSpec = (schema: any): boolean => {
	const properties = schema?.properties ?? {};
	return Boolean(properties?.image_size || properties?.aspect_ratio || properties?.resolution);
};

export const getImageValveProperty = (
	schema: any,
	key: 'image_size' | 'aspect_ratio' | 'resolution'
) =>
	schema?.properties?.[key] ?? null;

export const getPropertyEnumOptions = (
	property: any,
	fallback: Array<{ value: string; label: string }>
) => {
	const enumValues = Array.isArray(property?.enum) ? property.enum : [];
	if (enumValues.length > 0) {
		return enumValues.map((value: unknown) => ({
			value: `${value ?? ''}`,
			label: `${value ?? ''}`
		}));
	}

	return fallback;
};

export const modelSupportsNativeImageOptions = (model: any): boolean =>
	Boolean(
		model &&
			((model?.supports_image_size ?? false) ||
				(model?.supports_resolution ?? false) ||
				model?.size_mode === 'aspect_ratio' ||
				model?.generation_mode === 'gemini_generate_content_image' ||
				model?.generation_mode === 'xai_images')
	);

export const modelSupportsGeminiImageOptions = (model: any): boolean =>
	Boolean(
		model &&
			((model?.supports_image_size ?? false) ||
				model?.generation_mode === 'gemini_generate_content_image')
	);

export type OpenAIImageRouteOption = {
	value: string;
	label: string;
	disabled?: boolean;
	description?: string;
};

export const getOpenAIImageRouteOptions = (
	model: any | null,
	tr: (zh: string, en: string) => string
): OpenAIImageRouteOption[] => {
	const routes = Array.isArray(model?.supported_image_routes) ? model.supported_image_routes : [];
	const hasRoute = (route: string) => routes.includes(route);
	const isChatImageModel = model?.generation_mode === 'openai_chat_image';
	const hasConversationalRoute = hasRoute('chat') || hasRoute('responses') || isChatImageModel;
	const hasPlainRoute = hasRoute('generations') || hasConversationalRoute;
	const options: OpenAIImageRouteOption[] = hasRoute('generations')
		? [
				{
					value: 'generations',
					label: tr('普通生图', 'Default'),
					description: tr(
						'走 generations，按文字生成新图；不会把参考图当成要修改的原图。',
						'Uses generations to create a new image from text; reference images are not treated as the source image to edit.'
					)
				}
			]
		: !hasPlainRoute && hasRoute('edits')
			? [
					{
						value: '',
						label: tr('请选择接口', 'Select route'),
						disabled: true,
						description: tr(
							'该模型只有 edits，需要提供当前消息或历史上下文里的参考图。',
							'This model only supports edits and needs a reference image from the current message or chat history.'
						)
					}
				]
			: [];

	if (hasConversationalRoute) {
		const conversationalRoute =
			model?.default_image_route === 'responses' && hasRoute('responses')
				? 'responses'
				: hasRoute('chat') || isChatImageModel
					? 'chat'
					: 'responses';
		options.push({
			value: conversationalRoute,
			label: tr('对话图片', 'Chat Image'),
			description: tr(
				'走 chat 或 responses，把文字和图片一起发给支持对话生图的模型。',
				'Uses chat or responses to send text and images together to models that support conversational image generation.'
			)
		});
	}
	if (hasRoute('edits')) {
		options.push({
			value: 'edits',
			label: tr('编辑接口', 'Edit'),
			description: tr(
				'走 edits，用当前消息或历史上下文里的参考图改图；适合换风格、局部修改、重绘。',
				'Uses edits to modify a reference image from the current message or chat history; useful for style changes, local edits, and redraws.'
			)
		});
	}

	return options;
};

export const getBuiltinImageEngine = (model: any | null): '' | 'openai' | 'gemini' | 'grok' => {
	const provider = `${model?.provider ?? ''}`.trim().toLowerCase();
	if (provider === 'gemini' || provider === 'grok') {
		return provider;
	}

	const generationMode = `${model?.generation_mode ?? ''}`.trim().toLowerCase();
	if (generationMode.startsWith('openai_') || provider === 'openai') {
		return 'openai';
	}
	if (generationMode.startsWith('gemini_')) {
		return 'gemini';
	}
	if (generationMode === 'xai_images') {
		return 'grok';
	}
	return '';
};
