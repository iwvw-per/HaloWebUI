<script lang="ts">
	import fileSaver from 'file-saver';
	import { getContext } from 'svelte';
	import { goto } from '$app/navigation';
	import { Database, Package, X } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';

	import ArchivedChatsModal from '$lib/components/layout/Sidebar/ArchivedChatsModal.svelte';
	import Spinner from '$lib/components/common/Spinner.svelte';
	import Modal from '$lib/components/common/Modal.svelte';
	import DataManagementStatus from '$lib/components/settings/DataManagementStatus.svelte';
	import { getBackendConfig } from '$lib/apis';
	import { getErrorDetail } from '$lib/apis/response';
	import { chats, config, currentChatPage, scrollPaginationEnabled, user } from '$lib/stores';
	import {
		archiveAllChats,
		deleteAllChats,
		getAllChats,
		getAllUserChats,
		getChatList,
		importChatsBatch
	} from '$lib/apis/chats';
	import { exportConfig, importConfig } from '$lib/apis/configs';
	import {
		downloadDatabase,
		inspectDatabaseMerge,
		inspectDatabaseRestore,
		mergeDatabaseBackup,
		restoreDatabase,
		type DatabaseBackupKind,
		type DatabaseMergePlan
	} from '$lib/apis/utils';
	import { convertOpenAIChats, getImportOrigin } from '$lib/utils';

	const { saveAs } = fileSaver;
	const i18n = getContext('i18n');

	type DataManagementTab = 'chatManagement' | 'backups' | 'dangerZone';
	type ImportMode = 'merge' | 'replace';
	type DatabaseBackupApplyMode = 'merge' | 'restore';
	type OperationKey =
		| 'chatImport'
		| 'chatExport'
		| 'configImport'
		| 'configExport'
		| 'databaseExport'
		| 'databaseMerge'
		| 'databaseRestore'
		| 'allUserChatsExport'
		| 'archiveAll'
		| 'deleteAll';
	type OperationPhase = 'idle' | 'validating' | 'running' | 'success' | 'warning' | 'error';

	type OperationState = {
		phase: OperationPhase;
		title: string;
		detail: string;
		updatedAt: number | null;
	};

	type NormalizedChatImportItem = {
		chat: Record<string, any>;
		meta: Record<string, any>;
		pinned: boolean;
		folder_id: string | null;
		assistant_id: string | null;
		title: string;
	};

	type ChatImportDraft = {
		fileName: string;
		origin: 'openai' | 'webui';
		items: NormalizedChatImportItem[];
		count: number;
		invalidCount: number;
		mode: ImportMode;
		confirmReplace: boolean;
	};

	type ConfigImportDraft = {
		fileName: string;
		payload: Record<string, any>;
		topLevelKeys: string[];
		mode: ImportMode;
		confirmReplace: boolean;
	};

	type DatabaseRestoreInspectingFile = {
		fileName: string;
		fileSize: number;
		kind: DatabaseBackupKind;
		mode: DatabaseBackupApplyMode;
	};

	type DatabaseRestoreDraft = {
		fileName: string;
		fileSize: number;
		kind: DatabaseBackupKind;
		mode: DatabaseBackupApplyMode;
		token: string;
		warnings: string[];
		summary: {
			table_count: number;
			tables_preview: string[];
			has_chat_table: boolean;
			has_config_table: boolean;
			has_user_table?: boolean;
			upload_count?: number;
			upload_bytes?: number;
			missing_upload_count?: number;
			orphan_upload_count?: number;
		};
		merge?: DatabaseMergePlan;
		confirmationPhrase: string;
		confirmMerge: boolean;
		confirmOverwrite: boolean;
	};

	const createInitialOperationState = (): OperationState => ({
		phase: 'idle',
		title: '',
		detail: '',
		updatedAt: null
	});

	const createOperationStateMap = (): Record<OperationKey, OperationState> => ({
		chatImport: createInitialOperationState(),
		chatExport: createInitialOperationState(),
		configImport: createInitialOperationState(),
		configExport: createInitialOperationState(),
		databaseExport: createInitialOperationState(),
		databaseMerge: createInitialOperationState(),
		databaseRestore: createInitialOperationState(),
		allUserChatsExport: createInitialOperationState(),
		archiveAll: createInitialOperationState(),
		deleteAll: createInitialOperationState()
	});

	let operationStates = createOperationStateMap();

	// Historical route note: /settings/chats renders the Data Management page.
	let selectedTab: DataManagementTab = 'chatManagement';
	$: isAdmin = $user?.role === 'admin';
	$: visibleTabs = isAdmin
		? (['chatManagement', 'backups', 'dangerZone'] as DataManagementTab[])
		: (['chatManagement', 'dangerZone'] as DataManagementTab[]);
	$: if (!visibleTabs.includes(selectedTab)) {
		selectedTab = 'chatManagement';
	}

	$: tabMeta = {
		chatManagement: {
			label: $i18n.t('Chat Management'),
			description: `${$i18n.t('Import / Export')} · ${$i18n.t('Chat Archive')}`,
			badgeColor: 'bg-blue-50 dark:bg-blue-950/30',
			iconColor: 'text-blue-500 dark:text-blue-400'
		},
		backups: {
			label: $i18n.t('Database'),
			description: `${$i18n.t('Configuration')} · ${$i18n.t('Backup / Restore')}`,
			badgeColor: 'bg-emerald-50 dark:bg-emerald-950/30',
			iconColor: 'text-emerald-500 dark:text-emerald-400'
		},
		dangerZone: {
			label: $i18n.t('Danger Zone'),
			description: $i18n.t('Delete All Chats'),
			badgeColor: 'bg-red-50 dark:bg-red-950/30',
			iconColor: 'text-red-500 dark:text-red-400'
		}
	} satisfies Record<
		DataManagementTab,
		{ label: string; description: string; badgeColor: string; iconColor: string }
	>;
	$: activeTabMeta = tabMeta[selectedTab];

	$: databaseRestoreSupport = {
		supported: Boolean($config?.features?.database_restore_supported ?? false),
		backend: String($config?.features?.database_backend ?? ''),
		reason: $config?.features?.database_restore_reason ?? null,
		workerCount: Number($config?.features?.worker_count ?? 1)
	};

	let showArchiveConfirm = false;
	let showDeleteConfirm = false;
	let showArchivedChatsModal = false;
	let showDatabaseExportModal = false;
	let showDatabaseRestoreModal = false;
	let databaseBackupApplyMode: DatabaseBackupApplyMode = 'merge';
	let databaseExportKind: DatabaseBackupKind = 'sqlite';

	let chatImportDraft: ChatImportDraft | null = null;
	let configImportDraft: ConfigImportDraft | null = null;
	let databaseRestoreInspectingFile: DatabaseRestoreInspectingFile | null = null;
	let databaseRestoreDraft: DatabaseRestoreDraft | null = null;
	let databaseRestoreKind: DatabaseBackupKind = 'sqlite';

	let chatImportInputElement: HTMLInputElement;
	let configImportInputElement: HTMLInputElement;
	let databaseRestoreInputElement: HTMLInputElement;
	let databaseRestoreInspectionController: AbortController | null = null;

	const btnNeutral =
		'shrink-0 inline-flex items-center justify-center gap-2 h-8 px-4 text-xs font-medium rounded-lg glass-input text-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800/80 active:scale-[0.97] transition-all disabled:cursor-not-allowed disabled:opacity-60';
	const btnWarn =
		'shrink-0 inline-flex items-center justify-center gap-2 h-8 px-4 text-xs font-medium rounded-lg bg-orange-50 hover:bg-orange-100 text-orange-600 dark:bg-orange-950/30 dark:hover:bg-orange-900/40 dark:text-orange-400 border border-orange-200/60 dark:border-orange-800/30 active:scale-[0.97] transition-all disabled:cursor-not-allowed disabled:opacity-60';
	const btnDanger =
		'shrink-0 inline-flex items-center justify-center gap-2 h-8 px-4 text-xs font-medium rounded-lg bg-red-50 hover:bg-red-100 text-red-600 dark:bg-red-950/30 dark:hover:bg-red-900/40 dark:text-red-400 border border-red-200/60 dark:border-red-800/30 active:scale-[0.97] transition-all disabled:cursor-not-allowed disabled:opacity-60';
	const btnSmall =
		'shrink-0 inline-flex items-center justify-center h-7 px-3 text-xs font-medium rounded-md glass-input text-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800/80 active:scale-[0.97] transition-all';
	const btnSmallWarn =
		'shrink-0 inline-flex items-center justify-center h-7 px-3 text-xs font-medium rounded-md bg-orange-50 hover:bg-orange-100 text-orange-600 dark:bg-orange-950/30 dark:hover:bg-orange-900/40 dark:text-orange-400 border border-orange-200/60 dark:border-orange-800/30 active:scale-[0.97] transition-all';
	const btnSmallDanger =
		'shrink-0 inline-flex items-center justify-center h-7 px-3 text-xs font-medium rounded-md bg-red-50 hover:bg-red-100 text-red-600 dark:bg-red-950/30 dark:hover:bg-red-900/40 dark:text-red-400 border border-red-200/60 dark:border-red-800/30 active:scale-[0.97] transition-all';
	const modalCloseButton =
		'flex size-6 shrink-0 items-center justify-center rounded-md text-gray-400 hover:bg-gray-100 hover:text-gray-700 active:scale-[0.97] transition-all dark:hover:bg-gray-800 dark:hover:text-gray-200';
	const modalSecondaryButton =
		'inline-flex h-9 w-24 shrink-0 items-center justify-center rounded-lg border border-gray-200 bg-white text-sm font-medium text-gray-600 transition-all hover:bg-gray-50 active:scale-[0.98] dark:border-gray-800 dark:bg-gray-900 dark:text-gray-300 dark:hover:bg-gray-800';
	const modalPrimaryButton =
		'inline-flex h-9 w-24 shrink-0 items-center justify-center rounded-lg border border-gray-200 bg-gray-50 text-sm font-medium text-gray-700 transition-all hover:bg-gray-100 active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-60 dark:border-gray-800 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700';
	const badgeClass =
		'inline-flex items-center rounded-full border px-2 py-0.5 text-[11px] font-medium';
	const modeButtonBase =
		'inline-flex items-center justify-center h-8 rounded-lg px-3 text-xs font-medium transition-all border';
	const DATABASE_RESTORE_INSPECT_TIMEOUT_MS = 120000;

	const setOperationState = (
		key: OperationKey,
		phase: OperationPhase,
		title: string,
		detail = ''
	) => {
		operationStates = {
			...operationStates,
			[key]: {
				phase,
				title,
				detail,
				updatedAt: Date.now()
			}
		};
	};

	const hasOperationState = (key: OperationKey) => operationStates[key].updatedAt !== null;

	const formatError = (error: unknown, fallback: string) => getErrorDetail(error, fallback);

	const formatBytes = (size: number) => {
		if (!Number.isFinite(size) || size <= 0) {
			return '0 B';
		}

		const units = ['B', 'KB', 'MB', 'GB'];
		const power = Math.min(Math.floor(Math.log(size) / Math.log(1024)), units.length - 1);
		const value = size / 1024 ** power;
		return `${value >= 10 || power === 0 ? value.toFixed(0) : value.toFixed(1)} ${units[power]}`;
	};

	const isRecord = (value: unknown): value is Record<string, any> =>
		value !== null && typeof value === 'object' && !Array.isArray(value);

	const readFileText = async (file: File): Promise<string> => {
		return await new Promise((resolve, reject) => {
			const reader = new FileReader();
			reader.onload = (event) => resolve(String(event.target?.result ?? ''));
			reader.onerror = () => reject(new Error($i18n.t('Failed to read file')));
			reader.readAsText(file);
		});
	};

	const normalizeChatImportItems = (
		raw: unknown
	): {
		items: NormalizedChatImportItem[];
		origin: 'openai' | 'webui';
		invalidCount: number;
	} => {
		if (!Array.isArray(raw)) {
			throw new Error($i18n.t('Invalid JSON'));
		}

		let origin: 'openai' | 'webui' = 'webui';
		if (raw.length > 0) {
			try {
				origin = getImportOrigin(raw) === 'openai' ? 'openai' : 'webui';
			} catch {
				origin = 'webui';
			}
		}

		let sourceItems: any[] = raw;
		let invalidCount = 0;
		if (origin === 'openai') {
			sourceItems = convertOpenAIChats(raw);
			invalidCount = raw.length - sourceItems.length;
		}

		const items = sourceItems
			.map((item) => {
				if (isRecord(item) && isRecord(item.chat)) {
					return {
						chat: item.chat,
						meta: isRecord(item.meta) ? item.meta : {},
						pinned: Boolean(item.pinned),
						folder_id: typeof item.folder_id === 'string' ? item.folder_id : null,
						assistant_id: typeof item.assistant_id === 'string' ? item.assistant_id : null,
						title:
							typeof item.title === 'string'
								? item.title
								: typeof item.chat.title === 'string'
									? item.chat.title
									: 'New Chat'
					} satisfies NormalizedChatImportItem;
				}

				if (isRecord(item)) {
					return {
						chat: item,
						meta: {},
						pinned: false,
						folder_id: null,
						assistant_id: null,
						title: typeof item.title === 'string' ? item.title : 'New Chat'
					} satisfies NormalizedChatImportItem;
				}

				return null;
			})
			.filter(Boolean) as NormalizedChatImportItem[];

		if (origin === 'webui') {
			invalidCount += sourceItems.length - items.length;
		}

		return { items, origin, invalidCount };
	};

	const refreshChatList = async () => {
		currentChatPage.set(1);
		await chats.set(await getChatList(localStorage.token, $currentChatPage));
		scrollPaginationEnabled.set(true);
	};

	const refreshBackendConfig = async () => {
		await config.set(await getBackendConfig());
	};

	const resetChatImportDraft = () => {
		chatImportDraft = null;
	};

	const resetConfigImportDraft = () => {
		configImportDraft = null;
	};

	const resetDatabaseRestoreDraft = () => {
		databaseRestoreInspectingFile = null;
		databaseRestoreDraft = null;
	};

	const getDatabaseOperationKey = (mode: DatabaseBackupApplyMode) =>
		mode === 'merge' ? 'databaseMerge' : 'databaseRestore';

	const cancelDatabaseRestoreInspection = () => {
		const mode = databaseRestoreInspectingFile?.mode ?? databaseBackupApplyMode;
		const operationKey = getDatabaseOperationKey(mode);
		databaseRestoreInspectionController?.abort();
		databaseRestoreInspectionController = null;
		databaseRestoreInspectingFile = null;
		setOperationState(
			operationKey,
			'warning',
			$i18n.t('Database backup inspection cancelled.'),
			$i18n.t(
				mode === 'merge' ? 'No merge import has been performed.' : 'No restore has been performed.'
			)
		);
	};

	const handleArchivedChatsChange = async () => {
		await refreshChatList();
	};

	const handleChatImportFileChange = async (event: Event) => {
		const target = event.currentTarget as HTMLInputElement | null;
		const file = target?.files?.[0];
		if (!file) return;

		setOperationState('chatImport', 'validating', $i18n.t('Checking chat file...'), file.name);

		try {
			const rawContent = JSON.parse(await readFileText(file));
			const { items, origin, invalidCount } = normalizeChatImportItems(rawContent);

			chatImportDraft = {
				fileName: file.name,
				origin,
				items,
				count: items.length,
				invalidCount,
				mode: 'merge',
				confirmReplace: false
			};

			const phase = invalidCount > 0 ? 'warning' : 'success';
			const detail =
				invalidCount > 0
					? `${$i18n.t('Found {{count}} chats in {{fileName}}.', {
							count: items.length,
							fileName: file.name
						})} ${$i18n.t('Skipped {{count}} invalid entries during precheck.', {
							count: invalidCount
						})}`
					: $i18n.t('Found {{count}} chats in {{fileName}}.', {
							count: items.length,
							fileName: file.name
						});

			setOperationState('chatImport', phase, $i18n.t('Chat file is ready to import.'), detail);
		} catch (error) {
			chatImportDraft = null;
			setOperationState(
				'chatImport',
				'error',
				$i18n.t('Failed to inspect chat file.'),
				formatError(error, $i18n.t('Invalid JSON'))
			);
			toast.error(formatError(error, $i18n.t('Invalid JSON')));
		} finally {
			if (target) {
				target.value = '';
			}
		}
	};

	const runChatImport = async () => {
		if (!chatImportDraft) return;

		if (chatImportDraft.mode === 'replace' && !chatImportDraft.confirmReplace) {
			setOperationState(
				'chatImport',
				'warning',
				$i18n.t('Replace mode needs confirmation.'),
				$i18n.t('This will replace all current chats for your account.')
			);
			return;
		}

		const title =
			chatImportDraft.mode === 'replace'
				? $i18n.t('Restoring chats...')
				: $i18n.t('Importing chats...');
		setOperationState(
			'chatImport',
			'running',
			title,
			$i18n.t('{{count}} chats are being processed.', { count: chatImportDraft.count })
		);

		try {
			const response = await importChatsBatch(
				localStorage.token,
				chatImportDraft.items,
				chatImportDraft.mode
			);

			await refreshChatList();

			const detail =
				response.failed > 0
					? $i18n.t('Imported {{imported}} chats and skipped {{failed}} failed items.', {
							imported: response.imported,
							failed: response.failed
						})
					: $i18n.t('Imported {{imported}} chats successfully.', {
							imported: response.imported
						});

			setOperationState(
				'chatImport',
				response.failed > 0 ? 'warning' : 'success',
				$i18n.t('Chat import finished.'),
				detail
			);

			if (response.failed > 0) {
				toast.warning(detail);
			} else {
				toast.success(detail);
			}

			resetChatImportDraft();
		} catch (error) {
			const detail = formatError(error, $i18n.t('Failed to import chats.'));
			setOperationState('chatImport', 'error', $i18n.t('Failed to import chats.'), detail);
			toast.error(detail);
		}
	};

	const exportChats = async () => {
		setOperationState(
			'chatExport',
			'running',
			$i18n.t('Preparing chat export...'),
			$i18n.t('Your chat export file is being generated.')
		);

		try {
			const allChats = await getAllChats(localStorage.token);
			const blob = new Blob([JSON.stringify(allChats)], { type: 'application/json' });
			saveAs(blob, `chat-export-${Date.now()}.json`);

			const detail = $i18n.t('Chat export download started for {{count}} chats.', {
				count: allChats.length
			});
			setOperationState('chatExport', 'success', $i18n.t('Export Chats'), detail);
			toast.info(detail);
		} catch (error) {
			const detail = formatError(error, $i18n.t('Failed to export chats.'));
			setOperationState('chatExport', 'error', $i18n.t('Failed to export chats.'), detail);
			toast.error(detail);
		}
	};

	const handleConfigImportFileChange = async (event: Event) => {
		const target = event.currentTarget as HTMLInputElement | null;
		const file = target?.files?.[0];
		if (!file) return;

		setOperationState(
			'configImport',
			'validating',
			$i18n.t('Checking configuration file...'),
			file.name
		);

		try {
			const payload = JSON.parse(await readFileText(file));
			if (!isRecord(payload)) {
				throw new Error($i18n.t('Invalid JSON'));
			}

			const topLevelKeys = Object.keys(payload);
			configImportDraft = {
				fileName: file.name,
				payload,
				topLevelKeys,
				mode: 'merge',
				confirmReplace: false
			};

			setOperationState(
				'configImport',
				'success',
				$i18n.t('Configuration file is ready to import.'),
				$i18n.t('Found {{count}} top-level config sections.', {
					count: topLevelKeys.length
				})
			);
		} catch (error) {
			configImportDraft = null;
			setOperationState(
				'configImport',
				'error',
				$i18n.t('Failed to inspect configuration file.'),
				formatError(error, $i18n.t('Invalid JSON'))
			);
			toast.error(formatError(error, $i18n.t('Invalid JSON')));
		} finally {
			if (target) {
				target.value = '';
			}
		}
	};

	const runConfigImport = async () => {
		if (!configImportDraft) return;

		if (configImportDraft.mode === 'replace' && !configImportDraft.confirmReplace) {
			setOperationState(
				'configImport',
				'warning',
				$i18n.t('Replace mode needs confirmation.'),
				$i18n.t('This will replace the current application configuration.')
			);
			return;
		}

		setOperationState(
			'configImport',
			'running',
			configImportDraft.mode === 'replace'
				? $i18n.t('Restoring configuration...')
				: $i18n.t('Importing configuration...'),
			configImportDraft.fileName
		);

		try {
			await importConfig(localStorage.token, configImportDraft.payload, configImportDraft.mode);
			await refreshBackendConfig();

			setOperationState(
				'configImport',
				'success',
				$i18n.t('Configuration import finished.'),
				$i18n.t('Config imported successfully')
			);
			toast.success($i18n.t('Config imported successfully'));
			resetConfigImportDraft();
		} catch (error) {
			const detail = formatError(error, $i18n.t('Failed to import configuration.'));
			setOperationState(
				'configImport',
				'error',
				$i18n.t('Failed to import configuration.'),
				detail
			);
			toast.error(detail);
		}
	};

	const exportConfigToFile = async () => {
		setOperationState(
			'configExport',
			'running',
			$i18n.t('Preparing config export...'),
			$i18n.t('Your configuration export file is being generated.')
		);

		try {
			const response = await exportConfig(localStorage.token);
			const configPayload = response?.config ?? response;
			const blob = new Blob([JSON.stringify(configPayload)], { type: 'application/json' });
			saveAs(blob, `config-${Date.now()}.json`);

			const detail = $i18n.t('Configuration export download started.');
			setOperationState('configExport', 'success', $i18n.t('Export Config to JSON File'), detail);
			toast.info(detail);
		} catch (error) {
			const detail = formatError(error, $i18n.t('Failed to export configuration.'));
			setOperationState(
				'configExport',
				'error',
				$i18n.t('Failed to export configuration.'),
				detail
			);
			toast.error(detail);
		}
	};

	const exportAllUserChats = async () => {
		setOperationState(
			'allUserChatsExport',
			'running',
			$i18n.t('Preparing all users chat export...'),
			$i18n.t('The all-users chat export file is being generated.')
		);

		try {
			const exportedChats = await getAllUserChats(localStorage.token);
			const blob = new Blob([JSON.stringify(exportedChats)], {
				type: 'application/json'
			});
			saveAs(blob, `all-chats-export-${Date.now()}.json`);

			const detail = $i18n.t('All users chat export download started for {{count}} chats.', {
				count: exportedChats.length
			});
			setOperationState(
				'allUserChatsExport',
				'success',
				$i18n.t('Export All Chats (All Users)'),
				detail
			);
			toast.info(detail);
		} catch (error) {
			const detail = formatError(error, $i18n.t('Failed to export all users chats.'));
			setOperationState(
				'allUserChatsExport',
				'error',
				$i18n.t('Failed to export all users chats.'),
				detail
			);
			toast.error(detail);
		}
	};

	const getBackupKindLabel = (kind: DatabaseBackupKind) =>
		kind === 'full' ? $i18n.t('Full backup') : $i18n.t('Database only');

	const isDatabaseRestoreFileAllowed = (fileName: string, kind: DatabaseBackupKind) => {
		const normalizedName = fileName.toLowerCase();
		const sqliteExtensions = ['.db', '.sqlite', '.sqlite3', '.backup', '.bak'];
		const fullExtensions = ['.hwbk', '.zip'];
		const extensions = kind === 'full' ? fullExtensions : sqliteExtensions;
		return extensions.some((extension) => normalizedName.endsWith(extension));
	};

	const getDatabaseRestoreKindMismatchMessage = (kind: DatabaseBackupKind) =>
		kind === 'full'
			? $i18n.t('Full backup restore expects a .hwbk or .zip package.')
			: $i18n.t('Database-only restore expects a SQLite backup file.');

	const getDatabaseApplyKindTitle = () => $i18n.t('Import Database Backup');

	const getDatabaseApplyKindDescription = () =>
		$i18n.t('Choose import mode and backup type before selecting a backup file.');

	const getDatabaseApplyKindGuardText = () =>
		databaseBackupApplyMode === 'merge'
			? $i18n.t(
					'If the file type does not match the selected merge import type, the import will be blocked before any data is changed.'
				)
			: $i18n.t(
					'If the file type does not match the selected restore type, the restore will be blocked before any data is changed.'
				);

	const getDatabaseRestoreSummaryDetail = (draft: DatabaseRestoreDraft) => {
		const tableDetail = $i18n.t('Found {{count}} tables in the backup.', {
			count: draft.summary.table_count
		});

		if (draft.kind !== 'full') {
			return tableDetail;
		}

		return `${tableDetail} ${$i18n.t('Includes {{count}} uploaded files ({{size}}).', {
			count: draft.summary.upload_count ?? 0,
			size: formatBytes(draft.summary.upload_bytes ?? 0)
		})}`;
	};

	const getDatabaseMergeSummaryDetail = (draft: DatabaseRestoreDraft) => {
		if (!draft.merge) {
			return getDatabaseRestoreSummaryDetail(draft);
		}

		return $i18n.t(
			'Merge preview: {{insertRows}} new rows, {{existingRows}} existing rows skipped, {{chatMessages}} chat messages added.',
			{
				insertRows: draft.merge.insert_rows,
				existingRows: draft.merge.skip_existing_rows,
				chatMessages: draft.merge.chat_messages_merged
			}
		);
	};

	const getDatabaseApplySummaryDetail = (draft: DatabaseRestoreDraft) =>
		draft.mode === 'merge'
			? getDatabaseMergeSummaryDetail(draft)
			: getDatabaseRestoreSummaryDetail(draft);

	const backupChoiceClass = (active: boolean) =>
		`w-full rounded-lg border p-3 text-left transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-500/20 ${
			active
				? 'border-emerald-200 bg-emerald-50/35 shadow-sm shadow-emerald-100/30 dark:border-emerald-800/60 dark:bg-emerald-950/15 dark:shadow-none'
				: 'border-gray-200 bg-white hover:border-gray-300 hover:bg-gray-50 dark:border-gray-800 dark:bg-gray-900/40 dark:hover:border-gray-700 dark:hover:bg-gray-800/50'
		}`;
	const backupChoiceIconClass = (active: boolean) =>
		`mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-lg border ${
			active
				? 'border-emerald-200 bg-white text-emerald-600 dark:border-emerald-800/70 dark:bg-gray-900 dark:text-emerald-300'
				: 'border-gray-200 bg-gray-50 text-gray-500 dark:border-gray-800 dark:bg-gray-900 dark:text-gray-400'
		}`;
	const backupChoiceMarkClass = (active: boolean) =>
		`mt-1 flex size-4 shrink-0 items-center justify-center rounded-full border ${
			active
				? 'border-emerald-300 bg-white dark:border-emerald-700 dark:bg-gray-900'
				: 'border-gray-300 bg-white dark:border-gray-700 dark:bg-gray-900'
		}`;
	const databaseApplyPanelClass = (mode: DatabaseBackupApplyMode) =>
		`rounded-2xl border p-3 space-y-3 ${
			mode === 'merge'
				? 'border-emerald-200/70 bg-emerald-50/40 dark:border-emerald-800/40 dark:bg-emerald-950/10'
				: 'border-red-200/70 bg-red-50/40 dark:border-red-800/40 dark:bg-red-950/10'
		}`;
	const databaseApplyTextClass = (mode: DatabaseBackupApplyMode) =>
		mode === 'merge' ? 'text-emerald-700 dark:text-emerald-300' : 'text-red-700 dark:text-red-300';
	const databaseApplyNoticeClass = (mode: DatabaseBackupApplyMode) =>
		`rounded-xl border bg-white/80 px-3 py-2 text-xs dark:bg-gray-900/60 ${
			mode === 'merge'
				? 'border-emerald-200/70 text-emerald-700 dark:border-emerald-800/40 dark:text-emerald-300'
				: 'border-red-200/70 text-red-700 dark:border-red-800/40 dark:text-red-300'
		}`;
	const databaseApplyBadgeClass = (mode: DatabaseBackupApplyMode) =>
		mode === 'merge'
			? 'border-emerald-200/70 bg-white/80 text-emerald-700 dark:border-emerald-800/40 dark:bg-gray-900/60 dark:text-emerald-300'
			: 'border-red-200/70 bg-white/80 text-red-700 dark:border-red-800/40 dark:bg-gray-900/60 dark:text-red-300';

	const openDatabaseExportModal = () => {
		databaseExportKind = 'sqlite';
		showDatabaseExportModal = true;
	};

	const confirmDatabaseExport = async () => {
		const kind = databaseExportKind;
		showDatabaseExportModal = false;
		await exportDatabase(kind);
	};

	const exportDatabase = async (kind: DatabaseBackupKind) => {
		setOperationState(
			'databaseExport',
			'running',
			$i18n.t(kind === 'full' ? 'Preparing full backup...' : 'Preparing database backup...'),
			$i18n.t(
				kind === 'full'
					? 'A full backup package is being generated.'
					: 'A SQLite snapshot backup is being generated.'
			)
		);

		try {
			await downloadDatabase(localStorage.token, kind);
			const detail = $i18n.t(
				kind === 'full' ? 'Full backup download started.' : 'Database backup download started.'
			);
			setOperationState(
				'databaseExport',
				'success',
				$i18n.t(kind === 'full' ? 'Export Full Backup' : 'Export Database'),
				detail
			);
			toast.info(detail);
		} catch (error) {
			const detail = formatError(error, $i18n.t('Failed to export database backup.'));
			setOperationState(
				'databaseExport',
				'error',
				$i18n.t('Failed to export database backup.'),
				detail
			);
			toast.error(detail);
		}
	};

	const openDatabaseRestoreModal = (mode: DatabaseBackupApplyMode = 'merge') => {
		databaseBackupApplyMode = mode;
		databaseRestoreKind = 'sqlite';
		showDatabaseRestoreModal = true;
	};

	const confirmDatabaseRestoreKind = () => {
		if (!databaseRestoreSupport.supported) return;
		showDatabaseRestoreModal = false;
		openDatabaseRestorePicker(databaseRestoreKind);
	};

	const openDatabaseRestorePicker = (kind: DatabaseBackupKind) => {
		databaseRestoreKind = kind;
		databaseRestoreInputElement?.click();
	};

	const handleDatabaseRestoreFileChange = async (event: Event) => {
		const target = event.currentTarget as HTMLInputElement | null;
		const file = target?.files?.[0];
		if (!file) return;

		const applyMode = databaseBackupApplyMode;
		const operationKey = getDatabaseOperationKey(applyMode);

		if (!isDatabaseRestoreFileAllowed(file.name, databaseRestoreKind)) {
			const detail = getDatabaseRestoreKindMismatchMessage(databaseRestoreKind);
			resetDatabaseRestoreDraft();
			setOperationState(operationKey, 'error', $i18n.t('Backup type does not match.'), detail);
			toast.error(detail);
			if (target) {
				target.value = '';
			}
			return;
		}

		databaseRestoreInspectionController?.abort();
		resetDatabaseRestoreDraft();
		databaseRestoreInspectingFile = {
			fileName: file.name,
			fileSize: file.size,
			kind: databaseRestoreKind,
			mode: applyMode
		};
		const controller = new AbortController();
		databaseRestoreInspectionController = controller;
		const timeoutId = window.setTimeout(() => {
			controller.abort();
		}, DATABASE_RESTORE_INSPECT_TIMEOUT_MS);

		setOperationState(
			operationKey,
			'validating',
			$i18n.t(
				applyMode === 'merge'
					? 'Inspecting backup for merge import...'
					: 'Inspecting database backup...'
			),
			`${getBackupKindLabel(databaseRestoreKind)} · ${file.name}`
		);

		try {
			const response =
				applyMode === 'merge'
					? await inspectDatabaseMerge(
							localStorage.token,
							file,
							databaseRestoreKind,
							controller.signal
						)
					: await inspectDatabaseRestore(
							localStorage.token,
							file,
							databaseRestoreKind,
							controller.signal
						);
			databaseRestoreInspectingFile = null;
			databaseRestoreDraft = {
				fileName: response.filename,
				fileSize: response.size,
				kind: response.kind,
				mode: applyMode,
				token: response.token,
				warnings: response.warnings ?? [],
				summary: response.summary,
				merge: applyMode === 'merge' ? response.merge : undefined,
				confirmationPhrase: response.confirmation,
				confirmMerge: false,
				confirmOverwrite: false
			};

			setOperationState(
				operationKey,
				response.warnings?.length ? 'warning' : 'success',
				$i18n.t(
					applyMode === 'merge' ? 'Backup is ready to merge import.' : 'Database backup verified.'
				),
				getDatabaseApplySummaryDetail(databaseRestoreDraft)
			);
		} catch (error) {
			if (controller.signal.aborted) {
				if (databaseRestoreInspectionController !== controller) return;

				databaseRestoreInspectingFile = null;
				setOperationState(
					operationKey,
					'warning',
					$i18n.t('Database backup inspection cancelled.'),
					$i18n.t(
						applyMode === 'merge'
							? 'No merge import has been performed.'
							: 'No restore has been performed.'
					)
				);
				return;
			}

			resetDatabaseRestoreDraft();
			const detail = formatError(
				error,
				$i18n.t(
					applyMode === 'merge'
						? 'Failed to inspect backup for merge import.'
						: 'Failed to inspect database backup.'
				)
			);
			setOperationState(
				operationKey,
				'error',
				$i18n.t(
					applyMode === 'merge'
						? 'Failed to inspect backup for merge import.'
						: 'Failed to inspect database backup.'
				),
				detail
			);
			toast.error(detail);
		} finally {
			window.clearTimeout(timeoutId);
			if (databaseRestoreInspectionController === controller) {
				databaseRestoreInspectionController = null;
			}
			if (target) {
				target.value = '';
			}
		}
	};

	const runDatabaseRestore = async () => {
		if (!databaseRestoreDraft) return;
		const operationKey = getDatabaseOperationKey(databaseRestoreDraft.mode);

		if (databaseRestoreDraft.mode === 'merge' && !databaseRestoreDraft.confirmMerge) {
			databaseRestoreDraft = { ...databaseRestoreDraft, confirmMerge: true };
			return;
		}

		if (databaseRestoreDraft.mode === 'restore' && !databaseRestoreDraft.confirmOverwrite) {
			databaseRestoreDraft = { ...databaseRestoreDraft, confirmOverwrite: true };
			return;
		}

		setOperationState(
			operationKey,
			'running',
			$i18n.t(
				databaseRestoreDraft.mode === 'merge' ? 'Merging backup data...' : 'Restoring database...'
			),
			$i18n.t('The page will refresh after the operation finishes.')
		);

		try {
			const payload = {
				token: databaseRestoreDraft.token,
				confirmation: databaseRestoreDraft.confirmationPhrase
			};
			if (databaseRestoreDraft.mode === 'merge') {
				await mergeDatabaseBackup(localStorage.token, payload);
			} else {
				await restoreDatabase(localStorage.token, payload);
			}

			setOperationState(
				operationKey,
				'success',
				$i18n.t(
					databaseRestoreDraft.mode === 'merge'
						? 'Backup merge import finished. Refreshing the page...'
						: 'Database restored. Refreshing the page...'
				),
				$i18n.t('The page will refresh after the operation finishes.')
			);
			toast.success(
				$i18n.t(
					databaseRestoreDraft.mode === 'merge'
						? 'Backup merge import finished. Refreshing the page...'
						: 'Database restored. Refreshing the page...'
				)
			);

			setTimeout(() => {
				if (typeof window !== 'undefined') {
					window.location.reload();
				}
			}, 500);
		} catch (error) {
			const detail = formatError(
				error,
				$i18n.t(
					databaseRestoreDraft.mode === 'merge'
						? 'Failed to merge import backup.'
						: 'Failed to restore database.'
				)
			);
			setOperationState(
				operationKey,
				'error',
				$i18n.t(
					databaseRestoreDraft.mode === 'merge'
						? 'Failed to merge import backup.'
						: 'Failed to restore database.'
				),
				detail
			);
			toast.error(detail);
		}
	};

	const archiveAllChatsHandler = async () => {
		setOperationState(
			'archiveAll',
			'running',
			$i18n.t('Archiving chats...'),
			$i18n.t('All current chats are being moved into the archive.')
		);

		try {
			await goto('/');
			await archiveAllChats(localStorage.token);
			await refreshChatList();

			setOperationState(
				'archiveAll',
				'success',
				$i18n.t('Archive complete.'),
				$i18n.t('All current chats were moved into the archive.')
			);
			toast.success($i18n.t('Archive complete.'));
		} catch (error) {
			const detail = formatError(error, $i18n.t('Failed to archive chats.'));
			setOperationState('archiveAll', 'error', $i18n.t('Failed to archive chats.'), detail);
			toast.error(detail);
		}
	};

	const deleteAllChatsHandler = async () => {
		setOperationState(
			'deleteAll',
			'running',
			$i18n.t('Deleting chats...'),
			$i18n.t('All chats are being permanently deleted.')
		);

		try {
			await goto('/');
			await deleteAllChats(localStorage.token);
			await refreshChatList();

			setOperationState(
				'deleteAll',
				'success',
				$i18n.t('Delete complete.'),
				$i18n.t('All chats were permanently deleted.')
			);
			toast.success($i18n.t('Delete complete.'));
		} catch (error) {
			const detail = formatError(error, $i18n.t('Failed to delete chats.'));
			setOperationState('deleteAll', 'error', $i18n.t('Failed to delete chats.'), detail);
			toast.error(detail);
		}
	};

	const getRestoreUnavailableMessage = () => {
		if (databaseRestoreSupport.reason === 'backend_not_sqlite') {
			return $i18n.t('Database restore is only available for SQLite deployments.');
		}

		if (databaseRestoreSupport.reason === 'multiple_workers_not_supported') {
			return $i18n.t(
				'Database restore requires a single server worker. Current worker count: {{count}}.',
				{
					count: databaseRestoreSupport.workerCount
				}
			);
		}

		return $i18n.t('Database restore is unavailable in this environment.');
	};
