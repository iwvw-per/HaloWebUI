<script lang="ts">
	import { getContext, onMount, tick } from 'svelte';
	import type { ComponentType } from 'svelte';
	import type { Writable } from 'svelte/store';
	import { toast } from 'svelte-sonner';
	import { MessageCircleMore, PanelTop, UserCog, Wrench } from 'lucide-svelte';

	import { models, settings, user } from '$lib/stores';
	import {
		getNewUserDefaultSettings,
		getUserSettings,
		updateNewUserDefaultSettings,
		type NewUserDefaultSettingsPayload
	} from '$lib/apis/users';
	import { getNativeToolsConfig } from '$lib/apis/configs';
	import { ensureModels } from '$lib/services/models';
	import { getModelChatDisplayName } from '$lib/utils/model-display';
	import { getModelSelectionId } from '$lib/utils/model-identity';
	import { translateWithDefault } from '$lib/i18n';
	import { cloneSettingsSnapshot, isSettingsSnapshotEqual } from '$lib/utils/settings-dirty';
	import {
		buildNewUserDefaultSettingsPayload,
		createEmptyNewUserDefaultSettings,
		normalizeNewUserDefaultSettings,
		pickUserDefaultUiFields,
		type NativeToolBoolKey,
		type UserDefaultUiBoolKey
	} from '$lib/utils/user-default-settings';
	import { LOBE_HIGHLIGHTER_THEMES, LOBE_MERMAID_THEMES } from '$lib/utils/lobehub-chat-appearance';

	import HaloSelect from '$lib/components/common/HaloSelect.svelte';
	import InlineDirtyActions from '$lib/components/admin/Settings/InlineDirtyActions.svelte';
	import Switch from '$lib/components/common/Switch.svelte';
	import Spinner from '$lib/components/common/Spinner.svelte';
	import PreferenceSection from './PreferenceSection.svelte';

	const i18n: Writable<any> = getContext('i18n');
	const tr = (key: string, defaultValue: string, options: Record<string, any> = {}) =>
		translateWithDefault($i18n, key, defaultValue, options);

	type Draft = ReturnType<typeof normalizeNewUserDefaultSettings>;
	type BoolRow<Key extends string> = {
		label: string;
		key: Key;
		description: string;
	};
	type SectionKey = 'chat' | 'interface' | 'tools';

	let loading = true;
	let saving = false;
	let loadError = '';
	let draft: Draft = normalizeNewUserDefaultSettings(createEmptyNewUserDefaultSettings());
	let initialPayload: NewUserDefaultSettingsPayload = createEmptyNewUserDefaultSettings();

	let openSections = {
		chat: true,
		interface: true,
		tools: false
	};
	let activeSection: SectionKey = 'chat';

	$: payload = buildNewUserDefaultSettingsPayload(draft);
	$: dirty = !isSettingsSnapshotEqual(payload, initialPayload);
	const sectionOrder: SectionKey[] = ['chat', 'interface', 'tools'];
	const pageMeta = {
		title: tr('账户预设', 'Account Presets'),
		description: tr(
			'首次配置会带入当前管理员偏好；保存后仅影响新账号。',
			'The first draft uses the current admin preferences. Saved presets only affect new accounts.'
		),
		badgeColor: 'bg-blue-50 dark:bg-blue-950/30',
		iconColor: 'text-blue-500 dark:text-blue-400',
		icon: UserCog
	};
	const sectionMeta: Record<
		SectionKey,
		{
			title: string;
			description: string;
			badgeColor: string;
			iconColor: string;
			icon: ComponentType;
		}
	> = {
		chat: {
			title: tr('聊天行为', 'Chat Behavior'),
			description: tr(
				'设置默认模型、新聊天、标题、追问、引用和折叠行为。',
				'Set the default model, new chat, title, follow-up, citation, and collapse behavior.'
			),
			badgeColor: 'bg-indigo-50 dark:bg-indigo-950/30',
			iconColor: 'text-indigo-500 dark:text-indigo-400',
			icon: MessageCircleMore
		},
		interface: {
			title: tr('界面与输入', 'Interface and Input'),
			description: tr(
				'控制界面样式、输入体验、快捷键和默认系统提示词。',
				'Control appearance, input behavior, shortcuts, and the default system prompt.'
			),
			badgeColor: 'bg-emerald-50 dark:bg-emerald-950/30',
			iconColor: 'text-emerald-500 dark:text-emerald-400',
			icon: PanelTop
		},
		tools: {
			title: tr('内置工具', 'Built-in Tools'),
			description: tr(
				'配置新用户的联网、知识库、图片、记忆和终端工具偏好。',
				'Configure web, knowledge, image, memory, and terminal tool preferences for new users.'
			),
			badgeColor: 'bg-amber-50 dark:bg-amber-950/30',
			iconColor: 'text-amber-500 dark:text-amber-400',
			icon: Wrench
		}
	};

	const modelOptions = () => [
		{ value: '', label: tr('无默认模型', 'No default model') },
		...($models ?? []).map((model) => ({
			value: getModelSelectionId(model),
			label: getModelChatDisplayName(model)
		}))
	];

	const getDefaultModel = () => draft.ui.models?.[0] ?? '';
	const setDefaultModel = (value: string) => {
		draft.ui.models = value ? [value] : [];
		draft = draft;
	};

	const setUiBool = (key: UserDefaultUiBoolKey, value: boolean) => {
		draft.ui[key] = value;
		draft = draft;
	};

	const setNativeToolBool = (key: NativeToolBoolKey, value: boolean) => {
		draft.tools.native_tools[key] = value;
		draft = draft;
	};

	const openAndScrollToSection = async (section: SectionKey) => {
		activeSection = section;
		openSections = { ...openSections, [section]: true };
		await tick();
		document.getElementById(`new-user-defaults-${section}`)?.scrollIntoView({
			behavior: 'smooth',
			block: 'start'
		});
	};

	const hasTemplateContent = (value: NewUserDefaultSettingsPayload) =>
		Object.keys(value.ui ?? {}).length > 0 ||
		Object.keys(value.tools?.native_tools ?? {}).length > 0;

	const createCurrentAdminPreferenceDraft = async () => {
		const [userSettings, native] = await Promise.all([
			getUserSettings(localStorage.token).catch(() => null),
			getNativeToolsConfig(localStorage.token).catch(() => null)
		]);
		const copied = cloneSettingsSnapshot(
			pickUserDefaultUiFields(userSettings?.ui ?? $settings ?? {})
		);
		const normalized = normalizeNewUserDefaultSettings(createEmptyNewUserDefaultSettings());
		normalized.ui = {
			...normalized.ui,
			...copied,
			title: {
				...normalized.ui.title,
				...(copied.title ?? {})
			},
			imageCompressionSize: {
				...normalized.ui.imageCompressionSize,
				...(copied.imageCompressionSize ?? {})
			}
		};
		if (native) {
			normalized.tools.native_tools = {
				...normalized.tools.native_tools,
				...cloneSettingsSnapshot(native)
			};
		}

		normalized.roles = ['user', 'pending'];
		return normalizeNewUserDefaultSettings(normalized);
	};

	const syncInitial = (value: NewUserDefaultSettingsPayload) => {
		const normalized = normalizeNewUserDefaultSettings(value);
		const initial = buildNewUserDefaultSettingsPayload(normalized);
		const enabled = hasTemplateContent(initial);
		normalized.enabled = enabled;
		normalized.roles = ['user', 'pending'];
		draft = normalized;
		initialPayload = {
			...initial,
			enabled,
			roles: ['user', 'pending']
		};
	};

	const load = async () => {
		loading = true;
		loadError = '';
		try {
			const [data] = await Promise.all([
				getNewUserDefaultSettings(localStorage.token),
				ensureModels(localStorage.token, { reason: 'new-user-default-settings' }).catch(() => {})
			]);
			syncInitial(data);
			if (
				!data.configured &&
				!hasTemplateContent(
					buildNewUserDefaultSettingsPayload(normalizeNewUserDefaultSettings(data))
				)
			) {
				draft = await createCurrentAdminPreferenceDraft();
			}
		} catch (error) {
			loadError = String(error);
			toast.error(tr('加载账户预设失败。', 'Failed to load account presets.'));
		} finally {
			loading = false;
		}
	};

	const save = async () => {
		if (saving) return;
		saving = true;
		try {
			const saved = await updateNewUserDefaultSettings(localStorage.token, {
				...payload,
				enabled: hasTemplateContent(payload),
				roles: ['user', 'pending']
			});
			syncInitial(saved);
			toast.success(tr('账户预设已保存。', 'Account presets saved.'));
		} catch (error) {
			toast.error(String(error));
		} finally {
			saving = false;
		}
	};

	const reset = () => {
		const normalized = normalizeNewUserDefaultSettings(initialPayload);
		normalized.roles = ['user', 'pending'];
		draft = normalized;
	};

	const clearTemplate = () => {
		draft = normalizeNewUserDefaultSettings({
			...createEmptyNewUserDefaultSettings(),
			configured: true
		});
	};

	const boolRow = <Key extends string>(
		label: string,
		key: Key,
		description = ''
	): BoolRow<Key> => ({
		label,
		key,
		description
	});

	const interfaceRows: BoolRow<UserDefaultUiBoolKey>[] = [
		boolRow(
			tr('首页显示精选助手', 'Show featured assistants on home page'),
			'showFeaturedAssistantsOnHome'
		),
		boolRow(tr('浏览器标签页显示聊天标题', 'Display chat title in tab'), 'showChatTitleInTab'),
		boolRow(tr('聊天气泡界面', 'Chat Bubble UI'), 'chatBubble'),
		boolRow(tr('显示用户名', 'Display username'), 'showUsername'),
		boolRow(tr('宽屏模式', 'Widescreen Mode'), 'widescreenMode'),
		boolRow(tr('通知声音', 'Notification Sound'), 'notificationSound'),
		boolRow(tr('流式输出自动滚动', 'Auto-scroll during streaming'), 'enableAutoScrollOnStreaming'),
		boolRow(tr('富文本输入', 'Rich Text Input'), 'richTextInput'),
		boolRow(tr('提示词自动补全', 'Prompt Autocomplete'), 'promptAutocomplete'),
		boolRow(tr('格式工具栏', 'Formatting Toolbar'), 'showFormattingToolbar'),
		boolRow(tr('插入提示词为富文本', 'Insert prompt as rich text'), 'insertPromptAsRichText'),
		boolRow(tr('大段文本自动转文件', 'Large text as file'), 'largeTextAsFile'),
		boolRow(tr('复制时保留格式', 'Copy formatted'), 'copyFormatted'),
		boolRow(tr('Ctrl+Enter 发送', 'Ctrl+Enter to send'), 'ctrlEnterToSend')
	];

	const chatRows: (BoolRow<UserDefaultUiBoolKey> | BoolRow<'title.auto'>)[] = [
		boolRow(tr('自动生成标题', 'Auto-generate title'), 'title.auto' as const),
		boolRow(tr('自动生成标签', 'Auto-generate tags'), 'autoTags'),
		boolRow(tr('自动生成追问', 'Auto-generate follow-ups'), 'autoFollowUps'),
		boolRow(tr('检测 Artifacts', 'Detect artifacts'), 'detectArtifacts'),
		boolRow(tr('SVG 预览自动打开', 'Auto-open SVG preview'), 'svgPreviewAutoOpen'),
		boolRow(tr('自动复制回复', 'Auto-copy response'), 'responseAutoCopy'),
		boolRow(tr('分支切换时滚动', 'Scroll on branch change'), 'scrollOnBranchChange'),
		boolRow(tr('消息队列', 'Message queue'), 'enableMessageQueue'),
		boolRow(tr('默认临时聊天', 'Temporary chat by default'), 'temporaryChatByDefault'),
		boolRow(
			tr('新聊天继承上次状态', 'New chat inherits previous state'),
			'newChatInheritsPreviousState'
		),
		boolRow(tr('折叠代码块', 'Collapse code blocks'), 'collapseCodeBlocks'),
		boolRow(
			tr('折叠历史长回复', 'Collapse historical long responses'),
			'collapseHistoricalLongResponses'
		),
		boolRow(tr('显示引用', 'Show inline citations'), 'showInlineCitations'),
		boolRow(tr('显示消息大纲', 'Show message outline'), 'showMessageOutline'),
		boolRow(tr('公式快速复制', 'Formula quick copy'), 'showFormulaQuickCopyButton'),
		boolRow(tr('展开详情', 'Expand details'), 'expandDetails'),
		boolRow(tr('插入建议提示词', 'Insert suggestion prompt'), 'insertSuggestionPrompt'),
		boolRow(tr('保留追问提示', 'Keep follow-up prompts'), 'keepFollowUpPrompts'),
		boolRow(tr('插入追问提示', 'Insert follow-up prompt'), 'insertFollowUpPrompt'),
		boolRow(tr('重新生成菜单', 'Regenerate menu'), 'regenerateMenu'),
		boolRow(tr('预览中渲染 Markdown', 'Render Markdown in previews'), 'renderMarkdownInPreviews'),
		boolRow(
			tr('多模型回复用标签页显示', 'Display multi-model responses in tabs'),
			'displayMultiModelResponsesInTabs'
		),
		boolRow(tr('样式化 PDF 导出', 'Stylized PDF export'), 'stylizedPdfExport'),
		boolRow(
			tr('显示选中文本浮动按钮', 'Show floating action buttons'),
			'showFloatingActionButtons'
		),
		boolRow(tr('记忆', 'Memory'), 'memory'),
		boolRow(tr('图片压缩', 'Image compression'), 'imageCompression'),
		boolRow(tr('频道内图片也压缩', 'Compress images in channels'), 'imageCompressionInChannels')
	];

	const toolRows: BoolRow<NativeToolBoolKey>[] = [
		boolRow(tr('交错思考', 'Interleaved thinking'), 'ENABLE_INTERLEAVED_THINKING'),
		boolRow(tr('联网搜索工具', 'Web search tool'), 'ENABLE_WEB_SEARCH_TOOL'),
		boolRow(tr('URL 抓取', 'URL fetch'), 'ENABLE_URL_FETCH'),
		boolRow(tr('渲染后 URL 抓取', 'Rendered URL fetch'), 'ENABLE_URL_FETCH_RENDERED'),
		boolRow(tr('列出知识库', 'List knowledge bases'), 'ENABLE_LIST_KNOWLEDGE_BASES'),
		boolRow(tr('搜索知识库', 'Search knowledge bases'), 'ENABLE_SEARCH_KNOWLEDGE_BASES'),
		boolRow(tr('查询知识文件', 'Query knowledge files'), 'ENABLE_QUERY_KNOWLEDGE_FILES'),
		boolRow(tr('查看知识文件', 'View knowledge file'), 'ENABLE_VIEW_KNOWLEDGE_FILE'),
		boolRow(tr('图片生成工具', 'Image generation tool'), 'ENABLE_IMAGE_GENERATION_TOOL'),
		boolRow(tr('图片编辑工具', 'Image edit tool'), 'ENABLE_IMAGE_EDIT'),
		boolRow(tr('记忆工具', 'Memory tools'), 'ENABLE_MEMORY_TOOLS'),
		boolRow(tr('笔记工具', 'Notes'), 'ENABLE_NOTES'),
		boolRow(tr('聊天历史工具', 'Chat history tools'), 'ENABLE_CHAT_HISTORY_TOOLS'),
		boolRow(tr('时间工具', 'Time tools'), 'ENABLE_TIME_TOOLS'),
		boolRow(tr('频道工具', 'Channel tools'), 'ENABLE_CHANNEL_TOOLS'),
		boolRow(tr('终端工具', 'Terminal tool'), 'ENABLE_TERMINAL_TOOL')
	];

	onMount(load);
