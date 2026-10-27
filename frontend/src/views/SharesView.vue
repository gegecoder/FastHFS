<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref, watch } from "vue";
import {
  ArrowLeft,
  CopyDocument,
  Delete,
  Document,
  Folder,
  InfoFilled,
  Link,
  Plus,
  View,
} from "@element-plus/icons-vue";
import { ElMessageBox } from "element-plus";
import QRCode from "qrcode";
import * as Api from "../../bindings/FastHFS/app";
import type { Entry, Share } from "../../bindings/FastHFS/backend/models/models";
import { refreshMounts, refreshShares, state } from "../store";
import { fmtSize, fmtTime, guard, toastErr, toastOk } from "../ui";
import { i18n } from "../i18n";
import PageHeader from "../components/PageHeader.vue";

const dialogVisible = ref(false);
const saving = ref(false);
const qrVisible = ref(false);
const qrUrl = ref("");
const qrCanvas = ref<HTMLCanvasElement | null>(null);
const detailVisible = ref(false);
const detailShare = ref<Share | null>(null);
const detailUrl = ref("");

const mountEntries = ref<Entry[]>([]);
const browsing = ref(false);
const currentPath = ref("");
const targetPath = ref("");
const targetIsDir = ref(true);
const targetLabel = ref("");

const form = reactive({
  mountId: "",
  password: "",
  expiresIn: 24,
  maxVisits: 0,
  mode: "download",
});

onMounted(async () => {
  await Promise.all([refreshShares(), refreshMounts()]);
});

const crumbs = computed(() => {
  const parts = currentPath.value.split("/").filter(Boolean);
  const out = [{ label: i18n("shares.rootDir"), path: "" }];
  let acc = "";
  for (const p of parts) {
    acc = acc ? `${acc}/${p}` : p;
    out.push({ label: p, path: acc });
  }
  return out;
});

const dirEntries = computed(() => {
  const list = mountEntries.value.filter((e) => e.isDir);
  const files = mountEntries.value.filter((e) => !e.isDir);
  return [...list, ...files];
});

watch(
  () => form.mountId,
  async (id) => {
    currentPath.value = "";
    targetPath.value = "";
    targetIsDir.value = true;
    targetLabel.value = "";
    mountEntries.value = [];
    if (id) await loadDir("");
  },
);

async function loadDir(path: string) {
  if (!form.mountId) return;
  browsing.value = true;
  try {
    const entries = await Api.ListMountDir(form.mountId, path);
    mountEntries.value = entries || [];
    currentPath.value = path;
  } catch (error) {
    toastErr(error);
  } finally {
    browsing.value = false;
  }
}

async function enterDir(name: string) {
  const path = currentPath.value ? `${currentPath.value}/${name}` : name;
  await loadDir(path);
}

async function goUp() {
  if (!currentPath.value) return;
  const idx = currentPath.value.lastIndexOf("/");
  await loadDir(idx >= 0 ? currentPath.value.slice(0, idx) : "");
}

function selectDir() {
  targetPath.value = currentPath.value;
  targetIsDir.value = true;
  targetLabel.value = currentPath.value || i18n("shares.wholeRoot");
  if (form.mode === "collect") form.mode = "download";
}

function selectFile(name: string) {
  targetPath.value = currentPath.value ? `${currentPath.value}/${name}` : name;
  targetIsDir.value = false;
  targetLabel.value = name;
  if (form.mode === "collect") form.mode = "preview";
}

function openCreate() {
  form.mountId = state.mounts[0]?.id || "";
  form.password = "";
  form.expiresIn = 24;
  form.maxVisits = 0;
  form.mode = "download";
  targetPath.value = "";
  targetIsDir.value = true;
  targetLabel.value = "";
  dialogVisible.value = true;
}

async function create() {
  if (saving.value) return;
  if (!form.mountId) return toastErr(i18n("shares.errMount"));
  if (!targetLabel.value) return toastErr(i18n("shares.errTarget"));
  if (!targetIsDir.value && form.mode === "collect") {
    return toastErr(i18n("shares.errCollectFile"));
  }

  saving.value = true;
  try {
    const created = await Api.CreateShare({
      mountId: form.mountId,
      mount: "",
      path: targetPath.value,
      password: form.password.trim(),
      expiresIn: form.expiresIn || 0,
      maxVisits: form.maxVisits || 0,
      mode: form.mode,
    });
    if (created?.url) {
      dialogVisible.value = false;
      await refreshShares();
      await showQr(created.url);
      toastOk(i18n("shares.created"));
    }
  } catch (error) {
    toastErr(error);
  } finally {
    saving.value = false;
  }
}

