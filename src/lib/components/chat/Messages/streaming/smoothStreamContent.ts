import { clamp, countStreamCharacters, getStreamNow } from './shared';

type SmoothStreamPreset = 'balanced' | 'fast' | 'silky';

type FrameScheduler = {
	cancelFrame: (id: unknown) => void;
	now: () => number;
	requestFrame: (cb: (timestamp: number) => void) => unknown;
};

type SmoothStreamUpdateHandler = (content: string) => void;

const PRESET_CONFIG = {
	balanced: {
		activeInputWindowMs: 220,
		defaultCps: 38,
		emaAlpha: 0.2,
		flushCps: 120,
		largeAppendChars: 120,
		maxActiveCps: 132,
		maxCps: 72,
		maxFlushCps: 280,
		minCps: 18,
		settleAfterMs: 360,
		settleDrainMaxMs: 520,
		settleDrainMinMs: 180,
		targetBufferMs: 120
	},
	fast: {
		activeInputWindowMs: 140,
		defaultCps: 50,
		emaAlpha: 0.3,
		flushCps: 170,
		largeAppendChars: 180,
		maxActiveCps: 180,
		maxCps: 96,
		maxFlushCps: 360,
		minCps: 24,
		settleAfterMs: 260,
		settleDrainMaxMs: 360,
		settleDrainMinMs: 140,
		targetBufferMs: 40
	},
	silky: {
		activeInputWindowMs: 320,
		defaultCps: 28,
		emaAlpha: 0.14,
		flushCps: 96,
		largeAppendChars: 100,
		maxActiveCps: 102,
		maxCps: 56,
		maxFlushCps: 220,
		minCps: 14,
		settleAfterMs: 460,
		settleDrainMaxMs: 680,
		settleDrainMinMs: 240,
		targetBufferMs: 170
	}
} as const;

const defaultFrameScheduler = (): FrameScheduler => ({
	cancelFrame: (id) => {
		if (typeof cancelAnimationFrame === 'function') {
			cancelAnimationFrame(id as number);
			return;
		}
		clearTimeout(id as ReturnType<typeof setTimeout>);
	},
	now: getStreamNow,
	requestFrame: (cb) => {
		if (typeof requestAnimationFrame === 'function') {
			return requestAnimationFrame(cb);
		}
		return setTimeout(() => cb(getStreamNow()), 16);
	}
});

