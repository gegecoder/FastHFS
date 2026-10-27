<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import HomeView from "./views/HomeView.vue";
import ShareView from "./views/ShareView.vue";
import ToastHost from "./components/ToastHost.vue";
import { BASE_PATH } from "./api";
import { i18n } from "./i18n";

function parseShareToken(): string | null {
  const pathname = BASE_PATH && window.location.pathname.startsWith(BASE_PATH)
    ? window.location.pathname.slice(BASE_PATH.length)
    : window.location.pathname;
  const match = pathname.match(/^\/s\/([^/]+)\/?$/);
  if (!match) {
    return null;
  }
  try {
    return decodeURIComponent(match[1]);
  } catch {
    return match[1];
  }
}

const shareToken = ref<string | null>(parseShareToken());

function syncRoute(): void {
  shareToken.value = parseShareToken();
}

watch(shareToken, (token) => {
  document.title = token ? i18n("app.shareTitle") : i18n("app.title");
});

onMounted(() => {
  window.addEventListener("popstate", syncRoute);
  document.title = shareToken.value ? i18n("app.shareTitle") : i18n("app.title");
});

onBeforeUnmount(() => {
  window.removeEventListener("popstate", syncRoute);
});
</script>

<template>
  <HomeView v-if="!shareToken" />
  <ShareView v-else :key="shareToken" :token="shareToken" />
  <ToastHost />
</template>
