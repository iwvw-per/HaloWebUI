<script lang="ts">
		import { getContext, onMount, tick } from 'svelte';
		import { toast } from 'svelte-sonner';
		import { get } from 'svelte/store';

		import {
			getImageGenerationModels,
			getImageUsageConfig,
			imageGenerations
		} from '$lib/apis/images';
		import type { ImageGenerationModel, ImageUsageConfig } from '$lib/apis/images';
		import { updateUserSettings } from '$lib/apis/users';
		import HaloSelect from '$lib/components/common/HaloSelect.svelte';
		import ConfirmDialog from '$lib/components/common/ConfirmDialog.svelte';
		import ImagePreview from '$lib/components/common/ImagePreview.svelte';
		import ArrowDownTray from '$lib/components/icons/ArrowDownTray.svelte';
		import ArrowsPointingOut from '$lib/components/icons/ArrowsPointingOut.svelte';
		import Clipboard from '$lib/components/icons/Clipboard.svelte';
		import PhotoSolid from '$lib/components/icons/PhotoSolid.svelte';
		import Sparkles from '$lib/components/icons/Sparkles.svelte';
		import { WEBUI_NAME, settings, user } from '$lib/stores';
		import { copyToClipboard } from '$lib/utils';

		type GeneratedImage = {
			url: string;
		};

		type CredentialSource = 'auto' | 'personal' | 'shared';
		type ViewState = 'loading' | 'ready' | 'disabled' | 'denied' | 'error';

	type SizeOption = {
		value: string;
		ratio: string;
		label: string;
	};

	const i18n = getContext('i18n');

	export let variant: 'playground' | 'workspace' = 'playground';

	const promptIdeas = [
		'Cinematic portrait',
		'Clean product shot',
		'Editorial poster',
		'Cozy illustration',
		'Neon cityscape',
		'Minimal interior'
	];

	const curatedSizeOptions: SizeOption[] = [
		{ value: '1024x1024', ratio: '1:1', label: '1024x1024' },
		{ value: '1024x1536', ratio: '2:3', label: '1024x1536' },
		{ value: '1536x1024', ratio: '3:2', label: '1536x1024' }
	];

	const batchOptions = [1, 2, 4];
	const backgroundOptions: Array<'auto' | 'transparent' | 'opaque'> = [
		'auto',
		'transparent',
		'opaque'
	];

		let loaded = false;
		let loading = false;
		let viewState: ViewState = 'loading';
		let loadError: string | null = null;
		let usageConfig: ImageUsageConfig | null = null;
		let engine: string = 'openai';
		let sharedKeyEnabled = false;
		let sharedKeyAvailable = false;
		let personalKeySupported = false;
		let personalProvider: 'openai' | 'gemini' | null = null;
		let showCredentialControls = false;
		let providerConnections: { urls: string[]; keys: string[] } = { urls: [], keys: [] };
		let firstPersonalIndex: number | null = null;
		let selectedPersonalIndex = 0;
		let selectedPersonalUsable = false;

		let credentialSource: CredentialSource = 'auto';
		let openaiConnectionIndex = 0;
		let geminiConnectionIndex = 0;
		let sharedConfirmed = false;

		let showSharedKeyConfirm = false;
		let pendingCredentialSource: CredentialSource | null = null;
		let blockedReason: string | null = null;
		let resolvedCredential: { source: 'personal' | 'shared' | 'none'; connectionIndex?: number | null } =
			{ source: 'none', connectionIndex: null };
		let canSubmit = false;

		let prompt = '';
		let negativePrompt = '';

	let imageModels: ImageGenerationModel[] = [];
	let selectedModel = '';
	let defaultSize = '512x512';
	let selectedSize = '512x512';
	let batchSize = 1;
	let steps = 0;
	let background: 'auto' | 'transparent' | 'opaque' = 'auto';
	let modelRequestVersion = 0;
	let lastModelRequestKey = '';

	let generatedImages: GeneratedImage[] = [];
	let lastPrompt = '';

	let previewOpen = false;
	let previewSrc = '';
	let previewAlt = '';

	let resultsSectionElement: HTMLElement | null = null;

	const segmentedButtonClass = (active: boolean) =>
		`group flex min-w-0 flex-1 flex-col rounded-2xl border px-3 py-3 text-left transition-all duration-200 ${
			active
				? 'border-gray-900 bg-gray-900 text-white shadow-lg shadow-gray-900/10 dark:border-white dark:bg-white dark:text-gray-900 dark:shadow-gray-100/10'
				: 'border-gray-200/80 bg-white text-gray-700 hover:border-gray-300 hover:bg-gray-50 dark:border-gray-800 dark:bg-gray-900/70 dark:text-gray-300 dark:hover:border-gray-700 dark:hover:bg-gray-900'
		}`;

		const ghostButtonClass =
			'inline-flex items-center gap-2 rounded-xl border border-gray-200/80 bg-white px-3 py-2 text-sm font-medium text-gray-700 transition hover:border-gray-300 hover:bg-gray-50 dark:border-gray-800 dark:bg-gray-900/70 dark:text-gray-300 dark:hover:border-gray-700 dark:hover:bg-gray-900';

		const sourcePillClass = (active: boolean, disabled: boolean = false) =>
			`inline-flex items-center justify-center rounded-xl border px-3 py-2 text-xs font-semibold transition ${
				active
					? 'border-gray-900 bg-gray-900 text-white shadow-sm dark:border-white dark:bg-white dark:text-gray-900'
					: 'border-gray-200 bg-white text-gray-700 dark:border-gray-800 dark:bg-gray-900/70 dark:text-gray-300'
			} ${
				disabled
					? 'opacity-50 cursor-not-allowed'
					: 'hover:border-gray-300 hover:bg-gray-50 dark:hover:border-gray-700 dark:hover:bg-gray-900'
			}`;

		const providerFromEngine = (value: string): 'openai' | 'gemini' | null => {
			const engine = (value || '').toLowerCase();
			if (engine === 'openai') return 'openai';
			if (engine === 'gemini') return 'gemini';
			return null;
		};

		$: cardShellClass =
			variant === 'workspace'
				? 'workspace-section relative overflow-hidden'
				: 'relative overflow-hidden rounded-[28px] border border-gray-200/80 bg-gradient-to-br from-gray-50 via-white to-gray-100/80 p-6 shadow-sm dark:border-gray-800 dark:from-gray-900 dark:via-gray-950 dark:to-gray-900';

		$: introTitleClass =
			variant === 'workspace'
				? 'text-base font-semibold tracking-tight text-gray-900 dark:text-white'
				: 'text-2xl font-bold tracking-tight text-gray-900 dark:text-white';

		$: readyTitleClass =
			variant === 'workspace'
				? 'text-base font-semibold tracking-tight text-gray-900 dark:text-white'
				: 'text-2xl font-bold tracking-tight text-gray-900 dark:text-white';

		const getProviderConnections = (provider: 'openai' | 'gemini') => {
			const root = (($settings as any)?.connections ?? {}) as any;
			const cfg = (root?.[provider] ?? {}) as any;

			const urlsKey = provider === 'openai' ? 'OPENAI_API_BASE_URLS' : 'GEMINI_API_BASE_URLS';
			const keysKey = provider === 'openai' ? 'OPENAI_API_KEYS' : 'GEMINI_API_KEYS';

			const urls = Array.isArray(cfg?.[urlsKey]) ? cfg[urlsKey].map((v) => `${v ?? ''}`) : [];
			const keys = Array.isArray(cfg?.[keysKey]) ? cfg[keysKey].map((v) => `${v ?? ''}`) : [];

			const alignedKeys =
				keys.length >= urls.length
					? keys.slice(0, urls.length)
					: [...keys, ...Array(urls.length - keys.length).fill('')];

			return { urls, keys: alignedKeys };
		};

		const firstUsableConnectionIndex = (urls: string[], keys: string[]) => {
			for (let i = 0; i < Math.min(urls.length, keys.length); i++) {
				if (urls[i]?.trim() && keys[i]?.trim()) return i;
			}
			return null;
		};

		const isUsableConnectionIndex = (urls: string[], keys: string[], index: number) => {
			if (index < 0 || index >= urls.length) return false;
			return Boolean(urls[index]?.trim() && keys[index]?.trim());
		};

		$: modelOptions = imageModels.map((model) => ({
			value: model.id,
			label: model.name ?? model.id
		}));

	$: sizeOptions = curatedSizeOptions.some((option) => option.value === defaultSize)
		? curatedSizeOptions
		: [{ value: defaultSize, ratio: $i18n.t('Default'), label: defaultSize }, ...curatedSizeOptions];

		$: selectedModelLabel =
			modelOptions.find((option) => option.value === selectedModel)?.label ?? selectedModel;
		$: selectedModelMeta = imageModels.find((model) => model.id === selectedModel) ?? null;
		$: supportsBackground = Boolean(selectedModelMeta?.supports_background);
		$: supportsBatch = selectedModelMeta?.supports_batch ?? true;
		$: availableBatchOptions = supportsBatch ? batchOptions : [1];

		$: {
			engine = usageConfig?.engine ?? engine;
		}

		$: sharedKeyEnabled = Boolean(usageConfig?.shared_key?.enabled);
		$: sharedKeyAvailable = Boolean(usageConfig?.shared_key?.available);
		$: personalKeySupported = Boolean(usageConfig?.personal_key?.supported);
		$: personalProvider =
			usageConfig?.personal_key?.provider === 'openai' || usageConfig?.personal_key?.provider === 'gemini'
				? (usageConfig.personal_key.provider as 'openai' | 'gemini')
				: providerFromEngine(engine);

		$: showCredentialControls = personalKeySupported && personalProvider !== null;
		$: providerConnections = personalProvider ? getProviderConnections(personalProvider) : { urls: [], keys: [] };
		$: firstPersonalIndex = firstUsableConnectionIndex(providerConnections.urls, providerConnections.keys);
		$: selectedPersonalIndex = personalProvider === 'openai' ? openaiConnectionIndex : geminiConnectionIndex;
		$: selectedPersonalUsable = isUsableConnectionIndex(
			providerConnections.urls,
			providerConnections.keys,
			selectedPersonalIndex
		);

		$: {
			const requiresCredential = showCredentialControls;

			if (viewState !== 'ready') {
				resolvedCredential = { source: 'none', connectionIndex: null };
				blockedReason =
					viewState === 'denied'
						? $i18n.t('Your account does not have access to image generation.')
						: viewState === 'disabled'
							? $i18n.t('Image generation is disabled by the administrator.')
							: viewState === 'error'
								? loadError ?? $i18n.t('Failed to load image generation settings.')
								: null;
				canSubmit = false;
			} else if (!requiresCredential) {
				resolvedCredential = { source: 'shared', connectionIndex: null };
				blockedReason = null;
				canSubmit = !loading && Boolean(prompt.trim());
			} else {
			const effective =
				credentialSource === 'shared'
					? sharedKeyEnabled && sharedKeyAvailable
						? { source: 'shared' as const, connectionIndex: null }
						: { source: 'none' as const, connectionIndex: null }
					: credentialSource === 'personal'
						? selectedPersonalUsable
							? { source: 'personal' as const, connectionIndex: selectedPersonalIndex }
							: { source: 'none' as const, connectionIndex: null }
						: firstPersonalIndex !== null
							? { source: 'personal' as const, connectionIndex: firstPersonalIndex }
							: sharedKeyEnabled && sharedKeyAvailable
								? { source: 'shared' as const, connectionIndex: null }
								: { source: 'none' as const, connectionIndex: null };

			resolvedCredential = effective;

			if (effective.source === 'none') {
				if (credentialSource === 'shared' && !sharedKeyEnabled) {
					blockedReason = $i18n.t('Workspace shared key is disabled by the administrator.');
				} else if (credentialSource === 'shared' && sharedKeyEnabled && !sharedKeyAvailable) {
					blockedReason = $i18n.t('Workspace shared key is not configured.');
				} else if (credentialSource === 'personal') {
					blockedReason =
						providerConnections.urls.length === 0
							? $i18n.t('No personal connection found. Go to Settings > Connections.')
							: $i18n.t(
									'Selected connection is missing a URL or key. Update it in Settings > Connections.'
								);
				} else {
					blockedReason = $i18n.t(
						'No image generation connection configured. Add one in Settings > Connections or use the workspace shared key if available.'
					);
				}
			} else {
				blockedReason = null;
			}

			canSubmit = !loading && Boolean(prompt.trim()) && effective.source !== 'none';
			}
		}

		$: if (!supportsBackground) {
			background = 'auto';
		}
		$: if (!supportsBatch && batchSize !== 1) {
			batchSize = 1;
		}

	const syncSelectedModelWithAvailableModels = (models: ImageGenerationModel[]) => {
		const availableIds = new Set(
			(models ?? []).map((model) => `${model?.id ?? ''}`.trim()).filter(Boolean)
		);

		if (selectedModel && availableIds.has(selectedModel)) {
			return;
		}

		const defaultModelId = `${usageConfig?.defaults?.model ?? ''}`.trim();
		selectedModel =
			[selectedModel, defaultModelId, models?.[0]?.id ?? ''].find(
				(candidate) => candidate && availableIds.has(candidate)
			) ?? '';
	};

	const loadImageModels = async ({ silent = true }: { silent?: boolean } = {}) => {
		const requestVersion = ++modelRequestVersion;
		const nextModels = await getImageGenerationModels(localStorage.token, {
			context: 'runtime',
			credentialSource,
			connectionIndex:
				credentialSource === 'personal'
					? selectedPersonalIndex
					: credentialSource === 'shared'
						? null
						: resolvedCredential.source === 'personal'
							? (resolvedCredential.connectionIndex ?? null)
							: null
		}).catch((error) => {
			if (!silent) {
				toast.error(`${error}`);
			} else {
				console.log('Image models not available:', error);
			}
			return [];
		});

		if (requestVersion !== modelRequestVersion) {
			return null;
		}

		imageModels = Array.isArray(nextModels) ? nextModels : [];
		syncSelectedModelWithAvailableModels(imageModels);
		return imageModels;
	};

	const applyPromptIdea = (idea: string) => {
		const translatedIdea = $i18n.t(idea);
		prompt = prompt.trim() ? `${prompt.trim()}, ${translatedIdea}` : translatedIdea;
	};

	const setBackground = (value: 'auto' | 'transparent' | 'opaque') => {
		background = value;
	};

	const handleComposerKeydown = (event: KeyboardEvent) => {
		if ((event.metaKey || event.ctrlKey) && event.key === 'Enter' && !loading) {
			event.preventDefault();
			submitHandler();
		}
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

		let savePrefsTimer: ReturnType<typeof setTimeout> | null = null;
		const queueSavePrefs = () => {
			if (savePrefsTimer) clearTimeout(savePrefsTimer);

			savePrefsTimer = setTimeout(async () => {
				try {
					const current = (get(settings) ?? {}) as any;
					const next = {
						...current,
						imageGeneration: {
							...(current?.imageGeneration ?? {}),
							credentialSource,
							openaiConnectionIndex,
							geminiConnectionIndex,
							sharedConfirmed
						}
					};

					settings.set(next);
					await updateUserSettings(localStorage.token, { ui: next });
				} catch (err) {
					console.log('Failed to persist image generation preferences', err);
				}
			}, 450);
		};

		const requestCredentialSource = (next: CredentialSource) => {
			if (next === credentialSource) return;

			if (next === 'shared' && !sharedConfirmed) {
				pendingCredentialSource = next;
				showSharedKeyConfirm = true;
				return;
			}

			credentialSource = next;
			queueSavePrefs();
		};

		const handlePersonalConnectionChange = (value: string) => {
			const next = Number(value);
			if (!Number.isFinite(next)) return;

			if (personalProvider === 'openai') {
				openaiConnectionIndex = next;
			} else if (personalProvider === 'gemini') {
				geminiConnectionIndex = next;
			}

			queueSavePrefs();
		};

		const confirmCredentialSource = async () => {
			if (!pendingCredentialSource) return;

			if (pendingCredentialSource === 'shared') {
				sharedConfirmed = true;
			}

			credentialSource = pendingCredentialSource;
			pendingCredentialSource = null;
			queueSavePrefs();
		};

		const acknowledgeSharedKeyUsage = () => {
			if (sharedConfirmed) return;
			sharedConfirmed = true;
			queueSavePrefs();
		};

		const submitHandler = async () => {
			const trimmedPrompt = prompt.trim();
			if (!trimmedPrompt) {
				toast.error($i18n.t('Please enter a prompt'));
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
		const request: any = {
					prompt: trimmedPrompt,
					model: selectedModel || undefined,
					size: selectedSize || undefined,
					n: batchSize,
					negative_prompt: negativePrompt.trim() || undefined,
					steps: steps > 0 ? steps : undefined,
					background: supportsBackground && background !== 'auto' ? background : undefined
				};

				if (showCredentialControls) {
					request.credential_source = credentialSource;
					if (credentialSource === 'personal') {
						request.connection_index = selectedPersonalIndex;
					} else if (credentialSource !== 'shared' && resolvedCredential.source === 'personal') {
						request.connection_index = resolvedCredential.connectionIndex ?? undefined;
					}
				}

				const response = await imageGenerations(localStorage.token, {
					...request
				});

				if (response?.length) {
					generatedImages = response;
					await tick();
				resultsSectionElement?.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
			} else {
				toast.error(
					$i18n.t('Model returned an empty response. Try resending or switching models.')
				);
			}
		} catch (err) {
			toast.error(`${err}`);
		}

		loading = false;
	};

	onMount(async () => {
		// Load persisted UI preferences (per account).
		const prefs = ((($settings as any)?.imageGeneration ?? {}) as any) || {};
		const source = `${prefs.credentialSource ?? 'auto'}` as CredentialSource;
		credentialSource = (['auto', 'personal', 'shared'] as const).includes(source) ? source : 'auto';
		openaiConnectionIndex = Number.isInteger(prefs.openaiConnectionIndex) ? prefs.openaiConnectionIndex : 0;
		geminiConnectionIndex = Number.isInteger(prefs.geminiConnectionIndex) ? prefs.geminiConnectionIndex : 0;
		sharedConfirmed = Boolean(prefs.sharedConfirmed);

		const allowed =
			$user?.role === 'admin' || Boolean($user?.permissions?.features?.image_generation);
		if (!allowed) {
			viewState = 'denied';
			loaded = true;
			return;
		}

		viewState = 'loading';
		// Render the loading state immediately for a smoother UX.
		loaded = true;
		const usageResult = await getImageUsageConfig(localStorage.token).catch((error) => error);

		if (!(usageResult instanceof Error) && usageResult && typeof usageResult === 'object' && 'enabled' in usageResult) {
			usageConfig = usageResult as ImageUsageConfig;
			engine = usageConfig.engine ?? engine;

			const defaults = (usageConfig.defaults ?? {}) as any;
			if (typeof defaults.size === 'string' && defaults.size) {
				defaultSize = defaults.size;
				selectedSize = defaults.size;
			}
			if (typeof defaults.model === 'string' && defaults.model) {
				selectedModel = defaults.model;
			}
			if (typeof defaults.steps === 'number') {
				steps = defaults.steps;
			}

			viewState = usageConfig.enabled ? 'ready' : 'disabled';
		} else {
			loadError = `${usageResult ?? ''}`;
			viewState = 'error';
		}

		loaded = true;
	});

	$: if (loaded && usageConfig && viewState === 'ready') {
		const requestKey = JSON.stringify({
			engine,
			credentialSource,
			showCredentialControls,
			selectedPersonalIndex,
			firstPersonalIndex,
			personalProvider,
			sharedKeyEnabled,
			sharedKeyAvailable,
			selectedPersonalUsable
		});

		if (requestKey !== lastModelRequestKey) {
			lastModelRequestKey = requestKey;
			void loadImageModels();
		}
	}
</script>

<svelte:head>
	<title>{$i18n.t('Images')} | {$WEBUI_NAME}</title>
</svelte:head>

{#if loaded}
{#if variant === 'workspace'}
<!-- ═══════════════════════════════════════════════════════════════════════
     WORKSPACE VARIANT
     ═══════════════════════════════════════════════════════════════════════ -->
	{#if viewState !== 'ready'}
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
									: $i18n.t('Loading...')}
					</h2>
					<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
						{blockedReason ??
							(viewState === 'loading'
								? $i18n.t('Loading image generation settings...')
								: $i18n.t('Please try again later.'))}
					</p>
					<div class="mt-5 flex flex-wrap justify-center gap-2">
						{#if viewState === 'disabled' && $user?.role === 'admin'}
							<a class="workspace-secondary-button text-xs" href="/settings/images">
								{$i18n.t('Open image settings')}
							</a>
						{/if}
						{#if viewState !== 'denied'}
							<a class="workspace-secondary-button text-xs" href="/settings/connections">
								{$i18n.t('Go to Connections')}
							</a>
						{/if}
						<button type="button" class="workspace-secondary-button text-xs" on:click={() => location.reload()}>
							{$i18n.t('Refresh')}
						</button>
					</div>
				</div>
			</section>
		</div>
	{:else}
		<form class="space-y-4" on:submit|preventDefault={submitHandler}>
			<!-- ── Toolbar ── -->
			<section class="workspace-section space-y-4">
				<div class="flex flex-col gap-3 lg:flex-row lg:items-center">
					<div class="workspace-toolbar-summary">
						<div class="workspace-count-pill">
							<PhotoSolid className="size-3.5" />
							{$i18n.t('Image Studio')}
						</div>
						<div class="text-xs text-gray-500 dark:text-gray-400">
							{$i18n.t('Create polished visuals from a single prompt.')}
							<span class="hidden sm:inline ml-1 opacity-70">
								{$i18n.t('Press Ctrl/Command + Enter to generate.')}
							</span>
						</div>
					</div>

					<div class="workspace-toolbar">
						<HaloSelect
							bind:value={selectedModel}
							options={modelOptions}
							placeholder={$i18n.t('Select a model')}
							className="w-full lg:w-48 text-xs"
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

			<!-- ── Prompt & Settings ── -->
			<section class="workspace-section space-y-4">
				<!-- Prompt area -->
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

					<div class="space-y-1.5">
						<div class="text-xs font-medium text-gray-500 dark:text-gray-400">
							{$i18n.t('Negative Prompt')}
							<span class="font-normal opacity-70">({$i18n.t('Optional, useful for removing unwanted details.')})</span>
						</div>
						<textarea
							rows="2"
							bind:value={negativePrompt}
							on:keydown={handleComposerKeydown}
							placeholder={$i18n.t('Blur, low detail, extra fingers, cluttered background, text artifacts...')}
							class="w-full resize-none rounded-xl border border-gray-200/60 bg-white/85 p-3 text-sm leading-6 text-gray-900 outline-none placeholder:text-gray-400 dark:border-gray-700/50 dark:bg-gray-900/70 dark:text-gray-100 dark:placeholder:text-gray-500"
						/>
					</div>
				</div>

				<!-- Generation Settings -->
				<div class="glass-item p-4 space-y-4">
					<div class="text-sm font-semibold text-gray-900 dark:text-gray-100">
						{$i18n.t('Generation Settings')}
					</div>

					<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
						<!-- Size -->
						<div class="space-y-1.5">
							<div class="text-xs font-medium text-gray-500 dark:text-gray-400">{$i18n.t('Size')}</div>
							<HaloSelect
								bind:value={selectedSize}
								options={sizeOptions.map((o) => ({ value: o.value, label: `${o.ratio} · ${o.label}` }))}
								className="w-full text-xs"
							/>
						</div>

						<!-- Output Count -->
						<div class="space-y-1.5">
							<div class="text-xs font-medium text-gray-500 dark:text-gray-400">{$i18n.t('Output Count')}</div>
							<HaloSelect
								value={String(batchSize)}
								options={availableBatchOptions.map((n) => ({ value: String(n), label: `x${n}` }))}
								className="w-full text-xs"
								on:change={(e) => { batchSize = Number(e.detail.value); }}
							/>
						</div>

						<!-- Steps -->
						<div class="space-y-1.5">
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
						</div>

						<!-- Background -->
						{#if supportsBackground}
							<div class="space-y-1.5">
								<div class="text-xs font-medium text-gray-500 dark:text-gray-400">{$i18n.t('Background')}</div>
								<HaloSelect
									value={background}
									options={backgroundOptions.map((o) => ({
										value: o,
										label: $i18n.t(o === 'auto' ? 'Auto' : o === 'transparent' ? 'Transparent' : 'Opaque')
									}))}
									className="w-full text-xs"
									on:change={(e) => { setBackground(e.detail.value); }}
								/>
							</div>
						{/if}
					</div>
				</div>

				<!-- Credential Controls -->
				{#if showCredentialControls}
					<div class="glass-item p-4 space-y-3">
						<div class="flex items-center justify-between">
							<div>
								<div class="text-sm font-semibold text-gray-900 dark:text-gray-100">
									{$i18n.t('Key Source')}
								</div>
								<div class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
									{$i18n.t('Engine')}: {engine}
								</div>
							</div>
							<a class="text-xs font-medium text-gray-600 hover:underline dark:text-gray-300" href="/settings/connections">
								{$i18n.t('Go to Connections')}
							</a>
						</div>

						<div class="flex flex-wrap gap-2">
							<button type="button" class={sourcePillClass(credentialSource === 'auto')} on:click={() => requestCredentialSource('auto')}>
								{$i18n.t('Auto (Recommended)')}
							</button>
							<button type="button" class={sourcePillClass(credentialSource === 'personal')} on:click={() => requestCredentialSource('personal')}>
								{$i18n.t('My Connections')}
							</button>
							{#if sharedKeyEnabled}
								<button
									type="button"
									class={sourcePillClass(credentialSource === 'shared', !sharedKeyAvailable)}
									disabled={!sharedKeyAvailable}
									title={!sharedKeyAvailable ? $i18n.t('Workspace shared key is not configured.') : ''}
									on:click={() => requestCredentialSource('shared')}
								>
									{$i18n.t('Workspace Shared')}
								</button>
							{/if}
						</div>

						{#if resolvedCredential.source === 'shared' && !sharedConfirmed}
							<div class="rounded-xl border border-amber-200 bg-amber-50 p-3 text-xs text-amber-800 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200">
								<div class="font-semibold">{$i18n.t('Workspace shared key in use')}</div>
								<div class="mt-1 leading-5">
									{$i18n.t('Using the workspace shared key may incur costs. You can switch to your own connection in Settings > Connections.')}
								</div>
								<button
									type="button"
									class="mt-2 inline-flex items-center rounded-xl bg-amber-100 px-3 py-2 text-xs font-semibold text-amber-900 transition hover:bg-amber-200 dark:bg-amber-500/20 dark:text-amber-100 dark:hover:bg-amber-500/30"
									on:click={acknowledgeSharedKeyUsage}
								>
									{$i18n.t('I understand')}
								</button>
							</div>
						{/if}

						{#if credentialSource === 'personal'}
							<div class="space-y-1.5">
								<div class="text-xs font-medium text-gray-500 dark:text-gray-400">
									{$i18n.t('Connection')}
								</div>
								<HaloSelect
									value={String(selectedPersonalIndex)}
									options={providerConnections.urls.map((url, idx) => ({
										value: String(idx),
										label: `${idx + 1}. ${(url || '').trim() ? url : $i18n.t('Default')}`
									}))}
									placeholder={$i18n.t('Select a connection')}
									className="w-full"
									on:change={(e) => handlePersonalConnectionChange(e.detail.value)}
								/>
							</div>
						{/if}

						<div class="flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
							{#if resolvedCredential.source === 'personal'}
								<span class="rounded-full border border-gray-200 bg-white px-3 py-1 dark:border-gray-800 dark:bg-gray-900">
									{$i18n.t('Using')}: {$i18n.t('My Connections')}
									{#if resolvedCredential.connectionIndex !== null && resolvedCredential.connectionIndex !== undefined}
										#{resolvedCredential.connectionIndex + 1}
									{/if}
								</span>
							{:else if resolvedCredential.source === 'shared'}
								<span class="rounded-full border border-gray-200 bg-white px-3 py-1 dark:border-gray-800 dark:bg-gray-900">
									{$i18n.t('Using')}: {$i18n.t('Workspace Shared')}
								</span>
							{:else}
								<span class="rounded-full border border-gray-200 bg-white px-3 py-1 dark:border-gray-800 dark:bg-gray-900">
									{$i18n.t('Not configured')}
								</span>
							{/if}
							{#if blockedReason}
								<span class="rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200">
									{blockedReason}
								</span>
							{/if}
						</div>
					</div>
				{/if}
			</section>

			<!-- ── Results ── -->
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
						{#each Array.from({ length: Math.max(batchSize, 2) }) as _, index}
							<div
								class="shimmer h-56 rounded-xl glass-item"
								style={`animation-delay: ${index * 120}ms;`}
							/>
						{/each}
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
										<div class="text-xs font-medium">{selectedSize}</div>
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
	{/if}

{:else}
<!-- ═══════════════════════════════════════════════════════════════════════
     PLAYGROUND VARIANT (original markup)
     ═══════════════════════════════════════════════════════════════════════ -->
	{#if viewState !== 'ready'}
		<div class="playground-shell h-full overflow-y-auto scrollbar-hidden px-1 pb-10">
			<div class="mx-auto w-full max-w-3xl pt-8">
				<section
					class={cardShellClass}
				>
					<div class="pointer-events-none absolute -right-12 top-0 size-40 rounded-full bg-cyan-300/20 blur-3xl dark:bg-cyan-500/10" />
					<div class="pointer-events-none absolute -bottom-16 left-8 size-48 rounded-full bg-amber-200/30 blur-3xl dark:bg-amber-400/10" />

					<div class="relative flex items-start gap-4">
						<div
							class="flex size-14 shrink-0 items-center justify-center rounded-2xl bg-gray-100 text-gray-700 dark:bg-gray-900 dark:text-gray-200"
						>
							<PhotoSolid className="size-7" />
						</div>

						<div class="min-w-0">
							<h1 class={introTitleClass}>
								{viewState === 'denied'
									? $i18n.t('Image generation access required')
									: viewState === 'disabled'
										? $i18n.t('Image generation is disabled')
										: viewState === 'error'
											? $i18n.t('Unable to load image generation')
											: $i18n.t('Loading...')}
							</h1>

							<p class="mt-2 text-sm leading-6 text-gray-600 dark:text-gray-300">
								{blockedReason ??
									(viewState === 'loading'
										? $i18n.t('Loading image generation settings...')
										: $i18n.t('Please try again later.'))}
							</p>

							<div class="mt-5 flex flex-wrap items-center gap-2">
								{#if viewState === 'disabled' && $user?.role === 'admin'}
									<a class={ghostButtonClass} href="/settings/images">
										{$i18n.t('Open image settings')}
									</a>
								{/if}

								{#if viewState !== 'denied'}
									<a class={ghostButtonClass} href="/settings/connections">
										{$i18n.t('Go to Connections')}
									</a>
								{/if}

								<button
									type="button"
									class={ghostButtonClass}
									on:click={() => {
										location.reload();
									}}
								>
									{$i18n.t('Refresh')}
								</button>
							</div>
						</div>
					</div>
				</section>
			</div>
		</div>
	{:else}
		<form class="flex h-full flex-col justify-between text-sm" on:submit|preventDefault={submitHandler}>
		<div class="playground-shell h-full overflow-y-auto scrollbar-hidden px-1 pb-28">
			<div class="mx-auto flex w-full max-w-6xl flex-col gap-4 pt-1">
				<section
					class={cardShellClass}
				>
					<div class="pointer-events-none absolute -right-12 top-0 size-40 rounded-full bg-cyan-300/20 blur-3xl dark:bg-cyan-500/10" />
					<div class="pointer-events-none absolute -bottom-16 left-8 size-48 rounded-full bg-amber-200/30 blur-3xl dark:bg-amber-400/10" />

					<div class="relative flex flex-col gap-6 lg:flex-row lg:items-start lg:justify-between">
						<div class="flex max-w-2xl items-start gap-4">
							<div
								class="flex size-14 shrink-0 items-center justify-center rounded-2xl bg-gradient-to-br from-sky-500 via-cyan-500 to-teal-400 text-white shadow-lg shadow-cyan-500/20"
							>
								<PhotoSolid className="size-7" />
							</div>

							<div class="space-y-2">
								<div class="flex flex-wrap items-center gap-2">
									<h1 class={readyTitleClass}>
										{$i18n.t('Image Studio')}
									</h1>
									<span
										class="inline-flex items-center gap-1 rounded-full border border-white/60 bg-white/80 px-2.5 py-1 text-xs font-medium text-gray-600 shadow-sm backdrop-blur dark:border-gray-800 dark:bg-gray-900/80 dark:text-gray-300"
									>
										<Sparkles className="size-3.5" strokeWidth="2" />
										{$i18n.t('Image Generation')}
									</span>
								</div>

								<p class="max-w-xl text-sm leading-6 text-gray-600 dark:text-gray-300">
									{$i18n.t('Create polished visuals from a single prompt.')}
								</p>

								<div class="flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
									<span class="rounded-full bg-white/80 px-3 py-1 shadow-sm dark:bg-gray-900/80">
										{$i18n.t('Press Ctrl/Command + Enter to generate.')}
									</span>
									<span class="rounded-full bg-white/80 px-3 py-1 shadow-sm dark:bg-gray-900/80">
										{$i18n.t('Supports per-request options when the backend engine allows them.')}
									</span>
								</div>
							</div>
						</div>

							<div class="space-y-3">
								<div class="grid gap-3 sm:grid-cols-3">
								<div
									class="rounded-2xl border border-white/60 bg-white/70 px-4 py-3 shadow-sm backdrop-blur dark:border-gray-800 dark:bg-gray-900/70"
								>
									<div class="text-xs font-medium uppercase tracking-[0.12em] text-gray-400">
										{$i18n.t('Available Models')}
								</div>
								<div class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
									{imageModels.length || '--'}
								</div>
							</div>
							<div
								class="rounded-2xl border border-white/60 bg-white/70 px-4 py-3 shadow-sm backdrop-blur dark:border-gray-800 dark:bg-gray-900/70"
							>
								<div class="text-xs font-medium uppercase tracking-[0.12em] text-gray-400">
									{$i18n.t('Size')}
								</div>
								<div class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
									{selectedSize}
								</div>
							</div>
							<div
								class="rounded-2xl border border-white/60 bg-white/70 px-4 py-3 shadow-sm backdrop-blur dark:border-gray-800 dark:bg-gray-900/70"
							>
								<div class="text-xs font-medium uppercase tracking-[0.12em] text-gray-400">
									{$i18n.t('Output Count')}
								</div>
									<div class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
										x{batchSize}
									</div>
								</div>
								</div>

								{#if showCredentialControls}
									<div
										class="rounded-2xl border border-white/60 bg-white/70 px-4 py-4 shadow-sm backdrop-blur dark:border-gray-800 dark:bg-gray-900/70"
									>
										<div class="flex items-start justify-between gap-3">
											<div>
												<div class="text-xs font-medium uppercase tracking-[0.12em] text-gray-400">
													{$i18n.t('Key Source')}
												</div>
												<div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
													{$i18n.t('Engine')}: {engine}
												</div>
											</div>

											<a class="text-xs font-medium text-gray-600 hover:underline dark:text-gray-300" href="/settings/connections">
												{$i18n.t('Go to Connections')}
											</a>
										</div>

										<div class="mt-3 flex flex-wrap gap-2">
											<button
												type="button"
												class={sourcePillClass(credentialSource === 'auto')}
												on:click={() => requestCredentialSource('auto')}
											>
												{$i18n.t('Auto (Recommended)')}
											</button>
											<button
												type="button"
												class={sourcePillClass(credentialSource === 'personal')}
												on:click={() => requestCredentialSource('personal')}
											>
												{$i18n.t('My Connections')}
											</button>
											{#if sharedKeyEnabled}
												<button
													type="button"
													class={sourcePillClass(credentialSource === 'shared', !sharedKeyAvailable)}
													disabled={!sharedKeyAvailable}
													title={!sharedKeyAvailable ? $i18n.t('Workspace shared key is not configured.') : ''}
													on:click={() => requestCredentialSource('shared')}
												>
													{$i18n.t('Workspace Shared')}
												</button>
											{/if}
										</div>

										{#if resolvedCredential.source === 'shared' && !sharedConfirmed}
											<div
												class="mt-3 rounded-2xl border border-amber-200 bg-amber-50 p-3 text-xs text-amber-800 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200"
											>
												<div class="font-semibold">
													{$i18n.t('Workspace shared key in use')}
												</div>
												<div class="mt-1 leading-5">
													{$i18n.t(
														'Using the workspace shared key may incur costs. You can switch to your own connection in Settings > Connections.'
													)}
												</div>
												<button
													type="button"
													class="mt-2 inline-flex items-center rounded-xl bg-amber-100 px-3 py-2 text-xs font-semibold text-amber-900 transition hover:bg-amber-200 dark:bg-amber-500/20 dark:text-amber-100 dark:hover:bg-amber-500/30"
													on:click={acknowledgeSharedKeyUsage}
												>
													{$i18n.t('I understand')}
												</button>
											</div>
										{/if}

										{#if credentialSource === 'personal'}
											<div class="mt-3 space-y-2">
												<div class="text-xs font-medium text-gray-500 dark:text-gray-400">
													{$i18n.t('Connection')}
												</div>
												<HaloSelect
													value={String(selectedPersonalIndex)}
													options={providerConnections.urls.map((url, idx) => ({
														value: String(idx),
														label: `${idx + 1}. ${(url || '').trim() ? url : $i18n.t('Default')}`
													}))}
													placeholder={$i18n.t('Select a connection')}
													className="w-full"
													on:change={(e) => handlePersonalConnectionChange(e.detail.value)}
												/>
											</div>
										{/if}

										<div class="mt-3 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
											{#if resolvedCredential.source === 'personal'}
												<span class="rounded-full border border-gray-200 bg-white px-3 py-1 dark:border-gray-800 dark:bg-gray-900">
													{$i18n.t('Using')}: {$i18n.t('My Connections')}
													{#if resolvedCredential.connectionIndex !== null && resolvedCredential.connectionIndex !== undefined}
														#{resolvedCredential.connectionIndex + 1}
													{/if}
												</span>
											{:else if resolvedCredential.source === 'shared'}
												<span class="rounded-full border border-gray-200 bg-white px-3 py-1 dark:border-gray-800 dark:bg-gray-900">
													{$i18n.t('Using')}: {$i18n.t('Workspace Shared')}
												</span>
											{:else}
												<span class="rounded-full border border-gray-200 bg-white px-3 py-1 dark:border-gray-800 dark:bg-gray-900">
													{$i18n.t('Not configured')}
												</span>
											{/if}

											{#if blockedReason}
												<span
													class="rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200"
												>
													{blockedReason}
												</span>
											{/if}
										</div>
									</div>
								{/if}
							</div>
						</div>
					</section>

				<div class="grid gap-4 xl:grid-cols-[minmax(0,1.05fr)_minmax(22rem,0.95fr)]">
					<div class="space-y-4">
						<section
							class="overflow-hidden rounded-2xl border border-gray-200 dark:border-gray-800"
						>
							<div
								class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-gray-800"
							>
								<div>
									<div class="text-base font-semibold text-gray-900 dark:text-gray-100">
										{$i18n.t('Main Prompt')}
									</div>
									<div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
										{$i18n.t('Describe the image you want to generate...')}
									</div>
								</div>
								<Sparkles className="size-5 text-gray-400" />
							</div>

							<div class="space-y-4 p-5">
								<div
									class="rounded-2xl border border-gray-100 bg-white p-3 shadow-sm dark:border-gray-800 dark:bg-gray-900"
								>
									<textarea
										rows="7"
										bind:value={prompt}
										on:keydown={handleComposerKeydown}
										placeholder={$i18n.t('Describe the image you want to generate...')}
										class="min-h-[10rem] w-full resize-none bg-transparent text-sm leading-6 text-gray-900 outline-none placeholder:text-gray-400 dark:text-gray-100 dark:placeholder:text-gray-500"
									/>
								</div>

								<div class="space-y-2">
									<div class="text-xs font-medium text-gray-500 dark:text-gray-400">
										{$i18n.t('Prompt ideas')}
									</div>
									<div class="flex flex-wrap gap-2">
										{#each promptIdeas as idea}
											<button
												type="button"
												class="rounded-full border border-gray-200/80 bg-white px-3 py-1.5 text-xs font-medium text-gray-700 transition hover:border-gray-300 hover:bg-gray-50 dark:border-gray-800 dark:bg-gray-900 dark:text-gray-300 dark:hover:border-gray-700 dark:hover:bg-gray-900/80"
												on:click={() => applyPromptIdea(idea)}
											>
												{$i18n.t(idea)}
											</button>
										{/each}
									</div>
								</div>

								<div class="grid gap-4 lg:grid-cols-[minmax(0,1fr)_18rem]">
									<div class="space-y-2">
										<div class="text-sm font-medium text-gray-900 dark:text-gray-100">
											{$i18n.t('Negative Prompt')}
										</div>
										<div class="text-xs text-gray-500 dark:text-gray-400">
											{$i18n.t('Optional, useful for removing unwanted details.')}
										</div>
										<div
											class="rounded-2xl border border-gray-100 bg-white p-3 shadow-sm dark:border-gray-800 dark:bg-gray-900"
										>
											<textarea
												rows="4"
												bind:value={negativePrompt}
												on:keydown={handleComposerKeydown}
												placeholder={$i18n.t(
													'Blur, low detail, extra fingers, cluttered background, text artifacts...'
												)}
												class="min-h-[7rem] w-full resize-none bg-transparent text-sm leading-6 text-gray-900 outline-none placeholder:text-gray-400 dark:text-gray-100 dark:placeholder:text-gray-500"
											/>
										</div>
									</div>

									<div
										class="rounded-2xl border border-dashed border-gray-200 bg-gray-50/80 p-4 dark:border-gray-800 dark:bg-gray-900/60"
									>
										<div class="text-sm font-medium text-gray-900 dark:text-gray-100">
											{$i18n.t('Generation Settings')}
										</div>
										<div class="mt-2 text-xs leading-5 text-gray-500 dark:text-gray-400">
											{selectedModelLabel || $i18n.t('No models available')}
											<br />
											{selectedSize}
											<br />
											x{batchSize}
											{#if steps > 0}
												<br />
												{$i18n.t('Set Steps')}: {steps}
											{/if}
										</div>
									</div>
								</div>
							</div>
						</section>

						<section
							class="overflow-hidden rounded-2xl border border-gray-200 dark:border-gray-800"
						>
							<div
								class="border-b border-gray-100 px-5 py-4 text-base font-semibold text-gray-900 dark:border-gray-800 dark:text-gray-100"
							>
								{$i18n.t('Generation Settings')}
							</div>

							<div class="space-y-5 p-5">
								<div class="space-y-2">
									<div class="text-xs font-medium text-gray-500 dark:text-gray-400">
										{$i18n.t('Model')}
									</div>
									<HaloSelect
										bind:value={selectedModel}
										options={modelOptions}
										placeholder={$i18n.t('Select a model')}
										className="w-full"
									/>
									{#if imageModels.length === 0}
										<div class="text-xs text-amber-600 dark:text-amber-400">
											{$i18n.t('Image models are unavailable right now. Check your image settings.')}
										</div>
									{/if}
								</div>

								<div class="space-y-2">
									<div class="text-xs font-medium text-gray-500 dark:text-gray-400">
										{$i18n.t('Size')}
									</div>
									<div class="grid gap-2 sm:grid-cols-3">
										{#each sizeOptions as option}
											<button
												type="button"
												class={segmentedButtonClass(selectedSize === option.value)}
												on:click={() => {
													selectedSize = option.value;
												}}
											>
												<span class="text-xs font-semibold uppercase tracking-[0.14em] opacity-70">
													{option.ratio}
												</span>
												<span class="mt-1 text-sm font-semibold">{option.label}</span>
											</button>
										{/each}
									</div>
								</div>

								<div class="grid gap-5 lg:grid-cols-[minmax(0,16rem)_minmax(0,1fr)]">
									<div class="space-y-2">
										<div class="text-xs font-medium text-gray-500 dark:text-gray-400">
											{$i18n.t('Output Count')}
										</div>
										<div class="flex gap-2">
											{#each availableBatchOptions as option}
												<button
													type="button"
													class={segmentedButtonClass(batchSize === option)}
													on:click={() => {
														batchSize = option;
													}}
												>
													<span class="text-xs font-semibold uppercase tracking-[0.14em] opacity-70">
														x{option}
													</span>
													<span class="mt-1 text-sm font-semibold">{option}</span>
												</button>
											{/each}
										</div>
									</div>

									<div class="space-y-3">
										<div class="flex items-center justify-between gap-3">
											<div>
												<div class="text-xs font-medium text-gray-500 dark:text-gray-400">
													{$i18n.t('Set Steps')}
												</div>
												<div class="mt-1 text-sm font-semibold text-gray-900 dark:text-gray-100">
													{steps === 0 ? $i18n.t('Auto') : steps}
												</div>
											</div>
											<div class="text-xs text-gray-500 dark:text-gray-400">
												0 - 80
											</div>
										</div>
										<input
											type="range"
											min="0"
											max="80"
											step="5"
											bind:value={steps}
											class="image-range h-2 w-full cursor-pointer appearance-none rounded-full bg-gray-200 dark:bg-gray-800"
										/>
									</div>
								</div>

								{#if supportsBackground}
									<div class="space-y-2">
										<div class="text-xs font-medium text-gray-500 dark:text-gray-400">
											{$i18n.t('Background')}
										</div>
										<div class="grid gap-2 sm:grid-cols-3">
											{#each backgroundOptions as option}
												<button
													type="button"
													class={segmentedButtonClass(background === option)}
													on:click={() => {
														setBackground(option);
													}}
												>
													<span class="text-sm font-semibold">{$i18n.t(option === 'auto'
														? 'Auto'
														: option === 'transparent'
															? 'Transparent'
															: 'Opaque')}</span>
												</button>
											{/each}
										</div>
									</div>
								{/if}
							</div>
						</section>
					</div>

					<section
						bind:this={resultsSectionElement}
						class="overflow-hidden rounded-2xl border border-gray-200 dark:border-gray-800"
					>
						<div
							class="flex items-center justify-between gap-3 border-b border-gray-100 px-5 py-4 dark:border-gray-800"
						>
							<div>
								<div class="text-base font-semibold text-gray-900 dark:text-gray-100">
									{$i18n.t('Recent Result')}
								</div>
								<div class="mt-1 truncate text-xs text-gray-500 dark:text-gray-400">
									{#if lastPrompt}
										{lastPrompt}
									{:else}
										{$i18n.t('Your images will appear here after generation.')}
									{/if}
								</div>
							</div>

							{#if lastPrompt}
								<button type="button" class={ghostButtonClass} on:click={copyPromptHandler}>
									<Clipboard className="size-4" />
									{$i18n.t('Copy prompt')}
								</button>
							{/if}
						</div>

						<div class="min-h-[32rem] bg-gray-50/70 p-5 dark:bg-gray-950/40">
							{#if loading}
								<div class="grid gap-3 sm:grid-cols-2">
									{#each Array.from({ length: Math.max(batchSize, 2) }) as _, index}
										<div
											class="shimmer h-72 rounded-[1.4rem] border border-gray-200/70 bg-white shadow-sm dark:border-gray-800 dark:bg-gray-900"
											style={`animation-delay: ${index * 120}ms;`}
										/>
									{/each}
								</div>
							{:else if generatedImages.length > 0}
								<div class="grid gap-3 sm:grid-cols-2">
									{#each generatedImages as image, index}
										<div
											role="button"
											tabindex="0"
											class="result-card group overflow-hidden rounded-[1.5rem] border border-gray-200/70 bg-white p-2 text-left shadow-sm transition hover:-translate-y-0.5 hover:shadow-xl dark:border-gray-800 dark:bg-gray-900"
											style={`animation-delay: ${index * 60}ms;`}
											on:click={() => openPreview(image, index)}
											on:keydown={(event) => {
												if (event.key === 'Enter' || event.key === ' ') {
													event.preventDefault();
													openPreview(image, index);
												}
											}}
										>
											<div class="relative overflow-hidden rounded-[1.05rem] bg-gray-100 dark:bg-gray-950">
												<img
													src={image.url}
													alt={`${$i18n.t('Generated image')} ${index + 1}`}
													class="max-h-[26rem] min-h-[14rem] w-full object-cover transition duration-500 group-hover:scale-[1.02]"
													loading="lazy"
												/>

												<div
													class="absolute inset-x-0 bottom-0 flex items-end justify-between gap-2 bg-gradient-to-t from-black/70 via-black/20 to-transparent p-3 text-white opacity-0 transition duration-200 group-hover:opacity-100"
												>
													<div class="text-xs font-medium">
														{selectedSize}
													</div>
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
								<div
									class="flex h-full min-h-[28rem] flex-col items-center justify-center rounded-[1.75rem] border border-dashed border-gray-200 bg-white/80 px-6 text-center shadow-sm dark:border-gray-800 dark:bg-gray-900/70"
								>
									<div
										class="flex size-16 items-center justify-center rounded-2xl bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-300"
									>
										<PhotoSolid className="size-8" />
									</div>
									<div class="mt-5 text-lg font-semibold text-gray-900 dark:text-gray-100">
										{$i18n.t('Your images will appear here after generation.')}
									</div>
									<div class="mt-2 max-w-sm text-sm leading-6 text-gray-500 dark:text-gray-400">
										{$i18n.t('Start with a strong subject, then add lighting, composition, materials, and mood for better results.')}
									</div>
								</div>
							{/if}
						</div>
					</section>
				</div>
			</div>
		</div>

		<div
			class="sticky bottom-0 z-20 border-t border-gray-100 bg-white/85 px-1 py-3 backdrop-blur-xl dark:border-gray-800 dark:bg-gray-950/80"
		>
			<div class="mx-auto flex w-full max-w-6xl flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
				<div class="flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
					{#if selectedModelLabel}
						<span
							class="rounded-full border border-gray-200 bg-white px-3 py-1 dark:border-gray-800 dark:bg-gray-900"
						>
							{selectedModelLabel}
						</span>
					{/if}
					<span
						class="rounded-full border border-gray-200 bg-white px-3 py-1 dark:border-gray-800 dark:bg-gray-900"
					>
						{selectedSize}
					</span>
					<span
						class="rounded-full border border-gray-200 bg-white px-3 py-1 dark:border-gray-800 dark:bg-gray-900"
					>
						x{batchSize}
					</span>
					{#if steps > 0}
						<span
							class="rounded-full border border-gray-200 bg-white px-3 py-1 dark:border-gray-800 dark:bg-gray-900"
						>
							{$i18n.t('Set Steps')}: {steps}
						</span>
					{/if}
				</div>

					<button
						type="submit"
						class="inline-flex items-center justify-center gap-2 rounded-2xl bg-gray-900 px-5 py-3 text-sm font-medium text-white shadow-lg shadow-gray-900/15 transition hover:bg-black active:scale-[0.99] disabled:cursor-not-allowed disabled:bg-gray-300 disabled:text-gray-500 dark:bg-white dark:text-gray-900 dark:shadow-gray-100/10 dark:hover:bg-gray-100 dark:disabled:bg-gray-700 dark:disabled:text-gray-400"
						disabled={!canSubmit}
						title={blockedReason ?? ''}
					>
					{#if loading}
						<svg
							class="size-4 animate-spin"
							xmlns="http://www.w3.org/2000/svg"
							fill="none"
							viewBox="0 0 24 24"
						>
							<circle
								class="opacity-25"
								cx="12"
								cy="12"
								r="10"
								stroke="currentColor"
								stroke-width="4"
							/>
							<path
								class="opacity-75"
								fill="currentColor"
								d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
							/>
						</svg>
					{:else}
						<Sparkles className="size-4" strokeWidth="2" />
					{/if}
					{loading ? $i18n.t('Generating...') : $i18n.t('Generate')}
				</button>
			</div>
		</div>
	</form>

	<ConfirmDialog
		bind:show={showSharedKeyConfirm}
		title={$i18n.t('Use workspace shared key?')}
		message={$i18n.t(
			'Using the workspace shared key may incur costs and will use the administrator-configured provider. Continue?'
		)}
		confirmLabel={$i18n.t('Use shared key')}
		cancelLabel={$i18n.t('Cancel')}
		onConfirm={confirmCredentialSource}
		on:cancel={() => {
			pendingCredentialSource = null;
		}}
	/>

	<ImagePreview bind:show={previewOpen} src={previewSrc} alt={previewAlt} />
{/if}
<!-- END PLAYGROUND VARIANT -->
{/if}

<!-- Shared dialogs (available to both variants) -->
{#if variant === 'workspace'}
	<ConfirmDialog
		bind:show={showSharedKeyConfirm}
		title={$i18n.t('Use workspace shared key?')}
		message={$i18n.t(
			'Using the workspace shared key may incur costs and will use the administrator-configured provider. Continue?'
		)}
		confirmLabel={$i18n.t('Use shared key')}
		cancelLabel={$i18n.t('Cancel')}
		onConfirm={confirmCredentialSource}
		on:cancel={() => {
			pendingCredentialSource = null;
		}}
	/>

	<ImagePreview bind:show={previewOpen} src={previewSrc} alt={previewAlt} />
{/if}
{/if}

<style>
	.playground-shell {
		background-image:
			radial-gradient(circle at top right, rgba(6, 182, 212, 0.08), transparent 26rem),
			radial-gradient(circle at bottom left, rgba(245, 158, 11, 0.08), transparent 24rem);
	}

	.result-card {
		animation: card-in 420ms cubic-bezier(0.2, 0.85, 0.2, 1) both;
	}

	.shimmer {
		position: relative;
		overflow: hidden;
	}

	.shimmer::after {
		content: '';
		position: absolute;
		inset: 0;
		transform: translateX(-100%);
		background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.6), transparent);
		animation: shimmer 1.6s infinite;
	}

	:global(.dark) .shimmer::after {
		background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.08), transparent);
	}

	.image-range::-webkit-slider-thumb {
		appearance: none;
		width: 1rem;
		height: 1rem;
		border-radius: 999px;
		background: #111827;
		border: 2px solid #ffffff;
		box-shadow: 0 8px 20px -10px rgba(17, 24, 39, 0.7);
	}

	:global(.dark) .image-range::-webkit-slider-thumb {
		background: #f9fafb;
		border-color: #111827;
	}

	.image-range::-moz-range-thumb {
		width: 1rem;
		height: 1rem;
		border: none;
		border-radius: 999px;
		background: #111827;
		box-shadow: 0 8px 20px -10px rgba(17, 24, 39, 0.7);
	}

	:global(.dark) .image-range::-moz-range-thumb {
		background: #f9fafb;
	}

	@keyframes shimmer {
		100% {
			transform: translateX(100%);
		}
	}

	@keyframes card-in {
		from {
			opacity: 0;
			transform: translateY(12px) scale(0.985);
		}

		to {
			opacity: 1;
			transform: translateY(0) scale(1);
		}
	}
</style>
