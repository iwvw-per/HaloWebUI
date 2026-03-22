<script lang="ts">
	import fileSaver from 'file-saver';
	import { getContext } from 'svelte';
	import { goto } from '$app/navigation';
	import { toast } from 'svelte-sonner';

	import ArchivedChatsModal from '$lib/components/layout/Sidebar/ArchivedChatsModal.svelte';
	import { chats, config, currentChatPage, scrollPaginationEnabled, user } from '$lib/stores';
	import {
		archiveAllChats,
		createNewChat,
		deleteAllChats,
		getAllChats,
		getAllUserChats,
		getChatList
	} from '$lib/apis/chats';
	import { downloadDatabase } from '$lib/apis/utils';
	import { exportConfig, importConfig } from '$lib/apis/configs';
	import { convertOpenAIChats, getImportOrigin } from '$lib/utils';

	const { saveAs } = fileSaver;
	const i18n = getContext('i18n');

	type DataManagementTab = 'chatManagement' | 'backups' | 'dangerZone';

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
			label: $i18n.t('Data Export'),
			description: `${$i18n.t('Configuration')} · ${$i18n.t('Database')}`,
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

	const shouldSpanDataManagementTabFullRowOnMobile = (index: number) =>
		visibleTabs.length % 2 === 1 && index === visibleTabs.length - 1;

	let importFiles: FileList | null = null;
	let showArchiveConfirm = false;
	let showDeleteConfirm = false;
	let showArchivedChatsModal = false;
	let chatImportInputElement: HTMLInputElement;
	let configImportInputElement: HTMLInputElement;

	const btnNeutral =
		'shrink-0 inline-flex items-center justify-center h-8 px-4 text-xs font-medium rounded-lg glass-input text-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800/80 active:scale-[0.97] transition-all';
	const btnWarn =
		'shrink-0 inline-flex items-center justify-center h-8 px-4 text-xs font-medium rounded-lg bg-orange-50 hover:bg-orange-100 text-orange-600 dark:bg-orange-950/30 dark:hover:bg-orange-900/40 dark:text-orange-400 border border-orange-200/60 dark:border-orange-800/30 active:scale-[0.97] transition-all';
	const btnDanger =
		'shrink-0 inline-flex items-center justify-center h-8 px-4 text-xs font-medium rounded-lg bg-red-50 hover:bg-red-100 text-red-600 dark:bg-red-950/30 dark:hover:bg-red-900/40 dark:text-red-400 border border-red-200/60 dark:border-red-800/30 active:scale-[0.97] transition-all';
	const btnSmall =
		'shrink-0 inline-flex items-center justify-center h-7 px-3 text-xs font-medium rounded-md glass-input text-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800/80 active:scale-[0.97] transition-all';
	const btnSmallWarn =
		'shrink-0 inline-flex items-center justify-center h-7 px-3 text-xs font-medium rounded-md bg-orange-50 hover:bg-orange-100 text-orange-600 dark:bg-orange-950/30 dark:hover:bg-orange-900/40 dark:text-orange-400 border border-orange-200/60 dark:border-orange-800/30 active:scale-[0.97] transition-all';
	const btnSmallDanger =
		'shrink-0 inline-flex items-center justify-center h-7 px-3 text-xs font-medium rounded-md bg-red-50 hover:bg-red-100 text-red-600 dark:bg-red-950/30 dark:hover:bg-red-900/40 dark:text-red-400 border border-red-200/60 dark:border-red-800/30 active:scale-[0.97] transition-all';

	$: if (importFiles) {
		const reader = new FileReader();
		reader.onload = (event) => {
			let nextChats = JSON.parse(String(event.target?.result ?? '[]'));
			if (getImportOrigin(nextChats) === 'openai') {
				try {
					nextChats = convertOpenAIChats(nextChats);
				} catch (error) {
					console.log('Unable to import chats:', error);
				}
			}
			importChats(nextChats);
		};

		if (importFiles.length > 0) {
			reader.readAsText(importFiles[0]);
		}
	}

	const importChats = async (nextChats: any[]) => {
		for (const chat of nextChats) {
			if (chat.chat) {
				await createNewChat(localStorage.token, chat.chat);
			} else {
				await createNewChat(localStorage.token, chat);
			}
		}

		currentChatPage.set(1);
		await chats.set(await getChatList(localStorage.token, $currentChatPage));
		scrollPaginationEnabled.set(true);
	};

	const exportChats = async () => {
		const blob = new Blob([JSON.stringify(await getAllChats(localStorage.token))], {
			type: 'application/json'
		});
		saveAs(blob, `chat-export-${Date.now()}.json`);
	};

	const exportAllUserChats = async () => {
		const blob = new Blob([JSON.stringify(await getAllUserChats(localStorage.token))], {
			type: 'application/json'
		});
		saveAs(blob, `all-chats-export-${Date.now()}.json`);
	};

	const archiveAllChatsHandler = async () => {
		await goto('/');
		await archiveAllChats(localStorage.token).catch((error) => {
			toast.error(`${error}`);
		});

		currentChatPage.set(1);
		await chats.set(await getChatList(localStorage.token, $currentChatPage));
		scrollPaginationEnabled.set(true);
	};

	const deleteAllChatsHandler = async () => {
		await goto('/');
		await deleteAllChats(localStorage.token).catch((error) => {
			toast.error(`${error}`);
		});

		currentChatPage.set(1);
		await chats.set(await getChatList(localStorage.token, $currentChatPage));
		scrollPaginationEnabled.set(true);
	};

	const handleArchivedChatsChange = async () => {
		currentChatPage.set(1);
		await chats.set(await getChatList(localStorage.token, $currentChatPage));
		scrollPaginationEnabled.set(true);
	};

	const importConfigFromFile = async (event: Event) => {
		const target = event.currentTarget as HTMLInputElement | null;
		const file = target?.files?.[0];
		if (!file) return;

		const reader = new FileReader();
		reader.onload = async (loadEvent) => {
			try {
				const payload = JSON.parse(String(loadEvent.target?.result ?? '{}'));
				const res = await importConfig(localStorage.token, payload).catch((error) => {
					toast.error(`${error}`);
					return null;
				});

				if (res) {
					toast.success($i18n.t('Config imported successfully'));
				}
			} catch (error) {
				toast.error($i18n.t('Invalid JSON file'));
			} finally {
				if (target) {
					target.value = '';
				}
			}
		};

		reader.readAsText(file);
	};

	const exportConfigToFile = async () => {
		const cfg = await exportConfig(localStorage.token);
		const blob = new Blob([JSON.stringify(cfg)], { type: 'application/json' });
		saveAs(blob, `config-${Date.now()}.json`);
	};
