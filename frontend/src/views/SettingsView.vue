<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { Check, Edit, Monitor, Plus, Refresh, Search, Warning } from "@element-plus/icons-vue";
import * as Api from "../../bindings/FastHFS/app";
import type { Config, NetworkInterface } from "../../bindings/FastHFS/backend/models/models";
import {
  applyTheme,
  isRunning,
  refreshConfig,
  refreshEnv,
  refreshStatus,
  state,
  theme,
} from "../store";
import { guard, toastErr, toastOk } from "../ui";
import { SUPPORTED_LOCALES, i18n, resolveLocale, setLocale } from "../i18n";
import PageHeader from "../components/PageHeader.vue";

const serviceRunning = computed(() => isRunning());

const saving = ref(false);
const checking = ref(false);
const autoStartLoading = ref(false);
/** 编辑态：默认只读展示，点击「编辑」后才可修改 */
const editing = ref(false);

/** 监听方式：仅本机 / 所有网卡 / 绑定指定网卡 */
type ListenMode = "loopback" | "all" | "interface";

/** 日志外发目标 */
interface SyslogTarget {
  host: string;
  port: number;
}

const listenMode = ref<ListenMode>("loopback");
const selectedIP = ref("");
const interfaces = ref<NetworkInterface[]>([]);
const loadingInterfaces = ref(false);

const form = reactive({
  port: 8080,
  downloadLimitKB: 0,
  uploadLimitKB: 0,
  anonymousMode: "readonly",
  maxUploadMB: 4096,
  blacklist: "",
  basePath: "",
  rateLimitPerMin: 300,
  syslogEnabled: false,
  syslogTargets: [] as SyslogTarget[],
  language: "zh",
});

const currentTheme = ref<"light" | "dark">(theme.value);

/** 可用于绑定监听的网卡地址（回环网卡由「仅本机」模式覆盖，这里排除） */
const interfaceChoices = computed(() => {
  const choices: { value: string; label: string }[] = [];
  for (const iface of interfaces.value) {
    if (iface.loopback) continue;
    for (const ip of iface.addresses || []) {
      choices.push({ value: ip, label: `${iface.name || i18n("settings.interfaceFallback")} · ${ip}` });
    }
  }
  return choices;
});

/** 最终提交给后端的监听地址 */
const listenAddr = computed(() => {
  if (listenMode.value === "all") return "0.0.0.0";
  if (listenMode.value === "interface") return selectedIP.value || "127.0.0.1";
  return "127.0.0.1";
});

// 先用内存中的配置渲染表单，避免进入设置页瞬间先显示「仅本机」再切到指定网卡
syncForm(state.config);

onMounted(async () => {
  await Promise.all([refreshConfig(), refreshEnv(), refreshStatus(), loadInterfaces()]);
  syncForm(state.config);
});

async function loadInterfaces() {
  loadingInterfaces.value = true;
  try {
    interfaces.value = (await Api.GetNetworkInterfaces()) || [];
    if (!interfaceChoices.value.some((c) => c.value === selectedIP.value)) {
      selectedIP.value = interfaceChoices.value[0]?.value || "";
    }
  } catch (error) {
    toastErr(error);
  } finally {
    loadingInterfaces.value = false;
  }
}

function syncForm(cfg: Config) {
  const addr = cfg.listenAddr || "127.0.0.1";
  if (addr === "0.0.0.0") {
    listenMode.value = "all";
  } else if (addr === "127.0.0.1") {
    listenMode.value = "loopback";
  } else {
    listenMode.value = "interface";
    selectedIP.value = addr;
  }
  form.port = cfg.port || 8080;
  form.downloadLimitKB = cfg.downloadLimitKB || 0;
  form.uploadLimitKB = cfg.uploadLimitKB || 0;
  form.anonymousMode = cfg.anonymousMode || "readonly";
  form.maxUploadMB = cfg.maxUploadMB || 0;
  form.blacklist = (cfg.blacklist || []).join("\n");
  form.basePath = cfg.basePath || "";
  form.rateLimitPerMin = cfg.rateLimitPerMin || 0;
  form.syslogEnabled = Boolean(cfg.syslogEnabled);
  form.syslogTargets = (cfg.syslogTargets || []).map(parseSyslogTarget);
  form.language = resolveLocale(cfg.language);
  currentTheme.value = theme.value;
}

