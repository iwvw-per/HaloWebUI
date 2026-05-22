import { describe, expect, it } from 'vitest';

import {
	buildNewUserDefaultSettingsPayload,
	normalizeNewUserDefaultSettings,
	pickUserDefaultUiFields
} from './user-default-settings';

describe('user default settings helpers', () => {
	it('picks only template-safe user settings from current preferences', () => {
		const picked = pickUserDefaultUiFields({
			models: ['gpt-4o'],
			temporaryChatByDefault: true,
			showInlineCitations: false,
			system: 'You are helpful.',
			title: { auto: false, prompt: 'private title prompt' },
			imageCompressionSize: { width: 1600, height: 900 },
			landingPageMode: 'chat',
			pinnedModels: ['gpt-4.1'],
			modelSelectorTagOrder: ['OpenAI'],
			connections: { openai: { OPENAI_API_KEYS: ['secret'] } },
			notifications: { webhook_url: 'https://example.test/hook' },
			userLocation: true,
			notificationEnabled: true,
			audio: {
				stt: { engine: 'web', language: 'zh-CN' },
				tts: { voice: 'personal' }
			}
		});

		expect(picked).toMatchObject({
			models: ['gpt-4o'],
			temporaryChatByDefault: true,
			showInlineCitations: false,
			system: 'You are helpful.',
			title: { auto: false },
			imageCompressionSize: { width: '1600', height: '900' }
		});
		expect(picked).not.toHaveProperty('landingPageMode');
		expect(picked).not.toHaveProperty('pinnedModels');
		expect(picked).not.toHaveProperty('modelSelectorTagOrder');
		expect(picked).not.toHaveProperty('connections');
		expect(picked).not.toHaveProperty('notifications');
		expect(picked).not.toHaveProperty('userLocation');
		expect(picked).not.toHaveProperty('notificationEnabled');
		expect(picked).not.toHaveProperty('audio');
	});

	it('turns sampled preferences into a saveable draft without unsafe fields', () => {
		const draft = normalizeNewUserDefaultSettings({
			enabled: false,
			roles: ['user', 'pending'],
			ui: pickUserDefaultUiFields({
				models: ['gpt-4o'],
				temporaryChatByDefault: true,
				landingPageMode: 'chat',
				connections: { openai: { OPENAI_API_KEYS: ['secret'] } }
			}),
			tools: {
				native_tools: {
					TOOL_CALLING_MODE: 'native',
					ENABLE_WEB_SEARCH_TOOL: false
				}
			}
		});

		const payload = buildNewUserDefaultSettingsPayload(draft);

		expect(payload.ui).toEqual({
			models: ['gpt-4o'],
			temporaryChatByDefault: true
		});
		expect(payload.tools.native_tools).toEqual({
			TOOL_CALLING_MODE: 'native',
			ENABLE_WEB_SEARCH_TOOL: false
		});
	});

	it('preserves the configured marker for an intentionally empty preset', () => {
		const draft = normalizeNewUserDefaultSettings({
			configured: true,
			enabled: false,
			roles: ['user', 'pending'],
			ui: {},
			tools: { native_tools: {} }
		});

		const payload = buildNewUserDefaultSettingsPayload(draft);

		expect(payload).toEqual({
			configured: true,
			enabled: false,
			roles: ['user', 'pending'],
			ui: {},
			tools: { native_tools: {} }
		});
	});
});