async function showQr(url: string) {
  qrUrl.value = url;
  qrVisible.value = true;
  await nextTick();
  if (qrCanvas.value) {
    try {
      await QRCode.toCanvas(qrCanvas.value, url, {
        width: 228,
        margin: 1,
        color: { dark: "#0f172a", light: "#ffffff" },
      });
    } catch {
      // 二维码绘制失败时仍保留链接文本
    }
  }
}

async function showShareQr(row: Share) {
  const url = await guard(() => Api.GetShareURL(row.token));
  if (url) await showQr(url);
}

async function copyQr() {
  await guard(() => Api.CopyText(qrUrl.value), i18n("shares.copyDone"));
}

/** 点击二维码图片复制到系统剪贴板（PNG） */
async function copyQrImage() {
  const canvas = qrCanvas.value;
  if (!canvas) return;
  try {
    const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, "image/png"));
    if (!blob) throw new Error("empty canvas");
    await navigator.clipboard.write([new ClipboardItem({ "image/png": blob })]);
    toastOk(i18n("shares.qrCopied"));
  } catch {
    toastErr(i18n("shares.qrCopyFailed"));
  }
}

/** 打开分享详情弹窗，并异步取回完整链接 */
async function openDetail(row: Share) {
  detailShare.value = row;
  detailUrl.value = "";
  detailVisible.value = true;
  const url = await guard(() => Api.GetShareURL(row.token));
  if (url) detailUrl.value = url;
}

async function copyDetailLink() {
  if (!detailUrl.value) return;
  await guard(() => Api.CopyText(detailUrl.value), i18n("shares.copyDone"));
}

async function copyShare(row: Share) {
  const url = await guard(() => Api.GetShareURL(row.token));
  if (url) await guard(() => Api.CopyText(url), i18n("shares.copyDone"));
}

async function revoke(row: Share) {
  try {
    await ElMessageBox.confirm(i18n("shares.revokeConfirm", { token: row.token }), i18n("shares.revokeTitle"), {
      type: "warning",
      confirmButtonText: i18n("shares.revoke"),
      cancelButtonText: i18n("common.cancel"),
    });
  } catch {
    return;
  }
  const result = await guard(() => Api.DeleteShare(row.id));
  if (result?.success) await refreshShares();
}

const modeKeys: Record<string, string> = {
  preview: "shares.preview",
  download: "shares.download",
  collect: "shares.collect",
};

function modeLabel(mode: string): string {
  const key = modeKeys[mode];
  return key ? i18n(key) : mode;
}

const modeClass: Record<string, string> = {
  preview: "blue",
  download: "green",
  collect: "",
};

function isExpired(row: Share) {
  const now = Date.now() / 1000;
  if (row.expiresAt > 0 && row.expiresAt < now) return true;
  return row.maxVisits > 0 && row.visits >= row.maxVisits;
}

function sharePath(row: Share) {
  return row.path ? `/${row.path}` : i18n("shares.rootDir");
}
</script>

