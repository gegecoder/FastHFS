<script setup lang="ts">
import { computed } from "vue";
import { Download } from "@element-plus/icons-vue";
import * as Api from "../../bindings/FastHFS/app";
import { useUpdate } from "../composables/useUpdate";
import { i18n } from "../i18n";

const { updateInfo, showUpdateModal } = useUpdate();

const info = computed(() => updateInfo.value);

function close() {
  showUpdateModal.value = false;
}

function onVisibleChange(visible: boolean) {
  if (!visible) close();
}

function openURL(url?: string) {
  if (url) void Api.OpenURL(url);
}

function download() {
  openURL(info.value?.downloadUrl || info.value?.releaseUrl);
}

function openReleasePage() {
  openURL(info.value?.releaseUrl);
}
</script>

<template>
  <el-dialog
    :model-value="showUpdateModal && !!info"
    width="460px"
    :title="i18n('update.title')"
    class="update-dialog"
    @update:model-value="onVisibleChange"
  >
    <div v-if="info" class="update-body">
      <div class="update-versions">
        <span class="ver-old">v{{ info.currentVersion }}</span>
        <span class="ver-arrow">→</span>
        <span class="ver-new">v{{ info.latestVersion }}</span>
        <span v-if="info.prerelease" class="ver-beta">{{ i18n("update.prerelease") }}</span>
      </div>
      <p v-if="info.publishedAt" class="update-date">
        {{ i18n("update.publishedAt", { date: info.publishedAt }) }}
      </p>

      <div v-if="info.releaseNotes" class="update-notes">
        <p class="notes-title">{{ i18n("update.notes") }}</p>
        <pre class="notes-body">{{ info.releaseNotes }}</pre>
      </div>

      <div class="update-actions">
        <el-button @click="close">{{ i18n("update.later") }}</el-button>
        <el-button type="primary" :icon="Download" @click="download">{{ i18n("update.download") }}</el-button>
      </div>
      <a class="update-page" href="javascript:void(0)" @click="openReleasePage">{{ i18n("update.viewPage") }}</a>
    </div>
  </el-dialog>
</template>

<style scoped>
.update-body {
  text-align: center;
}

.update-versions {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  margin-bottom: 4px;
}

.ver-old,
.ver-new {
  padding: 3px 12px;
  border-radius: 10px;
  font-size: 13px;
  font-weight: 700;
  font-family: Consolas, "SFMono-Regular", Menlo, monospace;
}

.ver-old {
  background: var(--panel-soft);
  color: var(--muted);
  border: 1px solid var(--border);
}

.ver-new {
  background: var(--primary-soft);
  color: var(--primary);
  border: 1px solid var(--border);
}

.ver-arrow {
  color: var(--muted);
}

.ver-beta {
  padding: 2px 8px;
  border-radius: 8px;
  font-size: 11px;
  font-weight: 700;
  color: #b45309;
  background: rgba(217, 119, 6, 0.12);
}

.update-date {
  margin: 0 0 12px;
  font-size: 12px;
  color: var(--muted);
}

.update-notes {
  margin: 0 0 16px;
  text-align: left;
  border: 1px solid var(--border);
  border-radius: 10px;
  overflow: hidden;
}

.notes-title {
  margin: 0;
  padding: 8px 12px;
  font-size: 12.5px;
  font-weight: 600;
  border-bottom: 1px solid var(--border);
}

.notes-body {
  margin: 0;
  padding: 10px 12px;
  max-height: 200px;
  overflow-y: auto;
  font-size: 12.5px;
  line-height: 1.7;
  color: var(--muted);
  white-space: pre-wrap;
  word-break: break-word;
  font-family: inherit;
}

.update-actions {
  display: flex;
  gap: 10px;
  justify-content: center;
}

.update-actions .el-button {
  flex: 1;
}

.update-page {
  display: inline-block;
  margin-top: 12px;
  font-size: 12px;
  color: var(--primary);
  text-decoration: none;
}

.update-page:hover {
  text-decoration: underline;
}
</style>
