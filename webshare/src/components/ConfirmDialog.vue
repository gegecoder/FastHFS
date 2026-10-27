<script setup lang="ts">
import ModalShell from "./ModalShell.vue";
import { i18n } from "../i18n";

withDefaults(
  defineProps<{
    title: string;
    message: string;
    confirmText?: string;
    danger?: boolean;
    busy?: boolean;
  }>(),
  { confirmText: i18n("common.confirm"), danger: false, busy: false },
);

const emit = defineEmits<{ confirm: []; close: [] }>();
</script>

<template>
  <ModalShell :title="title" @close="emit('close')">
    <div class="modal-body">
      <p class="message">{{ message }}</p>
    </div>
    <div class="modal-footer">
      <button type="button" class="btn" :disabled="busy" @click="emit('close')">{{ i18n("common.cancel") }}</button>
      <button
        type="button"
        class="btn"
        :class="danger ? 'btn-danger' : 'btn-primary'"
        :disabled="busy"
        @click="emit('confirm')"
      >
        <span v-if="busy" class="spinner" style="width: 14px; height: 14px" />
        {{ confirmText }}
      </button>
    </div>
  </ModalShell>
</template>

<style scoped>
.message {
  color: var(--text-2);
  font-size: 13.5px;
  line-height: 1.65;
  word-break: break-word;
}
</style>
