<script lang="ts">
	import { user as currentUser } from '$lib/stores';
	import Messages from '$lib/components/chat/Messages.svelte';
	import type { ChatPdfExportMode } from '$lib/utils/chat-pdf-export';

	export let chat: any = null;
	export let visible = false;
	export let darkMode = false;
	export let mode: ChatPdfExportMode = 'stylized';
	export let container: HTMLDivElement | null = null;

	const noop = async () => {};

	let autoScroll = false;
	let prompt = '';
</script>

{#if visible && chat?.chat?.history}
	<div
		bind:this={container}
		class="pdf-export-shell"
		data-mode={mode}
		data-theme={darkMode ? 'dark' : 'light'}
		aria-hidden="true"
	>
		<div class="pdf-export-surface">
			<div class="pdf-export-header">
				<div class="pdf-export-badge">Halo WebUI</div>
				<h1 class="pdf-export-title">{chat?.chat?.title ?? 'Chat Export'}</h1>
			</div>

			<Messages
				className="flex pt-4 pb-8 w-full"
				chatId={`pdf-export-${chat?.id ?? 'chat'}`}
				user={$currentUser}
				history={chat.chat.history}
				{prompt}
				selectedModels={[]}
				atSelectedModel={null}
				sendPrompt={noop}
				showMessage={noop}
				submitMessage={noop}
				continueResponse={noop}
				regenerateResponse={noop}
				mergeResponses={noop}
				chatActionHandler={noop}
				addMessages={noop}
				readOnly={true}
				showAllMessages={true}
				bottomPadding={false}
				{autoScroll}
			/>
		</div>
	</div>
{/if}

<style>
	.pdf-export-shell {
		position: fixed;
		top: 0;
		left: -20000px;
		width: 820px;
		pointer-events: none;
		z-index: -1;
	}

	.pdf-export-surface {
		width: 100%;
		padding: 28px 0 40px;
		background: #ffffff;
		color: #111827;
	}

	.pdf-export-shell[data-theme='dark'][data-mode='stylized'] .pdf-export-surface {
		background: #020617;
		color: #f8fafc;
	}

	.pdf-export-shell[data-mode='compact'] .pdf-export-surface {
		background: #ffffff;
		color: #111827;
	}

	.pdf-export-header {
		max-width: 64rem;
		margin: 0 auto;
		padding: 0 2rem 0.5rem;
	}

	.pdf-export-badge {
		display: inline-flex;
		align-items: center;
		padding: 0.25rem 0.75rem;
		border-radius: 999px;
		font-size: 0.75rem;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		background: rgba(15, 23, 42, 0.08);
		color: inherit;
	}

	.pdf-export-shell[data-theme='dark'][data-mode='stylized'] .pdf-export-badge {
		background: rgba(148, 163, 184, 0.16);
	}

	.pdf-export-title {
		margin: 0.9rem 0 0;
		font-size: 1.9rem;
		font-weight: 700;
		line-height: 1.2;
		word-break: break-word;
	}

	:global(.pdf-export-shell button) {
		pointer-events: none !important;
	}

	:global(.pdf-export-shell [class*='animate-']) {
		animation: none !important;
	}

	:global(.pdf-export-shell [class*='transition']) {
		transition: none !important;
	}

	:global(.pdf-export-shell [class*='backdrop-blur']) {
		backdrop-filter: none !important;
	}

	:global(.pdf-export-shell[data-mode='compact'] [class*='shadow']) {
		box-shadow: none !important;
	}
</style>
