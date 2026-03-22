<script lang="ts">
	import { getContext } from 'svelte';
	import { toast } from 'svelte-sonner';
	import InterfacePreferences from '$lib/components/settings/InterfacePreferences.svelte';
	import AdminInterface from '$lib/components/admin/Settings/Interface.svelte';
	import InlineDirtyActions from '$lib/components/admin/Settings/InlineDirtyActions.svelte';
	import { user } from '$lib/stores';

	import type { Writable } from 'svelte/store';
	import type { UserSettingsContext } from '$lib/types/user-settings';

	const i18n: Writable<any> = getContext('i18n');
	const { saveSettings } = getContext<UserSettingsContext>('user-settings');

	type InterfaceTab = 'appearance' | 'layout' | 'chat' | 'input' | 'advanced' | 'tasks';

	let preferencesForm: InterfacePreferences | null = null;
	let adminForm: AdminInterface | null = null;
	let isAdmin = false;
	let selectedTab: InterfaceTab = 'appearance';

	// Per-section dirty state
	let sectionDirty: Record<string, boolean> = {};
	let tasksDirty = false;
	let saving = false;

	$: isAdmin = $user?.role === 'admin';
	$: activeDirty = selectedTab === 'tasks' ? tasksDirty : (sectionDirty[selectedTab] ?? false);

	// Static tab metadata (mirrors InterfacePreferences.baseSections + admin tasks tab)
	const tabMeta: Array<{ key: InterfaceTab; titleKey: string; iconPaths: string[]; badgeColor: string; iconColor: string; descKey: string; adminOnly?: boolean }> = [
		{
			key: 'appearance',
			titleKey: '界面设置',
			descKey: '主题、语言、代码高亮、背景图片等外观偏好',
			badgeColor: 'bg-amber-50 dark:bg-amber-950/30',
			iconColor: 'text-amber-500 dark:text-amber-400',
			iconPaths: ['M12 2.25a.75.75 0 0 1 .75.75v2.25a.75.75 0 0 1-1.5 0V3a.75.75 0 0 1 .75-.75ZM7.5 12a4.5 4.5 0 1 1 9 0 4.5 4.5 0 0 1-9 0ZM18.894 6.166a.75.75 0 0 0-1.06-1.06l-1.591 1.59a.75.75 0 1 0 1.06 1.061l1.591-1.59ZM21.75 12a.75.75 0 0 1-.75.75h-2.25a.75.75 0 0 1 0-1.5H21a.75.75 0 0 1 .75.75ZM17.834 18.894a.75.75 0 0 0 1.06-1.06l-1.59-1.591a.75.75 0 1 0-1.061 1.06l1.59 1.591ZM12 18a.75.75 0 0 1 .75.75V21a.75.75 0 0 1-1.5 0v-2.25A.75.75 0 0 1 12 18ZM7.758 17.303a.75.75 0 0 0-1.061-1.06l-1.591 1.59a.75.75 0 0 0 1.06 1.061l1.591-1.59ZM6 12a.75.75 0 0 1-.75.75H3a.75.75 0 0 1 0-1.5h2.25A.75.75 0 0 1 6 12ZM6.697 7.757a.75.75 0 0 0 1.06-1.06l-1.59-1.591a.75.75 0 0 0-1.061 1.06l1.59 1.591Z']
		},
		{
			key: 'layout',
			titleKey: '显示布局',
			descKey: '默认模型、页面模式、通知、宽屏等布局选项',
			badgeColor: 'bg-blue-50 dark:bg-blue-950/30',
			iconColor: 'text-blue-500 dark:text-blue-400',
			iconPaths: ['M2.25 5.25a3 3 0 0 1 3-3h13.5a3 3 0 0 1 3 3V15a3 3 0 0 1-3 3h-3v.257c0 .597.237 1.17.659 1.591l.621.622a.75.75 0 0 1-.53 1.28h-9a.75.75 0 0 1-.53-1.28l.621-.622a2.25 2.25 0 0 0 .659-1.59V18h-3a3 3 0 0 1-3-3V5.25Zm1.5 0v7.5a1.5 1.5 0 0 0 1.5 1.5h13.5a1.5 1.5 0 0 0 1.5-1.5v-7.5a1.5 1.5 0 0 0-1.5-1.5H5.25a1.5 1.5 0 0 0-1.5 1.5Z']
		},
		{
			key: 'chat',
			titleKey: '对话功能',
			descKey: '自动生成、显示渲染、交互行为、记忆等对话功能',
			badgeColor: 'bg-indigo-50 dark:bg-indigo-950/30',
			iconColor: 'text-indigo-500 dark:text-indigo-400',
			iconPaths: ['M4.804 21.644A6.707 6.707 0 0 0 6 21.75a6.721 6.721 0 0 0 3.583-1.029c.774.182 1.584.279 2.417.279 5.322 0 9.75-3.97 9.75-9 0-5.03-4.428-9-9.75-9s-9.75 3.97-9.75 9c0 2.409 1.025 4.587 2.674 6.192.232.226.277.428.254.543a3.73 3.73 0 0 1-.814 1.686.75.75 0 0 0 .44 1.223ZM8.25 10.875a1.125 1.125 0 1 0 0 2.25 1.125 1.125 0 0 0 0-2.25ZM10.875 12a1.125 1.125 0 1 1 2.25 0 1.125 1.125 0 0 1-2.25 0Zm4.875-1.125a1.125 1.125 0 1 0 0 2.25 1.125 1.125 0 0 0 0-2.25Z']
		},
		{
			key: 'input',
			titleKey: '输入设置',
			descKey: '富文本、自动补全、格式工具栏、发送方式等输入偏好',
			badgeColor: 'bg-cyan-50 dark:bg-cyan-950/30',
			iconColor: 'text-cyan-500 dark:text-cyan-400',
			iconPaths: ['M7.5 3.375c0-1.036.84-1.875 1.875-1.875h5.25c1.035 0 1.875.84 1.875 1.875v.375h1.125C18.832 3.75 19.75 4.668 19.75 5.875v12.25c0 1.207-.918 2.125-2.125 2.125H6.375A2.125 2.125 0 0 1 4.25 18.125V5.875C4.25 4.668 5.168 3.75 6.375 3.75H7.5v-.375ZM6.375 5.25a.625.625 0 0 0-.625.625v12.25c0 .345.28.625.625.625h11.25c.345 0 .625-.28.625-.625V5.875a.625.625 0 0 0-.625-.625H6.375ZM9.375 3c-.207 0-.375.168-.375.375V4.5h6V3.375A.375.375 0 0 0 14.625 3h-5.25ZM7.5 8.25a.75.75 0 0 1 .75-.75h1.5a.75.75 0 0 1 .75.75v1.5a.75.75 0 0 1-.75.75h-1.5a.75.75 0 0 1-.75-.75v-1.5Zm5.25-.75a.75.75 0 0 0 0 1.5h1.5a.75.75 0 0 0 0-1.5h-1.5ZM7.5 13.5a.75.75 0 0 1 .75-.75h1.5a.75.75 0 0 1 .75.75v1.5a.75.75 0 0 1-.75.75h-1.5a.75.75 0 0 1-.75-.75v-1.5Zm5.25-.75a.75.75 0 0 0 0 1.5h1.5a.75.75 0 0 0 0-1.5h-1.5Z']
		},
		{
			key: 'advanced',
			titleKey: '高级选项',
			descKey: '位置权限、联网搜索、沙箱安全、触觉反馈等高级选项',
			badgeColor: 'bg-slate-50 dark:bg-slate-950/30',
			iconColor: 'text-slate-500 dark:text-slate-400',
			iconPaths: [
				'M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.325.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 0 1 1.37.49l1.296 2.247a1.125 1.125 0 0 1-.26 1.431l-1.003.827c-.293.241-.438.613-.43.992a7.723 7.723 0 0 1 0 .255c-.008.378.137.75.43.991l1.004.827c.424.35.534.955.26 1.43l-1.298 2.247a1.125 1.125 0 0 1-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.47 6.47 0 0 1-.22.128c-.331.183-.581.495-.644.869l-.213 1.281c-.09.543-.56.94-1.11.94h-2.594c-.55 0-1.019-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 0 1-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 0 1-1.369-.49l-1.297-2.247a1.125 1.125 0 0 1 .26-1.431l1.004-.827c.292-.24.437-.613.43-.991a6.932 6.932 0 0 1 0-.255c.007-.38-.138-.751-.43-.992l-1.004-.827a1.125 1.125 0 0 1-.26-1.43l1.297-2.247a1.125 1.125 0 0 1 1.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.086.22-.128.332-.183.582-.495.644-.869l.214-1.28Z',
				'M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z'
			]
		},
		{
			key: 'tasks',
			titleKey: '智能生成',
			descKey: '管理员可在此配置任务模型、提示建议和系统横幅',
			badgeColor: 'bg-rose-50 dark:bg-rose-950/30',
			iconColor: 'text-rose-400 dark:text-rose-400',
			iconPaths: [
				'M5.625 1.5c-1.036 0-1.875.84-1.875 1.875v17.25c0 1.035.84 1.875 1.875 1.875h12.75c1.035 0 1.875-.84 1.875-1.875V12.75A3.75 3.75 0 0 0 16.5 9h-1.875a1.875 1.875 0 0 1-1.875-1.875V5.25A3.75 3.75 0 0 0 9 1.5H5.625ZM7.5 15a.75.75 0 0 1 .75-.75h7.5a.75.75 0 0 1 0 1.5h-7.5A.75.75 0 0 1 7.5 15Zm.75 2.25a.75.75 0 0 0 0 1.5H12a.75.75 0 0 0 0-1.5H8.25Z',
				'M12.971 1.816A5.23 5.23 0 0 1 14.25 5.25v1.875c0 .207.168.375.375.375H16.5a5.23 5.23 0 0 1 3.434 1.279 9.768 9.768 0 0 0-6.963-6.963Z'
			],
			adminOnly: true
		}
	];

	let allTabs: Array<typeof tabMeta[number] & { title: string; description: string }> = [];
	$: allTabs = tabMeta.filter((t) => !t.adminOnly || isAdmin).map((t) => ({
		...t,
		title: $i18n.t(t.titleKey),
		description: $i18n.t(t.descKey)
	}));

	$: activeTab = allTabs.find((t) => t.key === selectedTab) ?? allTabs[0];

	const shouldSpanFullRowOnMobile = (index: number) =>
		allTabs.length % 2 === 1 && index === allTabs.length - 1;

	const handleSave = async () => {
		if (saving) return;
		saving = true;
		try {
			if (selectedTab === 'tasks') {
				await (adminForm as any)?.save?.();
			} else {
				await preferencesForm?.saveSection?.(selectedTab as any);
			}
			toast.success($i18n.t('Settings saved successfully!'));
		} catch (error) {
			console.error(error);
			toast.error($i18n.t('Failed to save settings.'));
		} finally {
			saving = false;
		}
	};

	const handleReset = async () => {
		if (selectedTab === 'tasks') {
			await (adminForm as any)?.reset?.();
		} else {
			await preferencesForm?.resetSection?.(selectedTab as any);
		}
	};
