<script lang="ts">
	import { onDestroy, onMount } from 'svelte';

	import {
		DEFAULT_CHAT_TRANSITION_MODE,
		type ChatTransitionMode
	} from '$lib/utils/lobehub-chat-appearance';

	import Markdown from '$lib/components/chat/Messages/Markdown.svelte';

	import {
		CHAT_TRANSITION_PREVIEW_CONTENT,
		CHAT_TRANSITION_PREVIEW_STREAMING_SPEED,
		getPreviewChunkStep,
		getPreviewInitialContent
	} from './chatTransitionPreviewFrames';

	export let mode: ChatTransitionMode = DEFAULT_CHAT_TRANSITION_MODE;

	let streamedContent = '';
	let isStreaming = true;
	let intervalId: ReturnType<typeof setInterval> | null = null;

	const startStreaming = () => {
		stopStreaming();

		streamedContent = getPreviewInitialContent(mode);
		isStreaming = true;

		const chunkStep = getPreviewChunkStep(mode);
		let currentPosition = streamedContent.length;

		intervalId = setInterval(() => {
			if (currentPosition < CHAT_TRANSITION_PREVIEW_CONTENT.length) {
				const nextChunkSize = Math.min(
					chunkStep,
					CHAT_TRANSITION_PREVIEW_CONTENT.length - currentPosition
				);
				streamedContent = CHAT_TRANSITION_PREVIEW_CONTENT.slice(
					0,
					Math.max(0, currentPosition + nextChunkSize)
				);
				currentPosition += nextChunkSize;
			} else {
				stopStreaming();
				isStreaming = false;
			}
		}, CHAT_TRANSITION_PREVIEW_STREAMING_SPEED);
	};

	const stopStreaming = () => {
		if (intervalId !== null) {
			clearInterval(intervalId);
			intervalId = null;
		}
	};

	onMount(() => {
		startStreaming();
	});

	$: if (mode) {
		startStreaming();
	}

	onDestroy(() => {
		stopStreaming();
	});
</script>

<div class="preview-frame">
	<Markdown
		content={streamedContent}
		streaming={isStreaming}
		transitionMode={mode}
	/>
</div>

<style>
	.preview-frame {
		height: 180px;
		overflow: hidden;
	}
</style>
