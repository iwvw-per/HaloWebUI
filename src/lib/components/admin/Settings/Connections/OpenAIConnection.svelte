<script lang="ts">
	import { getContext } from 'svelte';
	import type { Writable } from 'svelte/store';
	import type { i18n as i18nType } from 'i18next';

	const i18n = getContext('i18n') as Writable<i18nType>;

	import Cog6 from '$lib/components/icons/Cog6.svelte';
	import LetterAvatar from '$lib/components/common/LetterAvatar.svelte';
	import ModelIcon from '$lib/components/common/ModelIcon.svelte';
	import AddConnectionModal from '$lib/components/AddConnectionModal.svelte';
	import ConfirmDialog from '$lib/components/common/ConfirmDialog.svelte';
	import { getApiKeyPoolModeLabel, getApiKeyPoolSummary } from '$lib/utils/api-key-pool';

	type ConnectionConfig = {
		enable?: boolean;
		remark?: string;
		tags?: Array<{ name: string }>;
		icon?: string;
		api_key_pool?: any;
	};
	type Connection = {
		url: string;
		key: string;
		config: ConnectionConfig;
	};

	export let onDelete: () => void | Promise<void> = () => {};
	export let onSubmit: (connection: Connection) => void | Promise<void> = async () => {};

	export let url = '';
	export let key = '';
	export let config: ConnectionConfig = {};

	let showConfigModal = false;
	let showDeleteConfirmDialog = false;

	$: isEnabled = config?.enable ?? true;
	$: displayName = config?.remark || url;
	$: tags = config?.tags ?? [];
	$: keyPoolSummary = getApiKeyPoolSummary(config, key);

	const handleSubmit = async (connection: Connection) => {
		url = connection.url;
		key = connection.key;
		config = connection.config;
		await onSubmit(connection);
	};
</script>

<ConfirmDialog
	bind:show={showDeleteConfirmDialog}
	on:confirm={() => {
		onDelete();
	}}
/>

<AddConnectionModal
	edit
	bind:show={showConfigModal}
	connection={{
		url,
		key,
		config
	}}
	onDelete={() => {
		showDeleteConfirmDialog = true;
	}}
	onSubmit={handleSubmit}
/>

<button
	type="button"
	class="w-full bg-white dark:bg-gray-900 rounded-lg px-4 py-3 border border-gray-100 dark:border-gray-800 hover:border-gray-200 dark:hover:border-gray-700 transition cursor-pointer text-left {!isEnabled
		? 'opacity-60'
		: ''}"
	on:click={() => {
		showConfigModal = true;
	}}
>
	<div class="flex items-center justify-between gap-3">
		<div class="flex items-center gap-3 flex-1 min-w-0">
			{#if config?.icon}
				<ModelIcon
					src={config.icon}
					alt="avatar"
					title={displayName}
					className="rounded-xl size-8 shrink-0"
				/>
			{:else}
				<LetterAvatar name={displayName} size="size-8" />
			{/if}
			<div class="flex-1 min-w-0">
				<div class="flex items-center gap-2 flex-wrap">
					<div class="text-sm font-medium text-gray-800 dark:text-gray-100 truncate">
						{displayName}
					</div>
					{#if !isEnabled}
						<span
							class="text-xs px-1.5 py-0.5 bg-gray-100 dark:bg-gray-800 text-gray-500 dark:text-gray-400 rounded"
						>
							{$i18n.t('Disabled')}
						</span>
					{/if}
					{#each tags as tag}
						<span
							class="text-xs px-1.5 py-0.5 bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400 rounded"
						>
							{tag.name}
						</span>
					{/each}
					{#if keyPoolSummary.total > 0}
						<span
							class="text-xs px-1.5 py-0.5 bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-300 rounded"
						>
							{$i18n.t('{{count}} keys', { count: keyPoolSummary.total })}
						</span>
						<span
							class="text-xs px-1.5 py-0.5 bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-300 rounded"
						>
							{getApiKeyPoolModeLabel(keyPoolSummary.mode, (key) => $i18n.t(key))}
						</span>
						{#if keyPoolSummary.retry}
							<span
								class="text-xs px-1.5 py-0.5 bg-emerald-50 dark:bg-emerald-900/20 text-emerald-700 dark:text-emerald-300 rounded"
							>
								{$i18n.t('Auto retry')}
							</span>
						{/if}
					{/if}
				</div>
				{#if config?.remark && url}
					<div class="text-xs text-gray-400 dark:text-gray-500 truncate mt-0.5">
						{url}
					</div>
				{/if}
			</div>
		</div>
		<div class="flex-shrink-0">
			<Cog6 className="size-4 text-gray-400 dark:text-gray-500" />
		</div>
	</div>
</button>
