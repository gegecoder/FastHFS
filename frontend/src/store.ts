import { reactive, ref } from "vue";
import { Events } from "@wailsio/runtime";
import * as Api from "../bindings/FastHFS/app";
import { resolveLocale, setLocale } from "./i18n";
import type {
  Config,
  Mount,
  ServiceStatus,
  Share,
  User,
} from "../bindings/FastHFS/backend/models/models";

const THEME_KEY = "fasthfs-theme";

/** 默认配置（后端不可用时兜底展示） */
export function fallbackConfig(): Config {
  return {
    listenAddr: "127.0.0.1",
    port: 8080,
    downloadLimitKB: 0,
    uploadLimitKB: 0,
    anonymousMode: "readonly",
    theme: "light",
    maxUploadMB: 4096,
    blacklist: [".exe", ".bat", ".cmd", ".com", ".scr", ".msi", ".ps1", ".sh"],
    basePath: "",
    rateLimitPerMin: 300,
    autoStart: false,
    syslogEnabled: false,
    syslogTargets: [],
    language: "zh",
  };
}

export const state = reactive({
  config: fallbackConfig(),
  status: null as ServiceStatus | null,
  mounts: [] as Mount[],
  users: [] as User[],
  shares: [] as Share[],
  urls: [] as string[],
  localIPs: [] as string[],
  version: {} as Record<string, string>,
  runtime: {} as Record<string, unknown>,
  autoStart: false,
  loaded: false,
  busy: false,
});

export const theme = ref<"light" | "dark">("light");

export function applyTheme(next: "light" | "dark") {
  theme.value = next;
  document.documentElement.classList.toggle("dark", next === "dark");
  localStorage.setItem(THEME_KEY, next);
}

/** 启动前先按本地缓存应用主题，避免闪烁 */
export function initTheme() {
  const saved = localStorage.getItem(THEME_KEY);
  applyTheme(saved === "dark" ? "dark" : "light");
}

export function toggleTheme() {
  applyTheme(theme.value === "dark" ? "light" : "dark");
}

/** 服务是否运行中 */
export function isRunning(): boolean {
  return !!state.status?.running;
}

export async function refreshStatus() {
  try {
    state.status = await Api.GetStatus();
    state.urls = (await Api.GetURLs()) || [];
  } catch {
    state.status = null;
    state.urls = [];
  }
}

export async function refreshConfig() {
  try {
    const cfg = await Api.GetConfig();
    if (cfg) {
      if (!cfg.blacklist) cfg.blacklist = [];
      state.config = cfg;
      // 应用界面语言，保证切换语言后重启也能恢复
      setLocale(resolveLocale(cfg.language));
      // 仅在用户尚未手动选择过主题时采用服务端配置，避免覆盖本地偏好
      if (!localStorage.getItem(THEME_KEY) && (cfg.theme === "dark" || cfg.theme === "light")) {
        applyTheme(cfg.theme);
      }
    }
  } catch {
    state.config = fallbackConfig();
    setLocale(resolveLocale(state.config.language));
  }
}

export async function refreshMounts() {
  try {
    state.mounts = (await Api.ListMounts()) || [];
  } catch {
    state.mounts = [];
  }
}

export async function refreshUsers() {
  try {
    state.users = (await Api.ListUsers()) || [];
  } catch {
    state.users = [];
  }
}

export async function refreshShares() {
  try {
    state.shares = (await Api.ListShares()) || [];
  } catch {
    state.shares = [];
  }
}

export async function refreshEnv() {
  try {
    state.version = ((await Api.Version()) || {}) as Record<string, string>;
  } catch {
    state.version = {};
  }
  try {
    state.runtime = (await Api.RuntimeInfo()) || {};
  } catch {
    state.runtime = {};
  }
  try {
    state.localIPs = (await Api.GetLocalIPs()) || [];
  } catch {
    state.localIPs = [];
  }
  try {
    state.autoStart = await Api.GetAutoStartStatus();
  } catch {
    state.autoStart = false;
  }
}

export async function loadAll() {
  await Promise.all([
    refreshStatus(),
    refreshConfig(),
    refreshMounts(),
    refreshUsers(),
    refreshShares(),
    refreshEnv(),
  ]);
  state.loaded = true;
  setLocale(resolveLocale(state.config.language));
}

/** 订阅服务状态事件，返回取消函数 */
export function subscribeStatus(): () => void {
  try {
    const off = Events.On("service:status", (event: any) => {
      const payload = (event && event.data ? event.data : event) as ServiceStatus;
      if (!payload || typeof payload.running !== "boolean") return;
      state.status = payload;
      if (payload.running && payload.urls) {
        state.urls = payload.urls;
      } else if (!payload.running) {
        state.urls = [];
      }
    });
    return typeof off === "function" ? off : () => undefined;
  } catch {
    return () => undefined;
  }
}
