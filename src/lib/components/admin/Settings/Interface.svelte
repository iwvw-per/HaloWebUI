<script lang="ts">
	import { slide } from 'svelte/transition';
	import { quintOut } from 'svelte/easing';

	import { getBackendConfig, getTaskConfig, updateTaskConfig } from '$lib/apis';
	import { config, models } from '$lib/stores';
	import { createEventDispatcher, onDestroy, onMount, getContext, tick } from 'svelte';

	import { revealExpandedSection } from '$lib/utils/expanded-section-scroll';

	import Tooltip from '$lib/components/common/Tooltip.svelte';
	import Switch from '$lib/components/common/Switch.svelte';
	import Textarea from '$lib/components/common/Textarea.svelte';
	import Spinner from '$lib/components/common/Spinner.svelte';
	import ChevronDown from '$lib/components/icons/ChevronDown.svelte';
	import { getModelChatDisplayName } from '$lib/utils/model-display';
	import HaloSelect from '$lib/components/common/HaloSelect.svelte';
	import InlineDirtyActions from './InlineDirtyActions.svelte';
	import { cloneSettingsSnapshot, isSettingsSnapshotEqual } from '$lib/utils/settings-dirty';
	import { getModelSelectionId, resolveModelSelectionId } from '$lib/utils/model-identity';

	import type { Writable } from 'svelte/store';
	const i18n: Writable<any> = getContext('i18n');

	const dispatch = createEventDispatcher();

	// 折叠状态
	let expandedSections = {
		tasks: true
	};
	let sectionEl_tasks: HTMLElement;

	// When embedded inside a larger settings page, avoid "full-height tab" layout.
	export let embedded: boolean = false;

	let rootClass = 'flex flex-col space-y-3 text-sm';
	let bodyClass = 'space-y-3 overflow-y-auto scrollbar-hidden';
	$: {
		rootClass = embedded
			? 'flex flex-col space-y-3 text-sm'
			: 'flex flex-col h-full justify-between space-y-3 text-sm';

		bodyClass = embedded ? 'space-y-3' : 'overflow-y-auto scrollbar-hidden h-full pr-2 space-y-3';
	}

	let taskConfig = {
		TASK_MODEL: '',
		TASK_MODEL_EXTERNAL: '',
		ENABLE_TITLE_GENERATION: true,
		TITLE_GENERATION_PROMPT_TEMPLATE: '',
		IMAGE_PROMPT_GENERATION_PROMPT_TEMPLATE: '',
		ENABLE_AUTOCOMPLETE_GENERATION: true,
		AUTOCOMPLETE_GENERATION_INPUT_MAX_LENGTH: -1,
		TAGS_GENERATION_PROMPT_TEMPLATE: '',
		ENABLE_TAGS_GENERATION: true,
		ENABLE_SEARCH_QUERY_GENERATION: true,
		ENABLE_RETRIEVAL_QUERY_GENERATION: true,
		QUERY_GENERATION_PROMPT_TEMPLATE: '',
		TOOLS_FUNCTION_CALLING_PROMPT_TEMPLATE: '',
		CODE_INTERPRETER_PROMPT_TEMPLATE: ''
	};

	let loading = true;
	let saving = false;
	let initialSnapshot: ReturnType<typeof buildSnapshot> | null = null;
	let autoSyncBaseline = false;
	let baselineSyncTimeout: ReturnType<typeof setTimeout> | null = null;
	const BASELINE_SYNC_WINDOW_MS = 400;

	const buildSnapshot = () => ({
		taskConfig: cloneSettingsSnapshot(taskConfig)
	});

	let snapshot: ReturnType<typeof buildSnapshot> | null = null;
	let tasksDirty = false;

	$: {
		taskConfig;
		snapshot = buildSnapshot();
	}
	$: tasksDirty = !!(
		snapshot &&
		initialSnapshot &&
		!isSettingsSnapshotEqual(snapshot, initialSnapshot)
	);
	$: if ($models.length > 0 && taskConfig?.TASK_MODEL_EXTERNAL) {
		const resolvedTaskModel = resolveModelSelectionId($models, taskConfig.TASK_MODEL_EXTERNAL);
		if (resolvedTaskModel && resolvedTaskModel !== taskConfig.TASK_MODEL_EXTERNAL) {
			taskConfig = { ...taskConfig, TASK_MODEL_EXTERNAL: resolvedTaskModel };
		}
	}

	// Unified dirty / save / reset API for parent page shell
	let lastDirtyState: boolean | null = null;
	$: if (tasksDirty !== lastDirtyState) {
		lastDirtyState = tasksDirty;
		dispatch('dirtyChange', { value: tasksDirty });
	}

	export const save = async () => {
		await updateInterfaceHandler();
	};

	export const reset = async () => {
		resetTasksChanges();
	};

	$: if (
		snapshot &&
		autoSyncBaseline &&
		(initialSnapshot === null || !isSettingsSnapshotEqual(snapshot, initialSnapshot))
	) {
		initialSnapshot = cloneSettingsSnapshot(snapshot);
	}

	const startBaselineSync = () => {
		autoSyncBaseline = true;
		if (baselineSyncTimeout) {
			clearTimeout(baselineSyncTimeout);
		}
		baselineSyncTimeout = setTimeout(() => {
			autoSyncBaseline = false;
			baselineSyncTimeout = null;
		}, BASELINE_SYNC_WINDOW_MS);
	};

	const updateInterfaceHandler = async () => {
		saving = true;
		try {
			taskConfig = await updateTaskConfig(localStorage.token, taskConfig);
			await config.set(await getBackendConfig());
			await tick();
			startBaselineSync();
			initialSnapshot = cloneSettingsSnapshot(buildSnapshot());
		} finally {
			saving = false;
		}
	};

	const resetTasksChanges = () => {
		if (!initialSnapshot) return;
		taskConfig = cloneSettingsSnapshot(initialSnapshot.taskConfig);
	};

	onMount(async () => {
		taskConfig = await getTaskConfig(localStorage.token);
		await tick();
		startBaselineSync();
		initialSnapshot = cloneSettingsSnapshot(buildSnapshot());
		loading = false;
	});

	onDestroy(() => {
		if (baselineSyncTimeout) {
			clearTimeout(baselineSyncTimeout);
		}
	});
