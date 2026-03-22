<script lang="ts">
	import { onMount, tick, getContext } from 'svelte';

	import Textarea from '$lib/components/common/Textarea.svelte';
	import { toast } from 'svelte-sonner';
	import Tooltip from '$lib/components/common/Tooltip.svelte';
	import InlineDirtyActions from '$lib/components/admin/Settings/InlineDirtyActions.svelte';
	import { cloneSettingsSnapshot, isSettingsSnapshotEqual } from '$lib/utils/settings-dirty';
	import AccessControl from '../common/AccessControl.svelte';
	import LockClosed from '$lib/components/icons/LockClosed.svelte';
	import AccessControlModal from '../common/AccessControlModal.svelte';
	import { user } from '$lib/stores';

	export let onSubmit: Function;
	export let edit = false;
	export let prompt = null;

	const i18n = getContext('i18n');

	let loading = false;

	let name = '';
	let command = '';
	let content = '';
	let tags: string[] = [];
	let tagInput = '';

	const normalizeAccessControl = (value) =>
		value === null
			? null
			: {
					read: {
						group_ids: value?.read?.group_ids ?? [],
						user_ids: value?.read?.user_ids ?? []
					},
					write: {
						group_ids: value?.write?.group_ids ?? [],
						user_ids: value?.write?.user_ids ?? []
					}
				};

	let accessControl = normalizeAccessControl({});
	let loaded = false;
	let initialSnapshot = null;
	let snapshot = null;
	let dirty = false;

	let showAccessControlModal = false;

	$: if (!edit) {
		command = name !== '' ? `${name.replace(/\s+/g, '-').toLowerCase()}` : '';
	}

	const buildSnapshot = () => ({
		name,
		command,
		content,
		tags: cloneSettingsSnapshot(tags),
		accessControl: cloneSettingsSnapshot(accessControl)
	});

	$: {
		name;
		command;
		content;
		tags;
		accessControl;
		snapshot = buildSnapshot();
	}
	$: dirty =
		loaded &&
		!!snapshot &&
		!!initialSnapshot &&
		!isSettingsSnapshotEqual(snapshot, initialSnapshot);

	const submitHandler = async () => {
		loading = true;

		if (validateCommandString(command)) {
			await onSubmit({
				name,
				command,
				content,
				tags: tags.length > 0 ? tags : null,
				access_control: accessControl
			});
		} else {
			toast.error(
				$i18n.t('Only alphanumeric characters and hyphens are allowed in the command string.')
			);
		}

		loading = false;
	};

	const validateCommandString = (inputString) => {
		const regex = /^[a-zA-Z0-9-]+$/;
		return regex.test(inputString);
	};

	const addTag = () => {
		const val = tagInput.trim();
		if (val && !tags.includes(val)) {
			tags = [...tags, val];
		}
		tagInput = '';
	};

	const removeTag = (tag: string) => {
		tags = tags.filter((t) => t !== tag);
	};

	const handleTagKeydown = (e: KeyboardEvent) => {
		if (e.key === 'Enter' || e.key === ',') {
			e.preventDefault();
			addTag();
		} else if (e.key === 'Backspace' && tagInput === '' && tags.length > 0) {
			tags = tags.slice(0, -1);
		}
	};

	const handleReset = () => {
		if (!initialSnapshot) return;
		const next = cloneSettingsSnapshot(initialSnapshot);
		name = next.name;
		command = next.command;
		content = next.content;
		tags = next.tags;
		accessControl = next.accessControl;
		tagInput = '';
	};

	onMount(async () => {
		if (prompt) {
			name = prompt.name || prompt.title || '';
			await tick();

			command = prompt.command.at(0) === '/' ? prompt.command.slice(1) : prompt.command;
			content = prompt.content;
			tags = prompt.tags ?? [];

			accessControl = normalizeAccessControl(prompt?.access_control ?? null);
		}

		loaded = true;
		await tick();
		initialSnapshot = cloneSettingsSnapshot(buildSnapshot());
	});
</script>

<AccessControlModal
	bind:show={showAccessControlModal}
	bind:accessControl
	accessRoles={['read', 'write']}
	allowPublic={$user?.permissions?.sharing?.public_prompts || $user?.role === 'admin'}
/>

