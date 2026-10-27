import { createApp } from "vue";
import ElementPlus from "element-plus";
import "element-plus/dist/index.css";
import "element-plus/theme-chalk/dark/css-vars.css";
import App from "./App.vue";
import "./styles.css";
import { initTheme } from "./store";

initTheme();

createApp(App).use(ElementPlus).mount("#app");
