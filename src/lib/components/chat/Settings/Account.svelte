<script lang="ts">
	import dayjs from 'dayjs';
	import { getContext, onMount } from 'svelte';
	import { page } from '$app/stores';

	import { user, config } from '$lib/stores';
	import { getUsers } from '$lib/apis/users';

	import PersonalSettings from './PersonalSettings.svelte';
	import UserList from '$lib/components/admin/Users/UserList.svelte';
	import Groups from '$lib/components/admin/Users/Groups.svelte';
	import UsersSolid from '$lib/components/icons/UsersSolid.svelte';
	import WrenchSolid from '$lib/components/icons/WrenchSolid.svelte';

	const i18n = getContext('i18n');

	export let saveHandler: Function;
	export let saveSettings: Function;

	let selectedTab: 'personal' | 'users' | 'groups' = 'personal';
	let users: any[] = [];
	let groupCount = 0;
	let permissionCount = 0;
	let loaded = false;

	$: isAdmin = $user?.role === 'admin';

	const getUsersHandler = async () => {
		users = await getUsers(localStorage.token);
	};

	const setUsers = (nextUsers: any[] = []) => {
		users = nextUsers;
	};

	const setGroupCount = (nextCount = 0) => {
		groupCount = nextCount;
	};

	const setPermissionCount = (nextCount = 0) => {
		permissionCount = nextCount;
	};

	$: totalUsers = users.length;
	$: activeUsers = users.filter((u) => {
		const lastActiveAt = Number(u?.last_active_at ?? 0);
		return lastActiveAt > 0 && lastActiveAt >= dayjs().subtract(30, 'day').unix();
	}).length;
	$: seatLimit = $config?.license_metadata?.seats ?? null;
	$: seatLabel = seatLimit !== null ? `${totalUsers}/${seatLimit}` : `${totalUsers}`;

	$: heroStats =
		selectedTab === 'groups'
			? [
					{ label: $i18n.t('Groups'), value: groupCount },
					{ label: $i18n.t('Users'), value: totalUsers },
					{ label: $i18n.t('Permissions'), value: permissionCount }
				]
			: selectedTab === 'users'
				? [
						{ label: $i18n.t('Users'), value: totalUsers },
						{ label: $i18n.t('Active in 30 days'), value: activeUsers },
						{ label: $i18n.t('Seat usage'), value: seatLabel, attention: seatLimit !== null && totalUsers > seatLimit }
					]
				: [];

	const tabMeta = {
		personal: {
			label: 'Personal Settings',
			description: 'Manage your profile, security credentials, and API access.'
		},
		users: {
			label: 'User Management',
			description: 'Manage roles, access, and user lifecycle across your workspace.'
		},
		groups: {
			label: 'Permission Groups',
			description: 'Organize membership with reusable permission groups and a shared default access policy.'
		}
	};

	$: activeTabMeta = tabMeta[selectedTab];

	const shouldSpanAccountTabFullRowOnMobile = (index: number) => index === 2;

	onMount(async () => {
		// Read tab from URL query parameter
		const tabParam = $page.url.searchParams.get('tab');
		if (tabParam && (tabParam === 'users' || tabParam === 'groups') && isAdmin) {
			selectedTab = tabParam;
		}

		if (isAdmin) {
			await getUsersHandler();
		}
		loaded = true;
	});
</script>

