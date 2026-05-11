export const TEXT_SCALE_DEFAULT = 1;
export const TEXT_SCALE_MIN = 0.8;
export const TEXT_SCALE_MAX = 1.5;

const normalizeTextScale = (scale: unknown): number => {
	if (scale === null || scale === undefined || scale === '') {
		return TEXT_SCALE_DEFAULT;
	}

	const parsed = Number(scale);
	if (!Number.isFinite(parsed)) {
		return TEXT_SCALE_DEFAULT;
	}

	// Keep aligned with UI slider bounds in Interface settings.
	if (parsed < TEXT_SCALE_MIN || parsed > TEXT_SCALE_MAX) {
		return TEXT_SCALE_DEFAULT;
	}

	return parsed;
};

export const setTextScale = (scale: unknown) => {
	if (typeof document === 'undefined') {
		return;
	}

	document.documentElement.style.setProperty('--app-text-scale', `${normalizeTextScale(scale)}`);
};
