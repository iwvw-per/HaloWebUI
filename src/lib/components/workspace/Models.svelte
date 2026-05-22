<script lang="ts">
	import { marked } from 'marked';

	import { toast } from 'svelte-sonner';
	import Sortable from 'sortablejs';

	import fileSaver from 'file-saver';
	const { saveAs } = fileSaver;

	import { onMount, getContext, tick } from 'svelte';
	import { goto } from '$app/navigation';
	const i18n = getContext('i18n');

	import { WEBUI_NAME, config, mobile, models as _models, settings, user } from '$lib/stores';
	import {
		createNewModel,
		deleteModelById,
		getModels as getWorkspaceModels,
		toggleModelById,
		updateModelById
	} from '$lib/apis/models';

	import { getModels } from '$lib/apis';
	import { getGroups } from '$lib/apis/groups';

	import EllipsisHorizontal from '../icons/EllipsisHorizontal.svelte';
	import ModelMenu from './Models/ModelMenu.svelte';
	import ModelDeleteConfirmDialog from '../common/ConfirmDialog.svelte';
	import Tooltip from '../common/Tooltip.svelte';
	import GarbageBin from '../icons/GarbageBin.svelte';
	import Search from '../icons/Search.svelte';
	import Plus from '../icons/Plus.svelte';
	import ChevronRight from '../icons/ChevronRight.svelte';
	import Switch from '../common/Switch.svelte';
	import Spinner from '../common/Spinner.svelte';
	import { capitalizeFirstLetter } from '$lib/utils';
	import { getModelChatDisplayName } from '$lib/utils/model-display';
	import HaloSelect from '$lib/components/common/HaloSelect.svelte';
	import { cloneSettingsSnapshot } from '$lib/utils/settings-dirty';

	let shiftKey = false;

	let importFiles;
	let modelsImportInputElement: HTMLInputElement;
	let loaded = false;

	let models = [];

	let filteredModels = [];
	let selectedModel = null;

	let showModelDeleteConfirm = false;

	let group_ids = [];
	let visibilityFilter = 'all'; // 'all' | 'public' | 'private'

	const canWriteModel = (model) => {
		if ($user?.role === 'admin') return true;
		if (model?.user_id === $user?.id) return true;

		const writeAccess = model?.access_control?.write ?? {};
		return (
			(writeAccess?.user_ids ?? []).includes($user?.id) ||
			(writeAccess?.group_ids ?? []).some((groupId) => group_ids.includes(groupId))
		);
	};

	const getModelFormPayload = (model) => {
		const info = cloneSettingsSnapshot(model?.info ?? model ?? {});
		delete info.user;

		return {
			id: model.id,
			base_model_id: info?.base_model_id ?? model?.base_model_id ?? null,
			name: model.name,
			params: info?.params ?? model?.params ?? {},
			meta: {
				...(model?.meta ?? {}),
				...(info?.meta ?? {})
			},
			access_control:
				info?.access_control !== undefined ? info.access_control : (model?.access_control ?? null),
			is_active: info?.is_active ?? model?.is_active ?? true
		};
	};

	$: if (models) {
		filteredModels = models.filter((m) => {
			const matchesSearch =
				searchValue === '' ||
				getModelChatDisplayName(m).toLowerCase().includes(searchValue.toLowerCase());

			const matchesVisibility =
				visibilityFilter === 'all' ||
				(visibilityFilter === 'public' && m.access_control == null) ||
				(visibilityFilter === 'private' && m.access_control != null);

			return matchesSearch && matchesVisibility;
		});
	}

	let searchValue = '';

	const deleteModelHandler = async (model) => {
		const res = await deleteModelById(localStorage.token, model.id).catch((e) => {
			toast.error(`${e}`);
			return null;
		});

		if (res) {
			toast.success($i18n.t(`Deleted {{name}}`, { name: model.id }));
		}

		await _models.set(
			await getModels(
				localStorage.token,
				$config?.features?.enable_direct_connections && ($settings?.directConnections ?? null)
			)
		);
		models = await getWorkspaceModels(localStorage.token);
	};

	const cloneModelHandler = async (model) => {
		const modelInfo = model?.info ?? model;
		sessionStorage.model = JSON.stringify({
			...modelInfo,
			id: `${model.id}-clone`,
			name: `${model.name} (Clone)`
		});
		goto('/workspace/models/create');
	};

	const shareModelHandler = async (model) => {
		toast.success($i18n.t('Redirecting you to Open WebUI Community'));

		const url = 'https://openwebui.com';

		const tab = await window.open(`${url}/models/create`, '_blank');

		const messageHandler = (event) => {
			if (event.origin !== url) return;
			if (event.data === 'loaded') {
				tab.postMessage(JSON.stringify(model), '*');
				window.removeEventListener('message', messageHandler);
			}
		};

		window.addEventListener('message', messageHandler, false);
	};

	const hideModelHandler = async (model) => {
		const info = getModelFormPayload(model);

		info.meta = {
			...info.meta,
			hidden: !(info?.meta?.hidden ?? false)
		};

		console.log(info);

		const res = await updateModelById(localStorage.token, info.id, info);

		if (res) {
			toast.success(
				$i18n.t(`Model {{name}} is now {{status}}`, {
					name: info.id,
					status: info.meta.hidden ? 'hidden' : 'visible'
				})
			);
		}

		await _models.set(
			await getModels(
				localStorage.token,
				$config?.features?.enable_direct_connections && ($settings?.directConnections ?? null)
			)
		);
		models = await getWorkspaceModels(localStorage.token);
	};

	const downloadModels = async (models) => {
		let blob = new Blob([JSON.stringify(models)], {
			type: 'application/json'
		});
		saveAs(blob, `models-export-${Date.now()}.json`);
	};

	const exportModelHandler = async (model) => {
		let blob = new Blob([JSON.stringify([model])], {
			type: 'application/json'
		});
		saveAs(blob, `${model.id}-${Date.now()}.json`);
	};

	onMount(async () => {
		models = await getWorkspaceModels(localStorage.token);
		let groups = await getGroups(localStorage.token);
		group_ids = groups.map((group) => group.id);

		loaded = true;

		const onKeyDown = (event) => {
			if (event.key === 'Shift') {
				shiftKey = true;
			}
		};

		const onKeyUp = (event) => {
			if (event.key === 'Shift') {
				shiftKey = false;
			}
		};

		const onBlur = () => {
			shiftKey = false;
		};

		window.addEventListener('keydown', onKeyDown);
		window.addEventListener('keyup', onKeyUp);
		window.addEventListener('blur', onBlur);

		return () => {
			window.removeEventListener('keydown', onKeyDown);
			window.removeEventListener('keyup', onKeyUp);
			window.removeEventListener('blur', onBlur);
		};
	});
