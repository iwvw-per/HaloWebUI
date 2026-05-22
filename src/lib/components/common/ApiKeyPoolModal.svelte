<script lang="ts">
	import { getContext } from 'svelte';
	import type { Writable } from 'svelte/store';
	import type { i18n as i18nType } from 'i18next';

	import Modal from '$lib/components/common/Modal.svelte';
	import SensitiveInput from '$lib/components/common/SensitiveInput.svelte';
	import Switch from '$lib/components/common/Switch.svelte';
	import Tooltip from '$lib/components/common/Tooltip.svelte';
	import Plus from '$lib/components/icons/Plus.svelte';
	import XMark from '$lib/components/icons/XMark.svelte';
	import ChevronUp from '$lib/components/icons/ChevronUp.svelte';
	import ChevronDown from '$lib/components/icons/ChevronDown.svelte';
	import GarbageBin from '$lib/components/icons/GarbageBin.svelte';

	const i18n = getContext('i18n') as Writable<i18nType>;

	type ApiKeyPoolMode = 'round_robin' | 'random' | 'priority';

	type ApiKeyPoolEntry = {
		id: string;
		label: string;
		key: string;
		enabled: boolean;
	};

	type ApiKeyPoolRetry = {
		enabled: boolean;
		preset: string;
		status_codes: number[];
		error_keywords: string[];
	};

	type ApiKeyPool = {
		keys: ApiKeyPoolEntry[];
		mode: ApiKeyPoolMode;
		retry: ApiKeyPoolRetry;
	};

	export let show = false;
	export let pool: ApiKeyPool;
	export let onTestKey: (entry: ApiKeyPoolEntry) => void = () => {};

	const RETRY_PRESET = 'rate_limit_transient';
	const DEFAULT_STATUS_CODES = [429, 500, 502, 503, 504];
	const DEFAULT_ERROR_KEYWORDS = [
		'rate limit',
		'rate_limit',
		'too many requests',
		'quota',
		'insufficient_quota',
		'over quota',
		'temporarily unavailable',
		'timeout',
		'timed out',
		'overloaded',
		'server error',
		'internal server error',
		'bad gateway',
		'service unavailable',
		'gateway timeout',
		'限流',
		'额度',
		'配额',
		'超时',
		'暂时不可用',
		'服务繁忙'
	];
	const MODE_OPTIONS: { value: ApiKeyPoolMode; label: string }[] = [
		{ value: 'round_robin', label: 'Round Robin' },
		{ value: 'random', label: 'Random' },
		{ value: 'priority', label: 'Priority' }
	];
	const defaultKeyLabel = (index: number) => `Key ${index}`;
	const normalizeKeyLabel = (value: any, index: number) => {
		const label = (value || '').toString().trim();
		return label && !/^(Key|密钥|金鑰)\s*\d+$/i.test(label) ? label : defaultKeyLabel(index);
	};

	let statusCodesText = DEFAULT_STATUS_CODES.join(', ');
	let errorKeywordsText = DEFAULT_ERROR_KEYWORDS.join('\n');
	let syncedPoolRef: ApiKeyPool | null = null;

	const newId = () =>
		`k_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`;

	const normalizeMode = (value: any): ApiKeyPoolMode =>
		['round_robin', 'random', 'priority'].includes(value) ? value : 'round_robin';

	const normalizePool = (value: ApiKeyPool | undefined | null): ApiKeyPool => {
		const keys = Array.isArray(value?.keys)
			? value.keys.map((entry, idx) => ({
					id: (entry?.id || '').toString().trim() || newId(),
					label: normalizeKeyLabel(entry?.label, idx + 1),
					key: (entry?.key || '').toString(),
					enabled: entry?.enabled !== false
				}))
			: [];

		return {
			keys: keys.length
				? keys
				: [
						{
							id: newId(),
							label: defaultKeyLabel(1),
							key: '',
							enabled: true
						}
					],
			mode: normalizeMode(value?.mode),
			retry: {
				enabled: value?.retry?.enabled !== false,
				preset: value?.retry?.preset || RETRY_PRESET,
				status_codes: Array.isArray(value?.retry?.status_codes)
					? value.retry.status_codes
					: [...DEFAULT_STATUS_CODES],
				error_keywords: Array.isArray(value?.retry?.error_keywords)
					? value.retry.error_keywords
					: [...DEFAULT_ERROR_KEYWORDS]
			}
		};
	};

	const parseStatusCodes = (value: string) => {
		const parsed = value
			.split(/[,\s]+/)
			.map((item) => Number.parseInt(item.trim(), 10))
			.filter((code, idx, arr) => Number.isInteger(code) && code >= 100 && code <= 599 && arr.indexOf(code) === idx);
		return parsed.length ? parsed : [...DEFAULT_STATUS_CODES];
	};

	const parseKeywords = (value: string) => {
		const parsed = value
			.split(/\n|,/)
			.map((item) => item.trim())
			.filter((item, idx, arr) => item && arr.indexOf(item) === idx);
		return parsed.length ? parsed : [...DEFAULT_ERROR_KEYWORDS];
	};

	const refreshTextFields = () => {
		statusCodesText = (pool?.retry?.status_codes?.length
			? pool.retry.status_codes
			: DEFAULT_STATUS_CODES
		).join(', ');
		errorKeywordsText = (pool?.retry?.error_keywords?.length
			? pool.retry.error_keywords
			: DEFAULT_ERROR_KEYWORDS
		).join('\n');
	};

	const commitRetryText = () => {
		pool = {
			...pool,
			retry: {
				...pool.retry,
				preset: RETRY_PRESET,
				status_codes: parseStatusCodes(statusCodesText),
				error_keywords: parseKeywords(errorKeywordsText)
			}
		};
		refreshTextFields();
	};

	const updateEntry = (id: string, patch: Partial<ApiKeyPoolEntry>) => {
		pool = {
			...pool,
			keys: pool.keys.map((entry) => (entry.id === id ? { ...entry, ...patch } : entry))
		};
	};

	const addKey = () => {
		pool = {
			...pool,
			keys: [
				...pool.keys,
				{
					id: newId(),
					label: defaultKeyLabel(pool.keys.length + 1),
					key: '',
					enabled: true
				}
			]
		};
	};

	const removeKey = (id: string) => {
		const nextKeys = pool.keys.filter((entry) => entry.id !== id);
		pool = {
			...pool,
			keys: nextKeys.length
				? nextKeys
				: [
						{
							id: newId(),
							label: defaultKeyLabel(1),
							key: '',
							enabled: true
						}
					]
		};
	};

	const touchPool = () => {
		pool = { ...pool, keys: [...pool.keys] };
	};

	const moveKey = (index: number, direction: -1 | 1) => {
		const nextIndex = index + direction;
		if (nextIndex < 0 || nextIndex >= pool.keys.length) return;
		const nextKeys = [...pool.keys];
		const [entry] = nextKeys.splice(index, 1);
		nextKeys.splice(nextIndex, 0, entry);
		pool = { ...pool, keys: nextKeys };
	};

	const setMode = (mode: ApiKeyPoolMode) => {
		pool = { ...pool, mode };
	};

	const updateEntryLabelFromEvent = (id: string, event: Event) => {
		updateEntry(id, { label: (event.currentTarget as HTMLInputElement).value });
	};

	$: if (pool && pool !== syncedPoolRef) {
		pool = normalizePool(pool);
		syncedPoolRef = pool;
		refreshTextFields();
	}
