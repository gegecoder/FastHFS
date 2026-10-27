<script setup lang="ts">
import { nextTick, onMounted, ref } from "vue";
import ModalShell from "./ModalShell.vue";
import { i18n } from "../i18n";

const props = withDefaults(
  defineProps<{
    title: string;
    label?: string;
    initial?: string;
    placeholder?: string;
    confirmText?: string;
    selectBaseName?: boolean;
  }>(),
  {
    label: i18n("inputDialog.label"),
    initial: "",
    placeholder: "",
    confirmText: i18n("common.confirm"),
    selectBaseName: false,
  },
);

const emit = defineEmits<{ confirm: [value: string]; close: [] }>();

const value = ref(props.initial);
const error = ref("");
const inputRef = ref<HTMLInputElement | null>(null);

onMounted(() => {
  nextTick(() => {
    const input = inputRef.value;
    if (!input) {
      return;
    }
    input.focus();
    if (props.selectBaseName) {
      const dot = input.value.lastIndexOf(".");
      input.setSelectionRange(0, dot > 0 ? dot : input.value.length);
    } else {
      input.select();
    }
  });
});

function submit(): void {
  const name = value.value.trim();
  if (!name) {
    error.value = i18n("inputDialog.nameEmpty");
    return;
  }
  if (name.includes("/") || name.includes("\\")) {
    error.value = i18n("inputDialog.invalidSeparator");
    return;
  }
  if (name === "." || name === "..") {
    error.value = i18n("inputDialog.invalidName");
    return;
  }
  error.value = "";
  emit("confirm", name);
}
</script>

<template>
  <ModalShell :title="title" @close="emit('close')">
    <form @submit.prevent="submit">
      <div class="modal-body">
        <label class="form-row">
          <span class="form-label">{{ label }}</span>
          <input
            ref="inputRef"
            v-model="value"
            class="input"
            type="text"
            autocomplete="off"
            spellcheck="false"
            :placeholder="placeholder"
            @input="error = ''"
          />
        </label>
        <p v-if="error" class="form-error">{{ error }}</p>
      </div>
      <div class="modal-footer">
        <button type="button" class="btn" @click="emit('close')">{{ i18n("common.cancel") }}</button>
        <button type="submit" class="btn btn-primary">{{ confirmText }}</button>
      </div>
    </form>
  </ModalShell>
</template>
