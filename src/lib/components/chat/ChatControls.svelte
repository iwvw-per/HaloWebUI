<script lang="ts">
	import { SvelteFlowProvider } from '@xyflow/svelte';
	import { Pane, PaneResizer } from 'paneforge';

	import { onDestroy, onMount, tick } from 'svelte';
	import {
		artifactAutoOpenDismissedMessageId,
		artifactPreviewTarget,
		showControls,
		showCallOverlay,
		showOverview,
		showArtifacts
	} from '$lib/stores';

	import Controls from './Controls/Controls.svelte';
	import CallOverlay from './MessageInput/CallOverlay.svelte';
	import Drawer from '../common/Drawer.svelte';
	import Overview from './Overview.svelte';
	import EllipsisVertical from '../icons/EllipsisVertical.svelte';
	import Artifacts from './Artifacts.svelte';

	export let history;
	export let models: any[] = [];

	export let chatId = null;

	export let chatFiles: any[] = [];
	export let params: Record<string, any> = {};

	export let eventTarget: EventTarget;
	export let submitPrompt: Function;
	export let stopResponse: Function;
	export let showMessage: Function;
	export let files: any[] = [];
	export let modelId: string | string[] | null = null;
	export let imageGenerationEnabled = false;
	export let currentValvesContext = null;

	export let pane: any;

	type ViewportMode = 'mobile' | 'tablet' | 'desktop';
	type DesktopPaneMode = 'standard' | 'immersive';

	const DESKTOP_BREAKPOINT = 1024;
	const TABLET_BREAKPOINT = 768;
	const DESKTOP_MIN_WIDTH = 340;
	const DESKTOP_STANDARD_DEFAULT_WIDTH = 400;
	const DESKTOP_STANDARD_MIN_MAIN_WIDTH = 680;
	const DESKTOP_STANDARD_MAX_WIDTH = 720;
	const DESKTOP_STANDARD_MAX_RATIO = 0.5;
	const DESKTOP_IMMERSIVE_DEFAULT_WIDTH = 760;
	const DESKTOP_IMMERSIVE_MIN_MAIN_WIDTH = 40;
	const DESKTOP_SIZE_KEY = 'chatControlsDesktopSize';
	const LEGACY_SIZE_KEY = 'chatControlsSize';

	let viewportMode: ViewportMode = 'mobile';
	let desktopPaneMode: DesktopPaneMode = 'standard';
	let dragged = false;
	let container: HTMLElement | null = null;
	let resizeObserver: ResizeObserver | null = null;
	let desktopMinSize = 0;
	let desktopDefaultSize = 0;
	let desktopMaxSize = 0;
	let desktopPaneRenderKey = 0;
	let ignoreNextPaneCollapse = false;
	let immersivePaneSize: number | null = null;
	let drawerPlacement: 'bottom' | 'right' = 'bottom';
	let drawerClassName = '';

	const clamp = (value: number, min: number, max: number) => Math.min(max, Math.max(min, value));

	const getViewportMode = (): ViewportMode => {
		if (window.innerWidth >= DESKTOP_BREAKPOINT) {
			return 'desktop';
		}

		if (window.innerWidth >= TABLET_BREAKPOINT) {
			return 'tablet';
		}

		return 'mobile';
	};

	const getDesktopPaneMode = (): DesktopPaneMode =>
		$showOverview || $showArtifacts ? 'immersive' : 'standard';

	const getDesktopMaxWidth = (width: number, mode: DesktopPaneMode) => {
		if (mode === 'immersive') {
			// Overview/artifacts are canvas-like surfaces, so only keep a narrow chat strip visible.
			return Math.max(DESKTOP_MIN_WIDTH, Math.floor(width - DESKTOP_IMMERSIVE_MIN_MAIN_WIDTH));
		}

		const maxWidthByRatio = Math.floor(width * DESKTOP_STANDARD_MAX_RATIO);
		const maxWidthByMainPane = Math.floor(width - DESKTOP_STANDARD_MIN_MAIN_WIDTH);

		return Math.max(
			DESKTOP_MIN_WIDTH,
			Math.min(DESKTOP_STANDARD_MAX_WIDTH, maxWidthByRatio, maxWidthByMainPane)
		);
	};

	const getDesktopDefaultWidth = (maxWidth: number, mode: DesktopPaneMode) =>
		clamp(
			mode === 'immersive'
				? DESKTOP_IMMERSIVE_DEFAULT_WIDTH
				: DESKTOP_STANDARD_DEFAULT_WIDTH,
			DESKTOP_MIN_WIDTH,
			maxWidth
		);

	const updateDesktopPaneLimits = (width: number, mode: DesktopPaneMode = desktopPaneMode) => {
		if (!width) {
			return;
		}

		const maxWidth = getDesktopMaxWidth(width, mode);
		const defaultWidth = getDesktopDefaultWidth(maxWidth, mode);

		desktopMinSize = Math.floor((DESKTOP_MIN_WIDTH / width) * 100);
		desktopDefaultSize = Math.round((defaultWidth / width) * 100);
		desktopMaxSize = Math.ceil((maxWidth / width) * 100);
		desktopDefaultSize = clamp(desktopDefaultSize, desktopMinSize, desktopMaxSize);
	};

	const getStoredDesktopPaneSize = () => {
		const rawValue =
			localStorage.getItem(DESKTOP_SIZE_KEY) ?? localStorage.getItem(LEGACY_SIZE_KEY) ?? '';
		const size = Number.parseFloat(rawValue);

		return Number.isFinite(size) && size > 0 ? size : null;
	};

	const persistDesktopPaneSize = (size: number) => {
		const normalized = String(size);
		localStorage.setItem(DESKTOP_SIZE_KEY, normalized);
		localStorage.setItem(LEGACY_SIZE_KEY, normalized);
	};

	const getRememberedDesktopPaneSize = (mode: DesktopPaneMode) =>
		mode === 'immersive' ? immersivePaneSize : getStoredDesktopPaneSize();

	const rememberDesktopPaneSize = (mode: DesktopPaneMode, size: number) => {
		if (mode === 'immersive') {
			immersivePaneSize = size;
			return;
		}

		persistDesktopPaneSize(size);
	};

	const getDesktopPaneTargetSize = (mode: DesktopPaneMode, fallbackSize: number | null = null) => {
		const rememberedSize =
			mode === 'immersive'
				? immersivePaneSize ?? Math.max(fallbackSize ?? 0, desktopDefaultSize)
				: getRememberedDesktopPaneSize(mode) ?? desktopDefaultSize;

		return clamp(rememberedSize, desktopMinSize, desktopMaxSize);
	};

	const applyDesktopPaneMode = async (mode: DesktopPaneMode) => {
		const modeChanged = mode !== desktopPaneMode;
		desktopPaneMode = mode;

		if (container) {
			updateDesktopPaneLimits(container.clientWidth, mode);
		}

		if (mode === 'standard') {
			immersivePaneSize = null;
		}

		if (modeChanged && viewportMode === 'desktop') {
			ignoreNextPaneCollapse = true;
			desktopPaneRenderKey += 1;
			pane = null;
			await tick();
		}
	};

	const syncCallOverlay = async () => {
		if ($showCallOverlay) {
			showCallOverlay.set(false);
			await tick();
			showCallOverlay.set(true);
		}
	};

	const syncViewportMode = async () => {
		const nextMode = getViewportMode();
		const modeChanged = nextMode !== viewportMode;
		viewportMode = nextMode;

		if (container) {
			updateDesktopPaneLimits(container.clientWidth, getDesktopPaneMode());
		}

		if (!modeChanged) {
			return;
		}

		await syncCallOverlay();

		if (viewportMode !== 'desktop') {
			pane = null;
			return;
		}

		if ($showControls) {
			await tick();
			await openPane();
		}
	};

	export const openPane = async (mode: DesktopPaneMode = getDesktopPaneMode()) => {
		if (viewportMode !== 'desktop') {
			return;
		}

		const currentSize = pane?.getSize() ?? null;
		await applyDesktopPaneMode(mode);

		if (!pane) {
			return;
		}

		const targetSize = getDesktopPaneTargetSize(mode, currentSize);

		if (targetSize > 0) {
			pane.resize(targetSize);
			rememberDesktopPaneSize(mode, targetSize);
		}

		ignoreNextPaneCollapse = false;
	};

	const onMouseDown = (_event: MouseEvent) => {
		dragged = true;
	};

	const onMouseUp = (_event: MouseEvent) => {
		dragged = false;
	};

	const clampDesktopPaneSize = (size: number) => {
		if (desktopMinSize === 0 && desktopMaxSize === 0) {
			return size;
		}

		return clamp(size, desktopMinSize, desktopMaxSize);
	};

	const syncDesktopPaneMode = async (mode: DesktopPaneMode) => {
		const currentSize = pane?.getSize() ?? null;
		await applyDesktopPaneMode(mode);

		if (viewportMode !== 'desktop' || !pane || !$showControls) {
			return;
		}

		const nextSize = pane.getSize();
		const targetSize = getDesktopPaneTargetSize(mode, currentSize);

		if (targetSize > 0 && Math.abs(targetSize - nextSize) > 0.1) {
			pane.resize(targetSize);
		}

		rememberDesktopPaneSize(mode, targetSize);
		ignoreNextPaneCollapse = false;
	};

	onMount(async () => {
		container = document.getElementById('chat-container');
		desktopPaneMode = getDesktopPaneMode();

		if (container) {
			updateDesktopPaneLimits(container.clientWidth, desktopPaneMode);

			resizeObserver = new ResizeObserver((entries) => {
				for (const entry of entries) {
					updateDesktopPaneLimits(entry.contentRect.width, desktopPaneMode);

					if ($showControls && pane && pane.isExpanded()) {
						const nextSize = clampDesktopPaneSize(pane.getSize());
						if (Math.abs(nextSize - pane.getSize()) > 0.1) {
							pane.resize(nextSize);
						}
						rememberDesktopPaneSize(desktopPaneMode, nextSize);
					}
				}
			});

			resizeObserver.observe(container);
		}

		window.addEventListener('resize', syncViewportMode);
		await syncViewportMode();
		document.addEventListener('mousedown', onMouseDown);
		document.addEventListener('mouseup', onMouseUp);
	});

	onDestroy(() => {
		showControls.set(false);

		window.removeEventListener('resize', syncViewportMode);
		resizeObserver?.disconnect();
		document.removeEventListener('mousedown', onMouseDown);
		document.removeEventListener('mouseup', onMouseUp);
	});

	const closeHandler = () => {
		if ($showArtifacts && $artifactPreviewTarget?.messageId) {
			artifactAutoOpenDismissedMessageId.set($artifactPreviewTarget.messageId);
		}

		artifactPreviewTarget.set(null);
		showControls.set(false);
		showOverview.set(false);
		showArtifacts.set(false);

		if ($showCallOverlay) {
			showCallOverlay.set(false);
		}
	};

	$: if (!chatId) {
		closeHandler();
	}

	$: {
		const nextDesktopPaneMode = getDesktopPaneMode();
		if (nextDesktopPaneMode !== desktopPaneMode) {
			void syncDesktopPaneMode(nextDesktopPaneMode);
		}
	}

	$: drawerPlacement = viewportMode === 'tablet' ? 'right' : 'bottom';
	$: drawerClassName =
		viewportMode === 'tablet'
			? `h-[100dvh] w-full ${$showCallOverlay || $showOverview || $showArtifacts ? 'max-w-[720px]' : 'max-w-[400px]'} bg-gray-50/95 dark:bg-gray-900/95 dark:text-gray-100 border-l border-gray-200/40 dark:border-gray-700/30 shadow-2xl backdrop-blur-xl`
			: 'h-[100dvh] w-full bg-gray-50 dark:bg-gray-900 dark:text-gray-100';
