<script>
	import { toast } from 'svelte-sonner';

	import { createEventDispatcher, getContext, onMount } from 'svelte';
	const i18n = getContext('i18n');
	const dispatch = createEventDispatcher();

	import { models } from '$lib/stores';
	import { deleteAllModels } from '$lib/apis/models';

	import Modal from '$lib/components/common/Modal.svelte';
	import ConfirmDialog from '$lib/components/common/ConfirmDialog.svelte';
	import Tooltip from '$lib/components/common/Tooltip.svelte';
	import ModelList from './ModelList.svelte';
	import { getModelsConfig, setModelsConfig } from '$lib/apis/configs';
	import Spinner from '$lib/components/common/Spinner.svelte';
	import ChevronUp from '$lib/components/icons/ChevronUp.svelte';
	import ChevronDown from '$lib/components/icons/ChevronDown.svelte';
	import { getModelChatDisplayName } from '$lib/utils/model-display';
	import { findModelByIdentity, getModelSelectionId, resolveModelSelectionId } from '$lib/utils/model-identity';

	export let show = false;
	export let initHandler = () => {};

	let config = null;
	let modelIds = [];

	let sortKey = '';
	let sortOrder = '';

	let loading = false;
	let showResetModal = false;
	const modelListCollator = new Intl.Collator(undefined, {
		numeric: true,
		sensitivity: 'base'
	});

	const getModelDisplayName = (modelId) => {
		const model = findModelByIdentity($models, modelId);
		return (getModelChatDisplayName(model) ?? model?.name ?? model?.id ?? modelId).toString();
	};

	const normalizeModelOrderId = (modelId) => resolveModelSelectionId($models, modelId) || '';

	const compareModelIdsByDisplayName = (leftId, rightId, order = 'asc') => {
		const result = modelListCollator.compare(
			getModelDisplayName(leftId),
			getModelDisplayName(rightId)
		);

		return order === 'desc' ? -result : result;
	};

	$: if (show) {
		init();
	}

	const init = async () => {
		config = await getModelsConfig(localStorage.token);
		const modelOrderList = config.MODEL_ORDER_LIST || [];
		const allModelIds = $models.map((model) => getModelSelectionId(model)).filter(Boolean);

		const normalizedOrderedIds = Array.from(
			new Set(modelOrderList.map(normalizeModelOrderId).filter(Boolean))
		);
		const orderedSet = new Set(normalizedOrderedIds);

		modelIds = [
			// Add all IDs from MODEL_ORDER_LIST that still exist, migrating old IDs to selection IDs.
			...normalizedOrderedIds.filter((id) => allModelIds.includes(id)),
			// Add remaining IDs not in MODEL_ORDER_LIST, sorted by the same display name users see elsewhere.
			...allModelIds.filter((id) => !orderedSet.has(id)).sort((a, b) => compareModelIdsByDisplayName(a, b))
		];

		sortKey = '';
		sortOrder = '';
	};
	const submitHandler = async () => {
		loading = true;

		const res = await setModelsConfig(localStorage.token, {
			DEFAULT_MODELS: '',
			MODEL_ORDER_LIST: modelIds
		});

		if (res) {
			toast.success($i18n.t('Model order saved successfully'));
			initHandler();
			show = false;
		} else {
			toast.error($i18n.t('Failed to save model order'));
		}

		loading = false;
	};

	onMount(async () => {
		init();
	});
</script>

<ConfirmDialog
	title={$i18n.t('Reset All Models')}
	message={$i18n.t('This will delete all models including custom models and cannot be undone.')}
	bind:show={showResetModal}
	onConfirm={async () => {
		const res = deleteAllModels(localStorage.token);
		if (res) {
			toast.success($i18n.t('All models deleted successfully'));
			initHandler();
		}
	}}
/>

