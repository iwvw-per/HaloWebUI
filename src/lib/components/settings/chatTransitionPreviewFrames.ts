import type { ChatTransitionMode } from '$lib/utils/lobehub-chat-appearance';

export const CHAT_TRANSITION_PREVIEW_CONTENT = `
### Features

**Key Highlights**
- 🌐 Multi-model: GPT-4/Gemini/Ollama
- 🖼️ Vision: \`gpt-4-vision\` integration
- 🛠️ Plugins: Function Calling & real-time data
`;

export const CHAT_TRANSITION_PREVIEW_STREAMING_SPEED = 25;

export const randomPreviewIntRange = (min = 0, max = min + 10) =>
	Math.floor(Math.random() * (max - min + 1)) + min;

export const getPreviewInitialContent = (
	mode: ChatTransitionMode,
	randomIntRange: typeof randomPreviewIntRange = randomPreviewIntRange
) => {
	if (mode === 'none') {
		return CHAT_TRANSITION_PREVIEW_CONTENT.slice(0, Math.max(0, randomIntRange(10, 100)));
	}

	return '';
};

export const getPreviewChunkStep = (
	mode: ChatTransitionMode,
	randomIntRange: typeof randomPreviewIntRange = randomPreviewIntRange
) => {
	if (mode === 'none') {
		return Math.ceil(CHAT_TRANSITION_PREVIEW_CONTENT.length / randomIntRange(3, 5));
	}

	return 3;
};
