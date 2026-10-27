<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import {
  CopyDocument,
  FolderAdd,
  FolderOpened,
  Grid,
  Link,
  Refresh,
  Share,
  TopRight,
  User,
  VideoPause,
  VideoPlay,
} from "@element-plus/icons-vue";
import QRCode from "qrcode";
import * as Api from "../../bindings/FastHFS/app";
import { isRunning, refreshStatus, state } from "../store";
import { fmtDuration, fmtTime, guard, toastErr, toastOk } from "../ui";
import { i18n } from "../i18n";
import PageHeader from "../components/PageHeader.vue";

const emit = defineEmits<{ (e: "navigate", page: "mounts" | "users" | "shares"): void }>();

const running = computed(() => isRunning());
const busy = ref(false);
const tick = ref(Date.now());

let timer: number | undefined;

onMounted(() => {
  timer = window.setInterval(() => (tick.value = Date.now()), 30_000);
});

onUnmounted(() => {
  if (timer) window.clearInterval(timer);
});

const uptime = computed(() => {
  void tick.value;
  return fmtDuration(state.status?.startedAt);
});

const activeShares = computed(() => {
  const now = Date.now() / 1000;
  return state.shares.filter(
    (s) => (!s.expiresAt || s.expiresAt > now) && (!s.maxVisits || s.visits < s.maxVisits),
  ).length;
});

const listenLabel = computed(() => {
  const addr = state.config.listenAddr;
  if (addr === "0.0.0.0") return i18n("home.addrAll");
  if (!addr || addr === "127.0.0.1") return i18n("home.addrLoopback");
  return i18n("home.addrSpecific", { addr });
});

/** 仅监听回环地址时，其他设备无法访问 */
const lanRestricted = computed(() => !state.config.listenAddr || state.config.listenAddr === "127.0.0.1");

const lastStarted = computed(() => (state.status?.startedAt ? fmtTime(state.status.startedAt) : "—"));

async function toggle() {
  busy.value = true;
  try {
    const result = running.value ? await Api.StopService() : await Api.StartService();
    if (result?.success) toastOk(result.message);
    else toastErr(result?.message || i18n("common.actionFailed"));
    await refreshStatus();
  } catch (error) {
    toastErr(error);
  } finally {
    busy.value = false;
  }
}

async function restart() {
  busy.value = true;
  try {
    const result = await Api.RestartService();
    if (result?.success) toastOk(result.message);
    else toastErr(result?.message || i18n("home.restartFailed"));
    await refreshStatus();
  } catch (error) {
    toastErr(error);
  } finally {
    busy.value = false;
  }
}

async function copy(url: string) {
  await guard(() => Api.CopyText(url), i18n("home.copyDone"));
}

async function open(url: string) {
  await guard(() => Api.OpenURL(url), i18n("common.openSuccess"));
}

/** 访问地址二维码：按地址缓存 data URL，弹层打开时首次生成 */
const qrImages = ref<Record<string, string>>({});

async function drawQr(url: string) {
  if (qrImages.value[url]) return;
  try {
    const data = await QRCode.toDataURL(url, {
      width: 300,
      margin: 1,
      color: { dark: "#0f172a", light: "#ffffff" },
    });
    qrImages.value = { ...qrImages.value, [url]: data };
  } catch {
    // 生成失败时弹层保留提示文案，不影响地址展示与复制
  }
}

/** 点击二维码图片复制到系统剪贴板（PNG） */
async function copyQr(url: string) {
  const dataUrl = qrImages.value[url];
  if (!dataUrl) return;
  try {
    const blob = await (await fetch(dataUrl)).blob();
    await navigator.clipboard.write([new ClipboardItem({ "image/png": blob })]);
    toastOk(i18n("home.qrCopied"));
  } catch {
    toastErr(i18n("home.qrCopyFailed"));
  }
}
</script>

