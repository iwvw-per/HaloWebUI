<script lang="ts">
	import { createEventDispatcher, getContext } from 'svelte';
	import type { Writable } from 'svelte/store';
	import Spinner from '$lib/components/common/Spinner.svelte';

	const i18n: Writable<any> = getContext('i18n');
	const dispatch = createEventDispatcher();

	export let dirty = false;
	export let saving = false;
	export let disabled = false;
	export let saveAsSubmit = true;
	export let align = 'end';
	export let forceShow = false;

	const dirtyLabelClass =
		'inline-flex items-center gap-1.5 whitespace-nowrap text-xs font-medium leading-none text-rose-600 dark:text-rose-300';
	const dirtyDotClass = 'mt-[0.5px] h-1.5 w-1.5 shrink-0 rounded-full bg-rose-400 dark:bg-rose-300';
	const buttonsClass = 'flex flex-wrap items-center gap-2.5';
	const actionButtonBaseClass =
		'inline-flex h-9 min-w-[80px] items-center justify-center px-3.5 text-xs leading-none transition-all duration-200 disabled:cursor-not-allowed disabled:opacity-50';
	const resetButtonClass =
		`${actionButtonBaseClass} rounded-xl border border-gray-200/90 bg-white font-medium text-gray-600 shadow-[0_12px_24px_-20px_rgba(15,23,42,0.28)] hover:-translate-y-[1px] hover:border-gray-300 hover:bg-gray-50 hover:text-gray-800 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-300 dark:hover:border-gray-600 dark:hover:bg-gray-800 dark:hover:text-gray-100`;
	const saveButtonClass =
		`${actionButtonBaseClass} gap-1.5 rounded-xl bg-gray-800 font-medium text-white shadow-[0_12px_26px_-18px_rgba(31,41,55,0.42)] ring-1 ring-black/5 hover:-translate-y-[1px] hover:bg-gray-700 hover:shadow-[0_14px_28px_-18px_rgba(31,41,55,0.46)] active:scale-[0.98] dark:bg-white dark:text-gray-900 dark:ring-white/10 dark:hover:bg-gray-100`;
</script>

{#if dirty || forceShow}
	<div
		class={`flex flex-wrap items-center gap-2.5 ${align === 'end' ? 'justify-end' : ''}`}
		data-no-toggle
		on:click|stopPropagation
		on:keydown|stopPropagation
	>
		{#if dirty}
			<span class={dirtyLabelClass}>
				<span class={dirtyDotClass}></span>
				{$i18n.t('Unsaved')}
			</span>
		{/if}

		<div class={buttonsClass}>
			{#if dirty}
				<button
					type="button"
					class={resetButtonClass}
					disabled={disabled || saving}
					on:click={() => dispatch('reset')}
				>
					{$i18n.t('Reset')}
				</button>
			{/if}

			<button
				type={saveAsSubmit ? 'submit' : 'button'}
				class={saveButtonClass}
				disabled={disabled || saving}
				on:click={() => {
					if (!saveAsSubmit) {
						dispatch('save');
					}
				}}
			>
				{#if saving}
					<Spinner className="size-3.5" />
					{$i18n.t('Saving...')}
				{:else}
					{$i18n.t('Save')}
				{/if}
			</button>
		</div>
	</div>
{/if}
