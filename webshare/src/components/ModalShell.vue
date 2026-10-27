<script setup lang="ts">
import { onBeforeUnmount, onMounted } from "vue";
import { i18n } from "../i18n";

const props = withDefaults(
  defineProps<{
    title?: string;
    subtitle?: string;
    wide?: boolean;
    closable?: boolean;
  }>(),
  { title: "", subtitle: "", wide: false, closable: true },
);

const emit = defineEmits<{ close: [] }>();

function onKeydown(event: KeyboardEvent): void {
  if (event.key === "Escape" && props.closable) {
    emit("close");
  }
}

function onOverlay(event: MouseEvent): void {
  if (props.closable && event.target === event.currentTarget) {
    emit("close");
  }
}

onMounted(() => {
  document.addEventListener("keydown", onKeydown);
  document.body.classList.add("modal-open");
});

onBeforeUnmount(() => {
  document.removeEventListener("keydown", onKeydown);
  document.body.classList.remove("modal-open");
});
</script>

<template>
  <Teleport to="body">
    <div class="modal-overlay" role="presentation" @mousedown="onOverlay">
      <div class="modal-card" :class="{ 'modal-wide': wide }" role="dialog" aria-modal="true">
        <div v-if="title || $slots.header" class="modal-header">
          <div class="modal-heading">
            <h2 class="modal-title">{{ title }}</h2>
            <p v-if="subtitle" class="modal-subtitle">{{ subtitle }}</p>
          </div>
          <slot name="header-extra" />
          <button
            v-if="closable"
            type="button"
            class="btn btn-ghost icon-btn"
            :aria-label="i18n('common.close')"
            @click="emit('close')"
          >
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <path d="M6 6l12 12M18 6L6 18" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
            </svg>
          </button>
        </div>
        <slot />
      </div>
    </div>
  </Teleport>
</template>