<template>
  <div class="page">
    <PageHeader :title="i18n('home.title')" :desc="i18n('home.desc')" />
    <main class="content">
      <div class="page-stack">
        <section class="hero panel" :class="{ on: running }">
          <div class="hero-main">
            <div class="hero-copy">
              <div class="hero-title">
                <div class="hero-orb">
                  <svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true">
                    <path
                      d="M3 7.5A2.5 2.5 0 0 1 5.5 5h4l2 2h7A2.5 2.5 0 0 1 21 9.5v8A2.5 2.5 0 0 1 18.5 20h-13A2.5 2.5 0 0 1 3 17.5v-10Z"
                      fill="currentColor"
                    />
                    <path d="M12 11v5m0 0 2.2-2.2M12 16l-2.2-2.2" stroke="#fff" stroke-width="1.4" stroke-linecap="round" />
                  </svg>
                </div>
                <h1>{{ running ? i18n("home.runningTitle") : i18n("home.stoppedTitle") }}</h1>
              </div>
              <p>
                {{ running ? i18n("home.runningDesc") : i18n("home.stoppedDesc") }}
              </p>
              <div class="hero-actions">
                <el-button
                  :type="running ? 'danger' : 'primary'"
                  :icon="running ? VideoPause : VideoPlay"
                  :loading="busy"
                  @click="toggle"
                >
                  {{ running ? i18n("home.stop") : i18n("home.start") }}
                </el-button>
                <el-button v-if="running" :icon="Refresh" :loading="busy" @click="restart">
                  {{ i18n("home.restart") }}
                </el-button>
                <el-button :icon="FolderOpened" @click="emit('navigate', 'mounts')">{{ i18n("home.manageMounts") }}</el-button>
              </div>
            </div>
          </div>
          <div class="hero-meta">
            <div class="meta-row">
              <span>{{ i18n("home.listenAddr") }}</span>
              <strong>{{ listenLabel }}</strong>
            </div>
            <div class="meta-row">
              <span>{{ i18n("home.port") }}</span>
              <strong class="mono">{{ state.config.port }}</strong>
            </div>
            <div class="meta-row">
              <span>{{ i18n("home.startedAt") }}</span>
              <strong>{{ running ? lastStarted : "—" }}</strong>
            </div>
            <div class="meta-row">
              <span>{{ i18n("home.uptime") }}</span>
              <strong>{{ running ? uptime : "—" }}</strong>
            </div>
          </div>
        </section>

        <div class="stat-grid">
          <button class="stat-card" type="button" @click="emit('navigate', 'mounts')">
            <el-icon class="stat-icon"><FolderOpened /></el-icon>
            <div>
              <strong>{{ state.mounts.length }}</strong>
              <span>{{ i18n("home.statMounts") }}</span>
            </div>
          </button>
          <button class="stat-card" type="button" @click="emit('navigate', 'shares')">
            <el-icon class="stat-icon green"><Link /></el-icon>
            <div>
              <strong>{{ activeShares }}</strong>
              <span>{{ i18n("home.statShares") }}</span>
            </div>
          </button>
          <button class="stat-card" type="button" @click="emit('navigate', 'users')">
            <el-icon class="stat-icon blue"><User /></el-icon>
            <div>
              <strong>{{ state.users.length }}</strong>
              <span>{{ i18n("home.statUsers") }}</span>
            </div>
          </button>
        </div>

        <div class="two-col">
          <section class="panel">
            <div class="panel-head">
              <div>
                <h3>{{ i18n("home.accessTitle") }}</h3>
                <p class="sub">{{ i18n("home.accessSub") }}</p>
              </div>
              <el-tag v-if="running" type="success" size="small" effect="light">
                {{ i18n("home.accessible") }}
              </el-tag>
              <el-tag v-else type="info" size="small" effect="light">{{ i18n("home.notStarted") }}</el-tag>
            </div>

            <div v-if="state.urls.length" class="addr-list">
              <div v-for="url in state.urls" :key="url" class="addr-row">
                <span class="addr-dot" />
                <el-tooltip :content="url" placement="top" :show-after="250">
                  <code>{{ url }}</code>
                </el-tooltip>
                <div class="addr-actions">
                  <el-popover
                    trigger="hover"
                    placement="top"
                    :width="172"
                    popper-class="qr-popover"
                    @show="drawQr(url)"
                  >
                    <template #reference>
                      <el-button text size="small" :icon="Grid" :aria-label="i18n('home.qrCode')" />
                    </template>
                    <div class="qr-pop">
                      <el-tooltip v-if="qrImages[url]" :content="i18n('home.qrCopy')" placement="bottom" :show-after="200">
                        <img
                          class="qr-img"
                          :src="qrImages[url]"
                          :alt="i18n('home.qrTip')"
                          @click="copyQr(url)"
                        />
                      </el-tooltip>
                      <span v-else class="qr-tip">{{ i18n("home.qrTip") }}</span>
                    </div>
                  </el-popover>
                  <el-tooltip :content="i18n('common.copy')" placement="top" :show-after="250">
                    <el-button
                      text
                      size="small"
                      :icon="CopyDocument"
                      :aria-label="i18n('common.copy')"
                      @click="copy(url)"
                    />
                  </el-tooltip>
                  <el-tooltip :content="i18n('common.open')" placement="top" :show-after="250">
                    <el-button
                      text
                      size="small"
                      :icon="TopRight"
                      :aria-label="i18n('common.open')"
                      @click="open(url)"
                    />
                  </el-tooltip>
                </div>
              </div>
            </div>
            <div v-else class="empty">{{ i18n("home.emptyUrls") }}</div>

            <el-alert
              v-if="lanRestricted"
              class="listen-alert"
              type="warning"
              :closable="false"
              show-icon
              :title="i18n('home.restrictedTitle')"
              :description="i18n('home.restrictedDesc')"
            />
          </section>

          <section class="panel">
            <div class="panel-head">
              <div>
                <h3>{{ i18n("home.quickTitle") }}</h3>
                <p class="sub">{{ i18n("home.quickSub") }}</p>
              </div>
            </div>
            <ol class="steps">
              <li>
                <span class="step-no">1</span>
                <div>
                  <strong>{{ i18n("home.step1Title") }}</strong>
                  <p>{{ i18n("home.step1Desc") }}</p>
                </div>
              </li>
              <li>
                <span class="step-no">2</span>
                <div>
                  <strong>{{ i18n("home.step2Title") }}</strong>
                  <p>{{ i18n("home.step2Desc") }}</p>
                </div>
              </li>
              <li>
                <span class="step-no">3</span>
                <div>
                  <strong>{{ i18n("home.step3Title") }}</strong>
                  <p>{{ i18n("home.step3Desc") }}</p>
                </div>
              </li>
            </ol>
            <div class="quick-links">
              <el-button size="small" :icon="FolderAdd" @click="emit('navigate', 'mounts')">{{ i18n("home.addMount") }}</el-button>
              <el-button size="small" :icon="Share" @click="emit('navigate', 'shares')">{{ i18n("home.createShare") }}</el-button>
              <el-button size="small" :icon="User" @click="emit('navigate', 'users')">{{ i18n("home.addAccount") }}</el-button>
            </div>
          </section>
        </div>
      </div>
    </main>
  </div>
