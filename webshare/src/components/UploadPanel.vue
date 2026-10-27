<script setup lang="ts">
import { computed, ref } from "vue";
import type { UploadTask } from "../composables/useUploader";
import { formatSize, formatSpeed, formatTime } from "../utils";
import { i18n } from "../i18n";
import FileIcon from "./FileIcon.vue";

const props = defineProps<{
  tasks: UploadTask[];
  uploadingCount: number;
  finishedCount: number;
  totalProgress: number;
}>();

const emit = defineEmits<{
  cancel: [task: UploadTask];
  retry: [task: UploadTask];
  remove: [task: UploadTask];
  clear: [];
}>();

const collapsed = ref(false);

const summary = computed(() => {
  if (props.uploadingCount > 0) {
    return i18n("upload.uploading", { n: props.uploadingCount });
  }
  const failed = props.tasks.filter((task) => task.status === "error").length;
  if (failed > 0) {
    return i18n("upload.failed", { n: failed });
  }
  if (props.finishedCount > 0) {
    return i18n("upload.done");
  }
  return i18n("upload.queue");
});

const overallText = computed(() => `${Math.round(props.totalProgress)}%`);

function statusText(task: UploadTask): string {
  switch (task.status) {
    case "queued":
      return i18n("upload.statusQueued");
    case "uploading":
      return `${Math.round(task.progress)}% · ${formatSpeed(task.speed)}`;
    case "success":
      return i18n("upload.statusSuccess");
    case "error":
      return task.error || i18n("upload.statusError");
    case "canceled":
      return i18n("upload.statusCanceled");
    default:
      return "";
  }
}

function taskPercent(task: UploadTask): number {
  if (task.status === "success") {
    return 100;
  }
  return Math.max(0, Math.min(100, task.progress));
}
</script>

<template>
  <Teleport to="body">
    <section class="upload-panel" :class="{ collapsed }" :aria-label="i18n('upload.queueLabel')">
      <header class="panel-header" @click="collapsed = !collapsed">
        <span class="panel-title">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path d="M12 16V4.5M8 8l4-3.8L16 8" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" />
            <path d="M4.8 15.2v2.6A2.2 2.2 0 0 0 7 20h10a2.2 2.2 0 0 0 2.2-2.2v-2.6" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" />
          </svg>
          {{ summary }}
        </span>
        <span class="panel-progress-text">{{ overallText }}</span>
        <span class="panel-toggle" :class="{ rotated: collapsed }" aria-hidden="true">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none">
            <path d="M6 9.5l6 6 6-6" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
          </svg>
        </span>
      </header>
      <div class="overall-track" :class="{ 'is-idle': uploadingCount === 0 }">
        <span class="overall-fill" :style="{ width: `${totalProgress}%` }" />
      </div>

      <div v-show="!collapsed" class="panel-body">
        <div class="task-list">
          <div v-for="task in tasks" :key="task.id" class="task" :class="`task-${task.status}`">
            <div class="task-main">
              <FileIcon :name="task.name" :is-dir="false" :size="20" />
              <div class="task-text">
                <div class="task-line">
                  <span class="task-name" :title="task.dir ? `${task.dir}/${task.name}` : task.name">{{ task.name }}</span>
                  <span class="task-percent">{{ task.status === "uploading" ? `${Math.round(task.progress)}%` : "" }}</span>
                </div>
                <div class="task-meta">
                  <span>{{ formatSize(task.size) }}</span>
                  <span v-if="task.status === 'uploading'">{{ formatSpeed(task.speed) }}</span>
                  <span class="task-status" :title="statusText(task)">{{ statusText(task) }}</span>
                </div>
                <div class="task-track">
                  <span class="task-fill" :style="{ width: `${taskPercent(task)}%` }" />
                </div>
              </div>
              <div class="task-actions">
                <button
                  v-if="task.status === 'error' || task.status === 'canceled'"
                  type="button"
                  class="btn btn-sm"
                  @click.stop="emit('retry', task)"
                >
                  {{ i18n("common.retry") }}
                </button>
                <button
                  v-if="task.status === 'queued' || task.status === 'uploading'"
                  type="button"
                  class="btn btn-ghost btn-sm icon-btn"
                  :title="i18n('upload.cancel')"
                  :aria-label="i18n('upload.cancel')"
                  @click.stop="emit('cancel', task)"
                >
                  <svg width="15" height="15" viewBox="0 0 24 24" fill="none">
                    <path d="M6 6l12 12M18 6L6 18" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
                  </svg>
                </button>
                <button
                  v-else
                  type="button"
                  class="btn btn-ghost btn-sm icon-btn"
                  :title="i18n('upload.remove')"
                  :aria-label="i18n('upload.remove')"
                  @click.stop="emit('remove', task)"
                >
                  <svg width="15" height="15" viewBox="0 0 24 24" fill="none">
                    <path d="M6 6l12 12M18 6L6 18" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
                  </svg>
                </button>
              </div>
            </div>
          </div>
        </div>
        <div class="panel-footer">
          <span class="panel-count">{{ i18n("upload.tasksCount", { n: tasks.length }) }}</span>
          <button v-if="finishedCount > 0" type="button" class="btn btn-ghost btn-sm" @click="emit('clear')">{{ i18n("upload.clearFinished") }}</button>
        </div>
      </div>
    </section>
  </Teleport>
