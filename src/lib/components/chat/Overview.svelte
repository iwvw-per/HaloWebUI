<script lang="ts">
	import { getContext, createEventDispatcher, onDestroy } from 'svelte';
	import { useSvelteFlow, useNodesInitialized } from '@xyflow/svelte';

	const dispatch = createEventDispatcher();
	const i18n = getContext('i18n');

	import { onMount, tick } from 'svelte';

	import { writable } from 'svelte/store';
	import { models, overviewFocusedMessageId, showOverview, user } from '$lib/stores';

	import '@xyflow/svelte/dist/style.css';

	import CustomNode from './Overview/Node.svelte';
	import Flow from './Overview/Flow.svelte';
	import XMark from '../icons/XMark.svelte';
	import ArrowLeft from '../icons/ArrowLeft.svelte';
	import { findModelByIdentity } from '$lib/utils/model-identity';

	const { fitView, flowToScreenPosition, getNodesBounds, getViewport, setViewport } = useSvelteFlow();
	const nodesInitialized = useNodesInitialized();

	export let history;

	let highlightedMessageId = null;
	let overviewElement: HTMLDivElement;
	let headerElement: HTMLDivElement;

	const nodes = writable([]);
	const edges = writable([]);

	const nodeTypes = {
		custom: CustomNode
	};

	$: highlightedMessageId =
		history?.messages?.[$overviewFocusedMessageId]
			? $overviewFocusedMessageId
			: history?.currentId ?? null;

	$: if (history) {
		highlightedMessageId;
		drawFlow();
	}

	const drawFlow = async () => {
		const nodeList = [];
		const edgeList = [];
		const levelOffset = 150; // Vertical spacing between layers
		const siblingOffset = 250; // Horizontal spacing between nodes at the same layer

		// Map to keep track of node positions at each level
		let positionMap = new Map();

		// Helper function to truncate labels
		function createLabel(content) {
			const maxLength = 100;
			return content.length > maxLength ? content.substr(0, maxLength) + '...' : content;
		}

		// Create nodes and map children to ensure alignment in width
		let layerWidths = {}; // Track widths of each layer

		Object.keys(history.messages).forEach((id) => {
			const message = history.messages[id];
			const level = message.parentId ? (positionMap.get(message.parentId)?.level ?? -1) + 1 : 0;
			if (!layerWidths[level]) layerWidths[level] = 0;

			positionMap.set(id, {
				id: message.id,
				level,
				position: layerWidths[level]++
			});
		});

		// Adjust positions based on siblings count to centralize vertical spacing
		Object.keys(history.messages).forEach((id) => {
			const pos = positionMap.get(id);
			const xOffset = pos.position * siblingOffset;
			const y = pos.level * levelOffset;
			const x = xOffset;

			nodeList.push({
				id: pos.id,
				type: 'custom',
				selected: highlightedMessageId === pos.id,
				data: {
					user: $user,
					message: history.messages[id],
					model: findModelByIdentity($models, history.messages[id].model),
					isCurrent: highlightedMessageId === pos.id,
					isOnCurrentPath:
						highlightedMessageId === id || recurseCheckChild(id, highlightedMessageId)
				},
				position: { x, y }
			});

			// Create edges
			const parentId = history.messages[id].parentId;
			if (parentId) {
				edgeList.push({
					id: parentId + '-' + pos.id,
					source: parentId,
					target: pos.id,
					selectable: false,
					class:
						highlightedMessageId === id || recurseCheckChild(id, highlightedMessageId)
							? 'overview-edge-active'
							: 'overview-edge',
					type: 'smoothstep',
					animated: highlightedMessageId === id || recurseCheckChild(id, highlightedMessageId)
				});
			}
		});

		await edges.set([...edgeList]);
		await nodes.set([...nodeList]);
	};

	const recurseCheckChild = (nodeId, currentId) => {
		const node = history.messages[nodeId];
		return (
			node.childrenIds &&
			node.childrenIds.some((id) => id === currentId || recurseCheckChild(id, currentId))
		);
	};

	const nudgeNodeIntoView = async (messageId: string) => {
		if (!messageId || !overviewElement) {
			return;
		}

		let bounds;
		try {
			bounds = getNodesBounds([messageId]);
		} catch (error) {
			return;
		}

		if (!bounds || !Number.isFinite(bounds.width) || !Number.isFinite(bounds.height)) {
			return;
		}

		const topLeft = flowToScreenPosition({ x: bounds.x, y: bounds.y });
		const bottomRight = flowToScreenPosition({
			x: bounds.x + bounds.width,
			y: bounds.y + bounds.height
		});

		const panelRect = overviewElement.getBoundingClientRect();
		const headerRect = headerElement?.getBoundingClientRect();
		const safeInsetX = 24;
		const safeInsetBottom = 28;
		const safeInsetTop = 18;

		const safeLeft = panelRect.left + safeInsetX;
		const safeRight = panelRect.right - safeInsetX;
		const safeTop = Math.max(
			panelRect.top + safeInsetTop,
			(headerRect?.bottom ?? panelRect.top) + safeInsetTop
		);
		const safeBottom = panelRect.bottom - safeInsetBottom;

		let deltaX = 0;
		let deltaY = 0;

		if (topLeft.x < safeLeft) {
			deltaX = safeLeft - topLeft.x;
		} else if (bottomRight.x > safeRight) {
			deltaX = safeRight - bottomRight.x;
		}

		if (topLeft.y < safeTop) {
			deltaY = safeTop - topLeft.y;
		} else if (bottomRight.y > safeBottom) {
			deltaY = safeBottom - bottomRight.y;
		}

		if (deltaX === 0 && deltaY === 0) {
			return;
		}

		const viewport = getViewport();
		await setViewport(
			{
				...viewport,
				x: viewport.x + deltaX,
				y: viewport.y + deltaY
			},
			{ duration: 180 }
		);
	};

	onMount(() => {
		drawFlow();

		nodesInitialized.subscribe(async (initialized) => {
			if (initialized) {
				await tick();
				const focusId = highlightedMessageId ?? history?.currentId ?? null;
				if (focusId) {
					await fitView({ nodes: [{ id: focusId }] });
				}
			}
		});
	});

	onDestroy(() => {
		console.log('Overview destroyed');

		nodes.set([]);
		edges.set([]);
	});