/** 把 "IP:端口" 拆成主机与端口，缺省端口为 514 */
function parseSyslogTarget(value: string): SyslogTarget {
  const raw = (value || "").trim();
  const idx = raw.lastIndexOf(":");
  if (idx <= 0) return { host: raw.replace(/^\[|\]$/g, ""), port: 514 };
  const port = Number(raw.slice(idx + 1));
  return {
    host: raw.slice(0, idx).replace(/^\[|\]$/g, ""),
    port: Number.isFinite(port) && port > 0 ? port : 514,
  };
}

function addSyslogTarget() {
  form.syslogTargets.push({ host: "", port: 514 });
}

function removeSyslogTarget(index: number) {
  form.syslogTargets.splice(index, 1);
}

function onModeChange(mode: ListenMode) {
  if (mode === "interface" && !selectedIP.value) {
    selectedIP.value = interfaceChoices.value[0]?.value || "";
  }
}

const lanWarning = computed(() => listenMode.value !== "loopback");

const secondaryUrls = computed(() => state.urls.slice(0, 3));

async function checkPort() {
  checking.value = true;
  try {
    const used = await Api.CheckPortInUse(form.port);
    if (used) toastErr(i18n("settings.portInUse", { port: form.port }));
    else toastOk(i18n("settings.portAvailable", { port: form.port }));
  } catch (error) {
    toastErr(error);
  } finally {
    checking.value = false;
  }
}

async function autoPort() {
  checking.value = true;
  try {
    const port = await Api.FindFreePort(form.port + 1);
    if (port > 0) {
      form.port = port;
      toastOk(i18n("settings.portFound", { port }));
    } else {
      toastErr(i18n("settings.portNotFound"));
    }
  } catch (error) {
    toastErr(error);
  } finally {
    checking.value = false;
  }
}

function normalizeBasePath(value: string): string {
  let v = value.trim();
  if (!v) return "";
  if (!v.startsWith("/")) v = `/${v}`;
  v = v.replace(/\/+$/, "");
  if (v === "/") return "";
  return v;
}

async function save() {
  if (saving.value) return;
  if (!form.port || form.port < 1 || form.port > 65535) return toastErr(i18n("settings.errPort"));
  if (form.maxUploadMB < 0) return toastErr(i18n("settings.errMaxUpload"));
  if (form.downloadLimitKB < 0 || form.uploadLimitKB < 0) return toastErr(i18n("settings.errSpeed"));
  if (listenMode.value === "interface" && !selectedIP.value) {
    return toastErr(i18n("settings.errInterface"));
  }
  for (const target of form.syslogTargets) {
    if (!target.host.trim()) continue;
    if (!target.port || target.port < 1 || target.port > 65535) {
      return toastErr(i18n("settings.errTargetPort", { host: target.host.trim() }));
    }
  }
  const syslogTargets = form.syslogTargets
    .filter((target) => target.host.trim())
    .map((target) => `${target.host.trim()}:${target.port}`);
  if (form.syslogEnabled && !syslogTargets.length) {
    return toastErr(i18n("settings.errTargets"));
  }

  const payload: Config = {
    listenAddr: listenAddr.value,
    port: form.port,
    downloadLimitKB: form.downloadLimitKB || 0,
    uploadLimitKB: form.uploadLimitKB || 0,
    anonymousMode: form.anonymousMode,
    theme: currentTheme.value,
    maxUploadMB: form.maxUploadMB || 0,
    blacklist: form.blacklist
      .split(/[\n,]+/)
      .map((s) => s.trim())
      .filter(Boolean),
    basePath: normalizeBasePath(form.basePath),
    rateLimitPerMin: form.rateLimitPerMin || 0,
    autoStart: state.autoStart,
    syslogEnabled: form.syslogEnabled,
    syslogTargets,
    language: form.language,
  };

  saving.value = true;
  try {
    const result = await Api.SaveConfig(payload);
    if (result?.success) {
      applyTheme(currentTheme.value);
      await refreshConfig();
      await refreshStatus();
      syncForm(state.config);
      editing.value = false;
      setLocale(resolveLocale(state.config.language));
      toastOk(result.message);
    } else {
      toastErr(result?.message || i18n("settings.saveFailed"));
    }
  } catch (error) {
    toastErr(error);
  } finally {
    saving.value = false;
  }
}