<template>
  <div class="page">
    <PageHeader :title="i18n('shares.title')" :desc="i18n('shares.desc')" />
    <main class="content">
      <div class="page-stack">
        <section class="panel">
          <div class="panel-head">
            <div>
              <h3>{{ i18n("shares.panelTitle") }}</h3>
              <p class="sub">{{ i18n("shares.panelSub") }}</p>
            </div>
            <el-button type="primary" :icon="Plus" :disabled="!state.mounts.length" @click="openCreate">
              {{ i18n("shares.create") }}
            </el-button>
          </div>

          <el-table v-if="state.shares.length" :data="state.shares" row-key="id" style="width: 100%">
            <el-table-column :label="i18n('shares.colResource')" min-width="230">
              <template #default="{ row }">
                <div class="share-res">
                  <el-icon><Folder v-if="!row.path" /><Document v-else /></el-icon>
                  <div>
                    <strong>{{ row.mountName || row.mountAlias }}</strong>
                    <span class="mono muted">{{ sharePath(row) }}</span>
                  </div>
                </div>
              </template>
            </el-table-column>
            <el-table-column :label="i18n('shares.colMode')" width="110">
              <template #default="{ row }">
                <span class="tag-soft" :class="modeClass[row.mode]">{{ modeLabel(row.mode) }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="i18n('shares.colStatus')" width="100">
              <template #default="{ row }">
                <span v-if="isExpired(row)" class="tag-soft red">{{ i18n("shares.expired") }}</span>
                <span v-else class="tag-soft green">{{ i18n("shares.valid") }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="i18n('shares.colActions')" width="300" align="right">
              <template #default="{ row }">
                <el-button text size="small" :icon="InfoFilled" @click="openDetail(row)">
                  {{ i18n("shares.details") }}
                </el-button>
                <el-button text size="small" :icon="CopyDocument" @click="copyShare(row)">
                  {{ i18n("common.copy") }}
                </el-button>
                <el-button text size="small" :icon="View" @click="showShareQr(row)">
                  {{ i18n("shares.qrCode") }}
                </el-button>
                <el-button text size="small" type="danger" :icon="Delete" @click="revoke(row)">
                  {{ i18n("shares.revoke") }}
                </el-button>
              </template>
            </el-table-column>
          </el-table>
          <div v-else class="empty">
            <p v-if="!state.mounts.length">{{ i18n("shares.emptyNoMounts") }}</p>
            <template v-else>
              <p>{{ i18n("shares.empty") }}</p>
              <el-button type="primary" plain :icon="Plus" @click="openCreate">{{ i18n("shares.emptyAction") }}</el-button>
            </template>
          </div>
        </section>

        <section class="panel">
          <div class="panel-head">
            <h3>{{ i18n("shares.modeTitle") }}</h3>
          </div>
          <div class="mode-grid">
            <div>
              <span class="tag-soft blue">{{ i18n("shares.preview") }}</span>
              <strong>{{ i18n("shares.previewTitle") }}</strong>
              <p>{{ i18n("shares.previewDesc") }}</p>
            </div>
            <div>
              <span class="tag-soft green">{{ i18n("shares.download") }}</span>
              <strong>{{ i18n("shares.downloadTitle") }}</strong>
              <p>{{ i18n("shares.downloadDesc") }}</p>
            </div>
            <div>
              <span class="tag-soft">{{ i18n("shares.collect") }}</span>
              <strong>{{ i18n("shares.collectTitle") }}</strong>
              <p>{{ i18n("shares.collectDesc") }}</p>
            </div>
          </div>
          <p class="hint">{{ i18n("shares.modeNote") }}</p>
        </section>

        <el-dialog v-model="dialogVisible" :title="i18n('shares.dialogTitle')" width="640px" :close-on-click-modal="false">
          <el-form label-position="top">
            <el-form-item :label="i18n('shares.mountLabel')">
              <el-select v-model="form.mountId" :placeholder="i18n('shares.mountPlaceholder')" style="width: 100%">
                <el-option
                  v-for="m in state.mounts"
                  :key="m.id"
                  :label="i18n('shares.mountOption', { name: m.name, alias: m.alias })"
                  :value="m.id"
                />
              </el-select>
            </el-form-item>

            <el-form-item :label="i18n('shares.targetLabel')">
              <div class="target-row">
                <div class="target-current">
                  <el-icon><Folder v-if="targetIsDir" /><Document v-else /></el-icon>
                  <span>{{ targetLabel || i18n("shares.targetNone") }}</span>
                </div>
                <el-button size="small" @click="selectDir">{{ i18n("shares.shareCurrentDir") }}</el-button>
              </div>
              <div class="browser">
                <div class="browser-head">
                  <el-button text size="small" :icon="ArrowLeft" :disabled="!currentPath" @click="goUp">
                    {{ i18n("shares.up") }}
                  </el-button>
                  <div class="crumbs">
                    <button
                      v-for="(c, i) in crumbs"
                      :key="c.path"
                      type="button"
                      class="crumb"
                      :class="{ active: i === crumbs.length - 1 }"
                      @click="loadDir(c.path)"
                    >
                      {{ c.label }}
                    </button>
                  </div>
                </div>
                <div v-loading="browsing" class="browser-list">
                  <div v-if="!dirEntries.length && !browsing" class="browser-empty">
                    {{ i18n("shares.emptyDir") }}
                  </div>
                  <button
                    v-for="entry in dirEntries"
                    :key="entry.name"
                    type="button"
                    class="entry-row"
                    :class="{ picked: !targetIsDir && targetPath === (currentPath ? currentPath + '/' + entry.name : entry.name) }"
                    @click="entry.isDir ? enterDir(entry.name) : selectFile(entry.name)"
                  >
                    <el-icon class="entry-icon" :class="{ dir: entry.isDir }">
                      <Folder v-if="entry.isDir" /><Document v-else />
                    </el-icon>
                    <span class="entry-name">{{ entry.name }}</span>
                    <span class="entry-meta muted">{{ entry.isDir ? i18n("shares.dir") : fmtSize(entry.size) }}</span>
                  </button>
                </div>
              </div>
              <div class="hint">{{ i18n("shares.targetHint") }}</div>
            </el-form-item>

            <div class="form-grid">
              <el-form-item :label="i18n('shares.modeLabel')">
                <el-select v-model="form.mode" style="width: 100%">
                  <el-option :label="i18n('shares.preview')" value="preview" />
                  <el-option :label="i18n('shares.download')" value="download" />
                  <el-option v-if="targetIsDir" :label="i18n('shares.collectOnlyDir')" value="collect" />
                </el-select>
              </el-form-item>
              <el-form-item :label="i18n('shares.passwordLabel')">
                <el-input
                  v-model="form.password"
                  :placeholder="i18n('shares.passwordPlaceholder')"
                  maxlength="64"
                  show-password
                />
              </el-form-item>
              <el-form-item :label="i18n('shares.expiresIn')">
                <el-input-number v-model="form.expiresIn" :min="0" :max="8760" style="width: 100%" />
                <div class="hint">{{ i18n("shares.expiresInHint") }}</div>
              </el-form-item>
              <el-form-item :label="i18n('shares.maxVisits')">
                <el-input-number v-model="form.maxVisits" :min="0" :max="100000" style="width: 100%" />
                <div class="hint">{{ i18n("shares.maxVisitsHint") }}</div>
              </el-form-item>
            </div>
          </el-form>
          <template #footer>
            <el-button @click="dialogVisible = false">{{ i18n("common.cancel") }}</el-button>
            <el-button type="primary" :loading="saving" :icon="Link" @click="create">
              {{ i18n("shares.createLink") }}
            </el-button>
          </template>
        </el-dialog>

        <el-dialog v-model="qrVisible" :title="i18n('shares.qrTitle')" width="380px">
          <div class="qr-wrap">
            <el-tooltip :content="i18n('shares.qrClickCopy')" placement="top" :show-after="200">
              <div class="qr-box qr-clickable" @click="copyQrImage">
                <canvas ref="qrCanvas" />
              </div>
            </el-tooltip>
            <div class="qr-hint">{{ i18n("shares.qrClickCopy") }}</div>
            <div class="share-url">{{ qrUrl }}</div>
            <el-button type="primary" :icon="CopyDocument" @click="copyQr">{{ i18n("shares.copyLink") }}</el-button>
          </div>
        </el-dialog>

        <el-dialog v-model="detailVisible" :title="i18n('shares.detailTitle')" width="520px">
          <el-descriptions v-if="detailShare" :column="1" border size="small">
            <el-descriptions-item :label="i18n('shares.colResource')">
              {{ detailShare.mountName || detailShare.mountAlias }}
              <span class="mono muted"> {{ sharePath(detailShare) }}</span>
            </el-descriptions-item>
            <el-descriptions-item :label="i18n('shares.colMode')">
              <span class="tag-soft" :class="modeClass[detailShare.mode]">{{ modeLabel(detailShare.mode) }}</span>
            </el-descriptions-item>
            <el-descriptions-item :label="i18n('shares.colStatus')">
              <span v-if="isExpired(detailShare)" class="tag-soft red">{{ i18n("shares.expired") }}</span>
              <span v-else class="tag-soft green">{{ i18n("shares.valid") }}</span>
            </el-descriptions-item>
            <el-descriptions-item :label="i18n('shares.colVisits')">
              <span class="mono">{{ detailShare.visits }} / {{ detailShare.maxVisits || "∞" }}</span>
            </el-descriptions-item>
            <el-descriptions-item :label="i18n('shares.colPassword')">
              <span v-if="detailShare.hasPassword" class="tag-soft">{{ i18n("shares.hasPassword") }}</span>
              <span v-else class="muted">{{ i18n("shares.noPassword") }}</span>
            </el-descriptions-item>
            <el-descriptions-item :label="i18n('shares.colExpiresAt')">
              <span class="muted">{{ fmtTime(detailShare.expiresAt) }}</span>
            </el-descriptions-item>
            <el-descriptions-item :label="i18n('shares.colCreatedAt')">
              <span class="muted">{{ fmtTime(detailShare.createdAt) }}</span>
            </el-descriptions-item>
            <el-descriptions-item :label="i18n('shares.detailLink')">
              <div class="detail-link">
                <span class="mono">{{ detailUrl || "…" }}</span>
                <el-button
                  v-if="detailUrl"
                  text
                  size="small"
                  :icon="CopyDocument"
                  @click="copyDetailLink"
                />
              </div>
            </el-descriptions-item>
          </el-descriptions>
        </el-dialog>
      </div>
    </main>
  </div>
</template>

<style scoped>
.share-res {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  color: var(--primary);
}

.share-res strong {
  display: block;
  color: var(--text);
  font-size: 13px;
}

.share-res span {
  display: block;
  font-size: 11.5px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mode-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
  margin-bottom: 12px;
}

.mode-grid > div {
  padding: 14px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--panel-soft);
}

.mode-grid strong {
  display: block;
  margin: 8px 0 4px;
  font-size: 13px;
}

.mode-grid p {
  margin: 0;
  font-size: 12px;
  color: var(--muted);
  line-height: 1.65;
}

.target-row {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
}

.target-current {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 11px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--panel-soft);
  font-size: 12.5px;
  color: var(--muted);
}

