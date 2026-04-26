<script lang="ts">
	import VirtualList from '@sveltejs/svelte-virtual-list';
	import { Select } from 'bits-ui';
	import { tick } from 'svelte';

	import { flyAndScale } from '$lib/utils/transitions';
	import { mobile } from '$lib/stores';
	import { getModelChatDisplayName } from '$lib/utils/model-display';
	import { getModelSelectionId, resolveModelSelectionId } from '$lib/utils/model-identity';

	import ChevronDown from '$lib/components/icons/ChevronDown.svelte';
	import Check from '$lib/components/icons/Check.svelte';
	import Search from '$lib/components/icons/Search.svelte';
	import ModelIcon from '$lib/components/common/ModelIcon.svelte';
	import OverflowTooltip from '$lib/components/common/OverflowTooltip.svelte';

	type ModelLike = any;
	type Item = { value: string; label: string; model: ModelLike | null };

	export let value = '';
	export let models: ModelLike[] = [];
	export let disabled = false;

	export let includeEmpty = false;
	export let emptyValue = '';
	export let emptyLabel = 'Not set';

	export let placeholder = 'Select a model';
	export let searchEnabled = true;
	export let searchPlaceholder = 'Search a model';

	export let showIdInList = true;
	export let showIdInTrigger = false;

	export let triggerClassName = 'text-sm';
	export let contentClassName = '';
	export let side: 'bottom' | 'right' | 'left' | 'top' = 'bottom';

	let searchValue = '';
	let open = false;
	let visibleStart = 0;
	let visibleEnd = 0;
	let searchInputEl: HTMLInputElement | null = null;

	const VIRTUALIZATION_THRESHOLD = 40;
	const EAGER_ICON_COUNT = 8;
	const LIST_MAX_HEIGHT = 320;
	const VIRTUAL_ROW_HEIGHT = 60;
	const ITEM_VISIBILITY_STYLE = 'content-visibility: auto; contain-intrinsic-size: 60px;';

	const getIconSrc = (model: ModelLike | null) =>
		model?.info?.meta?.profile_image_url ?? model?.meta?.profile_image_url ?? '/static/favicon.png';

	const getListIconLoading = (index: number) =>
		index < EAGER_ICON_COUNT || index < visibleEnd ? 'eager' : 'lazy';

	$: items = [
		...(includeEmpty ? [{ value: emptyValue, label: emptyLabel, model: null }] : []),
		...(models ?? []).map((m) => ({
			value: getModelSelectionId(m),
			label: getModelChatDisplayName(m) || m?.name || m?.id,
			model: m
		}))
	] as Item[];

	$: selectedItem =
		items.find((item) => item.value === (resolveModelSelectionId(models, value) || value)) ?? null;

	$: filteredItems = searchValue
		? items.filter((item) => {
				const q = searchValue.toLowerCase();
				const v = item.value.toLowerCase();
				const l = item.label.toLowerCase();
				return v.includes(q) || l.includes(q);
			})
		: items;
	$: useVirtualList = filteredItems.length > VIRTUALIZATION_THRESHOLD;
	$: listHeight = `${Math.min(
		LIST_MAX_HEIGHT,
		Math.max(VIRTUAL_ROW_HEIGHT, filteredItems.length * VIRTUAL_ROW_HEIGHT)
	)}px`;
</script>

<Select.Root
	{items}
	bind:open={open}
	{disabled}
	onOpenChange={async (next) => {
		searchValue = '';
		visibleStart = 0;
		visibleEnd = next ? Math.min(filteredItems.length, EAGER_ICON_COUNT) : 0;
		if (next && searchEnabled) {
			await tick();
			searchInputEl?.focus();
		}
	}}
	selected={selectedItem ?? undefined}
	onSelectedChange={(next) => {
		value = next?.value ?? emptyValue;
	}}
