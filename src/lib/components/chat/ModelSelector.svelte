<script lang="ts">
	import { models, modelsError, modelsStatus, user } from '$lib/stores';
	import { getContext } from 'svelte';
	import type { Writable } from 'svelte/store';
	import Selector from './ModelSelector/Selector.svelte';
	import Tooltip from '../common/Tooltip.svelte';
	import Switch from '$lib/components/common/Switch.svelte';
	import { getModelChatDisplayName } from '$lib/utils/model-display';
	import { getModelSelectionId, resolveModelSelectionId } from '$lib/utils/model-identity';
	import { getTemporaryChatAccess } from '$lib/utils/temporary-chat';
	const i18n: Writable<any> = getContext('i18n');

	export let selectedModels = [''];
	export let disabled = false;
	export let multiModelDiscussionEnabled = false;
	export let maxDiscussionModels = 5;

	export let showSetDefault = true;

	$: canUseMultipleModels =
		$user?.role === 'admin' || ($user?.permissions?.chat?.multiple_models ?? true);
	$: selectedModelCount = selectedModels.filter((model) => `${model ?? ''}`.trim()).length;
	$: showMultiModelDiscussionToggle = canUseMultipleModels && selectedModels.length >= 2;
	$: discussionDisabled = disabled || !canUseMultipleModels || selectedModelCount < 2;
	$: discussionIssue = !canUseMultipleModels
		? $i18n.t('Multiple models are not enabled for your account')
		: selectedModelCount < 2
			? $i18n.t('Select at least 2 models to start a discussion')
			: selectedModelCount > maxDiscussionModels
				? $i18n.t('Discussion uses the first {{count}} selected models', {
						count: maxDiscussionModels
					})
				: '';
	$: if ((!canUseMultipleModels || selectedModelCount < 2) && multiModelDiscussionEnabled) {
		multiModelDiscussionEnabled = false;
	}

	// Stable items array: only recomputed when $models reference changes
	$: selectorItems = $models.map((model) => ({
		value: getModelSelectionId(model),
		label: getModelChatDisplayName(model),
		model: model
	}));

	$: if (selectedModels.length > 0 && $models.length > 0) {
		const normalizedModels = selectedModels.map((model) =>
			resolveModelSelectionId($models, model, { preserveAmbiguous: true, preserveMissing: true })
		);
		if (JSON.stringify(normalizedModels) !== JSON.stringify(selectedModels)) {
			selectedModels = normalizedModels;
		}
	}

	let temporaryChatAccess = { allowed: true, enforced: false };
	$: temporaryChatAccess = getTemporaryChatAccess($user);
</script>

