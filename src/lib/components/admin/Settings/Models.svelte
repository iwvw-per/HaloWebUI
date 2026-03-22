<script lang="ts">
	import { marked } from 'marked';
	import fileSaver from 'file-saver';
	const { saveAs } = fileSaver;

	import { onMount, getContext, tick } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import type { Writable } from 'svelte/store';

	const i18n: Writable<any> = getContext('i18n');

	import { WEBUI_NAME, config, models as _models, settings, user } from '$lib/stores';
	import { getModels } from '$lib/apis';
	import {
		bulkUpsertBaseModels,
		createNewModel,
		getBaseModels,
		updateModelById
	} from '$lib/apis/models';

	import Search from '$lib/components/icons/Search.svelte';
	import Tooltip from '$lib/components/common/Tooltip.svelte';
	import Dropdown from '$lib/components/common/Dropdown.svelte';
	import Switch from '$lib/components/common/Switch.svelte';
	import Spinner from '$lib/components/common/Spinner.svelte';
	import ModelIcon from '$lib/components/common/ModelIcon.svelte';
	import LetterAvatar from '$lib/components/common/LetterAvatar.svelte';
	import ArrowDownTray from '$lib/components/icons/ArrowDownTray.svelte';
	import EllipsisHorizontal from '$lib/components/icons/EllipsisHorizontal.svelte';
	import ChevronDown from '$lib/components/icons/ChevronDown.svelte';
	import Eye from '$lib/components/icons/Eye.svelte';
	import EyeSlash from '$lib/components/icons/EyeSlash.svelte';
	import GlobeAlt from '$lib/components/icons/GlobeAlt.svelte';
	import LockClosed from '$lib/components/icons/LockClosed.svelte';
	import Minus from '$lib/components/icons/Minus.svelte';
	import Check from '$lib/components/icons/Check.svelte';
	import { DropdownMenu } from 'bits-ui';

	import ModelEditor from '$lib/components/workspace/Models/ModelEditor.svelte';
	import ConfigureModelsModal from './Models/ConfigureModelsModal.svelte';
	import ManageModelsModal from './Models/ManageModelsModal.svelte';
	import ModelMenu from '$lib/components/admin/Settings/Models/ModelMenu.svelte';
	import { toast } from 'svelte-sonner';

	import { getModelChatDisplayName, getModelConnectionName } from '$lib/utils/model-display';
	import { applyModelIcons } from '$lib/utils/model-icons';

	type EnabledFilter = 'all' | 'enabled' | 'disabled';
	type HiddenFilter = 'all' | 'hidden' | 'visible';
	type VisibilityFilter = 'all' | 'public' | 'private' | 'unset';
	type ProviderTab = 'all' | 'openai' | 'gemini' | 'anthropic' | 'ollama' | 'other';

	type ModelLike = any;
	type Row =
		| {
				type: 'group';
				key: string;
				name: string;
				count: number;
				enabledCount: number;
				hiddenCount: number;
				publicCount: number;
				privateCount: number;
				unsetCount: number;
		  }
		| {
				type: 'model';
				groupKey: string;
				model: ModelLike;
		  };


	let models: ModelLike[] | null = null;
	let workspaceModels: any[] | null = null;
	let baseModels: any[] | null = null;

	let filteredModels: ModelLike[] = [];
	let modelById: Map<string, any> = new Map();
	let grouped: Map<string, { name: string; models: any[] }> = new Map();
	let groupKeys: string[] = [];
	let rows: Row[] = [];

	let selectedModelId: string | null = null;

	let showConfigModal = false;
	let showManageModal = false;


	let searchValue = '';
	let enabledFilter: EnabledFilter = 'all';
	let hiddenFilter: HiddenFilter = 'all';

	// ==================== Provider Tab ====================
	let selectedTab: ProviderTab = 'all';
	let prevTab: ProviderTab = 'all';

	const tabMeta: Record<ProviderTab, { label: string; description: string; badgeColor: string; iconColor: string }> = {
		all:       { label: '全部模型',       description: '查看和管理所有已连接的模型。',              badgeColor: 'bg-gray-100 dark:bg-gray-800/40',      iconColor: 'text-gray-500 dark:text-gray-400' },
		openai:    { label: 'OpenAI 接口',    description: '管理通过 OpenAI 兼容接口连接的模型。',     badgeColor: 'bg-emerald-50 dark:bg-emerald-950/30',  iconColor: 'text-emerald-500 dark:text-emerald-400' },
		gemini:    { label: 'Gemini 接口',    description: '管理通过 Google Gemini 接口连接的模型。',  badgeColor: 'bg-blue-50 dark:bg-blue-950/30',        iconColor: 'text-blue-500 dark:text-blue-400' },
		anthropic: { label: 'Anthropic 接口', description: '管理通过 Anthropic 接口连接的模型。',      badgeColor: 'bg-orange-50 dark:bg-orange-950/30',    iconColor: 'text-orange-500 dark:text-orange-400' },
		ollama:    { label: 'Ollama 接口',    description: '管理通过 Ollama 本地接口连接的模型。',     badgeColor: 'bg-violet-50 dark:bg-violet-950/30',    iconColor: 'text-violet-500 dark:text-violet-400' },
		other:     { label: '其他接口',       description: '管理未归类到上述接口的模型。',              badgeColor: 'bg-rose-50 dark:bg-rose-950/30',        iconColor: 'text-rose-500 dark:text-rose-400' }
	};

	$: activeTabMeta = tabMeta[selectedTab];

	const getProviderTab = (m: any): Exclude<ProviderTab, 'all'> => {
		const owned = (m?.owned_by ?? '').toString().toLowerCase();
		if (owned === 'openai' || owned.startsWith('openai') || owned === 'system') return 'openai';
		if (owned === 'google' || owned.startsWith('google')) return 'gemini';
		if (owned === 'anthropic' || owned.startsWith('anthropic')) return 'anthropic';
		if (owned === 'ollama' || owned.startsWith('ollama')) return 'ollama';
		return 'other';
	};

	$: tabFilteredModels = selectedTab === 'all'
		? (models ?? [])
		: (models ?? []).filter(m => getProviderTab(m) === selectedTab);

	$: tabCounts = (() => {
		const counts: Record<ProviderTab, number> = { all: 0, openai: 0, gemini: 0, anthropic: 0, ollama: 0, other: 0 };
		for (const m of models ?? []) { counts.all++; counts[getProviderTab(m)]++; }
		return counts;
	})();

	$: visibleTabs = (['all', 'openai', 'gemini', 'anthropic', 'ollama', 'other'] as ProviderTab[]).filter(
		t => t !== 'other' || tabCounts.other > 0
	);

	const shouldSpanModelTabFullRowOnMobile = (index: number) =>
		visibleTabs.length % 2 === 1 && index === visibleTabs.length - 1;

	$: if (selectedTab !== prevTab) {
		prevTab = selectedTab;
		clearSelection();
		expandedGroupsInitialized = false;
		tick().then(() => initExpandedGroups());
	}
	let visibilityFilter: VisibilityFilter = 'all';

	let expandedGroups: Record<string, boolean> = {};
	let expandedGroupsInitialized = false;

	let listContainerEl: HTMLDivElement | null = null;
	let listHeight = 400;
	let resizeObserver: ResizeObserver | null = null;

	let shiftKey = false;
	let lastSelectedId: string | null = null;

	let selectedIds: Set<string> = new Set();
	let selectMode = false;
	let bulkBusy = false;
	let bulkMenuOpen = false;
	let filterOpen = false;
	let suppressRouteSelection = false;

	const getRequestedModelId = () => $page.url.searchParams.get('id');

	const syncSelectedModelFromRoute = () => {
		const requestedId = getRequestedModelId();
		if (suppressRouteSelection || !requestedId || !Array.isArray(models)) return;

		const match = models.find((model) => model.id === requestedId);
		if (match && selectedModelId !== requestedId) {
			selectedModelId = requestedId;
		}
	};

	const clearSelectedModelRoute = async () => {
		if (!getRequestedModelId()) return;

		const nextUrl = new URL($page.url);
		nextUrl.searchParams.delete('id');

		const nextSearch = nextUrl.searchParams.toString();
		await goto(`${nextUrl.pathname}${nextSearch ? `?${nextSearch}` : ''}`, {
			replaceState: true,
			keepFocus: true,
			noScroll: true
		});
	};

	$: activeFilterCount =
		(enabledFilter !== 'all' ? 1 : 0) +
		(hiddenFilter !== 'all' ? 1 : 0) +
		(visibilityFilter !== 'all' ? 1 : 0);

	const init = async () => {
		[workspaceModels, baseModels] = await Promise.all([
			getBaseModels(localStorage.token),
			getModels(localStorage.token, null, true)
		]);

		const wsMap = new Map((workspaceModels ?? []).map((wm: any) => [wm.id, wm]));
		models =
			(baseModels ?? []).map((m: any) => {
				const workspaceModel = wsMap.get(m.id);
				// Preserve the upstream/provider default display name so the editor can "restore default"
				// even if the admin has overridden `name` in the workspace model DB.
				const default_name = m?.name ?? m?.id;

				if (workspaceModel) {
					const mergedMeta = { ...(m?.meta ?? {}), ...(workspaceModel?.meta ?? {}) };
					return {
						...m,
						...workspaceModel,
						meta: mergedMeta,
						default_name
					};
				}

				return {
					...m,
					id: m.id,
					name: m.name,
					default_name,
					is_active: true
				};
			}) ?? [];

		models = (applyModelIcons(models) ?? models) as any;

		initExpandedGroups();
		syncSelectedModelFromRoute();
	};

	$: syncSelectedModelFromRoute();

	const refreshGlobalModels = async () => {
		_models.set(
			await getModels(
				localStorage.token,
				($config as any)?.features?.enable_direct_connections &&
					(($settings as any)?.directConnections ?? null)
			)
		);
		await init();
	};

	const initExpandedGroups = () => {
		if (expandedGroupsInitialized) return;
		if (!Array.isArray(models)) return;

		const total = models.length;
		if (total <= 200) {
			// Small lists: expand everything.
			expandedGroups = {};
			for (const m of models) {
				const key = getModelConnectionName(m) || m?.owned_by || 'Unknown';
				expandedGroups[key] = true;
			}
		} else {
			// Large lists: start collapsed; search will auto-open matching groups.
			expandedGroups = {};
		}

		expandedGroupsInitialized = true;
	};

	const getVisibilityState = (m: any): 'public' | 'private' | 'unset' => {
		if (!m) return 'unset';
		if (m.access_control === null) return 'public';
		if (typeof m.access_control === 'undefined') return 'unset';
		return 'private';
	};

	const matchesSearch = (m: any, q: string): boolean => {
		if (!q) return true;
		const query = q.toLowerCase();

		const display = (getModelChatDisplayName(m) ?? '').toString().toLowerCase();
		const id = (m?.id ?? '').toString().toLowerCase();
		const originalId = (m?.originalId ?? m?.original_id ?? '').toString().toLowerCase();
		const desc = (m?.meta?.description ?? '').toString().toLowerCase();
		const connection = (getModelConnectionName(m) ?? '').toString().toLowerCase();

		return (
			display.includes(query) ||
			id.includes(query) ||
			originalId.includes(query) ||
			desc.includes(query) ||
			connection.includes(query)
		);
	};

	$: if (Array.isArray(tabFilteredModels)) {
		filteredModels = tabFilteredModels
			.filter((m) => {
				if (!matchesSearch(m, searchValue)) return false;
				if (enabledFilter !== 'all') {
					const active = m?.is_active ?? true;
					if (enabledFilter === 'enabled' ? !active : active) return false;
				}
				if (hiddenFilter !== 'all') {
					const hidden = m?.meta?.hidden ?? false;
					if (hiddenFilter === 'hidden' ? !hidden : hidden) return false;
				}
				if (visibilityFilter !== 'all') {
					const v = getVisibilityState(m);
					if (v !== visibilityFilter) return false;
				}
				return true;
			})
			.sort((a, b) => {
				const ca = (getModelConnectionName(a) ?? a?.owned_by ?? 'Unknown').toString();
				const cb = (getModelConnectionName(b) ?? b?.owned_by ?? 'Unknown').toString();
				const diff = ca.localeCompare(cb);
				if (diff !== 0) return diff;
				return (a?.name ?? a?.id ?? '').localeCompare(b?.name ?? b?.id ?? '');
			});
	} else {
		filteredModels = [];
	}

	$: modelById = (() => {
		const map = new Map<string, any>();
		for (const m of models ?? []) map.set(m.id, m);
		return map;
	})();

	$: {
		// --- Build grouped, groupKeys, rows in one pass to avoid reactive cascade ---
		const _grouped = new Map<string, { name: string; models: any[] }>();
		for (const m of filteredModels) {
			const rawKey = getModelConnectionName(m) || m?.owned_by || 'Unknown';
			const key = rawKey.toString();
			const entry = _grouped.get(key) ?? { name: key, models: [] as any[] };
			entry.models.push(m);
			_grouped.set(key, entry);
		}
		grouped = _grouped;

		const _keys = Array.from(_grouped.keys());
		_keys.sort((a, b) => {
			const da = _grouped.get(a)?.models?.length ?? 0;
			const db = _grouped.get(b)?.models?.length ?? 0;
			if (db !== da) return db - da;
			return String(a).localeCompare(String(b));
		});
		groupKeys = _keys;

		// Auto-expand groups when searching
		if (searchValue.trim()) {
			for (const key of _keys) expandedGroups[key] = true;
			expandedGroups = { ...expandedGroups };
		}

		// Build rows
		const out: Row[] = [];
		for (const key of _keys) {
			const ms = _grouped.get(key)?.models ?? [];
			let enabledCount = 0, hiddenCount = 0, publicCount = 0, privateCount = 0, unsetCount = 0;
			for (const m of ms) {
				if (m?.is_active ?? true) enabledCount++;
				if (m?.meta?.hidden ?? false) hiddenCount++;
				const v = getVisibilityState(m);
				if (v === 'public') publicCount++;
				else if (v === 'private') privateCount++;
				else unsetCount++;
			}
			out.push({ type: 'group', key, name: key, count: ms.length, enabledCount, hiddenCount, publicCount, privateCount, unsetCount });
			if (expandedGroups[key]) {
				for (const m of ms) out.push({ type: 'model', groupKey: key, model: m });
			}
		}
		rows = out;
	}

	// Selection range should follow the list order the user sees — only compute when needed.
	$: visibleModelIds = selectMode ? rows.filter((r) => r.type === 'model').map((r: any) => r.model.id) : [];

	const isSelected = (id: string) => selectedIds.has(id);

	const setSelectedIds = (next: Set<string>) => {
		selectedIds = next;
	};

	const toggleSelect = (id: string, event?: MouseEvent) => {
		const next = new Set(selectedIds);
		const isRange = !!event?.shiftKey && lastSelectedId && visibleModelIds.includes(lastSelectedId);

		if (isRange) {
			const a = visibleModelIds.indexOf(lastSelectedId as string);
			const b = visibleModelIds.indexOf(id);
			const [from, to] = a < b ? [a, b] : [b, a];
			for (let i = from; i <= to; i++) next.add(visibleModelIds[i]);
		} else {
			if (next.has(id)) next.delete(id);
			else next.add(id);
		}

		lastSelectedId = id;
		setSelectedIds(next);
	};

	const selectAllFiltered = () => {
		const next = new Set(selectedIds);
		for (const m of filteredModels) next.add(m.id);
		setSelectedIds(next);
	};

	const clearSelection = () => {
		setSelectedIds(new Set());
		lastSelectedId = null;
	};

	const bulkMenuItemClass =
		'flex gap-2 items-center px-3 py-2 text-sm font-medium cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 rounded-md';

	const toolbarSelectClass =
		'text-xs text-gray-700 dark:text-gray-200 rounded-xl hover:bg-gray-100 dark:hover:bg-gray-850 transition';

	const iconButtonClass =
		'p-1.5 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg transition text-gray-500 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-100';

	const getGroupModelIds = (groupKey: string): string[] => {
		return (grouped.get(groupKey)?.models ?? []).map((m) => m.id);
	};

	const getSelectedCountInGroup = (groupKey: string) => {
		let count = 0;
		for (const id of getGroupModelIds(groupKey)) {
			if (selectedIds.has(id)) count += 1;
		}
		return count;
	};

	const toggleSelectGroup = (groupKey: string) => {
		const ids = getGroupModelIds(groupKey);
		if (!ids.length) return;

		const selectedCount = getSelectedCountInGroup(groupKey);
		const shouldSelectAll = selectedCount !== ids.length;

		const next = new Set(selectedIds);
		if (shouldSelectAll) {
			for (const id of ids) next.add(id);
		} else {
			for (const id of ids) next.delete(id);
		}
		setSelectedIds(next);
	};

	const setListHeightFromContainer = () => {
		if (!listContainerEl) return;
		const rect = listContainerEl.getBoundingClientRect();
		const available = Math.max(320, Math.floor(window.innerHeight - rect.top - 24));
		const fallback = Math.max(320, Math.floor(window.innerHeight * 0.55));
		let nextHeight = Math.max(320, Math.floor(rect.height || 0), available, fallback);
		if (!Number.isFinite(nextHeight)) {
			nextHeight = 600;
		}
		listHeight = nextHeight;
	};

	const downloadModels = async (modelsToSave: any[]) => {
		const blob = new Blob([JSON.stringify(modelsToSave)], { type: 'application/json' });
		saveAs(blob, `models-export-${Date.now()}.json`);
	};

	const exportModelHandler = async (m: any) => {
		const blob = new Blob([JSON.stringify([m])], { type: 'application/json' });
		saveAs(blob, `${m.id}-${Date.now()}.json`);
	};

	const upsertModelHandler = async (m: any) => {
		m.base_model_id = null;
		if (typeof m.access_control === 'undefined') {
			m.access_control = null;
		}

		if ((workspaceModels ?? []).find((wm: any) => wm.id === m.id)) {
			const res = await updateModelById(localStorage.token, m.id, m).catch(() => null);
			if (res) toast.success($i18n.t('Model updated successfully'));
		} else {
			const res = await createNewModel(localStorage.token, {
				meta: {},
				id: m.id,
				name: m.name,
				base_model_id: null,
				params: {},
				access_control: null,
				...m
			}).catch(() => null);
			if (res) toast.success($i18n.t('Model updated successfully'));
		}

		await refreshGlobalModels();
	};

	const bulkApplyPatch = async (patch: any) => {
		if (!selectedIds.size) return;
		if (bulkBusy) return;

		bulkBusy = true;
		try {
			const items = Array.from(selectedIds).map((id) => {
				const m = modelById.get(id);
				return { id, name: m?.name ?? id };
			});

			const res = await bulkUpsertBaseModels(localStorage.token, items, patch);
			const created = res?.created ?? 0;
			const updated = res?.updated ?? 0;

			toast.success(
				$i18n.t('Updated {{updated}} models (created {{created}} overrides)', {
					updated,
					created
				})
			);

			await refreshGlobalModels();
		} catch (e) {
			toast.error($i18n.t('Bulk update failed: {{error}}', { error: String(e) }));
		} finally {
			bulkBusy = false;
		}
	};

	const setModelActive = async (m: any, nextActive: boolean) => {
		if (bulkBusy) return;
		const prev = m?.is_active ?? true;
		m.is_active = nextActive;

		try {
			await bulkUpsertBaseModels(localStorage.token, [{ id: m.id, name: m?.name ?? m.id }], {
				is_active: nextActive
			});
			await refreshGlobalModels();
		} catch (e) {
			m.is_active = prev;
			toast.error($i18n.t('Failed to update model: {{error}}', { error: String(e) }));
		}
	};

	const setModelHidden = async (m: any, nextHidden: boolean) => {
		if (bulkBusy) return;
		const prev = m?.meta?.hidden ?? false;
		m.meta = { ...(m.meta ?? {}), hidden: nextHidden };

		try {
			await bulkUpsertBaseModels(localStorage.token, [{ id: m.id, name: m?.name ?? m.id }], {
				meta: { hidden: nextHidden }
			});
			await refreshGlobalModels();
		} catch (e) {
			m.meta = { ...(m.meta ?? {}), hidden: prev };
			toast.error($i18n.t('Failed to update model: {{error}}', { error: String(e) }));
		}
	};

	const setModelVisibility = async (m: any, next: 'public' | 'private') => {
		if (bulkBusy) return;
		const prev = m?.access_control;
		const access_control = next === 'public' ? null : {};
		m.access_control = access_control;

		try {
			await bulkUpsertBaseModels(localStorage.token, [{ id: m.id, name: m?.name ?? m.id }], {
				access_control
			});
			await refreshGlobalModels();
		} catch (e) {
			m.access_control = prev;
			toast.error($i18n.t('Failed to update model: {{error}}', { error: String(e) }));
		}
	};

	const toggleGroupExpanded = (key: string) => {
		expandedGroups[key] = !expandedGroups[key];
		expandedGroups = { ...expandedGroups };
	};

	const handleEditorSubmit = (m: any) => {
		upsertModelHandler(m);
		selectedModelId = null;
		clearSelection();
	};

	onMount(() => {
		void init();

		const onKeyDown = (event: KeyboardEvent) => {
			if (event.key === 'Shift') shiftKey = true;
			if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'a') {
				event.preventDefault();
				selectAllFiltered();
			}
		};

		const onKeyUp = (event: KeyboardEvent) => {
			if (event.key === 'Shift') shiftKey = false;
		};

		window.addEventListener('keydown', onKeyDown);
		window.addEventListener('keyup', onKeyUp);

		resizeObserver = new ResizeObserver(() => {
			setListHeightFromContainer();
		});

		if (listContainerEl) resizeObserver.observe(listContainerEl);
		void tick().then(() => {
			setListHeightFromContainer();
		});

		return () => {
			window.removeEventListener('keydown', onKeyDown);
			window.removeEventListener('keyup', onKeyUp);
			if (resizeObserver && listContainerEl) resizeObserver.unobserve(listContainerEl);
			resizeObserver = null;
		};
	});