</template>

<style scoped>
.hero {
  display: flex;
  gap: 26px;
  align-items: stretch;
  justify-content: space-between;
  background:
    radial-gradient(120% 140% at 0% 0%, color-mix(in srgb, var(--primary) 9%, transparent) 0%, transparent 55%),
    var(--panel);
}

.hero-main {
  display: flex;
  align-items: flex-start;
  min-width: 0;
}

/* 图标与标题同一行 */
.hero-title {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 0 0 6px;
}

.hero-title h1 {
  margin: 0;
  font-size: 24px;
  letter-spacing: 0.2px;
}

.hero-orb {
  width: 38px;
  height: 38px;
  flex: 0 0 38px;
  border-radius: 11px;
  display: grid;
  place-items: center;
  color: #94a3b8;
  background: var(--panel-soft);
  border: 1px solid var(--border);
  transition: all 0.25s ease;
}

.hero.on .hero-orb {
  color: #fff;
  background: linear-gradient(145deg, #22c55e, #15803d);
  border-color: transparent;
  box-shadow: 0 10px 24px rgba(22, 163, 74, 0.28);
}

.hero-copy p {
  margin: 0;
  color: var(--muted);
  font-size: 13px;
  max-width: 520px;
  line-height: 1.7;
}

.hero-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-top: 16px;
}