{#if loaded}
	<form class="flex h-full min-h-0 flex-col text-sm">
		<div class="h-full space-y-6 overflow-y-auto scrollbar-hidden">
			<div class="max-w-6xl mx-auto space-y-6">
				<!-- Hero Section (only for admin) -->
				{#if isAdmin}
					<section class="glass-section p-5 space-y-5">
						<div class="flex flex-col gap-5">
							<div class="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
								<div class="min-w-0">
									<div class="inline-flex h-8 items-center gap-2 whitespace-nowrap rounded-full border border-gray-200/80 bg-white/80 px-3.5 text-xs font-medium leading-none text-gray-600 dark:border-gray-700/80 dark:bg-gray-900/70 dark:text-gray-300">
										<span class="leading-none text-gray-400 dark:text-gray-500">{$i18n.t('Settings')}</span>
										<span class="leading-none text-gray-300 dark:text-gray-600">/</span>
										<span class="leading-none text-gray-900 dark:text-white"
											>{$i18n.t('Account Management', { defaultValue: $i18n.t('Account') })}</span
										>
									</div>

									<div class="mt-3 flex items-start gap-3">
										<div class="glass-icon-badge {selectedTab === 'personal' ? 'bg-blue-50 dark:bg-blue-950/30' : selectedTab === 'users' ? 'bg-pink-50 dark:bg-pink-950/30' : 'bg-violet-50 dark:bg-violet-950/30'}">
											{#if selectedTab === 'personal'}
												<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-[18px] {selectedTab === 'personal' ? 'text-blue-500 dark:text-blue-400' : selectedTab === 'users' ? 'text-pink-500 dark:text-pink-400' : 'text-violet-500 dark:text-violet-400'}">
													<path stroke-linecap="round" stroke-linejoin="round" d="M15.75 6a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0ZM4.501 20.118a7.5 7.5 0 0 1 14.998 0A17.933 17.933 0 0 1 12 21.75c-2.676 0-5.216-.584-7.499-1.632Z" />
												</svg>
											{:else if selectedTab === 'users'}
												<UsersSolid className="size-[18px] text-pink-500 dark:text-pink-400" />
											{:else}
												<WrenchSolid className="size-[18px] text-violet-500 dark:text-violet-400" />
											{/if}
										</div>

										<div class="min-w-0">
											<div class="text-base font-semibold text-gray-800 dark:text-gray-100">
												{$i18n.t(activeTabMeta.label)}
											</div>
											<p class="mt-1 text-xs text-gray-400 dark:text-gray-500">
												{$i18n.t(activeTabMeta.description)}
											</p>
										</div>
									</div>
								</div>

								<!-- Tabs moved to right -->
								<div class="grid w-full grid-cols-2 rounded-2xl bg-gray-100 p-1 dark:bg-gray-850 md:inline-flex md:w-fit md:flex-col lg:mt-11 lg:flex-row">
									<button
										type="button"
										class={`flex min-w-0 items-center gap-2 rounded-xl px-4 py-2 text-sm font-medium transition-all ${
											selectedTab === 'personal'
												? 'bg-white text-gray-900 shadow-sm dark:bg-gray-800 dark:text-white'
												: 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'
										}`}
										on:click={() => { selectedTab = 'personal'; }}
									>
										<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-4">
											<path stroke-linecap="round" stroke-linejoin="round" d="M15.75 6a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0ZM4.501 20.118a7.5 7.5 0 0 1 14.998 0A17.933 17.933 0 0 1 12 21.75c-2.676 0-5.216-.584-7.499-1.632Z" />
										</svg>
										<span>{$i18n.t('Personal Settings')}</span>
									</button>

									<button
										type="button"
										class={`flex min-w-0 items-center gap-2 rounded-xl px-4 py-2 text-sm font-medium transition-all ${
											selectedTab === 'users'
												? 'bg-white text-gray-900 shadow-sm dark:bg-gray-800 dark:text-white'
												: 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'
										}`}
										on:click={() => { selectedTab = 'users'; }}
									>
										<UsersSolid className="size-4" />
										<span>{$i18n.t('User Management')}</span>
									</button>

									<button
										type="button"
										class={`flex min-w-0 items-center gap-2 rounded-xl px-4 py-2 text-sm font-medium transition-all ${
											shouldSpanAccountTabFullRowOnMobile(2) ? 'col-span-2 md:col-span-1 ' : ''
										}${
											selectedTab === 'groups'
												? 'bg-white text-gray-900 shadow-sm dark:bg-gray-800 dark:text-white'
												: 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'
										}`}
										on:click={() => { selectedTab = 'groups'; }}
									>
										<WrenchSolid className="size-4" />
										<span>{$i18n.t('Permission Groups')}</span>
									</button>
								</div>
							</div>

							<!-- Hero Stats (only for users/groups tabs) -->
							{#if heroStats.length > 0}
								<div class="grid grid-cols-2 gap-3 sm:grid-cols-3">
									{#each heroStats as stat}
										<div
											class={`glass-item p-4 flex flex-col justify-center ${
												stat.attention
													? '!border-red-200 !bg-red-50/90 dark:!border-red-900/60 dark:!bg-red-950/30'
													: ''
											}`}
										>
											<div class="text-xs font-medium text-gray-500 dark:text-gray-400">
												{stat.label}
											</div>
											<div class="mt-2 text-2xl font-semibold tracking-tight {stat.attention ? 'text-red-700 dark:text-red-300' : ''}">
												{stat.value}
											</div>
										</div>
									{/each}
								</div>
							{/if}
						</div>
					</section>
				{/if}

				<!-- Tab Content -->
				{#if selectedTab === 'personal'}
					<PersonalSettings {saveHandler} {saveSettings} />
				{:else if selectedTab === 'users' && isAdmin}
					<div class="space-y-6">
						<UserList {users} {setUsers} />
					</div>
				{:else if selectedTab === 'groups' && isAdmin}
					<div class="space-y-6">
						<Groups {users} {setGroupCount} {setPermissionCount} />
					</div>
				{/if}
			</div>
		</div>
	</form>
{/if}
