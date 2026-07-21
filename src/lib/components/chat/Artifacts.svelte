<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { onMount, onDestroy, getContext, createEventDispatcher, tick } from 'svelte';
	const i18n = getContext('i18n');
	const dispatch = createEventDispatcher();

	import {
		artifactAutoOpenDismissedMessageId,
		artifactPreviewTarget,
		chatId,
		settings,
		showArtifacts,
		showControls
	} from '$lib/stores';
	import XMark from '../icons/XMark.svelte';
	import { copyToClipboard, createMessagesList } from '$lib/utils';
	import ArrowsPointingOut from '../icons/ArrowsPointingOut.svelte';
	import Tooltip from '../common/Tooltip.svelte';
	import SvgPanZoom from '../common/SVGPanZoom.svelte';
	import ArrowLeft from '../icons/ArrowLeft.svelte';
	import { extractSvgMarkupBlocks, normalizeSvgMarkup } from './Messages/Markdown/svgMarkupTokens';

	export let overlay = false;
	export let history;
	let messages = [];

	let contents: Array<{ type: string; content: string; messageId: string }> = [];
	let selectedContentIdx = 0;

	let copied = false;
	let iframeElement: HTMLIFrameElement;
	let alive = true;

	/** Strip thinking/reasoning blocks from content before code extraction */
	const stripThinkingBlocks = (text: string): string => {
		return text.replace(/<(think|thinking|reasoning)>[\s\S]*?<\/(think|thinking|reasoning)>/g, '');
	};

	$: if (history) {
		messages = createMessagesList(history, history.currentId);
		getContents();
	} else {
		messages = [];
		getContents();
	}

	const getContents = () => {
		contents = [];
		messages.forEach((message) => {
			if (message?.role !== 'user' && message?.content) {
				// Strip thinking/reasoning blocks before extracting code
				const cleanedContent = stripThinkingBlocks(message.content);

				const codeBlockContents = cleanedContent.match(/```[\s\S]*?```/g);
				let codeBlocks = [];

				if (codeBlockContents) {
					codeBlockContents.forEach((block) => {
						const lang = block.split('\n')[0].replace('```', '').trim().toLowerCase();
						const code = block.replace(/```[\s\S]*?\n/, '').replace(/```$/, '');
						codeBlocks.push({ lang, code });
					});
				}

				let htmlContent = '';
				let cssContent = '';
				let jsContent = '';

				codeBlocks.forEach((block) => {
					const { lang, code } = block;

					if (lang === 'html') {
						htmlContent += code + '\n';
					} else if (lang === 'css') {
						cssContent += code + '\n';
					} else if (lang === 'javascript' || lang === 'js') {
						jsContent += code + '\n';
					}
				});

				const inlineHtml = cleanedContent.match(/<html>[\s\S]*?<\/html>/gi);
				const inlineCss = cleanedContent.match(/<style>[\s\S]*?<\/style>/gi);
				const inlineJs = cleanedContent.match(/<script>[\s\S]*?<\/script>/gi);

				if (inlineHtml) {
					inlineHtml.forEach((block) => {
						const content = block.replace(/<\/?html>/gi, ''); // Remove <html> tags
						htmlContent += content + '\n';
					});
				}
				if (inlineCss) {
					inlineCss.forEach((block) => {
						const content = block.replace(/<\/?style>/gi, ''); // Remove <style> tags
						cssContent += content + '\n';
					});
				}
				if (inlineJs) {
					inlineJs.forEach((block) => {
						const content = block.replace(/<\/?script>/gi, ''); // Remove <script> tags
						jsContent += content + '\n';
					});
				}

				if (htmlContent || cssContent || jsContent) {
					const renderedContent = `
                        <!DOCTYPE html>
                        <html lang="zh-CN">
                        <head>
                            <meta charset="UTF-8">
                            <meta name="viewport" content="width=device-width, initial-scale=1.0">
							<${''}style>
								body {
									background-color: white; /* Ensure the iframe has a white background */
								}

								${cssContent}
							</${''}style>
                        </head>
                        <body>
                            ${htmlContent}

							<${''}script>
                            	${jsContent}
							</${''}script>
                        </body>
                        </html>
                    `;
					contents = [...contents, { type: 'iframe', content: renderedContent, messageId: message.id }];
				}

				for (const block of codeBlocks) {
					if (block.lang === 'svg' || (block.lang === 'xml' && block.code.includes('<svg'))) {
						contents = [
							...contents,
							{ type: 'svg', content: normalizeSvgMarkup(block.code), messageId: message.id }
						];
					}
				}

				for (const markup of extractSvgMarkupBlocks(cleanedContent)) {
					contents = [
						...contents,
						{ type: 'svg', content: normalizeSvgMarkup(markup), messageId: message.id }
					];
				}
			}
		});

		if (contents.length === 0) {
			// Defer store mutations out of the reactive block to avoid re-trigger loops
			tick().then(() => {
				if (alive) {
					showControls.set(false);
					showArtifacts.set(false);
				}
			});
		}

		const target = $artifactPreviewTarget;
		if (target?.type === 'svg' && target.content) {
			const normalizedContent = normalizeSvgMarkup(target.content);
			const existingIdx = contents.findIndex(
				(content) => content.type === 'svg' && content.content === normalizedContent
			);

			if (existingIdx >= 0) {
				selectedContentIdx = existingIdx;
				return;
			}

			contents = [
				...contents,
				{
					type: 'svg',
					content: normalizedContent,
					messageId: target.messageId ?? '__svg-preview__'
				}
			];
			selectedContentIdx = contents.length - 1;
			return;
		}

		if (target?.messageId) {
			for (let idx = contents.length - 1; idx >= 0; idx -= 1) {
				const content = contents[idx];
				if (
					content.messageId === target.messageId &&
					(!target.type || content.type === target.type)
				) {
					selectedContentIdx = idx;
					return;
				}
			}
		}

		selectedContentIdx = contents.length > 0 ? contents.length - 1 : 0;
	};

	function navigateContent(direction: 'prev' | 'next') {
		console.log(selectedContentIdx);

		selectedContentIdx =
			direction === 'prev'
				? Math.max(selectedContentIdx - 1, 0)
				: Math.min(selectedContentIdx + 1, contents.length - 1);

		console.log(selectedContentIdx);
	}

	const iframeLoadHandler = () => {
		iframeElement.contentWindow.addEventListener(
			'click',
			function (e) {
				const target = e.target.closest('a');
				if (target && target.href) {
					e.preventDefault();
					const url = new URL(target.href, iframeElement.baseURI);
					if (url.origin === window.location.origin) {
						iframeElement.contentWindow.history.pushState(
							null,
							'',
							url.pathname + url.search + url.hash
						);
					} else {
						console.log('External navigation blocked:', url.href);
					}
				}
			},
			true
		);

		// Cancel drag when hovering over iframe
		iframeElement.contentWindow.addEventListener('mouseenter', function (e) {
			e.preventDefault();
			iframeElement.contentWindow.addEventListener('dragstart', (event) => {
				event.preventDefault();
			});
		});
	};

	const showFullScreen = () => {
		if (iframeElement.requestFullscreen) {
			iframeElement.requestFullscreen();
		} else if (iframeElement.webkitRequestFullscreen) {
			iframeElement.webkitRequestFullscreen();
		} else if (iframeElement.msRequestFullscreen) {
			iframeElement.msRequestFullscreen();
		}
	};

	onMount(() => {});

	onDestroy(() => {
		alive = false;
	});

	const getActivePreviewMessageId = () =>
		$artifactPreviewTarget?.messageId ?? contents[selectedContentIdx]?.messageId ?? null;

	const dismissArtifacts = ({ closeControls = false } = {}) => {
		const messageId = getActivePreviewMessageId();

		if (messageId) {
			artifactAutoOpenDismissedMessageId.set(messageId);
		}

		artifactPreviewTarget.set(null);
		showArtifacts.set(false);

		if (closeControls) {
			dispatch('close');
			showControls.set(false);
		}
	};

	const closeArtifacts = () => {
		dismissArtifacts({ closeControls: true });
	};
