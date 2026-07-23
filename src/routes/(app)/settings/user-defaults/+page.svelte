<script lang="ts">
	import LazySettingsPanel from '$lib/components/settings/LazySettingsPanel.svelte';

	const loadPanel = () =>
		import('$lib/components/admin/UserDefaults/NewUserDefaultSettings.svelte');
</script>

<LazySettingsPanel load={loadPanel} />
