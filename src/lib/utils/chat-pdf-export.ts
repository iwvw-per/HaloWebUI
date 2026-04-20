export type ChatPdfExportMode = 'stylized' | 'compact';

type ExportChatPdfOptions = {
	sourceElement: HTMLElement;
	title?: string | null;
	mode?: ChatPdfExportMode;
	darkMode?: boolean;
};

type RgbaColor = {
	r: number;
	g: number;
	b: number;
	a: number;
};

const PDF_PAGE_WIDTH_MM = 210;
const PDF_PAGE_HEIGHT_MM = 297;

const MODE_CONFIG: Record<
	ChatPdfExportMode,
	{
		width: number;
		scale: number;
		quality: number;
		backgroundColor: (darkMode: boolean) => string;
	}
> = {
	stylized: {
		width: 820,
		scale: 2,
		quality: 0.78,
		backgroundColor: (darkMode) => (darkMode ? '#020617' : '#ffffff')
	},
	compact: {
		width: 760,
		scale: 1.25,
		quality: 0.72,
		backgroundColor: () => '#ffffff'
	}
};

const waitForNextFrame = () =>
	new Promise<void>((resolve) => {
		requestAnimationFrame(() => resolve());
	});

const waitForStableLayout = async () => {
	await waitForNextFrame();
	await waitForNextFrame();

	if (document.fonts?.ready) {
		try {
			await document.fonts.ready;
		} catch {
			// ignore font loading errors and let html2canvas continue
		}
	}

	await new Promise((resolve) => setTimeout(resolve, 60));
};

