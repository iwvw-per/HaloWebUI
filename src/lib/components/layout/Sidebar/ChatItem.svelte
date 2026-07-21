<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { goto } from '$app/navigation';
	import { getContext, createEventDispatcher, tick } from 'svelte';
	const i18n = getContext('i18n');

	const dispatch = createEventDispatcher();

	import {
		archiveChatById,
		cloneChatById,
		deleteChatById,
		getAllTags,
		getChatList,
		getPinnedChatList,
		toggleChatPinnedStatusById,
		updateChatTitleById
	} from '$lib/apis/chats';
	import {
		chatId,
		chatTitle as _chatTitle,
		chats,
		mobile,
		pinnedChats,
		selectedAssistantScene,
		showSidebar,
		currentChatPage,
		tags,
		activeChatIds
	} from '$lib/stores';

	import ChatMenu from './ChatMenu.svelte';
	import DeleteConfirmDialog from '$lib/components/common/ConfirmDialog.svelte';
	import ShareChatModal from '$lib/components/chat/ShareChatModal.svelte';
	import GarbageBin from '$lib/components/icons/GarbageBin.svelte';
	import Tooltip from '$lib/components/common/Tooltip.svelte';
	import ArchiveBox from '$lib/components/icons/ArchiveBox.svelte';
	import Check from '$lib/components/icons/Check.svelte';
	import XMark from '$lib/components/icons/XMark.svelte';
	import { Pin } from 'lucide-svelte';

	export let className = '';
	export let uiStyle: 'flat' | 'card' = 'flat';
	export let variant: 'default' | 'folder' = 'default';

	export let id;
	export let title;
	export let assistantId: string | null = null;
	export let folderId: string | null = null;
	export let folderOptions: Array<{
		id: string;
		name: string;
		parent_id?: string | null;
		depth: number;
	}> = [];

	export let selected = false;
	export let shiftKey = false;

	$: isFolderVariant = variant === 'folder';
	$: isPinned = $pinnedChats?.some((chat) => chat.id === id) ?? false;

	$: itemShellClass = isFolderVariant
		? 'w-full flex justify-between rounded-md border border-transparent px-2.5 py-1.5 text-[13px] transition-colors duration-150'
		: uiStyle === 'card'
			? 'w-full flex justify-between rounded-xl border border-transparent px-3 py-2 transition-colors duration-150'
			: 'w-full flex justify-between rounded-lg px-3 py-2 transition-colors duration-150';

	$: itemStateClass = isFolderVariant
		? id === $chatId || confirmEdit
			? 'bg-white text-gray-900 border-gray-200/80 shadow-sm font-medium dark:bg-gray-900/80 dark:text-gray-100 dark:border-gray-700/70'
			: selected
				? 'bg-white/70 text-gray-800 border-gray-200/60 dark:bg-gray-900/55 dark:text-gray-200 dark:border-gray-800/70'
				: 'text-gray-600 group-hover:bg-white/70 group-hover:border-gray-200/70 dark:text-gray-300 dark:group-hover:bg-gray-900/55 dark:group-hover:border-gray-800/70'
		: uiStyle === 'card'
			? id === $chatId || confirmEdit
				? 'bg-white/85 dark:bg-gray-900/60 border-gray-200/70 dark:border-gray-800/70 shadow-sm font-medium'
				: selected
					? 'bg-white/65 dark:bg-gray-900/45 border-gray-200/60 dark:border-gray-800/60'
					: 'group-hover:bg-white/60 dark:group-hover:bg-gray-900/40 group-hover:border-gray-200/60 dark:group-hover:border-gray-800/60'
			: id === $chatId || confirmEdit
				? 'bg-gray-200 dark:bg-gray-800 font-medium'
				: selected
					? 'bg-gray-100 dark:bg-gray-850'
					: 'group-hover:bg-gray-100 dark:group-hover:bg-gray-850';

	$: menuFromClass = isFolderVariant
		? id === $chatId || confirmEdit
			? 'from-white dark:from-gray-900'
			: selected
				? 'from-white/70 dark:from-gray-900/55'
				: 'from-white/70 dark:from-gray-900/55'
		: uiStyle === 'card'
			? id === $chatId || confirmEdit
				? 'from-white/85 dark:from-gray-900/60'
				: selected
					? 'from-white/65 dark:from-gray-900/45'
					: 'from-white/60 dark:from-gray-900/40'
			: id === $chatId || confirmEdit
				? 'from-gray-200 dark:from-gray-800'
				: selected
					? 'from-gray-100 dark:from-gray-850'
					: 'from-gray-100 dark:from-gray-850';

	$: titleClass = isFolderVariant
		? 'text-left self-center overflow-hidden w-full h-[19px] leading-5'
		: 'text-left self-center overflow-hidden w-full h-[20px]';

	$: menuOffsetClass = isFolderVariant ? 'top-[4px]' : 'top-[6px]';

	let mouseOver = false;

	let showShareChatModal = false;
	let confirmEdit = false;
	let showChatMenu = false;

	let chatTitle = title;

	const editChatTitle = async (id, requestedTitle) => {
		if (requestedTitle === '') {
			toast.error($i18n.t('Title cannot be an empty string.'));
		} else {
			const updatedChat = await updateChatTitleById(localStorage.token, id, requestedTitle).catch(
				(error) => {
					toast.error(`${error}`);
					return null;
				}
			);

			if (!updatedChat) {
				return;
			}

			const nextTitle = updatedChat.title ?? requestedTitle;

			if (id === $chatId) {
				_chatTitle.set(nextTitle);
			}

			title = nextTitle;

			chats.update(
				(list) =>
					list?.map((chat) => (chat.id === id ? { ...chat, title: nextTitle } : chat)) ?? list
			);
			pinnedChats.update(
				(list) =>
					list?.map((chat) => (chat.id === id ? { ...chat, title: nextTitle } : chat)) ?? list
			);

			dispatch('title-change', { id, title: nextTitle, chat: updatedChat });
		}
	};

	const cloneChatHandler = async (id) => {
		const res = await cloneChatById(
			localStorage.token,
			id,
			$i18n.t('Clone of {{TITLE}}', {
				TITLE: title
			})
		).catch((error) => {
			toast.error(`${error}`);
			return null;
		});

		if (res) {
			goto(`/c/${res.id}`);

			currentChatPage.set(1);
			await chats.set(await getChatList(localStorage.token, $currentChatPage));
			await pinnedChats.set(await getPinnedChatList(localStorage.token));
		}
	};

	const deleteChatHandler = async (id) => {
		const res = await deleteChatById(localStorage.token, id).catch((error) => {
			toast.error(`${error}`);
			return null;
		});

		if (res) {
			tags.set(await getAllTags(localStorage.token));
			if ($chatId === id) {
				await goto('/');

				await chatId.set('');
				await tick();
			}

			dispatch('change');
		}
	};

	const archiveChatHandler = async (id) => {
		await archiveChatById(localStorage.token, id);

		if ($chatId === id) {
			await goto('/');
			await chatId.set('');
			await tick();
		}

		currentChatPage.set(1);
		await chats.set(await getChatList(localStorage.token, $currentChatPage));
		await pinnedChats.set(await getPinnedChatList(localStorage.token));

		dispatch('change');
	};

	const pinChatHandler = async (id) => {
		await toggleChatPinnedStatusById(localStorage.token, id);

		currentChatPage.set(1);
		await chats.set(await getChatList(localStorage.token, $currentChatPage));
		await pinnedChats.set(await getPinnedChatList(localStorage.token));

		dispatch('change');
	};

	const focusEdit = async (node: HTMLInputElement) => {
		node.focus();
	};

	let itemElement;

	let showDeleteConfirm = false;

	const chatTitleInputKeydownHandler = (e) => {
		if (e.key === 'Enter') {
			e.preventDefault();
			editChatTitle(id, chatTitle);
			confirmEdit = false;
			chatTitle = '';
		} else if (e.key === 'Escape') {
			e.preventDefault();
			confirmEdit = false;
			chatTitle = '';
		}
	};
