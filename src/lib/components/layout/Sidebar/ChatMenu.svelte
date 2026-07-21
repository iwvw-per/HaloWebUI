<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { DropdownMenu } from 'bits-ui';
	import { flyAndScale } from '$lib/utils/transitions';
	import { getContext, createEventDispatcher } from 'svelte';

	import fileSaver from 'file-saver';
	const { saveAs } = fileSaver;

	const dispatch = createEventDispatcher();

	import Dropdown from '$lib/components/common/Dropdown.svelte';
	import Tooltip from '$lib/components/common/Tooltip.svelte';
	import {
		Pin,
		PinOff,
		PencilLine,
		Copy,
		Archive,
		Share2,
		Download,
		Trash2,
		FolderInput,
		Check
	} from 'lucide-svelte';
	import {
		getChatById,
		getChatPinnedStatusById,
		toggleChatPinnedStatusById,
		updateChatFolderIdById
	} from '$lib/apis/chats';
	import { createMessagesList } from '$lib/utils';
	import { downloadChatAsPDF } from '$lib/apis/utils';
	import { buildPdfExportMessages, buildPdfFileName } from '$lib/utils/chat-pdf-document';
	import { getErrorDetail } from '$lib/apis/response';

	const i18n = getContext('i18n');

	export let shareHandler: Function;
	export let cloneChatHandler: Function;
	export let archiveChatHandler: Function;
	export let renameHandler: Function;
	export let deleteHandler: Function;
	export let onClose: Function;

	export let chatId = '';
	export let currentFolderId: string | null = null;
	export let folderOptions: Array<{
		id: string;
		name: string;
		parent_id?: string | null;
		depth: number;
	}> = [];

	export let show = false;
	let pinned = false;

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
		pinned = await getChatPinnedStatusById(localStorage.token, chatId);
	};

	const getChatAsText = async (chat) => {
		const history = chat.chat.history;
		const messages = createMessagesList(history, history.currentId);
		const chatText = messages.reduce((a, message, i, arr) => {
			return `${a}### ${message.role.toUpperCase()}\n${message.content}\n\n`;
		}, '');

		return chatText.trim();
	};

	const downloadTxt = async () => {
		const chat = await getChatById(localStorage.token, chatId);
		if (!chat) {
			return;
		}

		const chatText = await getChatAsText(chat);
		let blob = new Blob([chatText], {
			type: 'text/plain'
		});

		saveAs(blob, `chat-${chat.chat.title}.txt`);
	};

	const downloadPdf = async () => {
		const targetChat = await getChatById(localStorage.token, chatId);
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

			if (!blob) {
				throw new Error('Failed to export PDF');
			}

			saveAs(blob, buildPdfFileName(targetChat?.chat?.title));
		} catch (error) {
			console.error('Error generating PDF', error);
			toast.error(getErrorDetail(error, $i18n.t('Failed to export PDF')));
		}
	};

	const downloadJSONExport = async () => {
		const chat = await getChatById(localStorage.token, chatId);

		if (chat) {
			let blob = new Blob([JSON.stringify([chat])], {
				type: 'application/json'
			});
			saveAs(blob, `chat-export-${Date.now()}.json`);
		}
	};

	$: if (show) {
		checkPinned();
	}
</script>

<Dropdown
	bind:show
	on:change={(e) => {
		if (e.detail === false) {
			onClose();
		}
	}}