</script>

<svelte:head>
	<title>
		{$i18n.t('Assistants')} | {$WEBUI_NAME}
	</title>
</svelte:head>

{#if loaded}
	<ModelDeleteConfirmDialog
		bind:show={showModelDeleteConfirm}
		on:confirm={() => {
			deleteModelHandler(selectedModel);
		}}
	/>

	<div class="space-y-4">
		<section class="workspace-section space-y-4">
			<div class="flex flex-col gap-3 lg:flex-row lg:items-center">
				<div class="workspace-toolbar-summary">
					<div class="workspace-count-pill">
						{filteredModels.length} {$i18n.t('Assistants')}
					</div>
					<div class="text-xs text-gray-500 dark:text-gray-400">
						{$i18n.t('Manage assistant presets, visibility, and tool-ready model profiles for your workspace.')}
					</div>
				</div>

				<div class="workspace-toolbar">
					<div class="workspace-search workspace-toolbar-search">
						<Search className="size-4 text-gray-400" />
						<input
							class="w-full bg-transparent text-sm outline-hidden"
							bind:value={searchValue}
							placeholder={$i18n.t('Search Assistants')}
						/>
					</div>

					<div class="workspace-toolbar-actions">
						<HaloSelect
							bind:value={visibilityFilter}
							options={[
								{ value: 'all', label: $i18n.t('All') },
								{ value: 'public', label: $i18n.t('Public') },
								{ value: 'private', label: $i18n.t('Private') }
							]}
							className="w-fit max-w-full text-xs"
						/>

						{#if $user?.role === 'admin' && filteredModels.length > 0}
							<Tooltip content={$i18n.t('Show All')}>
								<button
									class="workspace-icon-button px-3 py-2"
									on:click={async () => {
										let count = 0;
										for (const model of filteredModels) {
											if (model.info?.meta?.hidden) {
												const info = getModelFormPayload(model);
												info.meta = { ...info.meta, hidden: false };
												await updateModelById(localStorage.token, info.id, info);
												count++;
											}
										}
										if (count > 0) {
											toast.success($i18n.t('{{count}} models shown', { count }));
											await _models.set(
												await getModels(
													localStorage.token,
													$config?.features?.enable_direct_connections &&
														($settings?.directConnections ?? null)
												)
											);
											models = await getWorkspaceModels(localStorage.token);
										}
									}}
								>
									<svg
										xmlns="http://www.w3.org/2000/svg"
										fill="none"
										viewBox="0 0 24 24"
										stroke-width="1.5"
										stroke="currentColor"
										class="size-4"
									>
										<path
											stroke-linecap="round"
											stroke-linejoin="round"
											d="M2.036 12.322a1.012 1.012 0 010-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178z"
										/>
										<path
											stroke-linecap="round"
											stroke-linejoin="round"
											d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
										/>
									</svg>
								</button>
							</Tooltip>

							<Tooltip content={$i18n.t('Hide All')}>
								<button
									class="workspace-icon-button px-3 py-2"
									on:click={async () => {
										let count = 0;
										for (const model of filteredModels) {
											const info = getModelFormPayload(model);
											if (!info?.meta?.hidden) {
												info.meta = { ...info.meta, hidden: true };
												await updateModelById(localStorage.token, info.id, info);
												count++;
											}
										}
										if (count > 0) {
											toast.success($i18n.t('{{count}} models hidden', { count }));
											await _models.set(
												await getModels(
													localStorage.token,
													$config?.features?.enable_direct_connections &&
														($settings?.directConnections ?? null)
												)
											);
											models = await getWorkspaceModels(localStorage.token);
										}
									}}
								>
									<svg
										xmlns="http://www.w3.org/2000/svg"
										fill="none"
										viewBox="0 0 24 24"
										stroke-width="1.5"
										stroke="currentColor"
										class="size-4"
									>
										<path
											stroke-linecap="round"
											stroke-linejoin="round"
											d="M3.98 8.223A10.477 10.477 0 001.934 12C3.226 16.338 7.244 19.5 12 19.5c.993 0 1.953-.138 2.863-.395M6.228 6.228A10.45 10.45 0 0112 4.5c4.756 0 8.773 3.162 10.065 7.498a10.523 10.523 0 01-4.293 5.774M6.228 6.228L3 3m3.228 3.228l3.65 3.65m7.894 7.894L21 21m-3.228-3.228l-3.65-3.65m0 0a3 3 0 10-4.243-4.243m4.242 4.242L9.88 9.88"
										/>
									</svg>
								</button>
							</Tooltip>
						{/if}

						<a class="workspace-primary-button" href="/workspace/models/create">
							<Plus className="size-4" />
							<span>{$i18n.t('Create')}</span>
						</a>
					</div>
				</div>
			</div>
		</section>

		<section class="workspace-section">
			{#if filteredModels.length > 0}
			<div class="grid gap-3 lg:grid-cols-2 xl:grid-cols-3" id="model-list">
		{#each filteredModels as model}
			<div
				class="glass-item flex flex-col cursor-pointer w-full px-4 py-3 transition"
				id="model-item-{model.id}"
			>
				<div class="flex gap-4 mt-0.5 mb-0.5">
					<div class=" w-[44px]">
						<div
							class=" rounded-xl object-cover {model.is_active
								? ''
								: 'opacity-50 dark:opacity-50'} "
						>
							<img
								src={model?.meta?.profile_image_url ?? '/static/favicon.png'}
								alt="modelfile profile"
								class=" rounded-xl w-full h-auto object-cover {model?.meta?.profile_image_url
									? ''
									: 'dark:invert'}"
							/>
						</div>
					</div>

					<a
						class=" flex flex-1 cursor-pointer w-full"
						href={`/?models=${encodeURIComponent(model.id)}`}
					>
						<div class=" flex-1 self-center {model.is_active ? '' : 'text-gray-500'}">
							<Tooltip
								content={marked.parse(
									model?.meta?.description ?? model?.originalId ?? model?.original_id ?? model.id
								)}
								className=" w-fit"
								placement="top-start"
							>
								<div class=" font-semibold line-clamp-1">{getModelChatDisplayName(model)}</div>
							</Tooltip>

								<div class="flex gap-1 text-xs overflow-hidden">
									<div class="line-clamp-1">
										{#if (model?.meta?.description ?? '').trim()}
											{model?.meta?.description}
										{:else}
											{model?.originalId ?? model?.original_id ?? model.id}
										{/if}
									</div>
								</div>

							</div>
						</a>
					</div>

				<div class="flex justify-between items-center -mb-0.5 px-0.5">
					<div class=" text-xs mt-0.5">
						<Tooltip
							content={model?.user?.email ?? (model?.user_id === $user?.id ? $user?.email : $i18n.t('Deleted User'))}
							className="flex shrink-0"
							placement="top-start"
						>
							<div class="shrink-0 text-gray-500">
								{$i18n.t('By {{name}}', {
									name: capitalizeFirstLetter(
										model?.user?.name ??
											model?.user?.email ??
											(model?.user_id === $user?.id
												? ($user?.name ?? $user?.email)
												: $i18n.t('Deleted User'))
									)
								})}
							</div>
						</Tooltip>
					</div>

					<div class="flex flex-row gap-0.5 items-center">
						{#if shiftKey}
							<Tooltip content={$i18n.t('Delete')}>
								<button
									class="self-center w-fit text-sm px-2 py-2 dark:text-gray-300 dark:hover:text-white hover:bg-black/5 dark:hover:bg-white/5 rounded-xl"
									type="button"
									on:click={() => {
										deleteModelHandler(model);
									}}
								>
									<GarbageBin />
								</button>
							</Tooltip>
							{:else}
								{#if canWriteModel(model)}
									<a
										class="self-center w-fit text-sm px-2 py-2 dark:text-gray-300 dark:hover:text-white hover:bg-black/5 dark:hover:bg-white/5 rounded-xl"
									type="button"
									href={`/workspace/models/edit?id=${encodeURIComponent(model.id)}`}
								>
									<svg
										xmlns="http://www.w3.org/2000/svg"
										fill="none"
										viewBox="0 0 24 24"
										stroke-width="1.5"
										stroke="currentColor"
										class="w-4 h-4"
									>
										<path
											stroke-linecap="round"
											stroke-linejoin="round"
											d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L6.832 19.82a4.5 4.5 0 0 1-1.897 1.13l-2.685.8.8-2.685a4.5 4.5 0 0 1 1.13-1.897L16.863 4.487Zm0 0L19.5 7.125"
										/>
									</svg>
								</a>
							{/if}

							<ModelMenu
								user={$user}
								{model}
								shareHandler={() => {
									shareModelHandler(model);
								}}
								cloneHandler={() => {
									cloneModelHandler(model);
								}}
								exportHandler={() => {
									exportModelHandler(model);
								}}
								hideHandler={() => {
									hideModelHandler(model);
								}}
								deleteHandler={() => {
									selectedModel = model;
									showModelDeleteConfirm = true;
								}}
								onClose={() => {}}
							>
								<button
									class="self-center w-fit text-sm p-1.5 dark:text-gray-300 dark:hover:text-white hover:bg-black/5 dark:hover:bg-white/5 rounded-xl"
									type="button"
								>
									<EllipsisHorizontal className="size-5" />
								</button>
							</ModelMenu>

							<div class="ml-1">
								<Tooltip content={model.is_active ? $i18n.t('Enabled') : $i18n.t('Disabled')}>
									<Switch
										bind:state={model.is_active}
										on:change={async (e) => {
											toggleModelById(localStorage.token, model.id);
											_models.set(
												await getModels(
													localStorage.token,
													$config?.features?.enable_direct_connections &&
														($settings?.directConnections ?? null)
												)
											);
										}}
									/>
								</Tooltip>
							</div>
						{/if}
					</div>
				</div>
			</div>
		{/each}
			</div>
			{:else}
			<div class="workspace-empty-state">
				<p class="text-sm text-gray-500 dark:text-gray-400">
					{searchValue
						? $i18n.t('No assistants found matching your search')
						: $i18n.t('No assistants yet. Create your first assistant to get started.')}
				</p>
			</div>
			{/if}
		</section>
	</div>

	{#if $user?.role === 'admin'}
		<section class="workspace-section">
			<div class="flex flex-wrap justify-end gap-2">
				<input
					id="models-import-input"
					bind:this={modelsImportInputElement}
					bind:files={importFiles}
					type="file"
					accept=".json"
					hidden
					on:change={() => {
						console.log(importFiles);

						let reader = new FileReader();
						reader.onload = async (event) => {
							let savedModels = JSON.parse(event.target.result);
							console.log(savedModels);

							for (const model of savedModels) {
								if (model?.info ?? false) {
									if ($_models.find((m) => m.id === model.id)) {
										await updateModelById(localStorage.token, model.id, model.info).catch(
											(error) => {
												return null;
											}
										);
									} else {
										await createNewModel(localStorage.token, model.info).catch((error) => {
											return null;
										});
									}
								} else {
									if (model?.id && model?.name) {
										await createNewModel(localStorage.token, model).catch((error) => {
											return null;
										});
									}
								}
							}

							await _models.set(
								await getModels(
									localStorage.token,
									$config?.features?.enable_direct_connections &&
										($settings?.directConnections ?? null)
								)
							);
							models = await getWorkspaceModels(localStorage.token);
						};

						reader.readAsText(importFiles[0]);
					}}
				/>

				<button
					class="workspace-secondary-button text-xs"
					on:click={() => {
						modelsImportInputElement.click();
					}}
				>
					<div class=" self-center mr-2 font-medium line-clamp-1">
						{$i18n.t('Import Assistants')}
					</div>

					<div class=" self-center">
						<svg
							xmlns="http://www.w3.org/2000/svg"
							viewBox="0 0 16 16"
							fill="currentColor"
							class="w-4 h-4"
						>
							<path
								fill-rule="evenodd"
								d="M4 2a1.5 1.5 0 0 0-1.5 1.5v9A1.5 1.5 0 0 0 4 14h8a1.5 1.5 0 0 0 1.5-1.5V6.621a1.5 1.5 0 0 0-.44-1.06L9.94 2.439A1.5 1.5 0 0 0 8.878 2H4Zm4 9.5a.75.75 0 0 1-.75-.75V8.06l-.72.72a.75.75 0 0 1-1.06-1.06l2-2a.75.75 0 0 1 1.06 0l2 2a.75.75 0 1 1-1.06 1.06l-.72-.72v2.69a.75.75 0 0 1-.75.75Z"
								clip-rule="evenodd"
							/>
						</svg>
					</div>
				</button>

				{#if models.length}
					<button
						class="workspace-secondary-button text-xs"
						on:click={async () => {
							downloadModels(models);
						}}
					>
						<div class=" self-center mr-2 font-medium line-clamp-1">
							{$i18n.t('Export Assistants')}
						</div>

						<div class=" self-center">
							<svg
								xmlns="http://www.w3.org/2000/svg"
								viewBox="0 0 16 16"
								fill="currentColor"
								class="w-4 h-4"
							>
								<path
									fill-rule="evenodd"
									d="M4 2a1.5 1.5 0 0 0-1.5 1.5v9A1.5 1.5 0 0 0 4 14h8a1.5 1.5 0 0 0 1.5-1.5V6.621a1.5 1.5 0 0 0-.44-1.06L9.94 2.439A1.5 1.5 0 0 0 8.878 2H4Zm4 3.5a.75.75 0 0 1 .75.75v2.69l.72-.72a.75.75 0 1 1 1.06 1.06l-2 2a.75.75 0 0 1-1.06 0l-2-2a.75.75 0 0 1 1.06-1.06l.72.72V6.25A.75.75 0 0 1 8 5.5Z"
									clip-rule="evenodd"
								/>
							</svg>
						</div>
					</button>
				{/if}
			</div>
		</section>
	{/if}
{:else}
	<div class="w-full h-full flex justify-center items-center">
		<Spinner />
	</div>
{/if}
