<script lang="ts">
	import { toast } from 'svelte-sonner';
	import fileSaver from 'file-saver';
	const { saveAs } = fileSaver;

	import { onMount, getContext, tick } from 'svelte';
	import { WEBUI_NAME, config, functions, user } from '$lib/stores';

	import { goto } from '$app/navigation';
	import {
		createNewFunction,
		deleteFunctionById,
		exportFunctions,
		getFunctionById,
		getFunctions,
		toggleFunctionById,
		toggleGlobalById
	} from '$lib/apis/functions';
	import Tooltip from '../common/Tooltip.svelte';
	import ConfirmDialog from '../common/ConfirmDialog.svelte';
	import { refreshModels } from '$lib/services/models';
	import FunctionMenu from '../admin/Functions/FunctionMenu.svelte';
	import EllipsisHorizontal from '../icons/EllipsisHorizontal.svelte';
	import Switch from '../common/Switch.svelte';
	import ValvesModal from './common/ValvesModal.svelte';
	import ManifestModal from './common/ManifestModal.svelte';
	import Heart from '../icons/Heart.svelte';
	import DeleteConfirmDialog from '$lib/components/common/ConfirmDialog.svelte';
	import GarbageBin from '../icons/GarbageBin.svelte';
	import Search from '../icons/Search.svelte';
	import Plus from '../icons/Plus.svelte';
	import ChevronRight from '../icons/ChevronRight.svelte';
	import Spinner from '../common/Spinner.svelte';
	import { capitalizeFirstLetter } from '$lib/utils';
	import HaloSelect from '$lib/components/common/HaloSelect.svelte';

	const i18n = getContext('i18n');

	let shiftKey = false;
	let loaded = false;

	let functionsImportInputElement: HTMLInputElement;
	let importFiles;

	let showConfirm = false;
	let query = '';

	let showManifestModal = false;
	let showValvesModal = false;
	let selectedFunction = null;

	let showDeleteConfirm = false;

	let sortBy = 'updated'; // 'name' | 'updated'

	let filteredItems = [];
	$: isInitialLoading = $functions === null;
	$: countLabel = isInitialLoading ? '...' : `${filteredItems.length}`;

	const sortItems = (items: any[]) => {
		return [...items].sort((a, b) => {
			if (sortBy === 'name') return (a.name || '').localeCompare(b.name || '');
			return (b.updated_at || 0) - (a.updated_at || 0);
		});
	};

	$: filteredItems = sortItems(
		($functions ?? []).filter(
			(f) =>
				query === '' ||
				f.name.toLowerCase().includes(query.toLowerCase()) ||
				f.id.toLowerCase().includes(query.toLowerCase())
		)
	);

	const shareHandler = async (func) => {
		const item = await getFunctionById(localStorage.token, func.id).catch((error) => {
			toast.error(`${error}`);
			return null;
		});

		toast.success($i18n.t('Redirecting you to Open WebUI Community'));

		const url = 'https://openwebui.com';
		const tab = await window.open(`${url}/functions/create`, '_blank');

		const messageHandler = (event) => {
			if (event.origin !== url) return;
			if (event.data === 'loaded') {
				tab.postMessage(JSON.stringify(item), '*');
				window.removeEventListener('message', messageHandler);
			}
		};

		window.addEventListener('message', messageHandler, false);
	};

	const cloneHandler = async (func) => {
		const _function = await getFunctionById(localStorage.token, func.id).catch((error) => {
			toast.error(`${error}`);
			return null;
		});

		if (_function) {
			sessionStorage.function = JSON.stringify({
				..._function,
				id: `${_function.id}_clone`,
				name: `${_function.name} (Clone)`
			});
			goto('/workspace/functions/create');
		}
	};

	const exportHandler = async (func) => {
		const _function = await getFunctionById(localStorage.token, func.id).catch((error) => {
			toast.error(`${error}`);
			return null;
		});

		if (_function) {
			let blob = new Blob([JSON.stringify([_function])], {
				type: 'application/json'
			});
			saveAs(blob, `function-${_function.id}-export-${Date.now()}.json`);
		}
	};

	const deleteHandler = async (func) => {
		const res = await deleteFunctionById(localStorage.token, func.id).catch((error) => {
			toast.error(`${error}`);
			return null;
		});

		if (res) {
			toast.success($i18n.t('Function deleted successfully'));
			functions.set(await getFunctions(localStorage.token));
			await refreshModels(localStorage.token, { force: true, reason: 'admin-functions' });
		}
	};

	const toggleGlobalHandler = async (func) => {
		const res = await toggleGlobalById(localStorage.token, func.id).catch((error) => {
			toast.error(`${error}`);
		});

		if (res) {
			if (func.is_global) {
				func.type === 'filter'
					? toast.success($i18n.t('Filter is now globally enabled'))
					: toast.success($i18n.t('Function is now globally enabled'));
			} else {
				func.type === 'filter'
					? toast.success($i18n.t('Filter is now globally disabled'))
					: toast.success($i18n.t('Function is now globally disabled'));
			}

			functions.set(await getFunctions(localStorage.token));
			await refreshModels(localStorage.token, { force: true, reason: 'admin-functions' });
		}
	};

	onMount(async () => {
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
		{$i18n.t('Functions')} | {$WEBUI_NAME}
	</title>
</svelte:head>

{#if loaded}
	<div class="space-y-4">
		<section class="workspace-section space-y-4">
			<div class="flex flex-col gap-3 lg:flex-row lg:items-center">
				<div class="workspace-toolbar-summary">
					<div class="workspace-count-pill">
						{countLabel} {$i18n.t('Functions')}
					</div>
					<div class="text-xs text-gray-500 dark:text-gray-400">
						{$i18n.t('Build and maintain filters, pipelines, and automation functions for admin-controlled workflows.')}
					</div>
				</div>

				<div class="workspace-toolbar">
					<div class="workspace-search workspace-toolbar-search">
						<Search className="size-4 text-gray-400" />
						<input
							class="w-full bg-transparent text-sm outline-hidden"
							bind:value={query}
							placeholder={$i18n.t('Search Functions')}
						/>
					</div>

					<div class="workspace-toolbar-actions">
						<HaloSelect
							bind:value={sortBy}
							options={[
								{ value: 'updated', label: $i18n.t('Recently Updated') },
								{ value: 'name', label: $i18n.t('Name') }
							]}
							className="w-fit max-w-full text-xs"
						/>

						<a class="workspace-primary-button" href="/workspace/functions/create">
							<Plus className="size-4" />
							<span>{$i18n.t('Create')}</span>
						</a>
					</div>
				</div>
			</div>
		</section>

		<section class="workspace-section">
			{#if isInitialLoading}
			<div class="workspace-empty-state">
				<Spinner />
			</div>
			{:else if filteredItems.length > 0}
			<div class="grid gap-3 lg:grid-cols-2 xl:grid-cols-3">
		{#each filteredItems as func (func.id)}
			<div
				class="glass-item flex space-x-4 cursor-pointer w-full px-4 py-3 transition"
			>
				<a
					class=" flex flex-1 space-x-3.5 cursor-pointer w-full"
					href={`/workspace/functions/edit?id=${encodeURIComponent(func.id)}`}
				>
					<div class="flex items-center text-left">
						<div class=" flex-1 self-center">
							<Tooltip content={func?.meta?.description ?? ''} placement="top-start">
								<div class=" font-semibold flex items-center gap-1.5">
									<div
										class=" text-xs font-bold px-1 rounded-sm uppercase line-clamp-1 bg-gray-500/20 text-gray-700 dark:text-gray-200"
									>
										{func.type}
									</div>

									{#if func?.meta?.manifest?.version}
										<div
											class="text-xs font-bold px-1 rounded-sm line-clamp-1 bg-gray-500/20 text-gray-700 dark:text-gray-200"
										>
											v{func?.meta?.manifest?.version ?? ''}
										</div>
									{/if}

									<div class="line-clamp-1">
										{func.name}

										<span class=" text-gray-500 text-xs font-medium shrink-0">{func.id}</span>
									</div>
								</div>
							</Tooltip>

							<div class="px-0.5">
								<div class="flex gap-1.5 mt-0.5 mb-0.5">
									<div class=" text-xs overflow-hidden text-ellipsis line-clamp-1">
										{func.meta.description}
									</div>
								</div>

								<div class="text-xs text-gray-500 shrink-0">
									<Tooltip
										content={func?.user?.email ??
											(func?.user_id === $user?.id ? $user?.email : $i18n.t('Deleted User'))}
										className="flex shrink-0"
										placement="top-start"
									>
										{$i18n.t('By {{name}}', {
											name: capitalizeFirstLetter(
												func?.user?.name ??
													func?.user?.email ??
													(func?.user_id === $user?.id
														? $user?.name ?? $user?.email
														: $i18n.t('Deleted User'))
											)
										})}
									</Tooltip>
								</div>
							</div>
						</div>
					</div>
				</a>
				<div class="flex flex-row gap-0.5 self-center">
					{#if shiftKey}
						<Tooltip content={$i18n.t('Delete')}>
							<button
								class="self-center w-fit text-sm px-2 py-2 dark:text-gray-300 dark:hover:text-white hover:bg-black/5 dark:hover:bg-white/5 rounded-xl"
								type="button"
								on:click={() => {
									deleteHandler(func);
								}}
							>
								<GarbageBin />
							</button>
						</Tooltip>
					{:else}
						{#if func?.meta?.manifest?.funding_url ?? false}
							<Tooltip content={$i18n.t('Support')}>
								<button
									class="self-center w-fit text-sm px-2 py-2 dark:text-gray-300 dark:hover:text-white hover:bg-black/5 dark:hover:bg-white/5 rounded-xl"
									type="button"
									on:click={() => {
										selectedFunction = func;
										showManifestModal = true;
									}}
								>
									<Heart />
								</button>
							</Tooltip>
						{/if}

						<Tooltip content={$i18n.t('Valves')}>
							<button
								class="self-center w-fit text-sm px-2 py-2 dark:text-gray-300 dark:hover:text-white hover:bg-black/5 dark:hover:bg-white/5 rounded-xl"
								type="button"
								on:click={() => {
									selectedFunction = func;
									showValvesModal = true;
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
										d="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.325.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 0 1 1.37.49l1.296 2.247a1.125 1.125 0 0 1-.26 1.431l-1.003.827c-.293.241-.438.613-.43.992a7.723 7.723 0 0 1 0 .255c-.008.378.137.75.43.991l1.004.827c.424.35.534.955.26 1.43l-1.298 2.247a1.125 1.125 0 0 1-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.47 6.47 0 0 1-.22.128c-.331.183-.581.495-.644.869l-.213 1.281c-.09.543-.56.94-1.11.94h-2.594c-.55 0-1.019-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 0 1-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 0 1-1.369-.49l-1.297-2.247a1.125 1.125 0 0 1 .26-1.431l1.004-.827c.292-.24.437-.613.43-.991a6.932 6.932 0 0 1 0-.255c.007-.38-.138-.751-.43-.992l-1.004-.827a1.125 1.125 0 0 1-.26-1.43l1.297-2.247a1.125 1.125 0 0 1 1.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.086.22-.128.332-.183.582-.495.644-.869l.214-1.28Z"
									/>
									<path
										stroke-linecap="round"
										stroke-linejoin="round"
										d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z"
									/>
								</svg>
							</button>
						</Tooltip>

						<FunctionMenu
							{func}
							editHandler={() => {
								goto(`/workspace/functions/edit?id=${encodeURIComponent(func.id)}`);
							}}
							shareHandler={() => {
								shareHandler(func);
							}}
							cloneHandler={() => {
								cloneHandler(func);
							}}
							exportHandler={() => {
								exportHandler(func);
							}}
							deleteHandler={async () => {
								selectedFunction = func;
								showDeleteConfirm = true;
							}}
							toggleGlobalHandler={() => {
								if (['filter', 'action'].includes(func.type)) {
									toggleGlobalHandler(func);
								}
							}}
							onClose={() => {}}
						>
							<button
								class="self-center w-fit text-sm p-1.5 dark:text-gray-300 dark:hover:text-white hover:bg-black/5 dark:hover:bg-white/5 rounded-xl"
								type="button"
							>
								<EllipsisHorizontal className="size-5" />
							</button>
						</FunctionMenu>
					{/if}

					<div class=" self-center mx-1">
						<Tooltip content={func.is_active ? $i18n.t('Enabled') : $i18n.t('Disabled')}>
							<Switch
								bind:state={func.is_active}
								on:change={async (e) => {
									toggleFunctionById(localStorage.token, func.id);
									await refreshModels(localStorage.token, {
										force: true,
										reason: 'admin-functions'
									});
								}}
							/>
						</Tooltip>
					</div>
				</div>
			</div>
		{/each}
			</div>
			{:else}
			<div class="workspace-empty-state">
				<p class="text-sm text-gray-500 dark:text-gray-400">
					{query
						? $i18n.t('No functions found matching your search')
						: $i18n.t('No functions yet. Create your first function to get started.')}
				</p>
			</div>
			{/if}
		</section>

		{#if $user?.role === 'admin'}
			<section class="workspace-section">
				<div class="flex flex-wrap justify-end gap-2">
				<input
					id="functions-import-input"
					bind:this={functionsImportInputElement}
					bind:files={importFiles}
					type="file"
					accept=".json"
					hidden
					on:change={() => {
						showConfirm = true;
					}}
				/>

				<button
					class="workspace-secondary-button text-xs"
					on:click={() => {
						functionsImportInputElement.click();
					}}
				>
					<div class=" self-center mr-2 font-medium line-clamp-1">
						{$i18n.t('Import Functions')}
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

				{#if ($functions ?? []).length}
					<button
						class="workspace-secondary-button text-xs"
						on:click={async () => {
							const _functions = await exportFunctions(localStorage.token).catch((error) => {
								toast.error(`${error}`);
								return null;
							});

							if (_functions) {
								let blob = new Blob([JSON.stringify(_functions)], {
									type: 'application/json'
								});
								saveAs(blob, `functions-export-${Date.now()}.json`);
							}
						}}
					>
						<div class=" self-center mr-2 font-medium line-clamp-1">
							{$i18n.t('Export Functions')}
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

		{#if $config?.features.enable_community_sharing}
			<section class="workspace-section space-y-3">
				<div class="text-base font-semibold text-gray-900 dark:text-gray-100">
					{$i18n.t('Made by Open WebUI Community')}
				</div>

				<a
					class="glass-item flex cursor-pointer items-center justify-between w-full px-4 py-3 transition"
					href="https://openwebui.com/#open-webui-community"
					target="_blank"
				>
					<div class=" self-center">
						<div class=" font-semibold line-clamp-1">{$i18n.t('Discover a function')}</div>
						<div class=" text-sm line-clamp-1 text-gray-500 dark:text-gray-400">
							{$i18n.t('Discover, download, and explore custom functions')}
						</div>
					</div>

					<div>
						<div>
							<ChevronRight />
						</div>
					</div>
				</a>
			</section>
		{/if}
	</div>

	<DeleteConfirmDialog
		bind:show={showDeleteConfirm}
		title={$i18n.t('Delete function?')}
		on:confirm={() => {
			deleteHandler(selectedFunction);
		}}
	>
		<div class=" text-sm text-gray-500">
			{$i18n.t('This will delete')} <span class="  font-semibold">{selectedFunction?.name}</span>.
		</div>
	</DeleteConfirmDialog>

	<ManifestModal
		bind:show={showManifestModal}
		manifest={selectedFunction?.meta?.manifest ?? {}}
	/>
	<ValvesModal
		bind:show={showValvesModal}
		type="function"
		id={selectedFunction?.id ?? null}
		on:save={async () => {
			await tick();
			await refreshModels(localStorage.token, { force: true, reason: 'admin-functions' });
		}}
	/>

	<ConfirmDialog
		bind:show={showConfirm}
		on:confirm={() => {
			const reader = new FileReader();
			reader.onload = async (event) => {
				const _functions = JSON.parse(event.target.result);

				for (const func of _functions) {
					const res = await createNewFunction(localStorage.token, func).catch((error) => {
						toast.error(`${error}`);
						return null;
					});
				}

				toast.success($i18n.t('Functions imported successfully'));
				functions.set(await getFunctions(localStorage.token));
				await refreshModels(localStorage.token, { force: true, reason: 'admin-functions' });
			};

			reader.readAsText(importFiles[0]);
		}}
	>
		<div class="text-sm text-gray-500">
			<div class=" bg-yellow-500/20 text-yellow-700 dark:text-yellow-200 rounded-lg px-4 py-3">
				<div>{$i18n.t('Please carefully review the following warnings:')}</div>

				<ul class=" mt-1 list-disc pl-4 text-xs">
					<li>{$i18n.t('Functions allow arbitrary code execution.')}</li>
					<li>{$i18n.t('Do not install functions from sources you do not fully trust.')}</li>
				</ul>
			</div>

			<div class="my-3">
				{$i18n.t(
					'I acknowledge that I have read and I understand the implications of my action. I am aware of the risks associated with executing arbitrary code and I have verified the trustworthiness of the source.'
				)}
			</div>
		</div>
	</ConfirmDialog>
{:else}
	<div class="w-full h-full flex justify-center items-center">
		<Spinner />
	</div>
{/if}
