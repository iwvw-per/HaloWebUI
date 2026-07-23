<script lang="ts">
	import LazySettingsPanel from '$lib/components/settings/LazySettingsPanel.svelte';

	const loadPanel = () => import('$lib/components/settings/AudioSettingsPage.svelte');
</script>

<LazySettingsPanel load={loadPanel} />
