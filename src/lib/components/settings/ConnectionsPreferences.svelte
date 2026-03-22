<script lang="ts">
	import type { Writable } from 'svelte/store';
	import { createEventDispatcher, getContext, onMount } from 'svelte';

	import { toast } from 'svelte-sonner';
	import { settings } from '$lib/stores';
	import Spinner from '$lib/components/common/Spinner.svelte';
	import Plus from '$lib/components/icons/Plus.svelte';

	import AddConnectionModal from '$lib/components/AddConnectionModal.svelte';

	// Reuse the admin UI building blocks to keep a single visual language.
	import OpenAIConnection from '$lib/components/admin/Settings/Connections/OpenAIConnection.svelte';
	import GeminiConnection from '$lib/components/admin/Settings/Connections/GeminiConnection.svelte';
	import AnthropicConnection from '$lib/components/admin/Settings/Connections/AnthropicConnection.svelte';
	import OllamaConnectionCard from '$lib/components/settings/OllamaConnectionCard.svelte';

	export let saveSettings: (updated: any, options?: { refreshModels?: boolean }) => Promise<void>;

	const dispatch = createEventDispatcher();
	const i18n: Writable<any> = getContext('i18n');

	let connections: any = null;

	let showAddOpenAIConnectionModal = false;
	let showAddGeminiConnectionModal = false;
	let showAddAnthropicConnectionModal = false;
	let showAddOllamaConnectionModal = false;

	type ConnectionTab = 'openai' | 'gemini' | 'anthropic' | 'ollama';
	let selectedTab: ConnectionTab = 'openai';
	const tabOrder: ConnectionTab[] = ['openai', 'gemini', 'anthropic', 'ollama'];
	const tabMeta: Record<ConnectionTab, { label: string; tabName: string; descKey: string; badgeColor: string; iconColor: string }> = {
		openai:    { label: 'OpenAI API',    tabName: 'OpenAI',    descKey: '管理 OpenAI 兼容 API 接口',    badgeColor: 'bg-emerald-50 dark:bg-emerald-950/30', iconColor: 'text-emerald-500 dark:text-emerald-400' },
		gemini:    { label: 'Gemini API',    tabName: 'Gemini',    descKey: '管理 Google Gemini API 接口',   badgeColor: 'bg-blue-50 dark:bg-blue-950/30',       iconColor: 'text-blue-500 dark:text-blue-400' },
		anthropic: { label: 'Anthropic API', tabName: 'Anthropic', descKey: '管理 Anthropic Claude API 接口', badgeColor: 'bg-violet-50 dark:bg-violet-950/30',   iconColor: 'text-violet-500 dark:text-violet-400' },
		ollama:    { label: 'Ollama API',    tabName: 'Ollama',    descKey: '管理本地 Ollama API 接口',       badgeColor: 'bg-orange-50 dark:bg-orange-950/30',   iconColor: 'text-orange-500 dark:text-orange-400' }
	};

	const ensureOpenAI = () => {
		connections.openai ??= {};
		connections.openai.OPENAI_API_BASE_URLS ??= [];
		connections.openai.OPENAI_API_KEYS ??= [];
		connections.openai.OPENAI_API_CONFIGS ??= {};
		const u = connections.openai.OPENAI_API_BASE_URLS.length;
		const k = connections.openai.OPENAI_API_KEYS.length;
		if (k > u) connections.openai.OPENAI_API_KEYS = connections.openai.OPENAI_API_KEYS.slice(0, u);
		if (k < u) connections.openai.OPENAI_API_KEYS = [...connections.openai.OPENAI_API_KEYS, ...Array(u - k).fill('')];
	};

	const ensureGemini = () => {
		connections.gemini ??= {};
		connections.gemini.GEMINI_API_BASE_URLS ??= [];
		connections.gemini.GEMINI_API_KEYS ??= [];
		connections.gemini.GEMINI_API_CONFIGS ??= {};
		const u = connections.gemini.GEMINI_API_BASE_URLS.length;
		const k = connections.gemini.GEMINI_API_KEYS.length;
		if (k > u) connections.gemini.GEMINI_API_KEYS = connections.gemini.GEMINI_API_KEYS.slice(0, u);
		if (k < u) connections.gemini.GEMINI_API_KEYS = [...connections.gemini.GEMINI_API_KEYS, ...Array(u - k).fill('')];
	};

	const ensureAnthropic = () => {
		connections.anthropic ??= {};
		connections.anthropic.ANTHROPIC_API_BASE_URLS ??= [];
		connections.anthropic.ANTHROPIC_API_KEYS ??= [];
		connections.anthropic.ANTHROPIC_API_CONFIGS ??= {};
		const u = connections.anthropic.ANTHROPIC_API_BASE_URLS.length;
		const k = connections.anthropic.ANTHROPIC_API_KEYS.length;
		if (k > u) connections.anthropic.ANTHROPIC_API_KEYS = connections.anthropic.ANTHROPIC_API_KEYS.slice(0, u);
		if (k < u) connections.anthropic.ANTHROPIC_API_KEYS = [...connections.anthropic.ANTHROPIC_API_KEYS, ...Array(u - k).fill('')];
	};

	const ensureOllama = () => {
		connections.ollama ??= {};
		connections.ollama.OLLAMA_BASE_URLS ??= [];
		connections.ollama.OLLAMA_API_CONFIGS ??= {};
	};

	const ensureAll = () => {
		connections ??= {};
		ensureOpenAI();
		ensureGemini();
		ensureAnthropic();
		ensureOllama();
	};

	const removeIdxFromIndexedConfig = (urls: string[], keys: string[], cfgs: any, idx: number) => {
		const nextUrls = urls.filter((_, i) => i !== idx);
		const nextKeys = keys.filter((_, i) => i !== idx);
		const nextCfgs: any = {};
		nextUrls.forEach((_, newIdx) => { const oldIdx = newIdx < idx ? newIdx : newIdx + 1; nextCfgs[newIdx] = cfgs?.[oldIdx]; });
		return { nextUrls, nextKeys, nextCfgs };
	};

	const removeIdxFromOllamaConfig = (urls: string[], cfgs: any, idx: number) => {
		const nextUrls = urls.filter((_, i) => i !== idx);
		const nextCfgs: any = {};
		nextUrls.forEach((_, newIdx) => { const oldIdx = newIdx < idx ? newIdx : newIdx + 1; nextCfgs[newIdx] = cfgs?.[oldIdx]; });
		return { nextUrls, nextCfgs };
	};

	const getConnectionRenderKey = (url: string, key: string | undefined, config: any) =>
		config ?? `${url}::${key ?? ''}`;

	const getOllamaRenderKey = (url: string, config: any) => config ?? url;

	const updateHandler = async (refreshModels = true) => {
		ensureAll();
		try {
			await saveSettings({ connections }, { refreshModels });
			dispatch('save');
			toast.success($i18n.t('Settings saved successfully!'));
		} catch (error) {
			toast.error($i18n.t('Failed to save connections'));
			throw error;
		}
	};

	const addOpenAIHandler = async (connection) => {
		ensureOpenAI();
		const prev = JSON.parse(JSON.stringify(connections));
		try {
			connections.openai.OPENAI_API_BASE_URLS.push(connection.url);
			connections.openai.OPENAI_API_KEYS.push(connection.key);
			connections.openai.OPENAI_API_CONFIGS[connections.openai.OPENAI_API_BASE_URLS.length - 1] = connection.config;
			await updateHandler(true);
		} catch (error) { connections = prev; throw error; }
	};

	const addGeminiHandler = async (connection) => {
		ensureGemini();
		const prev = JSON.parse(JSON.stringify(connections));
		try {
			connections.gemini.GEMINI_API_BASE_URLS.push(connection.url);
			connections.gemini.GEMINI_API_KEYS.push(connection.key);
			connections.gemini.GEMINI_API_CONFIGS[connections.gemini.GEMINI_API_BASE_URLS.length - 1] = connection.config;
			await updateHandler(true);
		} catch (error) { connections = prev; throw error; }
	};

	const addAnthropicHandler = async (connection) => {
		ensureAnthropic();
		const prev = JSON.parse(JSON.stringify(connections));
		try {
			connections.anthropic.ANTHROPIC_API_BASE_URLS.push(connection.url);
			connections.anthropic.ANTHROPIC_API_KEYS.push(connection.key);
			connections.anthropic.ANTHROPIC_API_CONFIGS[connections.anthropic.ANTHROPIC_API_BASE_URLS.length - 1] = connection.config;
			await updateHandler(true);
		} catch (error) { connections = prev; throw error; }
	};

	const addOllamaHandler = async (connection) => {
		ensureOllama();
		const prev = JSON.parse(JSON.stringify(connections));
		try {
			connections.ollama.OLLAMA_BASE_URLS.push(connection.url);
			connections.ollama.OLLAMA_API_CONFIGS[connections.ollama.OLLAMA_BASE_URLS.length - 1] = { ...connection.config, key: connection.key };
			await updateHandler(true);
		} catch (error) { connections = prev; throw error; }
	};

	onMount(async () => {
		connections = ((($settings as any)?.connections ?? {}) as any) || {};
		connections = JSON.parse(JSON.stringify(connections));
		ensureAll();
	});