.target-current span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.browser {
  margin-top: 8px;
  width: 100%;
  border: 1px solid var(--border);
  border-radius: 10px;
  overflow: hidden;
}

.browser-head {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 8px;
  background: var(--panel-soft);
  border-bottom: 1px solid var(--border);
}

.crumbs {
  display: flex;
  gap: 2px;
  flex-wrap: wrap;
  overflow: hidden;
}

.crumb {
  border: none;
  background: transparent;
  color: var(--muted);
  font-size: 12px;
  font-family: inherit;
  cursor: pointer;
  padding: 2px 5px;
  border-radius: 5px;
}

.crumb:hover {
  color: var(--primary);
  background: var(--primary-soft);
}

.crumb.active {
  color: var(--text);
  font-weight: 600;
}

.browser-list {
  height: 190px;
  overflow-y: auto;
  padding: 4px;
}

.browser-empty {
  padding: 30px;
  text-align: center;
  color: var(--muted);
  font-size: 12.5px;
}

.entry-row {
  display: flex;
  align-items: center;
  gap: 9px;
  width: 100%;
  padding: 7px 9px;
  border: none;
  border-radius: 7px;
  background: transparent;
  color: var(--text);
  font-family: inherit;
  font-size: 12.5px;
  text-align: left;
  cursor: pointer;
}

.entry-row:hover {
  background: var(--panel-soft);
}

.entry-row.picked {
  background: var(--primary-soft);
  color: var(--primary);
}

.entry-icon {
  color: var(--muted);
}

.entry-icon.dir {
  color: var(--primary);
}

.entry-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.entry-meta {
  font-size: 11px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 14px;
}

.qr-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
}

.qr-clickable {
  cursor: pointer;
  transition: box-shadow 0.15s ease;
}

.qr-clickable:hover {
  box-shadow: 0 0 0 2px var(--el-color-primary);
}

.qr-hint {
  font-size: 12px;
  color: var(--muted);
}

.detail-link {
  display: flex;
  align-items: center;
  gap: 6px;
}

.detail-link .mono {
  word-break: break-all;
}

@media (max-width: 760px) {
  .mode-grid,
  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
