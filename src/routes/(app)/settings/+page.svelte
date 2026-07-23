<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { getContext, tick } from 'svelte';
	import type { Writable } from 'svelte/store';

	import LazySettingsPanel from '$lib/components/settings/LazySettingsPanel.svelte';
	import { config, user } from '$lib/stores';
	import { getBackendConfig } from '$lib/apis';

	const i18n: Writable<any> = getContext('i18n');
	const loadPanel = () => import('$lib/components/admin/Settings/General.svelte');
	const saveHandler = async () => {
		toast.success($i18n.t('Settings saved successfully!'));
		await tick();
		await config.set(await getBackendConfig());
	};
</script>

<div class="h-full min-h-0 overflow-y-auto pr-1 scrollbar-hidden space-y-10">
	{#if $user?.role === 'admin'}
		<section class="space-y-2">
			<LazySettingsPanel load={loadPanel} props={{ saveHandler }} />
		</section>
	{/if}
</div>
