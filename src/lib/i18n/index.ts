import i18next from 'i18next';
import resourcesToBackend from 'i18next-resources-to-backend';
import LanguageDetector from 'i18next-browser-languagedetector';
import type { i18n as i18nType } from 'i18next';
import { writable } from 'svelte/store';

const PRIMARY_CHINESE_LOCALE = 'zh-CN';
const DEFAULT_LOCALE = PRIMARY_CHINESE_LOCALE;
let supportedLocaleCodesPromise: Promise<Set<string>> | null = null;
const ZH_TW_PHRASE_MAP: Array<[string, string]> = [
	['设置', '設定'],
	['工具集成', '工具整合'],
	['工作空间', '工作空間'],
	['工作流', '工作流程'],
	['当前运行的是', '目前使用的是'],
	['危险区域', '危險區域'],
	['关于', '關於'],
	['界面', '介面'],
	['接口', '介面'],
	['内置', '內建'],
	['默认', '預設'],
	['代码', '程式碼'],
	['文档', '文件'],
	['图像', '圖片'],
	['图片', '圖片'],
	['画图', '繪圖'],
	['联网', '聯網'],
	['网址', '網址'],
	['抓取', '擷取'],
	['知识库', '知識庫'],
	['导入', '匯入'],
	['导出', '匯出'],
	['加载', '載入'],
	['上传', '上傳'],
	['下载', '下載'],
	['删除', '刪除'],
	['编辑', '編輯'],
	['选择', '選擇'],
	['启用', '啟用'],
	['关闭', '關閉'],
	['选项', '選項'],
	['开关', '開關'],
	['验证', '驗證'],
	['权限', '權限'],
	['并发', '併發'],
	['异步', '非同步'],
	['压缩', '壓縮'],
	['服务端点', '服務端點'],
	['服务地址', '服務位址'],
	['服务器', '伺服器'],
	['服务', '服務'],
	['地址', '位址'],
	['密钥', '金鑰'],
	['请求', '請求'],
	['运行时', '執行環境'],
	['镜像', '映像'],
	['响应', '回應'],
	['会话', '對話'],
	['对话', '對話'],
	['语音', '語音'],
	['总览', '總覽'],
	['显示', '顯示'],
	['状态', '狀態'],
	['这里', '這裡'],
	['影响', '影響'],
	['频率', '頻率'],
	['轻量版', '輕量版'],
	['扩展', '擴充'],
	['历史', '歷史'],
	['系统', '系統'],
	['标题', '標題'],
	['标准', '標準'],
	['应用', '應用'],
	['链路', '鏈路'],
	['后续', '後續'],
	['阈值', '閾值'],
	['参数', '參數'],
	['层级', '層級'],
	['专用', '專用'],
	['适配', '適配'],
	['确认', '確認'],
	['执行', '執行'],
	['允许', '允許'],
	['输入', '輸入'],
	['输出', '輸出'],
	['说明', '說明'],
	['保存', '儲存'],
	['用户', '使用者'],
	['留空', '留空'],
	['查看', '檢視'],
	['记忆', '記憶'],
	['笔记', '筆記'],
	['频道', '頻道'],
	['点击', '點擊'],
	['简体', '簡體'],
	['繁體', '繁體']
];
const ZH_TW_CHAR_MAP: Record<string, string> = {
	与: '與',
	专: '專',
	业: '業',
	个: '個',
	为: '為',
	义: '義',
	云: '雲',
	仅: '僅',
	优: '優',
	会: '會',
	体: '體',
	传: '傳',
	关: '關',
	内: '內',
	册: '冊',
	写: '寫',
	决: '決',
	删: '刪',
	别: '別',
	创: '創',
	动: '動',
	务: '務',
	协: '協',
	单: '單',
	历: '歷',
	压: '壓',
	参: '參',
	发: '發',
	变: '變',
	叠: '疊',
	吗: '嗎',
	启: '啟',
	围: '圍',
	图: '圖',
	场: '場',
	块: '塊',
	处: '處',
	备: '備',
	复: '復',
	头: '頭',
	夹: '夾',
	实: '實',
	审: '審',
	宽: '寬',
	对: '對',
	导: '導',
	尔: '爾',
	层: '層',
	库: '庫',
	应: '應',
	开: '開',
	异: '異',
	强: '強',
	当: '當',
	录: '錄',
	忆: '憶',
	态: '態',
	愿: '願',
	户: '戶',
	扩: '擴',
	执: '執',
	扫: '掃',
	择: '擇',
	换: '換',
	据: '據',
	数: '數',
	无: '無',
	旧: '舊',
	时: '時',
	显: '顯',
	暂: '暫',
	杂: '雜',
	权: '權',
	条: '條',
	构: '構',
	档: '檔',
	检: '檢',
	标: '標',
	没: '沒',
	测: '測',
	浅: '淺',
	响: '響',
	点: '點',
	独: '獨',
	现: '現',
	环: '環',
	盖: '蓋',
	码: '碼',
	确: '確',
	种: '種',
	称: '稱',
	笔: '筆',
	简: '簡',
	类: '類',
	级: '級',
	细: '細',
	终: '終',
	结: '結',
	给: '給',
	络: '絡',
	统: '統',
	继: '繼',
	续: '續',
	编: '編',
	缓: '緩',
	缩: '縮',
	网: '網',
	范: '範',
	荐: '薦',
	获: '獲',
	补: '補',
	装: '裝',
	见: '見',
	规: '規',
	览: '覽',
	计: '計',
	认: '認',
	议: '議',
	记: '記',
	许: '許',
	设: '設',
	访: '訪',
	证: '證',
	识: '識',
	词: '詞',
	试: '試',
	话: '話',
	询: '詢',
	该: '該',
	语: '語',
	说: '說',
	请: '請',
	调: '調',
	账: '帳',
	质: '質',
	踪: '蹤',
	轻: '輕',
	载: '載',
	较: '較',
	辑: '輯',
	输: '輸',
	过: '過',
	运: '運',
	还: '還',
	这: '這',
	进: '進',
	远: '遠',
	连: '連',
	适: '適',
	选: '選',
	鉴: '鑑',
	钥: '鑰',
	链: '鏈',
	错: '錯',
	镜: '鏡',
	长: '長',
	门: '門',
	闭: '閉',
	问: '問',
	间: '間',
	闲: '閒',
	阀: '閥',
	险: '險',
	项: '項',
	须: '須',
	预: '預',
	频: '頻',
	题: '題',
	额: '額',
	验: '驗',
	于: '於',
	里: '裡',
	驻: '駐'
};

