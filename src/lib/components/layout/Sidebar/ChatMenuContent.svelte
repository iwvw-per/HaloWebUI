<script lang="ts">
	import { createEventDispatcher, getContext } from 'svelte';
	import type { Writable } from 'svelte/store';
	import { ContextMenu, DropdownMenu } from 'bits-ui';
	import fileSaver from 'file-saver';
	import type { i18n as I18n } from 'i18next';
	import {
		Archive,
		Check,
		Copy,
		Download,
		FolderInput,
		PencilLine,
		Pin,
		PinOff,
		Share2,
		Trash2
	} from 'lucide-svelte';
	import { toast } from 'svelte-sonner';

	import {
		getChatById,
		getChatPinnedStatusById,
		toggleChatPinnedStatusById,
		updateChatFolderIdById
	} from '$lib/apis/chats';
	import { getErrorDetail } from '$lib/apis/response';
	import { downloadChatAsPDF } from '$lib/apis/utils';
	import { createMessagesList } from '$lib/utils';
	import { buildPdfExportMessages, buildPdfFileName } from '$lib/utils/chat-pdf-document';
	import { flyAndScale } from '$lib/utils/transitions';

	const { saveAs } = fileSaver;
	const dispatch = createEventDispatcher();
	type MenuAction = () => void | Promise<void>;
	type ExportChat = {
		chat: {
			title?: string | null;
			history: {
				currentId: string;
				[key: string]: unknown;
			};
		};
	};
	type ExportMessage = { role: string; content: string };

	const i18n = getContext<Writable<I18n>>('i18n');

	export let mode: 'dropdown' | 'context' = 'dropdown';
	export let show = false;
	export let chatId = '';
	export let currentFolderId: string | null = null;
	export let folderOptions: Array<{
		id: string;
		name: string;
		parent_id?: string | null;
		depth: number;
	}> = [];

	export let shareHandler: MenuAction;
	export let cloneChatHandler: MenuAction;
	export let archiveChatHandler: MenuAction;
	export let renameHandler: MenuAction;
	export let deleteHandler: MenuAction;

	let pinned = false;

	$: menu = mode === 'context' ? ContextMenu : DropdownMenu;
	$: contentProps =
		mode === 'context'
			? { collisionPadding: 8 }
			: { sideOffset: -2, side: 'bottom' as const, align: 'start' as const };

	const pinHandler = async () => {
		await toggleChatPinnedStatusById(localStorage.token, chatId);
		dispatch('change');
	};

	const moveToFolder = async (folderId: string | null) => {
		if (folderId === currentFolderId) {
			show = false;
			return;
		}

		const res = await updateChatFolderIdById(localStorage.token, chatId, folderId).catch(
			(error) => {
				toast.error(`${error}`);
				return null;
			}
		);

		if (res) {
			dispatch('change');
			show = false;
			toast.success(folderId === null ? $i18n.t('Removed from group') : $i18n.t('Moved to group'));
		}
	};

	const checkPinned = async () => {
		pinned = Boolean(await getChatPinnedStatusById(localStorage.token, chatId));
	};

	const getChatAsText = (chat: ExportChat) => {
		const history = chat.chat.history;
		const messages = createMessagesList(history, history.currentId) as ExportMessage[];
		const chatText = messages.reduce((text, message) => {
			return `${text}### ${message.role.toUpperCase()}\n${message.content}\n\n`;
		}, '');

		return chatText.trim();
	};

	const downloadTxt = async () => {
		const chat = (await getChatById(localStorage.token, chatId)) as ExportChat | null;
		if (!chat) return;

		const chatText = await getChatAsText(chat);
		const blob = new Blob([chatText], { type: 'text/plain' });
		saveAs(blob, `chat-${chat.chat.title}.txt`);
	};

	const downloadPdf = async () => {
		const targetChat = (await getChatById(localStorage.token, chatId)) as ExportChat | null;
		if (!targetChat?.chat?.history) {
			toast.error($i18n.t('Failed to export PDF'));
			return;
		}

		try {
			const messages = buildPdfExportMessages(targetChat);
			const blob = await downloadChatAsPDF(
				localStorage.token,
				targetChat?.chat?.title ?? 'chat',
				messages
			);

			if (!blob) throw new Error('Failed to export PDF');

			saveAs(blob, buildPdfFileName(targetChat?.chat?.title));
		} catch (error) {
			console.error('Error generating PDF', error);
			toast.error(getErrorDetail(error, $i18n.t('Failed to export PDF')));
		}
	};

	const downloadJSONExport = async () => {
		const chat = (await getChatById(localStorage.token, chatId)) as ExportChat | null;
		if (!chat) return;

		const blob = new Blob([JSON.stringify([chat])], { type: 'application/json' });
		saveAs(blob, `chat-export-${Date.now()}.json`);
	};

	$: if (show) checkPinned();
</script>

<svelte:component
	this={menu.Content}
	{...contentProps}
	class="select-none w-full max-w-[200px] rounded-xl px-1 py-1.5 border border-gray-300/30 dark:border-gray-700/50 z-50 bg-white dark:bg-gray-850 dark:text-white shadow-lg transition"
	transition={flyAndScale}