>
	<Select.Trigger
		class="relative w-full font-primary rounded-lg outline-hidden bg-transparent {disabled
			? 'cursor-not-allowed opacity-60'
			: ''}"
		aria-label={selectedItem?.label ?? placeholder}
	>
		<div class="flex items-center gap-2 w-full min-w-0 px-0.5 {triggerClassName}">
			{#if selectedItem?.model}
				<ModelIcon
					src={getIconSrc(selectedItem.model)}
					alt={selectedItem.label}
					className="rounded-lg size-5 shrink-0"
					loading="eager"
				/>
			{:else}
				<div class="rounded-lg size-5 shrink-0 bg-gray-200/70 dark:bg-gray-700/60" />
			{/if}

			<div class="flex flex-col min-w-0 flex-1">
				{#if selectedItem}
					<OverflowTooltip
						content={selectedItem.label}
						className="min-w-0"
						textClassName="block font-medium truncate whitespace-nowrap"
					>
						{selectedItem.label}
					</OverflowTooltip>
				{:else}
					<div class="font-medium line-clamp-1 text-gray-400">{placeholder}</div>
				{/if}
				{#if showIdInTrigger && selectedItem?.model}
					<OverflowTooltip
						content={selectedItem.model.id}
						className="min-w-0"
						textClassName="block text-xs text-gray-400 truncate whitespace-nowrap"
					>
						{selectedItem.model.id}
					</OverflowTooltip>
				{/if}
			</div>

			<ChevronDown className="ml-2 size-3.5 shrink-0 text-gray-400" strokeWidth="2.5" />
		</div>
	</Select.Trigger>

	<Select.Content
		class="z-40 {$mobile
			? 'w-[calc(100vw-1rem)]'
			: 'w-full'} max-w-[calc(100vw-1rem)] rounded-xl bg-white dark:bg-gray-900 dark:text-white shadow-lg border border-gray-200/80 dark:border-gray-700/60 outline-hidden {contentClassName}"
		transition={flyAndScale}
		{side}
		sideOffset={4}
		sameWidth={!$mobile && side === 'bottom'}
		fitViewport={true}
	>
		{#if searchEnabled}
			<div class="flex items-center gap-2.5 px-4 mt-3 mb-2">
				<Search className="size-4 text-gray-400" strokeWidth="2.5" />
				<input
					bind:this={searchInputEl}
					bind:value={searchValue}
					class="w-full text-sm bg-transparent outline-hidden"
					placeholder={searchPlaceholder}
					autocomplete="off"
				/>
			</div>

			<hr class="border-gray-100 dark:border-gray-850" />
		{/if}

		<div class="px-2 my-2">
			{#if filteredItems.length > 0}
				{#if useVirtualList}
					<div class="model-select-virtual-list">
						<VirtualList
							items={filteredItems}
							itemHeight={VIRTUAL_ROW_HEIGHT}
							height={listHeight}
							bind:start={visibleStart}
							bind:end={visibleEnd}
							let:item
						>
							<Select.Item
								class="flex w-full select-none items-center rounded-lg py-2.5 pl-2.5 pr-2 text-sm text-gray-700 dark:text-gray-100 outline-hidden transition hover:bg-gray-100 dark:hover:bg-gray-850 cursor-pointer data-highlighted:bg-muted"
								style={ITEM_VISIBILITY_STYLE}
								value={item.value}
								label={item.label}
							>
								<div class="flex items-center gap-2 w-full min-w-0">
									{#if item.model}
										<ModelIcon
											src={getIconSrc(item.model)}
											alt={item.label}
											className="rounded-lg size-6 shrink-0"
											loading="eager"
										/>
									{:else}
										<div class="rounded-lg size-6 shrink-0 bg-gray-200/70 dark:bg-gray-700/60" />
									{/if}

									<div class="flex flex-col min-w-0 flex-1">
										<div class="block font-medium truncate whitespace-nowrap" title={item.label}>
											{item.label}
										</div>
										{#if showIdInList && item.model}
											<div
												class="block text-xs text-gray-400 truncate whitespace-nowrap"
												title={item.model.id}
											>
												{item.model.id}
											</div>
										{/if}
									</div>

									{#if value === item.value}
										<div class="shrink-0 text-gray-500 dark:text-gray-300">
											<Check />
										</div>
									{/if}
								</div>
							</Select.Item>
						</VirtualList>
					</div>
				{:else}
					<div class="max-h-80 overflow-y-auto scrollbar-hidden">
						{#each filteredItems as item, index (item.value)}
							<Select.Item
								class="flex w-full select-none items-center rounded-lg py-2.5 pl-2.5 pr-2 text-sm text-gray-700 dark:text-gray-100 outline-hidden transition hover:bg-gray-100 dark:hover:bg-gray-850 cursor-pointer data-highlighted:bg-muted"
								style={ITEM_VISIBILITY_STYLE}
								value={item.value}
								label={item.label}
							>
								<div class="flex items-center gap-2 w-full min-w-0">
									{#if item.model}
										<ModelIcon
											src={getIconSrc(item.model)}
											alt={item.label}
											className="rounded-lg size-6 shrink-0"
											loading={getListIconLoading(index)}
										/>
									{:else}
										<div class="rounded-lg size-6 shrink-0 bg-gray-200/70 dark:bg-gray-700/60" />
									{/if}

									<div class="flex flex-col min-w-0 flex-1">
										<div class="block font-medium truncate whitespace-nowrap" title={item.label}>
											{item.label}
										</div>
										{#if showIdInList && item.model}
											<div
												class="block text-xs text-gray-400 truncate whitespace-nowrap"
												title={item.model.id}
											>
												{item.model.id}
											</div>
										{/if}
									</div>

									{#if value === item.value}
										<div class="shrink-0 text-gray-500 dark:text-gray-300">
											<Check />
										</div>
									{/if}
								</div>
							</Select.Item>
						{/each}
					</div>
				{/if}
			{:else}
				<div class="px-4 py-2 text-sm text-gray-500">No results found</div>
			{/if}
		</div>
	</Select.Content>
</Select.Root>

<style>
	:global(.model-select-virtual-list svelte-virtual-list-viewport) {
		overflow-y: auto;
		scrollbar-width: none;
	}

	:global(.model-select-virtual-list svelte-virtual-list-viewport::-webkit-scrollbar) {
		display: none;
	}
</style>