const getSupportedLocaleCodes = async () => {
	if (!supportedLocaleCodesPromise) {
		supportedLocaleCodesPromise = import(`./locales/languages.json`).then(({ default: languages }) => {
			return new Set(languages.map((language) => language.code));
		});
	}

	return supportedLocaleCodesPromise;
};

const resolveSupportedLocale = async (locale?: string | null, fallback = DEFAULT_LOCALE) => {
	const supportedLocaleCodes = await getSupportedLocaleCodes();
	const candidate = typeof locale === 'string' ? locale.trim() : '';

	if (candidate && supportedLocaleCodes.has(candidate)) {
		return candidate;
	}

	if (supportedLocaleCodes.has(fallback)) {
		return fallback;
	}

	return supportedLocaleCodes.values().next().value ?? DEFAULT_LOCALE;
};

const persistResolvedLocale = (locale: string) => {
	if (typeof window !== 'undefined') {
		window.localStorage.setItem('locale', locale);
	}
};

const normalizeZhTwText = (value: string) => {
	let next = `${value ?? ''}`;

	for (const [source, target] of ZH_TW_PHRASE_MAP) {
		next = next.replaceAll(source, target);
	}

	return Array.from(next, (char) => ZH_TW_CHAR_MAP[char] ?? char).join('');
};

export const translateWithDefault = (
	i18nInstance: {
		resolvedLanguage?: string | null;
		language?: string | null;
		t?: (key: string, options?: Record<string, any>) => string;
	} | null | undefined,
	key: string,
	defaultValue: string,
	options: Record<string, any> = {}
) => {
	const currentLanguage = `${i18nInstance?.resolvedLanguage ?? i18nInstance?.language ?? ''}`.trim();
	const localizedDefaultValue = currentLanguage === 'zh-TW'
		? normalizeZhTwText(key)
		: currentLanguage.startsWith('zh')
			? key
			: defaultValue;

	return (
		i18nInstance?.t?.(key, { ...options, defaultValue: localizedDefaultValue }) ??
		localizedDefaultValue
	);
};

