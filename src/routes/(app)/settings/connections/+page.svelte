<script lang="ts">
	import LazySettingsPanel from '$lib/components/settings/LazySettingsPanel.svelte';
	import { getContext } from 'svelte';
	import type { UserSettingsContext } from '$lib/types/user-settings';

	const { saveSettings } = getContext<UserSettingsContext>('user-settings');
	const loadPanel = () => import('$lib/components/settings/ConnectionsPreferences.svelte');
</script>

<LazySettingsPanel load={loadPanel} props={{ saveSettings }} />
