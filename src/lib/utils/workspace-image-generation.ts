export const CUSTOM_SIZE_OPTION_VALUE = '__custom_size__';

export const WORKSPACE_IMAGE_SIZE_PRESETS = [
	'1024x1024',
	'1024x1536',
	'1536x1024',
	'1536x1536',
	'2048x2048',
	'2048x3072',
	'3072x2048'
] as const;

const IMAGE_SIZE_RE = /^(\d+)x(\d+)$/i;

export type ParsedImageSize = {
	value: string;
	width: number;
	height: number;
	pixels: number;
	aspectRatio: string;
	aspectValue: number;
};

export type LearnedImageConstraint = {
	minPixels?: number;
	requestId?: string;
	rawMessage?: string;
	updatedAt?: number;
};

const gcd = (a: number, b: number): number => (b === 0 ? a : gcd(b, a % b));

const getErrorText = (error: unknown): string => {
	if (typeof error === 'string') {
		return error;
	}

	if (error instanceof Error) {
		return error.message;
	}

	if (error && typeof error === 'object') {
		if ('detail' in error && typeof error.detail === 'string') {
			return error.detail;
		}

		if ('message' in error && typeof error.message === 'string') {
			return error.message;
		}
	}

	return `${error ?? ''}`;
};

export const parseImageSize = (value: unknown): ParsedImageSize | null => {
	const normalized = `${value ?? ''}`.trim().toLowerCase();
	const match = normalized.match(IMAGE_SIZE_RE);
	if (!match) {
		return null;
	}

	const width = Number(match[1]);
	const height = Number(match[2]);
	if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) {
		return null;
	}

	const divisor = gcd(width, height);
	return {
		value: `${width}x${height}`,
		width,
		height,
		pixels: width * height,
		aspectRatio: divisor > 0 ? `${width / divisor}:${height / divisor}` : `${width}:${height}`,
		aspectValue: width / height
	};
};

export const formatPixelCount = (value: number): string => {
	if (!Number.isFinite(value)) {
		return `${value ?? ''}`;
	}

	return new Intl.NumberFormat('en-US').format(value);
};

export const extractImageConstraintFromError = (
	error: unknown
): LearnedImageConstraint | null => {
	const text = getErrorText(error).replace(/\s+/g, ' ').trim();
	if (!text) {
		return null;
	}

	const requestId = text.match(/request id:\s*([a-z0-9_-]+)/i)?.[1];
	const minPixelsMatch = text.match(/image size must be at least\s+(\d+)\s+pixels/i);
	const sizeRejected = /parameter\s*[`'"]?size[`'"]?.*not valid/i.test(text);

	if (!sizeRejected && !minPixelsMatch) {
		return null;
	}

	return {
		...(minPixelsMatch ? { minPixels: Number(minPixelsMatch[1]) } : {}),
		...(requestId ? { requestId } : {}),
		rawMessage: text,
		updatedAt: Date.now()
	};
};

export const getRecommendedImageSizes = (
	currentSize: unknown,
	options: {
		minPixels?: number;
		limit?: number;
		candidates?: readonly string[];
	} = {}
): string[] => {
	const current = parseImageSize(currentSize);
	const minPixels = Number(options.minPixels ?? 0);
	const limit = Math.max(1, Number(options.limit ?? 3));
	const candidates = options.candidates ?? WORKSPACE_IMAGE_SIZE_PRESETS;

	return [...candidates]
		.map((candidate) => parseImageSize(candidate))
		.filter((candidate): candidate is ParsedImageSize => Boolean(candidate))
		.filter((candidate) => (minPixels > 0 ? candidate.pixels >= minPixels : true))
		.sort((a, b) => {
			if (current) {
				const ratioDeltaA = Math.abs(a.aspectValue - current.aspectValue);
				const ratioDeltaB = Math.abs(b.aspectValue - current.aspectValue);
				if (ratioDeltaA !== ratioDeltaB) {
					return ratioDeltaA - ratioDeltaB;
				}
			}

			if (a.pixels !== b.pixels) {
				return a.pixels - b.pixels;
			}

			return a.value.localeCompare(b.value);
		})
		.filter((candidate) => candidate.value !== current?.value)
		.slice(0, limit)
		.map((candidate) => candidate.value);
};
