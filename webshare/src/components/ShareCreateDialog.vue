<script setup lang="ts">
import { computed, ref } from "vue";
import { api, errorMessage } from "../api";
import { useToast } from "../composables/useToast";
import type { ShareCreated, ShareMode } from "../types";
import { copyText, formatDuration, formatTime } from "../utils";
import { i18n } from "../i18n";
import ModalShell from "./ModalShell.vue";

const props = defineProps<{
  mount: string;
  path: string;
  name: string;
  isDir: boolean;
}>();

const emit = defineEmits<{ close: [] }>();
const toast = useToast();

const password = ref("");
const expiresIn = ref(0);
const maxVisits = ref(0);
const mode = ref<ShareMode>(props.isDir ? "download" : "preview");
const loading = ref(false);
const error = ref("");
const result = ref<ShareCreated | null>(null);

const modeOptions = computed(() => {
  const options: { value: ShareMode; label: string; hint: string }[] = [
    { value: "preview", label: i18n("shareCreate.modePreview"), hint: i18n("shareCreate.modePreviewHint") },
    { value: "download", label: i18n("shareCreate.modeDownload"), hint: i18n("shareCreate.modeDownloadHint") },
  ];
  if (props.isDir) {
    options.push({ value: "collect", label: i18n("shareCreate.modeCollect"), hint: i18n("shareCreate.modeCollectHint") });
  }
  return options;
});

const modeHint = computed(() => modeOptions.value.find((item) => item.value === mode.value)?.hint ?? "");

async function submit(): Promise<void> {
  if (loading.value) {
    return;
  }
  const hours = Number(expiresIn.value);
  const visits = Number(maxVisits.value);
  if (!Number.isFinite(hours) || hours < 0) {
    error.value = i18n("shareCreate.errExpires");
    return;
  }
  if (!Number.isFinite(visits) || visits < 0) {
    error.value = i18n("shareCreate.errVisits");
    return;
  }
  loading.value = true;
  error.value = "";
  try {
    result.value = await api.createShare({
      mount: props.mount,
      path: props.path,
      password: password.value.trim(),
      expiresIn: Math.floor(hours),
      maxVisits: Math.floor(visits),
      mode: mode.value,
    });
  } catch (err) {
    error.value = errorMessage(err);
  } finally {
    loading.value = false;
  }
}

async function copyLink(): Promise<void> {
  if (!result.value) {
    return;
  }
  const ok = await copyText(result.value.url);
  if (ok) {
    toast.success(i18n("shareCreate.copied"));
  } else {
    toast.error(i18n("shareCreate.copyFailed"));
  }
}

function expiresText(expiresAt: number): string {
  return expiresAt > 0 ? formatTime(expiresAt) : i18n("utils.forever");
}
</script>

<template>
  <ModalShell
    :title="result ? i18n('shareCreate.created') : i18n('shareCreate.title')"
    :subtitle="result ? result.share.mountName : `${name}${isDir ? i18n('shareCreate.dirSuffix') : ''}`"
    @close="emit('close')"
  >
    <div v-if="result" class="modal-body">
      <div class="share-success">
        <div class="success-mark" aria-hidden="true">
          <svg width="22" height="22" viewBox="0 0 24 24" fill="none">
            <circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="1.7" />
            <path d="M8.2 12.4l2.6 2.6 5-5.4" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round" />
          </svg>
        </div>
        <div>
          <p class="success-title">{{ i18n("shareCreate.createdHint") }}</p>
          <p class="success-sub">
            {{ modeOptions.find((item) => item.value === result?.share.mode)?.label ?? result.share.mode }}
            · {{ expiresText(result.share.expiresAt) }}
            · {{ result.share.maxVisits > 0 ? i18n("shareCreate.visitsLimited", { n: result.share.maxVisits }) : i18n("shareCreate.visitsUnlimited") }}
            · {{ result.share.hasPassword ? i18n("shareCreate.passwordYes") : i18n("shareCreate.passwordNo") }}
          </p>
        </div>
      </div>
      <div class="link-box">
        <input class="input" :value="result.url" readonly @focus="($event.target as HTMLInputElement).select()" />
        <button type="button" class="btn btn-primary" @click="copyLink">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <rect x="8.5" y="8.5" width="11" height="11" rx="2" stroke="currentColor" stroke-width="1.6" />
            <path d="M15.5 5.5H6.2A1.7 1.7 0 0 0 4.5 7.2v9.3" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" />
          </svg>
          {{ i18n("common.copy") }}
        </button>
      </div>
      <p v-if="result.share.hasPassword" class="form-hint" style="margin-top: 8px">{{ i18n("shareCreate.passwordNote") }}</p>
    </div>
    <div v-else class="modal-body">
      <div class="form-row">
        <span class="form-label">{{ i18n("shareCreate.mode") }}</span>
        <select v-model="mode" class="input">
          <option v-for="option in modeOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
        </select>
        <span class="form-hint">{{ modeHint }}</span>
      </div>
      <div class="form-grid">
        <label class="form-row">
          <span class="form-label">{{ i18n("shareCreate.expiresHours") }}</span>
          <input v-model.number="expiresIn" class="input" type="number" min="0" step="1" inputmode="numeric" />
          <span class="form-hint">{{ formatDuration(Number(expiresIn) || 0) }}</span>
        </label>
        <label class="form-row">
          <span class="form-label">{{ i18n("shareCreate.visits") }}</span>
          <input v-model.number="maxVisits" class="input" type="number" min="0" step="1" inputmode="numeric" />
          <span class="form-hint">{{ Number(maxVisits) > 0 ? i18n("shareCreate.visitsMax", { n: maxVisits }) : i18n("shareCreate.visitsZeroHint") }}</span>
        </label>
      </div>
      <label class="form-row">
        <span class="form-label">{{ i18n("shareCreate.passwordLabel") }}</span>
        <input v-model="password" class="input" type="text" autocomplete="off" :placeholder="i18n('shareCreate.passwordPlaceholder')" />
      </label>
      <p v-if="error" class="form-error">{{ error }}</p>
    </div>
    <div class="modal-footer">
      <button v-if="result" type="button" class="btn" @click="emit('close')">{{ i18n("common.close") }}</button>
      <template v-else>
        <button type="button" class="btn" @click="emit('close')">{{ i18n("common.cancel") }}</button>
        <button type="button" class="btn btn-primary" :disabled="loading" @click="submit">
          <span v-if="loading" class="spinner" style="width: 14px; height: 14px" />
          {{ loading ? i18n("shareCreate.creating") : i18n("shareCreate.create") }}
        </button>
      </template>
    </div>
  </ModalShell>
</template>

<style scoped>
.share-success {
  display: flex;
  gap: 11px;
  margin-bottom: 14px;
  padding: 12px;
  border-radius: 10px;
  background: var(--success-soft);
}

.success-mark {
  color: var(--success);
  flex: none;
  margin-top: 1px;
}

.success-title {
  font-size: 13.5px;
  font-weight: 600;
}

.success-sub {
  margin-top: 3px;
  color: var(--text-2);
  font-size: 12px;
}

.link-box {
  display: flex;
  gap: 8px;
}

.link-box .input {
  flex: 1;
  font-size: 12.5px;
}

@media (max-width: 520px) {
  .link-box {
    flex-direction: column;
  }
}
</style>