</script>

<div bind:this={overviewElement} class="w-full h-full relative">
	<div
		bind:this={headerElement}
		class=" absolute z-50 w-full flex justify-between dark:text-gray-100 px-4 py-3.5"
	>
		<div class="flex items-center gap-2.5">
			<button
				class="self-center p-0.5"
				on:click={() => {
					showOverview.set(false);
				}}
			>
				<ArrowLeft className="size-3.5" />
			</button>
			<div class=" text-lg font-medium self-center font-primary">{$i18n.t('Chat Overview')}</div>
		</div>
		<button
			class="self-center p-0.5"
			on:click={() => {
				dispatch('close');
				showOverview.set(false);
			}}
		>
			<XMark className="size-3.5" />
		</button>
	</div>

	{#if $nodes.length > 0}
		<Flow
			{nodes}
			{nodeTypes}
			{edges}
			on:nodeclick={(e) => {
				dispatch('nodeclick', e.detail);
				const messageId = e.detail.node.data.message.id;
				overviewFocusedMessageId.set(messageId);
				void tick().then(() => nudgeNodeIntoView(messageId));
			}}
		/>
	{/if}
</div>

<style>
	:global(.overview-edge path) {
		stroke: rgba(148, 163, 184, 0.55);
		stroke-width: 1.5px;
	}

	:global(.dark .overview-edge path) {
		stroke: rgba(148, 163, 184, 0.3);
	}

	:global(.overview-edge-active path) {
		stroke: rgba(14, 165, 233, 0.72);
		stroke-width: 2.5px;
	}

	:global(.dark .overview-edge-active path) {
		stroke: rgba(56, 189, 248, 0.8);
	}
</style>