const parseCssColor = (value: string): RgbaColor | null => {
	if (!value || value === 'transparent' || value === 'initial' || value === 'inherit') {
		return null;
	}

	const rgbMatch = value.match(
		/rgba?\(\s*([\d.]+)[,\s]+([\d.]+)[,\s]+([\d.]+)(?:[,\s/]+([\d.]+))?\s*\)/i
	);

	if (rgbMatch) {
		return {
			r: Number(rgbMatch[1]),
			g: Number(rgbMatch[2]),
			b: Number(rgbMatch[3]),
			a: rgbMatch[4] === undefined ? 1 : Number(rgbMatch[4])
		};
	}

	const hexMatch = value.trim().match(/^#([0-9a-f]{3}|[0-9a-f]{6}|[0-9a-f]{8})$/i);
	if (!hexMatch) {
		return null;
	}

	let hex = hexMatch[1];
	if (hex.length === 3) {
		hex = hex
			.split('')
			.map((char) => `${char}${char}`)
			.join('');
	}

	if (hex.length === 6) {
		return {
			r: parseInt(hex.slice(0, 2), 16),
			g: parseInt(hex.slice(2, 4), 16),
			b: parseInt(hex.slice(4, 6), 16),
			a: 1
		};
	}

	return {
		r: parseInt(hex.slice(0, 2), 16),
		g: parseInt(hex.slice(2, 4), 16),
		b: parseInt(hex.slice(4, 6), 16),
		a: parseInt(hex.slice(6, 8), 16) / 255
	};
};

const getLuminance = (color: RgbaColor | null) => {
	if (!color) {
		return 1;
	}

	const normalize = (channel: number) => {
		const value = channel / 255;
		return value <= 0.03928 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
	};

	return (
		0.2126 * normalize(color.r) + 0.7152 * normalize(color.g) + 0.0722 * normalize(color.b)
	);
};

const isDarkSurface = (color: RgbaColor | null) =>
	Boolean(color && color.a > 0.15 && getLuminance(color) < 0.38);

const isLightText = (color: RgbaColor | null) =>
	Boolean(color && color.a > 0.1 && getLuminance(color) > 0.82);

const sanitizeFileName = (title?: string | null) => {
	const baseName = (title ?? '').trim() || 'chat';
	return `chat-${baseName.replace(/[<>:"/\\|?*\u0000-\u001f]/g, '_')}.pdf`;
};

const applyCompactAppearance = (root: HTMLElement) => {
	root.style.background = '#ffffff';
	root.style.color = '#111827';

	const elements = [root, ...Array.from(root.querySelectorAll<HTMLElement>('*'))];
	for (const element of elements) {
		element.style.animation = 'none';
		element.style.transition = 'none';
		element.style.backdropFilter = 'none';
		element.style.filter = 'none';
		element.style.boxShadow = 'none';

		const computed = window.getComputedStyle(element);
		const backgroundColor = parseCssColor(computed.backgroundColor);
		const textColor = parseCssColor(computed.color);
		const borderColor = parseCssColor(computed.borderColor);
		const tagName = element.tagName.toLowerCase();
		const isCodeLike = ['pre', 'code', 'blockquote', 'table', 'thead', 'tbody', 'tr', 'td', 'th'].includes(
			tagName
		);

		if (isDarkSurface(backgroundColor)) {
			element.style.backgroundColor = isCodeLike ? '#f3f4f6' : '#ffffff';
		}

		if (isLightText(textColor)) {
			element.style.color = '#111827';
		}

		if (isDarkSurface(borderColor)) {
			element.style.borderColor = '#d1d5db';
		}
	}
};

const buildClone = (sourceElement: HTMLElement, mode: ChatPdfExportMode, width: number) => {
	const clone = sourceElement.cloneNode(true) as HTMLElement;
	clone.style.position = 'absolute';
	clone.style.left = '-20000px';
	clone.style.top = '0';
	clone.style.height = 'auto';
	clone.style.maxWidth = 'none';
	clone.style.width = `${width}px`;
	clone.style.pointerEvents = 'none';
	clone.style.opacity = '1';
	clone.style.zIndex = '-1';
	clone.setAttribute('data-pdf-capture-mode', mode);

	return clone;
};

const saveCanvasAsPdf = async (
	canvas: HTMLCanvasElement,
	title: string | null | undefined,
	quality: number,
	darkMode: boolean
) => {
	const jspdfModule = await import('jspdf');
	const JsPdf = (jspdfModule as any).jsPDF ?? (jspdfModule as any).default;
	const pdf = new JsPdf('p', 'mm', 'a4');
	const pagePixelHeight = Math.floor((canvas.width / PDF_PAGE_WIDTH_MM) * PDF_PAGE_HEIGHT_MM);

	let offsetY = 0;
	let page = 0;

	while (offsetY < canvas.height) {
		const sliceHeight = Math.min(pagePixelHeight, canvas.height - offsetY);
		const pageCanvas = document.createElement('canvas');
		pageCanvas.width = canvas.width;
		pageCanvas.height = sliceHeight;

		const ctx = pageCanvas.getContext('2d');
		if (!ctx) {
			throw new Error('无法创建 PDF 画布。');
		}

		ctx.drawImage(canvas, 0, offsetY, canvas.width, sliceHeight, 0, 0, canvas.width, sliceHeight);

		const imageData = pageCanvas.toDataURL('image/jpeg', quality);
		const imageHeightMM = (sliceHeight * PDF_PAGE_WIDTH_MM) / canvas.width;

		if (page > 0) {
			pdf.addPage();
		}

		if (darkMode) {
			pdf.setFillColor(2, 6, 23);
			pdf.rect(0, 0, PDF_PAGE_WIDTH_MM, PDF_PAGE_HEIGHT_MM, 'F');
		}

		pdf.addImage(imageData, 'JPEG', 0, 0, PDF_PAGE_WIDTH_MM, imageHeightMM);

		offsetY += sliceHeight;
		page += 1;
	}

	pdf.save(sanitizeFileName(title));
};

export const exportChatPdfFromElement = async ({
	sourceElement,
	title,
	mode = 'stylized',
	darkMode = false
}: ExportChatPdfOptions) => {
	const { default: html2canvas } = await import('html2canvas-pro');
	const config = MODE_CONFIG[mode];
	const clone = buildClone(sourceElement, mode, config.width);

	document.body.appendChild(clone);

	try {
		await waitForStableLayout();

		if (mode === 'compact') {
			applyCompactAppearance(clone);
			await waitForNextFrame();
		}

		const canvas = await html2canvas(clone, {
			backgroundColor: config.backgroundColor(darkMode),
			useCORS: true,
			scale: config.scale,
			width: config.width,
			windowWidth: config.width,
			logging: false
		});

		await saveCanvasAsPdf(canvas, title, config.quality, darkMode && mode === 'stylized');
	} finally {
		clone.remove();
	}
};
