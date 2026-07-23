<script lang="ts">
	import LazySettingsPanel from '$lib/components/settings/LazySettingsPanel.svelte';
	import { user } from '$lib/stores';

	const loadPanel = () => import('$lib/components/admin/Settings/Models.svelte');
</script>

{#if $user?.role === 'admin'}
	<LazySettingsPanel load={loadPanel} />
{/if}