</script>

<ArchivedChatsModal bind:show={showArchivedChatsModal} on:change={handleArchivedChatsChange} />

<input
	id="chat-import-input"
	bind:this={chatImportInputElement}
	bind:files={importFiles}
	type="file"
	accept=".json"
	hidden
/>

<input
	id="config-json-input"
	bind:this={configImportInputElement}
	type="file"
	accept=".json"
	hidden
	on:change={importConfigFromFile}
/>

<div class="h-full min-h-0 overflow-y-auto pr-1 scrollbar-hidden">
	<div class="max-w-6xl mx-auto space-y-6">
		<!-- ==================== Hero Section ==================== -->
		<section class="glass-section p-5 space-y-5">
			<div class="flex flex-col gap-5">
				<div class="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
					<div class="min-w-0">
						<div class="inline-flex h-8 items-center gap-2 whitespace-nowrap rounded-full border border-gray-200/80 bg-white/80 px-3.5 text-xs font-medium leading-none text-gray-600 dark:border-gray-700/80 dark:bg-gray-900/70 dark:text-gray-300">
							<span class="leading-none text-gray-400 dark:text-gray-500">{$i18n.t('Settings')}</span>
							<span class="leading-none text-gray-300 dark:text-gray-600">/</span>
							<span class="leading-none text-gray-900 dark:text-white">{$i18n.t('Database')}</span>
						</div>

						<div class="mt-3 flex items-start gap-3">
							<div class="glass-icon-badge {activeTabMeta.badgeColor}">
								{#if selectedTab === 'chatManagement'}
									<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-[18px] {activeTabMeta.iconColor}">
										<path stroke-linecap="round" stroke-linejoin="round" d="M14 9a2 2 0 0 1-2 2H6l-4 4V4c0-1.1.9-2 2-2h8a2 2 0 0 1 2 2z" />
										<path stroke-linecap="round" stroke-linejoin="round" d="M18 9h2a2 2 0 0 1 2 2v11l-4-4h-6a2 2 0 0 1-2-2v-1" />
									</svg>
								{:else if selectedTab === 'backups'}
									<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-[18px] {activeTabMeta.iconColor}">
										<path stroke-linecap="round" stroke-linejoin="round" d="M20.25 6.375c0 2.278-3.694 4.125-8.25 4.125S3.75 8.653 3.75 6.375m16.5 0c0-2.278-3.694-4.125-8.25-4.125S3.75 4.097 3.75 6.375m16.5 0v11.25c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125V6.375m16.5 0v3.75m-16.5-3.75v3.75m16.5 0v3.75C20.25 16.153 16.556 18 12 18s-8.25-1.847-8.25-4.125v-3.75m16.5 0c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125" />
									</svg>
								{:else}
									<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-[18px] {activeTabMeta.iconColor}">
										<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z" />
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

					<div class="grid w-full grid-cols-2 rounded-2xl bg-gray-100 p-1 dark:bg-gray-850 md:inline-flex md:w-fit md:flex-col lg:mt-11 lg:flex-row">
						{#each visibleTabs as tab, index}
							<button type="button" class={`flex min-w-0 items-center gap-2 rounded-xl px-4 py-2 text-sm font-medium transition-all ${shouldSpanDataManagementTabFullRowOnMobile(index) ? 'col-span-2 md:col-span-1' : ''} ${selectedTab === tab ? 'bg-white text-gray-900 shadow-sm dark:bg-gray-800 dark:text-white' : 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'}`} on:click={() => { selectedTab = tab; }}>
								{#if tab === 'chatManagement'}
									<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-4">
										<path stroke-linecap="round" stroke-linejoin="round" d="M14 9a2 2 0 0 1-2 2H6l-4 4V4c0-1.1.9-2 2-2h8a2 2 0 0 1 2 2z" />
										<path stroke-linecap="round" stroke-linejoin="round" d="M18 9h2a2 2 0 0 1 2 2v11l-4-4h-6a2 2 0 0 1-2-2v-1" />
									</svg>
								{:else if tab === 'backups'}
									<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-4">
										<path stroke-linecap="round" stroke-linejoin="round" d="M20.25 6.375c0 2.278-3.694 4.125-8.25 4.125S3.75 8.653 3.75 6.375m16.5 0c0-2.278-3.694-4.125-8.25-4.125S3.75 4.097 3.75 6.375m16.5 0v11.25c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125V6.375m16.5 0v3.75m-16.5-3.75v3.75m16.5 0v3.75C20.25 16.153 16.556 18 12 18s-8.25-1.847-8.25-4.125v-3.75m16.5 0c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125" />
									</svg>
								{:else}
									<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-4">
										<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z" />
									</svg>
								{/if}
								<span>{tabMeta[tab].label}</span>
							</button>
						{/each}
					</div>
				</div>
			</div>
		</section>

		<!-- ==================== Tab Content ==================== -->
		{#if selectedTab === 'chatManagement'}
			<section class="scroll-mt-2 p-5 space-y-5 transition-all duration-300 glass-section">
				<div class="space-y-3">
					<div class="text-sm font-medium text-gray-500 dark:text-gray-400 pl-1">
						{$i18n.t('Import / Export')}
					</div>
					<div class="grid grid-cols-1 md:grid-cols-2 gap-3">
						<div class="flex items-center justify-between glass-item px-4 py-3">
							<div class="min-w-0 mr-3">
								<div class="text-sm font-medium">{$i18n.t('Import Chats')}</div>
								<div class="text-xs text-gray-400 dark:text-gray-500 mt-0.5">
									{$i18n.t('Import chat history from a JSON file')}
								</div>
							</div>
							<button
								class={btnNeutral}
								type="button"
								on:click={() => chatImportInputElement.click()}
							>
								{$i18n.t('Import')}
							</button>
						</div>

						<div class="flex items-center justify-between glass-item px-4 py-3">
							<div class="min-w-0 mr-3">
								<div class="text-sm font-medium">{$i18n.t('Export Chats')}</div>
								<div class="text-xs text-gray-400 dark:text-gray-500 mt-0.5">
									{$i18n.t('Export your chat history to a JSON file')}
								</div>
							</div>
							<button class={btnNeutral} type="button" on:click={exportChats}>
								{$i18n.t('Export')}
							</button>
						</div>
					</div>

					<div class="text-sm font-medium text-gray-500 dark:text-gray-400 pl-1">
						{$i18n.t('Chat Archive')}
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

						<div class="flex items-center justify-between glass-item px-4 py-3">
							<div class="min-w-0 mr-3">
								<div class="text-sm font-medium">{$i18n.t('Archive All Chats')}</div>
								<div class="text-xs text-gray-400 dark:text-gray-500 mt-0.5">
									{$i18n.t('Move all current conversations to the archive')}
								</div>
							</div>
							{#if showArchiveConfirm}
								<div class="shrink-0 flex items-center gap-1.5">
									<span class="text-xs text-orange-600/80 dark:text-orange-400/80 whitespace-nowrap">
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
								>
									{$i18n.t('Archive All')}
								</button>
							{/if}
						</div>
					</div>
				</div>
			</section>
		{:else if selectedTab === 'backups' && isAdmin}
			<section class="scroll-mt-2 p-5 space-y-5 transition-all duration-300 glass-section">
				<div class="space-y-3">
					<div class="text-sm font-medium text-gray-500 dark:text-gray-400 pl-1">
						{$i18n.t('Configuration')}
					</div>
					<div class="grid grid-cols-1 md:grid-cols-2 gap-3">
						<div class="flex items-center justify-between glass-item px-4 py-3">
							<div class="min-w-0 mr-3">
								<div class="text-sm font-medium">{$i18n.t('Import Config from JSON File')}</div>
								<div class="text-xs text-gray-400 dark:text-gray-500 mt-0.5">
									{$i18n.t('Import your application configuration from a JSON file')}
								</div>
							</div>
							<button
								class={btnNeutral}
								type="button"
								on:click={() => configImportInputElement.click()}
							>
								{$i18n.t('Import')}
							</button>
						</div>

						<div class="flex items-center justify-between glass-item px-4 py-3">
							<div class="min-w-0 mr-3">
								<div class="text-sm font-medium">{$i18n.t('Export Config to JSON File')}</div>
								<div class="text-xs text-gray-400 dark:text-gray-500 mt-0.5">
									{$i18n.t('Export your current application configuration to a JSON file')}
								</div>
							</div>
							<button class={btnNeutral} type="button" on:click={exportConfigToFile}>
								{$i18n.t('Export')}
							</button>
						</div>
					</div>

					{#if $config?.features.enable_admin_export ?? true}
						<div class="text-sm font-medium text-gray-500 dark:text-gray-400 pl-1">
							{$i18n.t('Database')}
						</div>
						<div class="grid grid-cols-1 md:grid-cols-2 gap-3">
							<div class="flex items-center justify-between glass-item px-4 py-3">
								<div class="min-w-0 mr-3">
									<div class="text-sm font-medium">{$i18n.t('Export Database')}</div>
									<div class="text-xs text-gray-400 dark:text-gray-500 mt-0.5">
										{$i18n.t('Export the complete SQLite database backup, containing all system data')}
									</div>
								</div>
								<button
									class={btnNeutral}
									type="button"
									on:click={() => {
										downloadDatabase(localStorage.token).catch((error) => {
											toast.error(`${error}`);
										});
									}}
								>
									{$i18n.t('Export')}
								</button>
							</div>

							<div class="flex items-center justify-between glass-item px-4 py-3">
								<div class="min-w-0 mr-3">
									<div class="text-sm font-medium">{$i18n.t('Export All Chats (All Users)')}</div>
									<div class="text-xs text-gray-400 dark:text-gray-500 mt-0.5">
										{$i18n.t('Export all users chat records in JSON format')}
									</div>
								</div>
								<button class={btnNeutral} type="button" on:click={exportAllUserChats}>
									{$i18n.t('Export')}
								</button>
							</div>
						</div>
					{/if}
				</div>
			</section>
		{:else if selectedTab === 'dangerZone'}
			<section
				class="scroll-mt-2 p-5 space-y-5 transition-all duration-300 glass-section border-red-200/60 dark:border-red-800/30"
			>
				<div class="flex items-center gap-3">
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
				</div>

				<div class="space-y-3">
					<div
						class="flex items-center justify-between glass-item px-4 py-3 border-red-200/60 dark:border-red-800/30"
					>
						<div class="min-w-0 mr-3">
							<div class="text-sm font-medium text-red-700 dark:text-red-400">
								{$i18n.t('Delete All Chats')}
							</div>
							<div class="text-xs text-red-500/70 dark:text-red-400/70 mt-0.5">
								{$i18n.t('Permanently delete all of your chat records. This action cannot be undone.')}
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
							>
								{$i18n.t('Delete All')}
							</button>
						{/if}
					</div>
				</div>
			</section>
		{/if}
	</div>
</div>