</script>

<div class=" w-full h-full relative flex flex-col bg-gray-50 dark:bg-gray-850">
	<div class="w-full h-full flex flex-col flex-1 relative">
		{#if contents.length > 0}
			<div
				class="pointer-events-auto z-20 flex justify-between items-center p-2.5 font-primar text-gray-900 dark:text-white"
			>
				<button
					class="self-center pointer-events-auto p-1 rounded-full bg-white dark:bg-gray-850"
					on:click={() => dismissArtifacts()}
				>
					<ArrowLeft className="size-3.5  text-gray-900 dark:text-white" />
				</button>

				<div class="flex-1 flex items-center justify-between">
					<div class="flex items-center space-x-2">
						<div class="flex items-center gap-0.5 self-center min-w-fit" dir="ltr">
							<button
								class="self-center p-1 hover:bg-black/5 dark:hover:bg-white/5 dark:hover:text-white hover:text-black rounded-md transition disabled:cursor-not-allowed"
								on:click={() => navigateContent('prev')}
								disabled={contents.length <= 1}
							>
								<svg
									xmlns="http://www.w3.org/2000/svg"
									fill="none"
									viewBox="0 0 24 24"
									stroke="currentColor"
									stroke-width="2.5"
									class="size-3.5"
								>
									<path
										stroke-linecap="round"
										stroke-linejoin="round"
										d="M15.75 19.5 8.25 12l7.5-7.5"
									/>
								</svg>
							</button>

							<div class="text-xs self-center dark:text-gray-100 min-w-fit">
								{$i18n.t('Version {{selectedVersion}} of {{totalVersions}}', {
									selectedVersion: selectedContentIdx + 1,
									totalVersions: contents.length
								})}
							</div>

							<button
								class="self-center p-1 hover:bg-black/5 dark:hover:bg-white/5 dark:hover:text-white hover:text-black rounded-md transition disabled:cursor-not-allowed"
								on:click={() => navigateContent('next')}
								disabled={contents.length <= 1}
							>
								<svg
									xmlns="http://www.w3.org/2000/svg"
									fill="none"
									viewBox="0 0 24 24"
									stroke="currentColor"
									stroke-width="2.5"
									class="size-3.5"
								>
									<path
										stroke-linecap="round"
										stroke-linejoin="round"
										d="m8.25 4.5 7.5 7.5-7.5 7.5"
									/>
								</svg>
							</button>
						</div>
					</div>

					<div class="flex items-center gap-1">
						<button
							class="copy-code-button bg-none border-none text-xs bg-gray-50 hover:bg-gray-100 dark:bg-gray-850 dark:hover:bg-gray-800 transition rounded-md px-1.5 py-0.5"
							on:click={() => {
								copyToClipboard(contents[selectedContentIdx].content);
								copied = true;

								setTimeout(() => {
									copied = false;
								}, 2000);
							}}>{copied ? $i18n.t('Copied') : $i18n.t('Copy')}</button
						>

						{#if contents[selectedContentIdx].type === 'iframe'}
							<Tooltip content={$i18n.t('Open in full screen')}>
								<button
									class=" bg-none border-none text-xs bg-gray-50 hover:bg-gray-100 dark:bg-gray-850 dark:hover:bg-gray-800 transition rounded-md p-0.5"
									on:click={showFullScreen}
								>
									<ArrowsPointingOut className="size-3.5" />
								</button>
							</Tooltip>
						{/if}
					</div>
				</div>

				<button
					class="self-center pointer-events-auto p-1 rounded-full bg-white dark:bg-gray-850"
					on:click={closeArtifacts}
				>
					<XMark className="size-3.5 text-gray-900 dark:text-white" />
				</button>
			</div>
		{/if}

		{#if overlay}
			<div class=" absolute top-0 left-0 right-0 bottom-0 z-10"></div>
		{/if}

		<div class="flex-1 w-full h-full">
			<div class=" h-full flex flex-col">
				{#if contents.length > 0}
					<div class="max-w-full w-full h-full">
						{#if contents[selectedContentIdx].type === 'iframe'}
							<iframe
								bind:this={iframeElement}
								title="Content"
								srcdoc={contents[selectedContentIdx].content}
								class="w-full border-0 h-full rounded-none"
								sandbox="allow-scripts{($settings?.iframeSandboxAllowForms ?? false)
									? ' allow-forms'
									: ''}{($settings?.iframeSandboxAllowSameOrigin ?? false)
									? ' allow-same-origin'
									: ''}"
								on:load={iframeLoadHandler}
							></iframe>
						{:else if contents[selectedContentIdx].type === 'svg'}
							<SvgPanZoom
								className=" w-full h-full max-h-full overflow-hidden"
								svg={contents[selectedContentIdx].content}
							/>
						{/if}
					</div>
				{:else}
					<div class="m-auto font-medium text-xs text-gray-900 dark:text-white">
						{$i18n.t('No previewable HTML or SVG content found.')}
					</div>
				{/if}
			</div>
		</div>
	</div>
</div>
