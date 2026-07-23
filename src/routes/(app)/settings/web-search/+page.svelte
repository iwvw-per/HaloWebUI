<script lang="ts">
	import { getContext, tick } from 'svelte';
	import { toast } from 'svelte-sonner';
	import type { Writable } from 'svelte/store';

	import LazySettingsPanel from '$lib/components/settings/LazySettingsPanel.svelte';
	import { config, user } from '$lib/stores';
	import { getBackendConfig } from '$lib/apis';

	const i18n: Writable<any> = getContext('i18n');
	const loadPanel = () => import('$lib/components/admin/Settings/WebSearch.svelte');
	const saveHandler = async () => {
		toast.success($i18n.t('Settings saved successfully!'));
		await tick();
		await config.set(await getBackendConfig());
	};
</script>

{#if $user?.role === 'admin'}
	<LazySettingsPanel load={loadPanel} props={{ saveHandler }} />
{/if}