</script>

<AddConnectionModal bind:show={showAddOpenAIConnectionModal} onSubmit={addOpenAIHandler} />
<AddConnectionModal gemini bind:show={showAddGeminiConnectionModal} onSubmit={addGeminiHandler} />
<AddConnectionModal anthropic bind:show={showAddAnthropicConnectionModal} onSubmit={addAnthropicHandler} />
<AddConnectionModal ollama bind:show={showAddOllamaConnectionModal} onSubmit={addOllamaHandler} />

<div class="h-full min-h-0 overflow-y-auto pr-1 scrollbar-hidden text-sm">
	{#if connections !== null}
		<div class="max-w-6xl mx-auto space-y-6">
			<!-- ==================== Hero Section ==================== -->
			<section class="glass-section p-5 space-y-5">
				<div class="flex flex-col gap-5">
					<div class="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
						<div class="min-w-0">
							<!-- Breadcrumb -->
							<div class="flex items-center gap-3">
								<div class="inline-flex h-8 items-center gap-2 whitespace-nowrap rounded-full border border-gray-200/80 bg-white/80 px-3.5 text-xs font-medium leading-none text-gray-600 dark:border-gray-700/80 dark:bg-gray-900/70 dark:text-gray-300">
									<span class="leading-none text-gray-400 dark:text-gray-500">{$i18n.t('Settings')}</span>
									<span class="leading-none text-gray-300 dark:text-gray-600">/</span>
									<span class="leading-none text-gray-900 dark:text-white">{$i18n.t('Connections')}</span>
								</div>
								<span class="whitespace-nowrap text-xs text-gray-400 dark:text-gray-500">{$i18n.t('接口按账户独立保存，其他用户无法查看或使用你的密钥')}</span>
							</div>

							<div class="mt-3 flex items-start gap-3">
								<div class="glass-icon-badge {tabMeta[selectedTab].badgeColor}">
									{#if selectedTab === 'openai'}
										<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="size-[18px] {tabMeta[selectedTab].iconColor}"><path d="M21.55 10.004a5.416 5.416 0 00-.478-4.501c-1.217-2.09-3.662-3.166-6.05-2.66A5.59 5.59 0 0010.831 1C8.39.995 6.224 2.546 5.473 4.838A5.553 5.553 0 001.76 7.496a5.487 5.487 0 00.691 6.5 5.416 5.416 0 00.477 4.502c1.217 2.09 3.662 3.165 6.05 2.66A5.586 5.586 0 0013.168 23c2.443.006 4.61-1.546 5.361-3.84a5.553 5.553 0 003.715-2.66 5.488 5.488 0 00-.693-6.497v.001zm-8.381 11.558a4.199 4.199 0 01-2.675-.954c.034-.018.093-.05.132-.074l4.44-2.53a.71.71 0 00.364-.623v-6.176l1.877 1.069c.02.01.033.029.036.05v5.115c-.003 2.274-1.87 4.118-4.174 4.123zM4.192 17.78a4.059 4.059 0 01-.498-2.763c.032.02.09.055.131.078l4.44 2.53c.225.13.504.13.73 0l5.42-3.088v2.138a.068.068 0 01-.027.057L9.9 19.288c-1.999 1.136-4.552.46-5.707-1.51h-.001zM3.023 8.216A4.15 4.15 0 015.198 6.41l-.002.151v5.06a.711.711 0 00.364.624l5.42 3.087-1.876 1.07a.067.067 0 01-.063.005l-4.489-2.559c-1.995-1.14-2.679-3.658-1.53-5.63h.001zm15.417 3.54l-5.42-3.088L14.896 7.6a.067.067 0 01.063-.006l4.489 2.557c1.998 1.14 2.683 3.662 1.529 5.633a4.163 4.163 0 01-2.174 1.807V12.38a.71.71 0 00-.363-.623zm1.867-2.773a6.04 6.04 0 00-.132-.078l-4.44-2.53a.731.731 0 00-.729 0l-5.42 3.088V7.325a.068.068 0 01.027-.057L14.1 4.713c2-1.137 4.555-.46 5.707 1.513.487.833.664 1.809.499 2.757h.001zm-11.741 3.81l-1.877-1.068a.065.065 0 01-.036-.051V6.559c.001-2.277 1.873-4.122 4.181-4.12.976 0 1.92.338 2.671.954-.034.018-.092.05-.131.073l-4.44 2.53a.71.71 0 00-.365.623l-.003 6.173v.002zm1.02-2.168L12 9.25l2.414 1.375v2.75L12 14.75l-2.415-1.375v-2.75z"/></svg>
									{:else if selectedTab === 'gemini'}
										<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="size-[18px] {tabMeta[selectedTab].iconColor}"><path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 01-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z"/><path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/><path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l3.66-2.84z"/><path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/></svg>
									{:else if selectedTab === 'anthropic'}
										<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="currentColor" class="size-[18px] {tabMeta[selectedTab].iconColor}"><path d="m3.127 10.604 3.135-1.76.053-.153-.053-.085H6.11l-.525-.032-1.791-.048-1.554-.065-1.505-.081L0 7.832l.036-.234.32-.214.455.04 1.009.069 1.513.105 1.097.064 1.626.17h.259l.036-.105-.089-.065-.068-.064-1.566-1.062-1.695-1.121-.887-.646-.48-.327-.243-.306-.104-.67.435-.48.585.04.15.04.593.456 1.267.981 1.654 1.218.242.202.097-.068.012-.049-.109-.181-.9-1.626-.96-1.655-.428-.686-.113-.411a2 2 0 0 1-.068-.484l.496-.674L4.446 0l.662.089.279.242.411.94.666 1.48 1.033 2.014.302.597.162.553.06.17h.105v-.097l.085-1.134.157-1.392.154-1.792.052-.504.25-.605.497-.327.387.186.319.456-.045.294-.19 1.23-.37 1.93-.243 1.29h.142l.161-.16.654-.868 1.097-1.372.484-.545.565-.601.363-.287h.686l.505.751-.226.775-.707.895-.585.759-.839 1.13-.524.904.048.072.125-.012 1.897-.403 1.024-.186 1.223-.21.553.258.06.263-.218.536-1.307.323-1.533.307-2.284.54-.028.02.032.04 1.029.098.44.024h1.077l2.005.15.525.346.315.424-.053.323-.807.411-3.631-.863-.872-.218h-.12v.073l.726.71 1.331 1.202 1.667 1.55.084.383-.214.302-.226-.032-1.464-1.101-.565-.497-1.28-1.077h-.084v.113l.295.432 1.557 2.34.08.718-.112.234-.404.141-.444-.08-.911-1.28-.94-1.44-.759-1.291-.093.053-.448 4.821-.21.246-.484.186-.403-.307-.214-.496.214-.98.258-1.28.21-1.016.19-1.263.112-.42-.008-.028-.092.012-.953 1.307-1.448 1.957-1.146 1.227-.274.109-.477-.247.045-.44.266-.39 1.586-2.018.956-1.25.617-.723-.004-.105h-.036l-4.212 2.736-.75.096-.324-.302.04-.496.154-.162 1.267-.871z"/></svg>
									{:else}
										<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="size-[18px] {tabMeta[selectedTab].iconColor}"><path d="M16.361 10.26a.894.894 0 0 0-.558.47l-.072.148.001.207c0 .193.004.217.059.353.076.193.152.312.291.448.24.238.51.3.872.205a.86.86 0 0 0 .517-.436.752.752 0 0 0 .08-.498c-.064-.453-.33-.782-.724-.897a1.06 1.06 0 0 0-.466 0zm-9.203.005c-.305.096-.533.32-.65.639a1.187 1.187 0 0 0-.06.52c.057.309.31.59.598.667.362.095.632.033.872-.205.14-.136.215-.255.291-.448.055-.136.059-.16.059-.353l.001-.207-.072-.148a.894.894 0 0 0-.565-.472 1.02 1.02 0 0 0-.474.007Zm4.184 2c-.131.071-.223.25-.195.383.031.143.157.288.353.407.105.063.112.072.117.136.004.038-.01.146-.029.243-.02.094-.036.194-.036.222.002.074.07.195.143.253.064.052.076.054.255.059.164.005.198.001.264-.03.169-.082.212-.234.15-.525-.052-.243-.042-.28.087-.355.137-.08.281-.219.324-.314a.365.365 0 0 0-.175-.48.394.394 0 0 0-.181-.033c-.126 0-.207.03-.355.124l-.085.053-.053-.032c-.219-.13-.259-.145-.391-.143a.396.396 0 0 0-.193.032zm.39-2.195c-.373.036-.475.05-.654.086-.291.06-.68.195-.951.328-.94.46-1.589 1.226-1.787 2.114-.04.176-.045.234-.045.53 0 .294.005.357.043.524.264 1.16 1.332 2.017 2.714 2.173.3.033 1.596.033 1.896 0 1.11-.125 2.064-.727 2.493-1.571.114-.226.169-.372.22-.602.039-.167.044-.23.044-.523 0-.297-.005-.355-.045-.531-.288-1.29-1.539-2.304-3.072-2.497a6.873 6.873 0 0 0-.855-.031zm.645.937a3.283 3.283 0 0 1 1.44.514c.223.148.537.458.671.662.166.251.26.508.303.82.02.143.01.251-.043.482-.08.345-.332.705-.672.957a3.115 3.115 0 0 1-.689.348c-.382.122-.632.144-1.525.138-.582-.006-.686-.01-.853-.042-.57-.107-1.022-.334-1.35-.68-.264-.28-.385-.535-.45-.946-.03-.192.025-.509.137-.776.136-.326.488-.73.836-.963.403-.269.934-.46 1.422-.512.187-.02.586-.02.773-.002zm-5.503-11a1.653 1.653 0 0 0-.683.298C5.617.74 5.173 1.666 4.985 2.819c-.07.436-.119 1.04-.119 1.503 0 .544.064 1.24.155 1.721.02.107.031.202.023.208a8.12 8.12 0 0 1-.187.152 5.324 5.324 0 0 0-.949 1.02 5.49 5.49 0 0 0-.94 2.339 6.625 6.625 0 0 0-.023 1.357c.091.78.325 1.438.727 2.04l.13.195-.037.064c-.269.452-.498 1.105-.605 1.732-.084.496-.095.629-.095 1.294 0 .67.009.803.088 1.266.095.555.288 1.143.503 1.534.071.128.243.393.264.407.007.003-.014.067-.046.141a7.405 7.405 0 0 0-.548 1.873c-.062.417-.071.552-.071.991 0 .56.031.832.148 1.279L3.42 24h1.478l-.05-.091c-.297-.552-.325-1.575-.068-2.597.117-.472.25-.819.498-1.296l.148-.29v-.177c0-.165-.003-.184-.057-.293a.915.915 0 0 0-.194-.25 1.74 1.74 0 0 1-.385-.543c-.424-.92-.506-2.286-.208-3.451.124-.486.329-.918.544-1.154a.787.787 0 0 0 .223-.531c0-.195-.07-.355-.224-.522a3.136 3.136 0 0 1-.817-1.729c-.14-.96.114-2.005.69-2.834.563-.814 1.353-1.336 2.237-1.475.199-.033.57-.028.776.01.226.04.367.028.512-.041.179-.085.268-.19.374-.431.093-.215.165-.333.36-.576.234-.29.46-.489.822-.729.413-.27.884-.467 1.352-.561.17-.035.25-.04.569-.04.319 0 .398.005.569.04a4.07 4.07 0 0 1 1.914.997c.117.109.398.457.488.602.034.057.095.177.132.267.105.241.195.346.374.43.14.068.286.082.503.045.343-.058.607-.053.943.016 1.144.23 2.14 1.173 2.581 2.437.385 1.108.276 2.267-.296 3.153-.097.15-.193.27-.333.419-.301.322-.301.722-.001 1.053.493.539.801 1.866.708 3.036-.062.772-.26 1.463-.533 1.854a2.096 2.096 0 0 1-.224.258.916.916 0 0 0-.194.25c-.054.109-.057.128-.057.293v.178l.148.29c.248.476.38.823.498 1.295.253 1.008.231 2.01-.059 2.581a.845.845 0 0 0-.044.098c0 .006.329.009.732.009h.73l.02-.074.036-.134c.019-.076.057-.3.088-.516.029-.217.029-1.016 0-1.258-.11-.875-.295-1.57-.597-2.226-.032-.074-.053-.138-.046-.141.008-.005.057-.074.108-.152.376-.569.607-1.284.724-2.228.031-.26.031-1.378 0-1.628-.083-.645-.182-1.082-.348-1.525a6.083 6.083 0 0 0-.329-.7l-.038-.064.131-.194c.402-.604.636-1.262.727-2.04a6.625 6.625 0 0 0-.024-1.358 5.512 5.512 0 0 0-.939-2.339 5.325 5.325 0 0 0-.95-1.02 8.097 8.097 0 0 1-.186-.152.692.692 0 0 1 .023-.208c.208-1.087.201-2.443-.017-3.503-.19-.924-.535-1.658-.98-2.082-.354-.338-.716-.482-1.15-.455-.996.059-1.8 1.205-2.116 3.01a6.805 6.805 0 0 0-.097.726c0 .036-.007.066-.015.066a.96.96 0 0 1-.149-.078A4.857 4.857 0 0 0 12 3.03c-.832 0-1.687.243-2.456.698a.958.958 0 0 1-.148.078c-.008 0-.015-.03-.015-.066a6.71 6.71 0 0 0-.097-.725C8.997 1.392 8.337.319 7.46.048a2.096 2.096 0 0 0-.585-.041Zm.293 1.402c.248.197.523.759.682 1.388.03.113.06.244.069.292.007.047.026.152.041.233.067.365.098.76.102 1.24l.002.475-.12.175-.118.178h-.278c-.324 0-.646.041-.954.124l-.238.06c-.033.007-.038-.003-.057-.144a8.438 8.438 0 0 1 .016-2.323c.124-.788.413-1.501.696-1.711.067-.05.079-.049.157.013zm9.825-.012c.17.126.358.46.498.888.28.854.36 2.028.212 3.145-.019.14-.024.151-.057.144l-.238-.06a3.693 3.693 0 0 0-.954-.124h-.278l-.119-.178-.119-.175.002-.474c.004-.669.066-1.19.214-1.772.157-.623.434-1.185.68-1.382.078-.062.09-.063.159-.012z.295.306.544.744.734 1.263.191.522.315 1.1.362 1.68a5.054 5.054 0 012.049-.636l.051-.004c.87-.07 1.73.087 2.48.474.101.053.2.11.297.17.05-.569.172-1.134.36-1.644.19-.52.439-.957.733-1.264a1.67 1.67 0 01.589-.41c.257-.1.53-.118.796-.042.401.114.745.368 1.016.737.248.337.434.769.561 1.287.23.934.27 2.163.115 3.645l.053.04.026.019c.757.576 1.284 1.397 1.563 2.35.435 1.487.216 3.155-.534 4.088l-.018.021.002.003c.417.762.67 1.567.724 2.4l.002.03c.064 1.065-.2 2.137-.814 3.19l-.007.01.01.024c.472 1.157.62 2.322.438 3.486l-.006.039a.651.651 0 01-.747.536.648.648 0 01-.54-.742c.167-1.033.01-2.069-.48-3.123a.643.643 0 01.04-.617l.004-.006c.604-.924.854-1.83.8-2.72-.046-.779-.325-1.544-.8-2.273a.644.644 0 01.18-.886l.009-.006c.243-.159.467-.565.58-1.12a4.229 4.229 0 00-.095-1.974c-.205-.7-.58-1.284-1.105-1.683-.595-.454-1.383-.673-2.38-.61a.653.653 0 01-.632-.371c-.314-.665-.772-1.141-1.343-1.436a3.288 3.288 0 00-1.772-.332c-1.245.099-2.343.801-2.67 1.686a.652.652 0 01-.61.425c-1.067.002-1.893.252-2.497.703-.522.39-.878.935-1.066 1.588a4.07 4.07 0 00-.068 1.886c.112.558.331 1.02.582 1.269l.008.007c.212.207.257.53.109.785-.36.622-.629 1.549-.673 2.44-.05 1.018.186 1.902.719 2.536l.016.019a.643.643 0 01.095.69c-.576 1.236-.753 2.252-.562 3.052a.652.652 0 01-1.269.298c-.243-1.018-.078-2.184.473-3.498l.014-.035-.008-.012a4.339 4.339 0 01-.598-1.309l-.005-.019a5.764 5.764 0 01-.177-1.785c.044-.91.278-1.842.622-2.59l.012-.026-.002-.002c-.293-.418-.51-.953-.63-1.545l-.005-.024a5.352 5.352 0 01.093-2.49c.262-.915.777-1.701 1.536-2.269.06-.045.123-.09.186-.132-.159-1.493-.119-2.73.112-3.67.127-.518.314-.95.562-1.287.27-.368.614-.622 1.015-.737.266-.076.54-.059.797.042zm4.116 9.09c.936 0 1.8.313 2.446.855.63.527 1.005 1.235 1.005 1.94 0 .888-.406 1.58-1.133 2.022-.62.375-1.451.557-2.403.557-1.009 0-1.871-.259-2.493-.734-.617-.47-.963-1.13-.963-1.845 0-.707.398-1.417 1.056-1.946.668-.537 1.55-.849 2.485-.849zm0 .896a3.07 3.07 0 00-1.916.65c-.461.37-.722.835-.722 1.25 0 .428.21.829.61 1.134.455.347 1.124.548 1.943.548.799 0 1.473-.147 1.932-.426.463-.28.7-.686.7-1.257 0-.423-.246-.89-.683-1.256-.484-.405-1.14-.643-1.864-.643zm.662 1.21l.004.004c.12.151.095.37-.056.49l-.292.23v.446a.375.375 0 01-.376.373.375.375 0 01-.376-.373v-.46l-.271-.218a.347.347 0 01-.052-.49.353.353 0 01.494-.051l.215.172.22-.174a.353.353 0 01.49.051zm-5.04-1.919c.478 0 .867.39.867.871a.87.87 0 01-.868.871.87.87 0 01-.867-.87.87.87 0 01.867-.872zm8.706 0c.48 0 .868.39.868.871a.87.87 0 01-.868.871.87.87 0 01-.867-.87.87.87 0 01.867-.872zM7.44 2.3l-.003.002a.659.659 0 00-.285.238l-.005.006c-.138.189-.258.467-.348.832-.17.692-.216 1.631-.124 2.782.43-.128.899-.208 1.404-.237l.01-.001.019-.034c.046-.082.095-.161.148-.239.123-.771.022-1.692-.253-2.444-.134-.364-.297-.65-.453-.813a.628.628 0 00-.107-.09L7.44 2.3zm9.174.04l-.002.001a.628.628 0 00-.107.09c-.156.163-.32.45-.453.814-.29.794-.387 1.776-.23 2.572l.058.097.008.014h.03a5.184 5.184 0 011.466.212c.086-1.124.038-2.043-.128-2.722-.09-.365-.21-.643-.349-.832l-.004-.006a.659.659 0 00-.285-.239h-.004z"/></svg>
									{/if}
								</div>
								<div class="min-w-0">
									<div class="text-base font-semibold text-gray-800 dark:text-gray-100">{$i18n.t(tabMeta[selectedTab].label)}</div>
									<p class="mt-1 text-xs text-gray-400 dark:text-gray-500">{$i18n.t(tabMeta[selectedTab].descKey)}</p>
								</div>
							</div>
						</div>

						<!-- Tab pill bar -->
						<div class="grid w-full grid-cols-2 rounded-2xl bg-gray-100 p-1 dark:bg-gray-850 md:inline-flex md:w-fit md:flex-col lg:mt-11 lg:flex-row">
							{#each tabOrder as tab}
								<button type="button" class={`flex min-w-0 items-center gap-2 rounded-xl px-4 py-2 text-sm font-medium transition-all ${selectedTab === tab ? 'bg-white text-gray-900 shadow-sm dark:bg-gray-800 dark:text-white' : 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'}`} on:click={() => { selectedTab = tab; }}>
									{#if tab === 'openai'}
										<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="size-4"><path d="M21.55 10.004a5.416 5.416 0 00-.478-4.501c-1.217-2.09-3.662-3.166-6.05-2.66A5.59 5.59 0 0010.831 1C8.39.995 6.224 2.546 5.473 4.838A5.553 5.553 0 001.76 7.496a5.487 5.487 0 00.691 6.5 5.416 5.416 0 00.477 4.502c1.217 2.09 3.662 3.165 6.05 2.66A5.586 5.586 0 0013.168 23c2.443.006 4.61-1.546 5.361-3.84a5.553 5.553 0 003.715-2.66 5.488 5.488 0 00-.693-6.497v.001zm-8.381 11.558a4.199 4.199 0 01-2.675-.954c.034-.018.093-.05.132-.074l4.44-2.53a.71.71 0 00.364-.623v-6.176l1.877 1.069c.02.01.033.029.036.05v5.115c-.003 2.274-1.87 4.118-4.174 4.123zM4.192 17.78a4.059 4.059 0 01-.498-2.763c.032.02.09.055.131.078l4.44 2.53c.225.13.504.13.73 0l5.42-3.088v2.138a.068.068 0 01-.027.057L9.9 19.288c-1.999 1.136-4.552.46-5.707-1.51h-.001zM3.023 8.216A4.15 4.15 0 015.198 6.41l-.002.151v5.06a.711.711 0 00.364.624l5.42 3.087-1.876 1.07a.067.067 0 01-.063.005l-4.489-2.559c-1.995-1.14-2.679-3.658-1.53-5.63h.001zm15.417 3.54l-5.42-3.088L14.896 7.6a.067.067 0 01.063-.006l4.489 2.557c1.998 1.14 2.683 3.662 1.529 5.633a4.163 4.163 0 01-2.174 1.807V12.38a.71.71 0 00-.363-.623zm1.867-2.773a6.04 6.04 0 00-.132-.078l-4.44-2.53a.731.731 0 00-.729 0l-5.42 3.088V7.325a.068.068 0 01.027-.057L14.1 4.713c2-1.137 4.555-.46 5.707 1.513.487.833.664 1.809.499 2.757h.001zm-11.741 3.81l-1.877-1.068a.065.065 0 01-.036-.051V6.559c.001-2.277 1.873-4.122 4.181-4.12.976 0 1.92.338 2.671.954-.034.018-.092.05-.131.073l-4.44 2.53a.71.71 0 00-.365.623l-.003 6.173v.002zm1.02-2.168L12 9.25l2.414 1.375v2.75L12 14.75l-2.415-1.375v-2.75z"/></svg>
									{:else if tab === 'gemini'}
										<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="size-4"><path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 01-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z"/><path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/><path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l3.66-2.84z"/><path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/></svg>
									{:else if tab === 'anthropic'}
										<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="currentColor" class="size-4"><path d="m3.127 10.604 3.135-1.76.053-.153-.053-.085H6.11l-.525-.032-1.791-.048-1.554-.065-1.505-.081L0 7.832l.036-.234.32-.214.455.04 1.009.069 1.513.105 1.097.064 1.626.17h.259l.036-.105-.089-.065-.068-.064-1.566-1.062-1.695-1.121-.887-.646-.48-.327-.243-.306-.104-.67.435-.48.585.04.15.04.593.456 1.267.981 1.654 1.218.242.202.097-.068.012-.049-.109-.181-.9-1.626-.96-1.655-.428-.686-.113-.411a2 2 0 0 1-.068-.484l.496-.674L4.446 0l.662.089.279.242.411.94.666 1.48 1.033 2.014.302.597.162.553.06.17h.105v-.097l.085-1.134.157-1.392.154-1.792.052-.504.25-.605.497-.327.387.186.319.456-.045.294-.19 1.23-.37 1.93-.243 1.29h.142l.161-.16.654-.868 1.097-1.372.484-.545.565-.601.363-.287h.686l.505.751-.226.775-.707.895-.585.759-.839 1.13-.524.904.048.072.125-.012 1.897-.403 1.024-.186 1.223-.21.553.258.06.263-.218.536-1.307.323-1.533.307-2.284.54-.028.02.032.04 1.029.098.44.024h1.077l2.005.15.525.346.315.424-.053.323-.807.411-3.631-.863-.872-.218h-.12v.073l.726.71 1.331 1.202 1.667 1.55.084.383-.214.302-.226-.032-1.464-1.101-.565-.497-1.28-1.077h-.084v.113l.295.432 1.557 2.34.08.718-.112.234-.404.141-.444-.08-.911-1.28-.94-1.44-.759-1.291-.093.053-.448 4.821-.21.246-.484.186-.403-.307-.214-.496.214-.98.258-1.28.21-1.016.19-1.263.112-.42-.008-.028-.092.012-.953 1.307-1.448 1.957-1.146 1.227-.274.109-.477-.247.045-.44.266-.39 1.586-2.018.956-1.25.617-.723-.004-.105h-.036l-4.212 2.736-.75.096-.324-.302.04-.496.154-.162 1.267-.871z"/></svg>
									{:else}
										<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="size-4"><path d="M16.361 10.26a.894.894 0 0 0-.558.47l-.072.148.001.207c0 .193.004.217.059.353.076.193.152.312.291.448.24.238.51.3.872.205a.86.86 0 0 0 .517-.436.752.752 0 0 0 .08-.498c-.064-.453-.33-.782-.724-.897a1.06 1.06 0 0 0-.466 0zm-9.203.005c-.305.096-.533.32-.65.639a1.187 1.187 0 0 0-.06.52c.057.309.31.59.598.667.362.095.632.033.872-.205.14-.136.215-.255.291-.448.055-.136.059-.16.059-.353l.001-.207-.072-.148a.894.894 0 0 0-.565-.472 1.02 1.02 0 0 0-.474.007Zm4.184 2c-.131.071-.223.25-.195.383.031.143.157.288.353.407.105.063.112.072.117.136.004.038-.01.146-.029.243-.02.094-.036.194-.036.222.002.074.07.195.143.253.064.052.076.054.255.059.164.005.198.001.264-.03.169-.082.212-.234.15-.525-.052-.243-.042-.28.087-.355.137-.08.281-.219.324-.314a.365.365 0 0 0-.175-.48.394.394 0 0 0-.181-.033c-.126 0-.207.03-.355.124l-.085.053-.053-.032c-.219-.13-.259-.145-.391-.143a.396.396 0 0 0-.193.032zm.39-2.195c-.373.036-.475.05-.654.086-.291.06-.68.195-.951.328-.94.46-1.589 1.226-1.787 2.114-.04.176-.045.234-.045.53 0 .294.005.357.043.524.264 1.16 1.332 2.017 2.714 2.173.3.033 1.596.033 1.896 0 1.11-.125 2.064-.727 2.493-1.571.114-.226.169-.372.22-.602.039-.167.044-.23.044-.523 0-.297-.005-.355-.045-.531-.288-1.29-1.539-2.304-3.072-2.497a6.873 6.873 0 0 0-.855-.031zm.645.937a3.283 3.283 0 0 1 1.44.514c.223.148.537.458.671.662.166.251.26.508.303.82.02.143.01.251-.043.482-.08.345-.332.705-.672.957a3.115 3.115 0 0 1-.689.348c-.382.122-.632.144-1.525.138-.582-.006-.686-.01-.853-.042-.57-.107-1.022-.334-1.35-.68-.264-.28-.385-.535-.45-.946-.03-.192.025-.509.137-.776.136-.326.488-.73.836-.963.403-.269.934-.46 1.422-.512.187-.02.586-.02.773-.002zm-5.503-11a1.653 1.653 0 0 0-.683.298C5.617.74 5.173 1.666 4.985 2.819c-.07.436-.119 1.04-.119 1.503 0 .544.064 1.24.155 1.721.02.107.031.202.023.208a8.12 8.12 0 0 1-.187.152 5.324 5.324 0 0 0-.949 1.02 5.49 5.49 0 0 0-.94 2.339 6.625 6.625 0 0 0-.023 1.357c.091.78.325 1.438.727 2.04l.13.195-.037.064c-.269.452-.498 1.105-.605 1.732-.084.496-.095.629-.095 1.294 0 .67.009.803.088 1.266.095.555.288 1.143.503 1.534.071.128.243.393.264.407.007.003-.014.067-.046.141a7.405 7.405 0 0 0-.548 1.873c-.062.417-.071.552-.071.991 0 .56.031.832.148 1.279L3.42 24h1.478l-.05-.091c-.297-.552-.325-1.575-.068-2.597.117-.472.25-.819.498-1.296l.148-.29v-.177c0-.165-.003-.184-.057-.293a.915.915 0 0 0-.194-.25 1.74 1.74 0 0 1-.385-.543c-.424-.92-.506-2.286-.208-3.451.124-.486.329-.918.544-1.154a.787.787 0 0 0 .223-.531c0-.195-.07-.355-.224-.522a3.136 3.136 0 0 1-.817-1.729c-.14-.96.114-2.005.69-2.834.563-.814 1.353-1.336 2.237-1.475.199-.033.57-.028.776.01.226.04.367.028.512-.041.179-.085.268-.19.374-.431.093-.215.165-.333.36-.576.234-.29.46-.489.822-.729.413-.27.884-.467 1.352-.561.17-.035.25-.04.569-.04.319 0 .398.005.569.04a4.07 4.07 0 0 1 1.914.997c.117.109.398.457.488.602.034.057.095.177.132.267.105.241.195.346.374.43.14.068.286.082.503.045.343-.058.607-.053.943.016 1.144.23 2.14 1.173 2.581 2.437.385 1.108.276 2.267-.296 3.153-.097.15-.193.27-.333.419-.301.322-.301.722-.001 1.053.493.539.801 1.866.708 3.036-.062.772-.26 1.463-.533 1.854a2.096 2.096 0 0 1-.224.258.916.916 0 0 0-.194.25c-.054.109-.057.128-.057.293v.178l.148.29c.248.476.38.823.498 1.295.253 1.008.231 2.01-.059 2.581a.845.845 0 0 0-.044.098c0 .006.329.009.732.009h.73l.02-.074.036-.134c.019-.076.057-.3.088-.516.029-.217.029-1.016 0-1.258-.11-.875-.295-1.57-.597-2.226-.032-.074-.053-.138-.046-.141.008-.005.057-.074.108-.152.376-.569.607-1.284.724-2.228.031-.26.031-1.378 0-1.628-.083-.645-.182-1.082-.348-1.525a6.083 6.083 0 0 0-.329-.7l-.038-.064.131-.194c.402-.604.636-1.262.727-2.04a6.625 6.625 0 0 0-.024-1.358 5.512 5.512 0 0 0-.939-2.339 5.325 5.325 0 0 0-.95-1.02 8.097 8.097 0 0 1-.186-.152.692.692 0 0 1 .023-.208c.208-1.087.201-2.443-.017-3.503-.19-.924-.535-1.658-.98-2.082-.354-.338-.716-.482-1.15-.455-.996.059-1.8 1.205-2.116 3.01a6.805 6.805 0 0 0-.097.726c0 .036-.007.066-.015.066a.96.96 0 0 1-.149-.078A4.857 4.857 0 0 0 12 3.03c-.832 0-1.687.243-2.456.698a.958.958 0 0 1-.148.078c-.008 0-.015-.03-.015-.066a6.71 6.71 0 0 0-.097-.725C8.997 1.392 8.337.319 7.46.048a2.096 2.096 0 0 0-.585-.041Zm.293 1.402c.248.197.523.759.682 1.388.03.113.06.244.069.292.007.047.026.152.041.233.067.365.098.76.102 1.24l.002.475-.12.175-.118.178h-.278c-.324 0-.646.041-.954.124l-.238.06c-.033.007-.038-.003-.057-.144a8.438 8.438 0 0 1 .016-2.323c.124-.788.413-1.501.696-1.711.067-.05.079-.049.157.013zm9.825-.012c.17.126.358.46.498.888.28.854.36 2.028.212 3.145-.019.14-.024.151-.057.144l-.238-.06a3.693 3.693 0 0 0-.954-.124h-.278l-.119-.178-.119-.175.002-.474c.004-.669.066-1.19.214-1.772.157-.623.434-1.185.68-1.382.078-.062.09-.063.159-.012z.295.306.544.744.734 1.263.191.522.315 1.1.362 1.68a5.054 5.054 0 012.049-.636l.051-.004c.87-.07 1.73.087 2.48.474.101.053.2.11.297.17.05-.569.172-1.134.36-1.644.19-.52.439-.957.733-1.264a1.67 1.67 0 01.589-.41c.257-.1.53-.118.796-.042.401.114.745.368 1.016.737.248.337.434.769.561 1.287.23.934.27 2.163.115 3.645l.053.04.026.019c.757.576 1.284 1.397 1.563 2.35.435 1.487.216 3.155-.534 4.088l-.018.021.002.003c.417.762.67 1.567.724 2.4l.002.03c.064 1.065-.2 2.137-.814 3.19l-.007.01.01.024c.472 1.157.62 2.322.438 3.486l-.006.039a.651.651 0 01-.747.536.648.648 0 01-.54-.742c.167-1.033.01-2.069-.48-3.123a.643.643 0 01.04-.617l.004-.006c.604-.924.854-1.83.8-2.72-.046-.779-.325-1.544-.8-2.273a.644.644 0 01.18-.886l.009-.006c.243-.159.467-.565.58-1.12a4.229 4.229 0 00-.095-1.974c-.205-.7-.58-1.284-1.105-1.683-.595-.454-1.383-.673-2.38-.61a.653.653 0 01-.632-.371c-.314-.665-.772-1.141-1.343-1.436a3.288 3.288 0 00-1.772-.332c-1.245.099-2.343.801-2.67 1.686a.652.652 0 01-.61.425c-1.067.002-1.893.252-2.497.703-.522.39-.878.935-1.066 1.588a4.07 4.07 0 00-.068 1.886c.112.558.331 1.02.582 1.269l.008.007c.212.207.257.53.109.785-.36.622-.629 1.549-.673 2.44-.05 1.018.186 1.902.719 2.536l.016.019a.643.643 0 01.095.69c-.576 1.236-.753 2.252-.562 3.052a.652.652 0 01-1.269.298c-.243-1.018-.078-2.184.473-3.498l.014-.035-.008-.012a4.339 4.339 0 01-.598-1.309l-.005-.019a5.764 5.764 0 01-.177-1.785c.044-.91.278-1.842.622-2.59l.012-.026-.002-.002c-.293-.418-.51-.953-.63-1.545l-.005-.024a5.352 5.352 0 01.093-2.49c.262-.915.777-1.701 1.536-2.269.06-.045.123-.09.186-.132-.159-1.493-.119-2.73.112-3.67.127-.518.314-.95.562-1.287.27-.368.614-.622 1.015-.737.266-.076.54-.059.797.042z"/></svg>
									{/if}
									<span class="min-w-0 truncate">{tabMeta[tab].tabName}</span>
								</button>
							{/each}
						</div>
					</div>

				</div>
			</section>

			<!-- ==================== Tab Content ==================== -->
			{#if selectedTab === 'openai'}
				<section class="glass-section p-5 space-y-3">
					<div class="grid grid-cols-1 md:grid-cols-2 gap-2">
						{#each connections?.openai?.OPENAI_API_BASE_URLS ?? [] as url, idx (getConnectionRenderKey(url, connections.openai.OPENAI_API_KEYS[idx], connections.openai.OPENAI_API_CONFIGS[idx]))}
							<OpenAIConnection
								bind:url
								bind:key={connections.openai.OPENAI_API_KEYS[idx]}
								bind:config={connections.openai.OPENAI_API_CONFIGS[idx]}
								onSubmit={async () => { await updateHandler(true); }}
								onDelete={() => {
									const { nextUrls, nextKeys, nextCfgs } = removeIdxFromIndexedConfig(connections.openai.OPENAI_API_BASE_URLS, connections.openai.OPENAI_API_KEYS, connections.openai.OPENAI_API_CONFIGS, idx);
									connections.openai.OPENAI_API_BASE_URLS = nextUrls;
									connections.openai.OPENAI_API_KEYS = nextKeys;
									connections.openai.OPENAI_API_CONFIGS = nextCfgs;
									updateHandler(true).catch(() => {});
								}}
							/>
						{/each}
						<button type="button" class="w-full min-h-[62px] bg-white dark:bg-gray-900 rounded-lg px-4 py-3 border border-dashed border-gray-200 dark:border-gray-800 hover:border-gray-300 dark:hover:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800/50 transition flex items-center justify-center gap-2 text-gray-500 dark:text-gray-400 focus:outline-none focus:ring-2 focus:ring-gray-200 dark:focus:ring-gray-700" aria-label={$i18n.t('Add Connection')} on:click={() => { showAddOpenAIConnectionModal = true; }}>
							<div class="w-7 h-7 rounded-full bg-gray-100 dark:bg-gray-800 flex items-center justify-center text-gray-700 dark:text-gray-200"><Plus className="size-3" /></div>
							<div class="text-xs font-medium text-gray-600 dark:text-gray-300 whitespace-nowrap">{$i18n.t('Click to add a new connection')}</div>
						</button>
					</div>
				</section>
			{:else if selectedTab === 'gemini'}
				<section class="glass-section p-5 space-y-3">
					<div class="grid grid-cols-1 md:grid-cols-2 gap-2">
						{#each connections?.gemini?.GEMINI_API_BASE_URLS ?? [] as url, idx (getConnectionRenderKey(url, connections.gemini.GEMINI_API_KEYS[idx], connections.gemini.GEMINI_API_CONFIGS[idx]))}
							<GeminiConnection
								bind:url
								bind:key={connections.gemini.GEMINI_API_KEYS[idx]}
								bind:config={connections.gemini.GEMINI_API_CONFIGS[idx]}
								onSubmit={async () => { await updateHandler(true); }}
								onDelete={() => {
									const { nextUrls, nextKeys, nextCfgs } = removeIdxFromIndexedConfig(connections.gemini.GEMINI_API_BASE_URLS, connections.gemini.GEMINI_API_KEYS, connections.gemini.GEMINI_API_CONFIGS, idx);
									connections.gemini.GEMINI_API_BASE_URLS = nextUrls;
									connections.gemini.GEMINI_API_KEYS = nextKeys;
									connections.gemini.GEMINI_API_CONFIGS = nextCfgs;
									updateHandler(true).catch(() => {});
								}}
							/>
						{/each}
						<button type="button" class="w-full min-h-[62px] bg-white dark:bg-gray-900 rounded-lg px-4 py-3 border border-dashed border-gray-200 dark:border-gray-800 hover:border-gray-300 dark:hover:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800/50 transition flex items-center justify-center gap-2 text-gray-500 dark:text-gray-400 focus:outline-none focus:ring-2 focus:ring-gray-200 dark:focus:ring-gray-700" aria-label={$i18n.t('Add Connection')} on:click={() => { showAddGeminiConnectionModal = true; }}>
							<div class="w-7 h-7 rounded-full bg-gray-100 dark:bg-gray-800 flex items-center justify-center text-gray-700 dark:text-gray-200"><Plus className="size-3" /></div>
							<div class="text-xs font-medium text-gray-600 dark:text-gray-300 whitespace-nowrap">{$i18n.t('Click to add a new connection')}</div>
						</button>
					</div>
				</section>
			{:else if selectedTab === 'anthropic'}
				<section class="glass-section p-5 space-y-3">
					<div class="grid grid-cols-1 md:grid-cols-2 gap-2">
						{#each connections?.anthropic?.ANTHROPIC_API_BASE_URLS ?? [] as url, idx (getConnectionRenderKey(url, connections.anthropic.ANTHROPIC_API_KEYS[idx], connections.anthropic.ANTHROPIC_API_CONFIGS[idx]))}
							<AnthropicConnection
								bind:url
								bind:key={connections.anthropic.ANTHROPIC_API_KEYS[idx]}
								bind:config={connections.anthropic.ANTHROPIC_API_CONFIGS[idx]}
								onSubmit={async () => { await updateHandler(true); }}
								onDelete={() => {
									const { nextUrls, nextKeys, nextCfgs } = removeIdxFromIndexedConfig(connections.anthropic.ANTHROPIC_API_BASE_URLS, connections.anthropic.ANTHROPIC_API_KEYS, connections.anthropic.ANTHROPIC_API_CONFIGS, idx);
									connections.anthropic.ANTHROPIC_API_BASE_URLS = nextUrls;
									connections.anthropic.ANTHROPIC_API_KEYS = nextKeys;
									connections.anthropic.ANTHROPIC_API_CONFIGS = nextCfgs;
									updateHandler(true).catch(() => {});
								}}
							/>
						{/each}
						<button type="button" class="w-full min-h-[62px] bg-white dark:bg-gray-900 rounded-lg px-4 py-3 border border-dashed border-gray-200 dark:border-gray-800 hover:border-gray-300 dark:hover:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800/50 transition flex items-center justify-center gap-2 text-gray-500 dark:text-gray-400 focus:outline-none focus:ring-2 focus:ring-gray-200 dark:focus:ring-gray-700" aria-label={$i18n.t('Add Connection')} on:click={() => { showAddAnthropicConnectionModal = true; }}>
							<div class="w-7 h-7 rounded-full bg-gray-100 dark:bg-gray-800 flex items-center justify-center text-gray-700 dark:text-gray-200"><Plus className="size-3" /></div>
							<div class="text-xs font-medium text-gray-600 dark:text-gray-300 whitespace-nowrap">{$i18n.t('Click to add a new connection')}</div>
						</button>
					</div>
				</section>
			{:else}
				<section class="glass-section p-5 space-y-3">
					<div class="grid grid-cols-1 md:grid-cols-2 gap-2">
						{#each connections?.ollama?.OLLAMA_BASE_URLS ?? [] as url, idx (getOllamaRenderKey(url, connections.ollama.OLLAMA_API_CONFIGS[idx]))}
							<OllamaConnectionCard
								bind:url
								{idx}
								bind:config={connections.ollama.OLLAMA_API_CONFIGS[idx]}
								onSubmit={async () => { await updateHandler(true); }}
								onDelete={() => {
									const { nextUrls, nextCfgs } = removeIdxFromOllamaConfig(connections.ollama.OLLAMA_BASE_URLS, connections.ollama.OLLAMA_API_CONFIGS, idx);
									connections.ollama.OLLAMA_BASE_URLS = nextUrls;
									connections.ollama.OLLAMA_API_CONFIGS = nextCfgs;
									updateHandler(true).catch(() => {});
								}}
							/>
						{/each}
						<button type="button" class="w-full min-h-[62px] bg-white dark:bg-gray-900 rounded-lg px-4 py-3 border border-dashed border-gray-200 dark:border-gray-800 hover:border-gray-300 dark:hover:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800/50 transition flex items-center justify-center gap-2 text-gray-500 dark:text-gray-400 focus:outline-none focus:ring-2 focus:ring-gray-200 dark:focus:ring-gray-700" aria-label={$i18n.t('Add Connection')} on:click={() => { showAddOllamaConnectionModal = true; }}>
							<div class="w-7 h-7 rounded-full bg-gray-100 dark:bg-gray-800 flex items-center justify-center text-gray-700 dark:text-gray-200"><Plus className="size-3" /></div>
							<div class="text-xs font-medium text-gray-600 dark:text-gray-300 whitespace-nowrap">{$i18n.t('Click to add a new connection')}</div>
						</button>
					</div>
				</section>
			{/if}
		</div>
	{:else}
		<div class="flex h-[40vh] justify-center">
			<div class="my-auto">
				<Spinner className="size-6" />
			</div>
		</div>
	{/if}
</div>