</script>

<ShareChatModal bind:show={showShareChatModal} chatId={id} />

<DeleteConfirmDialog
	bind:show={showDeleteConfirm}
	title={$i18n.t('Delete chat?')}
	on:confirm={() => {
		deleteChatHandler(id);
	}}
>
	<div class=" text-sm text-gray-500 flex-1 line-clamp-3">
		{$i18n.t('This will delete')} <span class="  font-semibold">{title}</span>.
	</div>
</DeleteConfirmDialog>

<div bind:this={itemElement} class=" w-full {className} relative group mb-1 last:mb-0">
	{#if confirmEdit}
		<div class="{itemShellClass} {itemStateClass} whitespace-nowrap text-ellipsis">
			<input
				use:focusEdit
				bind:value={chatTitle}
				id="chat-title-input-{id}"
				class=" bg-transparent w-full outline-hidden mr-10"
				on:keydown={chatTitleInputKeydownHandler}
			/>
		</div>
	{:else}
		<a
			class="{itemShellClass} {itemStateClass} whitespace-nowrap text-ellipsis"
			href="/c/{id}"
			on:click={() => {
				dispatch('select');

				if ($selectedAssistantScene && $selectedAssistantScene.id !== assistantId) {
					selectedAssistantScene.set(null);
				}

				if ($mobile) {
					showSidebar.set(false);
				}
			}}
			on:dblclick={() => {
				chatTitle = title;
				confirmEdit = true;
			}}
			on:contextmenu|preventDefault={() => {
				dispatch('select');
				showChatMenu = true;
			}}
			on:mouseenter={(e) => {
				mouseOver = true;
			}}
			on:mouseleave={(e) => {
				mouseOver = false;
			}}
			on:focus={(e) => {}}
			draggable="false"
		>
			<div class=" flex self-center flex-1 w-full">
				<div dir="auto" class={titleClass}>
					{title}
				</div>
				{#if $activeChatIds.has(id)}
					<div class="flex-shrink-0 self-center ml-1">
						<span class="relative flex h-2 w-2">
							<span
								class="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-400 opacity-75"
							></span>
							<span class="relative inline-flex rounded-full h-2 w-2 bg-blue-500"></span>
						</span>
					</div>
				{/if}
			</div>
		</a>
	{/if}

	<!-- svelte-ignore a11y-no-static-element-interactions -->
	<div
		class="{menuFromClass} absolute {className === 'pr-2'
			? 'right-[8px]'
			: 'right-0'} {menuOffsetClass} py-0.5 pr-0.5 mr-1.5 pl-5 bg-linear-to-l from-80%

              to-transparent"
		on:mouseenter={(e) => {
			mouseOver = true;
		}}
		on:mouseleave={(e) => {
			mouseOver = false;
		}}
	>
		{#if confirmEdit}
			<div
				class="flex self-center items-center space-x-1.5 z-10 translate-y-[0.5px] -translate-x-[0.5px]"
			>
				<Tooltip content={$i18n.t('Confirm')}>
					<button
						class=" self-center dark:hover:text-white transition"
						on:click={() => {
							editChatTitle(id, chatTitle);
							confirmEdit = false;
							chatTitle = '';
						}}
					>
						<Check className=" size-3.5" strokeWidth="2.5" />
					</button>
				</Tooltip>

				<Tooltip content={$i18n.t('Cancel')}>
					<button
						class=" self-center dark:hover:text-white transition"
						on:click={() => {
							confirmEdit = false;
							chatTitle = '';
						}}
					>
						<XMark strokeWidth="2.5" />
					</button>
				</Tooltip>
			</div>
		{:else if shiftKey && mouseOver}
			<div class=" flex items-center self-center space-x-1.5">
				<Tooltip content={$i18n.t('Archive')} className="flex items-center">
					<button
						class=" self-center dark:hover:text-white transition"
						on:click={() => {
							archiveChatHandler(id);
						}}
						type="button"
					>
						<ArchiveBox className="size-4  translate-y-[0.5px]" strokeWidth="2" />
					</button>
				</Tooltip>

				<Tooltip content={$i18n.t('Delete')}>
					<button
						class=" self-center dark:hover:text-white transition"
						on:click={() => {
							deleteChatHandler(id);
						}}
						type="button"
					>
						<GarbageBin strokeWidth="2" />
					</button>
				</Tooltip>
			</div>
		{:else}
			<div class="flex h-6 w-[78px] shrink-0 items-center justify-end gap-0.5 z-10">
				<Tooltip content={isPinned ? $i18n.t('Unpin') : $i18n.t('Pin')}>
					<button
						aria-label={isPinned ? $i18n.t('Unpin') : $i18n.t('Pin')}
						class="flex size-6 items-center justify-center rounded-md p-0 text-gray-500 transition hover:bg-black/5 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-white/5 dark:hover:text-white"
						on:click|stopPropagation={() => {
							pinChatHandler(id);
						}}
						type="button"
					>
						<Pin
							class="size-3.5"
							strokeWidth={2.2}
							fill={isPinned ? 'currentColor' : 'none'}
							style={`transform: rotate(${isPinned ? 40 : 0}deg); transform-origin: center;`}
						/>
					</button>
				</Tooltip>

				<Tooltip content={$i18n.t('Archive')}>
					<button
						aria-label={$i18n.t('Archive')}
						class="flex size-6 items-center justify-center rounded-md p-0 text-gray-500 transition hover:bg-black/5 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-white/5 dark:hover:text-white"
						on:click|stopPropagation={() => {
							archiveChatHandler(id);
						}}
						type="button"
					>
						<ArchiveBox className="size-4 translate-y-[0.5px]" strokeWidth="2" />
					</button>
				</Tooltip>

				<ChatMenu
					bind:show={showChatMenu}
					chatId={id}
					currentFolderId={folderId}
					{folderOptions}
					cloneChatHandler={() => {
						cloneChatHandler(id);
					}}
					shareHandler={() => {
						showShareChatModal = true;
					}}
					archiveChatHandler={() => {
						archiveChatHandler(id);
					}}
					renameHandler={async () => {
						chatTitle = title;
						confirmEdit = true;

						await tick();
						const input = document.getElementById(`chat-title-input-${id}`);
						if (input) {
							input.focus();
						}
					}}
					deleteHandler={() => {
						showDeleteConfirm = true;
					}}
					onClose={() => {
						dispatch('unselect');
					}}
					on:change={async () => {
						dispatch('change');
					}}
					on:tag={(e) => {
						dispatch('tag', e.detail);
					}}
				>
					<button
						aria-label="Chat Menu"
						class="flex size-6 items-center justify-center rounded-md p-0 dark:hover:text-white transition"
						on:click={() => {
							dispatch('select');
						}}
					>
						<svg
							xmlns="http://www.w3.org/2000/svg"
							viewBox="0 0 16 16"
							fill="currentColor"
							class="w-4 h-4"
						>
							<path
								d="M2 8a1.5 1.5 0 1 1 3 0 1.5 1.5 0 0 1-3 0ZM6.5 8a1.5 1.5 0 1 1 3 0 1.5 1.5 0 0 1-3 0ZM12.5 6.5a1.5 1.5 0 1 0 0 3 1.5 1.5 0 0 0 0-3Z"
							/>
						</svg>
					</button>
				</ChatMenu>

				{#if id === $chatId}
					<!-- Shortcut support using "delete-chat-button" id -->
					<button
						id="delete-chat-button"
						class="hidden"
						on:click={() => {
							showDeleteConfirm = true;
						}}
					>
						<svg
							xmlns="http://www.w3.org/2000/svg"
							viewBox="0 0 16 16"
							fill="currentColor"
							class="w-4 h-4"
						>
							<path
								d="M2 8a1.5 1.5 0 1 1 3 0 1.5 1.5 0 0 1-3 0ZM6.5 8a1.5 1.5 0 1 1 3 0 1.5 1.5 0 0 1-3 0ZM12.5 6.5a1.5 1.5 0 1 0 0 3 1.5 1.5 0 0 0 0-3Z"
							/>
						</svg>
					</button>
				{/if}
			</div>
		{/if}
	</div>
</div>