<div class="workspace-section w-full max-h-full flex justify-center">
	<form
		class="flex flex-col w-full mb-10"
		on:submit|preventDefault={() => {
			submitHandler();
		}}
	>
		<div class="my-2">
			<div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
				<Tooltip
					content={`${$i18n.t('Only alphanumeric characters and hyphens are allowed')} - ${$i18n.t(
						'Activate this command by typing "/{{COMMAND}}" to chat input.',
						{
							COMMAND: command
						}
					)}`}
					placement="bottom-start"
				>
					<div class="flex flex-col w-full">
						<div class="flex items-center">
							<input
								class="text-2xl font-semibold w-full bg-transparent outline-hidden"
								placeholder={$i18n.t('Title')}
								bind:value={name}
								required
							/>

							<div class="self-center shrink-0">
								<button
									class="bg-gray-50 hover:bg-gray-100 text-black dark:bg-gray-850 dark:hover:bg-gray-800 dark:text-white transition px-2 py-1 rounded-full flex gap-1 items-center"
									type="button"
									on:click={() => {
										showAccessControlModal = true;
									}}
								>
									<LockClosed strokeWidth="2.5" className="size-3.5" />

									<div class="text-sm font-medium shrink-0">
										{$i18n.t('Access')}
									</div>
								</button>
							</div>
						</div>

						<div class="flex gap-0.5 items-center text-xs text-gray-500">
							<div class="">/</div>
							<input
								class=" w-full bg-transparent outline-hidden"
								placeholder={$i18n.t('Command')}
								bind:value={command}
								required
								disabled={edit}
							/>
						</div>
					</div>
				</Tooltip>

				<InlineDirtyActions
					dirty={dirty}
					saving={loading}
					saveAsSubmit={true}
					align="start"
					forceShow={!edit}
					on:reset={handleReset}
				/>
			</div>
		</div>

		<div class="my-2">
			<div class="flex w-full justify-between">
				<div class=" self-center text-sm font-semibold">{$i18n.t('Prompt Content')}</div>
			</div>

			<div class="mt-2">
				<div>
					<Textarea
						className="text-sm w-full bg-transparent outline-hidden overflow-y-hidden resize-none"
						placeholder={$i18n.t('Write a summary in 50 words that summarizes [topic or keyword].')}
						bind:value={content}
						rows={6}
						required
					/>
				</div>

				<div class="text-xs text-gray-400 dark:text-gray-500">
					ⓘ {$i18n.t('Format your variables using brackets like this:')}&nbsp;<span
						class=" text-gray-600 dark:text-gray-300 font-medium"
						>{'{{'}{$i18n.t('variable')}{'}}'}</span
					>.
					{$i18n.t('Make sure to enclose them with')}
					<span class=" text-gray-600 dark:text-gray-300 font-medium">{'{{'}</span>
					{$i18n.t('and')}
					<span class=" text-gray-600 dark:text-gray-300 font-medium">{'}}'}</span>.
				</div>

				<div class="text-xs text-gray-400 dark:text-gray-500">
					{$i18n.t('Utilize')}<span class=" text-gray-600 dark:text-gray-300 font-medium">
						{` {{CLIPBOARD}}`}</span
					>
					{$i18n.t('variable to have them replaced with clipboard content.')}
				</div>
			</div>
		</div>

		<div class="my-2">
			<div class="text-sm font-semibold mb-1">{$i18n.t('Tags')}</div>
			<div
				class="flex flex-wrap gap-1.5 items-center p-2 rounded-lg border border-gray-200 dark:border-gray-700 bg-transparent"
			>
				{#each tags as tag}
					<span
						class="inline-flex items-center gap-1 text-xs px-2 py-0.5 rounded-full bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-300"
					>
						{tag}
						<button
							type="button"
							class="hover:text-red-500 transition"
							on:click={() => removeTag(tag)}
						>
							<svg
								xmlns="http://www.w3.org/2000/svg"
								viewBox="0 0 16 16"
								fill="currentColor"
								class="size-3"
							>
								<path
									d="M5.28 4.22a.75.75 0 0 0-1.06 1.06L6.94 8l-2.72 2.72a.75.75 0 1 0 1.06 1.06L8 9.06l2.72 2.72a.75.75 0 1 0 1.06-1.06L9.06 8l2.72-2.72a.75.75 0 0 0-1.06-1.06L8 6.94 5.28 4.22Z"
								/>
							</svg>
						</button>
					</span>
				{/each}
				<input
					class="flex-1 min-w-[80px] text-sm bg-transparent outline-hidden"
					placeholder={tags.length === 0 ? $i18n.t('Add tags...') : ''}
					bind:value={tagInput}
					on:keydown={handleTagKeydown}
					on:blur={addTag}
				/>
			</div>
		</div>
	</form>
</div>
