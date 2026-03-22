export interface ConnectionErrorToastContent {
	title: string;
	description?: string;
}

type Translate = (key: string, options?: Record<string, unknown>) => string;

const PROVIDER_PREFIX_RE = /^(OpenAI|Gemini|Anthropic|Ollama):\s*/;
const INVALID_API_KEY_RE =
	/invalid_api_key|Incorrect API key provided|invalid api key|authentication failed|unauthorized/i;

const getRawErrorText = (error: unknown): string =>
	error instanceof Error ? error.message : typeof error === 'string' ? error : `${error ?? ''}`;

const localizeKnownFragments = (text: string, t: Translate): string =>
	text
		.replaceAll('URL is required', t('URL is required'))
		.replaceAll('Network Problem', t('Network Problem'))
		.replaceAll('Open WebUI: Server Connection Error', t('Open WebUI: Server Connection Error'));

const stripTransportWrappers = (text: string): string =>
	text
		.replace(/^Unexpected error:\s*/i, '')
		.replace(/^External Error:\s*/i, '')
		.trim();

const extractPayloadMessage = (text: string): string => {
	const messageMatch = text.match(/['"]message['"]:\s*['"]([\s\S]*?)['"],\s*['"]type['"]/);
	return messageMatch?.[1] ?? text;
};

const normalizeWhitespace = (text: string): string =>
	text
		.replaceAll('\\n', ' ')
		.replace(/\s+/g, ' ')
		.trim();

export const formatConnectionErrorToast = (
	error: unknown,
	t: Translate
): ConnectionErrorToastContent => {
	const raw = getRawErrorText(error);
	const providerMatch = raw.match(PROVIDER_PREFIX_RE);
	const providerPrefix = providerMatch ? `${providerMatch[1]}：` : '';
	const providerStripped = providerMatch ? raw.slice(providerMatch[0].length) : raw;
	const simplifiedDetail = normalizeWhitespace(
		extractPayloadMessage(stripTransportWrappers(providerStripped))
	);

	if (INVALID_API_KEY_RE.test(providerStripped)) {
		return {
			title: `${providerPrefix}${t('error.reason.api_auth_error')}`,
			description: t('connection.error.check_key_matches_url')
		};
	}

	const localizedSimpleDetail = localizeKnownFragments(simplifiedDetail, t);
	if (
		localizedSimpleDetail === t('URL is required') ||
		localizedSimpleDetail === t('Network Problem') ||
		localizedSimpleDetail === t('Open WebUI: Server Connection Error')
	) {
		return {
			title: providerPrefix ? `${providerPrefix}${localizedSimpleDetail}` : localizedSimpleDetail
		};
	}

	if (
		/Unexpected error:|External Error:/.test(providerStripped) ||
		providerStripped.includes("{'message':") ||
		providerStripped.includes('{"message":')
	) {
		return {
			title: providerPrefix ? `${providerPrefix}${t('Connection failed')}` : t('Connection failed'),
			description: localizedSimpleDetail
		};
	}

	const fallback = localizeKnownFragments(providerStripped, t);
	return {
		title: providerPrefix ? `${providerPrefix}${fallback}` : fallback
	};
};
