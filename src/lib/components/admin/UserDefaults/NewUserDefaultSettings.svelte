<script lang="ts">
	import { getContext, onMount, tick } from 'svelte';
	import type { ComponentType } from 'svelte';
	import type { Writable } from 'svelte/store';
	import { toast } from 'svelte-sonner';
	import { MessageCircleMore, PanelTop, UserCog } from 'lucide-svelte';

	import { models, settings, user } from '$lib/stores';
	import {
		getNewUserDefaultSettings,
		getUserSettings,
		updateNewUserDefaultSettings,
		type NewUserDefaultSettingsPayload
	} from '$lib/apis/users';
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
		type UserDefaultUiBoolKey
	} from '$lib/utils/user-default-settings';

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
	type SectionKey = 'chat' | 'interface';

	let loading = true;
	let saving = false;
	let loadError = '';
	let draft: Draft = normalizeNewUserDefaultSettings(createEmptyNewUserDefaultSettings());
	let initialPayload: NewUserDefaultSettingsPayload = createEmptyNewUserDefaultSettings();

	let openSections = {
		chat: true,
		interface: true
	};
	let activeSection: SectionKey = 'chat';

	$: payload = buildNewUserDefaultSettingsPayload(draft);
	$: dirty = !isSettingsSnapshotEqual(payload, initialPayload);
	const sectionOrder: SectionKey[] = ['chat', 'interface'];
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
				'控制输入体验、快捷键和默认系统提示词。',
				'Control input behavior, shortcuts, and the default system prompt.'
			),
			badgeColor: 'bg-emerald-50 dark:bg-emerald-950/30',
			iconColor: 'text-emerald-500 dark:text-emerald-400',
			icon: PanelTop
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
		Object.keys(value.ui ?? {}).length > 0;

	const createCurrentAdminPreferenceDraft = async () => {
		const userSettings = await getUserSettings(localStorage.token).catch(() => null);
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
			}
		};

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
		boolRow(tr('展开详情', 'Expand details'), 'expandDetails'),
		boolRow(tr('插入建议提示词', 'Insert suggestion prompt'), 'insertSuggestionPrompt'),
		boolRow(tr('保留追问提示', 'Keep follow-up prompts'), 'keepFollowUpPrompts'),
		boolRow(tr('插入追问提示', 'Insert follow-up prompt'), 'insertFollowUpPrompt'),
		boolRow(
			tr('多模型回复用标签页显示', 'Display multi-model responses in tabs'),
			'displayMultiModelResponsesInTabs'
		),
		boolRow(tr('显示选中文本浮动按钮', 'Show floating action buttons'), 'showFloatingActionButtons')
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
		</div>
	</div>
{/if}
