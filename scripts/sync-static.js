import { readdir, readFile, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';

const sourceRoot = resolve('static');

// ── SVG color analysis helpers (shared by model-icons & connection-avatars) ──

const isHexMono = (hex) => {
	const h = hex.replace(/^#/, '');
	const expand = (ch) => ch + ch;
	let r, g, b;
	if (h.length === 3 || h.length === 4) {
		r = parseInt(expand(h[0]), 16);
		g = parseInt(expand(h[1]), 16);
		b = parseInt(expand(h[2]), 16);
	} else if (h.length === 6 || h.length === 8) {
		r = parseInt(h.slice(0, 2), 16);
		g = parseInt(h.slice(2, 4), 16);
		b = parseInt(h.slice(4, 6), 16);
	} else {
		return false;
	}
	return Number.isFinite(r) && Number.isFinite(g) && Number.isFinite(b) && r === g && g === b;
};

const isRgbMono = (value) => {
	const m = value.replace(/\s+/g, '').match(/^(rgba?|hsla?)\(([^)]+)\)$/);
	if (!m) return false;
	if (m[1].startsWith('hsl')) return false;
	const parts = m[2].split(',').slice(0, 3);
	if (parts.length !== 3) return false;
	const nums = parts.map((p) => (p.endsWith('%') ? NaN : Number(p)));
	if (nums.some((n) => !Number.isFinite(n))) return false;
	const [r, g, b] = nums;
	return r === g && g === b;
};

const ALLOWED_NAMED = new Set(['black', 'white', 'currentcolor', 'none', 'transparent']);

/** Returns true if the SVG content has explicit non-monochrome colors. */
const svgHasExplicitColor = (lower) => {
	// Gradients / url fills → multi-color
	if (
		lower.includes('lineargradient') ||
		lower.includes('radialgradient') ||
		lower.includes('stop-color') ||
		lower.includes('fill="url(') ||
		lower.includes("fill='url(") ||
		lower.includes('stroke="url(') ||
		lower.includes("stroke='url(")
	)
		return true;

	for (const m of lower.matchAll(/#([0-9a-f]{3,8})/g)) {
		if (!isHexMono(`#${m[1]}`)) return true;
	}
	for (const m of lower.matchAll(/\b(?:rgba?|hsla?)\([^)]+\)/g)) {
		if (!isRgbMono(m[0])) return true;
	}

	const extractValues = (re) => [...lower.matchAll(re)].map((m) => (m[1] ?? '').trim());
	const attrValues = extractValues(/\b(?:fill|stroke)=['"]([^'"]+)['"]/g);
	const styleValues = extractValues(/\b(?:fill|stroke)\s*:\s*([^;]+);/g);

	for (const raw of [...attrValues, ...styleValues]) {
		if (!raw) continue;
		if (raw.startsWith('url(')) return true;
		if (raw.startsWith('#')) {
			if (!isHexMono(raw)) return true;
			continue;
		}
		if (raw.startsWith('rgb') || raw.startsWith('hsl')) {
			if (!isRgbMono(raw)) return true;
			continue;
		}
		if (/^[a-z]+$/.test(raw) && !ALLOWED_NAMED.has(raw)) return true;
	}
	return false;
};

/** Scan a directory for monochrome SVGs that should be inverted in dark mode. */
const findInvertCandidates = async (dir, files) => {
	const candidates = [];
	for (const filename of files) {
		if (!filename.toLowerCase().endsWith('.svg')) continue;
		if (/-color\.svg$/i.test(filename)) continue;
		const content = await readFile(resolve(dir, filename), 'utf8').catch(() => '');
		if (!content) continue;
		if (svgHasExplicitColor(content.toLowerCase())) continue;
		candidates.push(filename);
	}
	return candidates;
};

try {
	// Keep the model icon list in sync with the on-disk folder, so new icons work without manual TS edits.
	// This runs as part of `npm run dev/build` via `npm run sync:static`.
	const modelIconsDir = resolve(sourceRoot, 'static/model-icons');
	const manifestPath = resolve('src/lib/utils/model-icons.manifest.ts');
	try {
		const entries = await readdir(modelIconsDir, { withFileTypes: true });
		const files = entries
			.filter((e) => e.isFile() && !e.name.includes(':'))
			.map((e) => e.name)
			.sort((a, b) => a.localeCompare(b, 'en', { sensitivity: 'base' }));

		const invertCandidates = await findInvertCandidates(modelIconsDir, files);

		const next =
			`export const MODEL_ICON_FILES = ${JSON.stringify(files, null, '\t')} as const;\n` +
			`\n` +
			`export const DARK_MODE_INVERT_ICONS = new Set(${JSON.stringify(
				invertCandidates,
				null,
				'\t'
			)}) as ReadonlySet<string>;\n`;
		const prev = await readFile(manifestPath, 'utf8').catch(() => null);

		if (prev !== next) {
			await writeFile(manifestPath, next, 'utf8');
		}
	} catch (error) {
		// If the folder doesn't exist yet, skip manifest generation.
	}

	// Generate connection-avatars manifest (same pattern as model-icons, with dark-mode invert set).
	const connAvatarsDir = resolve(sourceRoot, 'static/connection-avatars');
	const connManifestPath = resolve('src/lib/utils/connection-avatars.manifest.ts');
	try {
		const entries = await readdir(connAvatarsDir, { withFileTypes: true });
		const files = entries
			.filter((e) => e.isFile() && !e.name.includes(':'))
			.map((e) => e.name)
			.sort((a, b) => a.localeCompare(b, 'en', { sensitivity: 'base' }));

		const invertCandidates = await findInvertCandidates(connAvatarsDir, files);

		const next =
			`export const CONNECTION_AVATAR_FILES = ${JSON.stringify(files, null, '\t')} as const;\n` +
			`\n` +
			`export const DARK_MODE_INVERT_CONN_AVATARS = new Set(${JSON.stringify(
				invertCandidates,
				null,
				'\t'
			)}) as ReadonlySet<string>;\n`;
		const prev = await readFile(connManifestPath, 'utf8').catch(() => null);

		if (prev !== next) {
			await writeFile(connManifestPath, next, 'utf8');
		}
	} catch (error) {
		// If the folder doesn't exist yet, skip manifest generation.
	}
} catch (error) {
	console.error('[sync-static] Failed to sync static assets.', error);
	process.exit(1);
}