.hero-meta {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding-left: 22px;
  border-left: 1px solid var(--border);
  min-width: 210px;
}

.meta-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 18px;
  font-size: 12.5px;
}

.meta-row span {
  color: var(--muted);
}

.meta-row strong {
  font-weight: 600;
  text-align: right;
}

.stat-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
}

.stat-card {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 17px 18px;
  border-radius: 14px;
  border: 1px solid var(--border);
  background: var(--panel);
  box-shadow: var(--shadow);
  color: var(--text);
  font-family: inherit;
  text-align: left;
  cursor: pointer;
  transition: transform 0.16s ease, border-color 0.16s ease;
}

.stat-card:hover {
  transform: translateY(-2px);
  border-color: color-mix(in srgb, var(--primary) 35%, var(--border));
}

.stat-icon {
  width: 40px;
  height: 40px;
  border-radius: 11px;
  background: var(--panel-soft);
  color: var(--primary);
  font-size: 20px;
}

.stat-icon.green {
  color: var(--success);
}
.stat-icon.blue {
  color: #6366f1;
}

.stat-card strong {
  display: block;
  font-size: 20px;
  line-height: 1.2;
}

.stat-card span {
  font-size: 12px;
  color: var(--muted);
}

.two-col {
  display: grid;
  grid-template-columns: minmax(0, 1.15fr) minmax(0, 1fr);
  gap: 16px;
}

.addr-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.addr-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 7px 10px;
  border-radius: 9px;
  border: 1px solid var(--border);
  background: var(--panel-soft);
}

.addr-row code {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: "JetBrains Mono", Consolas, monospace;
  font-size: 12px;
}

/* 二维码 / 复制 / 打开：靠右紧凑排列 */
.addr-actions {
  display: flex;
  align-items: center;
  gap: 2px;
  margin-left: auto;
  flex: 0 0 auto;
}

.addr-actions :deep(.el-button) {
  padding: 4px;
  height: auto;
  margin-left: 0;
}

.addr-actions :deep(.el-button + .el-button) {
  margin-left: 0;
}

.addr-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--success);
  flex: 0 0 7px;
}

.listen-alert {
  margin-top: 14px;
  border-radius: 10px;
}

.steps {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.steps li {
  display: flex;
  gap: 12px;
}

.step-no {
  width: 24px;
  height: 24px;
  flex: 0 0 24px;
  border-radius: 50%;
  background: var(--primary-soft);
  color: var(--primary);
  font-size: 12px;
  font-weight: 600;
  display: grid;
  place-items: center;
}

.steps strong {
  font-size: 13px;
}

.steps p {
  margin: 3px 0 0;
  font-size: 12px;
  color: var(--muted);
  line-height: 1.65;
}

.quick-links {
  display: flex;
  gap: 8px;
  margin-top: 16px;
  padding-top: 14px;
  border-top: 1px dashed var(--border);
}

@media (max-width: 1100px) {
  .stat-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .two-col {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 760px) {
  .hero {
    flex-direction: column;
    gap: 18px;
  }
  .hero-meta {
    border-left: none;
    border-top: 1px solid var(--border);
    padding-left: 0;
    padding-top: 14px;
  }
}
</style>
