import type { Model, NativeWebSearchSupport } from '$lib/stores';
import type { WebSearchMode } from '$lib/utils/web-search-mode';
import nativeWebSearchRules from '$lib/data/native-web-search-rules.json';

type Translator = (key: string, options?: Record<string, unknown>) => string;

type WebSearchConfigLike = {
	features?: {
		enable_web_search?: boolean;
		enable_halo_web_search?: boolean;
		enable_native_web_search?: boolean;
	};
};

type ModelLike = Partial<Model> & {
	id?: string;
	name?: string;
	owned_by?: string;
	native_web_search_supported?: boolean;
	native_web_search_support?: NativeWebSearchSupport;
};

export type WebSearchModeOption = {
	value: WebSearchMode;
	label: string;
	shortLabel?: string;
	description?: string;
	descriptionTone?: 'default' | 'info' | 'warning';
	disabled?: boolean;
	badge?: string;
};

export type NativeWebSearchSummary = {
	total: number;
	supportedCount: number;
	unknownCount: number;
	unsupportedCount: number;
	anySupported: boolean;
	anyUnknown: boolean;
	anyUnsupported: boolean;
	allSupported: boolean;
	allUnsupported: boolean;
	hasSelection: boolean;
	sampleSupported?: NativeWebSearchSupport;
	sampleUnknown?: NativeWebSearchSupport;
	sampleUnsupported?: NativeWebSearchSupport;
};

const SUPPORTED_STATUSES = new Set<NativeWebSearchSupport['status']>([
	'supported',
	'unknown',
	'unsupported'
]);

type NativeWebSearchRule = {
	type?: string;
	value?: string;
	reason?: string;
};

type NativeWebSearchProviderRules = {
	default_status?: NativeWebSearchSupport['status'];
	allow?: NativeWebSearchRule[];
	deny?: NativeWebSearchRule[];
};

const RULE_MATCH_REGEX = 'regex';
const RULE_MATCH_CONTAINS = 'contains';
const RULE_MATCH_EQUALS = 'equals';
const RULE_MATCH_PREFIX = 'prefix';

function normalizeRuleProvider(provider?: string | null): string {
	const normalized = (provider ?? '').toString().trim().toLowerCase();
	if (normalized === 'google' || normalized === 'gemini') {
		return 'gemini';
	}
	return normalized || 'unknown';
}

function normalizeModelLookupValue(value?: string | null): string {
	const normalized = (value ?? '').toString().trim().toLowerCase();
	if (!normalized) return '';
	return normalized.startsWith('models/') ? normalized.slice('models/'.length) : normalized;
}

function getProviderRules(provider?: string | null): NativeWebSearchProviderRules {
	const providers = (nativeWebSearchRules as { providers?: Record<string, NativeWebSearchProviderRules> })
		.providers;
	if (!providers || typeof providers !== 'object') {
		return {};
	}
	return providers[normalizeRuleProvider(provider)] ?? {};
}

function matchesRule(rule: NativeWebSearchRule, candidate: string): boolean {
	const matchType = (rule?.type ?? '').toString().trim().toLowerCase();
	const ruleValue = (rule?.value ?? '').toString().trim();

	if (!candidate || !matchType || !ruleValue) {
		return false;
	}

	switch (matchType) {
		case RULE_MATCH_REGEX:
			return new RegExp(ruleValue, 'i').test(candidate);
		case RULE_MATCH_CONTAINS:
			return candidate.includes(ruleValue.toLowerCase());
		case RULE_MATCH_EQUALS:
			return candidate === ruleValue.toLowerCase();
		case RULE_MATCH_PREFIX:
			return candidate.startsWith(ruleValue.toLowerCase());
		default:
			return false;
	}
}

