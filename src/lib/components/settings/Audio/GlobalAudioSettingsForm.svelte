<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { createEventDispatcher, onMount, getContext, tick } from 'svelte';
	import { slide } from 'svelte/transition';
	import { quintOut } from 'svelte/easing';
	const dispatch = createEventDispatcher();

	import { getBackendConfig } from '$lib/apis';
	import {
		getAudioConfig,
		updateAudioConfig,
		getModels as _getModels,
		getVoices as _getVoices
	} from '$lib/apis/audio';
	import { config, settings } from '$lib/stores';
	import { revealExpandedSection } from '$lib/utils/expanded-section-scroll';

	import SensitiveInput from '$lib/components/common/SensitiveInput.svelte';
	import Spinner from '$lib/components/common/Spinner.svelte';
	import ChevronDown from '$lib/components/icons/ChevronDown.svelte';
	import HaloSelect from '$lib/components/common/HaloSelect.svelte';
	import InlineDirtyActions from '$lib/components/admin/Settings/InlineDirtyActions.svelte';
	import { cloneSettingsSnapshot, isSettingsSnapshotEqual } from '$lib/utils/settings-dirty';

	import { TTS_RESPONSE_SPLIT } from '$lib/types';

	import type { Writable } from 'svelte/store';
	import type { i18n as i18nType } from 'i18next';

	// Global audio policy form rendered inside /settings/audio.
	const i18n = getContext<Writable<i18nType>>('i18n');

	export let saveHandler: () => void;
	export let embedded = false;
	export let showSubmit = true;
	export let showScopeBadges = false;
	export let scopeLabel = '';
	export let scopeBadgeClass =
		'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300';
	export let autoPersistOnEngineChange = true;
	export let visualVariant: 'default' | 'document-card' | 'flat' = 'default';
	export let defaultExpandedSections = { stt: true, tts: true, advanced: true };

	// Audio
	let TTS_OPENAI_API_BASE_URL = '';
	let TTS_OPENAI_API_KEY = '';
	let TTS_API_KEY = '';
	let TTS_ENGINE = '';
	let TTS_MODEL = '';
	let TTS_VOICE = '';
	let TTS_SPLIT_ON: TTS_RESPONSE_SPLIT = TTS_RESPONSE_SPLIT.PUNCTUATION;
	let TTS_AZURE_SPEECH_REGION = '';
	let TTS_AZURE_SPEECH_OUTPUT_FORMAT = '';

	let STT_OPENAI_API_BASE_URL = '';
	let STT_OPENAI_API_KEY = '';
	let STT_ENGINE = '';
	let STT_MODEL = '';
	let STT_WHISPER_MODEL = '';
	let STT_AZURE_API_KEY = '';
	let STT_AZURE_REGION = '';
	let STT_AZURE_LOCALES = '';
	let STT_DEEPGRAM_API_KEY = '';

	let STT_WHISPER_MODEL_LOADING = false;
	$: sttEngineOptions = [
		{ value: 'openai', label: 'OpenAI' },
		{ value: 'web', label: $i18n.t('Web API'), disabled: true, description: $i18n.t('Browser Web API is not wired to the recorder in the Go slim profile.') }
	];
	$: ttsEngineOptions = [
		{ value: '', label: $i18n.t('Web API') },
		{ value: 'openai', label: $i18n.t('OpenAI') }
	];
	let isSaving = false;
	let expandedSections = { stt: true, tts: true, advanced: true };
	let sectionEl_stt: HTMLElement;
	let sectionEl_tts: HTMLElement;
	let sectionEl_advanced: HTMLElement;
	let initialSnapshot = null;
	let isInitialized = false;

	export let isDirty = false;
	let lastDirtyState: boolean | null = null;

	const buildSnapshot = (
		currentSTTOpenAIBaseUrl: string,
		currentSTTOpenAIKey: string,
		currentSTTEngine: string,
		currentSTTModel: string,
		currentSTTWhisperModel: string,
		currentSTTAzureApiKey: string,
		currentSTTAzureRegion: string,
		currentSTTAzureLocales: string,
		currentSTTDeepgramApiKey: string,
		currentTTSOpenAIBaseUrl: string,
		currentTTSOpenAIKey: string,
		currentTTSApiKey: string,
		currentTTSEngine: string,
		currentTTSModel: string,
		currentTTSVoice: string,
		currentTTSSplitOn: TTS_RESPONSE_SPLIT,
		currentTTSAzureSpeechRegion: string,
		currentTTSAzureSpeechOutputFormat: string
	) => ({
		stt: {
			STT_OPENAI_API_BASE_URL: currentSTTOpenAIBaseUrl,
			STT_OPENAI_API_KEY: currentSTTOpenAIKey,
			STT_ENGINE: currentSTTEngine,
			STT_MODEL: currentSTTModel,
			STT_WHISPER_MODEL: currentSTTWhisperModel,
			STT_AZURE_API_KEY: currentSTTAzureApiKey,
			STT_AZURE_REGION: currentSTTAzureRegion,
			STT_AZURE_LOCALES: currentSTTAzureLocales,
			STT_DEEPGRAM_API_KEY: currentSTTDeepgramApiKey
		},
		tts: {
			TTS_OPENAI_API_BASE_URL: currentTTSOpenAIBaseUrl,
			TTS_OPENAI_API_KEY: currentTTSOpenAIKey,
			TTS_API_KEY: currentTTSApiKey,
			TTS_ENGINE: currentTTSEngine,
			TTS_MODEL: currentTTSModel,
			TTS_VOICE: currentTTSVoice,
			TTS_SPLIT_ON: currentTTSSplitOn,
			TTS_AZURE_SPEECH_REGION: currentTTSAzureSpeechRegion,
			TTS_AZURE_SPEECH_OUTPUT_FORMAT: currentTTSAzureSpeechOutputFormat
		}
	});

	let snapshot = {
		stt: {
			STT_OPENAI_API_BASE_URL: '',
			STT_OPENAI_API_KEY: '',
			STT_ENGINE: '',
			STT_MODEL: '',
			STT_WHISPER_MODEL: '',
			STT_AZURE_API_KEY: '',
			STT_AZURE_REGION: '',
			STT_AZURE_LOCALES: '',
			STT_DEEPGRAM_API_KEY: ''
		},
		tts: {
			TTS_OPENAI_API_BASE_URL: '',
			TTS_OPENAI_API_KEY: '',
			TTS_API_KEY: '',
			TTS_ENGINE: '',
			TTS_MODEL: '',
			TTS_VOICE: '',
			TTS_SPLIT_ON: TTS_RESPONSE_SPLIT.PUNCTUATION,
			TTS_AZURE_SPEECH_REGION: '',
			TTS_AZURE_SPEECH_OUTPUT_FORMAT: ''
		}
	};
	$: snapshot = buildSnapshot(
		STT_OPENAI_API_BASE_URL,
		STT_OPENAI_API_KEY,
		STT_ENGINE,
		STT_MODEL,
		STT_WHISPER_MODEL,
		STT_AZURE_API_KEY,
		STT_AZURE_REGION,
		STT_AZURE_LOCALES,
		STT_DEEPGRAM_API_KEY,
		TTS_OPENAI_API_BASE_URL,
		TTS_OPENAI_API_KEY,
		TTS_API_KEY,
		TTS_ENGINE,
		TTS_MODEL,
		TTS_VOICE,
		TTS_SPLIT_ON,
		TTS_AZURE_SPEECH_REGION,
		TTS_AZURE_SPEECH_OUTPUT_FORMAT
	);
	$: dirtySections = initialSnapshot
		? {
				stt: !isSettingsSnapshotEqual(snapshot.stt, initialSnapshot.stt),
				tts: !isSettingsSnapshotEqual(snapshot.tts, initialSnapshot.tts)
			}
		: { stt: false, tts: false };
	$: isDirty = dirtySections.stt || dirtySections.tts;
	$: if (lastDirtyState !== isDirty) {
		lastDirtyState = isDirty;
		dispatch('dirtyChange', { value: isDirty });
	}

	$: isDocumentCard = visualVariant === 'document-card';
	$: isFlat = visualVariant === 'flat';

	const normalizeText = (value: string | null | undefined) => value ?? '';

	// Audio config arrives asynchronously; lock the baseline to hydrated values, not a timeout window.
	const syncBaseline = () => {
		initialSnapshot = cloneSettingsSnapshot(
			buildSnapshot(
				STT_OPENAI_API_BASE_URL,
				STT_OPENAI_API_KEY,
				STT_ENGINE,
				STT_MODEL,
				STT_WHISPER_MODEL,
				STT_AZURE_API_KEY,
				STT_AZURE_REGION,
				STT_AZURE_LOCALES,
				STT_DEEPGRAM_API_KEY,
				TTS_OPENAI_API_BASE_URL,
				TTS_OPENAI_API_KEY,
				TTS_API_KEY,
				TTS_ENGINE,
				TTS_MODEL,
				TTS_VOICE,
				TTS_SPLIT_ON,
				TTS_AZURE_SPEECH_REGION,
				TTS_AZURE_SPEECH_OUTPUT_FORMAT
			)
		);
	};

	const toggleSection = async (section: 'stt' | 'tts' | 'advanced') => {
		expandedSections[section] = !expandedSections[section];
		if (expandedSections[section]) {
			const el = { stt: sectionEl_stt, tts: sectionEl_tts, advanced: sectionEl_advanced }[section];
			await revealExpandedSection(el);
		}
	};

	// eslint-disable-next-line no-undef
	let voices: SpeechSynthesisVoice[] = [];
	let models: Awaited<ReturnType<typeof _getModels>>['models'] = [];
	const MIMO_TTS_MODELS = [
		{ id: 'mimo-v2.5-tts', name: 'MiMo-V2.5-TTS' },
		{ id: 'mimo-v2-tts', name: 'MiMo-V2-TTS' }
	];
	const MIMO_TTS_VOICES = [
		{ id: 'mimo_default', name: 'MiMo Default' },
		{ id: 'default_zh', name: 'MiMo V2 Chinese Female' },
		{ id: 'default_en', name: 'MiMo V2 English Female' },
		{ id: '\u51b0\u7cd6', name: '\u51b0\u7cd6' },
		{ id: '\u8309\u8389', name: '\u8309\u8389' },
		{ id: '\u82cf\u6253', name: '\u82cf\u6253' },
		{ id: '\u767d\u6866', name: '\u767d\u6866' },
		{ id: 'Mia', name: 'Mia' },
		{ id: 'Chloe', name: 'Chloe' },
		{ id: 'Milo', name: 'Milo' },
		{ id: 'Dean', name: 'Dean' }
	];
	const MIMO_TTS_MODEL_OPTIONS = MIMO_TTS_MODELS.map((model) => ({
		value: model.id,
		label: model.name
	}));
	const MIMO_TTS_VOICE_OPTIONS = MIMO_TTS_VOICES.map((voice) => ({
		value: voice.id,
		label: voice.name,
		description: voice.id !== voice.name ? voice.id : undefined
	}));

	const applyTTSEngineDefaults = (engine: string) => {
		if (engine === 'openai') {
			TTS_VOICE = 'alloy';
			TTS_MODEL = 'tts-1';
		} else if (engine === 'mimo') {
			TTS_VOICE = 'mimo_default';
			TTS_MODEL = 'mimo-v2.5-tts';
		} else {
			TTS_VOICE = '';
			TTS_MODEL = '';
		}
	};

	const handleTTSEngineChange = async (engine: string) => {
		applyTTSEngineDefaults(engine);

		if (autoPersistOnEngineChange) {
			await updateConfigHandler({ notify: false });
			await getVoices();
			await getModels();
		} else if (engine === '') {
			await getVoices();
			models = [];
		} else if (engine === 'mimo') {
			voices = MIMO_TTS_VOICES as any;
			models = MIMO_TTS_MODELS;
		} else {
			voices = [];
			models = [];
		}
	};

	const getModels = async () => {
		if (TTS_ENGINE === '') {
			models = [];
		} else if (TTS_ENGINE === 'mimo') {
			models = MIMO_TTS_MODELS;
		} else {
			const res = await _getModels(
				localStorage.token,
				$config?.features?.enable_direct_connections && ($settings?.directConnections ?? null)
			).catch((e) => {
				toast.error(`${e}`);
			});

			if (res) {
				console.log(res);
				models = res.models;
			}
		}
	};

	const getVoices = async () => {
		if (TTS_ENGINE === '') {
			const getVoicesLoop = setInterval(() => {
				voices = speechSynthesis.getVoices();

				// do your loop
				if (voices.length > 0) {
					clearInterval(getVoicesLoop);
					voices.sort((a, b) => a.name.localeCompare(b.name, $i18n.resolvedLanguage));
				}
			}, 100);
		} else if (TTS_ENGINE === 'mimo') {
			voices = MIMO_TTS_VOICES as any;
		} else {
			const res = await _getVoices(localStorage.token).catch((e) => {
				toast.error(`${e}`);
			});

			if (res) {
				console.log(res);
				voices = res.voices;
				voices.sort((a, b) => a.name.localeCompare(b.name, $i18n.resolvedLanguage));
			}
		}
	};

	const buildGlobalPatch = () => {
		return {
			tts: {
				OPENAI_API_BASE_URL: TTS_OPENAI_API_BASE_URL,
				OPENAI_API_KEY: TTS_OPENAI_API_KEY,
				API_KEY: TTS_API_KEY,
				ENGINE: TTS_ENGINE,
				MODEL: TTS_MODEL,
				VOICE: TTS_VOICE,
				SPLIT_ON: TTS_SPLIT_ON,
				AZURE_SPEECH_REGION: TTS_AZURE_SPEECH_REGION,
				AZURE_SPEECH_OUTPUT_FORMAT: TTS_AZURE_SPEECH_OUTPUT_FORMAT
			},
			stt: {
				OPENAI_API_BASE_URL: STT_OPENAI_API_BASE_URL,
				OPENAI_API_KEY: STT_OPENAI_API_KEY,
				ENGINE: STT_ENGINE,
				MODEL: STT_MODEL,
				WHISPER_MODEL: STT_WHISPER_MODEL,
				DEEPGRAM_API_KEY: STT_DEEPGRAM_API_KEY,
				AZURE_API_KEY: STT_AZURE_API_KEY,
				AZURE_REGION: STT_AZURE_REGION,
				AZURE_LOCALES: STT_AZURE_LOCALES
			}
		};
	};

	const updateConfigHandler = async (options: { notify?: boolean } = {}) => {
		const { notify = true } = options;
		if (isSaving) {
			return false;
		}

		isSaving = true;
		try {
			const res = await updateAudioConfig(localStorage.token, buildGlobalPatch());

			if (res) {
				await tick();
				syncBaseline();
				if (notify) {
					saveHandler?.();
					dispatch('save');
				}
				config.set(await getBackendConfig());
			}

			return !!res;
		} finally {
			isSaving = false;
		}
	};

	export const save = async () => {
		return await updateConfigHandler({ notify: false });
	};

	const sttModelUpdateHandler = async () => {
		STT_WHISPER_MODEL_LOADING = true;
		await updateConfigHandler({ notify: autoPersistOnEngineChange });
		STT_WHISPER_MODEL_LOADING = false;
	};

	const onSubmitHandler = async () => {
		if (!showSubmit) {
			return;
		}

		await updateConfigHandler({ notify: true });
	};

	onMount(async () => {
		expandedSections = {
			stt: defaultExpandedSections?.stt ?? true,
			tts: defaultExpandedSections?.tts ?? true,
			advanced: defaultExpandedSections?.advanced ?? true
		};

		const res = await getAudioConfig(localStorage.token);

		if (res) {
			console.log(res);
			TTS_OPENAI_API_BASE_URL = normalizeText(res.tts.OPENAI_API_BASE_URL);
			TTS_OPENAI_API_KEY = normalizeText(res.tts.OPENAI_API_KEY);
			TTS_API_KEY = normalizeText(res.tts.API_KEY);

			TTS_ENGINE = normalizeText(res.tts.ENGINE);
			TTS_MODEL = normalizeText(res.tts.MODEL);
			TTS_VOICE = normalizeText(res.tts.VOICE);

			TTS_SPLIT_ON = res.tts.SPLIT_ON || TTS_RESPONSE_SPLIT.PUNCTUATION;

			TTS_AZURE_SPEECH_OUTPUT_FORMAT = normalizeText(res.tts.AZURE_SPEECH_OUTPUT_FORMAT);
			TTS_AZURE_SPEECH_REGION = normalizeText(res.tts.AZURE_SPEECH_REGION);

			STT_OPENAI_API_BASE_URL = normalizeText(res.stt.OPENAI_API_BASE_URL);
			STT_OPENAI_API_KEY = normalizeText(res.stt.OPENAI_API_KEY);

			STT_ENGINE = normalizeText(res.stt.ENGINE);
			STT_MODEL = normalizeText(res.stt.MODEL);
			STT_WHISPER_MODEL = normalizeText(res.stt.WHISPER_MODEL);
			STT_AZURE_API_KEY = normalizeText(res.stt.AZURE_API_KEY);
			STT_AZURE_REGION = normalizeText(res.stt.AZURE_REGION);
			STT_AZURE_LOCALES = normalizeText(res.stt.AZURE_LOCALES);
			STT_DEEPGRAM_API_KEY = normalizeText(res.stt.DEEPGRAM_API_KEY);
		}

		syncBaseline();
		isInitialized = true;
		await Promise.all([getVoices(), getModels()]);
		await tick();
	});

	const resetSectionChanges = (section: 'stt' | 'tts') => {
		if (!initialSnapshot) return;
		const next = cloneSettingsSnapshot(initialSnapshot[section]);

		if (section === 'stt') {
			STT_OPENAI_API_BASE_URL = next.STT_OPENAI_API_BASE_URL;
			STT_OPENAI_API_KEY = next.STT_OPENAI_API_KEY;
			STT_ENGINE = next.STT_ENGINE;
			STT_MODEL = next.STT_MODEL;
			STT_WHISPER_MODEL = next.STT_WHISPER_MODEL;
			STT_AZURE_API_KEY = next.STT_AZURE_API_KEY;
			STT_AZURE_REGION = next.STT_AZURE_REGION;
			STT_AZURE_LOCALES = next.STT_AZURE_LOCALES;
			STT_DEEPGRAM_API_KEY = next.STT_DEEPGRAM_API_KEY;
			return;
		}

		TTS_OPENAI_API_BASE_URL = next.TTS_OPENAI_API_BASE_URL;
		TTS_OPENAI_API_KEY = next.TTS_OPENAI_API_KEY;
		TTS_API_KEY = next.TTS_API_KEY;
		TTS_ENGINE = next.TTS_ENGINE;
		TTS_MODEL = next.TTS_MODEL;
		TTS_VOICE = next.TTS_VOICE;
		TTS_SPLIT_ON = next.TTS_SPLIT_ON;
		TTS_AZURE_SPEECH_REGION = next.TTS_AZURE_SPEECH_REGION;
		TTS_AZURE_SPEECH_OUTPUT_FORMAT = next.TTS_AZURE_SPEECH_OUTPUT_FORMAT;
	};

	export const reset = async () => {
		resetSectionChanges('stt');
		resetSectionChanges('tts');
		await getVoices();
		await getModels();
	};
