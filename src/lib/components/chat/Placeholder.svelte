<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { marked } from 'marked';

	import { onMount, getContext, tick, createEventDispatcher } from 'svelte';
	import { blur, fade } from 'svelte/transition';

	const dispatch = createEventDispatcher();

	import {
		config,
		user,
		models as _models,
		temporaryChatEnabled,
		settings,
		mobile,
		selectedAssistantScene,
		type Model
	} from '$lib/stores';
	import { sanitizeResponseContent, extractCurlyBraceWords } from '$lib/utils';
	import { WEBUI_BASE_URL } from '$lib/constants';
	import agentsData from '$lib/data/agents-zh.json';
	import {
		type ChatAssistantSnapshot,
		FEATURED_ASSISTANT_IDS,
		getFeaturedAssistantIds,
		setFeaturedAssistantIds,
		resetFeaturedAssistantIds,
		MAX_FEATURED_ASSISTANTS,
		toChatAssistantSnapshot
	} from '$lib/utils/chat-assistants';
	import { translateWithDefault } from '$lib/i18n';

	import Suggestions from './Suggestions.svelte';
	import ModelIcon from '$lib/components/common/ModelIcon.svelte';
	import Tooltip from '$lib/components/common/Tooltip.svelte';
	import { getModelChatDisplayName } from '$lib/utils/model-display';
	import { findModelByIdentity } from '$lib/utils/model-identity';
	import type { WebSearchMode, WebSearchModeSource } from '$lib/utils/web-search-mode';
	import EyeSlash from '$lib/components/icons/EyeSlash.svelte';
	import MessageInput from './MessageInput.svelte';
	import AssistantPickerModal from './AssistantPickerModal.svelte';
	import AssistantSceneTitle from './Placeholder/AssistantSceneTitle.svelte';
	import AssistantScenePlaceholder from './Placeholder/AssistantScenePlaceholder.svelte';
	import Pencil from '$lib/components/icons/Pencil.svelte';
	import Check from '$lib/components/icons/Check.svelte';
	import XMark from '$lib/components/icons/XMark.svelte';
	import Plus from '$lib/components/icons/Plus.svelte';
	import ArrowLeft from '$lib/components/icons/ArrowLeft.svelte';
	import ArrowRight from '$lib/components/icons/ArrowRight.svelte';

	const i18n = getContext('i18n');

	export let transparentBackground = false;

	export let createMessagePair: Function;
	export let stopResponse: Function;
	export let onChange: Function = () => {};

	export let autoScroll = false;

	export let atSelectedModel: Model | undefined;
	export let selectedModels: [''];
	export let activeAssistant: ChatAssistantSnapshot | null = null;
	export let onActivateAssistant: ((assistant: ChatAssistantSnapshot) => void) | null = null;
	export let onDeactivateAssistant: (() => void) | null = null;

	export let history;

	export let prompt = '';
	export let files = [];

	export let selectedToolIds = [];
	export let toolSelectionTouched = false;
	export let selectedSkillIds = [];
	export let skillSelectionTouched = false;
	export let imageGenerationEnabled = false;
	export let imageGenerationOptions = {};
	export let codeInterpreterEnabled = false;
	export let webSearchMode: WebSearchMode = 'off';
	export let webSearchModeSource: WebSearchModeSource = 'default';

	export let reasoningEffort: string | null = null;
	export let maxThinkingTokens: number | null = null;

	export let toolServers = [];

	let models = [];
	let editMode = false;
	let showPickerModal = false;
	let featuredIds = [...FEATURED_ASSISTANT_IDS];
	let dragSourceIdx: number | null = null;
	let dropTargetIdx: number | null = null;
	let isMobileSortingMode = false;
	let featuredAssistants: ChatAssistantSnapshot[] = [];
	const tr = (key: string, defaultValue: string, options: Record<string, any> = {}) =>
		translateWithDefault($i18n, key, defaultValue, options);

	$: isMobileSortingMode = $mobile;
	$: featuredAssistants = featuredIds
		.map((id) =>
			toChatAssistantSnapshot(
				agentsData.find((agent) => agent.id === id) as Record<string, unknown> | undefined
			)
		)
		.filter(Boolean) as ChatAssistantSnapshot[];

	const selectSuggestionPrompt = async (p) => {
		let text = p;

		if (p.includes('{{CLIPBOARD}}')) {
			const clipboardText = await navigator.clipboard.readText().catch((err) => {
				toast.error($i18n.t('Failed to read clipboard contents'));
				return '{{CLIPBOARD}}';
			});

			text = p.replaceAll('{{CLIPBOARD}}', clipboardText);

			console.log('Clipboard text:', clipboardText, text);
		}

		prompt = text;

		console.log(prompt);
		await tick();

		const chatInputContainerElement = document.getElementById('chat-input-container');
		const chatInputElement = document.getElementById('chat-input');

		if (chatInputContainerElement) {
			chatInputContainerElement.scrollTop = chatInputContainerElement.scrollHeight;
		}

		await tick();
		if (chatInputElement) {
			chatInputElement.focus();
			chatInputElement.dispatchEvent(new Event('input'));
		}

		await tick();

		if (!($settings?.insertSuggestionPrompt ?? false)) {
			dispatch('submit', text);
		}
	};

	let selectedModelIdx = 0;

	$: if (selectedModels.length > 0) {
		selectedModelIdx = models.length - 1;
	}

	$: models = selectedModels.map((id) => findModelByIdentity($_models, id));

	const persistFeaturedIds = (nextIds: string[]) => {
		featuredIds = nextIds;
		dragSourceIdx = null;
		dropTargetIdx = null;
		setFeaturedAssistantIds(nextIds);
	};

	const removeFeaturedAssistant = (assistantId: string) => {
		persistFeaturedIds(featuredIds.filter((id) => id !== assistantId));
	};

	const moveFeaturedAssistant = (index: number, direction: -1 | 1) => {
		const nextIndex = index + direction;
		if (nextIndex < 0 || nextIndex >= featuredIds.length) {
			return;
		}

		const nextIds = [...featuredIds];
		[nextIds[index], nextIds[nextIndex]] = [nextIds[nextIndex], nextIds[index]];
		persistFeaturedIds(nextIds);
	};

	const handleDragStart = (index: number) => {
		if (isMobileSortingMode) {
			return;
		}

		dragSourceIdx = index;
	};

	const handleDrop = (index: number) => {
		if (
			isMobileSortingMode ||
			dragSourceIdx === null ||
			dragSourceIdx === index ||
			dragSourceIdx < 0 ||
			dragSourceIdx >= featuredIds.length
		) {
			dragSourceIdx = null;
			dropTargetIdx = null;
			return;
		}

		const nextIds = [...featuredIds];
		const [moved] = nextIds.splice(dragSourceIdx, 1);
		nextIds.splice(index, 0, moved);
		persistFeaturedIds(nextIds);
		dragSourceIdx = null;
		dropTargetIdx = null;
	};

	const handleResetFeaturedAssistants = () => {
		resetFeaturedAssistantIds();
		featuredIds = getFeaturedAssistantIds();
	};

	onMount(() => {
		featuredIds = getFeaturedAssistantIds();
	});