>
	<Tooltip content={$i18n.t('More')}>
		<slot />
	</Tooltip>

	<div slot="content">
		<DropdownMenu.Content
			class="select-none w-full max-w-[200px] rounded-xl px-1 py-1.5 border border-gray-300/30 dark:border-gray-700/50 z-50 bg-white dark:bg-gray-850 dark:text-white shadow-lg transition"
			sideOffset={-2}
			side="bottom"
			align="start"
			transition={flyAndScale}
		>
			<DropdownMenu.Item
				class="flex gap-2 items-center px-3 py-2 text-sm cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 rounded-md"
				on:click={() => {
					pinHandler();
				}}
			>
				{#if pinned}
					<PinOff class="size-4" strokeWidth={2} />
					<div class="flex items-center">{$i18n.t('Unpin')}</div>
				{:else}
					<Pin class="size-4" strokeWidth={2} />
					<div class="flex items-center">{$i18n.t('Pin')}</div>
				{/if}
			</DropdownMenu.Item>

			<DropdownMenu.Item
				class="flex gap-2 items-center px-3 py-2 text-sm cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 rounded-md"
				on:click={() => {
					renameHandler();
				}}
			>
				<PencilLine class="size-4" strokeWidth={2} />
				<div class="flex items-center">{$i18n.t('Rename')}</div>
			</DropdownMenu.Item>

			<DropdownMenu.Item
				class="flex gap-2 items-center px-3 py-2 text-sm cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 rounded-md"
				on:click={() => {
					cloneChatHandler();
				}}
			>
				<Copy class="size-4" strokeWidth={2} />
				<div class="flex items-center">{$i18n.t('Clone')}</div>
			</DropdownMenu.Item>

			<DropdownMenu.Item
				class="flex gap-2 items-center px-3 py-2 text-sm cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 rounded-md"
				on:click={() => {
					archiveChatHandler();
				}}
			>
				<Archive class="size-4" strokeWidth={2} />
				<div class="flex items-center">{$i18n.t('Archive')}</div>
			</DropdownMenu.Item>

			<DropdownMenu.Item
				class="flex gap-2 items-center px-3 py-2 text-sm cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 rounded-md"
				on:click={() => {
					shareHandler();
				}}
			>
				<Share2 class="size-4" strokeWidth={2} />
				<div class="flex items-center">{$i18n.t('Share')}</div>
			</DropdownMenu.Item>

			<DropdownMenu.Sub>
				<DropdownMenu.SubTrigger
					class="flex gap-2 items-center px-3 py-2 text-sm cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 rounded-md"
				>
					<FolderInput class="size-4" strokeWidth={2} />
					<div class="flex items-center">{$i18n.t('Move to group')}</div>
				</DropdownMenu.SubTrigger>
				<DropdownMenu.SubContent
					class="select-none w-full min-w-[180px] max-w-[240px] rounded-xl p-1 z-50 bg-white dark:bg-gray-850 dark:text-white shadow-lg border border-gray-300/30 dark:border-gray-700/50"
					transition={flyAndScale}
					sideOffset={8}
				>
					{#if currentFolderId !== null}
						<DropdownMenu.Item
							class="flex gap-2 items-center px-3 py-2 text-sm cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 rounded-md"
							on:click={() => {
								moveToFolder(null);
							}}
						>
							<div class="size-4 shrink-0"></div>
							<div class="flex items-center line-clamp-1">{$i18n.t('Remove from group')}</div>
						</DropdownMenu.Item>
					{/if}

					{#if folderOptions.length > 0}
						{#if currentFolderId !== null}
							<hr class="border-gray-100 dark:border-gray-800 my-1" />
						{/if}
						{#each folderOptions as folder (folder.id)}
							<DropdownMenu.Item
								class="flex gap-2 items-center px-3 py-2 text-sm cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 rounded-md {currentFolderId ===
								folder.id
									? 'text-gray-400 dark:text-gray-500'
									: ''}"
								on:click={() => {
									moveToFolder(folder.id);
								}}
							>
								<Check
									class="size-4 shrink-0 {currentFolderId === folder.id
										? 'opacity-100'
										: 'opacity-0'}"
									strokeWidth={2}
								/>
								<div
									class="min-w-0 flex items-center line-clamp-1"
									style="padding-left: {folder.depth * 0.75}rem"
								>
									{folder.name}
								</div>
							</DropdownMenu.Item>
						{/each}
					{:else}
						<DropdownMenu.Item
							class="flex gap-2 items-center px-3 py-2 text-sm text-gray-400 dark:text-gray-500 cursor-default rounded-md"
							disabled
						>
							<div class="size-4 shrink-0"></div>
							<div class="flex items-center line-clamp-1">{$i18n.t('No chat folders')}</div>
						</DropdownMenu.Item>
					{/if}
				</DropdownMenu.SubContent>
			</DropdownMenu.Sub>

			<DropdownMenu.Sub>
				<DropdownMenu.SubTrigger
					class="flex gap-2 items-center px-3 py-2 text-sm cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 rounded-md"
				>
					<Download class="size-4" strokeWidth={2} />

					<div class="flex items-center">{$i18n.t('Download')}</div>
				</DropdownMenu.SubTrigger>
				<DropdownMenu.SubContent
					class="select-none w-full rounded-xl p-1 z-50 bg-white dark:bg-gray-850 dark:text-white shadow-lg border border-gray-300/30 dark:border-gray-700/50"
					transition={flyAndScale}
					sideOffset={8}
				>
					<DropdownMenu.Item
						class="flex gap-2 items-center px-3 py-2 text-sm cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 rounded-md"
						on:click={() => {
							downloadJSONExport();
						}}
					>
						<div class="flex items-center line-clamp-1">{$i18n.t('Export chat (.json)')}</div>
					</DropdownMenu.Item>
					<DropdownMenu.Item
						class="flex gap-2 items-center px-3 py-2 text-sm cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 rounded-md"
						on:click={() => {
							downloadTxt();
						}}
					>
						<div class="flex items-center line-clamp-1">{$i18n.t('Plain text (.txt)')}</div>
					</DropdownMenu.Item>

					<DropdownMenu.Item
						class="flex gap-2 items-center px-3 py-2 text-sm cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 rounded-md"
						on:click={() => {
							downloadPdf();
						}}
					>
						<div class="flex items-center line-clamp-1">{$i18n.t('PDF document (.pdf)')}</div>
					</DropdownMenu.Item>
				</DropdownMenu.SubContent>
			</DropdownMenu.Sub>
			<hr class="border-gray-100 dark:border-gray-800 my-1" />
			<DropdownMenu.Item
				class="flex gap-2 items-center px-3 py-2 text-sm cursor-pointer text-red-500 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/50 rounded-md"
				on:click={() => {
					deleteHandler();
				}}
			>
				<Trash2 class="size-4" strokeWidth={2} />
				<div class="flex items-center">{$i18n.t('Delete')}</div>
			</DropdownMenu.Item>
		</DropdownMenu.Content>
	</div>
</Dropdown>
