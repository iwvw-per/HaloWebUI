<script lang="ts">
	import { getContext } from 'svelte';
	import { toast } from 'svelte-sonner';
	import PersonalAudioSettingsForm from '$lib/components/settings/Audio/PersonalAudioSettingsForm.svelte';
	import GlobalAudioSettingsForm from '$lib/components/settings/Audio/GlobalAudioSettingsForm.svelte';
	import InlineDirtyActions from '$lib/components/admin/Settings/InlineDirtyActions.svelte';
	import { user } from '$lib/stores';

	import type { Writable } from 'svelte/store';
	import type { UserSettingsContext } from '$lib/types/user-settings';

	// Page shell for /settings/audio. It owns the hero and personal/global tab switching.

	const i18n: Writable<any> = getContext('i18n');
	const { saveSettings } = getContext<UserSettingsContext>('user-settings');

	let personalAudioForm:
		| { save?: () => Promise<void>; reset?: () => Promise<void>; isDirty?: boolean }
		| null = null;
	let globalAudioForm:
		| { save?: () => Promise<boolean>; reset?: () => Promise<void>; isDirty?: boolean }
		| null = null;
	let isAdmin = false;
	let personalDirty = false;
	let globalDirty = false;
	let personalSaving = false;
	let globalSaving = false;
	let selectedTab: 'personal' | 'global' = 'personal';

	$: isAdmin = $user?.role === 'admin';

	const tabMeta = {
		personal: {
			label: 'Personal Settings',
			description: 'Configure speech recognition and text-to-speech preferences',
			badgeColor: 'bg-blue-50 dark:bg-blue-950/30',
			iconColor: 'text-blue-500 dark:text-blue-400'
		},
		global: {
			label: 'Global Settings',
			description: 'Admin can edit personal preferences and global audio policy on this page.',
			badgeColor: 'bg-amber-50 dark:bg-amber-950/30',
			iconColor: 'text-amber-500 dark:text-amber-400'
		}
	};

	$: activeTabMeta = tabMeta[selectedTab];
	$: activeDirty = selectedTab === 'personal' ? personalDirty : globalDirty;
	$: activeSaving = selectedTab === 'personal' ? personalSaving : globalSaving;

	const savePersonal = async () => {
		if (personalSaving || !personalAudioForm?.save) return;

		personalSaving = true;
		try {
			await personalAudioForm.save();
			toast.success($i18n.t('Settings saved successfully!'));
		} catch (error) {
			console.error(error);
			toast.error($i18n.t('Failed to save personal audio settings.'));
		} finally {
			personalSaving = false;
		}
	};

	const resetPersonal = async () => {
		await personalAudioForm?.reset?.();
	};

	const saveGlobal = async () => {
		if (globalSaving || !globalAudioForm?.save) return;

		globalSaving = true;
		try {
			const saved = await globalAudioForm.save();
			if (!saved) {
				throw new Error('Global audio settings save failed.');
			}
			toast.success($i18n.t('Settings saved successfully!'));
		} catch (error) {
			console.error(error);
			toast.error($i18n.t('Failed to save global audio settings.'));
		} finally {
			globalSaving = false;
		}
	};

	const resetGlobal = async () => {
		await globalAudioForm?.reset?.();
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
							<span class="leading-none text-gray-900 dark:text-white">{$i18n.t('Audio')}</span>
						</div>

						<!-- Icon badge + title + description -->
						<div class="mt-3 flex items-start gap-3">
							<div class="glass-icon-badge {activeTabMeta.badgeColor}">
								{#if selectedTab === 'personal'}
									<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="size-[18px] {activeTabMeta.iconColor}">
										<path fill-rule="evenodd" d="M7.5 6a4.5 4.5 0 119 0 4.5 4.5 0 01-9 0zM3.751 20.105a8.25 8.25 0 0116.498 0 .75.75 0 01-.437.695A18.683 18.683 0 0112 22.5c-2.786 0-5.433-.608-7.812-1.7a.75.75 0 01-.437-.695z" clip-rule="evenodd" />
									</svg>
								{:else}
									<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="size-[18px] {activeTabMeta.iconColor}">
										<path fill-rule="evenodd" d="M11.078 2.25c-.917 0-1.699.663-1.855 1.567L8.982 5.38a7.269 7.269 0 00-.877.506l-1.524-.4a1.875 1.875 0 00-2.164.876l-.922 1.597a1.875 1.875 0 00.31 2.443l1.184.943a7.11 7.11 0 000 1.012l-1.184.944a1.875 1.875 0 00-.31 2.443l.922 1.597a1.875 1.875 0 002.164.876l1.524-.4c.28.19.573.36.877.506l.241 1.563a1.875 1.875 0 001.855 1.567h1.844c.916 0 1.699-.663 1.855-1.567l.24-1.563c.305-.146.598-.316.878-.506l1.524.4a1.875 1.875 0 002.163-.876l.922-1.597a1.875 1.875 0 00-.31-2.443l-1.183-.944a7.11 7.11 0 000-1.012l1.183-.943a1.875 1.875 0 00.31-2.443l-.922-1.597a1.875 1.875 0 00-2.163-.876l-1.524.4a7.268 7.268 0 00-.878-.506l-.24-1.563a1.875 1.875 0 00-1.855-1.567h-1.844zM12 15.75a3.75 3.75 0 100-7.5 3.75 3.75 0 000 7.5z" clip-rule="evenodd" />
									</svg>
								{/if}
							</div>
							<div class="min-w-0">
								<div class="flex items-center gap-3">
									<div class="text-base font-semibold text-gray-800 dark:text-gray-100">
										{$i18n.t(activeTabMeta.label)}
									</div>
									<InlineDirtyActions
										dirty={activeDirty}
										saving={activeSaving}
										saveAsSubmit={false}
										on:reset={selectedTab === 'personal' ? resetPersonal : resetGlobal}
										on:save={selectedTab === 'personal' ? savePersonal : saveGlobal}
									/>
								</div>
								<p class="mt-1 text-xs text-gray-400 dark:text-gray-500">
									{$i18n.t(activeTabMeta.description)}
								</p>
							</div>
						</div>
					</div>

					<!-- Tab buttons -->
					{#if isAdmin}
						<div class="grid w-full grid-cols-2 rounded-2xl bg-gray-100 p-1 dark:bg-gray-850 md:inline-flex md:w-fit md:flex-col lg:mt-11 lg:flex-row">
							<button type="button" class={`flex min-w-0 items-center gap-2 rounded-xl px-4 py-2 text-sm font-medium transition-all ${selectedTab === 'personal' ? 'bg-white text-gray-900 shadow-sm dark:bg-gray-800 dark:text-white' : 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'}`} on:click={() => { selectedTab = 'personal'; }}>
								<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="size-4">
									<path fill-rule="evenodd" d="M7.5 6a4.5 4.5 0 119 0 4.5 4.5 0 01-9 0zM3.751 20.105a8.25 8.25 0 0116.498 0 .75.75 0 01-.437.695A18.683 18.683 0 0112 22.5c-2.786 0-5.433-.608-7.812-1.7a.75.75 0 01-.437-.695z" clip-rule="evenodd" />
								</svg>
								<span>{$i18n.t('Personal Settings')}</span>
							</button>
							<button type="button" class={`flex min-w-0 items-center gap-2 rounded-xl px-4 py-2 text-sm font-medium transition-all ${selectedTab === 'global' ? 'bg-white text-gray-900 shadow-sm dark:bg-gray-800 dark:text-white' : 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'}`} on:click={() => { selectedTab = 'global'; }}>
								<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="size-4">
									<path fill-rule="evenodd" d="M11.078 2.25c-.917 0-1.699.663-1.855 1.567L8.982 5.38a7.269 7.269 0 00-.877.506l-1.524-.4a1.875 1.875 0 00-2.164.876l-.922 1.597a1.875 1.875 0 00.31 2.443l1.184.943a7.11 7.11 0 000 1.012l-1.184.944a1.875 1.875 0 00-.31 2.443l.922 1.597a1.875 1.875 0 002.164.876l1.524-.4c.28.19.573.36.877.506l.241 1.563a1.875 1.875 0 001.855 1.567h1.844c.916 0 1.699-.663 1.855-1.567l.24-1.563c.305-.146.598-.316.878-.506l1.524.4a1.875 1.875 0 002.163-.876l.922-1.597a1.875 1.875 0 00-.31-2.443l-1.183-.944a7.11 7.11 0 000-1.012l1.183-.943a1.875 1.875 0 00.31-2.443l-.922-1.597a1.875 1.875 0 00-2.163-.876l-1.524.4a7.268 7.268 0 00-.878-.506l-.24-1.563a1.875 1.875 0 00-1.855-1.567h-1.844zM12 15.75a3.75 3.75 0 100-7.5 3.75 3.75 0 000 7.5z" clip-rule="evenodd" />
								</svg>
								<span>{$i18n.t('Global Settings')}</span>
							</button>
						</div>
					{/if}
				</div>
			</div>
		</section>

		<!-- ==================== Tab Content ==================== -->
		{#if selectedTab === 'personal'}
			<section class="p-5 space-y-3 transition-all duration-300 {personalDirty ? 'glass-section glass-section-dirty' : 'glass-section'}">
				<PersonalAudioSettingsForm
					bind:this={personalAudioForm}
					{saveSettings}
					embedded={true}
					showSubmit={false}
					showScopeBadges={false}
					visualVariant="flat"
					defaultExpandedSections={{ stt: true, tts: true, voice: true }}
					on:dirtyChange={(event) => {
						personalDirty = !!event.detail?.value;
					}}
				/>
			</section>
		{:else if selectedTab === 'global' && isAdmin}
			<section class="p-5 space-y-3 transition-all duration-300 {globalDirty ? 'glass-section glass-section-dirty' : 'glass-section'}">
				<GlobalAudioSettingsForm
					bind:this={globalAudioForm}
					saveHandler={() => {}}
					embedded={true}
					showSubmit={false}
					showScopeBadges={false}
					visualVariant="flat"
					defaultExpandedSections={{ stt: true, tts: true, advanced: true }}
					autoPersistOnEngineChange={false}
					on:dirtyChange={(event) => {
						globalDirty = !!event.detail?.value;
					}}
				/>
			</section>
		{/if}
	</div>
</div>