>
	<svelte:component
		this={menu.Item}
		class="flex gap-2 items-center px-3 py-2 text-sm cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 rounded-md"
		on:click={pinHandler}
	>
		{#if pinned}
			<PinOff class="size-4" strokeWidth={2} />
			<div class="flex items-center">{$i18n.t('Unpin')}</div>
		{:else}
			<Pin class="size-4" strokeWidth={2} />
			<div class="flex items-center">{$i18n.t('Pin')}</div>
		{/if}
	</svelte:component>

	<svelte:component
		this={menu.Item}
		class="flex gap-2 items-center px-3 py-2 text-sm cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 rounded-md"
		on:click={() => renameHandler()}
	>
		<PencilLine class="size-4" strokeWidth={2} />
		<div class="flex items-center">{$i18n.t('Rename')}</div>
	</svelte:component>

	<svelte:component
		this={menu.Item}
		class="flex gap-2 items-center px-3 py-2 text-sm cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 rounded-md"
		on:click={() => cloneChatHandler()}
	>
		<Copy class="size-4" strokeWidth={2} />
		<div class="flex items-center">{$i18n.t('Clone')}</div>
	</svelte:component>

	<svelte:component
		this={menu.Item}
		class="flex gap-2 items-center px-3 py-2 text-sm cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 rounded-md"
		on:click={() => archiveChatHandler()}
	>
		<Archive class="size-4" strokeWidth={2} />
		<div class="flex items-center">{$i18n.t('Archive')}</div>
	</svelte:component>

	<svelte:component
		this={menu.Item}
		class="flex gap-2 items-center px-3 py-2 text-sm cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 rounded-md"
		on:click={() => shareHandler()}
	>
		<Share2 class="size-4" strokeWidth={2} />
		<div class="flex items-center">{$i18n.t('Share')}</div>
	</svelte:component>

	<svelte:component this={menu.Sub}>
		<svelte:component
			this={menu.SubTrigger}
			class="flex gap-2 items-center px-3 py-2 text-sm cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 rounded-md"
		>
			<FolderInput class="size-4" strokeWidth={2} />
			<div class="flex items-center">{$i18n.t('Move to group')}</div>
		</svelte:component>
		<svelte:component
			this={menu.SubContent}
			class="select-none w-full min-w-[180px] max-w-[240px] rounded-xl p-1 z-50 bg-white dark:bg-gray-850 dark:text-white shadow-lg border border-gray-300/30 dark:border-gray-700/50"
			transition={flyAndScale}
			sideOffset={8}
		>
			{#if currentFolderId !== null}
				<svelte:component
					this={menu.Item}
					class="flex gap-2 items-center px-3 py-2 text-sm cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 rounded-md"
					on:click={() => moveToFolder(null)}
				>
					<div class="size-4 shrink-0"></div>
					<div class="flex items-center line-clamp-1">{$i18n.t('Remove from group')}</div>
				</svelte:component>
			{/if}

			{#if folderOptions.length > 0}
				{#if currentFolderId !== null}
					<hr class="border-gray-100 dark:border-gray-800 my-1" />
				{/if}
				{#each folderOptions as folder (folder.id)}
					<svelte:component
						this={menu.Item}
						class="flex gap-2 items-center px-3 py-2 text-sm cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 rounded-md {currentFolderId ===
						folder.id
							? 'text-gray-400 dark:text-gray-500'
							: ''}"
						on:click={() => moveToFolder(folder.id)}
					>
						<Check
							class="size-4 shrink-0 {currentFolderId === folder.id ? 'opacity-100' : 'opacity-0'}"
							strokeWidth={2}
						/>
						<div
							class="min-w-0 flex items-center line-clamp-1"
							style="padding-left: {folder.depth * 0.75}rem"
						>
							{folder.name}
						</div>
					</svelte:component>
				{/each}
			{:else}
				<svelte:component
					this={menu.Item}
					class="flex gap-2 items-center px-3 py-2 text-sm text-gray-400 dark:text-gray-500 cursor-default rounded-md"
					disabled
				>
					<div class="size-4 shrink-0"></div>
					<div class="flex items-center line-clamp-1">{$i18n.t('No chat folders')}</div>
				</svelte:component>
			{/if}
		</svelte:component>
	</svelte:component>

	<svelte:component this={menu.Sub}>
		<svelte:component
			this={menu.SubTrigger}
			class="flex gap-2 items-center px-3 py-2 text-sm cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 rounded-md"
		>
			<Download class="size-4" strokeWidth={2} />
			<div class="flex items-center">{$i18n.t('Download')}</div>
		</svelte:component>
		<svelte:component
			this={menu.SubContent}
			class="select-none w-full rounded-xl p-1 z-50 bg-white dark:bg-gray-850 dark:text-white shadow-lg border border-gray-300/30 dark:border-gray-700/50"
			transition={flyAndScale}
			sideOffset={8}
		>
			<svelte:component
				this={menu.Item}
				class="flex gap-2 items-center px-3 py-2 text-sm cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 rounded-md"
				on:click={downloadJSONExport}
			>
				<div class="flex items-center line-clamp-1">{$i18n.t('Export chat (.json)')}</div>
			</svelte:component>
			<svelte:component
				this={menu.Item}
				class="flex gap-2 items-center px-3 py-2 text-sm cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 rounded-md"
				on:click={downloadTxt}
			>
				<div class="flex items-center line-clamp-1">{$i18n.t('Plain text (.txt)')}</div>
			</svelte:component>
			<svelte:component
				this={menu.Item}
				class="flex gap-2 items-center px-3 py-2 text-sm cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 rounded-md"
				on:click={downloadPdf}
			>
				<div class="flex items-center line-clamp-1">{$i18n.t('PDF document (.pdf)')}</div>
			</svelte:component>
		</svelte:component>
	</svelte:component>

	<hr class="border-gray-100 dark:border-gray-800 my-1" />
	<svelte:component
		this={menu.Item}
		class="flex gap-2 items-center px-3 py-2 text-sm cursor-pointer text-red-500 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/50 rounded-md"
		on:click={() => deleteHandler()}
	>
		<Trash2 class="size-4" strokeWidth={2} />
		<div class="flex items-center">{$i18n.t('Delete')}</div>
	</svelte:component>
</svelte:component>
