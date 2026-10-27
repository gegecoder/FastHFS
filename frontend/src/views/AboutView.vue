<script setup lang="ts">
import { computed, onMounted } from "vue";
import { CopyDocument, Monitor, Refresh } from "@element-plus/icons-vue";
import * as Api from "../../bindings/FastHFS/app";
import { refreshEnv, state } from "../store";
import { guard } from "../ui";
import { i18n } from "../i18n";
import { useUpdate } from "../composables/useUpdate";
import PageHeader from "../components/PageHeader.vue";

const { checking, checkStatus, checkMessage, checkForUpdate } = useUpdate();

const runtime = computed(() => state.runtime as Record<string, string>);

const name = computed(() => state.version.name || "FastHFS");
const version = computed(() => state.version.version || "1.0.0");
const description = computed(() => state.version.description || i18n("about.defaultDescription"));
const author = computed(() => state.version.author || "—");
const blog = computed(() => state.version.blog || "");
const homepage = computed(() => state.version.homepage || "");
const copyright = computed(() => state.version.copyright || "");
const dataDir = computed(() => state.version.dataDir || runtime.value.dataDir || "");
const logDir = computed(() => runtime.value.logDir || "");

onMounted(() => {
  void refreshEnv();
});

async function openLink(url: string) {
  if (!url) return;
  await guard(() => Api.OpenURL(url), i18n("common.openSuccess"));
}

async function copy(text: string) {
  if (!text) return;
  await guard(() => Api.CopyText(text), i18n("common.copySuccess"));
}
</script>

<template>
  <div class="page">
    <PageHeader :title="i18n('about.title')" :desc="i18n('about.desc')" />
    <main class="content">
      <div class="page-stack">
        <section class="panel about-hero">
          <div class="about-main">
            <div class="about-logo">
              <svg viewBox="0 0 24 24" width="34" height="34" aria-hidden="true">
                <path
                  d="M3 7.5A2.5 2.5 0 0 1 5.5 5h4l2 2h7A2.5 2.5 0 0 1 21 9.5v8A2.5 2.5 0 0 1 18.5 20h-13A2.5 2.5 0 0 1 3 17.5v-10Z"
                  fill="currentColor"
                />
                <path d="M12 11v5m0 0 2.2-2.2M12 16l-2.2-2.2" stroke="#fff" stroke-width="1.4" stroke-linecap="round" />
              </svg>
            </div>
            <div class="about-copy">
              <div class="title-row">
                <h2>{{ name }}</h2>
                <span class="ver-chip mono">v{{ version }}</span>
                <el-button size="small" :icon="Refresh" :loading="checking" @click="checkForUpdate(false)">
                  {{ i18n("about.checkUpdate") }}
                </el-button>
              </div>
              <p>{{ description }}</p>
              <p v-if="checkMessage" class="check-msg" :class="checkStatus">{{ checkMessage }}</p>
            </div>
          </div>
          <div class="about-donate">
            <p class="donate-title">{{ i18n("about.donateTitle") }}</p>
            <img class="donate-img" src="/donate.png" :alt="i18n('about.donateAlt')" />
            <p class="donate-tip">{{ i18n("about.donateTip") }}</p>
          </div>
        </section>

        <div class="two-col">
          <section class="panel">
            <div class="panel-head">
              <div>
                <h3>{{ i18n("about.infoTitle") }}</h3>
                <p class="sub">{{ i18n("about.infoSub") }}</p>
              </div>
            </div>
            <div class="info-list">
              <div class="info-row">
                <span>{{ i18n("about.appName") }}</span>
                <strong>{{ name }}</strong>
              </div>
              <div class="info-row">
                <span>{{ i18n("about.version") }}</span>
                <strong class="mono">v{{ version }}</strong>
              </div>
              <div class="info-row">
                <span>{{ i18n("about.author") }}</span>
                <strong>{{ author }}</strong>
              </div>
              <div class="info-row">
                <span>{{ i18n("about.homepage") }}</span>
                <a class="link" href="javascript:void(0)" @click="openLink(homepage)">{{ homepage || "—" }}</a>
              </div>
              <div class="info-row">
                <span>{{ i18n("about.blog") }}</span>
                <a class="link" href="javascript:void(0)" @click="openLink(blog)">{{ blog || "—" }}</a>
              </div>
            </div>
          </section>

          <section class="panel">
            <div class="panel-head">
              <div>
                <h3>{{ i18n("about.runtimeTitle") }}</h3>
                <p class="sub">{{ i18n("about.runtimeSub") }}</p>
              </div>
            </div>
            <div class="info-list">
              <div class="info-row">
                <span>{{ i18n("about.os") }}</span>
                <strong class="mono">{{ runtime.os || "—" }} / {{ runtime.arch || "—" }}</strong>
              </div>
              <div class="info-row">
                <span>{{ i18n("about.dataDir") }}</span>
                <span class="dir-row">
                  <code class="mono ellipsis">{{ dataDir || "—" }}</code>
                  <el-button text size="small" :icon="CopyDocument" :disabled="!dataDir" @click="copy(dataDir)">
                    {{ i18n("common.copy") }}
                  </el-button>
                </span>
              </div>
              <div class="info-row">
                <span>{{ i18n("about.logDir") }}</span>
                <span class="dir-row">
                  <code class="mono ellipsis">{{ logDir || "—" }}</code>
                  <el-button text size="small" :icon="CopyDocument" :disabled="!logDir" @click="copy(logDir)">
                    {{ i18n("common.copy") }}
                  </el-button>
                </span>
              </div>
            </div>
          </section>
        </div>

        <div class="foot-note">
          <el-icon><Monitor /></el-icon>
          {{ copyright || i18n("about.footNote", { name }) }}
        </div>
      </div>
    </main>
  </div>
