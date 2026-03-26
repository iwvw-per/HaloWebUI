import { describe, expect, it } from 'vitest';

import {
	DEFAULT_CHAT_TRANSITION_MODE,
	resolveChatTransitionMode
} from './lobehub-chat-appearance';

describe('resolveChatTransitionMode', () => {
	it('prefers the explicit transition mode', () => {
		expect(resolveChatTransitionMode({ transitionMode: 'smooth', chatFadeStreamingText: false })).toBe(
			'smooth'
		);
	});

	it('maps legacy enabled fade toggle to fadeIn', () => {
		expect(resolveChatTransitionMode({ chatFadeStreamingText: true })).toBe('fadeIn');
	});

	it('maps legacy disabled fade toggle to none', () => {
		expect(resolveChatTransitionMode({ chatFadeStreamingText: false })).toBe('none');
	});

	it('falls back to the default mode when no value is present', () => {
		expect(resolveChatTransitionMode({})).toBe(DEFAULT_CHAT_TRANSITION_MODE);
	});
});

