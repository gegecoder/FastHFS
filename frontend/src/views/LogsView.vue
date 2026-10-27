<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from "vue";
import { Delete, Document, FolderOpened, Refresh } from "@element-plus/icons-vue";
import { ElMessageBox } from "element-plus";
import * as Api from "../../bindings/FastHFS/app";
import type { Entry } from "../../bindings/FastHFS/backend/applog/models";
import { guard, toastErr, toastOk } from "../ui";
import { i18n } from "../i18n";
import PageHeader from "../components/PageHeader.vue";

const levels = ["all", "info", "warn", "error"] as const;

const levelKeys: Record<string, string> = {
  all: "logs.all",
  info: "logs.info",
  warn: "logs.warn",
  error: "logs.error",
};

function levelLabel(value: string): string {
  const key = levelKeys[value];
  return key ? i18n(key) : value;
}

const logs = ref<Entry[]>([]);
const loading = ref(false);
const clearing = ref(false);
const level = ref("all");
const autoScroll = ref(true);
const listRef = ref<HTMLElement | null>(null);

const filtered = computed(() =>
  level.value === "all" ? logs.value : logs.value.filter((item) => item.level === level.value),
);

function scrollToBottom() {
  const el = listRef.value;
  if (el) el.scrollTop = el.scrollHeight;
}

/** silent 用于定时刷新：失败时不打扰用户 */
async function fetchLogs(silent = false) {
  try {
    const list = await Api.GetRecentLogs();
    logs.value = list || [];
    await nextTick();
    if (autoScroll.value) scrollToBottom();
  } catch (error) {
    if (!silent) toastErr(error, i18n("logs.readFailed"));
  }
}

async function refresh() {
  loading.value = true;
  try {
    await fetchLogs();
  } finally {
    loading.value = false;
  }
}

async function openDir() {
  const result = await guard(() => Api.OpenLogDir());
  if (result && !result.success) toastErr(result.message || i18n("logs.openDirFailed"));
}

async function clearLogs() {
  try {
    await ElMessageBox.confirm(i18n("logs.clearConfirm"), i18n("logs.clearTitle"), {
      type: "warning",
      confirmButtonText: i18n("common.clear"),
      cancelButtonText: i18n("common.cancel"),
    });
  } catch {
    return;
  }
  clearing.value = true;
  try {
    await Api.ClearLogs();
    logs.value = [];
    toastOk(i18n("logs.cleared"));
  } catch (error) {
    toastErr(error, i18n("logs.clearFailed"));
  } finally {
    clearing.value = false;
  }
}

let timer: number | undefined;

onMounted(() => {
  fetchLogs();
  timer = window.setInterval(() => fetchLogs(true), 2000);
});

onUnmounted(() => {
  if (timer !== undefined) window.clearInterval(timer);
});
</script>

<template>
  <div class="page">
    <PageHeader :title="i18n('logs.title')" :desc="i18n('logs.desc')">
      <el-button :icon="Refresh" :loading="loading" @click="refresh">{{ i18n("common.refresh") }}</el-button>
      <el-button :icon="FolderOpened" @click="openDir">{{ i18n("logs.openDir") }}</el-button>
      <el-button type="danger" plain :icon="Delete" :loading="clearing" @click="clearLogs">
        {{ i18n("common.clear") }}
      </el-button>
    </PageHeader>

    <main class="content">
      <div class="page-stack">
        <section class="panel">
          <div class="panel-head">
            <div>
              <h3>{{ i18n("logs.panelTitle") }}</h3>
              <p class="sub">
                <el-icon><Document /></el-icon>
                {{ i18n("logs.summary", { n: logs.length }) }}
              </p>
            </div>
            <div class="log-tools">
              <el-radio-group v-model="level" size="small">
                <el-radio-button v-for="item in levels" :key="item" :value="item">
                  {{ levelLabel(item) }}
                </el-radio-button>
              </el-radio-group>
              <el-switch
                v-model="autoScroll"
                inline-prompt
                :active-text="i18n('logs.autoScroll')"
                :inactive-text="i18n('logs.manual')"
              />
            </div>
          </div>

          <div ref="listRef" class="log-list">
            <div v-for="(item, index) in filtered" :key="index" class="log-line" :class="item.level">
              <span class="log-time">{{ item.time }}</span>
              <span class="log-level">{{ levelLabel(item.level) }}</span>
              <span class="log-msg">{{ item.message }}</span>
            </div>
            <div v-if="!filtered.length" class="log-empty">
              {{ logs.length ? i18n("logs.emptyFiltered") : i18n("logs.empty") }}
            </div>
          </div>
        </section>
      </div>
    </main>
  </div>
</template>

<style scoped>
.log-tools {
  display: flex;
  align-items: center;
  gap: 12px;
}

.log-list {
  min-height: 220px;
  max-height: calc(100vh - 300px);
  overflow-y: auto;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--panel-soft);
  font-family: "JetBrains Mono", Consolas, monospace;
  font-size: 12px;
  line-height: 1.7;
}

.log-line {
  display: flex;
  gap: 10px;
  padding: 1px 0;
}

.log-time {
  flex: none;
  color: var(--muted);
}

.log-level {
  flex: none;
  width: 38px;
  height: 18px;
  margin-top: 2px;
  border-radius: 5px;
  font-size: 11px;
  line-height: 18px;
  text-align: center;
  background: var(--panel);
  color: var(--muted);
}

.log-line.info .log-level {
  color: var(--primary);
}

.log-line.warn .log-level {
  color: #b45309;
}

.log-line.error .log-level {
  color: var(--danger);
}

.log-line.error .log-msg {
  color: var(--danger);
}

.log-msg {
  white-space: pre-wrap;
  word-break: break-all;
}

.log-empty {
  padding: 40px 0;
  text-align: center;
  font-size: 12px;
  color: var(--muted);
}

.panel-head .sub {
  display: flex;
  align-items: center;
  gap: 5px;
}
</style>
