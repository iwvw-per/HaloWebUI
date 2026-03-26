export const STREAM_FADE_DURATION = 280;

export const clamp = (value: number, min: number, max: number) => {
	return Math.min(max, Math.max(min, value));
};

export const countStreamCharacters = (text: string) => {
	return [...(text ?? '')].length;
};

export const getStreamNow = () => {
	return typeof performance === 'undefined' ? Date.now() : performance.now();
};