</script>

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
							<span class="leading-none text-gray-900 dark:text-white">{$i18n.t('Interface')}</span>
						</div>

						{#if activeTab}
						<!-- Icon badge + title + description -->
						<div class="mt-3 flex items-start gap-3">
							<div class="glass-icon-badge {activeTab.badgeColor}">
								<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="size-[18px] {activeTab.iconColor}">
									{#each activeTab.iconPaths as pathD}
										<path fill-rule="evenodd" d={pathD} clip-rule="evenodd" />
									{/each}
								</svg>
							</div>
							<div class="min-w-0">
								<div class="flex items-center gap-3">
									<div class="text-base font-semibold text-gray-800 dark:text-gray-100">
										{activeTab.title}
									</div>
									<InlineDirtyActions
										dirty={activeDirty}
										{saving}
										saveAsSubmit={false}
										on:reset={handleReset}
										on:save={handleSave}
									/>
								</div>
								<p class="mt-1 text-xs text-gray-400 dark:text-gray-500">
									{activeTab.description}
								</p>
							</div>
						</div>
						{/if}
					</div>

					<!-- Tab buttons -->
					<div class="grid w-full grid-cols-2 rounded-2xl bg-gray-100 p-1 dark:bg-gray-850 md:inline-flex md:w-fit md:flex-col lg:mt-11 lg:flex-row xl:shrink-0">
						{#each allTabs as tab, index (tab.key)}
							<button
								type="button"
								class={`flex min-w-0 items-center gap-2 rounded-xl px-4 py-2 text-sm font-medium transition-all ${shouldSpanFullRowOnMobile(index) ? 'col-span-2 md:col-span-1' : ''} ${selectedTab === tab.key ? 'bg-white text-gray-900 shadow-sm dark:bg-gray-800 dark:text-white' : 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'}`}
								on:click={() => { selectedTab = tab.key; }}
							>
								<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="size-4">
									{#each tab.iconPaths as pathD}
										<path fill-rule="evenodd" d={pathD} clip-rule="evenodd" />
									{/each}
								</svg>
								<span>{tab.title}</span>
							</button>
						{/each}
					</div>
				</div>
			</div>
		</section>

		<!-- ==================== Tab Content ==================== -->
		{#if selectedTab !== 'tasks'}
			<section class="p-5 space-y-3 transition-all duration-300 {activeDirty ? 'glass-section glass-section-dirty' : 'glass-section'}">
				<InterfacePreferences
					bind:this={preferencesForm}
					{saveSettings}
					embedded={true}
					activeSection={selectedTab === 'tasks' ? null : selectedTab}
					on:sectionDirtyChange={(event) => {
						sectionDirty = event.detail?.sections ?? {};
					}}
					on:save={() => {
						toast.success($i18n.t('Settings saved successfully!'));
					}}
				/>
			</section>
		{:else if isAdmin}
			<section class="p-5 space-y-3 transition-all duration-300 {tasksDirty ? 'glass-section glass-section-dirty' : 'glass-section'}">
				<AdminInterface
					bind:this={adminForm}
					embedded={true}
					on:dirtyChange={(event) => {
						tasksDirty = !!event.detail?.value;
					}}
					on:save={() => {
						toast.success($i18n.t('Settings saved successfully!'));
					}}
				/>
			</section>
		{/if}
	</div>
</div>
