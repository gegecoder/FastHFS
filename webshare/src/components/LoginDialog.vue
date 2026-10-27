<script setup lang="ts">
import { nextTick, onMounted, ref } from "vue";
import { api, errorMessage } from "../api";
import { i18n } from "../i18n";
import type { AuthInfo } from "../types";
import ModalShell from "./ModalShell.vue";

withDefaults(defineProps<{ required?: boolean }>(), { required: false });

const emit = defineEmits<{ success: [info: AuthInfo]; close: [] }>();

const username = ref("");
const password = ref("");
const loading = ref(false);
const error = ref("");
const usernameRef = ref<HTMLInputElement | null>(null);
const passwordRef = ref<HTMLInputElement | null>(null);

onMounted(() => {
  nextTick(() => usernameRef.value?.focus());
});

async function submit(): Promise<void> {
  if (loading.value) {
    return;
  }
  const name = username.value.trim();
  if (!name || !password.value) {
    error.value = i18n("login.required");
    return;
  }
  loading.value = true;
  error.value = "";
  try {
    const info = await api.login({ username: name, password: password.value });
    emit("success", info);
  } catch (err) {
    error.value = errorMessage(err);
    password.value = "";
    nextTick(() => passwordRef.value?.focus());
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <ModalShell
    :title="i18n('login.title')"
    :subtitle="required ? i18n('login.subtitleRequired') : i18n('login.subtitleOptional')"
    @close="emit('close')"
  >
    <form @submit.prevent="submit">
      <div class="modal-body">
        <div class="login-badge">
          <svg width="22" height="22" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <rect x="4.5" y="10.5" width="15" height="9.5" rx="2.2" stroke="currentColor" stroke-width="1.6" />
            <path d="M8 10.5V8a4 4 0 0 1 8 0v2.5" stroke="currentColor" stroke-width="1.6" />
            <circle cx="12" cy="15.2" r="1.2" fill="currentColor" />
          </svg>
          <span>{{ i18n("login.badge") }}</span>
        </div>
        <label class="form-row">
          <span class="form-label">{{ i18n("login.username") }}</span>
          <input
            ref="usernameRef"
            v-model="username"
            class="input"
            type="text"
            autocomplete="username"
            spellcheck="false"
            :placeholder="i18n('login.usernamePlaceholder')"
            @input="error = ''"
          />
        </label>
        <label class="form-row">
          <span class="form-label">{{ i18n("login.password") }}</span>
          <input
            ref="passwordRef"
            v-model="password"
            class="input"
            type="password"
            autocomplete="current-password"
            :placeholder="i18n('login.passwordPlaceholder')"
            @input="error = ''"
          />
        </label>
        <p v-if="error" class="form-error">{{ error }}</p>
      </div>
      <div class="modal-footer">
        <button type="button" class="btn" @click="emit('close')">{{ i18n("common.cancel") }}</button>
        <button type="submit" class="btn btn-primary" :disabled="loading">
          <span v-if="loading" class="spinner" style="width: 14px; height: 14px" />
          {{ loading ? i18n("login.submitting") : i18n("login.submit") }}
        </button>
      </div>
    </form>
  </ModalShell>
</template>

<style scoped>
.login-badge {
  display: flex;
  align-items: center;
  gap: 9px;
  margin-bottom: 16px;
  padding: 10px 12px;
  border-radius: 10px;
  background: var(--primary-soft);
  color: var(--text-2);
  font-size: 12.5px;
}

.login-badge svg {
  color: var(--primary);
  flex: none;
}
</style>