<div class="flex min-w-0 flex-col w-full items-start">
	{#if $modelsStatus === 'error' && $models.length === 0}
		<div class="text-xs text-gray-500 ml-1 pb-1">
			<span>{$i18n.t('Failed to load models')}</span>
			{#if $modelsError}
				<span class="text-gray-400">: {$modelsError}</span>
			{/if}
		</div>
	{/if}

	{#if selectedModels.length <= 1}
		<!-- 单模型：添加入口紧跟当前模型，避免被整行布局推到右侧角落。 -->
		{#each selectedModels as selectedModel, selectedModelIdx}
			<div class="flex w-fit max-w-full min-w-0 items-center gap-1.5">
				<div class="min-w-0 max-w-full overflow-hidden">
					<div class="min-w-0 max-w-full">
						<Selector
							id={`${selectedModelIdx}`}
							placeholder={$i18n.t('Select a model')}
							items={selectorItems}
							showSetDefaultAction={showSetDefault && selectedModelIdx === 0}
							showTemporaryChatControl={temporaryChatAccess.allowed &&
								!temporaryChatAccess.enforced}
							bind:value={selectedModel}
						/>
					</div>
				</div>

				{#if canUseMultipleModels}
					<div class="shrink-0">
						<Tooltip content={$i18n.t('Add Model')}>
							<button
								class="inline-flex items-center justify-center
									size-7 shrink-0 rounded-xl
									text-gray-400 dark:text-gray-500
									bg-white dark:bg-gray-900/70
									border border-dashed border-gray-300 dark:border-gray-600
									hover:bg-gray-100/80 dark:hover:bg-gray-800/60
									hover:border-gray-400 dark:hover:border-gray-500
									hover:text-gray-600 dark:hover:text-gray-300
									active:scale-[0.92]
									transition-all duration-150
									disabled:opacity-40 disabled:pointer-events-none"
								{disabled}
								on:click={() => {
									selectedModels = [...selectedModels, ''];
								}}
								aria-label="Add Model"
							>
								<svg
									xmlns="http://www.w3.org/2000/svg"
									fill="none"
									viewBox="0 0 24 24"
									stroke-width="2"
									stroke="currentColor"
									class="size-3"
								>
									<path stroke-linecap="round" stroke-linejoin="round" d="M12 6v12m6-6H6" />
								</svg>
							</button>
						</Tooltip>
					</div>
				{/if}
			</div>
		{/each}
	{:else}
		<!-- 多模型：每个模型都是完整胶囊，删除按钮始终占位可见，避免 hover 时出现隐形框。 -->
		<div class="flex w-full min-w-0 flex-wrap items-center gap-1.5 max-w-full">
			{#each selectedModels as selectedModel, selectedModelIdx}
				<div
					class="group/chip flex min-w-0 max-w-full items-center gap-1 rounded-xl border border-gray-200 bg-white px-1 py-0.5 shadow-xs transition-colors hover:border-gray-300 dark:border-gray-700 dark:bg-gray-900/70 dark:hover:border-gray-600"
				>
					<div class="min-w-0 overflow-hidden max-w-[210px] rounded-lg">
						<Selector
							id={`${selectedModelIdx}`}
							placeholder={$i18n.t('Select a model')}
							items={selectorItems}
							showSetDefaultAction={showSetDefault && selectedModelIdx === 0}
							showTemporaryChatControl={selectedModelIdx === 0 &&
								temporaryChatAccess.allowed &&
								!temporaryChatAccess.enforced}
							triggerClassName="text-sm rounded-lg !border-transparent !bg-transparent !shadow-none hover:!border-transparent hover:!bg-transparent dark:!border-transparent dark:!bg-transparent dark:hover:!border-transparent dark:hover:!bg-transparent"
							bind:value={selectedModel}
						/>
					</div>
					{#if selectedModelIdx > 0}
						<Tooltip content={$i18n.t('Remove Model')}>
							<button
								class="inline-flex items-center justify-center
									size-6 shrink-0 rounded-lg
									text-gray-400 dark:text-gray-500
									bg-transparent
									border border-transparent
									hover:border-red-100 dark:hover:border-red-900/40
									hover:bg-red-50/80 dark:hover:bg-red-900/20
									hover:text-red-500 dark:hover:text-red-400
									active:scale-[0.88]
									transition-all duration-150
									disabled:opacity-40 disabled:pointer-events-none"
								{disabled}
								on:click={() => {
									selectedModels.splice(selectedModelIdx, 1);
									selectedModels = selectedModels;
								}}
								aria-label="Remove Model"
							>
								<svg
									xmlns="http://www.w3.org/2000/svg"
									fill="none"
									viewBox="0 0 24 24"
									stroke-width="2.5"
									stroke="currentColor"
									class="size-3"
								>
									<path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
								</svg>
							</button>
						</Tooltip>
					{/if}
				</div>
			{/each}

			{#if canUseMultipleModels}
				<Tooltip content={$i18n.t('Add Model')}>
					<button
						class="inline-flex items-center justify-center
							size-7 shrink-0 rounded-xl
							text-gray-400 dark:text-gray-500
							bg-white dark:bg-gray-900/70
							border border-dashed border-gray-300 dark:border-gray-600
							hover:bg-gray-100/80 dark:hover:bg-gray-800/60
							hover:border-gray-400 dark:hover:border-gray-500
							hover:text-gray-600 dark:hover:text-gray-300
							active:scale-[0.92]
							transition-all duration-150
							disabled:opacity-40 disabled:pointer-events-none"
						{disabled}
						on:click={() => {
							selectedModels = [...selectedModels, ''];
						}}
						aria-label="Add Model"
					>
						<svg
							xmlns="http://www.w3.org/2000/svg"
							fill="none"
							viewBox="0 0 24 24"
							stroke-width="2"
							stroke="currentColor"
							class="size-3"
						>
							<path stroke-linecap="round" stroke-linejoin="round" d="M12 6v12m6-6H6" />
						</svg>
					</button>
				</Tooltip>
			{/if}
		</div>
	{/if}

	{#if showMultiModelDiscussionToggle}
		<div class="mt-2.5 flex max-w-full items-center">
			<Switch
				bind:state={multiModelDiscussionEnabled}
				disabled={discussionDisabled}
				variant="success"
				size="md"
				className="halo-switch-secondary"
				tooltip={discussionIssue || ''}
				ariaLabel={$i18n.t('Multi-model discussion')}
			>
				{$i18n.t('Multi-model discussion')}
			</Switch>
		</div>
	{/if}
</div>
