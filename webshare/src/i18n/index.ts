/**
 * 轻量 i18n 运行时（无第三方依赖）。
 *
 * - 语言状态是 Vue 响应式 ref：在模板 / computed 中调用 i18n() 会自动收集依赖，
 *   setLocale 后所有文案即时刷新；
 * - i18n() 也可在组件外（api / 组合式函数）直接调用；
 * - 占位符使用 {name} 形式，见 zh.ts 中的文案。
 *
 * 对外分享站点的语言由服务端注入的 window.__FASTHFS_LANG__ 决定，
 * 缺失或非法时回退 zh。
 */
import { ref } from "vue";
import zhMessages, { type MessageSchema } from "./zh";
import enMessages from "./en";

declare global {
  interface Window {
    /** 服务端注入的界面语言 */
    __FASTHFS_LANG__?: string;
  }
}

export type Locale = "zh" | "en";

const messages: Record<Locale, MessageSchema> = {
  zh: zhMessages,
  en: enMessages,
};

/** 可选语言（label 使用各语言母语自称，方便辨认） */
export const SUPPORTED_LOCALES: ReadonlyArray<{ value: Locale; label: string }> = [
  { value: "zh", label: "简体中文" },
  { value: "en", label: "English" },
];

/** 兜底语言 */
export const FALLBACK_LOCALE: Locale = "zh";

/** 把服务端注入的语言配置（可能为空 / 非法值）解析为受支持的语言 */
export function resolveLocale(lang?: string | null): Locale {
  if (lang === "zh" || lang === "en") return lang;
  return FALLBACK_LOCALE;
}

/** 服务端注入的界面语言 */
const injectedLang = typeof window === "undefined" ? undefined : window.__FASTHFS_LANG__;

const locale = ref<Locale>(resolveLocale(injectedLang));

/** 当前生效语言 */
export function getLocale(): Locale {
  return locale.value;
}

/** 切换当前语言（响应式，立即生效） */
export function setLocale(lang: string): void {
  if (lang in messages) {
    locale.value = lang as Locale;
  }
}

type Params = Record<string, string | number>;

function lookup(key: string): string | undefined {
  const segments = key.split(".");
  let cur: unknown = messages[locale.value];
  for (const seg of segments) {
    if (cur && typeof cur === "object" && seg in (cur as Record<string, unknown>)) {
      cur = (cur as Record<string, unknown>)[seg];
    } else {
      return undefined;
    }
  }
  return typeof cur === "string" ? cur : undefined;
}

/**
 * 翻译：i18n('browser.count', { n: 3 })。
 * 找不到 key 时原样返回 key，便于排查缺失翻译。
 */
export function i18n(key: string, params?: Params): string {
  const raw = lookup(key);
  if (raw === undefined) return key;
  if (!params) return raw;
  return raw.replace(/\{(\w+)\}/g, (match, name: string) =>
    Object.prototype.hasOwnProperty.call(params, name) ? String(params[name]) : match,
  );
}