function resolveFallbackModelRule(
	provider?: string | null,
	model?: Pick<ModelLike, 'id' | 'name'> | null
): NativeWebSearchSupport {
	const normalizedProvider = normalizeRuleProvider(provider);
	const providerRules = getProviderRules(normalizedProvider);
	if (!providerRules.default_status && normalizedProvider !== 'openai' && normalizedProvider !== 'gemini') {
		return {
			status: 'unsupported',
			reason: normalizedProvider ? 'provider_not_supported' : 'unknown_model',
			source: 'inferred',
			provider: normalizedProvider,
			supported: false,
			can_attempt: false
		};
	}
	const candidates = [
		normalizeModelLookupValue(model?.id ?? ''),
		normalizeModelLookupValue(model?.name ?? '')
	].filter(Boolean);

	for (const [group, status] of [
		['deny', 'unsupported'],
		['allow', 'supported']
	] as const) {
		const rules = Array.isArray(providerRules[group]) ? providerRules[group] : [];
		for (const rule of rules) {
			for (const candidate of candidates) {
				if (matchesRule(rule, candidate)) {
					return {
						status,
						reason: rule.reason ?? 'model_rule_unknown',
						source: 'model_rules_fallback',
						provider: normalizedProvider,
						supported: status === 'supported',
						can_attempt: status !== 'unsupported',
						model_rule: {
							status,
							reason: rule.reason ?? 'model_rule_unknown',
							source: 'model_rules_fallback',
							match_type: rule.type,
							match_value: rule.value,
							matched_on: candidate
						}
					};
				}
			}
		}
	}

	return {
		status: providerRules.default_status ?? 'unknown',
		reason: 'model_rule_unknown',
		source: 'model_rules_fallback',
		provider: normalizedProvider,
		supported: false,
		can_attempt: normalizedProvider === 'openai' || normalizedProvider === 'gemini',
		model_rule: {
			status: providerRules.default_status ?? 'unknown',
			reason: 'model_rule_unknown',
			source: 'model_rules_fallback'
		}
	};
}

export function getNativeWebSearchSupport(model?: ModelLike | null): NativeWebSearchSupport {
	const support = model?.native_web_search_support;
	if (support && typeof support === 'object' && SUPPORTED_STATUSES.has(support.status)) {
		return support;
	}

	const fallbackRuleSupport = resolveFallbackModelRule(model?.owned_by, model);

	if (model?.native_web_search_supported === true) {
		return {
			status: 'supported',
			reason: 'legacy_supported',
			source: 'legacy'
		};
	}

	if (model?.native_web_search_supported === false) {
		return {
			...fallbackRuleSupport,
			status: 'unsupported',
			reason: fallbackRuleSupport.reason === 'model_rule_unknown' ? 'legacy_unsupported' : fallbackRuleSupport.reason,
			source: fallbackRuleSupport.source === 'model_rules_fallback' ? 'legacy+model_rules_fallback' : 'legacy'
		};
	}

	if (fallbackRuleSupport.source === 'model_rules_fallback') {
		return fallbackRuleSupport;
	}

	return {
		status: 'unsupported',
		reason: (model?.owned_by ?? '').toString().trim() ? 'provider_not_supported' : 'unknown_model',
		source: 'inferred'
	};
}

export function summarizeNativeWebSearchSupport(models: Array<ModelLike | null | undefined>): NativeWebSearchSummary {
	const validModels = models.filter(Boolean) as ModelLike[];
	const summary: NativeWebSearchSummary = {
		total: validModels.length,
		supportedCount: 0,
		unknownCount: 0,
		unsupportedCount: 0,
		anySupported: false,
		anyUnknown: false,
		anyUnsupported: false,
		allSupported: false,
		allUnsupported: false,
		hasSelection: validModels.length > 0
	};

	for (const model of validModels) {
		const support = getNativeWebSearchSupport(model);
		if (support.status === 'supported') {
			summary.supportedCount += 1;
			summary.sampleSupported ??= support;
			continue;
		}
		if (support.status === 'unknown') {
			summary.unknownCount += 1;
			summary.sampleUnknown ??= support;
			continue;
		}
		summary.unsupportedCount += 1;
		summary.sampleUnsupported ??= support;
	}

	summary.anySupported = summary.supportedCount > 0;
	summary.anyUnknown = summary.unknownCount > 0;
	summary.anyUnsupported = summary.unsupportedCount > 0;
	summary.allSupported = summary.hasSelection && summary.supportedCount === summary.total;
	summary.allUnsupported = summary.hasSelection && summary.unsupportedCount === summary.total;

	return summary;
}

export function describeNativeWebSearchSupport(
	t: Translator,
	support?: NativeWebSearchSupport | null
): string {
	switch (support?.reason) {
		case 'official_connection':
			return t('Official provider endpoint detected. This model can use built-in web search.');
		case 'connection_enabled':
			return t('Built-in web search is enabled for this connection.');
		case 'connection_disabled':
			return t('Built-in web search is disabled for this connection.');
		case 'compat_connection_unverified':
			return t(
				'This compatible endpoint is not verified yet. If the upstream supports built-in search tools, you can still try model-native web search in chat.'
			);
		case 'provider_not_supported':
			return t('This provider does not expose model-native web search in HaloWebUI yet.');
		case 'connection_not_found':
			return t('HaloWebUI could not resolve the connection behind this model.');
		default:
			break;
	}

	if (support?.status === 'supported') {
		return t('Model-native web search is available for this model.');
	}
	if (support?.status === 'unknown') {
		return t('Native web search availability for this model is currently unknown.');
	}
	return t('Model-native web search is unavailable for this model.');
}

