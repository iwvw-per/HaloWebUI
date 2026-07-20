<script lang="ts">
	import { v4 as uuidv4 } from 'uuid';

	import { getContext, onMount, tick, onDestroy } from 'svelte';
	import type { Writable } from 'svelte/store';
	import type { i18n as i18nType } from 'i18next';
	import { copyToClipboard } from '$lib/utils';

	import CodeEditor from '$lib/components/common/CodeEditor.svelte';
	import SvgPanZoom from '$lib/components/common/SVGPanZoom.svelte';
	import {
		Check,
		ChevronDown,
		ChevronUp,
		ChevronsUpDown,
		Copy,
		LoaderCircle,
		PanelRightOpen,
		Play,
		Save
	} from 'lucide-svelte';
	import {
		artifactAutoOpenDismissedMessageId,
		artifactPreviewTarget,
		config,
		settings,
		showArtifacts,
		showControls,
		showOverview
	} from '$lib/stores';
	import { executeCode } from '$lib/apis/utils';
	import { toast } from 'svelte-sonner';
	import {
		approvePyodideConsent,
		canUsePyodideRuntime,
		getPyodideDownloadSummary,
		getPyodidePackagesForCode,
		hasPyodideConsent,
		usesRemotePyodideRuntime
	} from '$lib/utils/browser-ai-assets';
	import { getLanguageIcon } from '$lib/utils/language-icons';
	import {
		DEFAULT_MERMAID_THEME,
		normalizeMermaidTheme,
		renderMermaidSvg
	} from '$lib/utils/lobehub-chat-appearance';

	const i18n = getContext<Writable<i18nType>>('i18n');

	type ExecutionFile = {
		type: string;
		data: string;
	};

	type CodeToken = {
		raw?: string;
		text?: string;
		[key: string]: unknown;
	};

	export let id = '';

	export let onSave: (value: string) => void = () => {};
	export let onCode: (value: { lang: string; code: string }) => void = () => {};

	export let save = false;
	export let run = true;
	export let collapsed = false;

	export let token: CodeToken | null = null;
	export let lang = '';
	export let code = '';
	export let messageId = '';

	$: langIcon = getLanguageIcon(lang);
	export let attributes: { output?: string } = {};

	export let className = 'my-2';
	export let editorClassName = '';
	export let stickyButtonsClassName = 'top-0';

	let pyodideWorker: Worker | null = null;

	let _code = '';
	$: if (code) {
		updateCode();
	}

	const updateCode = () => {
		_code = code;
	};

	const updateEditorCode = (value?: string) => {
		_code = value ?? '';
	};

	let _token: CodeToken | null = null;
	let mermaidThemeId = DEFAULT_MERMAID_THEME;

	let mermaidHtml: string | null = null;
	let executing = false;

	let stdout: unknown = null;
	let stderr: unknown = null;
	let result: unknown = null;
	let files: ExecutionFile[] = [];

	let copied = false;
	let saved = false;
	let mermaidThemeObserver: MutationObserver | null = null;
	let heightObserver: ResizeObserver | null = null;
	const PYODIDE_DISABLED_MESSAGE = 'Pyodide is disabled in this build.';
	let showPyodideConsent = false;
	let pendingPyodideCode = '';
	let pyodideConsentPackages: string[] = [];

	const MAX_COLLAPSED_HEIGHT = 350;
	let codeWrapperEl: HTMLDivElement = null as unknown as HTMLDivElement;
	let codeNaturalHeight = collapsed ? MAX_COLLAPSED_HEIGHT + 1 : 0;
	$: needsCollapse = codeNaturalHeight > MAX_COLLAPSED_HEIGHT;

	const collapseCodeBlock = () => {
		if (!needsCollapse) return;
		collapsed = !collapsed;
	};

	const isSvgPreviewable = (lang: string, code: string) => {
		const normalizedLang = String(lang ?? '').toLowerCase();
		return normalizedLang === 'svg' || (normalizedLang === 'xml' && code.includes('<svg'));
	};

	const isIframePreviewable = (lang: string) => {
		const normalizedLang = String(lang ?? '').toLowerCase();
		return ['html', 'css', 'javascript', 'js'].includes(normalizedLang);
	};

	const openIframePreview = () => {
		if (!messageId) return;

		artifactAutoOpenDismissedMessageId.set(null);
		artifactPreviewTarget.set({
			messageId,
			type: 'iframe'
		});
		showOverview.set(false);
		showArtifacts.set(true);
		showControls.set(true);
	};

	const previewSvg = () => {
		const previewContent = (_code || code || '').trim();
		if (!previewContent) return;

		artifactAutoOpenDismissedMessageId.set(null);
		artifactPreviewTarget.set({
			messageId,
			type: 'svg',
			content: previewContent
		});
		showOverview.set(false);
		showArtifacts.set(true);
		showControls.set(true);
	};

	const saveCode = () => {
		saved = true;

		code = _code;
		onSave(code);

		setTimeout(() => {
			saved = false;
		}, 1000);
	};

	const copyCode = async () => {
		copied = true;
		await copyToClipboard(code);

		setTimeout(() => {
			copied = false;
		}, 1000);
	};

	const checkPythonCode = (str: string) => {
		// Check if the string contains typical Python syntax characters
		const pythonSyntax = [
			'def ',
			'else:',
			'elif ',
			'try:',
			'except:',
			'finally:',
			'yield ',
			'lambda ',
			'assert ',
			'nonlocal ',
			'del ',
			'True',
			'False',
			'None',
			' and ',
			' or ',
			' not ',
			' in ',
			' is ',
			' with '
		];

		for (let syntax of pythonSyntax) {
			if (str.includes(syntax)) {
				return true;
			}
		}

		// If none of the above conditions met, it's probably not Python code
		return false;
	};

	const hasOwn = (value: unknown, key: string) =>
		Object.prototype.hasOwnProperty.call(value ?? {}, key);

	const getOutputImageType = (data: unknown) => {
		const match = typeof data === 'string' ? data.match(/^data:(image\/[^;]+);base64,/) : null;
		return match?.[1] ?? 'image/png';
	};

	const appendOutputImage = (data: string) => {
		files = [
			...files,
			{
				type: getOutputImageType(data),
				data
			}
		];
	};

	const extractInlineImages = (value: unknown) => {
		if (typeof value !== 'string') {
			return value;
		}

		const visibleLines = [];
		let changed = false;

		for (const line of value.split('\n')) {
			const trimmedLine = line.trim();
			if (trimmedLine.startsWith('data:image/') && trimmedLine.includes(';base64,')) {
				appendOutputImage(trimmedLine);
				changed = true;
			} else {
				visibleLines.push(line);
			}
		}

		return changed ? visibleLines.join('\n').trimEnd() : value;
	};

	const formatResult = (value: unknown) => {
		if (typeof value === 'string') {
			return value;
		}

		try {
			return JSON.stringify(value, null, 2) ?? `${value}`;
		} catch {
			return `${value}`;
		}
	};

	const resetExecutionFeedback = () => {
		result = null;
		stdout = null;
		stderr = null;
		files = [];
	};

	const applyExecutionOutput = (output: unknown) => {
		if (!output || typeof output !== 'object') return;
		const data = output as Record<string, unknown>;

		if (hasOwn(data, 'stdout') && data['stdout'] !== null) {
			stdout = extractInlineImages(data['stdout']);
		}

		if (hasOwn(data, 'result') && data['result'] !== null) {
			result = extractInlineImages(data['result']);
		}

		if (hasOwn(data, 'stderr') && data['stderr'] !== null) {
			stderr = data['stderr'];
		}
	};

	$: hasStdout = stdout !== null && stdout !== undefined && `${stdout}` !== '';
	$: hasStderr = stderr !== null && stderr !== undefined && `${stderr}` !== '';
	$: hasOutputText = hasStdout || hasStderr;
	$: hasResult = result !== null && result !== undefined && result !== '';
	$: hasFiles = files.length > 0;
	$: hasExecutionFeedback = executing || hasOutputText || hasResult || hasFiles;
	$: sourceIsCollapsed = collapsed && needsCollapse;
	$: codeExecutionEnabled = ($config?.features?.enable_code_execution ?? true) === true;

	const executePython = async (code: string) => {
		resetExecutionFeedback();

		executing = true;

		if ($config?.code?.engine === 'jupyter') {
			const output = await executeCode(localStorage.token, code).catch((error) => {
				toast.error(`${error}`);
				return null;
			});

			applyExecutionOutput(output);

			executing = false;
		} else {
			executePythonAsWorker(code);
		}
	};

	const executePythonAsWorker = async (code: string) => {
		if (!canUsePyodideRuntime()) {
			stderr = PYODIDE_DISABLED_MESSAGE;
			executing = false;
			return;
		}

		const packages = getPyodidePackagesForCode(code);

		if (usesRemotePyodideRuntime() && !hasPyodideConsent()) {
			pendingPyodideCode = code;
			pyodideConsentPackages = packages;
			showPyodideConsent = true;
			executing = false;
			return;
		}

		const { default: PyodideWorker } = await import('$lib/workers/pyodide.worker?worker');
		const worker = new PyodideWorker();
		pyodideWorker = worker;

		worker.postMessage({
			id: id,
			code: code,
			packages: packages
		});

		const executionTimeout = setTimeout(() => {
			if (executing) {
				executing = false;
				stderr = 'Execution Time Limit Exceeded';
				worker.terminate();
			}
		}, 60000);

		worker.onmessage = (event) => {
			const { id, ...data } = event.data;
			clearTimeout(executionTimeout);

			applyExecutionOutput(data);

			executing = false;
			worker.terminate();
			if (pyodideWorker === worker) {
				pyodideWorker = null;
			}
		};

		worker.onerror = () => {
			clearTimeout(executionTimeout);
			executing = false;
			worker.terminate();
			if (pyodideWorker === worker) {
				pyodideWorker = null;
			}
		};
	};

	const drawMermaidDiagram = async () => {
		if (typeof document === 'undefined') return;

		try {
			mermaidHtml = await renderMermaidSvg({
				code,
				id: `mermaid-${uuidv4()}`,
				isDark: document.documentElement.classList.contains('dark'),
				themeId: mermaidThemeId
			});
		} catch (error) {
			console.error('Error:', error);
			mermaidHtml = null;
		}
	};

	const render = async () => {
		if (lang === 'mermaid' && (token?.raw ?? '').slice(-4).includes('```')) {
			(async () => {
				await drawMermaidDiagram();
			})();
		}
	};

	$: if (token) {
		if (JSON.stringify(token) !== JSON.stringify(_token)) {
			_token = token;
		}
	}

	$: if (_token) {
		mermaidThemeId = normalizeMermaidTheme($settings?.mermaidTheme ?? DEFAULT_MERMAID_THEME);
		render();
	}

	$: onCode({ lang, code });

	$: if (attributes) {
		onAttributesUpdate();
	}

	const onAttributesUpdate = () => {
		if (attributes?.output) {
			// Create a helper function to unescape HTML entities
			const unescapeHtml = (html: string) => {
				const textArea = document.createElement('textarea');
				textArea.innerHTML = html;
				return textArea.value;
			};

			try {
				// Unescape the HTML-encoded string
				const unescapedOutput = unescapeHtml(attributes.output);

				// Parse the unescaped string into JSON
				const output = JSON.parse(unescapedOutput);

				resetExecutionFeedback();
				applyExecutionOutput(output);
			} catch (error) {
				console.error('Error:', error);
			}
		}
	};

	onMount(async () => {
		if (lang) {
			onCode({ lang, code });
		}

		mermaidThemeObserver = new MutationObserver(() => {
			if (lang === 'mermaid') {
				render();
			}
		});

		mermaidThemeObserver.observe(document.documentElement, {
			attributeFilter: ['class'],
			attributes: true
		});

		await tick();
		if (codeWrapperEl) {
			heightObserver = new ResizeObserver(() => {
				if (codeWrapperEl) {
					codeNaturalHeight = codeWrapperEl.scrollHeight;
				}
			});
			heightObserver.observe(codeWrapperEl);
		}
	});

	onDestroy(() => {
		mermaidThemeObserver?.disconnect();
		heightObserver?.disconnect();
		if (pyodideWorker) {
			pyodideWorker.terminate();
		}
	});