</template>

<style scoped>
.about-hero {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 196px;
  align-items: center;
  gap: 24px;
  background:
    radial-gradient(120% 150% at 0% 0%, var(--primary-soft) 0%, transparent 55%),
    var(--panel);
}

.about-main {
  display: flex;
  align-items: center;
  gap: 20px;
  min-width: 0;
}

.about-logo {
  width: 64px;
  height: 64px;
  flex: 0 0 64px;
  border-radius: 18px;
  display: grid;
  place-items: center;
  color: #fff;
  background: linear-gradient(145deg, #3b82f6, #1d4ed8);
  box-shadow: 0 10px 24px rgba(37, 99, 235, 0.28);
}

.about-copy h2 {
  margin: 0;
  font-size: 22px;
}

.title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.ver-chip {
  padding: 2px 10px;
  border-radius: 999px;
  background: var(--primary-soft);
  color: var(--primary);
  font-size: 12px;
  font-weight: 600;
}

.about-copy p {
  margin: 6px 0 0;
  color: var(--muted);
  font-size: 13px;
}

.check-msg.available {
  color: var(--primary);
}

.check-msg.error {
  color: #dc2626;
}

.about-donate {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 12px;
  border: 1px dashed var(--border);
  border-radius: 14px;
  background: var(--panel-soft);
}

.donate-title {
  margin: 0;
  font-size: 13px;
  font-weight: 600;
  letter-spacing: 0.02em;
}

.donate-img {
  display: block;
  width: 100%;
  max-width: 160px;
  height: auto;
  border-radius: 10px;
  border: 1px solid var(--border);
  background: #fff;
}

.donate-tip {
  margin: 0;
  font-size: 11.5px;
  line-height: 1.5;
  text-align: center;
  color: var(--muted);
}

.two-col {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.info-list {
  display: flex;
  flex-direction: column;
}

.info-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 10px 0;
  border-bottom: 1px solid var(--border);
  font-size: 13px;
}

.info-row:last-child {
  border-bottom: none;
}

.info-row > span:first-child {
  flex: 0 0 auto;
  color: var(--muted);
}

.info-row strong {
  font-weight: 600;
  text-align: right;
}

.link {
  color: var(--primary);
  text-decoration: none;
  text-align: right;
  word-break: break-all;
}

.link:hover {
  text-decoration: underline;
}

.dir-row {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.dir-row code {
  max-width: 360px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.foot-note {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  font-size: 11.5px;
  color: var(--muted);
  padding: 4px 0 10px;
}

@media (max-width: 1100px) {
  .two-col {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 760px) {
  .about-hero {
    grid-template-columns: 1fr;
    justify-items: center;
    text-align: center;
  }

  .about-main {
    flex-direction: column;
    text-align: center;
  }

  .title-row {
    justify-content: center;
  }
}
</style>
