<script lang="ts">
	import DOMPurify from 'dompurify';
	import { toast } from 'svelte-sonner';

	import type { Token } from 'marked';
	import { getContext } from 'svelte';

	const i18n = getContext('i18n');

	import { WEBUI_BASE_URL } from '$lib/constants';
	import { copyToClipboard, unescapeHtml } from '$lib/utils';

	import Image from '$lib/components/common/Image.svelte';
	import KatexRenderer from './KatexRenderer.svelte';
	import Source from './Source.svelte';
	import SourceToken from './SourceToken.svelte';
	import {
		isSvgMarkup,
		mergeSvgMarkupTokens,
		type RenderableHtmlToken
	} from './svgMarkupTokens';

	export let id: string;
	export let tokens: Token[] = [];
	export let onSourceClick: Function = () => {};
	export let charAnimation = false;

	let renderTokens: RenderableHtmlToken[] = [];
	$: renderTokens = mergeSvgMarkupTokens(tokens);
</script>

{#each renderTokens as token}
	{#if token.type === 'escape'}
		{#if charAnimation}
			{#each [...unescapeHtml(token.text)] as char}<span class="stream-char">{char}</span>{/each}
		{:else}
			{unescapeHtml(token.text)}
		{/if}
	{:else if token.type === 'html'}
		{@const isSvgMarkupToken = isSvgMarkup(token.text)}
		{@const html = DOMPurify.sanitize(token.text, { ADD_ATTR: ['style'] })}
		{#if isSvgMarkupToken}
			<span class="font-mono whitespace-pre-wrap break-all">{token.text}</span>
		{:else if html && html.includes('<video')}
			{@html html}
		{:else if token.text.includes(`<iframe src="${WEBUI_BASE_URL}/api/v1/files/`)}
			{@html `${token.text}`}
		{:else if token.text.includes(`<source_id`)}
			<Source {id} {token} onClick={onSourceClick} />
		{:else}
			{@html html}
		{/if}
	{:else if token.type === 'link'}
		{#if token.tokens}
			<a href={token.href} target="_blank" rel="nofollow" title={token.title}>
				<svelte:self
					id={`${id}-a`}
					tokens={token.tokens}
					{charAnimation}
					{onSourceClick}
				/>
			</a>
		{:else}
			<a href={token.href} target="_blank" rel="nofollow" title={token.title}>{token.text}</a>
		{/if}
	{:else if token.type === 'image'}
		<Image src={token.href} alt={token.text} />
	{:else if token.type === 'strong'}
		<strong>
			<svelte:self
				id={`${id}-strong`}
				tokens={token.tokens}
				{charAnimation}
				{onSourceClick}
			/>
		</strong>
	{:else if token.type === 'em'}
		<em>
			<svelte:self
				id={`${id}-em`}
				tokens={token.tokens}
				{charAnimation}
				{onSourceClick}
			/>
		</em>
	{:else if token.type === 'codespan'}
		<!-- svelte-ignore a11y-click-events-have-key-events -->
		<!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
		<code
			class="codespan cursor-pointer"
			on:click={() => {
				copyToClipboard(unescapeHtml(token.text));
				toast.success($i18n.t('Copied to clipboard'));
			}}>{unescapeHtml(token.text)}</code
		>
	{:else if token.type === 'br'}
		<br />
	{:else if token.type === 'del'}
		<del>
			<svelte:self
				id={`${id}-del`}
				tokens={token.tokens}
				{charAnimation}
				{onSourceClick}
			/>
		</del>
	{:else if token.type === 'inlineKatex'}
		{#if token.text}
			<KatexRenderer content={token.text} displayMode={false} />
		{/if}
	{:else if token.type === 'iframe'}
		<iframe
			src="{WEBUI_BASE_URL}/api/v1/files/{token.fileId}/content"
			title={token.fileId}
			width="100%"
			frameborder="0"
			sandbox="allow-scripts"
			onload="this.style.height=(this.contentWindow.document.body.scrollHeight+20)+'px';"
		></iframe>
	{:else if token.type === 'citation'}
		<SourceToken {id} {token} onClick={onSourceClick} />
	{:else if token.type === 'text'}
		{#if charAnimation}
			{#each [...(token.raw ?? token.text)] as char}<span class="stream-char">{char}</span>{/each}
		{:else}
			{token.raw ?? token.text}
		{/if}
	{/if}
{/each}
