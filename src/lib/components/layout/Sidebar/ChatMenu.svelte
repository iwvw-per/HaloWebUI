<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import { ContextMenu } from 'bits-ui';

	import Dropdown from '$lib/components/common/Dropdown.svelte';
	import ChatMenuContent from './ChatMenuContent.svelte';

	const dispatch = createEventDispatcher();
	type MenuAction = () => void | Promise<void>;

	export let mode: 'dropdown' | 'context' = 'dropdown';
	export let shareHandler: MenuAction;
	export let cloneChatHandler: MenuAction;
	export let archiveChatHandler: MenuAction;
	export let renameHandler: MenuAction;
	export let deleteHandler: MenuAction;
	export let onClose: () => void;

	export let chatId = '';
	export let currentFolderId: string | null = null;
	export let folderOptions: Array<{
		id: string;
		name: string;
		parent_id?: string | null;
		depth: number;
	}> = [];
	export let show = false;

	const handleOpenChange = (open: boolean) => {
		dispatch('open-change', open);
		if (!open) onClose();
	};
</script>

{#if mode === 'context'}
	<ContextMenu.Root
		bind:open={show}
		closeFocus={false}
		typeahead={false}
		onOpenChange={handleOpenChange}
	>
		<ContextMenu.Trigger class="contents">
			<slot />
		</ContextMenu.Trigger>

		<ChatMenuContent
			bind:show
			mode="context"
			{chatId}
			{currentFolderId}
			{folderOptions}
			{shareHandler}
			{cloneChatHandler}
			{archiveChatHandler}
			{renameHandler}
			{deleteHandler}
			on:change={() => dispatch('change')}
		/>
	</ContextMenu.Root>
{:else}
	<Dropdown
		bind:show
		on:change={(event) => {
			handleOpenChange(event.detail);
		}}
	>
		<slot />

		<div slot="content">
			<ChatMenuContent
				bind:show
				mode="dropdown"
				{chatId}
				{currentFolderId}
				{folderOptions}
				{shareHandler}
				{cloneChatHandler}
				{archiveChatHandler}
				{renameHandler}
				{deleteHandler}
				on:change={() => dispatch('change')}
			/>
		</div>
	</Dropdown>
{/if}
