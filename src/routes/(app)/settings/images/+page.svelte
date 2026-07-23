<script lang="ts">
	import { getContext } from 'svelte';
	import { toast } from 'svelte-sonner';
	import type { Writable } from 'svelte/store';

	import LazySettingsPanel from '$lib/components/settings/LazySettingsPanel.svelte';
	import { user } from '$lib/stores';

	const i18n: Writable<any> = getContext('i18n');
	const loadPanel = () => import('$lib/components/admin/Settings/Images.svelte');
	const handleSave = () => {
		toast.success($i18n.t('Settings saved successfully!'));
	};
</script>

{#if $user?.role === 'admin'}
	<LazySettingsPanel load={loadPanel} on:save={handleSave} />
{/if}