</script>

<div>
	<div
		class="relative {className} group/codeblock flex flex-col rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-950 my-2 overflow-hidden"
		dir="ltr"
	>
		{#if lang === 'mermaid'}
			{#if mermaidHtml}
				<SvgPanZoom
					className=" border border-gray-100 dark:border-gray-850 rounded-lg max-h-fit overflow-hidden"
					svg={mermaidHtml}
					content={_token?.text ?? ''}
				/>
			{:else}
				<pre class="mermaid">{code}</pre>
			{/if}
		{:else}
			<div
				class="sticky {stickyButtonsClassName} left-0 right-0 z-10 flex items-center justify-between gap-2 px-3 py-1.5 min-h-[36px] bg-white dark:bg-gray-900 text-gray-500 dark:text-gray-400 border-b border-gray-200 dark:border-gray-800"
			>
				{#if needsCollapse}
					<button
						type="button"
						class="min-w-0 flex flex-1 items-center gap-2 rounded-md text-left cursor-pointer hover:text-gray-700 dark:hover:text-gray-200"
						on:click={collapseCodeBlock}
						aria-expanded={!sourceIsCollapsed}
						title={sourceIsCollapsed ? $i18n.t('Expand') : $i18n.t('Collapse')}
					>
						<span class="size-4 flex-shrink-0" aria-hidden="true">
							{@html langIcon.svg}
						</span>
						<span class="truncate text-[13px] font-medium text-gray-600 dark:text-gray-400">
							{langIcon.label}
						</span>
					</button>
				{:else}
					<div class="min-w-0 flex flex-1 items-center gap-2 rounded-md">
						<span class="size-4 flex-shrink-0" aria-hidden="true">
							{@html langIcon.svg}
						</span>
						<span class="truncate text-[13px] font-medium text-gray-600 dark:text-gray-400">
							{langIcon.label}
						</span>
					</div>
				{/if}
				<div class="flex shrink-0 items-center gap-1">
					{#if codeExecutionEnabled && (lang.toLowerCase() === 'python' || lang.toLowerCase() === 'py' || (lang === '' && checkPythonCode(code)))}
						{#if executing}
							<div
								class="inline-flex items-center text-gray-400 dark:text-gray-500 p-1.5 rounded-md"
								title={$i18n.t('Running')}
							>
								<LoaderCircle class="size-4 animate-spin" size={16} strokeWidth={2.1} />
							</div>
						{:else if run}
							<button
								type="button"
								class="inline-flex items-center text-gray-400 dark:text-gray-500 hover:text-green-600 dark:hover:text-green-400 p-1.5 rounded-md hover:bg-gray-100 dark:hover:bg-gray-800 transition"
								on:click={async () => {
									code = _code;
									await tick();
									executePython(code);
								}}
								title={$i18n.t('Run')}
							>
								<Play class="size-4" size={16} strokeWidth={2.1} />
							</button>
						{/if}
					{/if}

					{#if messageId && isIframePreviewable(lang)}
						<button
							type="button"
							class="inline-flex items-center text-gray-400 dark:text-gray-500 hover:text-sky-600 dark:hover:text-sky-400 p-1.5 rounded-md hover:bg-gray-100 dark:hover:bg-gray-800 transition"
							on:click={openIframePreview}
							title={$i18n.t('Open preview')}
						>
							<PanelRightOpen class="size-[17.5px]" size={17.5} strokeWidth={1.8} />
						</button>
					{/if}

					{#if isSvgPreviewable(lang, code)}
						<button
							type="button"
							class="inline-flex items-center text-gray-400 dark:text-gray-500 hover:text-sky-600 dark:hover:text-sky-400 p-1.5 rounded-md hover:bg-gray-100 dark:hover:bg-gray-800 transition"
							on:click={previewSvg}
							title={`${$i18n.t('Preview')} SVG`}
						>
							<PanelRightOpen class="size-[17.5px]" size={17.5} strokeWidth={1.8} />
						</button>
					{/if}

					{#if save}
						<button
							type="button"
							class="inline-flex items-center text-gray-400 dark:text-gray-500 hover:text-gray-700 dark:hover:text-white p-1.5 rounded-md hover:bg-gray-100 dark:hover:bg-gray-800 transition"
							on:click={saveCode}
							title={$i18n.t('Save')}
						>
							{#if saved}
								<Check class="size-[17.5px] text-green-500" size={17.5} strokeWidth={2.15} />
							{:else}
								<Save class="size-[17.5px]" size={17.5} strokeWidth={1.95} />
							{/if}
						</button>
					{/if}

					<button
						type="button"
						class="inline-flex items-center text-gray-400 dark:text-gray-500 hover:text-gray-700 dark:hover:text-white p-1.5 rounded-md hover:bg-gray-100 dark:hover:bg-gray-800 transition"
						on:click={copyCode}
						title={$i18n.t('Copy')}
					>
						{#if copied}
							<Check class="size-4 text-green-500" size={16} strokeWidth={2.3} />
						{:else}
							<Copy class="size-4" size={16} strokeWidth={2.1} />
						{/if}
					</button>

					{#if needsCollapse}
						<button
							type="button"
							class="inline-flex items-center text-gray-400 dark:text-gray-500 hover:text-gray-700 dark:hover:text-white p-1.5 rounded-md hover:bg-gray-100 dark:hover:bg-gray-800 transition"
							on:click={collapseCodeBlock}
							title={sourceIsCollapsed ? $i18n.t('Expand') : $i18n.t('Collapse')}
						>
							<ChevronsUpDown class="size-4" size={16} strokeWidth={2.1} />
						</button>
					{/if}
				</div>
			</div>

			{#if showPyodideConsent}
				<div
					class="border-b border-amber-200 bg-amber-50 px-3 py-3 text-sm text-amber-800 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-200"
				>
					<div class="font-medium">
						{$i18n.t('浏览器 Python 运行时未准备就绪', {
							defaultValue: 'Browser Python runtime is not ready'
						})}
					</div>
					<div class="mt-1 text-xs leading-relaxed">
						{getPyodideDownloadSummary(pyodideConsentPackages)}
					</div>
					<div class="mt-3 flex gap-2">
						<button
							class="rounded-lg bg-amber-600 px-3 py-1.5 text-xs font-medium text-white transition hover:bg-amber-700"
							on:click={async () => {
								approvePyodideConsent();
								showPyodideConsent = false;
								const nextCode = pendingPyodideCode || code;
								pendingPyodideCode = '';
								pyodideConsentPackages = [];
								executing = true;
								await executePythonAsWorker(nextCode);
							}}
							type="button"
						>
							{$i18n.t('下载并启用', { defaultValue: 'Download and Enable' })}
						</button>
						<button
							class="rounded-lg bg-white px-3 py-1.5 text-xs font-medium text-amber-700 transition hover:bg-amber-100 dark:bg-transparent dark:text-amber-200 dark:hover:bg-amber-900/30"
							on:click={() => {
								showPyodideConsent = false;
								pendingPyodideCode = '';
								pyodideConsentPackages = [];
								stderr = $i18n.t('已取消下载浏览器 Python 运行时。', {
									defaultValue: 'Browser Python runtime download was cancelled.'
								});
							}}
							type="button"
						>
							{$i18n.t('暂不', { defaultValue: 'Not now' })}
						</button>
					</div>
				</div>
			{/if}

			<div
				bind:this={codeWrapperEl}
				class="language-{lang} {editorClassName
					? editorClassName
					: executing || stdout || stderr || result
						? ''
						: ''} font-mono"
				style={sourceIsCollapsed ? `max-height: ${MAX_COLLAPSED_HEIGHT}px; overflow-y: auto;` : ''}
			>
				<CodeEditor value={code} {id} {lang} onSave={saveCode} onChange={updateEditorCode} />
			</div>

			{#if needsCollapse}
				<button
					type="button"
					class="w-full flex items-center justify-center gap-1.5 py-1.5 text-xs text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-900 border-t border-gray-200 dark:border-gray-800 transition-colors"
					on:click={collapseCodeBlock}
				>
					{#if collapsed}
						<ChevronDown class="size-3.5" size={14} strokeWidth={2.2} />
						{$i18n.t('Expand')} ({code.split('\n').length})
					{:else}
						<ChevronUp class="size-3.5" size={14} strokeWidth={2.2} />
						{$i18n.t('Collapse')}
					{/if}
				</button>
			{/if}

			<div
				id="plt-canvas-{id}"
				class="bg-gray-50 dark:bg-gray-900 text-gray-800 dark:text-gray-200 max-w-full overflow-x-auto scrollbar-hidden"
			></div>

			{#if hasExecutionFeedback}
				<div
					class="bg-gray-50 dark:bg-gray-900/50 text-gray-800 dark:text-gray-200 border-t border-gray-100 dark:border-gray-800/50 py-3 px-4 flex flex-col gap-3"
				>
					<div class="flex items-center justify-between gap-3">
						<div
							class="inline-flex items-center gap-2 rounded-full bg-white dark:bg-gray-950 px-2.5 py-1 text-xs font-medium text-gray-600 dark:text-gray-300 border border-gray-100 dark:border-gray-800"
						>
							{#if executing}
								<LoaderCircle
									class="size-3.5 animate-spin text-gray-400"
									size={14}
									strokeWidth={2.2}
								/>
							{:else if stderr}
								<span class="size-1.5 rounded-full bg-rose-500"></span>
							{:else}
								<span class="size-1.5 rounded-full bg-emerald-500"></span>
							{/if}
							{$i18n.t('Output')}
						</div>

						{#if sourceIsCollapsed}
							<button
								type="button"
								class="shrink-0 rounded-md px-2 py-1 text-xs font-medium text-gray-500 hover:text-gray-700 hover:bg-white dark:text-gray-400 dark:hover:text-gray-200 dark:hover:bg-gray-950 border border-transparent hover:border-gray-100 dark:hover:border-gray-800 transition-colors"
								on:click={collapseCodeBlock}
							>
								{$i18n.t('Expand')} ({code.split('\n').length})
							</button>
						{/if}
					</div>

					{#if executing}
						<div class="text-sm flex items-center gap-2 text-gray-500 dark:text-gray-400">
							<LoaderCircle class="size-3.5 animate-spin" size={14} strokeWidth={2.2} />
							{$i18n.t('Running...')}
						</div>
					{:else}
						{#if hasOutputText}
							<div class="flex flex-col gap-2">
								{#if hasStdout}
									<div
										class="text-sm leading-6 font-mono whitespace-pre-wrap bg-white dark:bg-gray-950 rounded-lg p-3 border border-gray-100 dark:border-gray-800 {`${stdout}`.split(
											'\n'
										).length > 100
											? `max-h-96`
											: ''} overflow-y-auto"
										style="font-family: 'JetBrains Mono', 'Fira Code', Consolas, monospace;"
									>
										{stdout}
									</div>
								{/if}

								{#if hasStderr}
									<div
										class="text-sm leading-6 font-mono whitespace-pre-wrap bg-white dark:bg-gray-950 rounded-lg p-3 border border-rose-100 text-rose-700 dark:border-rose-900/60 dark:text-rose-200 {`${stderr}`.split(
											'\n'
										).length > 100
											? `max-h-96`
											: ''} overflow-y-auto"
										style="font-family: 'JetBrains Mono', 'Fira Code', Consolas, monospace;"
									>
										{stderr}
									</div>
								{/if}
							</div>
						{/if}

						{#if hasResult || hasFiles}
							<div class="flex flex-col gap-2">
								<div
									class="inline-flex w-fit items-center gap-2 rounded-full bg-white dark:bg-gray-950 px-2.5 py-1 text-xs font-medium text-gray-600 dark:text-gray-300 border border-gray-100 dark:border-gray-800"
								>
									{$i18n.t('Result')}
								</div>

								{#if hasResult}
									<div
										class="text-sm leading-6 font-mono whitespace-pre-wrap bg-white dark:bg-gray-950 rounded-lg p-3 border border-gray-100 dark:border-gray-800 overflow-x-auto"
										style="font-family: 'JetBrains Mono', 'Fira Code', Consolas, monospace;"
									>
										{formatResult(result)}
									</div>
								{/if}

								{#if hasFiles}
									<div class="flex flex-col gap-2">
										{#each files as file}
											{#if file.type.startsWith('image')}
												<img
													src={file.data}
													alt="Output"
													class="w-full max-w-[36rem] rounded-lg border border-gray-100 dark:border-gray-800 bg-white dark:bg-gray-950"
												/>
											{/if}
										{/each}
									</div>
								{/if}
							</div>
						{/if}
					{/if}
				</div>
			{/if}
		{/if}
	</div>
</div>