/** 进入编辑态 */
function startEdit() {
  editing.value = true;
}

/** 取消编辑：丢弃未保存的改动，回到只读展示 */
function cancelEdit() {
  syncForm(state.config);
  setLocale(resolveLocale(state.config.language));
  editing.value = false;
}

/** 语言选择即时预览；保存后写入配置，取消时回退到已保存的语言 */
function onLanguageChange(value: string | number | boolean | undefined) {
  setLocale(resolveLocale(typeof value === "string" ? value : form.language));
}

async function toggleAutoStart(value: boolean) {
  autoStartLoading.value = true;
  try {
    const result = await Api.SetAutoStart(value);
    if (result?.success) {
      state.autoStart = value;
      toastOk(result.message);
    } else {
      state.autoStart = !value;
      toastErr(result?.message || i18n("settings.toggleFailed"));
    }
  } catch (error) {
    state.autoStart = !value;
    toastErr(error);
  } finally {
    autoStartLoading.value = false;
  }
}

async function copyText(text: string) {
  await guard(() => Api.CopyText(text), i18n("common.copySuccess"));
}
</script>

<template>
  <div class="page">
    <PageHeader :title="i18n('settings.title')" :desc="i18n('settings.desc')">
      <el-tooltip
        v-if="!editing"
        :content="i18n('settings.editLocked')"
        placement="bottom"
        :disabled="!serviceRunning"
      >
        <span class="edit-btn-wrap">
          <el-button type="primary" :icon="Edit" :disabled="serviceRunning" @click="startEdit">
            {{ i18n("settings.edit") }}
          </el-button>
        </span>
      </el-tooltip>
      <template v-else>
        <el-button :disabled="saving" @click="cancelEdit">{{ i18n("common.cancel") }}</el-button>
        <el-button type="primary" :loading="saving" :icon="Check" @click="save">{{ i18n("common.save") }}</el-button>
      </template>
    </PageHeader>
    <main class="content">
      <div class="page-stack">
        <section class="panel">
          <div class="panel-head">
            <div>
              <h3>{{ i18n("settings.panelListen") }}</h3>
              <p class="sub">{{ i18n("settings.panelListenDesc") }}</p>
            </div>
          </div>

          <el-form label-position="top" :disabled="!editing">
            <el-form-item :label="i18n('settings.listenAddr')">
              <el-radio-group v-model="listenMode" @change="onModeChange">
                <el-radio-button value="loopback">{{ i18n("settings.loopback") }}</el-radio-button>
                <el-radio-button value="all">{{ i18n("settings.allInterfaces") }}</el-radio-button>
                <el-radio-button value="interface">{{ i18n("settings.specifiedInterface") }}</el-radio-button>
              </el-radio-group>
            </el-form-item>

            <el-form-item v-if="listenMode === 'interface'" :label="i18n('settings.interfaceAddr')">
              <div class="inline-row">
                <el-select
                  v-model="selectedIP"
                  :placeholder="i18n('settings.interfacePlaceholder')"
                  style="width: 260px"
                  :loading="loadingInterfaces"
                >
                  <el-option v-for="c in interfaceChoices" :key="c.value" :label="c.label" :value="c.value" />
                </el-select>
                <el-button class="refresh-btn" :icon="Refresh" :loading="loadingInterfaces" :disabled="!editing" @click="loadInterfaces">
                  {{ i18n("settings.refreshInterfaces") }}
                </el-button>
              </div>
              <div v-if="!interfaceChoices.length" class="hint">
                {{ i18n("settings.noInterfaces") }}
              </div>
              <div v-else class="hint">{{ i18n("settings.interfaceHint") }}</div>
            </el-form-item>

            <el-alert
              v-if="lanWarning"
              class="block-alert"
              type="warning"
              :closable="false"
              show-icon
              :title="listenMode === 'all' ? i18n('settings.lanAlertAll') : i18n('settings.lanAlertBind')"
              :description="i18n('settings.lanAlertDesc')"
            />

            <div class="field-grid">
              <el-form-item :label="i18n('settings.port')">
                <div class="inline-row">
                  <el-input-number v-model="form.port" :min="1" :max="65535" controls-position="right" style="width: 150px" />
                  <el-button :icon="Search" :loading="checking" :disabled="!editing" @click="checkPort">
                    {{ i18n("settings.checkPort") }}
                  </el-button>
                  <el-button :loading="checking" :disabled="!editing" @click="autoPort">
                    {{ i18n("settings.autoPort") }}
                  </el-button>
                </div>
              </el-form-item>
              <el-form-item :label="i18n('settings.basePath')">
                <el-input v-model="form.basePath" :placeholder="i18n('settings.basePathPlaceholder')" />
                <div class="hint">{{ i18n("settings.basePathHint") }}</div>
              </el-form-item>
            </div>

            <div v-if="state.status?.running && secondaryUrls.length" class="url-preview">
              <span class="muted">{{ i18n("settings.currentUrl") }}</span>
              <div class="tag-list">
                <span v-for="u in secondaryUrls" :key="u" class="tag-soft mono" @click="copyText(u)">{{ u }}</span>
              </div>
            </div>
          </el-form>
          <div class="restart-hint">
            <el-icon><Warning /></el-icon>
            {{ i18n("settings.restartHint") }}
          </div>
        </section>

        <section class="panel">
          <div class="panel-head">
            <div>
              <h3>{{ i18n("settings.panelLimits") }}</h3>
              <p class="sub">{{ i18n("settings.panelLimitsDesc") }}</p>
            </div>
          </div>
          <el-form label-position="top" :disabled="!editing">
            <div class="field-grid">
              <el-form-item :label="i18n('settings.downloadLimit')">
                <el-input-number v-model="form.downloadLimitKB" :min="0" :max="1048576" controls-position="right" style="width: 100%" />
                <div class="hint">{{ i18n("settings.unlimitedSpeed") }}</div>
              </el-form-item>
              <el-form-item :label="i18n('settings.uploadLimit')">
                <el-input-number v-model="form.uploadLimitKB" :min="0" :max="1048576" controls-position="right" style="width: 100%" />
                <div class="hint">{{ i18n("settings.unlimitedSpeed") }}</div>
              </el-form-item>
              <el-form-item :label="i18n('settings.maxUpload')">
                <el-input-number v-model="form.maxUploadMB" :min="0" :max="1048576" controls-position="right" style="width: 100%" />
                <div class="hint">{{ i18n("settings.unlimitedSize") }}</div>
              </el-form-item>
              <el-form-item :label="i18n('settings.rateLimit')">
                <el-input-number v-model="form.rateLimitPerMin" :min="0" :max="100000" controls-position="right" style="width: 100%" />
                <div class="hint">{{ i18n("settings.rateLimitHint") }}</div>
              </el-form-item>
            </div>
            <el-form-item :label="i18n('settings.blacklist')">
              <el-input
                v-model="form.blacklist"
                type="textarea"
                :rows="3"
                :placeholder="i18n('settings.blacklistPlaceholder')"
              />
              <div class="hint">{{ i18n("settings.blacklistHint") }}</div>
            </el-form-item>
          </el-form>
        </section>

        <section class="panel">
          <div class="panel-head">
            <div>
              <h3>{{ i18n("settings.panelSecurity") }}</h3>
              <p class="sub">{{ i18n("settings.panelSecurityDesc") }}</p>
            </div>
          </div>
          <el-form label-position="top" :disabled="!editing">
            <el-form-item :label="i18n('settings.anonymousMode')">
              <el-radio-group v-model="form.anonymousMode" class="mode-radio">
                <el-radio value="deny">{{ i18n("settings.anonDeny") }}</el-radio>
                <el-radio value="readonly">{{ i18n("settings.anonReadonly") }}</el-radio>
                <el-radio value="upload">{{ i18n("settings.anonUpload") }}</el-radio>
              </el-radio-group>
            </el-form-item>
            <el-alert
              v-if="form.anonymousMode === 'deny' && !state.users.length"
              type="error"
              :closable="false"
              show-icon
              :title="i18n('settings.noUsersTitle')"
              :description="i18n('settings.noUsersDesc')"
            />
          </el-form>
        </section>

        <section class="panel">
          <div class="panel-head">
            <div>
              <h3>{{ i18n("settings.panelSyslog") }}</h3>
              <p class="sub">{{ i18n("settings.panelSyslogDesc") }}</p>
            </div>
          </div>
          <el-form label-position="top" :disabled="!editing">
            <div class="setting-row">
              <div>
                <strong>{{ i18n("settings.syslogEnabled") }}</strong>
                <p>{{ i18n("settings.syslogEnabledDesc") }}</p>
              </div>
              <el-switch v-model="form.syslogEnabled" :disabled="!editing" />
            </div>
            <el-form-item :label="i18n('settings.syslogTargets')">
              <div class="syslog-list">
                <div v-for="(target, index) in form.syslogTargets" :key="index" class="syslog-row">
                  <el-input v-model="target.host" :placeholder="i18n('settings.syslogHostPlaceholder')" style="width: 240px" />
                  <el-input-number v-model="target.port" :min="1" :max="65535" controls-position="right" style="width: 140px" />
                  <el-button text type="danger" :disabled="!editing" @click="removeSyslogTarget(index)">
                    {{ i18n("common.delete") }}
                  </el-button>
                </div>
                <el-button :icon="Plus" plain :disabled="!editing" @click="addSyslogTarget">
                  {{ i18n("settings.addTarget") }}
                </el-button>
              </div>
              <div class="hint">{{ i18n("settings.syslogNote") }}</div>
            </el-form-item>
          </el-form>
        </section>

        <section class="panel">
          <div class="panel-head">
            <div>
              <h3>{{ i18n("settings.panelSystem") }}</h3>
              <p class="sub">{{ i18n("settings.panelSystemDesc") }}</p>
            </div>
          </div>

          <div class="setting-row">
            <div>
              <strong>{{ i18n("settings.autoStart") }}</strong>
              <p>{{ i18n("settings.autoStartDesc") }}</p>
            </div>
            <el-switch
              :model-value="state.autoStart"
              :loading="autoStartLoading"
              :disabled="!editing"
              @change="(v: string | number | boolean) => toggleAutoStart(Boolean(v))"
            />
          </div>

          <div class="setting-row">
            <div>
              <strong>{{ i18n("settings.theme") }}</strong>
              <p>{{ i18n("settings.themeDesc") }}</p>
            </div>
            <el-radio-group v-model="currentTheme" size="small" :disabled="!editing" @change="applyTheme(currentTheme)">
              <el-radio-button value="light">{{ i18n("settings.light") }}</el-radio-button>
              <el-radio-button value="dark">{{ i18n("settings.dark") }}</el-radio-button>
            </el-radio-group>
          </div>

          <div class="setting-row">
            <div>
              <strong>{{ i18n("settings.language") }}</strong>
              <p>{{ i18n("settings.languageDesc") }}</p>
            </div>
            <el-radio-group v-model="form.language" size="small" :disabled="!editing" @change="onLanguageChange">
              <el-radio-button v-for="o in SUPPORTED_LOCALES" :key="o.value" :value="o.value">{{ o.label }}</el-radio-button>
            </el-radio-group>
          </div>
        </section>

        <div class="foot-note">
          <el-icon><Monitor /></el-icon>
          {{ i18n("settings.footNote") }}
        </div>
      </div>
    </main>
  </div>
