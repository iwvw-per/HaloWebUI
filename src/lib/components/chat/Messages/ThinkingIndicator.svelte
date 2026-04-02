<script lang="ts">
	import { onMount, onDestroy, getContext } from 'svelte';
	import type { Writable } from 'svelte/store';
	import type { i18n as i18nType } from 'i18next';

	const i18n = getContext<Writable<i18nType>>('i18n');

	export let statusHistory: {
		done: boolean;
		action: string;
		description: string;
		urls?: string[];
		query?: string;
		count?: number;
		failed?: number;
		hidden?: boolean;
	}[] = [];
	export let messageTimestamp: number = 0;

	let elapsed = 0;
	let interval: ReturnType<typeof setInterval>;

	function calculateElapsed(): number {
		if (messageTimestamp > 0) {
			return Math.max(0, Math.floor(Date.now() / 1000 - messageTimestamp));
		}
		return 0;
	}

	onMount(() => {
		elapsed = calculateElapsed();
		interval = setInterval(() => {
			elapsed = calculateElapsed();
		}, 1000);
	});

	onDestroy(() => {
		if (interval) clearInterval(interval);
	});

	function formatTime(seconds: number): string {
		if (seconds < 60) return `${seconds}s`;
		const mins = Math.floor(seconds / 60);
		const secs = seconds % 60;
		return secs > 0 ? `${mins}m ${secs}s` : `${mins}m`;
	}

	function getStatusText(status: any): string {
		if (!status?.description) return '';
		const desc = status.description;

		if (
			desc.includes('{{searchQuery}}') ||
			desc.includes('{{count}}') ||
			desc.includes('{{failed}}')
		) {
			return $i18n.t(desc, {
				searchQuery: status.query || '',
				count: status.count ?? status.urls?.length ?? 0,
				failed: status.failed ?? 0
			});
		}
		if (desc.includes('{{model}}')) {
			return $i18n.t('Waiting for model response');
		}

		// Common backend descriptions
		if (desc === 'Generating search query') return $i18n.t('Generating search query');
		if (desc === 'No search query generated') return $i18n.t('No search query generated');
		if (desc === 'No search results found') return $i18n.t('No search results found');

		return $i18n.t(desc);
	}

	$: visibleStatuses = (statusHistory || []).filter((s) => !s.hidden);

	interface Step {
		label: string;
		completed: boolean;
	}

	$: steps = (() => {
		const result: Step[] = [];

		// Step 1: Request sent (always completed)
		result.push({ label: '请求已发送', completed: true });

		// Steps from statusHistory
		for (const status of visibleStatuses) {
			const text = getStatusText(status);
			if (text) {
				result.push({
					label: text,
					completed: status.done !== false
				});
			}
		}

		// If all steps done or no statuses, add a "waiting" step
		const lastVisible = visibleStatuses[visibleStatuses.length - 1];
		if (!lastVisible || lastVisible.done !== false) {
			result.push({ label: '等待模型响应', completed: false });
		}

		return result;
	})();
</script>

<div class="py-3 pl-1 pr-2 select-none">
	<div class="flex items-center gap-1 mb-2">
		<div class="flex gap-1">
			<div class="w-1.5 h-1.5 rounded-full bg-blue-500/60 animate-bounce [animation-delay:-0.3s]" />
			<div
				class="w-1.5 h-1.5 rounded-full bg-blue-500/60 animate-bounce [animation-delay:-0.15s]"
			/>
			<div class="w-1.5 h-1.5 rounded-full bg-blue-500/60 animate-bounce" />
		</div>
		<span class="text-xs text-gray-500 dark:text-gray-400 font-medium">思考中...</span>
	</div>

	{#each steps as step, i}
		<div class="flex items-start gap-2.5 relative ml-3">
			<!-- Vertical connecting line -->
			{#if i < steps.length - 1}
				<div
					class="absolute left-[7px] top-[18px] bottom-0 w-[1.5px] transition-colors duration-300
						{step.completed ? 'bg-blue-300/60 dark:bg-blue-600/40' : 'bg-gray-200 dark:bg-gray-700/60'}"
				/>
			{/if}

			<!-- Step indicator circle -->
			<div class="relative z-10 flex-shrink-0 mt-[2px]">
				{#if step.completed}
					<div
						class="w-[14px] h-[14px] rounded-full bg-gradient-to-br from-blue-100 to-blue-200 dark:from-blue-500/30 dark:to-blue-600/20 flex items-center justify-center shadow-sm"
					>
						<svg
							class="w-2.5 h-2.5 text-blue-500 dark:text-blue-400"
							fill="none"
							viewBox="0 0 24 24"
							stroke="currentColor"
							stroke-width="3"
						>
							<path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
						</svg>
					</div>
				{:else}
					<div
						class="w-[14px] h-[14px] rounded-full bg-gray-50 dark:bg-gray-800/50 flex items-center justify-center ring-1 ring-gray-200 dark:ring-gray-700"
					>
						<div
							class="w-2 h-2 rounded-full bg-gradient-to-r from-blue-400 to-blue-600 thinking-dot"
						/>
					</div>
				{/if}
			</div>

			<!-- Step label and timer -->
			<div class="flex items-center gap-2 pb-2.5 min-h-[20px]">
				<span
					class="text-[13px] leading-[18px] transition-colors duration-200
						{step.completed ? 'text-gray-400 dark:text-gray-500' : 'text-gray-600 dark:text-gray-300'}"
				>
					{step.label}
				</span>
				{#if !step.completed}
					<span class="text-[11px] text-gray-400/80 dark:text-gray-500/80 tabular-nums font-mono">
						{formatTime(elapsed)}
					</span>
				{/if}
			</div>
		</div>
	{/each}
</div>

<style>
	.thinking-dot {
		animation: thinkingPulse 1.4s cubic-bezier(0.4, 0, 0.2, 1) infinite;
	}

	@keyframes thinkingPulse {
		0%,
		100% {
			opacity: 1;
			transform: scale(1);
		}
		50% {
			opacity: 0.5;
			transform: scale(0.85);
		}
	}
</style>
