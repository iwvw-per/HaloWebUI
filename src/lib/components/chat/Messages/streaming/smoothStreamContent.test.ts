import { describe, expect, it } from 'vitest';

import { createSmoothStreamContentController } from './smoothStreamContent';

const createMockFrameScheduler = () => {
	let now = 0;
	let nextId = 1;
	const callbacks = new Map<number, (timestamp: number) => void>();

	return {
		cancelFrame(id: unknown) {
			callbacks.delete(id as number);
		},
		flushFrame(delta = 16) {
			const pending = [...callbacks.entries()];
			callbacks.clear();
			now += delta;
			for (const [, callback] of pending) {
				callback(now);
			}
		},
		hasPendingFrames() {
			return callbacks.size > 0;
		},
		now() {
			return now;
		},
		requestFrame(callback: (timestamp: number) => void) {
			const id = nextId++;
			callbacks.set(id, callback);
			return id;
		}
	};
};

describe('createSmoothStreamContentController', () => {
	it('smoothly reveals small incremental updates', () => {
		const scheduler = createMockFrameScheduler();
		const updates: string[] = [];
		const controller = createSmoothStreamContentController({
			onUpdate: (content) => {
				updates.push(content);
			},
			scheduler
		});

		controller.setContent('Hello world');

		expect(controller.getDisplayedContent()).toBe('');

		scheduler.flushFrame();
		scheduler.flushFrame();

		expect(controller.getDisplayedContent().length).toBeGreaterThan(0);
		expect(controller.getDisplayedContent().length).toBeLessThan('Hello world'.length);

		while (scheduler.hasPendingFrames()) {
			scheduler.flushFrame();
		}

		expect(controller.getDisplayedContent()).toBe('Hello world');
		expect(updates.at(-1)).toBe('Hello world');
	});

	it('does not jump to the full content for moderately large appends', () => {
		const scheduler = createMockFrameScheduler();
		const controller = createSmoothStreamContentController({ scheduler });
		const content = 'A'.repeat(60);

		controller.setContent(content);
		scheduler.flushFrame();
		scheduler.flushFrame();

		expect(controller.getDisplayedContent().length).toBeGreaterThan(0);
		expect(controller.getDisplayedContent().length).toBeLessThan(content.length);
	});

	it('syncs immediately when the incoming content resets instead of appending', () => {
		const scheduler = createMockFrameScheduler();
		const controller = createSmoothStreamContentController({ scheduler });

		controller.setContent('Streaming in progress');
		scheduler.flushFrame();
		scheduler.flushFrame();

		controller.setContent('Reset');

		expect(controller.getDisplayedContent()).toBe('Reset');
	});
});

