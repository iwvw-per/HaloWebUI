<script lang="ts">
	import { DARK_MODE_INVERT_ICONS } from '$lib/utils/model-icons.manifest';
	import { DARK_MODE_INVERT_CONN_AVATARS } from '$lib/utils/connection-avatars.manifest';

	export let src: string | null | undefined;
	export let alt = '';
	export let title: string | undefined = undefined;
	export let loading: 'eager' | 'lazy' | undefined = undefined;
	export let decoding: 'async' | 'sync' | 'auto' = 'async';
	export let bare = false;

	// Applied to the outer wrapper. This should include size classes (e.g. `size-5`) and rounding.
	export let className = '';
	// Optional override for the <img> `object-fit` class.
	// If omitted, we auto-pick `object-cover` for raster icons and `object-contain` for SVGs.
	export let imgClassName: string | undefined = undefined;

	let loaded = false;
	let lastSrc: string | null | undefined = undefined;

	const MODEL_ICONS_SEGMENT = '/static/model-icons/';
	const CONN_AVATARS_SEGMENT = '/static/connection-avatars/';

	function getModelIconFilename(url: string): string | null {
		const bare = url.split('#')[0].split('?')[0];
		const idx = bare.lastIndexOf(MODEL_ICONS_SEGMENT);
		if (idx === -1) return null;
		const tail = bare.slice(idx + MODEL_ICONS_SEGMENT.length);
		if (!tail) return null;
		const filename = tail.split('/').at(-1);
		return filename ?? null;
	}

	function getConnAvatarFilename(url: string): string | null {
		const bare = url.split('#')[0].split('?')[0];
		const idx = bare.lastIndexOf(CONN_AVATARS_SEGMENT);
		if (idx === -1) return null;
		const tail = bare.slice(idx + CONN_AVATARS_SEGMENT.length);
		if (!tail) return null;
		const filename = tail.split('/').at(-1);
		return filename ?? null;
	}

	function getExtension(url: string): string {
		const bare = url.split('#')[0].split('?')[0];
		const last = bare.split('/').at(-1) ?? '';
		const dot = last.lastIndexOf('.');
		return dot === -1 ? '' : last.slice(dot + 1).toLowerCase();
	}

	$: filename = src ? getModelIconFilename(src) : null;
	$: connFilename = src ? getConnAvatarFilename(src) : null;
	$: isMonochromeModelIcon = !!filename && DARK_MODE_INVERT_ICONS.has(filename);
	$: isMonochromeConnAvatar = !!connFilename && DARK_MODE_INVERT_CONN_AVATARS.has(connFilename);
	$: isDefaultFavicon = !!src && src.split('#')[0].split('?')[0].endsWith('/static/favicon.png');
	$: shouldInvertInDark = isMonochromeModelIcon || isMonochromeConnAvatar || isDefaultFavicon;
	$: activeFilename = filename ?? connFilename;
	$: ext = activeFilename ? getExtension(activeFilename) : src ? getExtension(src) : '';
	$: isRaster = ext !== '' && ext !== 'svg';

	const MONO_SCALE_OVERRIDES: Record<string, number> = {
		// These SVGs already fill the viewBox very tightly; extra scaling causes clipping inside rounded boxes.
		'ai21.svg': 1.0,
		'ibm.svg': 1.0,
		'inception.svg': 1.0,
		'noushermes.svg': 1.0,
		'grok.svg': 1.0,
		'flux.svg': 1.0,
		'xiaomimimo.svg': 1.0,
		'openai.svg': 1.0,
		'openrouter.svg': 1.0,
		'baai.svg': 1.0
	};

	// Make icons feel "more filled" inside their rounded box without per-asset tweaking.
	// - raster: often has built-in padding/soft edges
	// - monochrome SVG: tends to have generous viewBox whitespace
	$: scale = isRaster
		? 1.12
		: isMonochromeModelIcon
			? (MONO_SCALE_OVERRIDES[activeFilename ?? ''] ?? 1.16)
			: 1.0;

	$: fitClass = imgClassName ?? (isRaster ? 'object-cover' : 'object-contain');
	$: if (src !== lastSrc) {
		lastSrc = src;
		loaded = false;
	}
</script>

{#if src}
	<span
		class="model-icon {shouldInvertInDark ? 'model-icon--invert' : ''} {isMonochromeModelIcon ||
		isMonochromeConnAvatar
			? 'model-icon--mono'
			: ''} {bare ? 'model-icon--bare' : ''} {className}"
	>
		<img
			class="model-icon__img model-icon__img--{loaded ? 'loaded' : 'loading'} {fitClass}"
			style={`--mi-scale: ${scale};`}
			{src}
			{alt}
			{title}
			{loading}
			{decoding}
			draggable="false"
			on:load={() => {
				loaded = true;
			}}
			on:error={() => {
				loaded = true;
			}}
		/>
	</span>
{/if}

<style>
	.model-icon {
		position: relative;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex: none;
		overflow: hidden;
		background-color: #f3f4f6;
		box-shadow:
			0 1px 3px rgba(0, 0, 0, 0.1),
			0 1px 2px rgba(0, 0, 0, 0.06);
	}

	.model-icon__img {
		position: relative;
		display: block;
		width: 100%;
		height: 100%;
		opacity: 0;
		transform: scale(var(--mi-scale, 1));
		transform-origin: center;
		transition: opacity 140ms ease-out;
	}

	.model-icon__img--loaded {
		opacity: 1;
	}

	.model-icon--bare {
		background-color: transparent;
		box-shadow:
			var(--tw-ring-offset-shadow, 0 0 #0000),
			var(--tw-ring-shadow, 0 0 #0000),
			var(--tw-shadow, 0 0 #0000);
	}

	:global(html.dark) .model-icon--invert .model-icon__img {
		filter: invert(1) brightness(1.12) contrast(1.06);
	}

	.model-icon::after {
		content: '';
		position: absolute;
		inset: 0;
		border-radius: inherit;
		pointer-events: none;
		box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.06);
	}

	.model-icon--bare::after {
		content: none;
	}

	:global(html.dark) .model-icon {
		background-color: #1f2937;
		box-shadow:
			0 2px 8px rgba(0, 0, 0, 0.3),
			0 0 0 1px rgba(255, 255, 255, 0.06);
	}

	:global(html.dark) .model-icon--bare {
		background-color: transparent;
		box-shadow:
			var(--tw-ring-offset-shadow, 0 0 #0000),
			var(--tw-ring-shadow, 0 0 #0000),
			var(--tw-shadow, 0 0 #0000);
	}

	:global(html.dark) .model-icon::after {
		box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.12);
	}
</style>
