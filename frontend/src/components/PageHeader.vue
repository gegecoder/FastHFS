<script setup lang="ts">
defineProps<{
  title: string;
  desc?: string;
}>();
</script>

<template>
  <header class="page-header">
    <div class="page-header-title">
      <h2>{{ title }}</h2>
      <p v-if="desc">{{ desc }}</p>
    </div>
    <div class="page-header-actions">
      <slot />
    </div>
  </header>
</template>

<style scoped>
.page-header {
  flex: 0 0 50px;
  height: 50px;
  padding: 0 26px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  border-bottom: 1px solid var(--border);
  background: color-mix(in srgb, var(--panel) 82%, transparent);
  backdrop-filter: blur(10px);
}

.page-header-title {
  display: flex;
  align-items: baseline;
  gap: 10px;
  min-width: 0;
}

.page-header-title h2 {
  margin: 0;
  font-size: 15px;
  letter-spacing: 0.2px;
  white-space: nowrap;
}

.page-header-title p {
  margin: 0;
  font-size: 12px;
  color: var(--muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.page-header-actions {
  flex: none;
  display: flex;
  align-items: center;
  gap: 8px;
}

@media (max-width: 820px) {
  .page-header {
    padding: 0 16px;
  }

  .page-header-title p {
    display: none;
  }
}
</style>