</script>

<div class="m-auto w-full max-w-6xl px-4 @2xl:px-20 translate-y-2 py-16 text-center">
	{#if $temporaryChatEnabled}
		<Tooltip
			content={$i18n.t('This chat won’t appear in history and your messages will not be saved.')}
			className="w-full flex justify-center mb-0.5"
			placement="top"
		>
			<div class="flex items-center gap-2 text-gray-500 font-medium text-lg my-2 w-fit">
				<EyeSlash strokeWidth="2.5" className="size-5" />{$i18n.t('Temporary Chat')}
			</div>
		</Tooltip>
	{/if}

	<div class="w-full text-gray-800 dark:text-gray-100 text-center flex items-center font-primary">
		<div class="w-full flex flex-col justify-center items-center">
			{#if $selectedAssistantScene}
				<AssistantSceneTitle
					assistant={$selectedAssistantScene}
					on:clear={() => {
						selectedAssistantScene.set(null);
					}}
				/>
			{:else}
				<!-- Logo/Avatar 区域 - 居中显示，更大尺寸 -->
				<div class="flex justify-center mb-4" in:fade={{ duration: 100 }}>
					<div class="flex -space-x-4">
						{#each models as model, modelIdx}
							<Tooltip
								content={(models[modelIdx]?.info?.meta?.tags ?? [])
									.map((tag) => tag.name.toUpperCase())
									.join(', ')}
								placement="top"
							>
								<button
									on:click={() => {
										selectedModelIdx = modelIdx;
									}}
								>
									<ModelIcon
										src={model?.info?.meta?.profile_image_url ??
											model?.meta?.profile_image_url ??
											`${WEBUI_BASE_URL}/static/favicon.png`}
										className="size-14 @sm:size-16 rounded-2xl border-2 border-white dark:border-gray-800 shadow-lg"
										alt="logo"
									/>
								</button>
							</Tooltip>
						{/each}
					</div>
				</div>

				<!-- 模型名称/问候语 - 字体适中 -->
				<div class="text-xl @sm:text-2xl font-medium line-clamp-1 px-4" in:fade={{ duration: 100 }}>
					{#if models[selectedModelIdx]?.name}
						{getModelChatDisplayName(models[selectedModelIdx])}
					{:else}
						{$i18n.t('Hello, {{name}}', { name: $user?.name })}
					{/if}
				</div>

				<!-- 模型描述 -->
				<div class="flex mt-2 mb-4">
					<div in:fade={{ duration: 100, delay: 50 }}>
						{#if models[selectedModelIdx]?.info?.meta?.description ?? null}
							<Tooltip
								className=" w-fit"
								content={marked.parse(
									sanitizeResponseContent(models[selectedModelIdx]?.info?.meta?.description ?? '')
								)}
								placement="top"
							>
								<div
									class="mt-0.5 px-3 text-sm font-normal text-gray-500 dark:text-gray-400 line-clamp-2 max-w-xl markdown"
								>
									{@html marked.parse(
										sanitizeResponseContent(models[selectedModelIdx]?.info?.meta?.description)
									)}
								</div>
							</Tooltip>

							{#if models[selectedModelIdx]?.info?.meta?.user}
								<div class="mt-0.5 text-sm font-normal text-gray-400 dark:text-gray-500">
									By
									{#if models[selectedModelIdx]?.info?.meta?.user.community}
										<a
											href="https://openwebui.com/m/{models[selectedModelIdx]?.info?.meta?.user
												.username}"
											>{models[selectedModelIdx]?.info?.meta?.user.name
												? models[selectedModelIdx]?.info?.meta?.user.name
												: `@${models[selectedModelIdx]?.info?.meta?.user.username}`}</a
										>
									{:else}
										{models[selectedModelIdx]?.info?.meta?.user.name}
									{/if}
								</div>
							{/if}
						{/if}
					</div>
				</div>
			{/if}

			<div class="mx-auto w-full max-w-4xl pt-2 pb-3 text-base font-normal {atSelectedModel ? 'mt-2' : ''}">
				<MessageInput
					{history}
					{selectedModels}
					{activeAssistant}
					bind:files
					bind:prompt
					bind:autoScroll
					bind:selectedToolIds
					bind:toolSelectionTouched
					bind:selectedSkillIds
					bind:skillSelectionTouched
					bind:imageGenerationEnabled
					bind:imageGenerationOptions
					bind:codeInterpreterEnabled
					bind:webSearchMode
					{webSearchModeSource}
					bind:atSelectedModel
					bind:reasoningEffort
					bind:maxThinkingTokens
					{onChange}
					{onDeactivateAssistant}
					{toolServers}
					{transparentBackground}
					{stopResponse}
					{createMessagePair}
					placeholder={$i18n.t('How can I help you today?')}
					on:upload={(e) => {
						dispatch('upload', e.detail);
					}}
					on:submit={(e) => {
						dispatch('submit', e.detail);
					}}
				/>
			</div>
		</div>
	</div>
	{#if !$selectedAssistantScene && !activeAssistant && onActivateAssistant && ($settings?.showFeaturedAssistantsOnHome ?? true)}
		<div class="mx-auto mt-1 w-full max-w-4xl px-2.5" in:fade={{ duration: 160, delay: 120 }}>
			<div class="rounded-3xl border border-gray-200/60 bg-white/65 p-3 text-left shadow-sm backdrop-blur-xl dark:border-gray-700/30 dark:bg-white/[0.03]">
				<div class="flex items-center justify-between gap-3 px-1">
					<div class="text-xs font-medium text-gray-500 dark:text-gray-400">
						{tr('精选助手', 'Featured Assistants')}
					</div>
					<button
						class="inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-medium text-gray-500 transition hover:bg-gray-100 hover:text-gray-700 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-gray-200"
						on:click={() => {
							editMode = !editMode;
							dragSourceIdx = null;
							dropTargetIdx = null;
						}}
					>
						{#if editMode}
							<Check className="size-3.5" strokeWidth="2.25" />
							<span>{tr('完成', 'Done')}</span>
						{:else}
							<Pencil className="size-3.5" strokeWidth="2.1" />
							<span>{tr('管理', 'Manage')}</span>
						{/if}
					</button>
				</div>
				<div class="mt-2 grid grid-cols-1 gap-2 @md:grid-cols-2 @xl:grid-cols-3">
					{#each featuredAssistants as assistant, index}
						<div
							class="group relative rounded-2xl border bg-gray-50/90 px-3 py-3 text-left transition dark:bg-gray-900/50 {editMode
								? dropTargetIdx === index && !isMobileSortingMode
									? 'border-primary-300 bg-primary-50/60 dark:border-primary-700/60 dark:bg-primary-950/20'
									: 'border-gray-200/70 dark:border-gray-700/60'
								: 'border-transparent hover:border-primary-200/70 hover:bg-primary-50/70 dark:hover:border-primary-700/50 dark:hover:bg-primary-950/20'}"
							draggable={editMode && !isMobileSortingMode}
							on:dragstart={(e) => {
								e.dataTransfer?.setData('text/plain', assistant.id);
								if (e.dataTransfer) {
									e.dataTransfer.effectAllowed = 'move';
								}
								handleDragStart(index);
							}}
							on:dragover|preventDefault={(e) => {
								if (editMode && !isMobileSortingMode) {
									if (e.dataTransfer) {
										e.dataTransfer.dropEffect = 'move';
									}
									dropTargetIdx = index;
								}
							}}
							on:dragleave={() => {
								if (dropTargetIdx === index) {
									dropTargetIdx = null;
								}
							}}
							on:dragend={() => {
								dragSourceIdx = null;
								dropTargetIdx = null;
							}}
							on:drop|preventDefault={() => handleDrop(index)}
						>
							{#if editMode}
								<div class="absolute right-2 top-2 z-10 flex items-center gap-1">
									{#if isMobileSortingMode}
										<button
											class="rounded-full bg-white/90 p-1 text-gray-500 shadow-sm transition hover:bg-white hover:text-gray-700 dark:bg-gray-800/90 dark:hover:bg-gray-800 dark:hover:text-gray-200"
											on:click={() => moveFeaturedAssistant(index, -1)}
											disabled={index === 0}
											aria-label="Move Left"
										>
											<ArrowLeft className="size-3.5" strokeWidth="2.2" />
										</button>
										<button
											class="rounded-full bg-white/90 p-1 text-gray-500 shadow-sm transition hover:bg-white hover:text-gray-700 dark:bg-gray-800/90 dark:hover:bg-gray-800 dark:hover:text-gray-200"
											on:click={() => moveFeaturedAssistant(index, 1)}
											disabled={index === featuredAssistants.length - 1}
											aria-label="Move Right"
										>
											<ArrowRight className="size-3.5" strokeWidth="2.2" />
										</button>
									{/if}
									<button
										class="rounded-full bg-white/90 p-1 text-gray-500 shadow-sm transition hover:bg-white hover:text-red-600 dark:bg-gray-800/90 dark:hover:bg-gray-800 dark:hover:text-red-400"
										on:click={() => removeFeaturedAssistant(assistant.id)}
										aria-label="Remove Assistant"
									>
										<XMark className="size-3.5" strokeWidth="2.4" />
									</button>
								</div>
							{/if}

							<button
								class="flex w-full items-start gap-2.5 text-left"
								on:click={() => {
									if (!editMode) {
										onActivateAssistant(assistant);
									}
								}}
								disabled={editMode}
							>
								<div class="text-lg leading-none">{assistant.emoji}</div>
								<div class="min-w-0 flex-1 pr-8">
									<div class="text-sm font-medium text-gray-800 dark:text-gray-100">
										{assistant.name}
									</div>
									{#if assistant.description}
										<div class="mt-1 line-clamp-2 text-xs leading-5 text-gray-500 dark:text-gray-400">
											{assistant.description}
										</div>
									{/if}
								</div>
							</button>
						</div>
					{/each}

					{#if editMode && featuredIds.length < MAX_FEATURED_ASSISTANTS}
						<button
							class="flex min-h-[104px] items-center justify-center rounded-2xl border border-dashed border-gray-300/80 bg-transparent text-gray-400 transition hover:border-primary-300 hover:text-primary-500 dark:border-gray-700/80 dark:text-gray-500 dark:hover:border-primary-700 dark:hover:text-primary-400"
							on:click={() => {
								showPickerModal = true;
							}}
						>
							<div class="flex flex-col items-center gap-2">
								<div class="rounded-full border border-current p-1">
									<Plus className="size-4" />
								</div>
								<div class="text-xs font-medium">
									{tr('添加助手', 'Add Assistant')}
								</div>
							</div>
						</button>
					{/if}
				</div>

				{#if !editMode && featuredAssistants.length === 0}
					<div class="mt-2 rounded-2xl border border-dashed border-gray-200/80 px-4 py-8 text-center text-sm text-gray-400 dark:border-gray-700/70 dark:text-gray-500">
						{tr(
							'暂无精选助手，点击右上角“管理”即可添加',
							'No featured assistants yet. Click "Manage" in the top-right corner to add one.'
						)}
					</div>
				{/if}

				{#if editMode}
					<div class="mt-3 flex items-center justify-between px-1">
						<div class="text-xs text-gray-400 dark:text-gray-500">
							{tr('最多 {{count}} 个精选助手', 'Up to {{count}} featured assistants', {
								count: MAX_FEATURED_ASSISTANTS
							})}
						</div>
						<button
							class="text-xs font-medium text-gray-500 transition hover:text-primary-600 dark:text-gray-400 dark:hover:text-primary-400"
							on:click={handleResetFeaturedAssistants}
						>
							{tr('恢复默认', 'Restore Defaults')}
						</button>
					</div>
				{/if}
			</div>
		</div>
	{/if}
	{#if $selectedAssistantScene}
		<div
			class="mx-auto mt-4 w-full max-w-4xl px-2.5 font-primary"
			in:fade={{ duration: 200, delay: 200 }}
		>
			<AssistantScenePlaceholder assistant={$selectedAssistantScene} />
		</div>
	{:else}
		<div
			class="mx-auto w-full max-w-4xl px-2.5 font-primary {!activeAssistant && onActivateAssistant && ($settings?.showFeaturedAssistantsOnHome ?? true) ? 'mt-4' : 'mt-2'}"
			in:fade={{ duration: 200, delay: 200 }}
		>
			<div>
				<Suggestions
					suggestionPrompts={atSelectedModel?.info?.meta?.suggestion_prompts ??
						models[selectedModelIdx]?.info?.meta?.suggestion_prompts ??
						$config?.default_prompt_suggestions ??
						[]}
					inputValue={prompt}
					on:select={(e) => {
						selectSuggestionPrompt(e.detail);
					}}
				/>
			</div>
		</div>
	{/if}
</div>

<AssistantPickerModal
	bind:show={showPickerModal}
	excludeIds={featuredIds}
	on:select={(e) => {
		const assistant = e.detail;
		if (!assistant || featuredIds.includes(assistant.id)) {
			return;
		}

		persistFeaturedIds([...featuredIds, assistant.id]);
	}}
/>
