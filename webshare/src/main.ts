import { createApp } from "vue";
import App from "./App.vue";
import "./style.css";
import { i18n, resolveLocale, setLocale } from "./i18n";

// 语言由服务端注入（window.__FASTHFS_LANG__），缺失或非法时回退中文
const locale = resolveLocale(window.__FASTHFS_LANG__);
setLocale(locale);
document.documentElement.lang = locale === "en" ? "en" : "zh-CN";
document.title = i18n("app.title");

createApp(App).mount("#app");