export function getNativeWebSearchAvailabilityNote(
	t: Translator,
	summary: NativeWebSearchSummary,
	scope: 'selection' | 'catalog' = 'selection'
): string {
	if (!summary.hasSelection) {
		return '';
	}

	const prefix =
		scope === 'selection' ? t('Current selection') : t('Currently loaded models');

	if (summary.allSupported) {
		return t('{{scope}}: all {{count}} models support native web search.', {
			scope: prefix,
			count: summary.total
		});
	}

	if (summary.allUnsupported) {
		return t('{{scope}}: native web search is unavailable for all models.', {
			scope: prefix
		});
	}

	return t('{{scope}}: {{supported}} native, {{unknown}} unverified, {{unsupported}} unavailable.', {
		scope: prefix,
		supported: summary.supportedCount,
		unknown: summary.unknownCount,
		unsupported: summary.unsupportedCount
	});
}

function buildNativeModeDescription(t: Translator, summary: NativeWebSearchSummary): string {
	if (summary.allUnsupported) {
		return t('Model-native web search is unavailable for this model.');
	}
	if (summary.supportedCount === 0 && summary.unknownCount > 0) {
		return t('Current model has not been verified for built-in web search yet. You can still try it manually.');
	}

	return t('Use model-native web search directly for all selected models.');
}

function buildAutoModeDescription(
	t: Translator,
	haloEnabled: boolean,
	summary: NativeWebSearchSummary
): string {
	if (summary.allUnsupported) {
		return haloEnabled
			? t('Current model does not support built-in web search. Auto will use HaloWebUI instead.')
			: t('Model-native web search is unavailable for this model.');
	}
	if (summary.supportedCount === 0 && summary.unknownCount > 0) {
		return haloEnabled
			? t('Current model has not been verified for built-in web search yet. Auto will keep using HaloWebUI.')
			: t('Native web search availability for this model is currently unknown.');
	}

	return haloEnabled
		? t('Prefer model-native web search and keep HaloWebUI as fallback.')
		: t('Prefer native web search whenever it is supported.');
}

export function buildWebSearchModeOptions(
	t: Translator,
	config: WebSearchConfigLike | null | undefined,
	models: Array<ModelLike | null | undefined>
): WebSearchModeOption[] {
	const haloEnabled = Boolean(
		config?.features?.enable_halo_web_search ?? config?.features?.enable_web_search
	);
	const nativeEnabled = Boolean(config?.features?.enable_native_web_search);
	const summary = summarizeNativeWebSearchSupport(models);
	const nativeImpossible = summary.hasSelection && summary.allUnsupported;
	const autoImpossible = !haloEnabled && nativeImpossible;

	return [
		{
			value: 'off',
			label: t('Off'),
			description: t('Do not use any web search mode for this chat.')
		},
		...(haloEnabled
			? [
					{
						value: 'halo' as WebSearchMode,
						label: 'HaloWebUI',
						description: t(
							'Always use HaloWebUI web search for consistent behavior across selected models.'
						)
					}
				]
			: []),
		...(nativeEnabled
			? [
					{
						value: 'native' as WebSearchMode,
						label: t('模型原生联网'),
						description: buildNativeModeDescription(t, summary),
						descriptionTone:
							summary.allUnsupported
								? 'warning'
								: summary.supportedCount === 0 && summary.unknownCount > 0
									? 'info'
									: 'default',
						disabled: nativeImpossible
					},
					{
						value: 'auto' as WebSearchMode,
						label: t('Smart Web Search'),
						shortLabel: t('Smart'),
						description: buildAutoModeDescription(t, haloEnabled, summary),
						descriptionTone:
							summary.allUnsupported || (summary.supportedCount === 0 && summary.unknownCount > 0)
								? 'info'
								: 'default',
						disabled: autoImpossible,
						badge: haloEnabled && !autoImpossible ? t('Recommended') : undefined
					}
				]
			: [])
	];
}