</script>

<SvelteFlowProvider>
	{#if viewportMode !== 'desktop'}
		{#if $showControls}
			<Drawer
				placement={drawerPlacement}
				className={drawerClassName}
				show={$showControls}
				on:close={closeHandler}
			>
				<div
					class=" {$showCallOverlay || $showOverview || $showArtifacts
						? ' h-screen  w-full'
						: 'px-6 py-4'} h-full"
				>
					{#if $showCallOverlay}
						<div
							class=" h-full max-h-[100dvh] bg-white text-gray-700 dark:bg-black dark:text-gray-300 flex justify-center"
						>
							<CallOverlay
								bind:files
								{submitPrompt}
								{stopResponse}
								{modelId}
								{imageGenerationEnabled}
								{chatId}
								{eventTarget}
								on:close={() => {
									showControls.set(false);
								}}
							/>
						</div>
					{:else if $showArtifacts}
						<Artifacts {history} />
					{:else if $showOverview}
						<Overview
							{history}
							on:nodeclick={(e) => {
								showMessage(e.detail.node.data.message, { source: 'overview' });
							}}
							on:close={() => {
								showControls.set(false);
							}}
						/>
					{:else}
						<Controls
							on:close={() => {
								showControls.set(false);
							}}
							{models}
							{modelId}
							{currentValvesContext}
							bind:chatFiles
							bind:params
							on:chatFilesChange
						/>
					{/if}
				</div>
			</Drawer>
		{/if}
	{:else}
		<!-- if $showControls -->

		{#if $showControls}
			<PaneResizer
				class="relative flex w-4 cursor-col-resize items-center justify-center bg-background
					hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors duration-150 group"
			>
				<div class="z-10 flex h-7 w-5 items-center justify-center rounded-xs">
					<EllipsisVertical className="size-4 invisible group-hover:visible" />
				</div>
			</PaneResizer>
		{/if}

		{#key desktopPaneRenderKey}
			<Pane
				bind:pane
				defaultSize={0}
				minSize={desktopMinSize}
				maxSize={desktopMaxSize}
				onResize={(size) => {
					if ($showControls && pane.isExpanded()) {
						const nextSize = clampDesktopPaneSize(size);
						if (Math.abs(nextSize - size) > 0.1) {
							pane.resize(nextSize);
						}
						rememberDesktopPaneSize(desktopPaneMode, nextSize);
					}
				}}
				onCollapse={() => {
					if (ignoreNextPaneCollapse) {
						ignoreNextPaneCollapse = false;
						return;
					}

					closeHandler();
				}}
				collapsible={true}
				class=" z-10 "
			>
				{#if $showControls}
					<div class="flex max-h-full min-h-full">
						<div
							class="w-full {($showOverview || $showArtifacts) && !$showCallOverlay
								? ' '
								: 'px-5 py-5 bg-white/90 dark:bg-gray-900/80 backdrop-blur-xl border border-gray-200/40 dark:border-gray-700/30 shadow-sm'} z-40 pointer-events-auto overflow-y-auto scrollbar-hidden"
						>
							{#if $showCallOverlay}
								<div class="w-full h-full flex justify-center">
									<CallOverlay
										bind:files
										{submitPrompt}
										{stopResponse}
										{modelId}
										{imageGenerationEnabled}
										{chatId}
										{eventTarget}
										on:close={() => {
											showControls.set(false);
										}}
									/>
								</div>
							{:else if $showArtifacts}
								<Artifacts {history} overlay={dragged} />
							{:else if $showOverview}
								<Overview
									{history}
									on:nodeclick={(e) => {
										if (e.detail.node.data.message.favorite) {
											history.messages[e.detail.node.data.message.id].favorite = true;
										} else {
											history.messages[e.detail.node.data.message.id].favorite = null;
										}

										showMessage(e.detail.node.data.message, { source: 'overview' });
									}}
									on:close={() => {
										showControls.set(false);
									}}
								/>
							{:else}
								<Controls
									on:close={() => {
										showControls.set(false);
									}}
									{models}
									{modelId}
									{currentValvesContext}
									bind:chatFiles
									bind:params
									on:chatFilesChange
								/>
							{/if}
						</div>
					</div>
				{/if}
			</Pane>
		{/key}
	{/if}
</SvelteFlowProvider>
