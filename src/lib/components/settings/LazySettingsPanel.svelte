<script lang="ts">
	import { createEventDispatcher, onMount } from 'svelte';

	import Spinner from '$lib/components/common/Spinner.svelte';

	export let load: () => Promise<{ default: any }>;
	export let props: Record<string, unknown> = {};

	const dispatch = createEventDispatcher();
	let component: any = null;
	let loadError: unknown = null;
	let generation = 0;
	const forwardSave = (event: CustomEvent) => dispatch('save', event.detail);

	const loadComponent = async () => {
		const currentGeneration = ++generation;
		component = null;
		loadError = null;
		try {
			const module = await load();
			if (currentGeneration === generation) {
				component = module.default;
			}
		} catch (error) {
			if (currentGeneration === generation) {
				loadError = error;
				console.error('Failed to load settings panel', error);
			}
		}
	};

	onMount(() => {
		let secondFrame = 0;
		const firstFrame = requestAnimationFrame(() => {
			secondFrame = requestAnimationFrame(() => {
				void loadComponent();
			});
		});
		return () => {
			cancelAnimationFrame(firstFrame);
			cancelAnimationFrame(secondFrame);
			generation += 1;
		};
	});
</script>

{#if component}
	<svelte:component this={component} {...props} on:save={forwardSave} />
{:else}
	<div class="flex min-h-48 w-full items-center justify-center text-gray-500 dark:text-gray-400">
		{#if loadError}
			<button
				type="button"
				class="rounded-md px-3 py-2 text-sm hover:bg-gray-100 dark:hover:bg-gray-800"
				on:click={loadComponent}
			>
				Retry
			</button>
		{:else}
			<Spinner className="size-5" />
		{/if}
	</div>
{/if}