</script>

<Modal size="lg" bind:show dismissible={false}>
	<div class="flex max-h-[calc(100dvh-4rem)] flex-col overflow-hidden">
		<div class="flex items-center justify-between px-5 pb-3 pt-4">
			<div>
				<div class="text-lg font-medium text-gray-900 dark:text-gray-100">
					{$i18n.t('Manage API Keys')}
				</div>
				<div class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
					{$i18n.t('Keys are stored in this connection and only the first enabled key is mirrored to the legacy field.')}
				</div>
			</div>
			<button
				type="button"
				class="rounded-lg p-1 transition hover:bg-gray-100 dark:hover:bg-gray-800"
				aria-label={$i18n.t('Close modal')}
				on:click={() => {
					commitRetryText();
					show = false;
				}}
			>
				<XMark className="size-5" />
			</button>
		</div>

		<div class="flex-1 overflow-y-auto px-5 pb-4">
			<div class="flex flex-col gap-4">
				<div class="inline-grid w-full grid-cols-3 rounded-lg bg-gray-100 p-1 text-sm dark:bg-gray-800">
					{#each MODE_OPTIONS as option}
						<button
							type="button"
							class="rounded-md px-3 py-1.5 font-medium transition {pool.mode === option.value
								? 'bg-white text-gray-900 shadow-sm dark:bg-gray-950 dark:text-gray-100'
								: 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'}"
							on:click={() => setMode(option.value)}
						>
							{$i18n.t(option.label)}
						</button>
					{/each}
				</div>

				<div class="space-y-2">
					{#each pool.keys as entry, idx (entry.id)}
						<div class="rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-gray-800 dark:bg-gray-900">
							<div class="grid grid-cols-[minmax(7rem,0.5fr)_minmax(12rem,1fr)_auto] gap-2">
								<input
									class="min-w-0 rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm outline-none dark:border-gray-800 dark:bg-gray-950"
									value={entry.label}
									placeholder={$i18n.t('Label')}
									autocomplete="off"
									on:input={(event) => updateEntryLabelFromEvent(entry.id, event)}
								/>
								<SensitiveInput
									bind:value={entry.key}
									placeholder={$i18n.t('Enter API key')}
									required={false}
									outerClassName="min-w-0 flex px-3 py-2 bg-white dark:bg-gray-950 border border-gray-200 dark:border-gray-800 rounded-lg"
									inputClassName="w-full min-w-0 text-sm bg-transparent outline-none"
									on:input={touchPool}
								/>
								<div class="flex items-center gap-1">
									<Tooltip content={$i18n.t('Move up')}>
										<button
											type="button"
											class="rounded-lg p-2 transition hover:bg-gray-200 disabled:opacity-40 dark:hover:bg-gray-800"
											disabled={idx === 0}
											on:click={() => moveKey(idx, -1)}
										>
											<ChevronUp className="size-4" />
										</button>
									</Tooltip>
									<Tooltip content={$i18n.t('Move down')}>
										<button
											type="button"
											class="rounded-lg p-2 transition hover:bg-gray-200 disabled:opacity-40 dark:hover:bg-gray-800"
											disabled={idx === pool.keys.length - 1}
											on:click={() => moveKey(idx, 1)}
										>
											<ChevronDown className="size-4" />
										</button>
									</Tooltip>
									<Tooltip content={$i18n.t('Test key')}>
										<button
											type="button"
											class="rounded-lg px-2 py-1.5 text-xs font-medium text-gray-600 transition hover:bg-gray-200 dark:text-gray-300 dark:hover:bg-gray-800"
											on:click={() => onTestKey(entry)}
										>
											{$i18n.t('Test')}
										</button>
									</Tooltip>
									<Switch
										state={entry.enabled}
										on:change={(event) => updateEntry(entry.id, { enabled: event.detail })}
									/>
									<Tooltip content={$i18n.t('Delete')}>
										<button
											type="button"
											class="rounded-lg p-2 text-gray-500 transition hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20"
											on:click={() => removeKey(entry.id)}
										>
											<GarbageBin className="size-4" />
										</button>
									</Tooltip>
								</div>
							</div>
						</div>
					{/each}
					<button
						type="button"
						class="inline-flex items-center gap-2 rounded-lg border border-dashed border-gray-300 px-3 py-2 text-sm font-medium text-gray-600 transition hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-900"
						on:click={addKey}
					>
						<Plus className="size-4" />
						{$i18n.t('Add key')}
					</button>
				</div>

				<div class="rounded-lg border border-gray-200 p-3 dark:border-gray-800">
					<div class="flex items-center justify-between gap-4">
						<div>
							<div class="text-sm font-medium text-gray-800 dark:text-gray-100">
								{$i18n.t('Auto Retry With Next Key')}
							</div>
							<div class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
								{$i18n.t('Default rules cover rate limits, quota, timeout and temporary 5xx errors.')}
							</div>
						</div>
						<Switch
							state={pool.retry.enabled}
							on:change={(event) => {
								pool = { ...pool, retry: { ...pool.retry, enabled: event.detail, preset: RETRY_PRESET } };
							}}
						/>
					</div>

					{#if pool.retry.enabled}
						<div class="mt-3 grid gap-3 md:grid-cols-[minmax(10rem,0.35fr)_minmax(14rem,1fr)]">
							<div>
								<label for="api-key-pool-retry-status-codes" class="mb-1 block text-xs text-gray-500">
									{$i18n.t('Retry Status Codes')}
								</label>
								<input
									id="api-key-pool-retry-status-codes"
									class="w-full rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm outline-none dark:border-gray-800 dark:bg-gray-900"
									bind:value={statusCodesText}
									on:blur={commitRetryText}
									placeholder="429, 500, 502, 503, 504"
								/>
							</div>
							<div>
								<label for="api-key-pool-retry-error-keywords" class="mb-1 block text-xs text-gray-500">
									{$i18n.t('Retry Error Keywords')}
								</label>
								<textarea
									id="api-key-pool-retry-error-keywords"
									class="h-28 w-full resize-none rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm outline-none dark:border-gray-800 dark:bg-gray-900"
									bind:value={errorKeywordsText}
									on:blur={commitRetryText}
									placeholder={$i18n.t('One keyword per line, e.g. rate limit, quota, timeout')}
								/>
							</div>
						</div>
					{/if}
				</div>
			</div>
		</div>

		<div class="flex justify-end gap-2 border-t border-gray-100 px-5 py-3 dark:border-gray-800">
			<button
				type="button"
				class="rounded-lg bg-black px-4 py-2 text-sm font-medium text-white transition hover:bg-gray-800 dark:bg-white dark:text-black dark:hover:bg-gray-100"
				on:click={() => {
					commitRetryText();
					show = false;
				}}
			>
				{$i18n.t('Done')}
			</button>
		</div>
	</div>
</Modal>