</template>

<style scoped>
.upload-panel {
  position: fixed;
  right: 18px;
  bottom: 18px;
  z-index: 900;
  width: min(370px, calc(100vw - 32px));
  background: var(--bg-elev);
  border: 1px solid var(--border);
  border-radius: 14px;
  box-shadow: var(--shadow-lg);
  overflow: hidden;
  animation: panel-in 0.22s cubic-bezier(0.2, 0.8, 0.2, 1);
}

@keyframes panel-in {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.panel-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 11px 13px 9px;
  cursor: pointer;
  user-select: none;
}

.panel-title {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  flex: 1;
  min-width: 0;
  font-size: 13px;
  font-weight: 600;
}

.panel-title svg {
  color: var(--primary);
}

.panel-progress-text {
  color: var(--text-2);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}

.panel-toggle {
  display: inline-flex;
  color: var(--text-3);
  transition: transform 0.2s ease;
}

.panel-toggle.rotated {
  transform: rotate(180deg);
}

.overall-track {
  height: 3px;
  background: var(--bg-subtle);
}

.overall-fill {
  display: block;
  height: 100%;
  background: var(--primary);
  transition: width 0.25s ease;
}

.overall-track.is-idle .overall-fill {
  background: var(--success);
}

.panel-body {
  border-top: 1px solid var(--border);
}

.task-list {
  max-height: min(340px, 45vh);
  overflow-y: auto;
  padding: 4px 0;
}

.task {
  padding: 8px 13px;
}

.task:hover {
  background: var(--bg-hover);
}

.task-main {
  display: flex;
  align-items: flex-start;
  gap: 9px;
}

.task-text {
  flex: 1;
  min-width: 0;
}

.task-line {
  display: flex;
  align-items: center;
  gap: 8px;
}

.task-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 12.5px;
  font-weight: 500;
}

.task-percent {
  flex: none;
  color: var(--text-2);
  font-size: 11.5px;
  font-variant-numeric: tabular-nums;
}

.task-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 2px;
  color: var(--text-3);
  font-size: 11px;
}

.task-status {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: right;
}

.task-success .task-status {
  color: var(--success);
}

.task-error .task-status {
  color: var(--danger);
}

.task-track {
  height: 4px;
  margin-top: 6px;
  border-radius: 99px;
  background: var(--bg-subtle);
  overflow: hidden;
}

.task-fill {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--primary);
  transition: width 0.2s ease;
}

.task-success .task-fill {
  background: var(--success);
}

.task-error .task-fill {
  background: var(--danger);
}

.task-canceled .task-fill {
  background: var(--text-3);
}

.task-actions {
  display: flex;
  align-items: center;
  gap: 2px;
  flex: none;
  margin-top: -2px;
}

.panel-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 13px 9px;
  border-top: 1px solid var(--border);
}

.panel-count {
  color: var(--text-3);
  font-size: 11.5px;
}

@media (max-width: 640px) {
  .upload-panel {
    left: 0;
    right: 0;
    bottom: 0;
    width: 100%;
    border-radius: 16px 16px 0 0;
    border-bottom: 0;
    padding-bottom: env(safe-area-inset-bottom);
  }

  .task-list {
    max-height: 40vh;
  }
}
</style>