</template>

<style scoped>
.field-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 20px;
}

.inline-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.syslog-list {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
  width: 100%;
}

.syslog-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.block-alert {
  margin-bottom: 16px;
  border-radius: 10px;
}

.url-preview {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  padding: 10px 12px;
  border-radius: 9px;
  border: 1px dashed var(--border);
  background: var(--panel-soft);
  font-size: 12px;
}

.url-preview .tag-soft {
  cursor: pointer;
}

.mode-radio {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 6px;
}

.setting-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 0;
  border-bottom: 1px solid var(--border);
}

.setting-row:last-of-type {
  border-bottom: none;
}

.setting-row.column {
  flex-direction: column;
  align-items: flex-start;
  gap: 10px;
}

.setting-row strong {
  font-size: 13.5px;
}

.setting-row p {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--muted);
}

.restart-hint {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  margin-top: 12px;
  gap: 6px;
  font-size: 12px;
  color: var(--muted);
}

.edit-btn-wrap {
  display: inline-flex;
}

.foot-note {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  font-size: 11px;
  color: var(--muted);
  opacity: 0.75;
  padding: 4px 0 10px;
}

@media (max-width: 820px) {
  .field-grid {
    grid-template-columns: 1fr;
  }
}
.refresh-btn {
  margin-right: 10px;
}
</style>