const decorateI18nInstance = (i18nInstance: i18nType & { __haloZhTwDecorated?: boolean }) => {
	if (i18nInstance.__haloZhTwDecorated) {
		return i18nInstance;
	}

	const originalT = i18nInstance.t.bind(i18nInstance);

	i18nInstance.t = ((...args: Parameters<typeof originalT>) => {
		const result = originalT(...args);
		const currentLanguage = `${i18nInstance.resolvedLanguage ?? i18nInstance.language ?? ''}`.trim();

		if (currentLanguage !== 'zh-TW') {
			return result;
		}

		return typeof result === 'string' ? normalizeZhTwText(result) : result;
	}) as typeof i18nInstance.t;

	i18nInstance.__haloZhTwDecorated = true;
	return i18nInstance;
};

const getFallbackLocales = (locale?: string | readonly string[] | null) => {
	const primaryLocale = Array.isArray(locale) ? locale[0] : locale;
	const normalizedLocale = typeof primaryLocale === 'string' ? primaryLocale.trim() : '';

	if (!normalizedLocale) {
		return [DEFAULT_LOCALE];
	}

	if (normalizedLocale === DEFAULT_LOCALE) {
		return [DEFAULT_LOCALE];
	}

	if (normalizedLocale === PRIMARY_CHINESE_LOCALE) {
		return [PRIMARY_CHINESE_LOCALE];
	}

	if (normalizedLocale.startsWith('zh-')) {
		return [normalizedLocale, PRIMARY_CHINESE_LOCALE];
	}

	return [normalizedLocale, DEFAULT_LOCALE];
};

const createI18nStore = (i18n: i18nType) => {
	decorateI18nInstance(i18n);
	const i18nWritable = writable(i18n);

	i18n.on('initialized', () => {
		i18nWritable.set(i18n);
	});
	i18n.on('loaded', () => {
		i18nWritable.set(i18n);
	});
	i18n.on('added', () => i18nWritable.set(i18n));
	i18n.on('languageChanged', () => {
		i18nWritable.set(i18n);
	});
	return i18nWritable;
};

const createIsLoadingStore = (i18n: i18nType) => {
	const isLoading = writable(false);

	// if loaded resources are empty || {}, set loading to true
	i18n.on('loaded', (resources) => {
		// console.log('loaded:', resources);
		Object.keys(resources).length !== 0 && isLoading.set(false);
	});

	// if resources failed loading, set loading to true
	i18n.on('failedLoading', () => {
		isLoading.set(true);
	});

	return isLoading;
};

export const initI18n = async (_defaultLocale?: string | undefined) => {
	void _defaultLocale;
	const resolvedDefaultLocale = PRIMARY_CHINESE_LOCALE;
	const preloadLocales = [PRIMARY_CHINESE_LOCALE];

	const loadResource = (language: string, namespace: string) =>
		import(`./locales/${language}/${namespace}.json`);

	await i18next
		.use(resourcesToBackend(loadResource))
		.use(LanguageDetector)
		.init({
			debug: false,
			lng: PRIMARY_CHINESE_LOCALE,
			detection: {
				order: [],
				caches: [],
				lookupQuerystring: 'lang',
				lookupLocalStorage: 'locale'
			},
			fallbackLng: (code) => getFallbackLocales(code),
			preload: preloadLocales,
			ns: 'translation',
			returnEmptyString: false,
			interpolation: {
				escapeValue: false // not needed for svelte as it escapes by default
			}
		});

	const lang = await resolveSupportedLocale(i18next?.resolvedLanguage || i18next?.language || resolvedDefaultLocale);
	if (i18next.resolvedLanguage !== lang) {
		await i18next.changeLanguage(lang);
	}

	document.documentElement.setAttribute('lang', lang);
	persistResolvedLocale(lang);
};

const i18n = createI18nStore(i18next);
const isLoadingStore = createIsLoadingStore(i18next);

export const getLanguages = async () => {
	const languages = (await import(`./locales/languages.json`)).default;
	return languages;
};
export const changeLanguage = async (_lang: string) => {
	void _lang;
	// HaloWebUI is distributed as a Simplified Chinese-only interface.
	const resolvedLocale = PRIMARY_CHINESE_LOCALE;
	document.documentElement.setAttribute('lang', resolvedLocale);
	persistResolvedLocale(resolvedLocale);
	await i18next.changeLanguage(resolvedLocale);
};

export default i18n;
export const isLoading = isLoadingStore;
