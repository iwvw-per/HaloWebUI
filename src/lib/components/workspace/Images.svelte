<script lang="ts">
	import { getContext, onMount, tick } from 'svelte';
	import { toast } from 'svelte-sonner';

	import {
		getImageGenerationModels,
		getImageUsageConfig,
		imageGenerations
	} from '$lib/apis/images';
	import type { ImageGenerationModel, ImageUsageConfig } from '$lib/apis/images';
	import HaloSelect from '$lib/components/common/HaloSelect.svelte';
	import ImagePreview from '$lib/components/common/ImagePreview.svelte';
	import ArrowDownTray from '$lib/components/icons/ArrowDownTray.svelte';
	import ArrowsPointingOut from '$lib/components/icons/ArrowsPointingOut.svelte';
	import Clipboard from '$lib/components/icons/Clipboard.svelte';
	import PhotoSolid from '$lib/components/icons/PhotoSolid.svelte';
	import Sparkles from '$lib/components/icons/Sparkles.svelte';
	import { WEBUI_NAME, user } from '$lib/stores';
	import { copyToClipboard } from '$lib/utils';
	import { localizeCommonError } from '$lib/utils/common-errors';
	import { getModelChatDisplayName } from '$lib/utils/model-display';
	import {
		GROK_IMAGE_ASPECT_RATIO_OPTIONS,
		GROK_IMAGE_RESOLUTION_OPTIONS
	} from '$lib/utils/image-generation';
	import {
		CUSTOM_SIZE_OPTION_VALUE,
		WORKSPACE_IMAGE_SIZE_PRESETS,
		extractImageConstraintFromError,
		formatPixelCount,
		getRecommendedImageSizes,
		parseImageSize,
		type LearnedImageConstraint
	} from '$lib/utils/workspace-image-generation';

	type GeneratedImage = {
		url: string;
	};

	type ViewState = 'loading' | 'ready' | 'disabled' | 'denied' | 'error';

	type SizeOption = {
		value: string;
		ratio: string;
		label: string;
	};

	type TabKey = 'workbench' | 'prompts' | 'gallery' | 'history';

	type StylePreset = {
		id: string;
		nameKey: string;
		icon: string;
		promptTemplate: string;
		category?: string;
	};

	type ImageGenerationTemplate = {
		id: string;
		name: string;
		tags: string[];
		createdAt: number;
		updatedAt: number;
		config: {
			prompt?: string;
			negativePrompt?: string;
			model?: string;
			size?: string;
			aspectRatio?: string;
			resolution?: string;
			steps?: number;
			numberOfImages?: string;
			background?: string;
		};
	};

	type GalleryImage = {
		id: string;
		url: string;
		prompt: string;
		negativePrompt?: string;
		model: string;
		size: string;
		createdAt: number;
		favorite?: boolean;
		tags?: string[];
	};

	type GenerationHistory = {
		id: string;
		prompt: string;
		negativePrompt?: string;
		model: string;
		parameters: Record<string, any>;
		status: 'success' | 'failed';
		images?: string[];
		error?: string;
		createdAt: number;
		completedAt?: number;
	};

	type WorkspaceImagePrefs = {
		selectionKey?: string;
		model?: string;
		credential_source?: string;
		connection_index?: number | null;
		presetSize?: string;
		customSize?: string;
		useCustomSize?: boolean;
		aspect_ratio?: string;
		resolution?: string;
		steps?: number;
		numberOfImages?: string;
		negativePrompt?: string;
		showNegativePrompt?: boolean;
		background?: string;
		customBackground?: string;
		learnedConstraints?: Record<string, LearnedImageConstraint>;
	};

	const WORKSPACE_IMAGE_PREFS_KEY = 'workspace:image-studio:prefs:v1';
	const WORKSPACE_IMAGE_TEMPLATES_KEY = 'workspace:image-studio:templates:v1';
	const WORKSPACE_IMAGE_GALLERY_KEY = 'workspace:image-studio:gallery:v1';
	const WORKSPACE_IMAGE_HISTORY_KEY = 'workspace:image-studio:history:v1';
	const WORKSPACE_IMAGE_ACTIVE_TAB_KEY = 'workspace:image-studio:active-tab:v1';
	const i18n = getContext('i18n');

	const promptIdeas = [
		'Cinematic portrait',
		'Clean product shot',
		'Editorial poster',
		'Cozy illustration',
		'Neon cityscape',
		'Minimal interior'
	];

	const BUILT_IN_STYLE_PRESETS: StylePreset[] = [
		{
			id: 'cinematic-realistic',
			nameKey: 'Cinematic Realistic',
			icon: '🎬',
			promptTemplate: 'cinematic photography, photorealistic, dramatic lighting, film grain, professional color grading',
			category: 'photography'
		},
		{
			id: 'ecommerce-product',
			nameKey: 'E-commerce Product',
			icon: '🛍️',
			promptTemplate: 'clean product photography, white background, studio lighting, high resolution, commercial quality',
			category: 'commercial'
		},
		{
			id: 'transparent-sticker',
			nameKey: 'Transparent Sticker',
			icon: '🏷️',
			promptTemplate: 'sticker design, transparent background, clean edges, vibrant colors, die-cut style',
			category: 'design'
		},
		{
			id: '3d-toy',
			nameKey: '3D Designer Toy',
			icon: '🎨',
			promptTemplate: '3D render, designer toy, smooth plastic material, studio lighting, trendy collectible style',
			category: 'design'
		},
		{
			id: 'flat-icon',
			nameKey: 'Flat Icon',
			icon: '📱',
			promptTemplate: 'flat design icon, minimalist, simple shapes, solid colors, modern UI style',
			category: 'design'
		},
		{
			id: 'vintage-poster',
			nameKey: 'Vintage Poster',
			icon: '📜',
			promptTemplate: 'vintage poster design, retro style, aged paper texture, classic typography, nostalgic aesthetic',
			category: 'art'
		},
		{
			id: 'anime-illustration',
			nameKey: 'Anime Illustration',
			icon: '🎌',
			promptTemplate: 'anime style illustration, vibrant colors, detailed linework, expressive characters, Japanese animation aesthetic',
			category: 'art'
		},
		{
			id: 'minimal-wallpaper',
			nameKey: 'Minimal Wallpaper',
			icon: '🖼️',
			promptTemplate: 'minimalist wallpaper, clean composition, subtle gradients, modern aesthetic, high resolution',
			category: 'design'
		}
	];

	const negativePromptSuggestions = [
		'blurry',
		'low quality',
		'distorted',
		'watermark',
		'text',
		'oversaturated'
	];

	const curatedSizeOptions: SizeOption[] = [
		{ value: '1024x1024', ratio: '1:1', label: '1024x1024' },
		{ value: '1024x1536', ratio: '2:3', label: '1024x1536' },
		{ value: '1536x1024', ratio: '3:2', label: '1536x1024' },
		{ value: '1536x1536', ratio: '1:1', label: '1536x1536' },
		{ value: '2048x2048', ratio: '1:1', label: '2048x2048' },
		{ value: '2048x3072', ratio: '2:3', label: '2048x3072' },
		{ value: '3072x2048', ratio: '3:2', label: '3072x2048' }
	];

	let loaded = false;
	let loading = false;
	let preferencesReady = false;
	let lastPersistedPrefsSnapshot = '';
	let viewState: ViewState = 'loading';
	let loadError: string | null = null;
	let imageModels: ImageGenerationModel[] = [];
	let prompt = '';
	let selectedModel = '';
	let selectedModelRawId = '';
	let selectedPresetSize = WORKSPACE_IMAGE_SIZE_PRESETS[0];
	let usingCustomSize = false;
	let customSizeInput = '';
	let selectedAspectRatioOption = '1:1';
	let selectedResolution = '1k';
	let steps = 0;
	let canSubmit = false;
	let blockedReason: string | null = null;
	let workspaceNoModels = false;
	let learnedConstraints: Record<string, LearnedImageConstraint> = {};

	// 新增参数
	let numberOfImages = '1';
	let negativePrompt = '';
	let showNegativePrompt = false;
	let background = 'auto';
	let customBackground = '';

	// 标签页状态
	let activeTab: TabKey = 'workbench';

	// 模板相关
	let templateName = '';
	let templateTags = '';
	let savedTemplates: ImageGenerationTemplate[] = [];

	// 图库相关
	let galleryImages: GalleryImage[] = [];
	let gallerySearchQuery = '';
	let gallerySortBy = 'recent';

	// 历史记录相关
	let generationHistory: GenerationHistory[] = [];

	let generatedImages: GeneratedImage[] = [];
	let lastPrompt = '';
	let resultsSectionElement: HTMLElement | null = null;

	let previewOpen = false;
	let previewSrc = '';
	let previewAlt = '';
	let hadPersistedSelectionKey = false;

	const isAdmin = () => $user?.role === 'admin';
	const formatError = (error: unknown) =>
		localizeCommonError(error, (key, options) => $i18n.t(key, options));
	const getModelOptionValue = (model: ImageGenerationModel | null | undefined) =>
		`${model?.selection_id ?? model?.selection_key ?? model?.legacy_id ?? model?.id ?? ''}`.trim();
	const getModelSourceBadge = (model: ImageGenerationModel | null | undefined) => {
		const source = `${model?.source ?? ''}`.trim().toLowerCase();
		if (source === 'shared') {
			return $i18n.t('Shared');
		}
		if (source === 'personal') {
			return $i18n.t('Personal');
		}
		return '';
	};
	const getModelLabel = (model: ImageGenerationModel | null | undefined) =>
		getModelChatDisplayName(model as { id?: string; name?: string; connection_name?: string } | null) ||
		`${model?.name ?? model?.id ?? ''}`.trim();

	const loadWorkspacePrefs = () => {
		try {
			const raw = localStorage.getItem(WORKSPACE_IMAGE_PREFS_KEY);
			if (!raw) return;

			const prefs = JSON.parse(raw) as WorkspaceImagePrefs;
			selectedModel = `${prefs?.selectionKey ?? prefs?.model ?? ''}`.trim();
			selectedModelRawId = `${prefs?.model ?? ''}`.trim();
			hadPersistedSelectionKey = Boolean(`${prefs?.selectionKey ?? ''}`.trim());
			selectedPresetSize = curatedSizeOptions.some(
				(option) => option.value === `${prefs?.presetSize ?? ''}`.trim()
			)
				? (`${prefs?.presetSize}`.trim() as (typeof WORKSPACE_IMAGE_SIZE_PRESETS)[number])
				: WORKSPACE_IMAGE_SIZE_PRESETS[0];
			customSizeInput = `${prefs?.customSize ?? ''}`.trim();
			usingCustomSize = Boolean(prefs?.useCustomSize && customSizeInput);
			selectedAspectRatioOption = `${prefs?.aspect_ratio ?? '1:1'}`.trim() || '1:1';
			selectedResolution = `${prefs?.resolution ?? '1k'}`.trim().toLowerCase() || '1k';

			const nextSteps = Number(prefs?.steps ?? 0);
			steps = Number.isFinite(nextSteps) && nextSteps >= 0 ? nextSteps : 0;

			// 加载新参数
			numberOfImages = `${prefs?.numberOfImages ?? '1'}`.trim() || '1';
			negativePrompt = `${prefs?.negativePrompt ?? ''}`.trim();
			showNegativePrompt = Boolean(prefs?.showNegativePrompt);
			background = `${prefs?.background ?? 'auto'}`.trim() || 'auto';
			customBackground = `${prefs?.customBackground ?? ''}`.trim();

			learnedConstraints =
				prefs?.learnedConstraints && typeof prefs.learnedConstraints === 'object'
					? prefs.learnedConstraints
					: {};
			lastPersistedPrefsSnapshot = raw;
		} catch (error) {
			console.warn('Failed to load workspace image prefs', error);
		}
	};

	$: modelOptions = imageModels.map((model) => ({
		value: getModelOptionValue(model),
		label: getModelLabel(model),
		description: model.id,
		badge: getModelSourceBadge(model)
	}));
	$: nativeAspectRatioOptions = GROK_IMAGE_ASPECT_RATIO_OPTIONS.map((option) => ({
		value: option.value,
		label: option.label
	}));
	$: nativeResolutionOptions = GROK_IMAGE_RESOLUTION_OPTIONS.map((option) => ({
		value: option.value,
		label: option.label
	}));

	$: sizeOptions = [
		...curatedSizeOptions,
		{
			value: CUSTOM_SIZE_OPTION_VALUE,
			ratio: $i18n.t('Custom'),
			label: $i18n.t('Custom size')
		}
	];

	$: selectedModelLabel =
		modelOptions.find((option) => option.value === selectedModel)?.label ?? selectedModel;
	$: selectedModelMeta =
		imageModels.find((model) => getModelOptionValue(model) === selectedModel) ?? null;
	$: usesNativeAspectRatioControls = Boolean(
		selectedModelMeta &&
			(selectedModelMeta?.size_mode === 'aspect_ratio' || selectedModelMeta?.supports_resolution)
	);
	$: showsResolutionControl = Boolean(selectedModelMeta?.supports_resolution);
	$: showsStepsControl = !showsResolutionControl;
	$: activeSize = usingCustomSize ? `${customSizeInput ?? ''}`.trim() : selectedPresetSize;
	$: activeSizeLabel =
		usingCustomSize && activeSize ? activeSize : usingCustomSize ? $i18n.t('Custom size') : selectedPresetSize;
	$: activeSizeParsed = parseImageSize(activeSize);
	$: selectedAspectRatio = activeSizeParsed?.aspectRatio ?? null;
	$: currentConstraint = selectedModel ? learnedConstraints[selectedModel] ?? null : null;
	$: sizeSelectValue = usingCustomSize ? CUSTOM_SIZE_OPTION_VALUE : selectedPresetSize;
	$: recommendedSizes = getRecommendedImageSizes(activeSize, {
		minPixels: currentConstraint?.minPixels,
		limit: 3
	});
	$: if (selectedModelMeta?.id) {
		selectedModelRawId = selectedModelMeta.id;
	}

	$: sizeValidation = (() => {
		if (usesNativeAspectRatioControls) {
			return null;
		}

		if (usingCustomSize && !activeSize) {
			return {
				kind: 'empty',
				blocking: true,
				title: $i18n.t('Custom size is required'),
				description: $i18n.t('Enter a custom size like {{example}}.', { example: '1344x768' })
			};
		}

		if (usingCustomSize && !activeSizeParsed) {
			return {
				kind: 'format',
				blocking: true,
				title: $i18n.t('Custom size format is invalid'),
				description: $i18n.t('Use a value like {{example}}.', { example: '1344x768' })
			};
		}

		if (currentConstraint?.minPixels && activeSizeParsed) {
			if (activeSizeParsed.pixels < currentConstraint.minPixels) {
				return {
					kind: 'minPixels',
					blocking: true,
					title: $i18n.t("Current size does not meet this model's requirement."),
					description: $i18n.t(
						'{{size}} has {{pixels}} pixels. This model currently requires at least {{minPixels}} pixels.',
						{
							size: activeSizeParsed.value,
							pixels: formatPixelCount(activeSizeParsed.pixels),
							minPixels: formatPixelCount(currentConstraint.minPixels)
						}
					)
				};
			}
		}

		return null;
	})();

	$: blockedReason =
		viewState !== 'ready'
			? viewState === 'denied'
				? $i18n.t('Image generation access required')
				: viewState === 'disabled'
					? $i18n.t('Image generation is disabled by the administrator.')
					: loadError || $i18n.t('Failed to load image generation settings.')
			: workspaceNoModels
				? $i18n.t('Image models are unavailable right now. Check your image settings.')
				: sizeValidation?.description ?? null;

	$: canSubmit =
		!loading &&
		viewState === 'ready' &&
		imageModels.length > 0 &&
		Boolean(prompt.trim()) &&
		!sizeValidation?.blocking;

	$: currentPrefsSnapshot = preferencesReady
		? JSON.stringify({
				selectionKey: selectedModel,
				model: selectedModelMeta?.id ?? selectedModelRawId ?? '',
				credential_source: selectedModelMeta?.source ?? '',
				connection_index: selectedModelMeta?.connection_index ?? null,
				model_ref: selectedModelMeta?.model_ref ?? null,
				presetSize: selectedPresetSize,
				customSize: customSizeInput,
				useCustomSize: usingCustomSize,
				aspect_ratio: selectedAspectRatioOption,
				resolution: selectedResolution,
				steps,
				numberOfImages,
				negativePrompt,
				showNegativePrompt,
				background,
				customBackground,
				learnedConstraints
			})
		: '';

	$: if (
		preferencesReady &&
		currentPrefsSnapshot &&
		currentPrefsSnapshot !== lastPersistedPrefsSnapshot
	) {
		try {
			localStorage.setItem(WORKSPACE_IMAGE_PREFS_KEY, currentPrefsSnapshot);
			lastPersistedPrefsSnapshot = currentPrefsSnapshot;
		} catch (error) {
			console.warn('Failed to persist workspace image prefs', error);
		}
	}

	$: if (activeTab) {
		saveActiveTab(activeTab);
	}

	$: filteredGalleryImages = galleryImages.filter((img) => {
		if (!gallerySearchQuery.trim()) return true;
		const query = gallerySearchQuery.toLowerCase();
		return (
			img.prompt.toLowerCase().includes(query) ||
			img.model.toLowerCase().includes(query) ||
			img.tags?.some((tag) => tag.toLowerCase().includes(query))
		);
	});

	$: sortedGalleryImages = (() => {
		const filtered = [...filteredGalleryImages];
		if (gallerySortBy === 'recent') {
			return filtered.sort((a, b) => b.createdAt - a.createdAt);
		} else if (gallerySortBy === 'favorites') {
			return filtered.sort((a, b) => (b.favorite ? 1 : 0) - (a.favorite ? 1 : 0));
		}
		return filtered;
	})();

	const applyPromptIdea = (idea: string) => {
		const translatedIdea = $i18n.t(idea);
		prompt = prompt.trim() ? `${prompt.trim()}, ${translatedIdea}` : translatedIdea;
	};

	const applyStylePreset = (preset: StylePreset) => {
		const currentPrompt = prompt.trim();
		const template = preset.promptTemplate;

		if (!currentPrompt) {
			prompt = template;
		} else {
			// 智能合并，避免重复
			if (!currentPrompt.toLowerCase().includes(template.toLowerCase())) {
				prompt = `${currentPrompt}, ${template}`;
			}
		}
	};

	const addNegativePromptSuggestion = (suggestion: string) => {
		const translatedSuggestion = $i18n.t(suggestion);
		negativePrompt = negativePrompt.trim()
			? `${negativePrompt.trim()}, ${translatedSuggestion}`
			: translatedSuggestion;
	};

	const loadTemplates = () => {
		try {
			const raw = localStorage.getItem(WORKSPACE_IMAGE_TEMPLATES_KEY);
			if (raw) {
				savedTemplates = JSON.parse(raw);
			}
		} catch (error) {
			console.warn('Failed to load templates', error);
		}
	};

	const saveTemplate = () => {
		const name = templateName.trim();
		if (!name) return;

		const template: ImageGenerationTemplate = {
			id: `template_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`,
			name,
			tags: templateTags
				.split(',')
				.map((t) => t.trim())
				.filter(Boolean),
			createdAt: Date.now(),
			updatedAt: Date.now(),
			config: {
				prompt: prompt.trim() || undefined,
				negativePrompt: negativePrompt.trim() || undefined,
				model: selectedModel || selectedModelRawId || undefined,
				size: activeSize || undefined,
				aspectRatio: selectedAspectRatioOption || undefined,
				resolution: selectedResolution || undefined,
				steps: steps > 0 ? steps : undefined,
				numberOfImages: numberOfImages !== '1' ? numberOfImages : undefined,
				background: background !== 'auto' ? background : undefined
			}
		};

		savedTemplates = [template, ...savedTemplates];

		try {
			localStorage.setItem(WORKSPACE_IMAGE_TEMPLATES_KEY, JSON.stringify(savedTemplates));
			toast.success($i18n.t('Template saved successfully'));
			templateName = '';
			templateTags = '';
		} catch (error) {
			toast.error($i18n.t('Failed to save template'));
		}
	};

	const loadTemplate = (template: ImageGenerationTemplate) => {
		const config = template.config;

		if (config.prompt) prompt = config.prompt;
		if (config.negativePrompt) {
			negativePrompt = config.negativePrompt;
			showNegativePrompt = true;
		}
		if (config.size) {
			const matchedPreset = curatedSizeOptions.find((opt) => opt.value === config.size);
			if (matchedPreset) {
				selectedPresetSize = config.size;
				usingCustomSize = false;
			} else {
				customSizeInput = config.size;
				usingCustomSize = true;
			}
		}
		if (config.aspectRatio) selectedAspectRatioOption = config.aspectRatio;
		if (config.resolution) selectedResolution = config.resolution;
		if (config.steps !== undefined) steps = config.steps;
		if (config.numberOfImages) numberOfImages = config.numberOfImages;
		if (config.background) background = config.background;

		toast.success($i18n.t('Template loaded'));
	};

	const deleteTemplate = (id: string) => {
		savedTemplates = savedTemplates.filter((t) => t.id !== id);
		try {
			localStorage.setItem(WORKSPACE_IMAGE_TEMPLATES_KEY, JSON.stringify(savedTemplates));
			toast.success($i18n.t('Template deleted'));
		} catch (error) {
			toast.error($i18n.t('Failed to delete template'));
		}
	};

	const loadActiveTab = () => {
		try {
			const raw = localStorage.getItem(WORKSPACE_IMAGE_ACTIVE_TAB_KEY);
			if (raw) {
				activeTab = raw as TabKey;
			}
		} catch (error) {
			console.warn('Failed to load active tab', error);
		}
	};

	const saveActiveTab = (tab: TabKey) => {
		try {
			localStorage.setItem(WORKSPACE_IMAGE_ACTIVE_TAB_KEY, tab);
		} catch (error) {
			console.warn('Failed to save active tab', error);
		}
	};

	const addToGallery = (images: GeneratedImage[]) => {
		try {
			const raw = localStorage.getItem(WORKSPACE_IMAGE_GALLERY_KEY);
			const existing: GalleryImage[] = raw ? JSON.parse(raw) : [];

			const newImages: GalleryImage[] = images.map((img) => ({
				id: `gallery_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`,
				url: img.url,
				prompt: lastPrompt,
				negativePrompt: negativePrompt.trim() || undefined,
				model: selectedModelLabel,
				size: activeSizeLabel,
				createdAt: Date.now()
			}));

			galleryImages = [...newImages, ...existing];
			localStorage.setItem(WORKSPACE_IMAGE_GALLERY_KEY, JSON.stringify(galleryImages));
		} catch (error) {
			console.warn('Failed to add to gallery', error);
		}
	};

	const loadGallery = () => {
		try {
			const raw = localStorage.getItem(WORKSPACE_IMAGE_GALLERY_KEY);
			if (raw) {
				galleryImages = JSON.parse(raw);
			}
		} catch (error) {
			console.warn('Failed to load gallery', error);
		}
	};

	const toggleFavorite = (id: string) => {
		galleryImages = galleryImages.map((img) =>
			img.id === id ? { ...img, favorite: !img.favorite } : img
		);
		try {
			localStorage.setItem(WORKSPACE_IMAGE_GALLERY_KEY, JSON.stringify(galleryImages));
		} catch (error) {
			console.warn('Failed to update favorite', error);
		}
	};

	const addToHistory = (
		status: 'success' | 'failed',
		images?: string[],
		error?: string
	) => {
		try {
			const raw = localStorage.getItem(WORKSPACE_IMAGE_HISTORY_KEY);
			const existing: GenerationHistory[] = raw ? JSON.parse(raw) : [];

			const historyItem: GenerationHistory = {
				id: `history_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`,
				prompt: lastPrompt,
				negativePrompt: negativePrompt.trim() || undefined,
				model: selectedModelLabel,
				parameters: {
					size: activeSizeLabel,
					steps,
					numberOfImages,
					background
				},
				status,
				images,
				error,
				createdAt: Date.now(),
				completedAt: Date.now()
			};

			generationHistory = [historyItem, ...existing].slice(0, 100); // 保留最近100条
			localStorage.setItem(WORKSPACE_IMAGE_HISTORY_KEY, JSON.stringify(generationHistory));
		} catch (error) {
			console.warn('Failed to add to history', error);
		}
	};

	const loadHistory = () => {
		try {
			const raw = localStorage.getItem(WORKSPACE_IMAGE_HISTORY_KEY);
			if (raw) {
				generationHistory = JSON.parse(raw);
			}
		} catch (error) {
			console.warn('Failed to load history', error);
		}
	};

	const clearHistory = () => {
		if (confirm($i18n.t('Are you sure you want to clear all history?'))) {
			generationHistory = [];
			try {
				localStorage.removeItem(WORKSPACE_IMAGE_HISTORY_KEY);
				toast.success($i18n.t('History cleared'));
			} catch (error) {
				toast.error($i18n.t('Failed to clear history'));
			}
		}
	};

	const handleComposerKeydown = (event: KeyboardEvent) => {
		if ((event.metaKey || event.ctrlKey) && event.key === 'Enter' && !loading) {
			event.preventDefault();
			void submitHandler();
		}
	};

	const syncSelectedModelWithAvailableModels = (models: ImageGenerationModel[]) => {
		const normalizedModels = models ?? [];
		const availableValues = new Set(
			normalizedModels.map((model) => getModelOptionValue(model)).filter(Boolean)
		);

		if (selectedModel && availableValues.has(selectedModel)) {
			return;
		}

		const preferredModelId = `${selectedModelRawId ?? ''}`.trim();
		const nextModel =
			(preferredModelId
				? normalizedModels.find(
						(model) => model.selection_id === preferredModelId
					) ??
					normalizedModels.find(
						(model) => model.selection_key === preferredModelId
					) ??
					normalizedModels.find(
						(model) => model.legacy_id === preferredModelId
					) ??
					normalizedModels.find(
						(model) => model.id === preferredModelId && `${model.source ?? ''}`.trim() === 'shared'
					) ??
					normalizedModels.find((model) => model.id === preferredModelId)
				: null) ?? normalizedModels[0] ?? null;

		if (!nextModel) {
			selectedModel = '';
			return;
		}

		const nextValue = getModelOptionValue(nextModel);
		const sourceChanged =
			hadPersistedSelectionKey &&
			Boolean(selectedModel) &&
			selectedModel !== nextValue &&
			Boolean(preferredModelId) &&
			nextModel.id === preferredModelId;

		selectedModel = nextValue;
		selectedModelRawId = nextModel.id;

		if (sourceChanged) {
			toast.info($i18n.t('Your previous image model source is unavailable. Switched to another available source for the same model.'));
			hadPersistedSelectionKey = false;
		}
	};

	const loadWorkspaceModels = async () => {
		loadError = null;
		const nextModels = await getImageGenerationModels(localStorage.token, {
			context: 'workspace',
			credentialSource: 'auto'
		}).catch((error) => {
			loadError = `${error ?? ''}`;
			return null;
		});

		imageModels = Array.isArray(nextModels) ? nextModels : [];
		syncSelectedModelWithAvailableModels(imageModels);
		workspaceNoModels = !loadError && imageModels.length === 0;
	};

	const copyPromptHandler = async () => {
		const text = lastPrompt || prompt.trim();
		if (!text) return;

		const copied = await copyToClipboard(text);
		if (copied) {
			toast.success($i18n.t('Prompt copied'));
		}
	};

	const openPreview = (image: GeneratedImage, index: number) => {
		previewSrc = image.url;
		previewAlt = `${$i18n.t('Generated image')} ${index + 1}`;
		previewOpen = true;
	};

	const downloadImage = (url: string, index: number) => {
		const link = document.createElement('a');
		link.href = url;
		link.download = `generated-image-${index + 1}.png`;
		document.body.appendChild(link);
		link.click();
		document.body.removeChild(link);
	};

	const setLearnedConstraint = (constraint: LearnedImageConstraint | null) => {
		if (!constraint?.minPixels || !selectedModel) return;

		const previous = learnedConstraints[selectedModel];
		const nextMinPixels = Math.max(previous?.minPixels ?? 0, constraint.minPixels);

		learnedConstraints = {
			...learnedConstraints,
			[selectedModel]: {
				...previous,
				...constraint,
				minPixels: nextMinPixels
			}
		};
	};

	const buildToastDescription = (constraint: LearnedImageConstraint | null) => {
		const parts: string[] = [];

		if (constraint?.minPixels && activeSizeParsed) {
			parts.push(
				$i18n.t(
					'{{size}} has {{pixels}} pixels. This model currently requires at least {{minPixels}} pixels.',
					{
						size: activeSizeParsed.value,
						pixels: formatPixelCount(activeSizeParsed.pixels),
						minPixels: formatPixelCount(constraint.minPixels)
					}
				)
			);
		}

		if (constraint?.requestId) {
			parts.push(`${$i18n.t('Request ID')}: ${constraint.requestId}`);
		}

		return parts.join(' ');
	};

	const showSizeValidationToast = () => {
		if (!sizeValidation) return;

		toast.error(sizeValidation.title, {
			description: sizeValidation.description,
			duration: 6000
		});
	};

	const handleSizeSelect = (nextValue: string) => {
		if (nextValue === CUSTOM_SIZE_OPTION_VALUE) {
			usingCustomSize = true;
			customSizeInput = customSizeInput.trim() || selectedPresetSize;
			return;
		}

		usingCustomSize = false;
		selectedPresetSize = nextValue;
	};

	const restorePresetSize = () => {
		usingCustomSize = false;
		customSizeInput = '';
	};

	const applyRecommendedSize = (size: string) => {
		selectedPresetSize = size;
		usingCustomSize = false;
		customSizeInput = '';
	};

	const submitHandler = async () => {
		const trimmedPrompt = prompt.trim();
		if (!trimmedPrompt) {
			toast.error($i18n.t('Please enter a prompt'));
			return;
		}

		if (sizeValidation?.blocking) {
			showSizeValidationToast();
			return;
		}

		if (!canSubmit) {
			if (blockedReason) {
				toast.error(blockedReason);
			}
			return;
		}

		loading = true;
		generatedImages = [];
		lastPrompt = trimmedPrompt;

		try {
			const response = await imageGenerations(localStorage.token, {
				prompt: trimmedPrompt,
				model: selectedModel || selectedModelMeta?.id || selectedModelRawId || undefined,
				model_ref: selectedModelMeta?.model_ref ?? undefined,
				size: usesNativeAspectRatioControls ? undefined : activeSize || undefined,
				aspect_ratio: usesNativeAspectRatioControls ? selectedAspectRatioOption : undefined,
				resolution: showsResolutionControl ? selectedResolution : undefined,
				steps: showsStepsControl && steps > 0 ? steps : undefined,
				n: parseInt(numberOfImages) || 1,
				negative_prompt: negativePrompt.trim() || undefined,
				background:
					background === 'custom'
						? customBackground.trim() || undefined
						: background !== 'auto'
							? background
							: undefined,
				credential_source:
					selectedModelMeta?.source === 'personal' || selectedModelMeta?.source === 'shared'
						? selectedModelMeta.source
						: undefined,
				connection_index: selectedModelMeta?.connection_index ?? undefined
			});

			if (response?.length) {
				generatedImages = response;
				addToGallery(response);
				addToHistory('success', response.map((img) => img.url));
				await tick();
				resultsSectionElement?.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
			} else {
				toast.error(
					$i18n.t('Model returned an empty response. Try resending or switching models.')
				);
				addToHistory('failed', undefined, 'Empty response');
			}
		} catch (error) {
			const learnedConstraint = extractImageConstraintFromError(error);
			setLearnedConstraint(learnedConstraint);

			if (learnedConstraint) {
				toast.error($i18n.t("Current size does not meet this model's requirement."), {
					description: buildToastDescription(learnedConstraint) || formatError(error),
					duration: 6000
				});
			} else {
				toast.error(formatError(error));
			}
			addToHistory('failed', undefined, formatError(error));
		} finally {
			loading = false;
		}
	};

	onMount(async () => {
		loadWorkspacePrefs();
		loadTemplates();
		loadGallery();
		loadHistory();
		loadActiveTab();
		preferencesReady = true;

		const allowed =
			$user?.role === 'admin' || Boolean($user?.permissions?.features?.image_generation);
		if (!allowed) {
			viewState = 'denied';
			loaded = true;
			return;
		}

		viewState = 'loading';
		loaded = true;

		const usageResult = await getImageUsageConfig(localStorage.token).catch((error) => error);

		if (
			usageResult instanceof Error ||
			!usageResult ||
			typeof usageResult !== 'object' ||
			!('enabled' in usageResult)
		) {
			loadError = `${usageResult ?? ''}`;
			viewState = 'error';
			return;
		}

		const usageConfig = usageResult as ImageUsageConfig;
		if (!usageConfig.enabled) {
			viewState = 'disabled';
			return;
		}


		await loadWorkspaceModels();

		if (loadError) {
			viewState = 'error';
			return;
		}

		viewState = 'ready';
	});
