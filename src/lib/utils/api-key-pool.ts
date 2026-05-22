export type ApiKeyPoolMode = 'round_robin' | 'random' | 'priority';

export const getApiKeyPoolSummary = (config: any, legacyKey = '') => {
	const pool = config?.api_key_pool ?? {};
	const keys = Array.isArray(pool?.keys)
		? pool.keys.filter((entry: any) => (entry?.key ?? '').toString().trim())
		: legacyKey
			? [{ enabled: true }]
			: [];
	const enabled = keys.filter((entry: any) => entry?.enabled !== false);
	const mode: ApiKeyPoolMode = ['round_robin', 'random', 'priority'].includes(pool?.mode)
		? pool.mode
		: 'round_robin';

	return {
		total: keys.length,
		enabled: enabled.length,
		mode,
		retry: pool?.retry?.enabled !== false
	};
};

export const getApiKeyPoolModeLabel = (mode: ApiKeyPoolMode, t: (key: string) => string) => {
	if (mode === 'random') return t('Random');
	if (mode === 'priority') return t('Priority');
	return t('Round Robin');
};