</script>

<svelte:head>
	<title>{pageMeta.title}</title>
</svelte:head>

{#if $user?.role !== 'admin'}
	<div class="text-sm text-gray-500 dark:text-gray-400">
		{tr('只有管理员可以管理账户预设。', 'Only admins can manage account presets.')}
	</div>
{:else if loading}
	<div class="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
		<Spinner className="size-4" />
		<span>{tr('正在加载账户预设...', 'Loading account presets...')}</span>
	</div>
{:else if loadError}
	<div class="text-sm text-red-600 dark:text-red-400">{loadError}</div>
{:else}
	<div class="h-full space-y-6 overflow-y-auto scrollbar-hidden">
		<div class="mx-auto max-w-6xl space-y-6 pb-8">
			<section class="glass-section p-5 space-y-5">
				<div class="@container flex flex-col gap-5">
					<div
						class="flex flex-col gap-4 @[64rem]:flex-row @[64rem]:items-start @[64rem]:justify-between"
					>
						<div class="min-w-0 @[64rem]:flex-1">
							<div
								class="inline-flex h-8 items-center gap-2 whitespace-nowrap rounded-full border border-gray-200/80 bg-white/80 px-3.5 text-xs font-medium leading-none text-gray-600 dark:border-gray-700/80 dark:bg-gray-900/70 dark:text-gray-300"
							>
								<span class="leading-none text-gray-400 dark:text-gray-500">
									{$i18n.t('Settings')}
								</span>
								<span class="leading-none text-gray-300 dark:text-gray-600">/</span>
								<span class="leading-none text-gray-900 dark:text-white">{pageMeta.title}</span>
							</div>

							<div class="mt-3 flex items-start gap-3">
								<div class="glass-icon-badge {pageMeta.badgeColor}">
									<svelte:component
										this={pageMeta.icon}
										class="shrink-0 {pageMeta.iconColor}"
										size={18}
										strokeWidth={1.75}
									/>
								</div>
								<div class="min-w-0">
									<div class="flex flex-wrap items-center gap-2.5">
										<div class="text-base font-semibold text-gray-800 dark:text-gray-100">
											{pageMeta.title}
										</div>
										<button
											type="button"
											class="inline-flex h-9 items-center whitespace-nowrap rounded-xl border border-red-200 bg-white px-3.5 text-xs font-medium leading-none text-red-600 transition-all hover:border-red-300 hover:bg-red-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-red-900/50 dark:bg-gray-900 dark:text-red-400 dark:hover:bg-red-950/30"
											disabled={saving}
											on:click={clearTemplate}
										>
											{tr('清空预设', 'Clear Preset')}
										</button>
										<InlineDirtyActions
											{dirty}
											{saving}
											disabled={saving}
											saveAsSubmit={false}
											align="start"
											on:reset={reset}
											on:save={save}
										/>
									</div>
									<p class="mt-1 text-xs text-gray-400 dark:text-gray-500">
										{pageMeta.description}
									</p>
								</div>
							</div>
						</div>

						<div
							class="inline-flex max-w-full flex-wrap items-center gap-2 self-start rounded-2xl bg-gray-100 p-1 dark:bg-gray-850 @[64rem]:ml-auto @[64rem]:mt-11 @[64rem]:flex-nowrap @[64rem]:justify-end @[64rem]:shrink-0"
						>
							{#each sectionOrder as section}
								<button
									type="button"
									class={`flex min-w-0 items-center justify-start gap-2 whitespace-nowrap rounded-xl px-4 py-2 text-sm font-medium transition-all ${activeSection === section ? 'bg-white text-gray-900 shadow-sm dark:bg-gray-800 dark:text-white' : 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'}`}
									on:click={() => openAndScrollToSection(section)}
								>
									<svelte:component
										this={sectionMeta[section].icon}
										class="shrink-0"
										size={16}
										strokeWidth={1.75}
									/>
									<span>{sectionMeta[section].title}</span>
								</button>
							{/each}
						</div>
					</div>
				</div>
			</section>

			<div id="new-user-defaults-chat" class="scroll-mt-4">
				<PreferenceSection
					bind:open={openSections.chat}
					title={sectionMeta.chat.title}
					description={sectionMeta.chat.description}
					badgeColor={sectionMeta.chat.badgeColor}
					iconColor={sectionMeta.chat.iconColor}
					icon={sectionMeta.chat.icon}
					on:toggle={() => (activeSection = 'chat')}
				>
					<div class="space-y-3 pt-1">
						<div
							class="glass-item flex flex-col gap-3 px-4 py-3 sm:flex-row sm:items-center sm:justify-between"
						>
							<div>
								<div class="text-sm font-medium">{tr('默认模型', 'Default model')}</div>
								<div class="text-xs text-gray-500 dark:text-gray-400">
									{tr(
										'新用户第一次打开聊天时默认选中的模型。',
										'The model selected when a new user first opens chat.'
									)}
								</div>
							</div>
							<HaloSelect
								value={getDefaultModel()}
								options={modelOptions()}
								searchEnabled={true}
								className="w-full sm:w-72"
								on:change={(event) => setDefaultModel(event.detail.value)}
							/>
						</div>
						<div class="grid gap-2 md:grid-cols-2">
							{#each chatRows as row}
								<div class="glass-item flex items-center justify-between gap-3 px-4 py-3">
									<div class="min-w-0 text-sm font-medium">{row.label}</div>
									{#if row.key === 'title.auto'}
										<Switch bind:state={draft.ui.title.auto} />
									{:else}
										<Switch
											state={draft.ui[row.key]}
											on:change={(event) => setUiBool(row.key, event.detail)}
										/>
									{/if}
								</div>
							{/each}
						</div>
						<div class="glass-item grid gap-3 px-4 py-3 md:grid-cols-2">
							<label class="space-y-1">
								<div class="text-sm font-medium">{tr('压缩宽度', 'Compression width')}</div>
								<input
									class="w-full rounded-lg border border-gray-200 bg-transparent px-3 py-2 text-sm outline-hidden dark:border-gray-700"
									bind:value={draft.ui.imageCompressionSize.width}
									placeholder="1920"
								/>
							</label>
							<label class="space-y-1">
								<div class="text-sm font-medium">{tr('压缩高度', 'Compression height')}</div>
								<input
									class="w-full rounded-lg border border-gray-200 bg-transparent px-3 py-2 text-sm outline-hidden dark:border-gray-700"
									bind:value={draft.ui.imageCompressionSize.height}
									placeholder="1080"
								/>
							</label>
						</div>
					</div>
				</PreferenceSection>
			</div>

			<div id="new-user-defaults-interface" class="scroll-mt-4">
				<PreferenceSection
					bind:open={openSections.interface}
					title={sectionMeta.interface.title}
					description={sectionMeta.interface.description}
					badgeColor={sectionMeta.interface.badgeColor}
					iconColor={sectionMeta.interface.iconColor}
					icon={sectionMeta.interface.icon}
					on:toggle={() => (activeSection = 'interface')}
				>
					<div class="space-y-3 pt-1">
						<div class="grid gap-3 lg:grid-cols-2">
							<div class="glass-item space-y-1 px-4 py-3">
								<div class="text-sm font-medium">{tr('代码高亮主题', 'Code highlight theme')}</div>
								<HaloSelect
									bind:value={draft.ui.highlighterTheme}
									searchEnabled={true}
									className="w-full"
									options={LOBE_HIGHLIGHTER_THEMES.map((item) => ({
										value: item.id,
										label: item.displayName
									}))}
								/>
							</div>
							<div class="glass-item space-y-1 px-4 py-3">
								<div class="text-sm font-medium">{tr('Mermaid 主题', 'Mermaid theme')}</div>
								<HaloSelect
									bind:value={draft.ui.mermaidTheme}
									className="w-full"
									options={LOBE_MERMAID_THEMES.map((item) => ({
										value: item.id,
										label: item.displayName
									}))}
								/>
							</div>
							<div class="glass-item space-y-1 px-4 py-3">
								<div class="text-sm font-medium">{tr('聊天方向', 'Chat direction')}</div>
								<HaloSelect
									bind:value={draft.ui.chatDirection}
									className="w-full"
									options={[
										{ value: 'auto', label: tr('自动', 'Auto') },
										{ value: 'LTR', label: 'LTR' },
										{ value: 'RTL', label: 'RTL' }
									]}
								/>
							</div>
							<div class="glass-item space-y-1 px-4 py-3">
								<div class="text-sm font-medium">{tr('过渡动画', 'Transition animation')}</div>
								<HaloSelect
									bind:value={draft.ui.transitionMode}
									className="w-full"
									options={[
										{ value: 'fadeIn', label: tr('淡入', 'Fade in') },
										{ value: 'smooth', label: tr('平滑', 'Smooth') },
										{ value: 'none', label: tr('无', 'None') }
									]}
								/>
							</div>
							<label class="glass-item space-y-2 px-4 py-3 lg:col-span-2">
								<div class="flex items-center justify-between gap-3">
									<div class="text-sm font-medium">{tr('文字缩放', 'UI scale')}</div>
									<button
										type="button"
										class="rounded-md border border-gray-200 px-2 py-1 text-xs dark:border-gray-700"
										on:click={() => {
											draft.ui.textScale = draft.ui.textScale === null ? 1 : null;
											draft = draft;
										}}
									>
										{draft.ui.textScale === null ? tr('使用默认', 'Default') : tr('清除', 'Clear')}
									</button>
								</div>
								{#if draft.ui.textScale !== null}
									<div class="flex items-center gap-3">
										<input
											type="range"
											min="0.8"
											max="1.5"
											step="0.01"
											class="flex-1 accent-blue-500"
											bind:value={draft.ui.textScale}
										/>
										<span class="w-12 text-right text-xs tabular-nums text-gray-500">
											{Math.round((draft.ui.textScale ?? 1) * 100)}%
										</span>
									</div>
								{/if}
							</label>
						</div>

						<div class="grid gap-2 md:grid-cols-2">
							{#each interfaceRows as row}
								<div class="glass-item flex items-center justify-between gap-3 px-4 py-3">
									<div class="min-w-0 text-sm font-medium">{row.label}</div>
									<Switch
										state={draft.ui[row.key]}
										on:change={(event) => setUiBool(row.key, event.detail)}
									/>
								</div>
							{/each}
						</div>

						<label class="glass-item block space-y-2 px-4 py-3">
							<div class="text-sm font-medium">{tr('默认系统提示词', 'Default system prompt')}</div>
							<textarea
								class="min-h-28 w-full rounded-lg border border-gray-200 bg-transparent px-3 py-2 text-sm outline-hidden dark:border-gray-700"
								bind:value={draft.ui.system}
							/>
						</label>
					</div>
				</PreferenceSection>
			</div>

			<div id="new-user-defaults-tools" class="scroll-mt-4">
				<PreferenceSection
					bind:open={openSections.tools}
					title={sectionMeta.tools.title}
					description={sectionMeta.tools.description}
					badgeColor={sectionMeta.tools.badgeColor}
					iconColor={sectionMeta.tools.iconColor}
					icon={sectionMeta.tools.icon}
					on:toggle={() => (activeSection = 'tools')}
				>
					<div class="space-y-3 pt-1">
						<div class="grid gap-3 md:grid-cols-2">
							<div class="glass-item space-y-1 px-4 py-3">
								<div class="text-sm font-medium">{tr('工具调用模式', 'Tool calling mode')}</div>
								<HaloSelect
									bind:value={draft.tools.native_tools.TOOL_CALLING_MODE}
									className="w-full"
									options={[
										{ value: 'default', label: tr('兼容', 'Compatibility') },
										{ value: 'native', label: tr('原生', 'Native') },
										{ value: 'off', label: tr('关闭', 'Off') }
									]}
								/>
							</div>
							<label class="glass-item space-y-1 px-4 py-3">
								<div class="text-sm font-medium">
									{tr('最大工具调用轮数', 'Max tool call rounds')}
								</div>
								<input
									type="number"
									min="1"
									max="30"
									step="1"
									class="w-full rounded-lg border border-gray-200 bg-transparent px-3 py-2 text-sm outline-hidden dark:border-gray-700"
									bind:value={draft.tools.native_tools.MAX_TOOL_CALL_ROUNDS}
								/>
							</label>
						</div>
						<div class="grid gap-2 md:grid-cols-2">
							{#each toolRows as row}
								<div class="glass-item flex items-center justify-between gap-3 px-4 py-3">
									<div class="min-w-0 text-sm font-medium">{row.label}</div>
									<Switch
										state={draft.tools.native_tools[row.key]}
										on:change={(event) => setNativeToolBool(row.key, event.detail)}
									/>
								</div>
							{/each}
						</div>
						<div class="text-xs text-gray-500 dark:text-gray-400">
							{tr(
								'这些只是新账号的默认工具偏好，不能绕过全局功能开关或权限组限制。',
								'These are only default tool preferences for new accounts and cannot bypass global feature switches or permissions.'
							)}
						</div>
					</div>
				</PreferenceSection>
			</div>
		</div>
	</div>
{/if}
