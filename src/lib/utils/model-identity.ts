type AnyModel = {
	id?: string;
	selection_id?: string;
	selectionId?: string;
	selection_key?: string;
	selectionKey?: string;
	model_id?: string;
	original_id?: string;
	originalId?: string;
	legacy_id?: string | null;
	legacyId?: string | null;
	legacy_ids?: string[];
	legacyIds?: string[];
	model_ref?: Record<string, unknown>;
	[key: string]: any;
};

const normalize = (value: unknown) => (typeof value === 'string' ? value.trim() : '');

export const getModelSelectionId = (model?: AnyModel | null): string =>
	normalize(
		model?.selection_id ??
			model?.selectionId ??
			model?.selection_key ??
			model?.selectionKey ??
			model?.id
	);

export const getModelCleanId = (model?: AnyModel | null): string =>
	normalize(model?.model_id ?? model?.original_id ?? model?.originalId ?? model?.id);

export const getModelRef = (model?: AnyModel | null): Record<string, unknown> | null => {
	const ref = model?.model_ref;
	return ref && typeof ref === 'object' ? ref : null;
};

export const getModelLegacyIds = (model?: AnyModel | null): string[] => {
	const values = [
		model?.id,
		model?.selection_key,
		model?.selectionKey,
		model?.model_id,
		model?.original_id,
		model?.originalId,
		model?.legacy_id,
		model?.legacyId,
		...(Array.isArray(model?.legacy_ids) ? model?.legacy_ids : []),
		...(Array.isArray(model?.legacyIds) ? model?.legacyIds : [])
	];
	return Array.from(new Set(values.map(normalize).filter(Boolean)));
};

export const getModelIdentityAliases = (model?: AnyModel | null): string[] => {
	const selectionId = getModelSelectionId(model);
	return Array.from(new Set([selectionId, ...getModelLegacyIds(model)].filter(Boolean)));
};

export const buildModelIdentityLookup = <T extends AnyModel>(
	models: T[] = []
): { byId: Map<string, T>; ambiguous: Set<string> } => {
	const byId = new Map<string, T>();
	const ambiguous = new Set<string>();
	const identityOf = (model: T) => getModelSelectionId(model) || normalize(model?.id);

	for (const model of models ?? []) {
		const identity = identityOf(model);
		for (const alias of getModelIdentityAliases(model)) {
			if (ambiguous.has(alias)) continue;

			const existing = byId.get(alias);
			if (!existing) {
				byId.set(alias, model);
				continue;
			}

			if (identityOf(existing) === identity) continue;

			byId.delete(alias);
			ambiguous.add(alias);
		}
	}

	return { byId, ambiguous };
};

export const findModelByIdentity = <T extends AnyModel>(
	models: T[] = [],
	value?: string | null
): T | undefined => {
	const id = normalize(value);
	if (!id) return undefined;
	const { byId } = buildModelIdentityLookup(models);
	return byId.get(id);
};

export const resolveModelSelectionId = <T extends AnyModel>(
	models: T[] = [],
	value?: string | null,
	options: { preserveAmbiguous?: boolean } = {}
): string => {
	const id = normalize(value);
	if (!id) return '';
	const { byId, ambiguous } = buildModelIdentityLookup(models);
	const model = byId.get(id);
	if (options.preserveAmbiguous && ambiguous.has(id)) return id;
	return model ? getModelSelectionId(model) : '';
};