</script>

{#if !loading && taskConfig}
	<form
		class={rootClass}
		on:submit|preventDefault={async () => {
			await updateInterfaceHandler();
			dispatch('save');
		}}
	>
		<div class={bodyClass}>
			<div class={embedded ? '' : 'max-w-6xl mx-auto space-y-6'}>
			<!-- 任务 Tasks -->
			<div
				bind:this={sectionEl_tasks}
				class={embedded ? '' : `scroll-mt-2 transition-all duration-300 ${tasksDirty ? 'glass-section glass-section-dirty' : 'glass-section'}`}
			>
				{#if !embedded}
				<button
					type="button"
					class="w-full flex items-center justify-between px-5 py-4 text-left rounded-2xl hover:bg-black/[0.02] dark:hover:bg-white/[0.02] transition-colors"
					aria-expanded={expandedSections.tasks}
					on:click={async () => {
						expandedSections.tasks = !expandedSections.tasks;
						if (expandedSections.tasks) {
							await revealExpandedSection(sectionEl_tasks);
						}
					}}
				>
					<div class="flex items-center gap-3">
						<div class="glass-icon-badge bg-rose-50 dark:bg-rose-950/30">
							<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="size-[18px] text-rose-400 dark:text-rose-400">
								<path fill-rule="evenodd" d="M5.625 1.5c-1.036 0-1.875.84-1.875 1.875v17.25c0 1.035.84 1.875 1.875 1.875h12.75c1.035 0 1.875-.84 1.875-1.875V12.75A3.75 3.75 0 0 0 16.5 9h-1.875a1.875 1.875 0 0 1-1.875-1.875V5.25A3.75 3.75 0 0 0 9 1.5H5.625ZM7.5 15a.75.75 0 0 1 .75-.75h7.5a.75.75 0 0 1 0 1.5h-7.5A.75.75 0 0 1 7.5 15Zm.75 2.25a.75.75 0 0 0 0 1.5H12a.75.75 0 0 0 0-1.5H8.25Z" clip-rule="evenodd" />
								<path d="M12.971 1.816A5.23 5.23 0 0 1 14.25 5.25v1.875c0 .207.168.375.375.375H16.5a5.23 5.23 0 0 1 3.434 1.279 9.768 9.768 0 0 0-6.963-6.963Z" />
							</svg>
						</div>
						<span class="text-base font-semibold text-gray-800 dark:text-gray-100">{$i18n.t('AI Tasks')}</span>
					</div>
					<div class="transform transition-transform duration-200 {expandedSections.tasks ? 'rotate-180' : ''}">
						<ChevronDown className="size-5 text-gray-400 dark:text-gray-500" />
					</div>
				</button>
				{/if}

				{#if embedded || expandedSections.tasks}
				<div class="{embedded ? '' : 'px-5 pb-5'} space-y-3">
					{#if !embedded}
					<InlineDirtyActions dirty={tasksDirty} saving={saving} on:reset={resetTasksChanges} />
					{/if}
					<div class="space-y-3">
						<!-- Task Model -->
						<div class="glass-item p-4">
							<div class="flex items-center justify-between mb-3">
								<div class="text-sm font-medium">{$i18n.t('Set Task Model')}</div>
								<Tooltip
									content={$i18n.t(
										'A task model is used when performing tasks such as generating titles for chats and web search queries'
									)}
								>
									<svg
										xmlns="http://www.w3.org/2000/svg"
										fill="none"
										viewBox="0 0 24 24"
										stroke-width="1.5"
										stroke="currentColor"
										class="size-3.5"
									>
										<path
											stroke-linecap="round"
											stroke-linejoin="round"
											d="m11.25 11.25.041-.02a.75.75 0 0 1 1.063.852l-.708 2.836a.75.75 0 0 0 1.063.853l.041-.021M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9-3.75h.008v.008H12V8.25Z"
										/>
									</svg>
								</Tooltip>
							</div>
							<HaloSelect
								bind:value={taskConfig.TASK_MODEL_EXTERNAL}
								options={[
									{ value: '', label: $i18n.t('Current Model') },
									...$models.map((model) => ({
										value: getModelSelectionId(model),
										label: getModelChatDisplayName(model)
									}))
								]}
								placeholder={$i18n.t('Select a model')}
								className="w-full"
							/>
						</div>

						<!-- Title Generation -->
						<div class="glass-item p-4">
							<div class="flex items-center justify-between mb-3">
								<div class="text-sm font-medium">{$i18n.t('Title Generation')}</div>
								<Switch bind:state={taskConfig.ENABLE_TITLE_GENERATION} />
							</div>
							{#if taskConfig.ENABLE_TITLE_GENERATION}
								<div>
									<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('Title Generation Prompt')}</div>
									<Textarea
										bind:value={taskConfig.TITLE_GENERATION_PROMPT_TEMPLATE}
										placeholder={$i18n.t(
											'Leave empty to use the default prompt, or enter a custom prompt'
										)}
									/>
								</div>
							{/if}
						</div>

						<!-- Tags Generation -->
						<div class="glass-item p-4">
							<div class="flex items-center justify-between mb-3">
								<div class="text-sm font-medium">{$i18n.t('Tags Generation')}</div>
								<Switch bind:state={taskConfig.ENABLE_TAGS_GENERATION} />
							</div>
							{#if taskConfig.ENABLE_TAGS_GENERATION}
								<div>
									<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('Tags Generation Prompt')}</div>
									<Textarea
										bind:value={taskConfig.TAGS_GENERATION_PROMPT_TEMPLATE}
										placeholder={$i18n.t(
											'Leave empty to use the default prompt, or enter a custom prompt'
										)}
									/>
								</div>
							{/if}
						</div>

						<!-- Query Generation -->
						<div class="glass-item p-4">
							<div class="text-sm font-medium mb-3">{$i18n.t('Query Generation')}</div>
							<div>
								<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">{$i18n.t('Query Generation Prompt')}</div>
								<Textarea
									bind:value={taskConfig.QUERY_GENERATION_PROMPT_TEMPLATE}
									placeholder={$i18n.t(
										'Leave empty to use the default prompt, or enter a custom prompt'
									)}
								/>
							</div>
						</div>

						<!-- Image Prompt Generation -->
						<div class="glass-item p-4">
							<div class="text-sm font-medium mb-3">{$i18n.t('Image Prompt Generation')}</div>
							<div>
								<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">
									{$i18n.t('Image Prompt Generation Prompt')}
								</div>
								<Textarea
									bind:value={taskConfig.IMAGE_PROMPT_GENERATION_PROMPT_TEMPLATE}
									placeholder={$i18n.t(
										'Leave empty to use the default prompt, or enter a custom prompt'
									)}
								/>
							</div>
						</div>

						<!-- Tools Function Calling -->
						<div class="glass-item p-4">
							<div class="text-sm font-medium mb-3">{$i18n.t('Tools Function Calling')}</div>
							<div>
								<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">
									{$i18n.t('Tools Function Calling Prompt')}
								</div>
								<Textarea
									bind:value={taskConfig.TOOLS_FUNCTION_CALLING_PROMPT_TEMPLATE}
									placeholder={$i18n.t(
										'Leave empty to use the default prompt, or enter a custom prompt'
									)}
								/>
							</div>
						</div>

						<!-- Code Interpreter Prompt -->
						<div class="glass-item p-4">
							<div class="text-sm font-medium mb-3">{$i18n.t('Code Interpreter')}</div>
							<div>
								<div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1.5">
									{$i18n.t('Code Interpreter Prompt')}
								</div>
								<Textarea
									bind:value={taskConfig.CODE_INTERPRETER_PROMPT_TEMPLATE}
									placeholder={$i18n.t(
										'Leave empty to use the default prompt, or enter a custom prompt'
									)}
								/>
							</div>
						</div>

					</div>
				</div>
				{/if}
			</div>
			</div>
		</div>

	</form>
{:else}
	<div class="h-full w-full flex justify-center items-center">
		<Spinner className="size-5" />
	</div>
{/if}
