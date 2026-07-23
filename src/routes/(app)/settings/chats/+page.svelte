<script lang="ts">
	// Historical route path: /settings/chats renders the Data Management page.
	import LazySettingsPanel from '$lib/components/settings/LazySettingsPanel.svelte';

	const loadPanel = () => import('$lib/components/settings/DataManagementSettingsPage.svelte');
</script>

<LazySettingsPanel load={loadPanel} />
