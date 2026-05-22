<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import type { ComponentType } from 'svelte';
	import { slide } from 'svelte/transition';
	import { quintOut } from 'svelte/easing';
	import { Box } from 'lucide-svelte';

	import ChevronDown from '$lib/components/icons/ChevronDown.svelte';

	export let title = '';
	export let description = '';
	export let open = true;
	export let badgeColor = 'bg-blue-50 dark:bg-blue-950/30';
	export let iconColor = 'text-blue-500 dark:text-blue-400';
	export let icon: ComponentType = Box;

	const dispatch = createEventDispatcher<{ toggle: { open: boolean } }>();

	const toggle = () => {
		open = !open;
		dispatch('toggle', { open });
	};
</script>

<section class="glass-section p-5 space-y-3 transition-all duration-300">
	<button
		type="button"
		class="group flex w-full items-start justify-between gap-4 rounded-xl text-left transition-colors hover:bg-white/50 dark:hover:bg-gray-900/30"
		on:click={toggle}
	>
		<div class="flex min-w-0 items-start gap-3">
			<div class="glass-icon-badge shrink-0 {badgeColor}">
				<svelte:component this={icon} class="shrink-0 {iconColor}" size={18} strokeWidth={1.75} />
			</div>

			<div class="min-w-0">
				<div class="text-sm font-semibold text-gray-800 dark:text-gray-100">{title}</div>
				{#if description}
					<p class="mt-1 max-w-3xl text-xs leading-5 text-gray-500 dark:text-gray-400">
						{description}
					</p>
				{/if}
			</div>
		</div>

		<div
			class="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-xl text-gray-400 transition-all group-hover:bg-gray-100 group-hover:text-gray-700 dark:text-gray-500 dark:group-hover:bg-gray-850 dark:group-hover:text-gray-200 {open
				? 'rotate-180'
				: ''}"
		>
			<ChevronDown className="size-4" />
		</div>
	</button>

	{#if open}
		<div transition:slide={{ duration: 200, easing: quintOut }}>
			<slot />
		</div>
	{/if}
</section>