</script>

<svelte:head>
	<title>{$i18n.t('Models')} | {$WEBUI_NAME}</title>
</svelte:head>

<ConfigureModelsModal bind:show={showConfigModal} initHandler={init} />
<ManageModelsModal bind:show={showManageModal} />

<div class="max-w-6xl mx-auto w-full h-full">
	{#if models !== null}
		{#if selectedModelId === null}
			<div class="h-full space-y-6 overflow-y-auto scrollbar-hidden">
				<div class="max-w-6xl mx-auto space-y-6">
					<!-- ==================== Hero Section ==================== -->
					<section class="glass-section p-5 space-y-5">
						<div class="flex flex-col gap-5">
							<div class="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
								<div class="min-w-0">
									<!-- Breadcrumb -->
									<div class="inline-flex h-8 items-center gap-2 whitespace-nowrap rounded-full border border-gray-200/80 bg-white/80 px-3.5 text-xs font-medium leading-none text-gray-600 dark:border-gray-700/80 dark:bg-gray-900/70 dark:text-gray-300">
										<span class="leading-none text-gray-400 dark:text-gray-500">{$i18n.t('Settings')}</span>
										<span class="leading-none text-gray-300 dark:text-gray-600">/</span>
										<span class="leading-none text-gray-900 dark:text-white">{$i18n.t('模型管理')}</span>
									</div>

									<!-- Icon badge + title + description -->
									<div class="mt-3 flex items-start gap-3">
										<div class="glass-icon-badge {activeTabMeta.badgeColor}">
											{#if selectedTab === 'all'}
												<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-[18px] {activeTabMeta.iconColor}">
													<path stroke-linecap="round" stroke-linejoin="round" d="M21 7.5l-9-5.25L3 7.5m18 0l-9 5.25m9-5.25v9l-9 5.25M3 7.5l9 5.25M3 7.5v9l9 5.25m0-9v9" />
												</svg>
											{:else if selectedTab === 'openai'}
												<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="size-[18px] {activeTabMeta.iconColor}">
													<path d="M21.55 10.004a5.416 5.416 0 00-.478-4.501c-1.217-2.09-3.662-3.166-6.05-2.66A5.59 5.59 0 0010.831 1C8.39.995 6.224 2.546 5.473 4.838A5.553 5.553 0 001.76 7.496a5.487 5.487 0 00.691 6.5 5.416 5.416 0 00.477 4.502c1.217 2.09 3.662 3.165 6.05 2.66A5.586 5.586 0 0013.168 23c2.443.006 4.61-1.546 5.361-3.84a5.553 5.553 0 003.715-2.66 5.488 5.488 0 00-.693-6.497v.001zm-8.381 11.558a4.199 4.199 0 01-2.675-.954c.034-.018.093-.05.132-.074l4.44-2.53a.71.71 0 00.364-.623v-6.176l1.877 1.069c.02.01.033.029.036.05v5.115c-.003 2.274-1.87 4.118-4.174 4.123zM4.192 17.78a4.059 4.059 0 01-.498-2.763c.032.02.09.055.131.078l4.44 2.53c.225.13.504.13.73 0l5.42-3.088v2.138a.068.068 0 01-.027.057L9.9 19.288c-1.999 1.136-4.552.46-5.707-1.51h-.001zM3.023 8.216A4.15 4.15 0 015.198 6.41l-.002.151v5.06a.711.711 0 00.364.624l5.42 3.087-1.876 1.07a.067.067 0 01-.063.005l-4.489-2.559c-1.995-1.14-2.679-3.658-1.53-5.63h.001zm15.417 3.54l-5.42-3.088L14.896 7.6a.067.067 0 01.063-.006l4.489 2.557c1.998 1.14 2.683 3.662 1.529 5.633a4.163 4.163 0 01-2.174 1.807V12.38a.71.71 0 00-.363-.623zm1.867-2.773a6.04 6.04 0 00-.132-.078l-4.44-2.53a.731.731 0 00-.729 0l-5.42 3.088V7.325a.068.068 0 01.027-.057L14.1 4.713c2-1.137 4.555-.46 5.707 1.513.487.833.664 1.809.499 2.757h.001zm-11.741 3.81l-1.877-1.068a.065.065 0 01-.036-.051V6.559c.001-2.277 1.873-4.122 4.181-4.12.976 0 1.92.338 2.671.954-.034.018-.092.05-.131.073l-4.44 2.53a.71.71 0 00-.365.623l-.003 6.173v.002zm1.02-2.168L12 9.25l2.414 1.375v2.75L12 14.75l-2.415-1.375v-2.75z"/>
												</svg>
											{:else if selectedTab === 'gemini'}
												<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="size-[18px] {activeTabMeta.iconColor}">
													<path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 01-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z"/>
													<path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
													<path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l3.66-2.84z"/>
													<path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
												</svg>
											{:else if selectedTab === 'anthropic'}
												<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="currentColor" class="size-[18px] {activeTabMeta.iconColor}">
													<path d="m3.127 10.604 3.135-1.76.053-.153-.053-.085H6.11l-.525-.032-1.791-.048-1.554-.065-1.505-.08-.38-.081L0 7.832l.036-.234.32-.214.455.04 1.009.069 1.513.105 1.097.064 1.626.17h.259l.036-.105-.089-.065-.068-.064-1.566-1.062-1.695-1.121-.887-.646-.48-.327-.243-.306-.104-.67.435-.48.585.04.15.04.593.456 1.267.981 1.654 1.218.242.202.097-.068.012-.049-.109-.181-.9-1.626-.96-1.655-.428-.686-.113-.411a2 2 0 0 1-.068-.484l.496-.674L4.446 0l.662.089.279.242.411.94.666 1.48 1.033 2.014.302.597.162.553.06.17h.105v-.097l.085-1.134.157-1.392.154-1.792.052-.504.25-.605.497-.327.387.186.319.456-.045.294-.19 1.23-.37 1.93-.243 1.29h.142l.161-.16.654-.868 1.097-1.372.484-.545.565-.601.363-.287h.686l.505.751-.226.775-.707.895-.585.759-.839 1.13-.524.904.048.072.125-.012 1.897-.403 1.024-.186 1.223-.21.553.258.06.263-.218.536-1.307.323-1.533.307-2.284.54-.028.02.032.04 1.029.098.44.024h1.077l2.005.15.525.346.315.424-.053.323-.807.411-3.631-.863-.872-.218h-.12v.073l.726.71 1.331 1.202 1.667 1.55.084.383-.214.302-.226-.032-1.464-1.101-.565-.497-1.28-1.077h-.084v.113l.295.432 1.557 2.34.08.718-.112.234-.404.141-.443-.08-.911-1.28-.94-1.44-.759-1.291-.093.053-.448 4.821-.21.246-.484.186-.403-.307-.214-.496.214-.98.258-1.28.21-1.016.19-1.263.112-.42-.008-.028-.092.012-.953 1.307-1.448 1.957-1.146 1.227-.274.109-.477-.247.045-.44.266-.39 1.586-2.018.956-1.25.617-.723-.004-.105h-.036l-4.212 2.736-.75.096-.324-.302.04-.496.154-.162 1.267-.871z"/>
												</svg>
											{:else if selectedTab === 'ollama'}
												<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="size-[18px] {activeTabMeta.iconColor}">
													<path d="M7.905 1.09c.216.085.411.225.588.41.295.306.544.744.734 1.263.191.522.315 1.1.362 1.68a5.054 5.054 0 012.049-.636l.051-.004c.87-.07 1.73.087 2.48.474.101.053.2.11.297.17.05-.569.172-1.134.36-1.644.19-.52.439-.957.733-1.264a1.67 1.67 0 01.589-.41c.257-.1.53-.118.796-.042.401.114.745.368 1.016.737.248.337.434.769.561 1.287.23.934.27 2.163.115 3.645l.053.04.026.019c.757.576 1.284 1.397 1.563 2.35.435 1.487.216 3.155-.534 4.088l-.018.021.002.003c.417.762.67 1.567.724 2.4l.002.03c.064 1.065-.2 2.137-.814 3.19l-.007.01.01.024c.472 1.157.62 2.322.438 3.486l-.006.039a.651.651 0 01-.747.536.648.648 0 01-.54-.742c.167-1.033.01-2.069-.48-3.123a.643.643 0 01.04-.617l.004-.006c.604-.924.854-1.83.8-2.72-.046-.779-.325-1.544-.8-2.273a.644.644 0 01.18-.886l.009-.006c.243-.159.467-.565.58-1.12a4.229 4.229 0 00-.095-1.974c-.205-.7-.58-1.284-1.105-1.683-.595-.454-1.383-.673-2.38-.61a.653.653 0 01-.632-.371c-.314-.665-.772-1.141-1.343-1.436a3.288 3.288 0 00-1.772-.332c-1.245.099-2.343.801-2.67 1.686a.652.652 0 01-.61.425c-1.067.002-1.893.252-2.497.703-.522.39-.878.935-1.066 1.588a4.07 4.07 0 00-.068 1.886c.112.558.331 1.02.582 1.269l.008.007c.212.207.257.53.109.785-.36.622-.629 1.549-.673 2.44-.05 1.018.186 1.902.719 2.536l.016.019a.643.643 0 01.095.69c-.576 1.236-.753 2.252-.562 3.052a.652.652 0 01-1.269.298c-.243-1.018-.078-2.184.473-3.498l.014-.035-.008-.012a4.339 4.339 0 01-.598-1.309l-.005-.019a5.764 5.764 0 01-.177-1.785c.044-.91.278-1.842.622-2.59l.012-.026-.002-.002c-.293-.418-.51-.953-.63-1.545l-.005-.024a5.352 5.352 0 01.093-2.49c.262-.915.777-1.701 1.536-2.269.06-.045.123-.09.186-.132-.159-1.493-.119-2.73.112-3.67.127-.518.314-.95.562-1.287.27-.368.614-.622 1.015-.737.266-.076.54-.059.797.042zm4.116 9.09c.936 0 1.8.313 2.446.855.63.527 1.005 1.235 1.005 1.94 0 .888-.406 1.58-1.133 2.022-.62.375-1.451.557-2.403.557-1.009 0-1.871-.259-2.493-.734-.617-.47-.963-1.13-.963-1.845 0-.707.398-1.417 1.056-1.946.668-.537 1.55-.849 2.485-.849zm0 .896a3.07 3.07 0 00-1.916.65c-.461.37-.722.835-.722 1.25 0 .428.21.829.61 1.134.455.347 1.124.548 1.943.548.799 0 1.473-.147 1.932-.426.463-.28.7-.686.7-1.257 0-.423-.246-.89-.683-1.256-.484-.405-1.14-.643-1.864-.643zm.662 1.21l.004.004c.12.151.095.37-.056.49l-.292.23v.446a.375.375 0 01-.376.373.375.375 0 01-.376-.373v-.46l-.271-.218a.347.347 0 01-.052-.49.353.353 0 01.494-.051l.215.172.22-.174a.353.353 0 01.49.051zm-5.04-1.919c.478 0 .867.39.867.871a.87.87 0 01-.868.871.87.87 0 01-.867-.87.87.87 0 01.867-.872zm8.706 0c.48 0 .868.39.868.871a.87.87 0 01-.868.871.87.87 0 01-.867-.87.87.87 0 01.867-.872zM7.44 2.3l-.003.002a.659.659 0 00-.285.238l-.005.006c-.138.189-.258.467-.348.832-.17.692-.216 1.631-.124 2.782.43-.128.899-.208 1.404-.237l.01-.001.019-.034c.046-.082.095-.161.148-.239.123-.771.022-1.692-.253-2.444-.134-.364-.297-.65-.453-.813a.628.628 0 00-.107-.09L7.44 2.3zm9.174.04l-.002.001a.628.628 0 00-.107.09c-.156.163-.32.45-.453.814-.29.794-.387 1.776-.23 2.572l.058.097.008.014h.03a5.184 5.184 0 011.466.212c.086-1.124.038-2.043-.128-2.722-.09-.365-.21-.643-.349-.832l-.004-.006a.659.659 0 00-.285-.239h-.004z"/>
												</svg>
											{:else}
												<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-[18px] {activeTabMeta.iconColor}">
													<path stroke-linecap="round" stroke-linejoin="round" d="M14.25 6.087c0-.355.186-.676.401-.959.221-.29.349-.634.349-1.003 0-1.036-1.007-1.875-2.25-1.875s-2.25.84-2.25 1.875c0 .369.128.713.349 1.003.215.283.401.604.401.959v0a.64.64 0 0 1-.657.643 48.491 48.491 0 0 1-4.163-.3c.186 1.613.293 3.25.315 4.907a.656.656 0 0 1-.658.663v0c-.355 0-.676-.186-.959-.401a1.647 1.647 0 0 0-1.003-.349c-1.036 0-1.875 1.007-1.875 2.25s.84 2.25 1.875 2.25c.369 0 .713-.128 1.003-.349.283-.215.604-.401.959-.401v0c.31 0 .555.26.532.57a48.039 48.039 0 0 1-.642 5.056c1.518.19 3.058.309 4.616.354a.64.64 0 0 0 .657-.643v0c0-.355-.186-.676-.401-.959a1.647 1.647 0 0 1-.349-1.003c0-1.035 1.008-1.875 2.25-1.875 1.243 0 2.25.84 2.25 1.875 0 .369-.128.713-.349 1.003-.215.283-.4.604-.4.959v0c0 .333.277.599.61.58a48.1 48.1 0 0 0 5.427-.63 48.05 48.05 0 0 0 .582-4.717.532.532 0 0 0-.533-.57v0c-.355 0-.676.186-.959.401-.29.221-.634.349-1.003.349-1.035 0-1.875-1.007-1.875-2.25s.84-2.25 1.875-2.25c.37 0 .713.128 1.003.349.283.215.604.401.96.401v0a.656.656 0 0 0 .658-.663 48.422 48.422 0 0 0-.37-5.36c-1.886.342-3.81.574-5.766.689a.578.578 0 0 1-.61-.58v0Z" />
												</svg>
											{/if}
										</div>
										<div class="min-w-0">
											<div class="flex items-center gap-3">
												<div class="text-base font-semibold text-gray-800 dark:text-gray-100">
													{$i18n.t(activeTabMeta.label)}
												</div>
												<span class="text-sm font-medium text-gray-400 dark:text-gray-500">{filteredModels.length}</span>
											</div>
											<p class="mt-1 text-xs text-gray-400 dark:text-gray-500">
												{$i18n.t(activeTabMeta.description)}
											</p>
										</div>
									</div>
								</div>

							</div>
						</div>

						<!-- Tab buttons -->
						<div class="grid w-full grid-cols-2 rounded-2xl bg-gray-100 p-1 dark:bg-gray-850 md:inline-flex md:w-fit md:flex-col lg:flex-row">
							{#each visibleTabs as tab, index}
								<button
									type="button"
									class={`flex min-w-0 items-center gap-2 rounded-xl px-4 py-2 text-sm font-medium transition-all ${shouldSpanModelTabFullRowOnMobile(index) ? 'col-span-2 md:col-span-1' : ''} ${selectedTab === tab ? 'bg-white text-gray-900 shadow-sm dark:bg-gray-800 dark:text-white' : 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'}`}
									on:click={() => { selectedTab = tab; }}
								>
									{#if tab === 'all'}
										<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-4">
											<path stroke-linecap="round" stroke-linejoin="round" d="M21 7.5l-9-5.25L3 7.5m18 0l-9 5.25m9-5.25v9l-9 5.25M3 7.5l9 5.25M3 7.5v9l9 5.25m0-9v9" />
										</svg>
									{:else if tab === 'openai'}
										<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="size-4">
											<path d="M21.55 10.004a5.416 5.416 0 00-.478-4.501c-1.217-2.09-3.662-3.166-6.05-2.66A5.59 5.59 0 0010.831 1C8.39.995 6.224 2.546 5.473 4.838A5.553 5.553 0 001.76 7.496a5.487 5.487 0 00.691 6.5 5.416 5.416 0 00.477 4.502c1.217 2.09 3.662 3.165 6.05 2.66A5.586 5.586 0 0013.168 23c2.443.006 4.61-1.546 5.361-3.84a5.553 5.553 0 003.715-2.66 5.488 5.488 0 00-.693-6.497v.001zm-8.381 11.558a4.199 4.199 0 01-2.675-.954c.034-.018.093-.05.132-.074l4.44-2.53a.71.71 0 00.364-.623v-6.176l1.877 1.069c.02.01.033.029.036.05v5.115c-.003 2.274-1.87 4.118-4.174 4.123zM4.192 17.78a4.059 4.059 0 01-.498-2.763c.032.02.09.055.131.078l4.44 2.53c.225.13.504.13.73 0l5.42-3.088v2.138a.068.068 0 01-.027.057L9.9 19.288c-1.999 1.136-4.552.46-5.707-1.51h-.001zM3.023 8.216A4.15 4.15 0 015.198 6.41l-.002.151v5.06a.711.711 0 00.364.624l5.42 3.087-1.876 1.07a.067.067 0 01-.063.005l-4.489-2.559c-1.995-1.14-2.679-3.658-1.53-5.63h.001zm15.417 3.54l-5.42-3.088L14.896 7.6a.067.067 0 01.063-.006l4.489 2.557c1.998 1.14 2.683 3.662 1.529 5.633a4.163 4.163 0 01-2.174 1.807V12.38a.71.71 0 00-.363-.623zm1.867-2.773a6.04 6.04 0 00-.132-.078l-4.44-2.53a.731.731 0 00-.729 0l-5.42 3.088V7.325a.068.068 0 01.027-.057L14.1 4.713c2-1.137 4.555-.46 5.707 1.513.487.833.664 1.809.499 2.757h.001zm-11.741 3.81l-1.877-1.068a.065.065 0 01-.036-.051V6.559c.001-2.277 1.873-4.122 4.181-4.12.976 0 1.92.338 2.671.954-.034.018-.092.05-.131.073l-4.44 2.53a.71.71 0 00-.365.623l-.003 6.173v.002zm1.02-2.168L12 9.25l2.414 1.375v2.75L12 14.75l-2.415-1.375v-2.75z"/>
										</svg>
									{:else if tab === 'gemini'}
										<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="size-4">
											<path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 01-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z"/>
											<path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
											<path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l3.66-2.84z"/>
											<path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
										</svg>
									{:else if tab === 'anthropic'}
										<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="currentColor" class="size-4">
											<path d="m3.127 10.604 3.135-1.76.053-.153-.053-.085H6.11l-.525-.032-1.791-.048-1.554-.065-1.505-.08-.38-.081L0 7.832l.036-.234.32-.214.455.04 1.009.069 1.513.105 1.097.064 1.626.17h.259l.036-.105-.089-.065-.068-.064-1.566-1.062-1.695-1.121-.887-.646-.48-.327-.243-.306-.104-.67.435-.48.585.04.15.04.593.456 1.267.981 1.654 1.218.242.202.097-.068.012-.049-.109-.181-.9-1.626-.96-1.655-.428-.686-.113-.411a2 2 0 0 1-.068-.484l.496-.674L4.446 0l.662.089.279.242.411.94.666 1.48 1.033 2.014.302.597.162.553.06.17h.105v-.097l.085-1.134.157-1.392.154-1.792.052-.504.25-.605.497-.327.387.186.319.456-.045.294-.19 1.23-.37 1.93-.243 1.29h.142l.161-.16.654-.868 1.097-1.372.484-.545.565-.601.363-.287h.686l.505.751-.226.775-.707.895-.585.759-.839 1.13-.524.904.048.072.125-.012 1.897-.403 1.024-.186 1.223-.21.553.258.06.263-.218.536-1.307.323-1.533.307-2.284.54-.028.02.032.04 1.029.098.44.024h1.077l2.005.15.525.346.315.424-.053.323-.807.411-3.631-.863-.872-.218h-.12v.073l.726.71 1.331 1.202 1.667 1.55.084.383-.214.302-.226-.032-1.464-1.101-.565-.497-1.28-1.077h-.084v.113l.295.432 1.557 2.34.08.718-.112.234-.404.141-.443-.08-.911-1.28-.94-1.44-.759-1.291-.093.053-.448 4.821-.21.246-.484.186-.403-.307-.214-.496.214-.98.258-1.28.21-1.016.19-1.263.112-.42-.008-.028-.092.012-.953 1.307-1.448 1.957-1.146 1.227-.274.109-.477-.247.045-.44.266-.39 1.586-2.018.956-1.25.617-.723-.004-.105h-.036l-4.212 2.736-.75.096-.324-.302.04-.496.154-.162 1.267-.871z"/>
										</svg>
									{:else if tab === 'ollama'}
										<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="size-4">
											<path d="M7.905 1.09c.216.085.411.225.588.41.295.306.544.744.734 1.263.191.522.315 1.1.362 1.68a5.054 5.054 0 012.049-.636l.051-.004c.87-.07 1.73.087 2.48.474.101.053.2.11.297.17.05-.569.172-1.134.36-1.644.19-.52.439-.957.733-1.264a1.67 1.67 0 01.589-.41c.257-.1.53-.118.796-.042.401.114.745.368 1.016.737.248.337.434.769.561 1.287.23.934.27 2.163.115 3.645l.053.04.026.019c.757.576 1.284 1.397 1.563 2.35.435 1.487.216 3.155-.534 4.088l-.018.021.002.003c.417.762.67 1.567.724 2.4l.002.03c.064 1.065-.2 2.137-.814 3.19l-.007.01.01.024c.472 1.157.62 2.322.438 3.486l-.006.039a.651.651 0 01-.747.536.648.648 0 01-.54-.742c.167-1.033.01-2.069-.48-3.123a.643.643 0 01.04-.617l.004-.006c.604-.924.854-1.83.8-2.72-.046-.779-.325-1.544-.8-2.273a.644.644 0 01.18-.886l.009-.006c.243-.159.467-.565.58-1.12a4.229 4.229 0 00-.095-1.974c-.205-.7-.58-1.284-1.105-1.683-.595-.454-1.383-.673-2.38-.61a.653.653 0 01-.632-.371c-.314-.665-.772-1.141-1.343-1.436a3.288 3.288 0 00-1.772-.332c-1.245.099-2.343.801-2.67 1.686a.652.652 0 01-.61.425c-1.067.002-1.893.252-2.497.703-.522.39-.878.935-1.066 1.588a4.07 4.07 0 00-.068 1.886c.112.558.331 1.02.582 1.269l.008.007c.212.207.257.53.109.785-.36.622-.629 1.549-.673 2.44-.05 1.018.186 1.902.719 2.536l.016.019a.643.643 0 01.095.69c-.576 1.236-.753 2.252-.562 3.052a.652.652 0 01-1.269.298c-.243-1.018-.078-2.184.473-3.498l.014-.035-.008-.012a4.339 4.339 0 01-.598-1.309l-.005-.019a5.764 5.764 0 01-.177-1.785c.044-.91.278-1.842.622-2.59l.012-.026-.002-.002c-.293-.418-.51-.953-.63-1.545l-.005-.024a5.352 5.352 0 01.093-2.49c.262-.915.777-1.701 1.536-2.269.06-.045.123-.09.186-.132-.159-1.493-.119-2.73.112-3.67.127-.518.314-.95.562-1.287.27-.368.614-.622 1.015-.737.266-.076.54-.059.797.042zm4.116 9.09c.936 0 1.8.313 2.446.855.63.527 1.005 1.235 1.005 1.94 0 .888-.406 1.58-1.133 2.022-.62.375-1.451.557-2.403.557-1.009 0-1.871-.259-2.493-.734-.617-.47-.963-1.13-.963-1.845 0-.707.398-1.417 1.056-1.946.668-.537 1.55-.849 2.485-.849zm0 .896a3.07 3.07 0 00-1.916.65c-.461.37-.722.835-.722 1.25 0 .428.21.829.61 1.134.455.347 1.124.548 1.943.548.799 0 1.473-.147 1.932-.426.463-.28.7-.686.7-1.257 0-.423-.246-.89-.683-1.256-.484-.405-1.14-.643-1.864-.643zm.662 1.21l.004.004c.12.151.095.37-.056.49l-.292.23v.446a.375.375 0 01-.376.373.375.375 0 01-.376-.373v-.46l-.271-.218a.347.347 0 01-.052-.49.353.353 0 01.494-.051l.215.172.22-.174a.353.353 0 01.49.051zm-5.04-1.919c.478 0 .867.39.867.871a.87.87 0 01-.868.871.87.87 0 01-.867-.87.87.87 0 01.867-.872zm8.706 0c.48 0 .868.39.868.871a.87.87 0 01-.868.871.87.87 0 01-.867-.87.87.87 0 01.867-.872zM7.44 2.3l-.003.002a.659.659 0 00-.285.238l-.005.006c-.138.189-.258.467-.348.832-.17.692-.216 1.631-.124 2.782.43-.128.899-.208 1.404-.237l.01-.001.019-.034c.046-.082.095-.161.148-.239.123-.771.022-1.692-.253-2.444-.134-.364-.297-.65-.453-.813a.628.628 0 00-.107-.09L7.44 2.3zm9.174.04l-.002.001a.628.628 0 00-.107.09c-.156.163-.32.45-.453.814-.29.794-.387 1.776-.23 2.572l.058.097.008.014h.03a5.184 5.184 0 011.466.212c.086-1.124.038-2.043-.128-2.722-.09-.365-.21-.643-.349-.832l-.004-.006a.659.659 0 00-.285-.239h-.004z"/>
										</svg>
									{:else}
										<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-4">
											<path stroke-linecap="round" stroke-linejoin="round" d="M14.25 6.087c0-.355.186-.676.401-.959.221-.29.349-.634.349-1.003 0-1.036-1.007-1.875-2.25-1.875s-2.25.84-2.25 1.875c0 .369.128.713.349 1.003.215.283.401.604.401.959v0a.64.64 0 0 1-.657.643 48.491 48.491 0 0 1-4.163-.3c.186 1.613.293 3.25.315 4.907a.656.656 0 0 1-.658.663v0c-.355 0-.676-.186-.959-.401a1.647 1.647 0 0 0-1.003-.349c-1.036 0-1.875 1.007-1.875 2.25s.84 2.25 1.875 2.25c.369 0 .713-.128 1.003-.349.283-.215.604-.401.959-.401v0c.31 0 .555.26.532.57a48.039 48.039 0 0 1-.642 5.056c1.518.19 3.058.309 4.616.354a.64.64 0 0 0 .657-.643v0c0-.355-.186-.676-.401-.959a1.647 1.647 0 0 1-.349-1.003c0-1.035 1.008-1.875 2.25-1.875 1.243 0 2.25.84 2.25 1.875 0 .369-.128.713-.349 1.003-.215.283-.4.604-.4.959v0c0 .333.277.599.61.58a48.1 48.1 0 0 0 5.427-.63 48.05 48.05 0 0 0 .582-4.717.532.532 0 0 0-.533-.57v0c-.355 0-.676.186-.959.401-.29.221-.634.349-1.003.349-1.035 0-1.875-1.007-1.875-2.25s.84-2.25 1.875-2.25c.37 0 .713.128 1.003.349.283.215.604.401.96.401v0a.656.656 0 0 0 .658-.663 48.422 48.422 0 0 0-.37-5.36c-1.886.342-3.81.574-5.766.689a.578.578 0 0 1-.61-.58v0Z" />
										</svg>
									{/if}
									<span class="min-w-0 truncate">{$i18n.t(tabMeta[tab].label)}</span>
									<span class="text-[10px] opacity-60">{tabCounts[tab]}</span>
								</button>
							{/each}
						</div>
					</section>

					<!-- ==================== Search / Filter Toolbar ==================== -->
					<section class="glass-section p-4 space-y-3">
						<!-- Search + Filter button row -->
						<div class="flex items-center gap-2">
							<div
								class="flex flex-1 items-center bg-gray-50 dark:bg-gray-900/40 rounded-xl px-3 py-2 border border-gray-200/60 dark:border-gray-800"
							>
								<div class="self-center mr-2.5 text-gray-500 dark:text-gray-400">
									<Search className="size-4" />
								</div>
								<input
									class="w-full text-sm outline-hidden bg-transparent placeholder:text-gray-400 dark:placeholder:text-gray-500"
									bind:value={searchValue}
									placeholder={$i18n.t('Search Models')}
								/>
							</div>

							<button
								class="flex items-center gap-1.5 px-3 py-2 text-xs font-medium rounded-xl border transition
									{selectMode
										? 'border-blue-300 dark:border-blue-700 bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300'
										: 'border-gray-200/60 dark:border-gray-800 text-gray-500 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800/60'}"
								type="button"
								on:click={() => {
									selectMode = !selectMode;
									if (!selectMode) clearSelection();
								}}
							>
								<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="size-3.5">
									<path fill-rule="evenodd" d="M16.704 4.153a.75.75 0 0 1 .143 1.052l-8 10.5a.75.75 0 0 1-1.127.075l-4.5-4.5a.75.75 0 0 1 1.06-1.06l3.894 3.893 7.48-9.817a.75.75 0 0 1 1.05-.143Z" clip-rule="evenodd" />
								</svg>
								<span>{$i18n.t('Select')}</span>
							</button>

							<button
								class="flex items-center gap-1.5 px-3 py-2 text-xs font-medium rounded-xl border transition
									{filterOpen || activeFilterCount > 0
										? 'border-emerald-300 dark:border-emerald-700 bg-emerald-50 dark:bg-emerald-900/20 text-emerald-700 dark:text-emerald-300'
										: 'border-gray-200/60 dark:border-gray-800 text-gray-500 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800/60'}"
								type="button"
								on:click={() => (filterOpen = !filterOpen)}
							>
								<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="size-3.5">
									<path fill-rule="evenodd" d="M2.628 1.601C5.028 1.206 7.49 1 10 1s4.973.206 7.372.601a.75.75 0 0 1 .628.74v2.288a2.25 2.25 0 0 1-.659 1.59l-4.682 4.683a2.25 2.25 0 0 0-.659 1.59v3.037c0 .684-.31 1.33-.844 1.757l-1.937 1.55A.75.75 0 0 1 8 18.25v-5.757a2.25 2.25 0 0 0-.659-1.591L2.659 6.22A2.25 2.25 0 0 1 2 4.629V2.34a.75.75 0 0 1 .628-.74Z" clip-rule="evenodd" />
								</svg>
								<span>{$i18n.t('Filter')}</span>
								{#if activeFilterCount > 0}
									<span class="min-w-[18px] h-[18px] flex items-center justify-center rounded-full bg-emerald-100 dark:bg-emerald-900/40 text-emerald-700 dark:text-emerald-300 text-[10px] font-semibold leading-none">
										{activeFilterCount}
									</span>
								{/if}
							</button>
						</div>

						<!-- Collapsible filter panel -->
						{#if filterOpen}
							<div class="rounded-xl border border-gray-200/60 dark:border-gray-800 bg-gray-50/50 dark:bg-gray-900/30 p-3 space-y-2.5">
								<!-- Status -->
								<div class="flex items-center gap-2.5">
									<span class="text-[11px] font-medium text-gray-400 dark:text-gray-500 w-14 shrink-0">{$i18n.t('Status')}</span>
									<div class="flex flex-wrap gap-1">
										{#each [
											{ value: 'all', label: $i18n.t('All') },
											{ value: 'enabled', label: $i18n.t('Enabled') },
											{ value: 'disabled', label: $i18n.t('Disabled') }
										] as opt}
											<button
												class="px-2.5 py-1 text-xs rounded-lg transition {enabledFilter === opt.value
													? 'bg-white dark:bg-gray-700 font-medium text-gray-900 dark:text-white shadow-sm'
													: 'text-gray-500 dark:text-gray-400 hover:bg-white/60 dark:hover:bg-gray-800/60'}"
												type="button"
												on:click={() => (enabledFilter = opt.value)}
											>
												{opt.label}
											</button>
										{/each}
									</div>
								</div>
								<!-- Hidden -->
								<div class="flex items-center gap-2.5">
									<span class="text-[11px] font-medium text-gray-400 dark:text-gray-500 w-14 shrink-0">{$i18n.t('Hidden')}</span>
									<div class="flex flex-wrap gap-1">
										{#each [
											{ value: 'all', label: $i18n.t('All') },
											{ value: 'visible', label: $i18n.t('Visible') },
											{ value: 'hidden', label: $i18n.t('Hidden') }
										] as opt}
											<button
												class="px-2.5 py-1 text-xs rounded-lg transition {hiddenFilter === opt.value
													? 'bg-white dark:bg-gray-700 font-medium text-gray-900 dark:text-white shadow-sm'
													: 'text-gray-500 dark:text-gray-400 hover:bg-white/60 dark:hover:bg-gray-800/60'}"
												type="button"
												on:click={() => (hiddenFilter = opt.value)}
											>
												{opt.label}
											</button>
										{/each}
									</div>
								</div>
								<!-- Visibility -->
								<div class="flex items-center gap-2.5">
									<span class="text-[11px] font-medium text-gray-400 dark:text-gray-500 w-14 shrink-0">{$i18n.t('Visibility')}</span>
									<div class="flex flex-wrap gap-1">
										{#each [
											{ value: 'all', label: $i18n.t('All') },
											{ value: 'public', label: $i18n.t('Public') },
											{ value: 'private', label: $i18n.t('Private') },
											{ value: 'unset', label: $i18n.t('Default') }
										] as opt}
											<button
												class="px-2.5 py-1 text-xs rounded-lg transition {visibilityFilter === opt.value
													? 'bg-white dark:bg-gray-700 font-medium text-gray-900 dark:text-white shadow-sm'
													: 'text-gray-500 dark:text-gray-400 hover:bg-white/60 dark:hover:bg-gray-800/60'}"
												type="button"
												on:click={() => (visibilityFilter = opt.value)}
											>
												{opt.label}
											</button>
										{/each}
									</div>
								</div>

								<!-- Footer: count + clear -->
								{#if activeFilterCount > 0}
									<div class="flex items-center justify-between pt-1.5 border-t border-gray-200/60 dark:border-gray-700/50">
										<span class="text-[11px] text-gray-400 dark:text-gray-500">
											{$i18n.t('Showing {{count}} of {{total}} models', { count: filteredModels.length, total: tabFilteredModels.length })}
										</span>
										<button
											class="text-[11px] text-emerald-600 dark:text-emerald-400 hover:underline"
											type="button"
											on:click={() => {
												enabledFilter = 'all';
												hiddenFilter = 'all';
												visibilityFilter = 'all';
											}}
										>
											{$i18n.t('Clear')} {$i18n.t('Filter')}
										</button>
									</div>
								{/if}
							</div>
						{/if}

						{#if selectMode && selectedIds.size > 0}
							<div
								class="flex items-center justify-between gap-2 px-3 py-1.5 rounded-xl bg-emerald-50/60 dark:bg-emerald-900/10 border border-emerald-200/40 dark:border-emerald-800/30 sticky top-0 z-10"
							>
								<div class="flex items-center gap-2.5">
									<span class="text-sm font-medium text-emerald-800 dark:text-emerald-200">
										{$i18n.t('Selected: {{count}}', { count: selectedIds.size })}
									</span>
									<button
										class="text-xs text-emerald-600 dark:text-emerald-400 hover:underline"
										type="button"
										on:click={() => selectAllFiltered()}
									>
										{$i18n.t('Select All')} ({filteredModels.length})
									</button>
									<span class="text-xs text-gray-300 dark:text-gray-600">&middot;</span>
									<button
										class="text-xs text-gray-500 hover:text-gray-700 dark:hover:text-gray-300 transition"
										type="button"
										on:click={() => clearSelection()}
									>
										{$i18n.t('Clear')}
									</button>
								</div>

								<div class="flex items-center gap-1">
									<!-- Inline quick actions: Enable / Disable -->
									<Tooltip content={$i18n.t('Enable')}>
										<button
											class="{iconButtonClass} {bulkBusy ? 'opacity-50 pointer-events-none' : ''}"
											type="button"
											on:click={() => bulkApplyPatch({ is_active: true })}
										>
											<Check className="size-4" />
										</button>
									</Tooltip>
									<Tooltip content={$i18n.t('Disable')}>
										<button
											class="{iconButtonClass} {bulkBusy ? 'opacity-50 pointer-events-none' : ''}"
											type="button"
											on:click={() => bulkApplyPatch({ is_active: false })}
										>
											<Minus className="size-4" />
										</button>
									</Tooltip>

									<div class="w-px h-4 bg-gray-200 dark:bg-gray-700 mx-0.5" />

									<!-- More actions dropdown -->
									<Dropdown bind:show={bulkMenuOpen}>
										<button
											class="{iconButtonClass} {bulkBusy ? 'opacity-50 pointer-events-none' : ''}"
											type="button"
											disabled={bulkBusy}
										>
											<EllipsisHorizontal className="size-5" />
										</button>

										<div slot="content">
											<DropdownMenu.Content
												class="w-[200px] rounded-xl px-1 py-1.5 border border-gray-300/30 dark:border-gray-700/50 z-50 bg-white dark:bg-gray-850 dark:text-white shadow-lg"
												sideOffset={8}
												side="bottom"
												align="end"
											>
												<!-- Visibility section -->
												<div class="px-3 py-1 text-[10px] font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500">
													{$i18n.t('Visibility')}
												</div>
												<DropdownMenu.Item
													class={`${bulkMenuItemClass} ${bulkBusy ? 'opacity-50 pointer-events-none' : ''}`}
													on:click={() => {
														bulkMenuOpen = false;
														bulkApplyPatch({ meta: { hidden: false } });
													}}
												>
													<Eye className="size-4 text-gray-400" />
													<span>{$i18n.t('Show')}</span>
												</DropdownMenu.Item>
												<DropdownMenu.Item
													class={`${bulkMenuItemClass} ${bulkBusy ? 'opacity-50 pointer-events-none' : ''}`}
													on:click={() => {
														bulkMenuOpen = false;
														bulkApplyPatch({ meta: { hidden: true } });
													}}
												>
													<EyeSlash className="size-4 text-gray-400" />
													<span>{$i18n.t('Hide')}</span>
												</DropdownMenu.Item>

												<DropdownMenu.Separator class="my-1 h-px bg-gray-200 dark:bg-gray-700/60" />

												<!-- Access section -->
												<div class="px-3 py-1 text-[10px] font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500">
													{$i18n.t('Access')}
												</div>
												<DropdownMenu.Item
													class={`${bulkMenuItemClass} ${bulkBusy ? 'opacity-50 pointer-events-none' : ''}`}
													on:click={() => {
														bulkMenuOpen = false;
														bulkApplyPatch({ access_control: null });
													}}
												>
													<GlobeAlt className="size-4 text-gray-400" />
													<span>{$i18n.t('Public')}</span>
												</DropdownMenu.Item>
												<DropdownMenu.Item
													class={`${bulkMenuItemClass} ${bulkBusy ? 'opacity-50 pointer-events-none' : ''}`}
													on:click={() => {
														bulkMenuOpen = false;
														bulkApplyPatch({ access_control: {} });
													}}
												>
													<LockClosed className="size-4 text-gray-400" />
													<span>{$i18n.t('Private')}</span>
												</DropdownMenu.Item>

												<DropdownMenu.Separator class="my-1 h-px bg-gray-200 dark:bg-gray-700/60" />

												<DropdownMenu.Item
													class={`${bulkMenuItemClass} ${bulkBusy ? 'opacity-50 pointer-events-none' : ''}`}
													on:click={() => {
														bulkMenuOpen = false;
														downloadModels(
															Array.from(selectedIds)
																.map((id) => modelById.get(id))
																.filter(Boolean)
														);
													}}
												>
													<ArrowDownTray className="size-4 text-gray-400" />
													<span>{$i18n.t('Export')}</span>
												</DropdownMenu.Item>
											</DropdownMenu.Content>
										</div>
									</Dropdown>
								</div>
							</div>
						{/if}
					</section>

					<!-- ==================== Model List ==================== -->
					<section class="glass-section p-4">
						<div
							class="min-h-[320px] overflow-y-auto scrollbar-hidden"
							bind:this={listContainerEl}
							id="admin-models-list"
							data-rows={rows.length}
					data-height={listHeight}
				>
					{#if filteredModels.length === 0}
						<div class="flex flex-col items-center justify-center w-full h-24">
							<div class="text-gray-500 dark:text-gray-400 text-xs">
								{$i18n.t('No models found')}
							</div>
						</div>
					{:else}
							<div class="flex flex-col py-1">
								{#each rows as item}
									{#if item.type === 'group'}
										{@const groupModel = (grouped.get(item.key)?.models ?? [])[0]}
										{@const groupIds = getGroupModelIds(item.key)}
										{@const selectedInGroup = groupIds.filter(id => selectedIds.has(id)).length}
										<div class="px-2 py-1.5">
											<div
												class="flex items-center justify-between gap-2 w-full px-3 py-2 rounded-xl border border-gray-200/60 dark:border-gray-800 bg-gray-50/50 dark:bg-gray-900/30 hover:bg-gray-100/60 dark:hover:bg-gray-900/40 transition cursor-pointer"
												on:click={() => toggleGroupExpanded(item.key)}
												role="button"
												tabindex="0"
												on:keydown={(e) => { if (e.key === 'Enter' || e.key === ' ') toggleGroupExpanded(item.key); }}
											>
												<div class="flex items-center gap-2 min-w-0">
													{#if selectMode}
													<!-- svelte-ignore a11y-click-events-have-key-events -->
													<input
														type="checkbox"
														class="accent-emerald-600"
														checked={selectedInGroup === item.count && item.count > 0}
														on:change|stopPropagation={() => toggleSelectGroup(item.key)}
														on:click|stopPropagation
														title={$i18n.t('Select')}
													/>
													{/if}

													{#if groupModel?.connection_icon}
														<ModelIcon
															src={groupModel.connection_icon}
															alt="connection icon"
															className="rounded-xl size-7 shrink-0"
														/>
													{:else}
														<LetterAvatar name={item.name} size="size-7" />
													{/if}

													<span class="font-semibold truncate max-w-[40vw]">
														{item.name === 'Unknown' ? $i18n.t('Unknown') : item.name}
													</span>

													<span
														class="text-xs text-gray-600 dark:text-gray-300 px-2 py-0.5 rounded-lg bg-gray-100/80 dark:bg-gray-800/60 shrink-0"
													>
														{item.count}
													</span>

													{#if selectedInGroup > 0}
														<span
															class="text-xs text-emerald-700 dark:text-emerald-300 px-2 py-0.5 rounded-lg bg-emerald-50 dark:bg-emerald-900/25 shrink-0"
														>
															{$i18n.t('Selected: {{count}}', { count: selectedInGroup })}
														</span>
													{/if}
												</div>

												<div class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400 shrink-0">
													<!-- Only show deviation states -->
													{#if item.enabledCount < item.count}
														<span class="text-amber-600 dark:text-amber-400" title={$i18n.t('Disabled')}>
															{item.count - item.enabledCount} {$i18n.t('Disabled')}
														</span>
													{/if}

													{#if item.hiddenCount > 0}
														<span title={$i18n.t('Hidden')}>
															{item.hiddenCount} {$i18n.t('Hidden')}
														</span>
													{/if}

													{#if item.publicCount > 0 && item.privateCount > 0}
														<span class="text-blue-600 dark:text-blue-400" title={$i18n.t('Public')}>
															{item.publicCount} {$i18n.t('Public')}
														</span>
														<span class="text-purple-600 dark:text-purple-400" title={$i18n.t('Private')}>
															{item.privateCount} {$i18n.t('Private')}
														</span>
													{/if}

													<ChevronDown
														className={`size-4 transition-transform will-change-transform ${expandedGroups[item.key] ? 'rotate-180' : ''}`}
													/>
												</div>
											</div>
										</div>
									{:else}
										{@const m = item.model}
										{@const visibility = getVisibilityState(m)}
											<div
												class="flex items-center justify-between gap-3 px-3 py-2 mx-2 pl-10 rounded-xl hover:bg-black/5 dark:hover:bg-white/5 transition {m
													?.meta?.hidden
													? 'opacity-60'
													: ''}"
											>
												<div class="flex items-center gap-3 min-w-0 flex-1">
													{#if selectMode}
													<input
														type="checkbox"
														class="accent-emerald-600"
														checked={selectedIds.has(m.id)}
														on:click={(e) => toggleSelect(m.id, e)}
													/>
													{/if}

													<ModelIcon
														src={m?.meta?.profile_image_url ?? '/static/favicon.png'}
														alt="model icon"
														className="rounded-xl size-8 shrink-0"
													/>

													<button
														class="min-w-0 text-left flex-1"
														type="button"
														on:click={() => {
															selectedModelId = m.id;
														}}
													>
														<Tooltip
															content={marked.parse(
																!!m?.meta?.description
																	? m?.meta?.description
																	: m?.ollama?.digest
																		? `${m?.ollama?.digest} **(${m?.ollama?.modified_at})**`
																		: m.id
															)}
															className="w-fit"
															placement="top-start"
														>
															<div class="font-semibold line-clamp-1">
																{getModelChatDisplayName(m)}
															</div>
														</Tooltip>
														<div class="text-xs text-gray-500 line-clamp-1">
															{m?.originalId ?? m?.original_id ?? m.id}
														</div>
													</button>
												</div>

												<div class="flex items-center gap-2 shrink-0">
													<button
														class="text-xs px-2 py-1 rounded-lg border border-gray-200/60 dark:border-gray-800 hover:bg-black/5 dark:hover:bg-white/5 transition"
														type="button"
														on:click={() =>
															setModelVisibility(m, visibility === 'public' ? 'private' : 'public')}
													>
														{visibility === 'public'
															? $i18n.t('Public')
															: visibility === 'private'
																? $i18n.t('Private')
																: $i18n.t('Default')}
													</button>

													<Tooltip
														content={(m?.is_active ?? true)
															? $i18n.t('Enabled')
															: $i18n.t('Disabled')}
													>
														<Switch
															state={m?.is_active ?? true}
															on:change={(e) => {
																setModelActive(m, e.detail);
															}}
														/>
													</Tooltip>

													<ModelMenu
														user={$user}
														model={m}
														exportHandler={() => exportModelHandler(m)}
														hideHandler={() => setModelHidden(m, !(m?.meta?.hidden ?? false))}
														onClose={() => {}}
													>
														<button
															class="self-center w-fit text-sm p-1.5 hover:bg-black/5 dark:hover:bg-white/5 rounded-xl"
															type="button"
														>
															<EllipsisHorizontal className="size-5" />
														</button>
													</ModelMenu>
												</div>
											</div>
									{/if}
								{/each}
							</div>
					{/if}
				</div>
					</section>
				</div>
			</div>
		{:else}
				<ModelEditor
					edit
					model={models.find((m) => m.id === selectedModelId)}
					preset={false}
					onSubmit={handleEditorSubmit}
					onBack={async () => {
						suppressRouteSelection = true;
						selectedModelId = null;
						await clearSelectedModelRoute();
						suppressRouteSelection = false;
					}}
				/>
		{/if}
	{:else}
		<div class="h-full w-full flex justify-center items-center">
			<Spinner />
		</div>
	{/if}
</div>
