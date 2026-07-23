<script lang="ts">
	import LazySettingsPanel from '$lib/components/settings/LazySettingsPanel.svelte';

	const loadPanel = () => import('$lib/components/admin/Analytics.svelte');
</script>

<LazySettingsPanel load={loadPanel} />
