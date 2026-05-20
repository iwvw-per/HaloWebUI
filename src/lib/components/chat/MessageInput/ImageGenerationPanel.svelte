<script lang="ts">
	import { DropdownMenu } from 'bits-ui';
	import { getContext, tick } from 'svelte';
	import { toast } from 'svelte-sonner';

	import { flyAndScale } from '$lib/utils/transitions';
	import { translateWithDefault } from '$lib/i18n';
	import type { Model } from '$lib/stores';
	import { getImageGenerationModels, type ImageGenerationModel } from '$lib/apis/images';
	import {
		getUserValvesById as getFunctionUserValvesById,
		getUserValvesSpecById as getFunctionUserValvesSpecById,
		updateUserValvesById as updateFunctionUserValvesById
	} from '$lib/apis/functions';
	import {
		GEMINI_IMAGE_SIZE_OPTIONS,
		GROK_IMAGE_ASPECT_RATIO_OPTIONS,
		GROK_IMAGE_RESOLUTION_OPTIONS,
		IMAGE_ASPECT_RATIO_OPTIONS,
		getBuiltinImageEngine,
		getFunctionPipeRootId,
		getImageValveProperty,
		getOpenAIImageRouteOptions,
		getPropertyEnumOptions,
		looksLikeImageValveSpec,
		modelSupportsNativeImageOptions
	} from '$lib/utils/image-generation';
	import {
		findModelByIdentity,
		findModelByRef,
		getModelCleanId,
		getModelRef
	} from '$lib/utils/model-identity';

	import Switch from '$lib/components/common/Switch.svelte';
	import Tooltip from '$lib/components/common/Tooltip.svelte';
	import Spinner from '$lib/components/common/Spinner.svelte';
	import {
		Check,
		CircleHelp,
		Image as ImageIcon,
		SlidersHorizontal,
		Sparkles,
		Wand2,
		X
	} from 'lucide-svelte';

	const i18n = getContext('i18n');
	const tr = (zh: string, en: string, options: Record<string, any> = {}) =>
		translateWithDefault($i18n, zh, en, options);

	type ImageGenerationOptions = {
		size?: string | null;
		image_size?: string | null;
		aspect_ratio?: string | null;
		resolution?: string | null;
		n?: number | null;
		negative_prompt?: string | null;
		steps?: number | null;
		background?: string | null;
		image_route_mode?: string | null;
	};

	type RatioPreset = {
		id: string;
		labelZh: string;
		labelEn: string;
		ratio: string | null;
		size: string | null;
		preview: string;
	};

	type PromptPreset = {
		id: string;
		labelZh: string;
		labelEn: string;
		descriptionZh: string;
		descriptionEn: string;
		promptZh: string;
		promptEn: string;
	};

	export let open = false;
	export let prompt = '';
	export let imageGenerationEnabled = false;
	export let imageGenerationOptions: ImageGenerationOptions = {};
	export let currentModel: Model | null = null;
	export let hasReferenceImage = false;

	let builtinLoading = false;
	let builtinReady = false;
	let builtinEngine: '' | 'openai' | 'gemini' | 'grok' = '';
	let builtinModelMeta: ImageGenerationModel | null = null;
	let builtinRequestKey = '';
	let currentModelIdentity = '';
	let currentModelImageHint = '';
	let currentModelSourceSignature = '';
	let openAIRouteTouchedKey = '';

	let customLoading = false;
	let customFunctionId = '';
	let customValvesSpec: Record<string, any> | null = null;
	let customValves: Record<string, any> = {};
	let customHasImageFields = false;
	let customLoadedKey = '';

	const ratioPresets: RatioPreset[] = [
		{
			id: 'auto',
			labelZh: '自动',
			labelEn: 'Auto',
			ratio: null,
			size: null,
			preview: 'aspect-[4/3]'
		},
		{
			id: 'square',
			labelZh: '方形',
			labelEn: 'Square',
			ratio: '1:1',
			size: '1024x1024',
			preview: 'aspect-square'
		},
		{
			id: 'portrait',
			labelZh: '竖版',
			labelEn: 'Portrait',
			ratio: '2:3',
			size: '1024x1536',
			preview: 'aspect-[2/3]'
		},
		{
			id: 'story',
			labelZh: '故事版',
			labelEn: 'Story',
			ratio: '9:16',
			size: null,
			preview: 'aspect-[9/16]'
		},
		{
			id: 'landscape',
			labelZh: '横版',
			labelEn: 'Landscape',
			ratio: '3:2',
			size: '1536x1024',
			preview: 'aspect-[3/2]'
		},
		{
			id: 'wide',
			labelZh: '宽屏',
			labelEn: 'Wide',
			ratio: '16:9',
			size: null,
			preview: 'aspect-[16/9]'
		}
	];

	const stylePresets: PromptPreset[] = [
		{
			id: 'photo',
			labelZh: '摄影',
			labelEn: 'Photo',
			descriptionZh: '真实光线和镜头质感',
			descriptionEn: 'Natural light and camera realism',
			promptZh: '写实摄影风格，自然光线，主体清晰，细节真实',
			promptEn: 'photorealistic photography, natural light, clear subject, realistic details'
		},
		{
			id: 'product',
			labelZh: '商品图',
			labelEn: 'Product',
			descriptionZh: '干净背景，适合展示',
			descriptionEn: 'Clean background for showcase',
			promptZh: '高级商品摄影，干净背景，柔和棚拍光，商业质感',
			promptEn:
				'premium product photography, clean background, soft studio lighting, commercial quality'
		},
		{
			id: 'illustration',
			labelZh: '插画',
			labelEn: 'Illustration',
			descriptionZh: '温和色彩和手绘感',
			descriptionEn: 'Soft color and drawn texture',
			promptZh: '精致插画风格，柔和配色，细腻线条，画面有故事感',
			promptEn: 'polished illustration style, soft colors, fine linework, narrative composition'
		},
		{
			id: 'poster',
			labelZh: '海报',
			labelEn: 'Poster',
			descriptionZh: '强构图，适合宣传',
			descriptionEn: 'Bold composition for promotion',
			promptZh: '现代海报设计，强视觉中心，清晰层次，高级排版感',
			promptEn: 'modern poster design, strong focal point, clear hierarchy, premium layout'
		},
		{
			id: 'icon',
			labelZh: '图标',
			labelEn: 'Icon',
			descriptionZh: '简洁形状，适合界面',
			descriptionEn: 'Simple shapes for UI',
			promptZh: '简洁图标设计，现代扁平风格，边缘清晰，适合应用界面',
			promptEn: 'clean icon design, modern flat style, crisp edges, suitable for app UI'
		},
		{
			id: 'sticker',
			labelZh: '贴纸',
			labelEn: 'Sticker',
			descriptionZh: '透明感边缘，适合贴纸',
			descriptionEn: 'Cutout style with clean edges',
			promptZh: '贴纸设计风格，主体可爱突出，边缘干净，适合透明背景',
			promptEn:
				'sticker design style, cute prominent subject, clean edges, suitable for transparent background'
		}
	];

	const ideaPresets: PromptPreset[] = [
		{
			id: 'brand-visual',
			labelZh: '品牌主图',
			labelEn: 'Brand Hero',
			descriptionZh: '适合官网或产品首屏',
			descriptionEn: 'For landing pages or products',
			promptZh: '生成一张高级品牌主视觉，主体明确，空间干净，适合官网首屏展示',
			promptEn:
				'Create a premium brand hero visual with a clear subject, clean space, suitable for a website hero section'
		},
		{
			id: 'social-post',
			labelZh: '社媒配图',
			labelEn: 'Social Post',
			descriptionZh: '有冲击力，适合发布',
			descriptionEn: 'Eye-catching and shareable',
			promptZh: '生成一张适合社交媒体发布的图片，视觉冲击强，主题一眼清楚',
			promptEn:
				'Create an eye-catching social media image with a clear theme and strong visual impact'
		},
		{
			id: 'concept-art',
			labelZh: '概念图',
			labelEn: 'Concept Art',
			descriptionZh: '氛围完整，适合创意探索',
			descriptionEn: 'Atmospheric creative exploration',
			promptZh: '生成一张概念设计图，氛围完整，细节丰富，画面有探索感',
			promptEn:
				'Create a concept art image with a complete atmosphere, rich details, and a sense of exploration'
		}
	];

	const backgroundOptions = [
		{ value: '', labelZh: '自动', labelEn: 'Auto' },
		{ value: 'transparent', labelZh: '透明', labelEn: 'Transparent' },
		{ value: 'white', labelZh: '白色', labelEn: 'White' },
		{ value: 'black', labelZh: '黑色', labelEn: 'Black' }
	];

	const countOptions = [1, 2, 3, 4];
	const helpIconClass = 'size-3 shrink-0 cursor-help text-gray-400 dark:text-gray-500';

	const cleanValue = (value: unknown) =>
		typeof value === 'string' || typeof value === 'number' ? `${value}`.trim() : '';

	const getCurrentModelIdentity = () => {
		const model = currentModel as any;
		return `${model?.selection_id ?? model?.selectionId ?? model?.id ?? ''}`.trim();
	};

	const getCurrentModelImageHint = () => {
		const model = currentModel as any;
		return `${model?.info?.meta?.base_selection_id || model?.info?.base_model_id || getModelCleanId(model) || getCurrentModelIdentity()}`.trim();
	};

	const getCurrentModelSourceSignature = () => {
		const modelRef = getModelRef(currentModel as any);
		if (!modelRef) return '';
		return [
			cleanValue(modelRef.provider).toLowerCase(),
			cleanValue(modelRef.source ?? modelRef.effective_source),
			cleanValue(modelRef.connection_id ?? modelRef.prefix_id),
			cleanValue(modelRef.connection_index)
		].join(':');
	};

	const modelMatchesCurrentImageSource = (
		model: ImageGenerationModel,
		modelRef: Record<string, unknown> | null
	) => {
		if (!modelRef) return true;
		const candidateRef = getModelRef(model as any);
		const provider = cleanValue(modelRef.provider).toLowerCase();
		const candidateProvider = cleanValue(candidateRef?.provider ?? model.provider).toLowerCase();
		if (provider && candidateProvider && provider !== candidateProvider) return false;

		const source = cleanValue(modelRef.source ?? modelRef.effective_source);
		const candidateSource = cleanValue(candidateRef?.source ?? model.source);
		if (source && candidateSource && source !== candidateSource) return false;

		const connectionId = cleanValue(modelRef.connection_id ?? modelRef.prefix_id);
		if (connectionId) {
			const candidateConnectionId = cleanValue(
				candidateRef?.connection_id ?? candidateRef?.prefix_id
			);
			return candidateConnectionId === connectionId;
		}

		const connectionIndex = cleanValue(modelRef.connection_index);
		if (connectionIndex) {
			return (
				cleanValue(candidateRef?.connection_index ?? model.connection_index) === connectionIndex
			);
		}

		return true;
	};

	const loadBuiltinContext = async () => {
		const imageModelHint = getCurrentModelImageHint();
		const requestKey =
			open || imageGenerationEnabled
				? `enabled:${currentModelIdentity}:${imageModelHint}:${currentModelSourceSignature}`
				: 'disabled';
		if (requestKey === builtinRequestKey) return;

		builtinRequestKey = requestKey;
		if (!open && !imageGenerationEnabled) {
			builtinEngine = '';
			builtinModelMeta = null;
			builtinReady = false;
			return;
		}

		builtinReady = false;
		builtinEngine = '';
		builtinModelMeta = null;
		builtinLoading = true;
		try {
			const runtimeModels = await getImageGenerationModels(localStorage.token, {
				context: 'runtime'
			}).catch(() => []);
			const preferredId = currentModelIdentity;
			const currentModelRef = getModelRef(currentModel as any);
			const currentModelCleanId = getModelCleanId(currentModel as any);
			const identityLookupModels = currentModelRef
				? (runtimeModels ?? []).filter((model) =>
						modelMatchesCurrentImageSource(model, currentModelRef)
					)
				: (runtimeModels ?? []);
			builtinModelMeta =
				findModelByRef(
					runtimeModels ?? [],
					currentModelRef,
					imageModelHint || currentModelCleanId || preferredId
				) ??
				(preferredId ? findModelByIdentity(identityLookupModels, preferredId) : undefined) ??
				(imageModelHint ? findModelByIdentity(identityLookupModels, imageModelHint) : undefined) ??
				(currentModelCleanId
					? findModelByIdentity(identityLookupModels, currentModelCleanId)
					: undefined) ??
				null;
			builtinEngine = getBuiltinImageEngine(builtinModelMeta);
			builtinReady = true;
		} catch (error) {
			console.error('Failed to load native image context', error);
			builtinModelMeta = null;
			builtinEngine = '';
			builtinReady = true;
		} finally {
			builtinLoading = false;
		}
	};

	const loadCustomContext = async () => {
		const nextFunctionId =
			currentModel?.pipe && (currentModel as any)?.has_user_valves
				? getFunctionPipeRootId(currentModel?.id)
				: '';

		if (nextFunctionId === customLoadedKey) return;
		customLoadedKey = nextFunctionId;
		customFunctionId = nextFunctionId;
		customValvesSpec = null;
		customValves = {};
		customHasImageFields = false;

		if (!nextFunctionId) return;

		customLoading = true;
		try {
			const [nextValves, nextSpec] = await Promise.all([
				getFunctionUserValvesById(localStorage.token, nextFunctionId),
				getFunctionUserValvesSpecById(localStorage.token, nextFunctionId)
			]);

			customValvesSpec = nextSpec;
			customValves = { ...(nextValves ?? {}) };
			customHasImageFields = looksLikeImageValveSpec(nextSpec);

			if (customHasImageFields) {
				const nextImageSizeProperty = getImageValveProperty(nextSpec, 'image_size');
				const nextAspectRatioProperty = getImageValveProperty(nextSpec, 'aspect_ratio');
				if (customValves.image_size == null && nextImageSizeProperty?.default != null) {
					customValves.image_size = `${nextImageSizeProperty.default}`;
				}
				if (customValves.aspect_ratio == null && nextAspectRatioProperty?.default != null) {
					customValves.aspect_ratio = `${nextAspectRatioProperty.default}`;
				}
			}
		} catch (error) {
			console.error('Failed to load custom image valves', error);
			customHasImageFields = false;
			customValvesSpec = null;
		} finally {
			customLoading = false;
		}
	};

	const saveCustomValves = async (patch: Record<string, any>) => {
		if (!customFunctionId) return;
		const nextValves = { ...customValves, ...patch };
		customValves = nextValves;
		try {
			const res = await updateFunctionUserValvesById(
				localStorage.token,
				customFunctionId,
				nextValves
			);
			customValves = res ?? nextValves;
		} catch (error) {
			toast.error(`${error}`);
		}
	};

	const findOptionLabel = (
		options: Array<{ value: string; label: string }>,
		value: string | null | undefined,
		fallback?: string
	) => {
		if (value == null) return fallback ?? options[0]?.label ?? '';
		return (
			options.find((option) => `${option.value}` === `${value}`)?.label ?? fallback ?? `${value}`
		);
	};

	const setOption = (patch: ImageGenerationOptions) => {
		imageGenerationOptions = { ...imageGenerationOptions, ...patch };
		imageGenerationEnabled = true;
	};

	const clearOptions = (...keys: Array<keyof ImageGenerationOptions>) => {
		const next = { ...imageGenerationOptions };
		for (const key of keys) {
			next[key] = null;
		}
		imageGenerationOptions = next;
	};

	const getLocalePrefersChinese = () => {
		const language = `${$i18n?.language ?? ''}`.toLowerCase();
		if (language) {
			return language.startsWith('zh') || language.includes('hans') || language.includes('hant');
		}
		const locale =
			typeof localStorage !== 'undefined'
				? `${localStorage.getItem('locale') ?? ''}`.toLowerCase()
				: '';
		return locale.startsWith('zh') || locale.includes('hans') || locale.includes('hant');
	};

	const promptPrefersChinese = () => {
		const text = `${prompt ?? ''}`.trim();
		if (/[\u3400-\u9fff]/.test(text)) return true;
		if (/[a-zA-Z]/.test(text)) return false;
		return getLocalePrefersChinese();
	};

	const normalizePromptText = (value: string) => value.toLowerCase().replace(/\s+/g, '');

	const applyPromptPreset = async (preset: PromptPreset) => {
		const useChinese = promptPrefersChinese();
		const selectedText = useChinese ? preset.promptZh : preset.promptEn;
		const existing = `${prompt ?? ''}`.trim();
		const normalizedExisting = normalizePromptText(existing);
		const normalizedZh = normalizePromptText(preset.promptZh);
		const normalizedEn = normalizePromptText(preset.promptEn);

		if (normalizedExisting.includes(normalizedZh) || normalizedExisting.includes(normalizedEn)) {
			imageGenerationEnabled = true;
			return;
		}

		if (!existing) {
			prompt = selectedText;
		} else {
			const separator = useChinese ? '，' : ', ';
			prompt = `${existing}${separator}${selectedText}`;
		}

		imageGenerationEnabled = true;
		await tick();
		document.getElementById('chat-input')?.focus();
	};

	const handleOpenChange = (state: boolean) => {
		open = state;
		if (state) {
			imageGenerationEnabled = true;
		}
	};

	const setOpenAIRoute = (value: string) => {
		openAIRouteTouchedKey = openAIRouteContextKey;
		setOption({ image_route_mode: value || null });
	};

	const handleStepsInput = (event: Event) => {
		const target = event.currentTarget;
		if (target instanceof HTMLInputElement && canUseSteps) {
			setOption({ steps: Number(target.value) || null });
		}
	};

	const handleNegativePromptInput = (event: Event) => {
		const target = event.currentTarget;
		if (target instanceof HTMLTextAreaElement && canUseNegativePrompt) {
			setOption({ negative_prompt: target.value || null });
		}
	};

	const handleCustomValveChange =
		(key: 'image_size' | 'aspect_ratio' | 'resolution') => (event: Event) => {
			const target = event.currentTarget;
			if (target instanceof HTMLSelectElement) {
				void saveCustomValves({ [key]: target.value });
			}
		};

	const selectRatioPreset = (preset: RatioPreset) => {
		if (!preset.ratio) {
			clearOptions('size', 'aspect_ratio');
			imageGenerationEnabled = true;
			return;
		}

		if (canUseAspectRatioControl) {
			setOption({ aspect_ratio: preset.ratio, size: null });
			return;
		}

		if (canUseExactSizeControl && preset.size) {
			setOption({ size: preset.size, aspect_ratio: null });
		}
	};

	const optionDisabledClass = 'opacity-45 cursor-not-allowed grayscale';

	$: {
		currentModel;
		currentModelIdentity = getCurrentModelIdentity();
		currentModelImageHint = getCurrentModelImageHint();
		currentModelSourceSignature = getCurrentModelSourceSignature();
	}

	$: openAIRouteContextKey = `${currentModelIdentity}:${currentModelImageHint}:${currentModelSourceSignature}`;
	$: if (openAIRouteTouchedKey && openAIRouteTouchedKey !== openAIRouteContextKey) {
		openAIRouteTouchedKey = '';
	}

	$: if (open || imageGenerationEnabled) {
		currentModelIdentity;
		currentModelImageHint;
		currentModelSourceSignature;
		void loadBuiltinContext();
	} else if (builtinRequestKey !== 'disabled') {
		void loadBuiltinContext();
	}

	$: {
		currentModel;
		void loadCustomContext();
	}

	$: hasCustomImage = Boolean(customFunctionId) && customHasImageFields;
	$: hasBuiltinImage =
		builtinReady &&
		(['openai', 'gemini', 'grok'] as const).includes(builtinEngine as any) &&
		Boolean(builtinModelMeta) &&
		(builtinEngine === 'openai' || modelSupportsNativeImageOptions(builtinModelMeta));
	$: loading = builtinLoading || customLoading;
	$: openaiRouteOptions = getOpenAIImageRouteOptions(builtinModelMeta, tr);

	const getAutoOpenAIRouteValue = (options: Array<{ value: string }>) => {
		if (hasReferenceImage) {
			const referenceDefaultRoute =
				`${builtinModelMeta?.reference_image_default_route ?? ''}`.trim();
			if (
				referenceDefaultRoute &&
				options.some((option) => option.value === referenceDefaultRoute)
			) {
				return referenceDefaultRoute;
			}
			for (const route of ['chat', 'responses', 'edits']) {
				if (options.some((option) => option.value === route)) return route;
			}
		}
		return 'generations';
	};

	$: autoOpenAIRouteValue = getAutoOpenAIRouteValue(openaiRouteOptions);
	$: openaiRouteSelectedValue = (() => {
		const current = `${imageGenerationOptions?.image_route_mode ?? 'auto'}`;
		if (current === 'auto') {
			return openaiRouteOptions.some((option) => option.value === autoOpenAIRouteValue)
				? autoOpenAIRouteValue
				: (openaiRouteOptions[0]?.value ?? '');
		}
		return openaiRouteOptions.some((option) => option.value === current)
			? current
			: (openaiRouteOptions[0]?.value ?? '');
	})();

	$: if (
		builtinEngine === 'openai' &&
		imageGenerationOptions?.image_route_mode &&
		openaiRouteOptions.length > 0
	) {
		const selected = `${imageGenerationOptions.image_route_mode}`.trim();
		if (!openaiRouteOptions.some((option) => option.value === selected)) {
			imageGenerationOptions = { ...imageGenerationOptions, image_route_mode: null };
		}
	}

	$: if (
		builtinEngine === 'openai' &&
		hasReferenceImage &&
		openAIRouteTouchedKey !== openAIRouteContextKey &&
		`${imageGenerationOptions?.image_route_mode ?? ''}`.trim() === 'generations' &&
		openaiRouteOptions.some((option) => ['chat', 'responses', 'edits'].includes(option.value))
	) {
		imageGenerationOptions = { ...imageGenerationOptions, image_route_mode: null };
	}

	$: hasOpenAIRouteChoice =
		hasBuiltinImage && builtinEngine === 'openai' && openaiRouteOptions.length > 0;
	$: hasBuiltinSizeOption =
		hasBuiltinImage &&
		(builtinEngine === 'gemini' || builtinEngine === 'grok') &&
		Boolean(builtinModelMeta?.supports_image_size);
	$: hasBuiltinResolutionOption = hasBuiltinImage && Boolean(builtinModelMeta?.supports_resolution);
	$: hasBuiltinAspectOption =
		hasBuiltinImage &&
		(builtinModelMeta?.size_mode === 'aspect_ratio' || builtinModelMeta?.supports_image_size);
	$: openAIRouteUsesExactSize =
		builtinEngine !== 'openai' ||
		!hasOpenAIRouteChoice ||
		['generations', 'edits'].includes(openaiRouteSelectedValue);
	$: canUseAspectRatioControl = imageGenerationEnabled && !loading && hasBuiltinAspectOption;
	$: canUseExactSizeControl =
		imageGenerationEnabled &&
		!loading &&
		(!hasBuiltinImage ||
			(builtinEngine === 'openai' &&
				builtinModelMeta?.size_mode === 'exact' &&
				openAIRouteUsesExactSize));
	$: supportsBatch = !hasBuiltinImage || builtinModelMeta?.supports_batch !== false;
	$: canUseBatch = imageGenerationEnabled && !loading && supportsBatch;
	$: canUseBackground =
		imageGenerationEnabled &&
		!loading &&
		hasBuiltinImage &&
		Boolean(builtinModelMeta?.supports_background) &&
		openAIRouteUsesExactSize;
	$: canUseSteps = imageGenerationEnabled && !loading && !hasBuiltinImage;
	$: canUseNegativePrompt = imageGenerationEnabled && !loading && !hasBuiltinImage;
	$: canUseQuality =
		imageGenerationEnabled && !loading && (hasBuiltinResolutionOption || hasBuiltinSizeOption);
	$: qualityUnavailableLabel =
		builtinEngine === 'openai'
			? tr('不在此处选择', 'Set elsewhere')
			: tr('当前接口不支持', 'Not supported');
	$: qualityUnavailableDescription =
		builtinEngine === 'openai'
			? tr(
					'OpenAI 尺寸在左侧比例里选择；这里的档位只用于 Gemini/Grok。',
					'OpenAI size is selected from the aspect ratio presets on the left; this tier control is only used by Gemini/Grok.'
				)
			: tr(
					'当前接口没有暴露可选择的清晰度或尺寸档位。',
					'The current route does not expose selectable resolution or size tiers.'
				);
	$: qualityHelpDescription = canUseQuality
		? hasBuiltinResolutionOption
			? tr(
					'这里选择当前模型支持的清晰度档位。',
					'Select the resolution tier supported by the current model.'
				)
			: tr(
					'这里选择当前模型支持的尺寸档位。',
					'Select the size tier supported by the current model.'
				)
		: qualityUnavailableDescription;
	$: stepsUnavailableLabel = hasBuiltinImage
		? tr('当前接口不使用', 'Not used by this route')
		: tr('当前配置不支持', 'Not supported');

	$: builtinImageSizeOptions = GEMINI_IMAGE_SIZE_OPTIONS.map((option) => ({
		value: option.value,
		label: `${option.label} · ${option.pixels}`
	}));
	$: resolutionOptions = GROK_IMAGE_RESOLUTION_OPTIONS.map((option) => ({
		value: option.value,
		label: option.label
	}));
	$: customAspectRatioFallback = Array.from(
		new Map(
			[...GROK_IMAGE_ASPECT_RATIO_OPTIONS, ...IMAGE_ASPECT_RATIO_OPTIONS].map((option) => [
				option.value,
				option
			])
		).values()
	);
	$: customImageSizeOptions = getPropertyEnumOptions(
		getImageValveProperty(customValvesSpec, 'image_size'),
		GEMINI_IMAGE_SIZE_OPTIONS.map((option) => ({ value: option.value, label: option.value }))
	);
	$: customAspectRatioOptions = getPropertyEnumOptions(
		getImageValveProperty(customValvesSpec, 'aspect_ratio'),
		customAspectRatioFallback
	);
	$: customResolutionOptions = getPropertyEnumOptions(
		getImageValveProperty(customValvesSpec, 'resolution'),
		GROK_IMAGE_RESOLUTION_OPTIONS
	);

	$: selectedRatioId = (() => {
		const currentAspect = `${imageGenerationOptions?.aspect_ratio ?? ''}`.trim();
		const currentSize = `${imageGenerationOptions?.size ?? ''}`.trim();
		if (!currentAspect && !currentSize) return 'auto';
		return (
			ratioPresets.find(
				(preset) =>
					(preset.ratio && preset.ratio === currentAspect) ||
					(preset.size && preset.size === currentSize)
			)?.id ?? 'auto'
		);
	})();

	$: currentCount = Math.max(1, Math.min(Number(imageGenerationOptions?.n ?? 1) || 1, 4));
	$: currentBackground = `${imageGenerationOptions?.background ?? ''}`;
	$: currentSteps = Math.max(0, Math.min(Number(imageGenerationOptions?.steps ?? 0) || 0, 80));
	$: currentQuality = hasBuiltinResolutionOption
		? `${imageGenerationOptions?.resolution ?? resolutionOptions[0]?.value ?? '1k'}`
		: `${imageGenerationOptions?.image_size ?? builtinImageSizeOptions[1]?.value ?? '1K'}`;
	$: currentQualityLabel = hasBuiltinResolutionOption
		? findOptionLabel(resolutionOptions, currentQuality)
		: findOptionLabel(builtinImageSizeOptions, currentQuality);
	$: modelSummary = (() => {
		if (loading) return tr('正在读取当前图片模型能力...', 'Loading image model capabilities...');
		if (hasBuiltinImage) {
			const name =
				builtinModelMeta?.name || builtinModelMeta?.id || tr('当前图片模型', 'current image model');
			return tr(
				'当前模型：{{name}}。面板只会启用这个模型真正支持的参数。',
				'Current model: {{name}}. This panel only enables options supported by this model.',
				{
					name
				}
			);
		}
		if (hasCustomImage) {
			return tr(
				'当前自定义模型有图片参数，专属参数会保存到该模型自己的用户设置里。',
				'The current custom model has image options. Its custom options are saved in that model user settings.'
			);
		}
		return tr(
			'当前会使用管理员配置的图片服务；不确定支持的参数会保持可选或给出说明。',
			'This will use the administrator-configured image service. Options with uncertain support remain conservative or explained.'
		);
	})();