<Modal size="sm" bind:show>
	<div class="flex flex-col max-h-[85vh]">
		<!-- Header (fixed) -->
		<div class="flex items-center justify-between px-5 pt-4 pb-3 border-b border-gray-100 dark:border-gray-800">
			<div class="flex items-center gap-2.5">
				<div class="flex items-center justify-center size-8 rounded-lg bg-gray-100 dark:bg-gray-800">
					<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="size-4 text-gray-600 dark:text-gray-300">
						<path d="M2 4.75A.75.75 0 012.75 4h14.5a.75.75 0 010 1.5H2.75A.75.75 0 012 4.75zm0 10.5a.75.75 0 01.75-.75h7.5a.75.75 0 010 1.5h-7.5a.75.75 0 01-.75-.75zM2 10a.75.75 0 01.75-.75h10.5a.75.75 0 010 1.5H2.75A.75.75 0 012 10z" />
					</svg>
				</div>
				<div>
					<div class="text-sm font-semibold text-gray-800 dark:text-gray-100">
						{$i18n.t('Model Order Settings')}
					</div>
					<div class="text-xs text-gray-400 dark:text-gray-500">
						{$i18n.t('Model order controls how models are listed across the app.')}
					</div>
				</div>
			</div>
			<button
				class="p-1 rounded-lg text-gray-400 hover:text-gray-600 hover:bg-gray-100 dark:hover:text-gray-200 dark:hover:bg-gray-800 transition"
				on:click={() => { show = false; }}
			>
				<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="size-5">
					<path d="M6.28 5.22a.75.75 0 00-1.06 1.06L8.94 10l-3.72 3.72a.75.75 0 101.06 1.06L10 11.06l3.72 3.72a.75.75 0 101.06-1.06L11.06 10l3.72-3.72a.75.75 0 00-1.06-1.06L10 8.94 6.28 5.22z" />
				</svg>
			</button>
		</div>

		{#if config}
			<form
				class="flex flex-col min-h-0 flex-1"
				on:submit|preventDefault={() => { submitHandler(); }}
			>
				<!-- Toolbar (fixed) -->
				<div class="flex items-center justify-between px-5 py-2.5 border-b border-gray-50 dark:border-gray-850">
					<button
						class="flex items-center gap-1.5 px-2.5 py-1.5 text-xs font-medium rounded-lg text-gray-500 hover:text-gray-700 hover:bg-gray-100 dark:text-gray-400 dark:hover:text-gray-200 dark:hover:bg-gray-800 transition"
						type="button"
						on:click={() => {
							sortKey = 'model';
							const nextSortOrder = sortOrder === 'asc' ? 'desc' : 'asc';
							sortOrder = nextSortOrder;

							modelIds = modelIds
								.filter((id) => id !== '')
								.sort((a, b) => compareModelIdsByDisplayName(a, b, nextSortOrder));
						}}
					>
						<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="size-3.5">
							<path fill-rule="evenodd" d="M2 3.75A.75.75 0 012.75 3h11.5a.75.75 0 010 1.5H2.75A.75.75 0 012 3.75zM2 7.5a.75.75 0 01.75-.75h6.365a.75.75 0 010 1.5H2.75A.75.75 0 012 7.5zM14 7a.75.75 0 01.55.24l3.25 3.5a.75.75 0 11-1.1 1.02l-1.95-2.1v6.59a.75.75 0 01-1.5 0V9.66l-1.95 2.1a.75.75 0 11-1.1-1.02l3.25-3.5A.75.75 0 0114 7zM2 11.25a.75.75 0 01.75-.75H7A.75.75 0 017 12H2.75a.75.75 0 01-.75-.75z" clip-rule="evenodd" />
						</svg>
						{$i18n.t('Reorder Models')}
						{#if sortKey === 'model'}
							<span class="self-center">
								{#if sortOrder === 'asc'}
									<ChevronUp className="size-3" />
								{:else}
									<ChevronDown className="size-3" />
								{/if}
							</span>
						{/if}
					</button>
					<span class="text-xs text-gray-400 dark:text-gray-500 tabular-nums">
						{modelIds.length} {$i18n.t('models')}
					</span>
				</div>

				<!-- Model list (scrollable) -->
				<div class="flex-1 min-h-0 overflow-y-auto px-5 py-2">
					<ModelList bind:modelIds />
				</div>

				<!-- Footer (fixed) -->
				<div class="flex items-center justify-between px-5 py-3 border-t border-gray-100 dark:border-gray-800">
					<Tooltip content={$i18n.t('This will delete all models including custom models')}>
						<button
							class="px-3.5 py-1.5 text-xs font-medium rounded-lg border border-gray-200 dark:border-gray-700 text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 transition"
							type="button"
							on:click={() => { showResetModal = true; }}
						>
							{$i18n.t('Reset All Models')}
						</button>
					</Tooltip>

					<button
						class="px-4 py-1.5 text-xs font-medium bg-black hover:bg-gray-900 text-white dark:bg-white dark:text-black dark:hover:bg-gray-100 transition rounded-lg flex items-center gap-1.5 {loading
							? ' cursor-not-allowed'
							: ''}"
						type="submit"
						disabled={loading}
					>
						{$i18n.t('Save')}
						{#if loading}
							<Spinner className="size-3.5" />
						{/if}
					</button>
				</div>
			</form>
		{:else}
			<div class="flex items-center justify-center py-12">
				<Spinner />
			</div>
		{/if}
	</div>
</Modal>