export const createSmoothStreamContentController = ({
	enabled = true,
	initialContent = '',
	onUpdate = () => {},
	preset = 'balanced',
	scheduler = defaultFrameScheduler()
}: {
	enabled?: boolean;
	initialContent?: string;
	onUpdate?: SmoothStreamUpdateHandler;
	preset?: SmoothStreamPreset;
	scheduler?: FrameScheduler;
} = {}) => {
	const config = PRESET_CONFIG[preset];

	let displayedContent = '';
	let displayedCount = 0;
	let targetContent = '';
	let targetChars: string[] = [];
	let targetCount = 0;
	let emaCps = config.defaultCps;
	let lastInputTs = scheduler.now();
	let lastInputCount = 0;
	let chunkSizeEma = 1;
	let arrivalCpsEma = config.defaultCps;
	let rafId: unknown = null;
	let lastFrameTs: number | null = null;
	let isEnabled = enabled;

	const emit = (nextContent: string) => {
		if (nextContent === displayedContent) {
			displayedContent = nextContent;
			return;
		}

		displayedContent = nextContent;
		onUpdate(nextContent);
	};

	const stopFrameLoop = () => {
		if (rafId !== null) {
			scheduler.cancelFrame(rafId);
			rafId = null;
		}
		lastFrameTs = null;
	};

	const syncImmediate = (nextContent: string) => {
		stopFrameLoop();
		const chars = [...nextContent];
		const now = scheduler.now();

		targetContent = nextContent;
		targetChars = chars;
		targetCount = chars.length;
		displayedCount = chars.length;
		emit(nextContent);

		emaCps = config.defaultCps;
		chunkSizeEma = 1;
		arrivalCpsEma = config.defaultCps;
		lastInputTs = now;
		lastInputCount = chars.length;
	};

	const tick = (timestamp: number) => {
		if (lastFrameTs === null) {
			lastFrameTs = timestamp;
			rafId = scheduler.requestFrame(tick);
			return;
		}

		const dtSeconds = clamp((timestamp - lastFrameTs) / 1000, 0.001, 0.05);
		lastFrameTs = timestamp;

		const backlog = targetCount - displayedCount;
		if (backlog <= 0) {
			if (displayedContent !== targetContent) {
				emit(targetContent);
			}
			stopFrameLoop();
			return;
		}

		const idleMs = scheduler.now() - lastInputTs;
		const inputActive = idleMs <= config.activeInputWindowMs;
		const settling = !inputActive && idleMs >= config.settleAfterMs;
		const baseCps = clamp(emaCps, config.minCps, config.maxCps);
		const baseLagChars = Math.max(1, Math.round((baseCps * config.targetBufferMs) / 1000));
		const lagUpperBound = Math.max(baseLagChars + 2, baseLagChars * 3);
		const targetLagChars = inputActive
			? Math.round(
					clamp(
						baseLagChars + chunkSizeEma * 0.35,
						baseLagChars,
						lagUpperBound
					)
				)
			: 0;
		const desiredDisplayed = Math.max(0, targetCount - targetLagChars);

		let currentCps: number;
		if (inputActive) {
			const backlogPressure = targetLagChars > 0 ? backlog / targetLagChars : 1;
			const chunkPressure = targetLagChars > 0 ? chunkSizeEma / targetLagChars : 1;
			const arrivalPressure = arrivalCpsEma / Math.max(baseCps, 1);
			const combinedPressure = clamp(
				backlogPressure * 0.6 + chunkPressure * 0.25 + arrivalPressure * 0.15,
				1,
				4.5
			);
			const activeCap = clamp(
				config.maxActiveCps + chunkSizeEma * 6,
				config.maxActiveCps,
				config.maxFlushCps
			);
			currentCps = clamp(baseCps * combinedPressure, config.minCps, activeCap);
		} else if (settling) {
			const drainTargetMs = clamp(
				backlog * 8,
				config.settleDrainMinMs,
				config.settleDrainMaxMs
			);
			currentCps = clamp(
				(backlog * 1000) / drainTargetMs,
				config.flushCps,
				config.maxFlushCps
			);
		} else {
			currentCps = clamp(
				Math.max(config.flushCps, baseCps * 1.8, arrivalCpsEma * 0.8),
				config.flushCps,
				config.maxFlushCps
			);
		}

		const urgentBacklog =
			inputActive && targetLagChars > 0 && backlog > targetLagChars * 2.2;
		const burstyInput =
			inputActive && targetLagChars > 0 && chunkSizeEma >= targetLagChars * 0.9;
		const minRevealChars = inputActive ? (urgentBacklog || burstyInput ? 2 : 1) : 2;

		let revealChars = Math.max(minRevealChars, Math.round(currentCps * dtSeconds));
		if (inputActive) {
			const shortfall = desiredDisplayed - displayedCount;
			if (shortfall <= 0) {
				rafId = scheduler.requestFrame(tick);
				return;
			}
			revealChars = Math.min(revealChars, shortfall, backlog);
		} else {
			revealChars = Math.min(revealChars, backlog);
		}

		const nextCount = displayedCount + revealChars;
		const segment = targetChars.slice(displayedCount, nextCount).join('');
		if (segment) {
			displayedCount = nextCount;
			emit(displayedContent + segment);
		} else {
			displayedCount = targetCount;
			emit(targetContent);
		}

		rafId = scheduler.requestFrame(tick);
	};

	const startFrameLoop = () => {
		if (rafId !== null) {
			return;
		}

		rafId = scheduler.requestFrame(tick);
	};

	const setEnabled = (nextEnabled: boolean) => {
		isEnabled = nextEnabled;
		if (!isEnabled) {
			syncImmediate(targetContent);
			return;
		}

		if (displayedContent !== targetContent) {
			lastInputTs = scheduler.now();
			lastInputCount = displayedCount;
			startFrameLoop();
		}
	};

	const setContent = (nextContent: string) => {
		if (!isEnabled) {
			syncImmediate(nextContent);
			return;
		}

		const previousTargetContent = targetContent;
		if (nextContent === previousTargetContent) {
			return;
		}

		const now = scheduler.now();
		if (!nextContent.startsWith(previousTargetContent)) {
			syncImmediate(nextContent);
			return;
		}

		const appendedChars = [...nextContent.slice(previousTargetContent.length)];
		const appendedCount = appendedChars.length;
		if (appendedCount > config.largeAppendChars) {
			syncImmediate(nextContent);
			return;
		}

		targetContent = nextContent;
		targetChars = [...targetChars, ...appendedChars];
		targetCount += appendedCount;

		const deltaChars = targetCount - lastInputCount;
		const deltaMs = Math.max(1, now - lastInputTs);
		if (deltaChars > 0) {
			const instantCps = (deltaChars * 1000) / deltaMs;
			const normalizedInstantCps = clamp(
				instantCps,
				config.minCps,
				config.maxFlushCps * 2
			);
			const chunkEmaAlpha = 0.35;
			chunkSizeEma = chunkSizeEma * (1 - chunkEmaAlpha) + appendedCount * chunkEmaAlpha;
			arrivalCpsEma =
				arrivalCpsEma * (1 - chunkEmaAlpha) + normalizedInstantCps * chunkEmaAlpha;
			const clampedCps = clamp(instantCps, config.minCps, config.maxActiveCps);
			emaCps = emaCps * (1 - config.emaAlpha) + clampedCps * config.emaAlpha;
		}

		lastInputTs = now;
		lastInputCount = targetCount;
		startFrameLoop();
	};

	const destroy = () => {
		stopFrameLoop();
	};

	syncImmediate(initialContent);

	return {
		destroy,
		getDisplayedContent: () => displayedContent,
		setContent,
		setEnabled
	};
};