</script>

<svelte:head>
	<title>{$i18n.t('Images')} | {$WEBUI_NAME}</title>
</svelte:head>

{#if loaded}
	<!-- 标签页导航 -->
	<div class="glass-item p-1 mb-4">
		<div class="flex items-center gap-1">
			<button
				class="tab-button {activeTab === 'workbench' ? 'active' : ''}"
				on:click={() => (activeTab = 'workbench')}
			>
				<PhotoSolid className="size-4" />
				<span>{$i18n.t('Workbench')}</span>
			</button>
			<button
				class="tab-button {activeTab === 'prompts' ? 'active' : ''}"
				on:click={() => (activeTab = 'prompts')}
			>
				<Sparkles className="size-4" />
				<span>{$i18n.t('Prompt Management')}</span>
			</button>
			<button
				class="tab-button {activeTab === 'gallery' ? 'active' : ''}"
				on:click={() => (activeTab = 'gallery')}
			>
				<PhotoSolid className="size-4" />
				<span>{$i18n.t('Gallery')}</span>
			</button>
			<button
				class="tab-button {activeTab === 'history' ? 'active' : ''}"
				on:click={() => (activeTab = 'history')}
			>
				<Clipboard className="size-4" />
				<span>{$i18n.t('History')}</span>
			</button>
		</div>
	</div>

	{#if viewState !== 'ready' || workspaceNoModels}
		<div class="space-y-4">
			<section class="workspace-section space-y-4">
				<div class="flex flex-col gap-3 lg:flex-row lg:items-center">
					<div class="workspace-toolbar-summary">
						<div class="workspace-count-pill">
							<PhotoSolid className="size-3.5" />
							{$i18n.t('Image Studio')}
						</div>
						<div class="text-xs text-gray-500 dark:text-gray-400">
							{blockedReason ?? $i18n.t('Loading image generation settings...')}
						</div>
					</div>
				</div>
			</section>

			<section class="workspace-section">
				<div class="workspace-empty-state">
					<div class="flex size-14 mx-auto items-center justify-center rounded-2xl bg-gray-100 text-gray-400 dark:bg-gray-800 dark:text-gray-500">
						<PhotoSolid className="size-7" />
					</div>
					<h2 class="mt-4 text-base font-semibold text-gray-900 dark:text-white">
						{viewState === 'denied'
							? $i18n.t('Image generation access required')
							: viewState === 'disabled'
								? $i18n.t('Image generation is disabled')
								: viewState === 'error'
									? $i18n.t('Unable to load image generation')
									: workspaceNoModels
										? $i18n.t('No models available')
										: $i18n.t('Loading...')}
					</h2>
					<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
						{blockedReason ??
							(viewState === 'loading'
								? $i18n.t('Loading image generation settings...')
								: $i18n.t('Please try again later.'))}
					</p>
					<div class="mt-5 flex flex-wrap justify-center gap-2">
						<button type="button" class="workspace-secondary-button text-xs" on:click={() => location.reload()}>
							{$i18n.t('Refresh')}
						</button>
					</div>
				</div>
			</section>
		</div>
	{:else}
		{#if activeTab === 'workbench'}
			<form class="space-y-4" on:submit|preventDefault={submitHandler}>
			<section class="workspace-section space-y-4">
				<div class="flex flex-col gap-3 lg:flex-row lg:items-center">
					<div class="workspace-toolbar-summary">
						<div class="workspace-count-pill">
							<PhotoSolid className="size-3.5" />
							{$i18n.t('Image Studio')}
						</div>
						<div class="space-y-1 text-xs text-gray-500 dark:text-gray-400">
							<div>
								{$i18n.t('Create polished visuals from a single prompt.')}
								<span class="hidden sm:inline ml-1 opacity-70">
									{$i18n.t('Press Ctrl/Command + Enter to generate.')}
								</span>
							</div>
							<div class="opacity-80">
								{$i18n.t(
									'This image workbench remembers your last model and generation settings only in this browser.'
								)}
							</div>
							<div class="opacity-80">
								{$i18n.t(
									'Shows all currently available image models. The suffix in the name indicates the source channel.'
								)}
							</div>
						</div>
					</div>

					<div class="workspace-toolbar">
						<HaloSelect
							bind:value={selectedModel}
							options={modelOptions}
							placeholder={$i18n.t('Select a model')}
							searchEnabled={true}
							searchPlaceholder={$i18n.t('Search a model')}
							noResultsText={$i18n.t('No results found')}
							className="w-full lg:w-72 text-xs"
						/>

						<div class="workspace-toolbar-actions">
							<button
								type="submit"
								class="workspace-primary-button"
								disabled={!canSubmit}
								title={blockedReason ?? ''}
							>
								{#if loading}
									<svg class="size-4 animate-spin" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
										<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
										<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
									</svg>
								{:else}
									<Sparkles className="size-4" strokeWidth="2" />
								{/if}
								<span>{loading ? $i18n.t('Generating...') : $i18n.t('Generate')}</span>
							</button>
						</div>
					</div>
				</div>
			</section>

			<section class="workspace-section space-y-4">
				<div class="glass-item p-4 space-y-3">
					<div class="flex items-center justify-between">
						<div class="text-sm font-semibold text-gray-900 dark:text-gray-100">
							{$i18n.t('Main Prompt')}
						</div>
						<Sparkles className="size-4 text-gray-400" />
					</div>

					<textarea
						rows="5"
						bind:value={prompt}
						on:keydown={handleComposerKeydown}
						placeholder={$i18n.t('Describe the image you want to generate...')}
						class="min-h-[8rem] w-full resize-none rounded-xl border border-gray-200/60 bg-white/85 p-3 text-sm leading-6 text-gray-900 outline-none placeholder:text-gray-400 dark:border-gray-700/50 dark:bg-gray-900/70 dark:text-gray-100 dark:placeholder:text-gray-500"
					/>

					<div class="flex flex-wrap gap-1.5">
						{#each promptIdeas as idea}
							<button
								type="button"
								class="rounded-full border border-gray-200/60 bg-white/85 px-2.5 py-1 text-xs text-gray-600 transition hover:bg-gray-50 dark:border-gray-700/50 dark:bg-gray-900/70 dark:text-gray-400 dark:hover:bg-gray-800"
								on:click={() => applyPromptIdea(idea)}
							>
								{$i18n.t(idea)}
							</button>
						{/each}
					</div>
				</div>

				<!-- 风格预设 -->
				<div class="glass-item p-4 space-y-3">
					<div class="flex items-center justify-between">
						<div class="text-sm font-semibold text-gray-900 dark:text-gray-100">
							{$i18n.t('Style Presets')}
						</div>
					</div>

					<div class="grid grid-cols-2 sm:grid-cols-4 gap-2">
						{#each BUILT_IN_STYLE_PRESETS as preset}
							<button
								type="button"
								class="style-preset-card"
								on:click={() => applyStylePreset(preset)}
							>
								<div class="text-2xl">{preset.icon}</div>
								<div class="text-xs font-medium text-gray-700 dark:text-gray-300 text-center">
									{$i18n.t(preset.nameKey)}
								</div>
							</button>
						{/each}
					</div>
				</div>

				<!-- 负面提示词 -->
				<div class="glass-item p-4 space-y-3">
					<div class="flex items-center justify-between">
						<div class="text-sm font-semibold text-gray-900 dark:text-gray-100">
							{$i18n.t('Negative Prompt')}
						</div>
						<button
							type="button"
							class="text-xs text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 transition"
							on:click={() => (showNegativePrompt = !showNegativePrompt)}
						>
							{showNegativePrompt ? $i18n.t('Hide') : $i18n.t('Show')}
						</button>
					</div>

					{#if showNegativePrompt}
						<textarea
							rows="3"
							bind:value={negativePrompt}
							placeholder={$i18n.t('Describe what you want to avoid in the image...')}
							class="min-h-[6rem] w-full resize-none rounded-xl border border-gray-200/60 bg-white/85 p-3 text-sm leading-6 text-gray-900 outline-none placeholder:text-gray-400 dark:border-gray-700/50 dark:bg-gray-900/70 dark:text-gray-100 dark:placeholder:text-gray-500"
						/>

						<div class="flex flex-wrap gap-1.5">
							{#each negativePromptSuggestions as suggestion}
								<button
									type="button"
									class="rounded-full border border-gray-200/60 bg-white/85 px-2.5 py-1 text-xs text-gray-600 transition hover:bg-gray-50 dark:border-gray-700/50 dark:bg-gray-900/70 dark:text-gray-400 dark:hover:bg-gray-800"
									on:click={() => addNegativePromptSuggestion(suggestion)}
								>
									{$i18n.t(suggestion)}
								</button>
							{/each}
						</div>
					{/if}
				</div>

				<div class="glass-item p-4 space-y-4">
					<div class="text-sm font-semibold text-gray-900 dark:text-gray-100">
						{$i18n.t('Generation Settings')}
					</div>

					<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
						<div class="space-y-1.5">
							<div class="text-xs font-medium text-gray-500 dark:text-gray-400">
								{$i18n.t('Model')}
							</div>
							<HaloSelect
								bind:value={selectedModel}
								options={modelOptions}
								placeholder={$i18n.t('Select a model')}
								searchEnabled={true}
								searchPlaceholder={$i18n.t('Search a model')}
								noResultsText={$i18n.t('No results found')}
								className="w-full text-xs"
							/>
						</div>

						{#if usesNativeAspectRatioControls}
							<div class="space-y-1.5">
								<div class="text-xs font-medium text-gray-500 dark:text-gray-400">
									{$i18n.t('Aspect Ratio')}
								</div>
								<HaloSelect
									bind:value={selectedAspectRatioOption}
									options={nativeAspectRatioOptions}
									className="w-full text-xs"
								/>
								<div class="text-xs text-gray-500 dark:text-gray-400">
									{$i18n.t('This model uses aspect ratio instead of exact pixel size.')}
								</div>
							</div>
						{:else}
							<div class="space-y-1.5">
								<div class="text-xs font-medium text-gray-500 dark:text-gray-400">
									{$i18n.t('Size')}
								</div>
								<HaloSelect
									value={sizeSelectValue}
									options={sizeOptions.map((option) => ({
										value: option.value,
										label:
											option.value === CUSTOM_SIZE_OPTION_VALUE
												? option.label
												: `${option.ratio} · ${option.label}`
									}))}
									className="w-full text-xs"
									on:change={(event) => handleSizeSelect(event.detail.value)}
								/>

								{#if usingCustomSize}
									<div class="space-y-2 rounded-xl border border-dashed border-gray-200/80 bg-gray-50/70 p-3 dark:border-gray-700/60 dark:bg-gray-900/40">
										<div class="flex items-center justify-between gap-2">
											<div class="text-xs font-medium text-gray-700 dark:text-gray-200">
												{$i18n.t('Custom size')}
											</div>
											<button
												type="button"
												class="text-xs font-medium text-gray-500 transition hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
												on:click={restorePresetSize}
											>
												{$i18n.t('Restore preset')}
											</button>
										</div>
										<input
											bind:value={customSizeInput}
											placeholder="1344x768"
											class="w-full rounded-xl border border-gray-200/80 bg-white px-3 py-2 text-sm text-gray-800 outline-none transition focus:border-gray-300 dark:border-gray-700/60 dark:bg-gray-950/70 dark:text-gray-100 dark:focus:border-gray-600"
										/>
										<div class="flex flex-wrap gap-x-3 gap-y-1 text-[11px] text-gray-500 dark:text-gray-400">
											<div>{$i18n.t('Enter a custom size like {{example}}.', { example: '1344x768' })}</div>
											{#if activeSizeParsed}
												<div>
													{$i18n.t('Total pixels')}: {formatPixelCount(activeSizeParsed.pixels)}
												</div>
											{/if}
										</div>
									</div>
								{/if}

								{#if selectedModelMeta?.size_mode === 'aspect_ratio' && selectedAspectRatio}
									<div class="text-xs text-amber-600 dark:text-amber-400">
										{$i18n.t(
											'This model only accepts aspect ratio requests. {{size}} will be sent as {{ratio}}, not as an exact pixel size.',
											{ size: activeSizeLabel, ratio: selectedAspectRatio }
										)}
									</div>
								{/if}

								{#if sizeValidation}
									<div class="rounded-xl border border-amber-200 bg-amber-50 p-3 text-xs text-amber-800 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200">
										<div class="font-semibold">{sizeValidation.title}</div>
										<div class="mt-1 leading-5">{sizeValidation.description}</div>
										{#if currentConstraint?.requestId}
											<div class="mt-2 opacity-80">
												{$i18n.t('Request ID')}: {currentConstraint.requestId}
											</div>
										{/if}
										{#if recommendedSizes.length > 0}
											<div class="mt-3">
												<div class="mb-2 font-medium">{$i18n.t('Recommended sizes')}</div>
												<div class="flex flex-wrap gap-2">
													{#each recommendedSizes as size}
														<button
															type="button"
															class="rounded-full border border-amber-300 bg-white px-3 py-1 font-medium text-amber-700 transition hover:bg-amber-100 dark:border-amber-500/30 dark:bg-transparent dark:text-amber-200 dark:hover:bg-amber-500/20"
															on:click={() => applyRecommendedSize(size)}
														>
															{size}
														</button>
													{/each}
												</div>
											</div>
										{/if}
									</div>
								{/if}
							</div>
						{/if}

						<div class="space-y-1.5">
							{#if showsResolutionControl}
								<div class="text-xs font-medium text-gray-500 dark:text-gray-400">
									{$i18n.t('Resolution')}
								</div>
								<HaloSelect
									bind:value={selectedResolution}
									options={nativeResolutionOptions}
									className="w-full text-xs"
								/>
							{:else if showsStepsControl}
								<div class="flex items-center justify-between">
									<div class="text-xs font-medium text-gray-500 dark:text-gray-400">
										{$i18n.t('Set Steps')}
									</div>
									<div class="text-xs font-semibold text-gray-900 dark:text-gray-100">
										{steps === 0 ? $i18n.t('Auto') : steps}
									</div>
								</div>
								<input
									type="range"
									min="0"
									max="80"
									step="5"
									bind:value={steps}
									class="image-range mt-1 h-2 w-full cursor-pointer appearance-none rounded-full bg-gray-200 dark:bg-gray-800"
								/>
							{/if}
						</div>

						<!-- 生成数量 -->
						<div class="space-y-1.5">
							<div class="text-xs font-medium text-gray-500 dark:text-gray-400">
								{$i18n.t('Number of Images')}
							</div>
							<HaloSelect
								bind:value={numberOfImages}
								options={[
									{ value: '1', label: '1' },
									{ value: '2', label: '2' },
									{ value: '3', label: '3' },
									{ value: '4', label: '4' }
								]}
								className="w-full text-xs"
							/>
							<div class="text-xs text-gray-500 dark:text-gray-400">
								{$i18n.t('Generate multiple images at once')}
							</div>
						</div>

						<!-- 背景参数 -->
						<div class="space-y-1.5">
							<div class="text-xs font-medium text-gray-500 dark:text-gray-400">
								{$i18n.t('Background')}
							</div>
							<HaloSelect
								bind:value={background}
								options={[
									{ value: 'auto', label: $i18n.t('Auto') },
									{ value: 'transparent', label: $i18n.t('Transparent') },
									{ value: 'white', label: $i18n.t('White') },
									{ value: 'black', label: $i18n.t('Black') },
									{ value: 'custom', label: $i18n.t('Custom') }
								]}
								className="w-full text-xs"
							/>

							{#if background === 'custom'}
								<input
									bind:value={customBackground}
									placeholder={$i18n.t('Enter background description')}
									class="w-full rounded-xl border border-gray-200/80 bg-white px-3 py-2 text-sm text-gray-800 outline-none transition focus:border-gray-300 dark:border-gray-700/60 dark:bg-gray-950/70 dark:text-gray-100 dark:focus:border-gray-600"
								/>
							{/if}
						</div>
					</div>

					<!-- 模板保存 -->
					<div class="mt-4 pt-4 border-t border-gray-200/60 dark:border-gray-700/50">
						<div class="text-sm font-semibold text-gray-900 dark:text-gray-100 mb-3">
							{$i18n.t('Save as Template')}
						</div>

						<div class="space-y-2">
							<input
								bind:value={templateName}
								placeholder={$i18n.t('Template name')}
								class="w-full rounded-xl border border-gray-200/80 bg-white px-3 py-2 text-sm text-gray-800 outline-none transition focus:border-gray-300 dark:border-gray-700/60 dark:bg-gray-950/70 dark:text-gray-100 dark:focus:border-gray-600"
							/>

							<input
								bind:value={templateTags}
								placeholder={$i18n.t('Tags (comma separated)')}
								class="w-full rounded-xl border border-gray-200/80 bg-white px-3 py-2 text-sm text-gray-800 outline-none transition focus:border-gray-300 dark:border-gray-700/60 dark:bg-gray-950/70 dark:text-gray-100 dark:focus:border-gray-600"
							/>

							<button
								type="button"
								class="workspace-secondary-button w-full"
								on:click={saveTemplate}
								disabled={!templateName.trim()}
							>
								<svg class="size-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path
										stroke-linecap="round"
										stroke-linejoin="round"
										stroke-width="2"
										d="M5 5a2 2 0 012-2h10a2 2 0 012 2v16l-7-3.5L5 21V5z"
									/>
								</svg>
								<span>{$i18n.t('Save Template')}</span>
							</button>
						</div>

						{#if savedTemplates.length > 0}
							<div class="border-t border-gray-200/60 dark:border-gray-700/50 pt-3 mt-3">
								<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-2">
									{$i18n.t('Saved Templates')} ({savedTemplates.length})
								</div>
								<div class="space-y-1">
									{#each savedTemplates.slice(0, 3) as template}
										<button
											type="button"
											class="w-full flex items-center justify-between p-2 rounded-lg hover:bg-gray-100/50 dark:hover:bg-gray-800/50 transition"
											on:click={() => loadTemplate(template)}
										>
											<div class="text-left min-w-0 flex-1">
												<div class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate">
													{template.name}
												</div>
												{#if template.tags.length > 0}
													<div class="text-xs text-gray-500 dark:text-gray-400 truncate">
														{template.tags.join(', ')}
													</div>
												{/if}
											</div>
											<svg class="size-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
												<path
													stroke-linecap="round"
													stroke-linejoin="round"
													stroke-width="2"
													d="M9 5l7 7-7 7"
												/>
											</svg>
										</button>
									{/each}
									{#if savedTemplates.length > 3}
										<button
											type="button"
											class="w-full text-xs text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 py-1 transition"
											on:click={() => (activeTab = 'prompts')}
										>
											{$i18n.t('View all')} ({savedTemplates.length})
										</button>
									{/if}
								</div>
							</div>
						{/if}
					</div>
				</div>
			</section>

			<section bind:this={resultsSectionElement} class="workspace-section space-y-3">
				<div class="flex items-center justify-between">
					<div class="min-w-0 flex-1">
						<div class="text-sm font-semibold text-gray-900 dark:text-gray-100">
							{$i18n.t('Recent Result')}
						</div>
						{#if lastPrompt}
							<div class="mt-0.5 text-xs text-gray-500 dark:text-gray-400 truncate">
								{lastPrompt}
							</div>
						{/if}
					</div>
					{#if lastPrompt}
						<button type="button" class="workspace-icon-button" on:click={copyPromptHandler}>
							<Clipboard className="size-3.5" />
							<span class="text-xs">{$i18n.t('Copy prompt')}</span>
						</button>
					{/if}
				</div>

				{#if loading}
					<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
						<div class="shimmer h-56 rounded-xl glass-item" />
					</div>
				{:else if generatedImages.length > 0}
					<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
						{#each generatedImages as image, index}
							<div
								role="button"
								tabindex="0"
								class="result-card glass-item group overflow-hidden cursor-pointer p-1.5"
								style={`animation-delay: ${index * 60}ms;`}
								on:click={() => openPreview(image, index)}
								on:keydown={(event) => {
									if (event.key === 'Enter' || event.key === ' ') {
										event.preventDefault();
										openPreview(image, index);
									}
								}}
							>
								<div class="relative overflow-hidden rounded-lg bg-gray-100 dark:bg-gray-950">
									<img
										src={image.url}
										alt={`${$i18n.t('Generated image')} ${index + 1}`}
										class="max-h-[22rem] min-h-[12rem] w-full object-cover transition duration-500 group-hover:scale-[1.02]"
										loading="lazy"
									/>
									<div
										class="absolute inset-x-0 bottom-0 flex items-end justify-between gap-2 bg-gradient-to-t from-black/70 via-black/20 to-transparent p-3 text-white opacity-0 transition duration-200 group-hover:opacity-100"
									>
										<div class="text-xs font-medium">{activeSizeLabel}</div>
										<div class="flex items-center gap-2">
											<button
												type="button"
												class="rounded-xl bg-white/15 p-2 backdrop-blur transition hover:bg-white/25"
												on:click|stopPropagation={() => openPreview(image, index)}
												aria-label={$i18n.t('Open preview')}
											>
												<ArrowsPointingOut className="size-4" />
											</button>
											<button
												type="button"
												class="rounded-xl bg-white/15 p-2 backdrop-blur transition hover:bg-white/25"
												on:click|stopPropagation={() => downloadImage(image.url, index)}
												aria-label={$i18n.t('Save image')}
											>
												<ArrowDownTray className="size-4" />
											</button>
										</div>
									</div>
								</div>
							</div>
						{/each}
					</div>
				{:else}
					<div class="workspace-empty-state">
						<div class="flex size-14 mx-auto items-center justify-center rounded-2xl bg-gray-100 text-gray-400 dark:bg-gray-800 dark:text-gray-500">
							<PhotoSolid className="size-7" />
						</div>
						<div class="mt-4 text-base font-semibold text-gray-900 dark:text-gray-100">
							{$i18n.t('Your images will appear here after generation.')}
						</div>
						<div class="mt-2 max-w-sm mx-auto text-sm leading-6 text-gray-500 dark:text-gray-400">
							{$i18n.t('Start with a strong subject, then add lighting, composition, materials, and mood for better results.')}
						</div>
					</div>
				{/if}
			</section>
		</form>
	{:else if activeTab === 'prompts'}
		<!-- 提示词管理标签页 -->
		<div class="space-y-4">
			<div class="workspace-section">
				<div class="flex items-center justify-between mb-4">
					<div class="text-lg font-semibold text-gray-900 dark:text-gray-100">
						{$i18n.t('Saved Templates')}
					</div>
				</div>

				{#if savedTemplates.length > 0}
					<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
						{#each savedTemplates as template}
							<div class="glass-item p-4 space-y-3">
								<div class="flex items-start justify-between">
									<div class="min-w-0 flex-1">
										<div class="text-sm font-semibold text-gray-900 dark:text-gray-100 truncate">
											{template.name}
										</div>
										{#if template.tags.length > 0}
											<div class="flex flex-wrap gap-1 mt-1">
												{#each template.tags as tag}
													<span
														class="text-xs px-2 py-0.5 rounded-full bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400"
													>
														{tag}
													</span>
												{/each}
											</div>
										{/if}
									</div>
								</div>

								{#if template.config.prompt}
									<div class="text-xs text-gray-600 dark:text-gray-400 line-clamp-2">
										{template.config.prompt}
									</div>
								{/if}

								<div class="flex gap-2">
									<button
										type="button"
										class="workspace-secondary-button w-full text-xs"
										on:click={() => {
											loadTemplate(template);
											activeTab = 'workbench';
										}}
									>
										{$i18n.t('Load')}
									</button>
									<button
										type="button"
										class="workspace-secondary-button text-xs px-3"
										on:click={() => deleteTemplate(template.id)}
									>
										{$i18n.t('Delete')}
									</button>
								</div>
							</div>
						{/each}
					</div>
				{:else}
					<div class="workspace-empty-state">
						<div
							class="flex size-14 mx-auto items-center justify-center rounded-2xl bg-gray-100 text-gray-400 dark:bg-gray-800 dark:text-gray-500"
						>
							<Sparkles className="size-7" />
						</div>
						<div class="mt-4 text-base font-semibold text-gray-900 dark:text-gray-100">
							{$i18n.t('No templates saved yet')}
						</div>
						<div class="mt-2 text-sm text-gray-500 dark:text-gray-400">
							{$i18n.t('Save your favorite prompts and settings as templates for quick reuse.')}
						</div>
					</div>
				{/if}
			</div>
		</div>
	{:else if activeTab === 'gallery'}
		<!-- 图库标签页 -->
		<div class="space-y-4">
			<div class="workspace-section">
				<div class="flex items-center justify-between mb-4">
					<div class="text-lg font-semibold text-gray-900 dark:text-gray-100">
						{$i18n.t('Image Gallery')}
					</div>
					<HaloSelect
						bind:value={gallerySortBy}
						options={[
							{ value: 'recent', label: $i18n.t('Recent') },
							{ value: 'favorites', label: $i18n.t('Favorites') }
						]}
						className="text-xs w-32"
					/>
				</div>

				<div class="workspace-search mb-4">
					<svg class="size-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
						/>
					</svg>
					<input
						bind:value={gallerySearchQuery}
						placeholder={$i18n.t('Search images...')}
						class="flex-1 bg-transparent outline-none text-sm"
					/>
				</div>

				{#if sortedGalleryImages.length > 0}
					<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
						{#each sortedGalleryImages as image}
							<div class="glass-item p-1.5 group">
								<div class="relative overflow-hidden rounded-lg bg-gray-100 dark:bg-gray-950">
									<img
										src={image.url}
										alt={image.prompt}
										class="w-full h-48 object-cover transition duration-500 group-hover:scale-[1.02] cursor-pointer"
										loading="lazy"
										on:click={() => {
											previewSrc = image.url;
											previewAlt = image.prompt;
											previewOpen = true;
										}}
									/>
									<div class="absolute top-2 right-2">
										<button
											type="button"
											class="rounded-lg bg-white/90 dark:bg-gray-900/90 p-1.5 backdrop-blur transition hover:bg-white dark:hover:bg-gray-900"
											on:click={() => toggleFavorite(image.id)}
										>
											<svg
												class="size-4 {image.favorite
													? 'text-yellow-500 fill-yellow-500'
													: 'text-gray-400'}"
												fill={image.favorite ? 'currentColor' : 'none'}
												stroke="currentColor"
												viewBox="0 0 24 24"
											>
												<path
													stroke-linecap="round"
													stroke-linejoin="round"
													stroke-width="2"
													d="M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z"
												/>
											</svg>
										</button>
									</div>
								</div>
								<div class="p-2">
									<div class="text-xs text-gray-600 dark:text-gray-400 line-clamp-2">
										{image.prompt}
									</div>
									<div class="text-xs text-gray-500 dark:text-gray-500 mt-1">
										{new Date(image.createdAt).toLocaleDateString()}
									</div>
								</div>
							</div>
						{/each}
					</div>
				{:else}
					<div class="workspace-empty-state">
						<div
							class="flex size-14 mx-auto items-center justify-center rounded-2xl bg-gray-100 text-gray-400 dark:bg-gray-800 dark:text-gray-500"
						>
							<PhotoSolid className="size-7" />
						</div>
						<div class="mt-4 text-base font-semibold text-gray-900 dark:text-gray-100">
							{$i18n.t('No images in gallery')}
						</div>
						<div class="mt-2 text-sm text-gray-500 dark:text-gray-400">
							{$i18n.t('Generated images will be saved here automatically.')}
						</div>
					</div>
				{/if}
			</div>
		</div>
	{:else if activeTab === 'history'}
		<!-- 历史任务标签页 -->
		<div class="space-y-4">
			<div class="workspace-section">
				<div class="flex items-center justify-between mb-4">
					<div class="text-lg font-semibold text-gray-900 dark:text-gray-100">
						{$i18n.t('Generation History')}
					</div>
					{#if generationHistory.length > 0}
						<button
							type="button"
							class="workspace-secondary-button text-xs"
							on:click={clearHistory}
						>
							{$i18n.t('Clear History')}
						</button>
					{/if}
				</div>

				{#if generationHistory.length > 0}
					<div class="space-y-2">
						{#each generationHistory as item}
							<div class="glass-item p-4">
								<div class="flex items-start gap-3">
									<div class="shrink-0">
										{#if item.status === 'success'}
											<div
												class="size-8 rounded-lg bg-green-100 dark:bg-green-900/30 flex items-center justify-center"
											>
												<svg
													class="size-4 text-green-600 dark:text-green-400"
													fill="currentColor"
													viewBox="0 0 20 20"
												>
													<path
														fill-rule="evenodd"
														d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
														clip-rule="evenodd"
													/>
												</svg>
											</div>
										{:else}
											<div
												class="size-8 rounded-lg bg-red-100 dark:bg-red-900/30 flex items-center justify-center"
											>
												<svg
													class="size-4 text-red-600 dark:text-red-400"
													fill="currentColor"
													viewBox="0 0 20 20"
												>
													<path
														fill-rule="evenodd"
														d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
														clip-rule="evenodd"
													/>
												</svg>
											</div>
										{/if}
									</div>

									<div class="min-w-0 flex-1">
										<div class="text-sm text-gray-900 dark:text-gray-100 line-clamp-2">
											{item.prompt}
										</div>
										<div class="flex items-center gap-2 mt-1 text-xs text-gray-500 dark:text-gray-400">
											<span>{item.model}</span>
											<span>•</span>
											<span>{new Date(item.createdAt).toLocaleString()}</span>
										</div>

										{#if item.status === 'success' && item.images}
											<div class="flex gap-2 mt-2">
												{#each item.images.slice(0, 4) as imageUrl}
													<img
														src={imageUrl}
														alt=""
														class="size-16 rounded-lg object-cover cursor-pointer hover:opacity-80 transition"
														on:click={() => {
															previewSrc = imageUrl;
															previewAlt = item.prompt;
															previewOpen = true;
														}}
													/>
												{/each}
											</div>
										{/if}

										{#if item.status === 'failed' && item.error}
											<div class="mt-2 text-xs text-red-600 dark:text-red-400">
												{item.error}
											</div>
										{/if}
									</div>
								</div>
							</div>
						{/each}
					</div>
				{:else}
					<div class="workspace-empty-state">
						<div
							class="flex size-14 mx-auto items-center justify-center rounded-2xl bg-gray-100 text-gray-400 dark:bg-gray-800 dark:text-gray-500"
						>
							<Clipboard className="size-7" />
						</div>
						<div class="mt-4 text-base font-semibold text-gray-900 dark:text-gray-100">
							{$i18n.t('No generation history')}
						</div>
						<div class="mt-2 text-sm text-gray-500 dark:text-gray-400">
							{$i18n.t('Your generation history will appear here.')}
						</div>
					</div>
				{/if}
			</div>
		</div>
	{/if}
	{/if}
{/if}

<ImagePreview
	bind:show={previewOpen}
	src={previewSrc}
	alt={previewAlt}
/>