</script>

<ArchivedChatsModal bind:show={showArchivedChatsModal} on:change={handleArchivedChatsChange} />

<input
	id="chat-import-input"
	bind:this={chatImportInputElement}
	type="file"
	accept=".json"
	hidden
	on:change={handleChatImportFileChange}
/>

<input
	id="config-json-input"
	bind:this={configImportInputElement}
	type="file"
	accept=".json"
	hidden
	on:change={handleConfigImportFileChange}
/>

<input
	id="database-restore-input"
	bind:this={databaseRestoreInputElement}
	type="file"
	accept=".db,.sqlite,.sqlite3,.backup,.bak,.hwbk,.zip"
	hidden
	on:change={handleDatabaseRestoreFileChange}
/>

<Modal size="sm" bind:show={showDatabaseExportModal}>
	<div class="space-y-5 px-5 py-5">
		<div class="flex items-start justify-between gap-4">
			<div class="min-w-0 space-y-1">
				<div class="text-lg font-semibold text-gray-900 dark:text-gray-100">
					{$i18n.t('Choose export type')}
				</div>
				<div class="text-sm leading-5 text-gray-500 dark:text-gray-400">
					{$i18n.t('Pick what this backup should include before the download starts.')}
				</div>
			</div>
			<button
				class={modalCloseButton}
				type="button"
				aria-label={$i18n.t('Close')}
				on:click={() => (showDatabaseExportModal = false)}
			>
				<X className="size-3.5" strokeWidth={2.4} />
			</button>
		</div>

		<div class="grid gap-2.5">
			<button
				class={backupChoiceClass(databaseExportKind === 'sqlite')}
				type="button"
				on:click={() => (databaseExportKind = 'sqlite')}
			>
				<div class="flex items-start gap-3">
					<div class={backupChoiceIconClass(databaseExportKind === 'sqlite')}>
						<Database className="size-4" strokeWidth={1.8} />
					</div>
					<div class="min-w-0 flex-1">
						<div class="flex items-center justify-between gap-3">
							<div class="text-sm font-semibold text-gray-900 dark:text-gray-100">
								{$i18n.t('Database only')}
							</div>
							<div class={backupChoiceMarkClass(databaseExportKind === 'sqlite')}>
								{#if databaseExportKind === 'sqlite'}
									<span class="size-1.5 rounded-full bg-emerald-500 dark:bg-emerald-400" />
								{/if}
							</div>
						</div>
						<div class="mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400">
							{$i18n.t(
								'Exports the SQLite database only. Chat images and uploaded files are not included.'
							)}
						</div>
					</div>
				</div>
			</button>
			<button
				class={backupChoiceClass(databaseExportKind === 'full')}
				type="button"
				on:click={() => (databaseExportKind = 'full')}
			>
				<div class="flex items-start gap-3">
					<div class={backupChoiceIconClass(databaseExportKind === 'full')}>
						<Package className="size-4" strokeWidth={1.8} />
					</div>
					<div class="min-w-0 flex-1">
						<div class="flex items-center justify-between gap-3">
							<div class="text-sm font-semibold text-gray-900 dark:text-gray-100">
								{$i18n.t('Full backup')}
							</div>
							<div class={backupChoiceMarkClass(databaseExportKind === 'full')}>
								{#if databaseExportKind === 'full'}
									<span class="size-1.5 rounded-full bg-emerald-500 dark:bg-emerald-400" />
								{/if}
							</div>
						</div>
						<div class="mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400">
							{$i18n.t(
								'Exports the database and local uploaded files that are still referenced by the database.'
							)}
						</div>
					</div>
				</div>
			</button>
		</div>

		<div
			class="flex items-center justify-end gap-2 border-t border-gray-100 pt-4 dark:border-gray-800"
		>
			<button
				class={modalSecondaryButton}
				type="button"
				on:click={() => (showDatabaseExportModal = false)}
			>
				{$i18n.t('Cancel')}
			</button>
			<button class={modalPrimaryButton} type="button" on:click={confirmDatabaseExport}>
				{$i18n.t('Export')}
			</button>
		</div>
	</div>
</Modal>

<Modal size="sm" bind:show={showDatabaseRestoreModal}>
	<div class="space-y-5 px-5 py-5">
		<div class="flex items-start justify-between gap-4">
			<div class="min-w-0 space-y-1">
				<div class="text-lg font-semibold text-gray-900 dark:text-gray-100">
					{getDatabaseApplyKindTitle()}
				</div>
				<div class="text-sm leading-5 text-gray-500 dark:text-gray-400">
					{getDatabaseApplyKindDescription()}
				</div>
			</div>
			<button
				class={modalCloseButton}
				type="button"
				aria-label={$i18n.t('Close')}
				on:click={() => (showDatabaseRestoreModal = false)}
			>
				<X className="size-3.5" strokeWidth={2.4} />
			</button>
		</div>

		<div class="space-y-2">
			<div class="text-xs font-medium text-gray-500 dark:text-gray-400">
				{$i18n.t('Import mode')}
			</div>
			<div class="grid gap-2 sm:grid-cols-2">
				<button
					type="button"
					class={`w-full rounded-lg border p-3 text-left transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-500/20 ${databaseBackupApplyMode === 'merge' ? 'border-emerald-200 bg-emerald-50/35 shadow-sm shadow-emerald-100/30 dark:border-emerald-800/60 dark:bg-emerald-950/15 dark:shadow-none' : 'border-gray-200 bg-white hover:border-gray-300 hover:bg-gray-50 dark:border-gray-800 dark:bg-gray-900/40 dark:hover:border-gray-700 dark:hover:bg-gray-800/50'}`}
					on:click={() => (databaseBackupApplyMode = 'merge')}
				>
					<div class="flex items-start justify-between gap-3">
						<div class="min-w-0 space-y-1">
							<div class="text-sm font-semibold text-gray-900 dark:text-gray-100">
								{$i18n.t('Merge import')}
							</div>
							<div class="text-xs leading-5 text-gray-500 dark:text-gray-400">
								{$i18n.t(
									'Add missing data from the backup without overwriting or deleting current data.'
								)}
							</div>
						</div>
						<div class={backupChoiceMarkClass(databaseBackupApplyMode === 'merge')}>
							{#if databaseBackupApplyMode === 'merge'}
								<span class="size-1.5 rounded-full bg-emerald-500 dark:bg-emerald-400" />
							{/if}
						</div>
					</div>
				</button>
				<button
					type="button"
					class={`w-full rounded-lg border p-3 text-left transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-red-500/20 ${databaseBackupApplyMode === 'restore' ? 'border-red-200 bg-red-50/40 shadow-sm shadow-red-100/30 dark:border-red-800/60 dark:bg-red-950/15 dark:shadow-none' : 'border-gray-200 bg-white hover:border-gray-300 hover:bg-gray-50 dark:border-gray-800 dark:bg-gray-900/40 dark:hover:border-gray-700 dark:hover:bg-gray-800/50'}`}
					on:click={() => (databaseBackupApplyMode = 'restore')}
				>
					<div class="flex items-start justify-between gap-3">
						<div class="min-w-0 space-y-1">
							<div class="text-sm font-semibold text-gray-900 dark:text-gray-100">
								{$i18n.t('Full restore')}
							</div>
							<div class="text-xs leading-5 text-gray-500 dark:text-gray-400">
								{$i18n.t(
									'Overwrite current database with the selected backup. Use only when you need a full rollback.'
								)}
							</div>
						</div>
						<div class={backupChoiceMarkClass(databaseBackupApplyMode === 'restore')}>
							{#if databaseBackupApplyMode === 'restore'}
								<span class="size-1.5 rounded-full bg-red-500 dark:bg-red-400" />
							{/if}
						</div>
					</div>
				</button>
			</div>
		</div>

		<div class="space-y-2">
			<div class="text-xs font-medium text-gray-500 dark:text-gray-400">
				{$i18n.t('Backup type')}
			</div>
			<div class="grid gap-2.5">
				<button
					class={backupChoiceClass(databaseRestoreKind === 'sqlite')}
					type="button"
					on:click={() => (databaseRestoreKind = 'sqlite')}
				>
					<div class="flex items-start gap-3">
						<div class={backupChoiceIconClass(databaseRestoreKind === 'sqlite')}>
							<Database className="size-4" strokeWidth={1.8} />
						</div>
						<div class="min-w-0 flex-1">
							<div class="flex items-center justify-between gap-3">
								<div class="text-sm font-semibold text-gray-900 dark:text-gray-100">
									{$i18n.t('Database only')}
								</div>
								<div class={backupChoiceMarkClass(databaseRestoreKind === 'sqlite')}>
									{#if databaseRestoreKind === 'sqlite'}
										<span class="size-1.5 rounded-full bg-emerald-500 dark:bg-emerald-400" />
									{/if}
								</div>
							</div>
							<div class="mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400">
								{$i18n.t('Use this for .db, .sqlite, .sqlite3, .backup, or .bak files.')}
							</div>
						</div>
					</div>
				</button>
				<button
					class={backupChoiceClass(databaseRestoreKind === 'full')}
					type="button"
					on:click={() => (databaseRestoreKind = 'full')}
				>
					<div class="flex items-start gap-3">
						<div class={backupChoiceIconClass(databaseRestoreKind === 'full')}>
							<Package className="size-4" strokeWidth={1.8} />
						</div>
						<div class="min-w-0 flex-1">
							<div class="flex items-center justify-between gap-3">
								<div class="text-sm font-semibold text-gray-900 dark:text-gray-100">
									{$i18n.t('Full backup')}
								</div>
								<div class={backupChoiceMarkClass(databaseRestoreKind === 'full')}>
									{#if databaseRestoreKind === 'full'}
										<span class="size-1.5 rounded-full bg-emerald-500 dark:bg-emerald-400" />
									{/if}
								</div>
							</div>
							<div class="mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400">
								{$i18n.t(
									'Use this for .hwbk or .zip packages that include database and referenced local uploads.'
								)}
							</div>
						</div>
					</div>
				</button>
			</div>
		</div>

		<div
			class="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-xs leading-5 text-gray-600 dark:border-gray-800 dark:bg-gray-900/50 dark:text-gray-400"
		>
			{getDatabaseApplyKindGuardText()}
		</div>

		<div
			class="flex items-center justify-end gap-2 border-t border-gray-100 pt-4 dark:border-gray-800"
		>
			<button
				class={modalSecondaryButton}
				type="button"
				on:click={() => (showDatabaseRestoreModal = false)}
			>
				{$i18n.t('Cancel')}
			</button>
			<button
				class={modalPrimaryButton}
				type="button"
				on:click={confirmDatabaseRestoreKind}
				disabled={!databaseRestoreSupport.supported}
			>
				{$i18n.t('Choose file')}
			</button>
		</div>
	</div>
</Modal>

<div class="h-full min-h-0 overflow-y-auto pr-1 scrollbar-hidden">
	<div class="max-w-6xl mx-auto space-y-6">
		<section class="glass-section p-5 space-y-5">
			<div class="@container flex flex-col gap-5">
				<div class="flex flex-col gap-4">
					<div class="min-w-0 @[64rem]:flex-1">
						<div class="flex items-start gap-3">
							<div class="glass-icon-badge {activeTabMeta.badgeColor}">
								{#if selectedTab === 'chatManagement'}
									<svg
										xmlns="http://www.w3.org/2000/svg"
										fill="none"
										viewBox="0 0 24 24"
										stroke-width="1.5"
										stroke="currentColor"
										class="size-[18px] {activeTabMeta.iconColor}"
									>
										<path
											stroke-linecap="round"
											stroke-linejoin="round"
											d="M14 9a2 2 0 0 1-2 2H6l-4 4V4c0-1.1.9-2 2-2h8a2 2 0 0 1 2 2z"
										/>
										<path
											stroke-linecap="round"
											stroke-linejoin="round"
											d="M18 9h2a2 2 0 0 1 2 2v11l-4-4h-6a2 2 0 0 1-2-2v-1"
										/>
									</svg>
								{:else if selectedTab === 'backups'}
									<svg
										xmlns="http://www.w3.org/2000/svg"
										fill="none"
										viewBox="0 0 24 24"
										stroke-width="1.5"
										stroke="currentColor"
										class="size-[18px] {activeTabMeta.iconColor}"
									>
										<path
											stroke-linecap="round"
											stroke-linejoin="round"
											d="M20.25 6.375c0 2.278-3.694 4.125-8.25 4.125S3.75 8.653 3.75 6.375m16.5 0c0-2.278-3.694-4.125-8.25-4.125S3.75 4.097 3.75 6.375m16.5 0v11.25c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125V6.375m16.5 0v3.75m-16.5-3.75v3.75m16.5 0v3.75C20.25 16.153 16.556 18 12 18s-8.25-1.847-8.25-4.125v-3.75m16.5 0c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125"
										/>
									</svg>
								{:else}
									<svg
										xmlns="http://www.w3.org/2000/svg"
										fill="none"
										viewBox="0 0 24 24"
										stroke-width="1.5"
										stroke="currentColor"
										class="size-[18px] {activeTabMeta.iconColor}"
									>
										<path
											stroke-linecap="round"
											stroke-linejoin="round"
											d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z"
										/>
									</svg>
								{/if}
							</div>
							<div class="min-w-0">
								<div class="text-base font-semibold text-gray-800 dark:text-gray-100">
									{activeTabMeta.label}
								</div>
								<p class="mt-1 text-xs text-gray-400 dark:text-gray-500">
									{activeTabMeta.description}
								</p>
							</div>
						</div>
					</div>

					<div
						class="inline-flex max-w-full flex-wrap items-center gap-1.5 self-start rounded-xl bg-gray-100/70 p-1 shadow-[inset_0_1px_0_rgba(255,255,255,0.65)] dark:bg-gray-850/80 dark:shadow-none @[64rem]:flex-nowrap @[64rem]:shrink-0"
					>
						{#each visibleTabs as tab}
							<button
								type="button"
								class={`flex min-w-0 items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium transition-all ${selectedTab === tab ? 'bg-white text-gray-900 shadow-[0_1px_3px_rgba(15,23,42,0.08)] dark:bg-gray-800 dark:text-white' : 'text-gray-500 hover:bg-white/50 hover:text-gray-800 dark:text-gray-400 dark:hover:bg-gray-800/50 dark:hover:text-gray-200'}`}
								on:click={() => {
									selectedTab = tab;
								}}
							>
								{#if tab === 'chatManagement'}
									<svg
										xmlns="http://www.w3.org/2000/svg"
										fill="none"
										viewBox="0 0 24 24"
										stroke-width="1.5"
										stroke="currentColor"
										class="size-4"
									>
										<path
											stroke-linecap="round"
											stroke-linejoin="round"
											d="M14 9a2 2 0 0 1-2 2H6l-4 4V4c0-1.1.9-2 2-2h8a2 2 0 0 1 2 2z"
										/>
										<path
											stroke-linecap="round"
											stroke-linejoin="round"
											d="M18 9h2a2 2 0 0 1 2 2v11l-4-4h-6a2 2 0 0 1-2-2v-1"
										/>
									</svg>
								{:else if tab === 'backups'}
									<svg
										xmlns="http://www.w3.org/2000/svg"
										fill="none"
										viewBox="0 0 24 24"
										stroke-width="1.5"
										stroke="currentColor"
										class="size-4"
									>
										<path
											stroke-linecap="round"
											stroke-linejoin="round"
											d="M20.25 6.375c0 2.278-3.694 4.125-8.25 4.125S3.75 8.653 3.75 6.375m16.5 0c0-2.278-3.694-4.125-8.25-4.125S3.75 4.097 3.75 6.375m16.5 0v11.25c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125V6.375m16.5 0v3.75m-16.5-3.75v3.75m16.5 0v3.75C20.25 16.153 16.556 18 12 18s-8.25-1.847-8.25-4.125v-3.75m16.5 0c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125"
										/>
									</svg>
								{:else}
									<svg
										xmlns="http://www.w3.org/2000/svg"
										fill="none"
										viewBox="0 0 24 24"
										stroke-width="1.5"
										stroke="currentColor"
										class="size-4"
									>
										<path
											stroke-linecap="round"
											stroke-linejoin="round"
											d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z"
										/>
									</svg>
								{/if}
								<span>{tabMeta[tab].label}</span>
							</button>
						{/each}
					</div>
				</div>
			</div>
		</section>

		{#if selectedTab === 'chatManagement'}
			<section class="scroll-mt-2 p-5 space-y-5 transition-all duration-300 glass-section">
				<div class="space-y-3">
					<div class="flex flex-wrap items-center gap-2 pl-1">
						<div class="text-sm font-medium text-gray-500 dark:text-gray-400">
							{$i18n.t('Import / Export')}
						</div>
						<span
							class="{badgeClass} border-blue-200/70 bg-blue-50/80 text-blue-700 dark:border-blue-800/40 dark:bg-blue-950/20 dark:text-blue-300"
						>
							{$i18n.t('Current user only')}
						</span>
					</div>
					<div class="grid grid-cols-1 md:grid-cols-2 gap-3">
						<div class="glass-item px-4 py-3 space-y-3">
							<div class="flex items-center justify-between gap-3">
								<div class="min-w-0">
									<div class="text-sm font-medium">{$i18n.t('Import Chats')}</div>
									<div class="text-xs text-gray-400 dark:text-gray-500 mt-0.5">
										{$i18n.t('Import chat history from a JSON file')}
									</div>
								</div>
								<button
									class={btnNeutral}
									type="button"
									on:click={() => chatImportInputElement?.click()}
									disabled={operationStates.chatImport.phase === 'running'}
								>
									{$i18n.t('Select File')}
								</button>
							</div>

							{#if chatImportDraft}
								<div
									class="rounded-2xl border border-gray-200/70 bg-white/70 p-3 space-y-3 dark:border-gray-800/40 dark:bg-gray-900/40"
								>
									<div class="space-y-1">
										<div class="text-xs font-medium text-gray-500 dark:text-gray-400">
											{$i18n.t('Selected file')}
										</div>
										<div class="text-sm font-medium text-gray-800 dark:text-gray-100">
											{chatImportDraft.fileName}
										</div>
										<div class="text-xs text-gray-500 dark:text-gray-400">
											{$i18n.t('Source')}: {chatImportDraft.origin === 'openai'
												? 'OpenAI'
												: 'Halo WebUI'}
											· {$i18n.t('Found {{count}} chats.', { count: chatImportDraft.count })}
										</div>
									</div>

									<div class="space-y-2">
										<div class="text-xs font-medium text-gray-500 dark:text-gray-400">
											{$i18n.t('Import mode')}
										</div>
										<div class="flex flex-wrap gap-2">
											<button
												type="button"
												class={`${modeButtonBase} ${chatImportDraft.mode === 'merge' ? 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800/40 dark:bg-blue-950/20 dark:text-blue-300' : 'border-gray-200 bg-white text-gray-600 dark:border-gray-800/40 dark:bg-gray-900/40 dark:text-gray-300'}`}
												on:click={() => {
													chatImportDraft = {
														...chatImportDraft,
														mode: 'merge',
														confirmReplace: false
													};
												}}
											>
												{$i18n.t('Merge with current chats')}
											</button>
											<button
												type="button"
												class={`${modeButtonBase} ${chatImportDraft.mode === 'replace' ? 'border-orange-200 bg-orange-50 text-orange-700 dark:border-orange-800/40 dark:bg-orange-950/20 dark:text-orange-300' : 'border-gray-200 bg-white text-gray-600 dark:border-gray-800/40 dark:bg-gray-900/40 dark:text-gray-300'}`}
												on:click={() => {
													chatImportDraft = { ...chatImportDraft, mode: 'replace' };
												}}
											>
												{$i18n.t('Replace current chats')}
											</button>
										</div>
									</div>

									{#if chatImportDraft.invalidCount > 0}
										<div
											class="rounded-xl border border-amber-200/70 bg-amber-50/80 px-3 py-2 text-xs text-amber-700 dark:border-amber-800/40 dark:bg-amber-950/20 dark:text-amber-300"
										>
											{$i18n.t('Skipped {{count}} invalid entries during precheck.', {
												count: chatImportDraft.invalidCount
											})}
										</div>
									{/if}

									{#if chatImportDraft.mode === 'replace'}
										<div
											class="rounded-xl border border-orange-200/70 bg-orange-50/80 px-3 py-2 space-y-2 dark:border-orange-800/40 dark:bg-orange-950/20"
										>
											<div class="text-xs font-medium text-orange-700 dark:text-orange-300">
												{$i18n.t('This will replace all current chats for your account.')}
											</div>
											<label
												class="flex items-center gap-2 text-xs text-orange-700 dark:text-orange-300"
											>
												<input
													type="checkbox"
													class="rounded border-orange-300 text-orange-600 focus:ring-orange-200 dark:border-orange-700 dark:bg-gray-900"
													bind:checked={chatImportDraft.confirmReplace}
												/>
												<span
													>{$i18n.t(
														'I understand that this operation cannot be undone from the UI.'
													)}</span
												>
											</label>
										</div>
									{/if}

									<div class="flex items-center justify-end gap-2">
										<button class={btnSmall} type="button" on:click={resetChatImportDraft}>
											{$i18n.t('Cancel')}
										</button>
										<button
											class={chatImportDraft.mode === 'replace' ? btnWarn : btnNeutral}
											type="button"
											on:click={runChatImport}
											disabled={operationStates.chatImport.phase === 'running'}
										>
											{$i18n.t(
												chatImportDraft.mode === 'replace' ? 'Start restore' : 'Start import'
											)}
										</button>
									</div>
								</div>
							{/if}

							<DataManagementStatus
								visible={hasOperationState('chatImport')}
								phase={operationStates.chatImport.phase}
								title={operationStates.chatImport.title}
								detail={operationStates.chatImport.detail}
							/>
						</div>

						<div class="glass-item px-4 py-3 space-y-3">
							<div class="flex items-center justify-between gap-3">
								<div class="min-w-0">
									<div class="text-sm font-medium">{$i18n.t('Export Chats')}</div>
									<div class="text-xs text-gray-400 dark:text-gray-500 mt-0.5">
										{$i18n.t('Export your chat history to a JSON file')}
									</div>
								</div>
								<button
									class={btnNeutral}
									type="button"
									on:click={exportChats}
									disabled={operationStates.chatExport.phase === 'running'}
								>
									{$i18n.t(
										operationStates.chatExport.phase === 'running' ? 'Exporting...' : 'Export'
									)}
								</button>
							</div>
							<DataManagementStatus
								visible={hasOperationState('chatExport')}
								phase={operationStates.chatExport.phase}
								title={operationStates.chatExport.title}
								detail={operationStates.chatExport.detail}
							/>
						</div>
					</div>

					<div class="flex flex-wrap items-center gap-2 pl-1">
						<div class="text-sm font-medium text-gray-500 dark:text-gray-400">
							{$i18n.t('Chat Archive')}
						</div>
						<span
							class="{badgeClass} border-blue-200/70 bg-blue-50/80 text-blue-700 dark:border-blue-800/40 dark:bg-blue-950/20 dark:text-blue-300"
						>
							{$i18n.t('Current user only')}
						</span>
					</div>
					<div class="grid grid-cols-1 md:grid-cols-2 gap-3">
						<div class="flex items-center justify-between glass-item px-4 py-3">
							<div class="min-w-0 mr-3">
								<div class="text-sm font-medium">{$i18n.t('Archived Chats')}</div>
								<div class="text-xs text-gray-400 dark:text-gray-500 mt-0.5">
									{$i18n.t('View and manage your archived conversations')}
								</div>
							</div>
							<button
								class={btnNeutral}
								type="button"
								on:click={() => (showArchivedChatsModal = true)}
							>
								{$i18n.t('View')}
							</button>
						</div>

						<div class="glass-item px-4 py-3 space-y-3">
							<div class="flex items-center justify-between gap-3">
								<div class="min-w-0">
									<div class="text-sm font-medium">{$i18n.t('Archive All Chats')}</div>
									<div class="text-xs text-gray-400 dark:text-gray-500 mt-0.5">
										{$i18n.t('Move all current conversations to the archive')}
									</div>
								</div>
								{#if showArchiveConfirm}
									<div class="shrink-0 flex items-center gap-1.5">
										<span
											class="text-xs text-orange-600/80 dark:text-orange-400/80 whitespace-nowrap"
										>
											{$i18n.t('Are you sure?')}
										</span>
										<button
											class={btnSmall}
											type="button"
											on:click={() => (showArchiveConfirm = false)}
										>
											{$i18n.t('Cancel')}
										</button>
										<button
											class={btnSmallWarn}
											type="button"
											on:click={() => {
												archiveAllChatsHandler();
												showArchiveConfirm = false;
											}}
										>
											{$i18n.t('Confirm')}
										</button>
									</div>
								{:else}
									<button
										class={btnWarn}
										type="button"
										on:click={() => (showArchiveConfirm = true)}
										disabled={operationStates.archiveAll.phase === 'running'}
									>
										{$i18n.t('Archive All')}
									</button>
								{/if}
							</div>
							<DataManagementStatus
								visible={hasOperationState('archiveAll')}
								phase={operationStates.archiveAll.phase}
								title={operationStates.archiveAll.title}
								detail={operationStates.archiveAll.detail}
							/>
						</div>
					</div>
				</div>
			</section>
		{:else if selectedTab === 'backups' && isAdmin}
			<section class="scroll-mt-2 p-5 space-y-5 transition-all duration-300 glass-section">
				<div class="space-y-3">
					<div class="flex flex-wrap items-center gap-2 pl-1">
						<div class="text-sm font-medium text-gray-500 dark:text-gray-400">
							{$i18n.t('Configuration')}
						</div>
						<span
							class="{badgeClass} border-emerald-200/70 bg-emerald-50/80 text-emerald-700 dark:border-emerald-800/40 dark:bg-emerald-950/20 dark:text-emerald-300"
						>
							{$i18n.t('Admin only')}
						</span>
					</div>
					<div class="grid grid-cols-1 md:grid-cols-2 gap-3">
						<div class="glass-item px-4 py-3 space-y-3">
							<div class="flex items-center justify-between gap-3">
								<div class="min-w-0">
									<div class="text-sm font-medium">{$i18n.t('Import Config from JSON File')}</div>
									<div class="text-xs text-gray-400 dark:text-gray-500 mt-0.5">
										{$i18n.t('Import your application configuration from a JSON file')}
									</div>
								</div>
								<button
									class={btnNeutral}
									type="button"
									on:click={() => configImportInputElement?.click()}
									disabled={operationStates.configImport.phase === 'running'}
								>
									{$i18n.t('Select File')}
								</button>
							</div>

							{#if configImportDraft}
								<div
									class="rounded-2xl border border-gray-200/70 bg-white/70 p-3 space-y-3 dark:border-gray-800/40 dark:bg-gray-900/40"
								>
									<div class="space-y-1">
										<div class="text-xs font-medium text-gray-500 dark:text-gray-400">
											{$i18n.t('Selected file')}
										</div>
										<div class="text-sm font-medium text-gray-800 dark:text-gray-100">
											{configImportDraft.fileName}
										</div>
										<div class="text-xs text-gray-500 dark:text-gray-400">
											{$i18n.t('Found {{count}} top-level config sections.', {
												count: configImportDraft.topLevelKeys.length
											})}
										</div>
									</div>

									{#if configImportDraft.topLevelKeys.length > 0}
										<div class="flex flex-wrap gap-2">
											{#each configImportDraft.topLevelKeys.slice(0, 6) as key}
												<span
													class="{badgeClass} border-gray-200/70 bg-gray-50/80 text-gray-600 dark:border-gray-800/40 dark:bg-gray-900/50 dark:text-gray-300"
												>
													{key}
												</span>
											{/each}
										</div>
									{/if}

									<div class="space-y-2">
										<div class="text-xs font-medium text-gray-500 dark:text-gray-400">
											{$i18n.t('Import mode')}
										</div>
										<div class="flex flex-wrap gap-2">
											<button
												type="button"
												class={`${modeButtonBase} ${configImportDraft.mode === 'merge' ? 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800/40 dark:bg-blue-950/20 dark:text-blue-300' : 'border-gray-200 bg-white text-gray-600 dark:border-gray-800/40 dark:bg-gray-900/40 dark:text-gray-300'}`}
												on:click={() => {
													configImportDraft = {
														...configImportDraft,
														mode: 'merge',
														confirmReplace: false
													};
												}}
											>
												{$i18n.t('Merge with current config')}
											</button>
											<button
												type="button"
												class={`${modeButtonBase} ${configImportDraft.mode === 'replace' ? 'border-orange-200 bg-orange-50 text-orange-700 dark:border-orange-800/40 dark:bg-orange-950/20 dark:text-orange-300' : 'border-gray-200 bg-white text-gray-600 dark:border-gray-800/40 dark:bg-gray-900/40 dark:text-gray-300'}`}
												on:click={() => {
													configImportDraft = { ...configImportDraft, mode: 'replace' };
												}}
											>
												{$i18n.t('Replace current config')}
											</button>
										</div>
									</div>

									{#if configImportDraft.mode === 'replace'}
										<div
											class="rounded-xl border border-orange-200/70 bg-orange-50/80 px-3 py-2 space-y-2 dark:border-orange-800/40 dark:bg-orange-950/20"
										>
											<div class="text-xs font-medium text-orange-700 dark:text-orange-300">
												{$i18n.t('This will replace the current application configuration.')}
											</div>
											<label
												class="flex items-center gap-2 text-xs text-orange-700 dark:text-orange-300"
											>
												<input
													type="checkbox"
													class="rounded border-orange-300 text-orange-600 focus:ring-orange-200 dark:border-orange-700 dark:bg-gray-900"
													bind:checked={configImportDraft.confirmReplace}
												/>
												<span
													>{$i18n.t(
														'I understand that this operation can change system behavior immediately.'
													)}</span
												>
											</label>
										</div>
									{/if}

									<div class="flex items-center justify-end gap-2">
										<button class={btnSmall} type="button" on:click={resetConfigImportDraft}>
											{$i18n.t('Cancel')}
										</button>
										<button
											class={configImportDraft.mode === 'replace' ? btnWarn : btnNeutral}
											type="button"
											on:click={runConfigImport}
											disabled={operationStates.configImport.phase === 'running'}
										>
											{$i18n.t(
												configImportDraft.mode === 'replace' ? 'Start restore' : 'Start import'
											)}
										</button>
									</div>
								</div>
							{/if}

							<DataManagementStatus
								visible={hasOperationState('configImport')}
								phase={operationStates.configImport.phase}
								title={operationStates.configImport.title}
								detail={operationStates.configImport.detail}
							/>
						</div>

						<div class="glass-item px-4 py-3 space-y-3">
							<div class="flex items-center justify-between gap-3">
								<div class="min-w-0">
									<div class="text-sm font-medium">{$i18n.t('Export Config to JSON File')}</div>
									<div class="text-xs text-gray-400 dark:text-gray-500 mt-0.5">
										{$i18n.t('Export your current application configuration to a JSON file')}
									</div>
								</div>
								<button
									class={btnNeutral}
									type="button"
									on:click={exportConfigToFile}
									disabled={operationStates.configExport.phase === 'running'}
								>
									{$i18n.t(
										operationStates.configExport.phase === 'running' ? 'Exporting...' : 'Export'
									)}
								</button>
							</div>
							<DataManagementStatus
								visible={hasOperationState('configExport')}
								phase={operationStates.configExport.phase}
								title={operationStates.configExport.title}
								detail={operationStates.configExport.detail}
							/>
						</div>
					</div>

					{#if $config?.features.enable_admin_export ?? true}
						<div class="flex flex-wrap items-center gap-2 pl-1">
							<div class="text-sm font-medium text-gray-500 dark:text-gray-400">
								{$i18n.t('Database')}
							</div>
							<span
								class="{badgeClass} border-amber-200/55 bg-amber-50/45 text-amber-700/80 dark:border-amber-800/30 dark:bg-amber-950/15 dark:text-amber-300/80"
							>
								{$i18n.t('Entire system data')}
							</span>
						</div>
						<div class="grid grid-cols-1 md:grid-cols-2 gap-3">
							<div class="glass-item px-4 py-3 space-y-3">
								<div class="flex items-center justify-between gap-3">
									<div class="min-w-0">
										<div class="text-sm font-medium">{$i18n.t('Export Database')}</div>
										<div class="text-xs text-gray-400 dark:text-gray-500 mt-0.5">
											{$i18n.t('Export a database-only file or a full backup package.')}
										</div>
									</div>
									<button
										class={btnNeutral}
										type="button"
										on:click={openDatabaseExportModal}
										disabled={operationStates.databaseExport.phase === 'running'}
									>
										{$i18n.t(
											operationStates.databaseExport.phase === 'running' ? 'Exporting...' : 'Export'
										)}
									</button>
								</div>
								<DataManagementStatus
									visible={hasOperationState('databaseExport')}
									phase={operationStates.databaseExport.phase}
									title={operationStates.databaseExport.title}
									detail={operationStates.databaseExport.detail}
								/>
							</div>

							<div class="glass-item px-4 py-3 space-y-3">
								<div class="flex items-center justify-between gap-3">
									<div class="min-w-0">
										<div class="text-sm font-medium">{$i18n.t('Import Database Backup')}</div>
										<div class="text-xs text-gray-400 dark:text-gray-500 mt-0.5">
											{$i18n.t(
												'Merge old backup data safely, or fully restore the database when needed.'
											)}
										</div>
									</div>
									<button
										class={btnNeutral}
										type="button"
										on:click={() => openDatabaseRestoreModal()}
										disabled={!databaseRestoreSupport.supported ||
											operationStates.databaseMerge.phase === 'validating' ||
											operationStates.databaseMerge.phase === 'running' ||
											operationStates.databaseRestore.phase === 'validating' ||
											operationStates.databaseRestore.phase === 'running'}
									>
										{$i18n.t(
											operationStates.databaseMerge.phase === 'validating' ||
												operationStates.databaseRestore.phase === 'validating'
												? 'Checking...'
												: 'Import'
										)}
									</button>
								</div>

								{#if !databaseRestoreSupport.supported}
									<div
										class="rounded-lg border border-amber-200/70 bg-amber-50/70 px-3 py-2 text-xs text-amber-700 dark:border-amber-800/40 dark:bg-amber-950/20 dark:text-amber-300"
									>
										{getRestoreUnavailableMessage()}
									</div>
								{/if}

								{#if databaseRestoreInspectingFile && operationStates[getDatabaseOperationKey(databaseRestoreInspectingFile.mode)].phase === 'validating'}
									<div class={databaseApplyPanelClass(databaseRestoreInspectingFile.mode)}>
										<div class="flex items-start gap-3">
											<div
												class={`mt-0.5 shrink-0 ${databaseApplyTextClass(databaseRestoreInspectingFile.mode)}`}
											>
												<Spinner className="size-5" />
											</div>
											<div class="min-w-0 flex-1 space-y-1">
												<div
													class={`text-xs font-medium ${databaseApplyTextClass(databaseRestoreInspectingFile.mode)}`}
												>
													{$i18n.t('Backup file')}
												</div>
												<div class="truncate text-sm font-medium text-gray-800 dark:text-gray-100">
													{databaseRestoreInspectingFile.fileName}
												</div>
												<div class="text-xs text-gray-500 dark:text-gray-400">
													{getBackupKindLabel(databaseRestoreInspectingFile.kind)} · {formatBytes(
														databaseRestoreInspectingFile.fileSize
													)}
												</div>
											</div>
										</div>

										<div class={databaseApplyNoticeClass(databaseRestoreInspectingFile.mode)}>
											{$i18n.t('Checking backup contents. Large database files may take a while.')}
										</div>

										<div class="flex items-center justify-end gap-2">
											<button
												class={btnSmall}
												type="button"
												on:click={cancelDatabaseRestoreInspection}
											>
												{$i18n.t('Cancel')}
											</button>
										</div>
									</div>
								{/if}

								{#if databaseRestoreDraft}
									<div class={databaseApplyPanelClass(databaseRestoreDraft.mode)}>
										<div class="space-y-1">
											<div
												class={`text-xs font-medium ${databaseApplyTextClass(databaseRestoreDraft.mode)}`}
											>
												{$i18n.t('Backup file')}
											</div>
											<div class="text-sm font-medium text-gray-800 dark:text-gray-100">
												{databaseRestoreDraft.fileName}
											</div>
											<div class="text-xs text-gray-500 dark:text-gray-400">
												{getBackupKindLabel(databaseRestoreDraft.kind)} · {formatBytes(
													databaseRestoreDraft.fileSize
												)} · {$i18n.t('Found {{count}} tables in the backup.', {
													count: databaseRestoreDraft.summary.table_count
												})}
												{#if databaseRestoreDraft.kind === 'full'}
													· {$i18n.t('Includes {{count}} uploaded files ({{size}}).', {
														count: databaseRestoreDraft.summary.upload_count ?? 0,
														size: formatBytes(databaseRestoreDraft.summary.upload_bytes ?? 0)
													})}
												{/if}
											</div>
										</div>

										<div class="flex flex-wrap gap-2">
											{#each databaseRestoreDraft.summary.tables_preview as tableName}
												<span
													class={`${badgeClass} ${databaseApplyBadgeClass(databaseRestoreDraft.mode)}`}
												>
													{tableName}
												</span>
											{/each}
										</div>

										{#if databaseRestoreDraft.warnings.length > 0}
											<div
												class="rounded-xl border border-amber-200/70 bg-amber-50/80 px-3 py-2 text-xs text-amber-700 space-y-1 dark:border-amber-800/40 dark:bg-amber-950/20 dark:text-amber-300"
											>
												{#each databaseRestoreDraft.warnings as warning}
													<div>{$i18n.t(warning)}</div>
												{/each}
											</div>
										{/if}

										<div class={databaseApplyNoticeClass(databaseRestoreDraft.mode)}>
											{#if databaseRestoreDraft.mode === 'merge'}
												{$i18n.t(
													'Merge import will add missing rows and missing chat messages. Existing rows will not be overwritten or deleted.'
												)}
											{:else}
												{$i18n.t(
													'Restoring this backup will completely overwrite current system data.'
												)}
											{/if}
											{#if databaseRestoreDraft.confirmMerge || databaseRestoreDraft.confirmOverwrite}
												<div class="mt-1 font-medium">
													{$i18n.t(
														databaseRestoreDraft.mode === 'merge'
															? 'Click Confirm merge import to continue.'
															: 'Click Confirm restore to continue.'
													)}
												</div>
											{/if}
										</div>

										<div class="flex items-center justify-end gap-2">
											<button class={btnSmall} type="button" on:click={resetDatabaseRestoreDraft}>
												{$i18n.t('Cancel')}
											</button>
											<button
												class={databaseRestoreDraft.mode === 'merge' ? btnNeutral : btnDanger}
												type="button"
												on:click={runDatabaseRestore}
												disabled={operationStates[
													getDatabaseOperationKey(databaseRestoreDraft.mode)
												].phase === 'running'}
											>
												{$i18n.t(
													databaseRestoreDraft.mode === 'merge'
														? databaseRestoreDraft.confirmMerge
															? 'Confirm merge import'
															: 'Start merge import'
														: databaseRestoreDraft.confirmOverwrite
															? 'Confirm restore'
															: 'Start restore'
												)}
											</button>
										</div>
									</div>
								{/if}

								<DataManagementStatus
									visible={hasOperationState('databaseMerge')}
									phase={operationStates.databaseMerge.phase}
									title={operationStates.databaseMerge.title}
									detail={operationStates.databaseMerge.detail}
								/>
								<DataManagementStatus
									visible={hasOperationState('databaseRestore')}
									phase={operationStates.databaseRestore.phase}
									title={operationStates.databaseRestore.title}
									detail={operationStates.databaseRestore.detail}
								/>
							</div>

							<div class="glass-item px-4 py-3 space-y-3 md:col-span-2">
								<div class="flex items-center justify-between gap-3">
									<div class="min-w-0">
										<div class="text-sm font-medium">{$i18n.t('Export All Chats (All Users)')}</div>
										<div class="text-xs text-gray-400 dark:text-gray-500 mt-0.5">
											{$i18n.t('Export all users chat records in JSON format')}
										</div>
									</div>
									<button
										class={btnNeutral}
										type="button"
										on:click={exportAllUserChats}
										disabled={operationStates.allUserChatsExport.phase === 'running'}
									>
										{$i18n.t(
											operationStates.allUserChatsExport.phase === 'running'
												? 'Exporting...'
												: 'Export'
										)}
									</button>
								</div>
								<DataManagementStatus
									visible={hasOperationState('allUserChatsExport')}
									phase={operationStates.allUserChatsExport.phase}
									title={operationStates.allUserChatsExport.title}
									detail={operationStates.allUserChatsExport.detail}
								/>
							</div>
						</div>
					{/if}
				</div>
			</section>
		{:else if selectedTab === 'dangerZone'}
			<section
				class="scroll-mt-2 p-5 space-y-5 transition-all duration-300 glass-section border-red-200/60 dark:border-red-800/30"
			>
				<div class="flex flex-wrap items-center gap-3">
					<div class="glass-icon-badge bg-red-50 dark:bg-red-950/30">
						<svg
							xmlns="http://www.w3.org/2000/svg"
							fill="none"
							viewBox="0 0 24 24"
							stroke-width="1.5"
							stroke="currentColor"
							class="size-[18px] text-red-500 dark:text-red-400"
						>
							<path
								stroke-linecap="round"
								stroke-linejoin="round"
								d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z"
							/>
						</svg>
					</div>
					<div class="text-base font-semibold text-red-600 dark:text-red-400">
						{$i18n.t('Danger Zone')}
					</div>
					<span
						class="{badgeClass} border-red-200/70 bg-red-50/80 text-red-700 dark:border-red-800/40 dark:bg-red-950/20 dark:text-red-300"
					>
						{$i18n.t('Cannot be undone')}
					</span>
					<span
						class="{badgeClass} border-orange-200/70 bg-orange-50/80 text-orange-700 dark:border-orange-800/40 dark:bg-orange-950/20 dark:text-orange-300"
					>
						{$i18n.t('Current user only')}
					</span>
				</div>

				<div class="space-y-3">
					<div class="glass-item px-4 py-3 border-red-200/60 dark:border-red-800/30 space-y-3">
						<div class="flex items-center justify-between gap-3">
							<div class="min-w-0 mr-3">
								<div class="text-sm font-medium text-red-700 dark:text-red-400">
									{$i18n.t('Delete All Chats')}
								</div>
								<div class="text-xs text-red-500/70 dark:text-red-400/70 mt-0.5">
									{$i18n.t(
										'Permanently delete all of your chat records. This action cannot be undone.'
									)}
								</div>
							</div>
							{#if showDeleteConfirm}
								<div class="shrink-0 flex items-center gap-1.5">
									<span class="text-xs text-red-600/70 dark:text-red-400/80 whitespace-nowrap">
										{$i18n.t('Are you sure?')}
									</span>
									<button
										class={btnSmall}
										type="button"
										on:click={() => (showDeleteConfirm = false)}
									>
										{$i18n.t('Cancel')}
									</button>
									<button
										class={btnSmallDanger}
										type="button"
										on:click={() => {
											deleteAllChatsHandler();
											showDeleteConfirm = false;
										}}
									>
										{$i18n.t('Confirm')}
									</button>
								</div>
							{:else}
								<button
									class={btnDanger}
									type="button"
									on:click={() => (showDeleteConfirm = true)}
									disabled={operationStates.deleteAll.phase === 'running'}
								>
									{$i18n.t('Delete All')}
								</button>
							{/if}
						</div>
						<DataManagementStatus
							visible={hasOperationState('deleteAll')}
							phase={operationStates.deleteAll.phase}
							title={operationStates.deleteAll.title}
							detail={operationStates.deleteAll.detail}
						/>
					</div>
				</div>
			</section>
		{/if}
	</div>
</div>
