<script setup lang="ts">
import { computed } from "vue";
import { splitPath } from "../utils";
import { i18n } from "../i18n";

const props = defineProps<{
  rootLabel: string;
  path: string;
}>();

const emit = defineEmits<{ navigate: [path: string] }>();

const segments = computed(() => {
  const parts = splitPath(props.path);
  let current = "";
  return parts.map((label) => {
    current = current ? `${current}/${label}` : label;
    return { label, path: current };
  });
});
</script>

<template>
  <nav class="breadcrumb" :aria-label="i18n('breadcrumb.label')">
    <button type="button" class="crumb" :class="{ current: !path }" :title="rootLabel" @click="emit('navigate', '')">
      <svg width="15" height="15" viewBox="0 0 24 24" fill="none" aria-hidden="true">
        <path
          d="M3.6 10.4L12 3.4l8.4 7v8.3a1.9 1.9 0 0 1-1.9 1.9h-3.6v-5.2H9.1v5.2H5.5a1.9 1.9 0 0 1-1.9-1.9z"
          stroke="currentColor"
          stroke-width="1.6"
          stroke-linejoin="round"
        />
      </svg>
      <span>{{ rootLabel }}</span>
    </button>
    <template v-for="segment in segments" :key="segment.path">
      <span class="sep" aria-hidden="true">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none">
          <path d="M9 5.5l6.5 6.5L9 18.5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
      </span>
      <button type="button" class="crumb" :class="{ current: segment.path === path }" @click="emit('navigate', segment.path)">
        {{ segment.label }}
      </button>
    </template>
  </nav>
</template>

<style scoped>
.breadcrumb {
  display: flex;
  align-items: center;
  gap: 2px;
  min-width: 0;
  overflow-x: auto;
  scrollbar-width: none;
  font-size: 13px;
}

.breadcrumb::-webkit-scrollbar {
  display: none;
}

.crumb {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  flex: none;
  max-width: 220px;
  padding: 4px 7px;
  border: 0;
  border-radius: 7px;
  background: transparent;
  color: var(--text-2);
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  transition: background-color 0.12s ease, color 0.12s ease;
}

.crumb:hover {
  background: var(--bg-hover);
  color: var(--text);
}

.crumb.current {
  color: var(--text);
  font-weight: 600;
}

.sep {
  display: inline-flex;
  flex: none;
  color: var(--text-3);
  opacity: 0.7;
}

@media (max-width: 640px) {
  .crumb {
    max-width: 150px;
  }
}
</style>
