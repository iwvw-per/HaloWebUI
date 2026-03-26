import { describe, expect, it } from 'vitest';

import {
	CHAT_TRANSITION_PREVIEW_CONTENT,
	CHAT_TRANSITION_PREVIEW_STREAMING_SPEED,
	getPreviewChunkStep,
	getPreviewInitialContent,
	randomPreviewIntRange
} from './chatTransitionPreviewFrames';

describe('chat transition preview data', () => {
	it('keeps the canonical markdown content aligned with lobehub preview copy', () => {
		expect(CHAT_TRANSITION_PREVIEW_CONTENT).toContain('### Features');
		expect(CHAT_TRANSITION_PREVIEW_CONTENT).toContain('**Key Highlights**');
		expect(CHAT_TRANSITION_PREVIEW_CONTENT).toContain('`gpt-4-vision`');
		expect(CHAT_TRANSITION_PREVIEW_CONTENT).toContain('Function Calling & real-time data');
		expect(CHAT_TRANSITION_PREVIEW_STREAMING_SPEED).toBe(25);
	});

	it('matches lobehub none mode initial content and chunk sizing rules', () => {
		const initialContent = getPreviewInitialContent('none', () => 42);
		const chunkStep = getPreviewChunkStep('none', () => 4);

		expect(initialContent).toBe(CHAT_TRANSITION_PREVIEW_CONTENT.slice(0, 42));
		expect(chunkStep).toBe(Math.ceil(CHAT_TRANSITION_PREVIEW_CONTENT.length / 4));
	});

	it('matches lobehub fadeIn and smooth defaults', () => {
		expect(getPreviewInitialContent('fadeIn')).toBe('');
		expect(getPreviewInitialContent('smooth')).toBe('');
		expect(getPreviewChunkStep('fadeIn')).toBe(3);
		expect(getPreviewChunkStep('smooth')).toBe(3);
	});

	it('generates integers inside the requested bounds', () => {
		for (let index = 0; index < 20; index += 1) {
			const value = randomPreviewIntRange(3, 5);
			expect(value).toBeGreaterThanOrEqual(3);
			expect(value).toBeLessThanOrEqual(5);
			expect(Number.isInteger(value)).toBe(true);
		}
	});
});