</script>

{#if !isInitialized}
	<div class="glass-item flex items-center gap-3 px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
		<Spinner className="size-4" />
		<span>{$i18n.t('Loading audio settings...')}</span>
	</div>
{:else if isFlat}
	<!-- ====== Flat variant: glass-items directly, no wrapper cards/folding ====== -->
	<div class="space-y-3">
		<!-- STT -->
		<div class="text-sm font-medium text-gray-500 dark:text-gray-400 pl-1">
			{$i18n.t('STT Settings')}
		</div>

		<div class="glass-item px-4 py-3">
			<div class="flex items-center justify-between">
				<div class="text-sm font-medium">{$i18n.t('Speech-to-Text Engine')}</div>
				<HaloSelect
					bind:value={STT_ENGINE}
					placeholder="Select an engine"
					options={sttEngineOptions}
					className="w-fit"
				/>
			</div>
		</div>

		{#if STT_ENGINE === 'openai'}
			<div class="space-y-3">
				<div class="text-sm font-medium text-gray-500 dark:text-gray-400 pl-1">
					{$i18n.t('Engine Credentials')}
				</div>
				<div class="grid grid-cols-1 md:grid-cols-2 gap-3">
					<div class="glass-item p-4">
						<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('API Base URL')}</div>
						<input class="w-full py-2 px-3 text-sm dark:text-gray-300 glass-input" placeholder={$i18n.t('Enter API Base URL')} bind:value={STT_OPENAI_API_BASE_URL} required />
					</div>
					<div class="glass-item p-4">
						<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('API Key')}</div>
						<SensitiveInput placeholder={$i18n.t('Enter API Key')} bind:value={STT_OPENAI_API_KEY} />
					</div>
				</div>
				<div class="glass-item p-4">
					<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('STT Model')}</div>
					<input list="model-list" class="w-full py-2 px-3 text-sm dark:text-gray-300 glass-input" bind:value={STT_MODEL} placeholder="Select a model" />
					<datalist id="model-list"><option value="whisper-1" /></datalist>
				</div>
			</div>
		{:else if STT_ENGINE === 'deepgram'}
			<div class="space-y-3">
				<div class="text-sm font-medium text-gray-500 dark:text-gray-400 pl-1">
					{$i18n.t('Engine Credentials')}
				</div>
				<div class="glass-item p-4">
					<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('API Key')}</div>
					<SensitiveInput placeholder={$i18n.t('Enter Deepgram API Key')} bind:value={STT_DEEPGRAM_API_KEY} />
				</div>
				<div class="glass-item p-4">
					<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('STT Model')}</div>
					<input class="w-full py-2 px-3 text-sm dark:text-gray-300 glass-input" bind:value={STT_MODEL} placeholder="Select a model (optional)" />
					<div class="mt-1.5 text-xs text-gray-400 dark:text-gray-500">
						{$i18n.t('Leave model field empty to use the default model.')}
						<a class="hover:underline dark:text-gray-200 text-gray-800" href="https://developers.deepgram.com/docs/models" target="_blank">{$i18n.t('Click here to see available models.')}</a>
					</div>
				</div>
			</div>
		{:else if STT_ENGINE === 'azure'}
			<div class="space-y-3">
				<div class="text-sm font-medium text-gray-500 dark:text-gray-400 pl-1">
					{$i18n.t('Engine Credentials')}
				</div>
				<div class="grid grid-cols-1 md:grid-cols-2 gap-3">
					<div class="glass-item p-4">
						<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('API Key')}</div>
						<SensitiveInput placeholder={$i18n.t('Enter Azure API Key')} bind:value={STT_AZURE_API_KEY} required />
					</div>
					<div class="glass-item p-4">
						<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('Azure Region')}</div>
						<input class="w-full py-2 px-3 text-sm dark:text-gray-300 glass-input" placeholder={$i18n.t('Enter Azure Region')} bind:value={STT_AZURE_REGION} required />
					</div>
				</div>
				<div class="glass-item p-4">
					<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('Language Locales')}</div>
					<input class="w-full py-2 px-3 text-sm dark:text-gray-300 glass-input" bind:value={STT_AZURE_LOCALES} placeholder={$i18n.t('e.g., en-US,ja-JP (leave blank for auto-detect)')} />
				</div>
			</div>
		{:else if STT_ENGINE === ''}
			<div class="space-y-3">
				<div class="text-sm font-medium text-gray-500 dark:text-gray-400 pl-1">
					{$i18n.t('Model Configuration')}
				</div>
				<div class="glass-item p-4">
					<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('STT Model')}</div>
					<div class="flex w-full gap-2">
						<input class="flex-1 py-2 px-3 text-sm dark:text-gray-300 glass-input" placeholder={$i18n.t('Set whisper model')} bind:value={STT_WHISPER_MODEL} />
						<button type="button" class="px-2.5 bg-gray-50 hover:bg-gray-200 text-gray-800 dark:bg-gray-850 dark:hover:bg-gray-800 dark:text-gray-100 rounded-lg transition" on:click={() => { sttModelUpdateHandler(); }} disabled={STT_WHISPER_MODEL_LOADING}>
							{#if STT_WHISPER_MODEL_LOADING}
								<div class="self-center"><svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor" xmlns="http://www.w3.org/2000/svg"><style>.spinner_ajPY{transform-origin:center;animation:spinner_AtaB .75s infinite linear}@keyframes spinner_AtaB{100%{transform:rotate(360deg)}}</style><path d="M12,1A11,11,0,1,0,23,12,11,11,0,0,0,12,1Zm0,19a8,8,0,1,1,8-8A8,8,0,0,1,12,20Z" opacity=".25"/><path d="M10.14,1.16a11,11,0,0,0-9,8.92A1.59,1.59,0,0,0,2.46,12,1.52,1.52,0,0,0,4.11,10.7a8,8,0,0,1,6.66-6.61A1.42,1.42,0,0,0,12,2.69h0A1.57,1.57,0,0,0,10.14,1.16Z" class="spinner_ajPY"/></svg></div>
							{:else}
								<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="currentColor" class="w-4 h-4"><path d="M8.75 2.75a.75.75 0 0 0-1.5 0v5.69L5.03 6.22a.75.75 0 0 0-1.06 1.06l3.5 3.5a.75.75 0 0 0 1.06 0l3.5-3.5a.75.75 0 0 0-1.06-1.06L8.75 8.44V2.75Z"/><path d="M3.5 9.75a.75.75 0 0 0-1.5 0v1.5A2.75 2.75 0 0 0 4.75 14h6.5A2.75 2.75 0 0 0 14 11.25v-1.5a.75.75 0 0 0-1.5 0v1.5c0 .69-.56 1.25-1.25 1.25h-6.5c-.69 0-1.25-.56-1.25-1.25v-1.5Z"/></svg>
							{/if}
						</button>
					</div>
					<div class="mt-1.5 text-xs text-gray-400 dark:text-gray-500">
						{$i18n.t(`Open WebUI uses faster-whisper internally.`)}
						<a class="hover:underline dark:text-gray-200 text-gray-800" href="https://github.com/SYSTRAN/faster-whisper" target="_blank">{$i18n.t(`Click here to learn more about faster-whisper and see the available models.`)}</a>
					</div>
				</div>
			</div>
		{/if}

		<!-- TTS -->
		<div class="text-sm font-medium text-gray-500 dark:text-gray-400 pl-1">
			{$i18n.t('TTS Settings')}
		</div>

		<div class="glass-item px-4 py-3">
			<div class="flex items-center justify-between">
				<div class="text-sm font-medium">{$i18n.t('Text-to-Speech Engine')}</div>
				<HaloSelect
					bind:value={TTS_ENGINE}
					placeholder="Select a mode"
					options={ttsEngineOptions}
					className="w-fit"
					on:change={async (e) => {
						await handleTTSEngineChange(e.detail.value);
					}}
				/>
			</div>
		</div>

		{#if TTS_ENGINE === 'openai'}
			<div class="space-y-3">
				<div class="text-sm font-medium text-gray-500 dark:text-gray-400 pl-1">{$i18n.t('Engine Credentials')}</div>
				<div class="grid grid-cols-1 md:grid-cols-2 gap-3">
					<div class="glass-item p-4">
						<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('API Base URL')}</div>
						<input class="w-full py-2 px-3 text-sm dark:text-gray-300 glass-input" placeholder={$i18n.t('Enter API Base URL')} bind:value={TTS_OPENAI_API_BASE_URL} required />
					</div>
					<div class="glass-item p-4">
						<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('API Key')}</div>
						<SensitiveInput placeholder={$i18n.t('Enter API Key')} bind:value={TTS_OPENAI_API_KEY} />
					</div>
				</div>
			</div>
		{:else if TTS_ENGINE === 'elevenlabs'}
			<div class="space-y-3">
				<div class="text-sm font-medium text-gray-500 dark:text-gray-400 pl-1">{$i18n.t('Engine Credentials')}</div>
				<div class="glass-item p-4">
					<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('API Key')}</div>
					<SensitiveInput placeholder={$i18n.t('Enter ElevenLabs API Key')} bind:value={TTS_API_KEY} />
				</div>
			</div>
		{:else if TTS_ENGINE === 'mimo'}
			<div class="space-y-3">
				<div class="text-sm font-medium text-gray-500 dark:text-gray-400 pl-1">{$i18n.t('Engine Credentials')}</div>
				<div class="glass-item p-4">
					<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('API Key')}</div>
					<SensitiveInput placeholder="Enter Xiaomi MiMo API Key" bind:value={TTS_API_KEY} />
				</div>
			</div>
		{:else if TTS_ENGINE === 'azure'}
			<div class="space-y-3">
				<div class="text-sm font-medium text-gray-500 dark:text-gray-400 pl-1">{$i18n.t('Engine Credentials')}</div>
				<div class="grid grid-cols-1 md:grid-cols-2 gap-3">
					<div class="glass-item p-4">
						<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('API Key')}</div>
						<SensitiveInput placeholder={$i18n.t('Enter Azure API Key')} bind:value={TTS_API_KEY} />
					</div>
					<div class="glass-item p-4">
						<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('Azure Region')}</div>
						<input class="w-full py-2 px-3 text-sm dark:text-gray-300 glass-input" placeholder={$i18n.t('Enter Azure Region')} bind:value={TTS_AZURE_SPEECH_REGION} required />
					</div>
				</div>
			</div>
		{/if}

		<!-- Voice & Model -->
		<div class="space-y-3">
			<div class="text-sm font-medium text-gray-500 dark:text-gray-400 pl-1">{$i18n.t('Voice & Model')}</div>

			{#if TTS_ENGINE === ''}
				<div class="glass-item p-4">
					<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('TTS Voice')}</div>
					<div class="w-full max-w-[15rem]">
						<HaloSelect bind:value={TTS_VOICE} placeholder={$i18n.t('Default')} options={[{ value: '', label: $i18n.t('Default') }, ...voices.map((voice) => ({ value: voice.voiceURI, label: voice.name }))]} className="w-full" />
					</div>
				</div>
			{:else if TTS_ENGINE === 'transformers'}
				<div class="glass-item p-4">
					<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('TTS Model')}</div>
					<input list="model-list" class="w-full py-2 px-3 text-sm dark:text-gray-300 glass-input" bind:value={TTS_MODEL} placeholder="CMU ARCTIC speaker embedding name" />
					<datalist id="model-list"><option value="tts-1" /></datalist>
					<div class="mt-1.5 text-xs text-gray-400 dark:text-gray-500">
						{$i18n.t(`Open WebUI uses SpeechT5 and CMU Arctic speaker embeddings.`)}
						To learn more about SpeechT5, <a class="hover:underline dark:text-gray-200 text-gray-800" href="https://github.com/microsoft/SpeechT5" target="_blank">{$i18n.t(`click here`, { name: 'SpeechT5' })}.</a>
						To see the available CMU Arctic speaker embeddings, <a class="hover:underline dark:text-gray-200 text-gray-800" href="https://huggingface.co/datasets/Matthijs/cmu-arctic-xvectors" target="_blank">{$i18n.t(`click here`)}.</a>
					</div>
				</div>
			{:else if TTS_ENGINE === 'openai'}
				<div class="grid grid-cols-1 md:grid-cols-2 gap-3">
					<div class="glass-item p-4">
						<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('TTS Voice')}</div>
						<input list="voice-list" class="w-full py-2 px-3 text-sm dark:text-gray-300 glass-input" bind:value={TTS_VOICE} placeholder="Select a voice" />
						<datalist id="voice-list">{#each voices as voice}<option value={voice.id}>{voice.name}</option>{/each}</datalist>
					</div>
					<div class="glass-item p-4">
						<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('TTS Model')}</div>
						<input list="tts-model-list" class="w-full py-2 px-3 text-sm dark:text-gray-300 glass-input" bind:value={TTS_MODEL} placeholder="Select a model" />
						<datalist id="tts-model-list">{#each models as model}<option value={model.id} />{/each}</datalist>
					</div>
				</div>
			{:else if TTS_ENGINE === 'elevenlabs'}
				<div class="grid grid-cols-1 md:grid-cols-2 gap-3">
					<div class="glass-item p-4">
						<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('TTS Voice')}</div>
						<input list="voice-list" class="w-full py-2 px-3 text-sm dark:text-gray-300 glass-input" bind:value={TTS_VOICE} placeholder="Select a voice" />
						<datalist id="voice-list">{#each voices as voice}<option value={voice.id}>{voice.name}</option>{/each}</datalist>
					</div>
					<div class="glass-item p-4">
						<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('TTS Model')}</div>
						<input list="tts-model-list" class="w-full py-2 px-3 text-sm dark:text-gray-300 glass-input" bind:value={TTS_MODEL} placeholder="Select a model" />
						<datalist id="tts-model-list">{#each models as model}<option value={model.id} />{/each}</datalist>
					</div>
				</div>
			{:else if TTS_ENGINE === 'mimo'}
				<div class="grid grid-cols-1 md:grid-cols-2 gap-3">
					<div class="glass-item p-4">
						<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('TTS Voice')}</div>
						<HaloSelect
							bind:value={TTS_VOICE}
							placeholder="Select a voice"
							options={MIMO_TTS_VOICE_OPTIONS}
							className="w-full"
							contentAlign="start"
						/>
					</div>
					<div class="glass-item p-4">
						<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('TTS Model')}</div>
						<HaloSelect
							bind:value={TTS_MODEL}
							placeholder="Select a model"
							options={MIMO_TTS_MODEL_OPTIONS}
							className="w-full"
							contentAlign="start"
						/>
					</div>
				</div>
			{:else if TTS_ENGINE === 'azure'}
				<div class="grid grid-cols-1 md:grid-cols-2 gap-3">
					<div class="glass-item p-4">
						<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('TTS Voice')}</div>
						<input list="voice-list" class="w-full py-2 px-3 text-sm dark:text-gray-300 glass-input" bind:value={TTS_VOICE} placeholder="Select a voice" />
						<datalist id="voice-list">{#each voices as voice}<option value={voice.id}>{voice.name}</option>{/each}</datalist>
					</div>
					<div class="glass-item p-4">
						<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">
							{$i18n.t('Output format')}
							<a href="https://learn.microsoft.com/en-us/azure/ai-services/speech-service/rest-text-to-speech?tabs=streaming#audio-outputs" target="_blank"><small class="ml-1 text-gray-400 hover:underline">{$i18n.t('Available list')}</small></a>
						</div>
						<input list="tts-model-list" class="w-full py-2 px-3 text-sm dark:text-gray-300 glass-input" bind:value={TTS_AZURE_SPEECH_OUTPUT_FORMAT} placeholder="Select a output format" />
					</div>
				</div>
			{/if}
		</div>

		<!-- Response Splitting -->
		<div class="glass-item px-4 py-3">
			<div class="flex items-center justify-between">
				<div class="text-sm font-medium">{$i18n.t('Response splitting')}</div>
				<HaloSelect bind:value={TTS_SPLIT_ON} options={Object.values(TTS_RESPONSE_SPLIT).map((split) => ({ value: split, label: $i18n.t(split.charAt(0).toUpperCase() + split.slice(1)) }))} className="w-fit" />
			</div>
		</div>
		<div class="text-xs text-gray-400 dark:text-gray-500 pl-1">
			{$i18n.t("Control how message text is split for TTS requests. 'Punctuation' splits into sentences, 'paragraphs' splits into paragraphs, and 'none' keeps the message as a single string.")}
		</div>
	</div>
{:else if isDocumentCard}
	<!-- ====== Document-card variant (embedded/unified settings) ====== -->
	<form
		class="flex flex-col space-y-3 text-sm max-w-6xl mx-auto w-full"
		on:submit|preventDefault={onSubmitHandler}
	>
		<div class="space-y-3">
			<div class="flex flex-col gap-3">
				<!-- STT document-card -->
				<div
					bind:this={sectionEl_stt}
					class="scroll-mt-2 rounded-xl border border-gray-200 dark:border-gray-800 overflow-hidden"
				>
					<button
						type="button"
						class="w-full flex items-center justify-between px-4 py-3 text-left"
						on:click={() => toggleSection('stt')}
					>
						<div class="mb-0 text-sm font-medium flex items-center gap-2">
							<span>{$i18n.t('STT Settings')}</span>
							{#if showScopeBadges && scopeLabel}
								<span class={`px-1.5 py-0.5 rounded-md text-[10px] font-medium ${scopeBadgeClass}`}>{scopeLabel}</span>
							{/if}
						</div>
						<div class="transform transition-transform duration-200 {expandedSections.stt ? 'rotate-180' : ''}">
							<ChevronDown className="size-4 text-gray-400" />
						</div>
					</button>

					{#if expandedSections.stt}
						<div
							class="px-4 pb-4 border-t border-gray-100 dark:border-gray-800"
							transition:slide={{ duration: 180, easing: quintOut }}
						>
							<div class="flex w-full items-center justify-between gap-3 py-1">
								<div class="min-w-0 pr-3 text-xs font-medium">{$i18n.t('Speech-to-Text Engine')}</div>
								<div class="relative flex shrink-0 items-center max-w-full">
					<HaloSelect
						bind:value={STT_ENGINE}
						placeholder="Select an engine"
						options={sttEngineOptions}
										className="w-fit"
									/>
								</div>
							</div>

							{#if STT_ENGINE === 'openai'}
								<div class="mt-1 flex gap-2 mb-1">
									<input class="flex-1 w-full bg-transparent outline-hidden" placeholder={$i18n.t('API Base URL')} bind:value={STT_OPENAI_API_BASE_URL} required />
									<SensitiveInput placeholder={$i18n.t('API Key')} bind:value={STT_OPENAI_API_KEY} />
								</div>
								<hr class="border-gray-100 dark:border-gray-850 my-2" />
								<div class="mb-1.5 text-sm font-medium">{$i18n.t('STT Model')}</div>
								<input list="model-list" class="w-full rounded-lg py-2 px-4 text-sm bg-gray-50 dark:text-gray-300 dark:bg-gray-850 outline-hidden" bind:value={STT_MODEL} placeholder="Select a model" />
								<datalist id="model-list"><option value="whisper-1" /></datalist>
							{:else if STT_ENGINE === 'deepgram'}
								<div class="mt-1 flex gap-2 mb-1">
									<SensitiveInput placeholder={$i18n.t('API Key')} bind:value={STT_DEEPGRAM_API_KEY} />
								</div>
								<hr class="border-gray-100 dark:border-gray-850 my-2" />
								<div class="mb-1.5 text-sm font-medium">{$i18n.t('STT Model')}</div>
								<input class="w-full rounded-lg py-2 px-4 text-sm bg-gray-50 dark:text-gray-300 dark:bg-gray-850 outline-hidden" bind:value={STT_MODEL} placeholder="Select a model (optional)" />
								<div class="mt-2 mb-1 text-xs text-gray-400 dark:text-gray-500">
									{$i18n.t('Leave model field empty to use the default model.')}
									<a class="hover:underline dark:text-gray-200 text-gray-800" href="https://developers.deepgram.com/docs/models" target="_blank">{$i18n.t('Click here to see available models.')}</a>
								</div>
							{:else if STT_ENGINE === 'azure'}
								<div class="mt-1 flex gap-2 mb-1">
									<SensitiveInput placeholder={$i18n.t('API Key')} bind:value={STT_AZURE_API_KEY} required />
									<input class="flex-1 w-full rounded-lg py-2 pl-4 text-sm bg-gray-50 dark:text-gray-300 dark:bg-gray-850 outline-hidden" placeholder={$i18n.t('Azure Region')} bind:value={STT_AZURE_REGION} required />
								</div>
								<hr class="border-gray-100 dark:border-gray-850 my-2" />
								<div class="mb-1.5 text-sm font-medium">{$i18n.t('Language Locales')}</div>
								<input class="w-full rounded-lg py-2 px-4 text-sm bg-gray-50 dark:text-gray-300 dark:bg-gray-850 outline-hidden" bind:value={STT_AZURE_LOCALES} placeholder={$i18n.t('e.g., en-US,ja-JP (leave blank for auto-detect)')} />
							{:else if STT_ENGINE === ''}
								<div class="mb-1.5 text-sm font-medium">{$i18n.t('STT Model')}</div>
								<div class="flex w-full">
									<div class="flex-1 mr-2">
										<input class="w-full rounded-lg py-2 px-4 text-sm bg-gray-50 dark:text-gray-300 dark:bg-gray-850 outline-hidden" placeholder={$i18n.t('Set whisper model')} bind:value={STT_WHISPER_MODEL} />
									</div>
									<button class="px-2.5 bg-gray-50 hover:bg-gray-200 text-gray-800 dark:bg-gray-850 dark:hover:bg-gray-800 dark:text-gray-100 rounded-lg transition" on:click={() => { sttModelUpdateHandler(); }} disabled={STT_WHISPER_MODEL_LOADING}>
										{#if STT_WHISPER_MODEL_LOADING}
											<div class="self-center"><svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor" xmlns="http://www.w3.org/2000/svg"><style>.spinner_ajPY{transform-origin:center;animation:spinner_AtaB .75s infinite linear}@keyframes spinner_AtaB{100%{transform:rotate(360deg)}}</style><path d="M12,1A11,11,0,1,0,23,12,11,11,0,0,0,12,1Zm0,19a8,8,0,1,1,8-8A8,8,0,0,1,12,20Z" opacity=".25"/><path d="M10.14,1.16a11,11,0,0,0-9,8.92A1.59,1.59,0,0,0,2.46,12,1.52,1.52,0,0,0,4.11,10.7a8,8,0,0,1,6.66-6.61A1.42,1.42,0,0,0,12,2.69h0A1.57,1.57,0,0,0,10.14,1.16Z" class="spinner_ajPY"/></svg></div>
										{:else}
											<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="currentColor" class="w-4 h-4"><path d="M8.75 2.75a.75.75 0 0 0-1.5 0v5.69L5.03 6.22a.75.75 0 0 0-1.06 1.06l3.5 3.5a.75.75 0 0 0 1.06 0l3.5-3.5a.75.75 0 0 0-1.06-1.06L8.75 8.44V2.75Z"/><path d="M3.5 9.75a.75.75 0 0 0-1.5 0v1.5A2.75 2.75 0 0 0 4.75 14h6.5A2.75 2.75 0 0 0 14 11.25v-1.5a.75.75 0 0 0-1.5 0v1.5c0 .69-.56 1.25-1.25 1.25h-6.5c-.69 0-1.25-.56-1.25-1.25v-1.5Z"/></svg>
										{/if}
									</button>
								</div>
								<div class="mt-2 mb-1 text-xs text-gray-400 dark:text-gray-500">
									{$i18n.t(`Open WebUI uses faster-whisper internally.`)}
									<a class="hover:underline dark:text-gray-200 text-gray-800" href="https://github.com/SYSTRAN/faster-whisper" target="_blank">{$i18n.t(`Click here to learn more about faster-whisper and see the available models.`)}</a>
								</div>
							{/if}
						</div>
					{/if}
				</div>

				<!-- TTS document-card -->
				<div
					bind:this={sectionEl_tts}
					class="scroll-mt-2 rounded-xl border border-gray-200 dark:border-gray-800 overflow-hidden"
				>
					<button
						type="button"
						class="w-full flex items-center justify-between px-4 py-3 text-left"
						on:click={() => toggleSection('tts')}
					>
						<div class="mb-0 text-sm font-medium flex items-center gap-2">
							<span>{$i18n.t('TTS Settings')}</span>
							{#if showScopeBadges && scopeLabel}
								<span class={`px-1.5 py-0.5 rounded-md text-[10px] font-medium ${scopeBadgeClass}`}>{scopeLabel}</span>
							{/if}
						</div>
						<div class="transform transition-transform duration-200 {expandedSections.tts ? 'rotate-180' : ''}">
							<ChevronDown className="size-4 text-gray-400" />
						</div>
					</button>

					{#if expandedSections.tts}
						<div
							class="px-4 pb-4 border-t border-gray-100 dark:border-gray-800"
							transition:slide={{ duration: 180, easing: quintOut }}
						>
							<div class="flex w-full items-center justify-between gap-3 py-1">
								<div class="min-w-0 pr-3 text-xs font-medium">{$i18n.t('Text-to-Speech Engine')}</div>
								<div class="relative flex shrink-0 items-center max-w-full">
					<HaloSelect
						bind:value={TTS_ENGINE}
						placeholder="Select a mode"
						options={ttsEngineOptions}
										className="w-fit"
										on:change={async (e) => {
											await handleTTSEngineChange(e.detail.value);
										}}
									/>
								</div>
							</div>

							{#if TTS_ENGINE === 'openai'}
								<div class="mt-1 flex gap-2 mb-1">
									<input class="flex-1 w-full bg-transparent outline-hidden" placeholder={$i18n.t('API Base URL')} bind:value={TTS_OPENAI_API_BASE_URL} required />
									<SensitiveInput placeholder={$i18n.t('API Key')} bind:value={TTS_OPENAI_API_KEY} />
								</div>
							{:else if TTS_ENGINE === 'elevenlabs'}
								<div class="mt-1 flex gap-2 mb-1">
									<input class="flex-1 w-full rounded-lg py-2 pl-4 text-sm bg-gray-50 dark:text-gray-300 dark:bg-gray-850 outline-hidden" placeholder={$i18n.t('API Key')} bind:value={TTS_API_KEY} required />
								</div>
							{:else if TTS_ENGINE === 'mimo'}
								<div class="mt-1 flex gap-2 mb-1">
									<SensitiveInput placeholder="Xiaomi MiMo API Key" bind:value={TTS_API_KEY} />
								</div>
							{:else if TTS_ENGINE === 'azure'}
								<div class="mt-1 flex gap-2 mb-1">
									<input class="flex-1 w-full rounded-lg py-2 pl-4 text-sm bg-gray-50 dark:text-gray-300 dark:bg-gray-850 outline-hidden" placeholder={$i18n.t('API Key')} bind:value={TTS_API_KEY} required />
									<input class="flex-1 w-full rounded-lg py-2 pl-4 text-sm bg-gray-50 dark:text-gray-300 dark:bg-gray-850 outline-hidden" placeholder={$i18n.t('Azure Region')} bind:value={TTS_AZURE_SPEECH_REGION} required />
								</div>
							{/if}

							<hr class="border-gray-100 dark:border-gray-850 my-2" />

							{#if TTS_ENGINE === ''}
								<div class="mb-1.5 text-sm font-medium">{$i18n.t('TTS Voice')}</div>
								<div class="w-full max-w-[15rem]">
									<HaloSelect bind:value={TTS_VOICE} placeholder={$i18n.t('Default')} options={[{ value: '', label: $i18n.t('Default') }, ...voices.map((voice) => ({ value: voice.voiceURI, label: voice.name }))]} className="w-full" />
								</div>
							{:else if TTS_ENGINE === 'transformers'}
								<div class="mb-1.5 text-sm font-medium">{$i18n.t('TTS Model')}</div>
								<input list="model-list" class="w-full rounded-lg py-2 px-4 text-sm bg-gray-50 dark:text-gray-300 dark:bg-gray-850 outline-hidden" bind:value={TTS_MODEL} placeholder="CMU ARCTIC speaker embedding name" />
								<datalist id="model-list"><option value="tts-1" /></datalist>
								<div class="mt-2 mb-1 text-xs text-gray-400 dark:text-gray-500">
									{$i18n.t(`Open WebUI uses SpeechT5 and CMU Arctic speaker embeddings.`)}
									To learn more about SpeechT5, <a class="hover:underline dark:text-gray-200 text-gray-800" href="https://github.com/microsoft/SpeechT5" target="_blank">{$i18n.t(`click here`, { name: 'SpeechT5' })}.</a>
									To see the available CMU Arctic speaker embeddings, <a class="hover:underline dark:text-gray-200 text-gray-800" href="https://huggingface.co/datasets/Matthijs/cmu-arctic-xvectors" target="_blank">{$i18n.t(`click here`)}.</a>
								</div>
							{:else if TTS_ENGINE === 'openai'}
								<div class="flex gap-2">
									<div class="w-full"><div class="mb-1.5 text-sm font-medium">{$i18n.t('TTS Voice')}</div><input list="voice-list" class="w-full rounded-lg py-2 px-4 text-sm bg-gray-50 dark:text-gray-300 dark:bg-gray-850 outline-hidden" bind:value={TTS_VOICE} placeholder="Select a voice" /><datalist id="voice-list">{#each voices as voice}<option value={voice.id}>{voice.name}</option>{/each}</datalist></div>
									<div class="w-full"><div class="mb-1.5 text-sm font-medium">{$i18n.t('TTS Model')}</div><input list="tts-model-list" class="w-full rounded-lg py-2 px-4 text-sm bg-gray-50 dark:text-gray-300 dark:bg-gray-850 outline-hidden" bind:value={TTS_MODEL} placeholder="Select a model" /><datalist id="tts-model-list">{#each models as model}<option value={model.id} />{/each}</datalist></div>
								</div>
							{:else if TTS_ENGINE === 'elevenlabs'}
								<div class="flex gap-2">
									<div class="w-full"><div class="mb-1.5 text-sm font-medium">{$i18n.t('TTS Voice')}</div><input list="voice-list" class="w-full rounded-lg py-2 px-4 text-sm bg-gray-50 dark:text-gray-300 dark:bg-gray-850 outline-hidden" bind:value={TTS_VOICE} placeholder="Select a voice" /><datalist id="voice-list">{#each voices as voice}<option value={voice.id}>{voice.name}</option>{/each}</datalist></div>
									<div class="w-full"><div class="mb-1.5 text-sm font-medium">{$i18n.t('TTS Model')}</div><input list="tts-model-list" class="w-full rounded-lg py-2 px-4 text-sm bg-gray-50 dark:text-gray-300 dark:bg-gray-850 outline-hidden" bind:value={TTS_MODEL} placeholder="Select a model" /><datalist id="tts-model-list">{#each models as model}<option value={model.id} />{/each}</datalist></div>
								</div>
							{:else if TTS_ENGINE === 'mimo'}
								<div class="flex gap-2">
									<div class="w-full">
										<div class="mb-1.5 text-sm font-medium">{$i18n.t('TTS Voice')}</div>
										<HaloSelect
											bind:value={TTS_VOICE}
											placeholder="Select a voice"
											options={MIMO_TTS_VOICE_OPTIONS}
											className="w-full"
											contentAlign="start"
										/>
									</div>
									<div class="w-full">
										<div class="mb-1.5 text-sm font-medium">{$i18n.t('TTS Model')}</div>
										<HaloSelect
											bind:value={TTS_MODEL}
											placeholder="Select a model"
											options={MIMO_TTS_MODEL_OPTIONS}
											className="w-full"
											contentAlign="start"
										/>
									</div>
								</div>
							{:else if TTS_ENGINE === 'azure'}
								<div class="flex gap-2">
									<div class="w-full"><div class="mb-1.5 text-sm font-medium">{$i18n.t('TTS Voice')}</div><input list="voice-list" class="w-full rounded-lg py-2 px-4 text-sm bg-gray-50 dark:text-gray-300 dark:bg-gray-850 outline-hidden" bind:value={TTS_VOICE} placeholder="Select a voice" /><datalist id="voice-list">{#each voices as voice}<option value={voice.id}>{voice.name}</option>{/each}</datalist></div>
									<div class="w-full">
										<div class="mb-1.5 text-sm font-medium">{$i18n.t('Output format')} <a href="https://learn.microsoft.com/en-us/azure/ai-services/speech-service/rest-text-to-speech?tabs=streaming#audio-outputs" target="_blank"><small>{$i18n.t('Available list')}</small></a></div>
										<input list="tts-model-list" class="w-full rounded-lg py-2 px-4 text-sm bg-gray-50 dark:text-gray-300 dark:bg-gray-850 outline-hidden" bind:value={TTS_AZURE_SPEECH_OUTPUT_FORMAT} placeholder="Select a output format" />
									</div>
								</div>
							{/if}

							<hr class="border-gray-100 dark:border-gray-850 my-2" />

							<div class="flex w-full items-center justify-between gap-3 py-1">
								<div class="min-w-0 pr-3 text-xs font-medium">{$i18n.t('Response splitting')}</div>
								<div class="relative flex shrink-0 items-center max-w-full">
									<HaloSelect bind:value={TTS_SPLIT_ON} options={Object.values(TTS_RESPONSE_SPLIT).map((split) => ({ value: split, label: $i18n.t(split.charAt(0).toUpperCase() + split.slice(1)) }))} className="w-fit" />
								</div>
							</div>
							<div class="mt-2 mb-1 text-xs text-gray-400 dark:text-gray-500">
								{$i18n.t("Control how message text is split for TTS requests. 'Punctuation' splits into sentences, 'paragraphs' splits into paragraphs, and 'none' keeps the message as a single string.")}
							</div>
						</div>
					{/if}
				</div>
			</div>
		</div>
	</form>
{:else}
	<!-- ====== Glass design variant (admin settings page) ====== -->
	<form on:submit|preventDefault={onSubmitHandler}>
		<div class="h-full space-y-6 overflow-y-auto scrollbar-hidden">
			<div class="max-w-6xl mx-auto space-y-6">
				<!-- ====== STT Section ====== -->
				<section
					bind:this={sectionEl_stt}
					class="scroll-mt-2 p-5 space-y-5 transition-all duration-300 {dirtySections.stt
						? 'glass-section glass-section-dirty'
						: 'glass-section'}"
				>
					<div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
						<button
							type="button"
							class="flex min-w-0 flex-1 items-center justify-between gap-4 text-left"
							on:click={async () => {
								expandedSections.stt = !expandedSections.stt;
								if (expandedSections.stt) {
									await revealExpandedSection(sectionEl_stt);
								}
							}}
						>
							<div class="flex items-center gap-3">
								<div class="glass-icon-badge bg-green-50 dark:bg-green-950/30">
									<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="size-[18px] text-green-500 dark:text-green-400">
										<path d="M8.25 4.5a3.75 3.75 0 117.5 0v8.25a3.75 3.75 0 11-7.5 0V4.5z" />
										<path d="M6 10.5a.75.75 0 01.75.75v1.5a5.25 5.25 0 1010.5 0v-1.5a.75.75 0 011.5 0v1.5a6.751 6.751 0 01-6 6.709v2.291h3a.75.75 0 010 1.5h-7.5a.75.75 0 010-1.5h3v-2.291a6.751 6.751 0 01-6-6.709v-1.5A.75.75 0 016 10.5z" />
									</svg>
								</div>
								<div class="text-base font-semibold text-gray-800 dark:text-gray-100">
									{$i18n.t('STT Settings')}
									{#if showScopeBadges && scopeLabel}
										<span class={`ml-2 px-1.5 py-0.5 rounded-md text-[10px] font-medium ${scopeBadgeClass}`}>{scopeLabel}</span>
									{/if}
								</div>
							</div>
							<div class="transform transition-transform duration-200 {expandedSections.stt ? 'rotate-180' : ''}">
								<ChevronDown className="size-5 text-gray-400" />
							</div>
						</button>

						{#if showSubmit}
							<InlineDirtyActions
								dirty={dirtySections.stt}
								saving={isSaving}
								on:reset={() => resetSectionChanges('stt')}
							/>
						{/if}
					</div>

					{#if expandedSections.stt}
						<div transition:slide={{ duration: 200, easing: quintOut }} class="space-y-3">
							<!-- Engine Selection -->
							<div class="glass-item px-4 py-3">
								<div class="flex items-center justify-between">
									<div class="text-sm font-medium">{$i18n.t('Speech-to-Text Engine')}</div>
									<HaloSelect
										bind:value={STT_ENGINE}
										placeholder="Select an engine"
										options={[
											{ value: '', label: $i18n.t('Whisper (Local)') },
											{ value: 'openai', label: 'OpenAI' },
											{ value: 'web', label: $i18n.t('Web API') },
											{ value: 'deepgram', label: 'Deepgram' },
											{ value: 'azure', label: 'Azure AI Speech' }
										]}
										className="w-fit"
									/>
								</div>
							</div>

							<!-- Engine Credentials -->
							{#if STT_ENGINE === 'openai'}
								<div class="space-y-3">
									<div class="text-sm font-medium text-gray-500 dark:text-gray-400 pl-1">
										{$i18n.t('Engine Credentials')}
									</div>
									<div class="grid grid-cols-1 md:grid-cols-2 gap-3">
										<div class="glass-item p-4">
											<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('API Base URL')}</div>
											<input
												class="w-full py-2 px-3 text-sm dark:text-gray-300 glass-input"
												placeholder={$i18n.t('Enter API Base URL')}
												bind:value={STT_OPENAI_API_BASE_URL}
												required
											/>
										</div>
										<div class="glass-item p-4">
											<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('API Key')}</div>
											<SensitiveInput
												placeholder={$i18n.t('Enter API Key')}
												bind:value={STT_OPENAI_API_KEY}
											/>
										</div>
									</div>

									<div class="glass-item p-4">
										<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('STT Model')}</div>
										<input
											list="model-list"
											class="w-full py-2 px-3 text-sm dark:text-gray-300 glass-input"
											bind:value={STT_MODEL}
											placeholder="Select a model"
										/>
										<datalist id="model-list">
											<option value="whisper-1" />
										</datalist>
									</div>
								</div>
							{:else if STT_ENGINE === 'deepgram'}
								<div class="space-y-3">
									<div class="text-sm font-medium text-gray-500 dark:text-gray-400 pl-1">
										{$i18n.t('Engine Credentials')}
									</div>
									<div class="glass-item p-4">
										<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('API Key')}</div>
										<SensitiveInput
											placeholder={$i18n.t('Enter Deepgram API Key')}
											bind:value={STT_DEEPGRAM_API_KEY}
										/>
									</div>

									<div class="glass-item p-4">
										<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('STT Model')}</div>
										<input
											class="w-full py-2 px-3 text-sm dark:text-gray-300 glass-input"
											bind:value={STT_MODEL}
											placeholder="Select a model (optional)"
										/>
										<div class="mt-1.5 text-xs text-gray-400 dark:text-gray-500">
											{$i18n.t('Leave model field empty to use the default model.')}
											<a class="hover:underline dark:text-gray-200 text-gray-800" href="https://developers.deepgram.com/docs/models" target="_blank">
												{$i18n.t('Click here to see available models.')}
											</a>
										</div>
									</div>
								</div>
							{:else if STT_ENGINE === 'azure'}
								<div class="space-y-3">
									<div class="text-sm font-medium text-gray-500 dark:text-gray-400 pl-1">
										{$i18n.t('Engine Credentials')}
									</div>
									<div class="grid grid-cols-1 md:grid-cols-2 gap-3">
										<div class="glass-item p-4">
											<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('API Key')}</div>
											<SensitiveInput
												placeholder={$i18n.t('Enter Azure API Key')}
												bind:value={STT_AZURE_API_KEY}
												required
											/>
										</div>
										<div class="glass-item p-4">
											<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('Azure Region')}</div>
											<input
												class="w-full py-2 px-3 text-sm dark:text-gray-300 glass-input"
												placeholder={$i18n.t('Enter Azure Region')}
												bind:value={STT_AZURE_REGION}
												required
											/>
										</div>
									</div>

									<div class="glass-item p-4">
										<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('Language Locales')}</div>
										<input
											class="w-full py-2 px-3 text-sm dark:text-gray-300 glass-input"
											bind:value={STT_AZURE_LOCALES}
											placeholder={$i18n.t('e.g., en-US,ja-JP (leave blank for auto-detect)')}
										/>
									</div>
								</div>
							{:else if STT_ENGINE === ''}
								<div class="space-y-3">
									<div class="text-sm font-medium text-gray-500 dark:text-gray-400 pl-1">
										{$i18n.t('Model Configuration')}
									</div>
									<div class="glass-item p-4">
										<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('STT Model')}</div>
										<div class="flex w-full gap-2">
											<input
												class="flex-1 py-2 px-3 text-sm dark:text-gray-300 glass-input"
												placeholder={$i18n.t('Set whisper model')}
												bind:value={STT_WHISPER_MODEL}
											/>
											<button
												type="button"
												class="px-2.5 bg-gray-50 hover:bg-gray-200 text-gray-800 dark:bg-gray-850 dark:hover:bg-gray-800 dark:text-gray-100 rounded-lg transition"
												on:click={() => { sttModelUpdateHandler(); }}
												disabled={STT_WHISPER_MODEL_LOADING}
											>
												{#if STT_WHISPER_MODEL_LOADING}
													<div class="self-center"><svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor" xmlns="http://www.w3.org/2000/svg"><style>.spinner_ajPY{transform-origin:center;animation:spinner_AtaB .75s infinite linear}@keyframes spinner_AtaB{100%{transform:rotate(360deg)}}</style><path d="M12,1A11,11,0,1,0,23,12,11,11,0,0,0,12,1Zm0,19a8,8,0,1,1,8-8A8,8,0,0,1,12,20Z" opacity=".25"/><path d="M10.14,1.16a11,11,0,0,0-9,8.92A1.59,1.59,0,0,0,2.46,12,1.52,1.52,0,0,0,4.11,10.7a8,8,0,0,1,6.66-6.61A1.42,1.42,0,0,0,12,2.69h0A1.57,1.57,0,0,0,10.14,1.16Z" class="spinner_ajPY"/></svg></div>
												{:else}
													<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="currentColor" class="w-4 h-4"><path d="M8.75 2.75a.75.75 0 0 0-1.5 0v5.69L5.03 6.22a.75.75 0 0 0-1.06 1.06l3.5 3.5a.75.75 0 0 0 1.06 0l3.5-3.5a.75.75 0 0 0-1.06-1.06L8.75 8.44V2.75Z"/><path d="M3.5 9.75a.75.75 0 0 0-1.5 0v1.5A2.75 2.75 0 0 0 4.75 14h6.5A2.75 2.75 0 0 0 14 11.25v-1.5a.75.75 0 0 0-1.5 0v1.5c0 .69-.56 1.25-1.25 1.25h-6.5c-.69 0-1.25-.56-1.25-1.25v-1.5Z"/></svg>
												{/if}
											</button>
										</div>
										<div class="mt-1.5 text-xs text-gray-400 dark:text-gray-500">
											{$i18n.t(`Open WebUI uses faster-whisper internally.`)}
											<a class="hover:underline dark:text-gray-200 text-gray-800" href="https://github.com/SYSTRAN/faster-whisper" target="_blank">
												{$i18n.t(`Click here to learn more about faster-whisper and see the available models.`)}
											</a>
										</div>
									</div>
								</div>
							{/if}
						</div>
					{/if}
				</section>

				<!-- ====== TTS Section ====== -->
				<section
					bind:this={sectionEl_tts}
					class="scroll-mt-2 p-5 space-y-5 transition-all duration-300 {dirtySections.tts
						? 'glass-section glass-section-dirty'
						: 'glass-section'}"
				>
					<div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
						<button
							type="button"
							class="flex min-w-0 flex-1 items-center justify-between gap-4 text-left"
							on:click={async () => {
								expandedSections.tts = !expandedSections.tts;
								if (expandedSections.tts) {
									await revealExpandedSection(sectionEl_tts);
								}
							}}
						>
							<div class="flex items-center gap-3">
								<div class="glass-icon-badge bg-orange-50 dark:bg-orange-950/30">
									<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="size-[18px] text-orange-500 dark:text-orange-400">
										<path d="M13.5 4.06c0-1.336-1.616-2.005-2.56-1.06l-4.5 4.5H4.508c-1.141 0-2.318.664-2.66 1.905A9.76 9.76 0 001.5 12c0 .898.121 1.768.35 2.595.341 1.24 1.518 1.905 2.659 1.905h1.93l4.5 4.5c.945.945 2.561.276 2.561-1.06V4.06zM18.584 5.106a.75.75 0 011.06 0c3.808 3.807 3.808 9.98 0 13.788a.75.75 0 11-1.06-1.06 8.25 8.25 0 000-11.668.75.75 0 010-1.06z" />
										<path d="M15.932 7.757a.75.75 0 011.061 0 6 6 0 010 8.486.75.75 0 01-1.06-1.061 4.5 4.5 0 000-6.364.75.75 0 010-1.06z" />
									</svg>
								</div>
								<div class="text-base font-semibold text-gray-800 dark:text-gray-100">
									{$i18n.t('TTS Settings')}
									{#if showScopeBadges && scopeLabel}
										<span class={`ml-2 px-1.5 py-0.5 rounded-md text-[10px] font-medium ${scopeBadgeClass}`}>{scopeLabel}</span>
									{/if}
								</div>
							</div>
							<div class="transform transition-transform duration-200 {expandedSections.tts ? 'rotate-180' : ''}">
								<ChevronDown className="size-5 text-gray-400" />
							</div>
						</button>

						{#if showSubmit}
							<InlineDirtyActions
								dirty={dirtySections.tts}
								saving={isSaving}
								on:reset={() => resetSectionChanges('tts')}
							/>
						{/if}
					</div>

					{#if expandedSections.tts}
						<div transition:slide={{ duration: 200, easing: quintOut }} class="space-y-3">
							<!-- Engine Selection -->
							<div class="glass-item px-4 py-3">
								<div class="flex items-center justify-between">
									<div class="text-sm font-medium">{$i18n.t('Text-to-Speech Engine')}</div>
									<HaloSelect
										bind:value={TTS_ENGINE}
										placeholder="Select a mode"
										options={[
											{ value: '', label: $i18n.t('Web API') },
											{ value: 'transformers', label: `${$i18n.t('Transformers')} (${$i18n.t('Local')})` },
											{ value: 'openai', label: $i18n.t('OpenAI') },
											{ value: 'mimo', label: 'Xiaomi MiMo' },
											{ value: 'elevenlabs', label: $i18n.t('ElevenLabs') },
											{ value: 'azure', label: $i18n.t('Azure AI Speech') }
										]}
										className="w-fit"
										on:change={async (e) => {
											await handleTTSEngineChange(e.detail.value);
										}}
									/>
								</div>
							</div>

							<!-- Engine Credentials -->
							{#if TTS_ENGINE === 'openai'}
								<div class="space-y-3">
									<div class="text-sm font-medium text-gray-500 dark:text-gray-400 pl-1">
										{$i18n.t('Engine Credentials')}
									</div>
									<div class="grid grid-cols-1 md:grid-cols-2 gap-3">
										<div class="glass-item p-4">
											<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('API Base URL')}</div>
											<input
												class="w-full py-2 px-3 text-sm dark:text-gray-300 glass-input"
												placeholder={$i18n.t('Enter API Base URL')}
												bind:value={TTS_OPENAI_API_BASE_URL}
												required
											/>
										</div>
										<div class="glass-item p-4">
											<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('API Key')}</div>
											<SensitiveInput
												placeholder={$i18n.t('Enter API Key')}
												bind:value={TTS_OPENAI_API_KEY}
											/>
										</div>
									</div>
								</div>
							{:else if TTS_ENGINE === 'elevenlabs'}
								<div class="space-y-3">
									<div class="text-sm font-medium text-gray-500 dark:text-gray-400 pl-1">
										{$i18n.t('Engine Credentials')}
									</div>
									<div class="glass-item p-4">
										<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('API Key')}</div>
										<SensitiveInput
											placeholder={$i18n.t('Enter ElevenLabs API Key')}
											bind:value={TTS_API_KEY}
										/>
									</div>
								</div>
							{:else if TTS_ENGINE === 'mimo'}
								<div class="space-y-3">
									<div class="text-sm font-medium text-gray-500 dark:text-gray-400 pl-1">
										{$i18n.t('Engine Credentials')}
									</div>
									<div class="glass-item p-4">
										<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('API Key')}</div>
										<SensitiveInput
											placeholder="Enter Xiaomi MiMo API Key"
											bind:value={TTS_API_KEY}
										/>
									</div>
								</div>
							{:else if TTS_ENGINE === 'azure'}
								<div class="space-y-3">
									<div class="text-sm font-medium text-gray-500 dark:text-gray-400 pl-1">
										{$i18n.t('Engine Credentials')}
									</div>
									<div class="grid grid-cols-1 md:grid-cols-2 gap-3">
										<div class="glass-item p-4">
											<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('API Key')}</div>
											<SensitiveInput
												placeholder={$i18n.t('Enter Azure API Key')}
												bind:value={TTS_API_KEY}
											/>
										</div>
										<div class="glass-item p-4">
											<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('Azure Region')}</div>
											<input
												class="w-full py-2 px-3 text-sm dark:text-gray-300 glass-input"
												placeholder={$i18n.t('Enter Azure Region')}
												bind:value={TTS_AZURE_SPEECH_REGION}
												required
											/>
										</div>
									</div>
								</div>
							{/if}

							<!-- Voice / Model Configuration -->
							<div class="space-y-3">
								<div class="text-sm font-medium text-gray-500 dark:text-gray-400 pl-1">
									{$i18n.t('Voice & Model')}
								</div>

								{#if TTS_ENGINE === ''}
									<div class="glass-item p-4">
										<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('TTS Voice')}</div>
										<div class="w-full max-w-[15rem]">
											<HaloSelect
												bind:value={TTS_VOICE}
												placeholder={$i18n.t('Default')}
												options={[
													{ value: '', label: $i18n.t('Default') },
													...voices.map((voice) => ({ value: voice.voiceURI, label: voice.name }))
												]}
												className="w-full"
											/>
										</div>
									</div>
								{:else if TTS_ENGINE === 'transformers'}
									<div class="glass-item p-4">
										<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('TTS Model')}</div>
										<input
											list="model-list"
											class="w-full py-2 px-3 text-sm dark:text-gray-300 glass-input"
											bind:value={TTS_MODEL}
											placeholder="CMU ARCTIC speaker embedding name"
										/>
										<datalist id="model-list">
											<option value="tts-1" />
										</datalist>
										<div class="mt-1.5 text-xs text-gray-400 dark:text-gray-500">
											{$i18n.t(`Open WebUI uses SpeechT5 and CMU Arctic speaker embeddings.`)}
											To learn more about SpeechT5,
											<a class="hover:underline dark:text-gray-200 text-gray-800" href="https://github.com/microsoft/SpeechT5" target="_blank">
												{$i18n.t(`click here`, { name: 'SpeechT5' })}.
											</a>
											To see the available CMU Arctic speaker embeddings,
											<a class="hover:underline dark:text-gray-200 text-gray-800" href="https://huggingface.co/datasets/Matthijs/cmu-arctic-xvectors" target="_blank">
												{$i18n.t(`click here`)}.
											</a>
										</div>
									</div>
								{:else if TTS_ENGINE === 'openai'}
									<div class="grid grid-cols-1 md:grid-cols-2 gap-3">
										<div class="glass-item p-4">
											<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('TTS Voice')}</div>
											<input
												list="voice-list"
												class="w-full py-2 px-3 text-sm dark:text-gray-300 glass-input"
												bind:value={TTS_VOICE}
												placeholder="Select a voice"
											/>
											<datalist id="voice-list">
												{#each voices as voice}
													<option value={voice.id}>{voice.name}</option>
												{/each}
											</datalist>
										</div>
										<div class="glass-item p-4">
											<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('TTS Model')}</div>
											<input
												list="tts-model-list"
												class="w-full py-2 px-3 text-sm dark:text-gray-300 glass-input"
												bind:value={TTS_MODEL}
												placeholder="Select a model"
											/>
											<datalist id="tts-model-list">
												{#each models as model}
													<option value={model.id} />
												{/each}
											</datalist>
										</div>
									</div>
								{:else if TTS_ENGINE === 'elevenlabs'}
									<div class="grid grid-cols-1 md:grid-cols-2 gap-3">
										<div class="glass-item p-4">
											<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('TTS Voice')}</div>
											<input
												list="voice-list"
												class="w-full py-2 px-3 text-sm dark:text-gray-300 glass-input"
												bind:value={TTS_VOICE}
												placeholder="Select a voice"
											/>
											<datalist id="voice-list">
												{#each voices as voice}
													<option value={voice.id}>{voice.name}</option>
												{/each}
											</datalist>
										</div>
										<div class="glass-item p-4">
											<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('TTS Model')}</div>
											<input
												list="tts-model-list"
												class="w-full py-2 px-3 text-sm dark:text-gray-300 glass-input"
												bind:value={TTS_MODEL}
												placeholder="Select a model"
											/>
											<datalist id="tts-model-list">
												{#each models as model}
													<option value={model.id} />
												{/each}
											</datalist>
										</div>
									</div>
								{:else if TTS_ENGINE === 'mimo'}
									<div class="grid grid-cols-1 md:grid-cols-2 gap-3">
										<div class="glass-item p-4">
											<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('TTS Voice')}</div>
											<HaloSelect
												bind:value={TTS_VOICE}
												placeholder="Select a voice"
												options={MIMO_TTS_VOICE_OPTIONS}
												className="w-full"
												contentAlign="start"
											/>
										</div>
										<div class="glass-item p-4">
											<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('TTS Model')}</div>
											<HaloSelect
												bind:value={TTS_MODEL}
												placeholder="Select a model"
												options={MIMO_TTS_MODEL_OPTIONS}
												className="w-full"
												contentAlign="start"
											/>
										</div>
									</div>
								{:else if TTS_ENGINE === 'azure'}
									<div class="grid grid-cols-1 md:grid-cols-2 gap-3">
										<div class="glass-item p-4">
											<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('TTS Voice')}</div>
											<input
												list="voice-list"
												class="w-full py-2 px-3 text-sm dark:text-gray-300 glass-input"
												bind:value={TTS_VOICE}
												placeholder="Select a voice"
											/>
											<datalist id="voice-list">
												{#each voices as voice}
													<option value={voice.id}>{voice.name}</option>
												{/each}
											</datalist>
										</div>
										<div class="glass-item p-4">
											<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">
												{$i18n.t('Output format')}
												<a href="https://learn.microsoft.com/en-us/azure/ai-services/speech-service/rest-text-to-speech?tabs=streaming#audio-outputs" target="_blank">
													<small class="ml-1 text-gray-400 hover:underline">{$i18n.t('Available list')}</small>
												</a>
											</div>
											<input
												list="tts-model-list"
												class="w-full py-2 px-3 text-sm dark:text-gray-300 glass-input"
												bind:value={TTS_AZURE_SPEECH_OUTPUT_FORMAT}
												placeholder="Select a output format"
											/>
										</div>
									</div>
								{/if}
							</div>

							<!-- Response Splitting -->
							<div class="glass-item px-4 py-3">
								<div class="flex items-center justify-between">
									<div class="text-sm font-medium">{$i18n.t('Response splitting')}</div>
									<HaloSelect
										bind:value={TTS_SPLIT_ON}
										options={Object.values(TTS_RESPONSE_SPLIT).map((split) => ({
											value: split,
											label: $i18n.t(split.charAt(0).toUpperCase() + split.slice(1))
										}))}
										className="w-fit"
									/>
								</div>
							</div>
							<div class="text-xs text-gray-400 dark:text-gray-500">
								{$i18n.t(
									"Control how message text is split for TTS requests. 'Punctuation' splits into sentences, 'paragraphs' splits into paragraphs, and 'none' keeps the message as a single string."
								)}
							</div>
						</div>
					{/if}
				</section>
			</div>
		</div>
	</form>
{/if}