</script>

<DropdownMenu.Root bind:open closeFocus={false} typeahead={false} onOpenChange={handleOpenChange}>
	<Tooltip
		content={imageGenerationEnabled
			? tr('图片生成已开启，点击调整', 'Image generation is on. Click to adjust.')
			: tr('图片生成', 'Image generation')}
		placement="top"
	>
		<DropdownMenu.Trigger
			class="relative flex h-8 shrink-0 items-center justify-center rounded-full border transition
					{imageGenerationEnabled
				? 'gap-1.5 border-teal-200 bg-teal-50 px-2.5 text-teal-700 hover:bg-teal-100 dark:border-teal-500/30 dark:bg-teal-500/15 dark:text-teal-100'
				: 'w-8 border-transparent bg-transparent text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-800'}"
			aria-label={tr('图片生成设置', 'Image generation settings')}
			aria-pressed={imageGenerationEnabled}
		>
			<Wand2 class="size-4" strokeWidth={1.9} />
			{#if imageGenerationEnabled}
				<span class="pr-0.5 text-xs font-medium leading-none">{tr('绘图', 'Draw')}</span>
			{/if}
		</DropdownMenu.Trigger>
	</Tooltip>

	<DropdownMenu.Content
		class="z-50 w-[42rem] max-w-[calc(100vw-1.5rem)] overflow-hidden rounded-2xl border border-gray-200/80 bg-white text-gray-900 shadow-2xl outline-none dark:border-gray-700/70 dark:bg-gray-900 dark:text-gray-100"
		side="top"
		align="start"
		sideOffset={12}
		collisionPadding={12}
		transition={flyAndScale}
	>
		<div class="max-h-[min(78vh,44rem)] overflow-y-auto p-3">
			<div class="flex items-start justify-between gap-3">
				<div class="min-w-0">
					<div class="flex items-center gap-2">
						<div
							class="flex size-8 items-center justify-center rounded-lg bg-teal-50 text-teal-600 dark:bg-teal-500/15 dark:text-teal-200"
						>
							<ImageIcon class="size-4" strokeWidth={2} />
						</div>
						<div>
							<div class="text-sm font-semibold">{tr('图片生成', 'Image generation')}</div>
							<div class="mt-0.5 text-xs leading-5 text-gray-500 dark:text-gray-400">
								{modelSummary}
							</div>
						</div>
					</div>
				</div>
				<div class="flex shrink-0 items-center gap-2">
					<Switch bind:state={imageGenerationEnabled} />
					<button
						type="button"
						class="flex size-7 items-center justify-center rounded-full text-gray-500 transition hover:bg-gray-100 hover:text-gray-700 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-gray-200"
						aria-label={tr('关闭', 'Close')}
						on:click={() => (open = false)}
					>
						<X class="size-4" strokeWidth={2} />
					</button>
				</div>
			</div>

			{#if loading}
				<div
					class="mt-4 flex items-center gap-2 rounded-lg border border-gray-200/70 bg-gray-50 px-3 py-2 text-xs text-gray-500 dark:border-gray-700/70 dark:bg-gray-800/60 dark:text-gray-400"
				>
					<Spinner className="size-3.5" />
					{tr('正在加载当前模型的图片参数...', 'Loading image options for the current model...')}
				</div>
			{/if}

			<div class="mt-4 grid gap-4 lg:grid-cols-[1.08fr_0.92fr]">
				<div class="space-y-4">
					<section class="space-y-2">
						<div class="flex items-center justify-between">
							<div class="text-xs font-semibold text-gray-500 dark:text-gray-400">
								{tr('图片比例', 'Aspect ratio')}
							</div>
							{#if !canUseAspectRatioControl && !canUseExactSizeControl}
								<Tooltip
									content={tr(
										'当前模型没有暴露可选择的比例或尺寸参数。',
										'The current model does not expose selectable aspect ratio or size options.'
									)}
									placement="top"
								>
									<CircleHelp class={helpIconClass} strokeWidth={1.9} />
								</Tooltip>
							{/if}
						</div>
						<div class="grid grid-cols-3 gap-2 sm:grid-cols-6">
							{#each ratioPresets as preset}
								{@const usable =
									!preset.ratio ||
									canUseAspectRatioControl ||
									(canUseExactSizeControl && Boolean(preset.size))}
								<button
									type="button"
									disabled={!usable}
									class="group flex min-h-[5.5rem] flex-col items-center justify-center gap-2 rounded-lg border px-2 py-2 text-xs transition
										{selectedRatioId === preset.id
										? 'border-teal-400 bg-teal-50 text-teal-700 dark:border-teal-400/60 dark:bg-teal-500/15 dark:text-teal-100'
										: 'border-gray-200 bg-gray-50/80 text-gray-700 hover:border-gray-300 hover:bg-gray-100 dark:border-gray-700 dark:bg-gray-800/60 dark:text-gray-200 dark:hover:border-gray-600'}
										{!usable ? optionDisabledClass : ''}"
									title={!usable
										? tr(
												'当前模型不支持这个比例。',
												'The current model does not support this ratio.'
											)
										: ''}
									on:click={() => selectRatioPreset(preset)}
								>
									<div class="flex h-8 w-10 items-center justify-center">
										<div
											class="max-h-8 min-h-[1.1rem] w-full max-w-8 rounded border-2 {preset.preview}
												{selectedRatioId === preset.id
												? 'border-teal-500 bg-teal-100 dark:border-teal-300 dark:bg-teal-300/20'
												: 'border-gray-400 bg-white dark:border-gray-500 dark:bg-gray-900'}"
										/>
									</div>
									<div class="leading-4">
										<div>{tr(preset.labelZh, preset.labelEn)}</div>
										<div class="text-[11px] text-gray-400 dark:text-gray-500">
											{preset.ratio ?? tr('自动', 'Auto')}
										</div>
									</div>
								</button>
							{/each}
						</div>
					</section>

					<section class="space-y-2">
						<div
							class="flex items-center gap-1.5 text-xs font-semibold text-gray-500 dark:text-gray-400"
						>
							<Wand2 class="size-3.5" strokeWidth={1.9} />
							{tr('风格', 'Style')}
						</div>
						<div class="grid grid-cols-2 gap-2 sm:grid-cols-3">
							{#each stylePresets as preset}
								<button
									type="button"
									class="rounded-lg border border-gray-200 bg-white px-3 py-2 text-left transition hover:border-teal-300 hover:bg-teal-50 dark:border-gray-700 dark:bg-gray-850 dark:hover:border-teal-500/50 dark:hover:bg-teal-500/10"
									on:click={() => applyPromptPreset(preset)}
								>
									<div class="text-sm font-medium">{tr(preset.labelZh, preset.labelEn)}</div>
									<div class="mt-1 line-clamp-2 text-xs leading-4 text-gray-500 dark:text-gray-400">
										{tr(preset.descriptionZh, preset.descriptionEn)}
									</div>
								</button>
							{/each}
						</div>
					</section>

					<section class="space-y-2">
						<div
							class="flex items-center gap-1.5 text-xs font-semibold text-gray-500 dark:text-gray-400"
						>
							<Sparkles class="size-3.5" strokeWidth={1.9} />
							{tr('灵感模板', 'Prompt templates')}
						</div>
						<div class="grid gap-2 sm:grid-cols-3">
							{#each ideaPresets as preset}
								<button
									type="button"
									class="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-left transition hover:border-teal-300 hover:bg-teal-50 dark:border-gray-700 dark:bg-gray-800/60 dark:hover:border-teal-500/50 dark:hover:bg-teal-500/10"
									on:click={() => applyPromptPreset(preset)}
								>
									<div class="text-sm font-medium">{tr(preset.labelZh, preset.labelEn)}</div>
									<div class="mt-1 line-clamp-2 text-xs leading-4 text-gray-500 dark:text-gray-400">
										{tr(preset.descriptionZh, preset.descriptionEn)}
									</div>
								</button>
							{/each}
						</div>
					</section>

					<section class="space-y-2">
						<div class="flex items-center justify-between">
							<div class="text-xs font-semibold text-gray-500 dark:text-gray-400">
								{tr('负面提示词', 'Negative prompt')}
							</div>
							{#if !canUseNegativePrompt}
								<div class="text-[11px] text-gray-400">
									{tr('当前模型不支持', 'Not supported')}
								</div>
							{/if}
						</div>
						<textarea
							rows="3"
							disabled={!canUseNegativePrompt}
							value={imageGenerationOptions?.negative_prompt ?? ''}
							placeholder={canUseNegativePrompt
								? tr('写下不希望出现在图片里的内容', 'Describe what should not appear in the image')
								: tr(
										'当前模型不会使用负面提示词。',
										'The current model will not use a negative prompt.'
									)}
							class="min-h-[5.5rem] w-full resize-none rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm leading-5 text-gray-900 outline-none transition placeholder:text-gray-400 focus:border-teal-300 disabled:cursor-not-allowed disabled:bg-gray-50 disabled:text-gray-400 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100 dark:placeholder:text-gray-500 dark:focus:border-teal-500/70 dark:disabled:bg-gray-800/60"
							on:input={handleNegativePromptInput}
						/>
					</section>
				</div>

				<div class="space-y-4">
					<section
						class="rounded-lg border border-gray-200 bg-gray-50/80 p-3 dark:border-gray-700 dark:bg-gray-800/50"
					>
						<div
							class="mb-3 flex items-center gap-1.5 text-xs font-semibold text-gray-500 dark:text-gray-400"
						>
							<SlidersHorizontal class="size-3.5" strokeWidth={1.9} />
							{tr('生成参数', 'Generation settings')}
						</div>

						<div class="space-y-3">
							{#if hasOpenAIRouteChoice}
								<div>
									<div class="mb-1.5 text-xs font-medium text-gray-600 dark:text-gray-300">
										{tr('接口模式', 'Route mode')}
									</div>
									<div class="grid gap-1.5">
										{#each openaiRouteOptions as option}
											<button
												type="button"
												disabled={option.disabled}
												class="flex items-center justify-between gap-2 rounded-lg border px-2.5 py-2 text-left text-xs transition
													{openaiRouteSelectedValue === option.value
													? 'border-teal-400 bg-white text-teal-700 dark:border-teal-400/60 dark:bg-teal-500/15 dark:text-teal-100'
													: 'border-gray-200 bg-white text-gray-700 hover:border-gray-300 dark:border-gray-700 dark:bg-gray-900/60 dark:text-gray-200 dark:hover:border-gray-600'}
													{option.disabled ? optionDisabledClass : ''}"
												on:click={() => {
													if (!option.disabled) setOpenAIRoute(option.value);
												}}
											>
												<span class="min-w-0">
													<span class="block font-medium">{option.label}</span>
													{#if option.description}
														<span class="mt-0.5 line-clamp-2 text-gray-500 dark:text-gray-400">
															{option.description}
														</span>
													{/if}
												</span>
												{#if openaiRouteSelectedValue === option.value}
													<Check class="size-4 shrink-0 text-teal-500" strokeWidth={2.2} />
												{/if}
											</button>
										{/each}
									</div>
								</div>
							{:else}
								<div
									class="rounded-lg border border-gray-200 bg-white px-3 py-2 text-xs text-gray-500 dark:border-gray-700 dark:bg-gray-900/60 dark:text-gray-400"
								>
									{tr(
										'当前模型没有可切换的接口模式。',
										'The current model does not expose switchable image routes.'
									)}
								</div>
							{/if}

							<div>
								<div class="mb-1.5 flex items-center justify-between">
									<div class="text-xs font-medium text-gray-600 dark:text-gray-300">
										{tr('生成数量', 'Images')}
									</div>
									{#if !supportsBatch}
										<div class="text-[11px] text-gray-400">
											{tr('当前模型只支持 1 张', 'This model only supports 1 image')}
										</div>
									{/if}
								</div>
								<div class="grid grid-cols-4 gap-1.5">
									{#each countOptions as count}
										<button
											type="button"
											disabled={!canUseBatch && count > 1}
											class="rounded-lg border px-2 py-1.5 text-xs font-medium transition
												{currentCount === count
												? 'border-teal-400 bg-white text-teal-700 dark:border-teal-400/60 dark:bg-teal-500/15 dark:text-teal-100'
												: 'border-gray-200 bg-white text-gray-700 hover:border-gray-300 dark:border-gray-700 dark:bg-gray-900/60 dark:text-gray-200'}
												{!canUseBatch && count > 1 ? optionDisabledClass : ''}"
											on:click={() => {
												if (canUseBatch || count === 1) setOption({ n: count });
											}}
										>
											{count}
										</button>
									{/each}
								</div>
							</div>

							<div>
								<div class="mb-1.5 flex items-center justify-between">
									<div class="flex items-center gap-1.5 text-xs font-medium text-gray-600 dark:text-gray-300">
										<span>
											{hasBuiltinResolutionOption
												? tr('清晰度', 'Resolution')
												: tr('尺寸档位', 'Size tier')}
										</span>
										<Tooltip content={qualityHelpDescription} placement="top">
											<button
												type="button"
												class="-m-1 rounded-full p-1 outline-none transition hover:bg-gray-100 focus:bg-gray-100 dark:hover:bg-gray-800 dark:focus:bg-gray-800"
												aria-label={qualityHelpDescription}
											>
												<CircleHelp class={helpIconClass} strokeWidth={1.9} />
											</button>
										</Tooltip>
									</div>
									<div class="text-[11px] text-gray-400">
										{canUseQuality ? currentQualityLabel : qualityUnavailableLabel}
									</div>
								</div>
								<div class="grid grid-cols-2 gap-1.5">
									{#if hasBuiltinResolutionOption}
										{#each resolutionOptions as option}
											<button
												type="button"
												disabled={!canUseQuality}
												class="rounded-lg border px-2 py-1.5 text-xs font-medium transition
													{currentQuality === option.value
													? 'border-teal-400 bg-white text-teal-700 dark:border-teal-400/60 dark:bg-teal-500/15 dark:text-teal-100'
													: 'border-gray-200 bg-white text-gray-700 hover:border-gray-300 dark:border-gray-700 dark:bg-gray-900/60 dark:text-gray-200'}
													{!canUseQuality ? optionDisabledClass : ''}"
												on:click={() => canUseQuality && setOption({ resolution: option.value })}
											>
												{option.label}
											</button>
										{/each}
									{:else}
										{#each builtinImageSizeOptions as option}
											<button
												type="button"
												disabled={!canUseQuality}
												class="rounded-lg border px-2 py-1.5 text-xs font-medium transition
													{currentQuality === option.value
													? 'border-teal-400 bg-white text-teal-700 dark:border-teal-400/60 dark:bg-teal-500/15 dark:text-teal-100'
													: 'border-gray-200 bg-white text-gray-700 hover:border-gray-300 dark:border-gray-700 dark:bg-gray-900/60 dark:text-gray-200'}
													{!canUseQuality ? optionDisabledClass : ''}"
												on:click={() => canUseQuality && setOption({ image_size: option.value })}
											>
												{option.label}
											</button>
										{/each}
									{/if}
								</div>
							</div>

							<div>
								<div class="mb-1.5 flex items-center justify-between">
									<div class="text-xs font-medium text-gray-600 dark:text-gray-300">
										{tr('背景', 'Background')}
									</div>
									{#if !canUseBackground}
										<div class="text-[11px] text-gray-400">
											{tr('当前模型不支持', 'Not supported')}
										</div>
									{/if}
								</div>
								<div class="grid grid-cols-2 gap-1.5">
									{#each backgroundOptions as option}
										<button
											type="button"
											disabled={!canUseBackground && option.value !== ''}
											class="rounded-lg border px-2 py-1.5 text-xs font-medium transition
												{currentBackground === option.value
												? 'border-teal-400 bg-white text-teal-700 dark:border-teal-400/60 dark:bg-teal-500/15 dark:text-teal-100'
												: 'border-gray-200 bg-white text-gray-700 hover:border-gray-300 dark:border-gray-700 dark:bg-gray-900/60 dark:text-gray-200'}
												{!canUseBackground && option.value !== '' ? optionDisabledClass : ''}"
											on:click={() => {
												if (canUseBackground || option.value === '')
													setOption({ background: option.value || null });
											}}
										>
											{tr(option.labelZh, option.labelEn)}
										</button>
									{/each}
								</div>
							</div>

							<div>
								<div class="mb-1.5 flex items-center justify-between">
									<div class="text-xs font-medium text-gray-600 dark:text-gray-300">
										{tr('步数', 'Steps')}
									</div>
									<div class="text-[11px] text-gray-400">
										{canUseSteps
											? currentSteps === 0
												? tr('自动', 'Auto')
												: currentSteps
											: stepsUnavailableLabel}
									</div>
								</div>
								<input
									type="range"
									min="0"
									max="80"
									step="5"
									disabled={!canUseSteps}
									value={currentSteps}
									class="h-2 w-full cursor-pointer appearance-none rounded-full bg-gray-200 disabled:cursor-not-allowed disabled:opacity-40 dark:bg-gray-700"
									on:input={handleStepsInput}
								/>
							</div>
						</div>
					</section>

					{#if hasCustomImage}
						<section
							class="rounded-lg border border-gray-200 bg-white p-3 dark:border-gray-700 dark:bg-gray-850"
						>
							<div class="mb-2 text-xs font-semibold text-gray-500 dark:text-gray-400">
								{tr('自定义模型参数', 'Custom model options')}
							</div>
							<div class="space-y-2">
								{#if getImageValveProperty(customValvesSpec, 'image_size')}
									<label class="block">
										<span class="mb-1 block text-xs text-gray-500 dark:text-gray-400">
											{getImageValveProperty(customValvesSpec, 'image_size')?.title ??
												tr('图片尺寸', 'Image size')}
										</span>
										<select
											class="w-full rounded-lg border border-gray-200 bg-gray-50 px-2 py-2 text-xs outline-none dark:border-gray-700 dark:bg-gray-900"
											value={`${customValves?.image_size ?? customImageSizeOptions[0]?.value ?? ''}`}
											on:change={handleCustomValveChange('image_size')}
										>
											{#each customImageSizeOptions as option}
												<option value={option.value}>{option.label}</option>
											{/each}
										</select>
									</label>
								{/if}
								{#if getImageValveProperty(customValvesSpec, 'aspect_ratio')}
									<label class="block">
										<span class="mb-1 block text-xs text-gray-500 dark:text-gray-400">
											{getImageValveProperty(customValvesSpec, 'aspect_ratio')?.title ??
												tr('图片比例', 'Aspect ratio')}
										</span>
										<select
											class="w-full rounded-lg border border-gray-200 bg-gray-50 px-2 py-2 text-xs outline-none dark:border-gray-700 dark:bg-gray-900"
											value={`${customValves?.aspect_ratio ?? customAspectRatioOptions[0]?.value ?? ''}`}
											on:change={handleCustomValveChange('aspect_ratio')}
										>
											{#each customAspectRatioOptions as option}
												<option value={option.value}>{option.label}</option>
											{/each}
										</select>
									</label>
								{/if}
								{#if getImageValveProperty(customValvesSpec, 'resolution')}
									<label class="block">
										<span class="mb-1 block text-xs text-gray-500 dark:text-gray-400">
											{getImageValveProperty(customValvesSpec, 'resolution')?.title ??
												tr('清晰度', 'Resolution')}
										</span>
										<select
											class="w-full rounded-lg border border-gray-200 bg-gray-50 px-2 py-2 text-xs outline-none dark:border-gray-700 dark:bg-gray-900"
											value={`${customValves?.resolution ?? customResolutionOptions[0]?.value ?? ''}`}
											on:change={handleCustomValveChange('resolution')}
										>
											{#each customResolutionOptions as option}
												<option value={option.value}>{option.label}</option>
											{/each}
										</select>
									</label>
								{/if}
							</div>
						</section>
					{/if}

					<div class="flex items-center justify-end gap-2 pt-1">
						<button
							type="button"
							class="rounded-lg bg-gray-900 px-3 py-2 text-xs font-semibold text-white transition hover:bg-gray-800 dark:bg-gray-100 dark:text-gray-900 dark:hover:bg-white"
							on:click={() => (open = false)}
						>
							{tr('完成', 'Done')}
						</button>
					</div>
				</div>
			</div>
		</div>
	</DropdownMenu.Content>
</DropdownMenu.Root>
