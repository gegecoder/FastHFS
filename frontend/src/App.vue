<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watchEffect } from "vue";
import {
  Document,
  FolderOpened,
  HomeFilled,
  InfoFilled,
  Link,
  Moon,
  Setting,
  Sunny,
  User,
} from "@element-plus/icons-vue";
import zhCn from "element-plus/es/locale/lang/zh-cn";
import en from "element-plus/es/locale/lang/en";
import HomeView from "./views/HomeView.vue";
import MountsView from "./views/MountsView.vue";
import UsersView from "./views/UsersView.vue";
import SharesView from "./views/SharesView.vue";
import LogsView from "./views/LogsView.vue";
import AboutView from "./views/AboutView.vue";
import SettingsView from "./views/SettingsView.vue";
import UpdateDialog from "./components/UpdateDialog.vue";
import { checkForUpdate } from "./composables/useUpdate";
import { getLocale, i18n } from "./i18n";
import { isRunning, loadAll, state, subscribeStatus, theme, toggleTheme } from "./store";

type PageKey = "home" | "mounts" | "users" | "shares" | "settings" | "logs" | "about";

const page = ref<PageKey>("home");

const navItems = [
  { key: "home", labelKey: "app.navHome", icon: HomeFilled },
  { key: "mounts", labelKey: "app.navMounts", icon: FolderOpened },
  { key: "users", labelKey: "app.navUsers", icon: User },
  { key: "shares", labelKey: "app.navShares", icon: Link },
  { key: "settings", labelKey: "app.navSettings", icon: Setting },
  { key: "logs", labelKey: "app.navLogs", icon: Document },
  { key: "about", labelKey: "app.navAbout", icon: InfoFilled },
] as const;

const running = computed(() => isRunning());

// Element Plus 组件（日期、分页、确认框按钮等）语言跟随界面语言
const elLocale = computed(() => (getLocale() === "en" ? en : zhCn));

// 同步 <html lang>，方便无障碍与浏览器识别当前语言
watchEffect(() => {
  document.documentElement.lang = getLocale() === "en" ? "en" : "zh-CN";
});

let unsubscribe: (() => void) | null = null;

onMounted(async () => {
  unsubscribe = subscribeStatus();
  await loadAll();
  // 启动时静默检测更新，发现新版本自动弹窗
  void checkForUpdate(true);
});

onUnmounted(() => {
  if (unsubscribe) unsubscribe();
});
</script>

<template>
  <el-config-provider :locale="elLocale">
    <div class="app-shell">
      <aside class="sidebar">
        <div class="brand">
          <div class="brand-logo">
            <svg viewBox="0 0 24 24" width="22" height="22" aria-hidden="true">
              <path
                d="M3 7.5A2.5 2.5 0 0 1 5.5 5h4l2 2h7A2.5 2.5 0 0 1 21 9.5v8A2.5 2.5 0 0 1 18.5 20h-13A2.5 2.5 0 0 1 3 17.5v-10Z"
                fill="currentColor"
                opacity="0.9"
              />
              <path d="M12 11v5m0 0 2.2-2.2M12 16l-2.2-2.2" stroke="#fff" stroke-width="1.4" stroke-linecap="round" />
            </svg>
          </div>
          <div class="brand-text">
            <h1>FastHFS</h1>
            <span>{{ i18n("app.tagline") }}</span>
          </div>
        </div>

        <nav class="nav">
          <button
            v-for="item in navItems"
            :key="item.key"
            class="nav-item"
            :class="{ active: page === item.key }"
            type="button"
            @click="page = item.key as PageKey"
          >
            <el-icon :size="18"><component :is="item.icon" /></el-icon>
            <span>{{ i18n(item.labelKey) }}</span>
          </button>
        </nav>

        <div class="side-foot">
          <div class="status-chip" :class="{ running }">
            <span class="dot" />
            {{ running ? i18n("app.running") : i18n("app.stopped") }}
          </div>
          <button class="theme-btn" type="button" @click="toggleTheme">
            <el-icon :size="16"><Moon v-if="theme === 'light'" /><Sunny v-else /></el-icon>
            {{ theme === "light" ? i18n("app.darkMode") : i18n("app.lightMode") }}
          </button>
          <div class="version">v{{ state.version.version || "1.0.0" }}</div>
        </div>
      </aside>

      <div class="main">
        <HomeView v-if="page === 'home'" @navigate="page = $event" />
        <MountsView v-else-if="page === 'mounts'" />
        <UsersView v-else-if="page === 'users'" />
        <SharesView v-else-if="page === 'shares'" />
        <LogsView v-else-if="page === 'logs'" />
        <AboutView v-else-if="page === 'about'" />
        <SettingsView v-else />
      </div>

      <UpdateDialog />
    </div>
  </el-config-provider>
</template>
