<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from "vue";
import hljs from "highlight.js";
import * as pdfjsLib from "pdfjs-dist";
import type { PDFDocumentLoadingTask, PDFDocumentProxy, RenderTask } from "pdfjs-dist";
import workerUrl from "pdfjs-dist/build/pdf.worker.min.mjs?url";
import type { Entry } from "../types";
import { fileExtension, formatSize, formatTime, previewKind } from "../utils";
import { i18n } from "../i18n";
import FileIcon from "./FileIcon.vue";

pdfjsLib.GlobalWorkerOptions.workerSrc = workerUrl;

const props = defineProps<{
  entry: Entry;
  url: string;
  downloadUrl?: string | null;
}>();

const emit = defineEmits<{ close: [] }>();

const kind = previewKind(props.entry);
const loading = ref(kind !== "unsupported");
const error = ref("");
const objectUrl = ref("");
const textContent = ref("");
const language = ref("");
const lineNumbers = ref(localStorage.getItem("fasthfs-line-numbers") !== "0");

const pdfCanvas = ref<HTMLCanvasElement | null>(null);
const pdfPage = ref(1);
const pdfPages = ref(0);
const pdfScale = ref(1.15);
const pdfRendering = ref(false);

let pdfLoadingTask: PDFDocumentLoadingTask | null = null;
let pdfDocument: PDFDocumentProxy | null = null;
let renderTask: RenderTask | null = null;
let renderSequence = 0;
let disposed = false;

const LANGUAGE_MAP: Record<string, string> = {
  ts: "typescript",
  tsx: "typescript",
  mts: "typescript",
  cts: "typescript",
  js: "javascript",
  jsx: "javascript",
  mjs: "javascript",
  cjs: "javascript",
  md: "markdown",
  markdown: "markdown",
  py: "python",
  pyw: "python",
  rb: "ruby",
  rs: "rust",
  go: "go",
  kt: "kotlin",
  kts: "kotlin",
  sh: "bash",
  bash: "bash",
  zsh: "bash",
  fish: "bash",
  ps1: "powershell",
  yml: "yaml",
  toml: "ini",
  conf: "ini",
  cfg: "ini",
  properties: "ini",
  htm: "xml",
  html: "xml",
  vue: "xml",
  svelte: "xml",
  svg: "xml",
  cs: "csharp",
  cpp: "cpp",
  cc: "cpp",
  cxx: "cpp",
  hpp: "cpp",
  h: "c",
  c: "c",
};

const kindLabel = computed(() => {
  switch (kind) {
    case "image":
      return i18n("preview.kindImage");
    case "text":
      return i18n("preview.kindText");
    case "pdf":
      return i18n("preview.kindPdf");
    case "audio":
      return i18n("preview.kindAudio");
    default:
      return i18n("preview.kindFile");
  }
});

const metaText = computed(() => {
  const parts = [formatSize(props.entry.size), formatTime(props.entry.modTime)];
  return parts.filter((item) => item && item !== "—").join(" · ");
});

const lineCount = computed(() => (textContent.value ? textContent.value.split("\n").length : 0));

const highlighted = computed(() => {
  const raw = textContent.value;
  if (!raw) {
    return "";
  }
  if (raw.length > 400_000) {
    return escapeHtml(raw);
  }
  try {
    if (language.value && hljs.getLanguage(language.value)) {
      return hljs.highlight(raw, { language: language.value, ignoreIllegals: true }).value;
    }
    if (raw.length <= 80_000) {
      return hljs.highlightAuto(raw).value;
    }
  } catch {
    // 回退为纯文本
  }
  return escapeHtml(raw);
});

function escapeHtml(value: string): string {
  return value.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

async function fetchResource(url: string): Promise<Response> {
  const res = await fetch(url, { credentials: "same-origin" });
  if (!res.ok) {
    let message = i18n("preview.loadFailedHttp", { status: res.status });
    try {
      const data = (await res.json()) as { error?: string };
      if (data.error) {
        message = data.error;
      }
    } catch {
      // 非 JSON
    }
    throw new Error(message);
  }
  return res;
}

function releaseObjectUrl(): void {
  if (objectUrl.value) {
    URL.revokeObjectURL(objectUrl.value);
    objectUrl.value = "";
  }
}

async function loadContent(): Promise<void> {
  loading.value = true;
  error.value = "";
  try {
    if (kind === "image") {
      if (props.entry.size > 60 * 1024 * 1024) {
        throw new Error(i18n("preview.imageTooLarge"));
      }
      const res = await fetchResource(props.url);
      const blob = await res.blob();
      if (disposed) {
        return;
      }
      releaseObjectUrl();
      objectUrl.value = URL.createObjectURL(blob);
      loading.value = false;
      return;
    }
    if (kind === "text") {
      if (props.entry.size > 2 * 1024 * 1024) {
        throw new Error(i18n("preview.textTooLarge"));
      }
      const res = await fetchResource(props.url);
      const raw = (await res.text()).replace(/\r\n?/g, "\n");
      if (disposed) {
        return;
      }
      textContent.value = raw;
      const ext = fileExtension(props.entry.name);
      language.value = LANGUAGE_MAP[ext] ?? ext;
      loading.value = false;
      return;
    }
    if (kind === "pdf") {
      if (props.entry.size > 200 * 1024 * 1024) {
        throw new Error(i18n("preview.pdfTooLarge"));
      }
      const res = await fetchResource(props.url);
      const buffer = await res.arrayBuffer();
      if (disposed) {
        return;
      }
      pdfLoadingTask = pdfjsLib.getDocument({ data: new Uint8Array(buffer), isEvalSupported: false });
      pdfDocument = await pdfLoadingTask.promise;
      pdfPages.value = pdfDocument.numPages;
      pdfPage.value = 1;
      loading.value = false;
      await nextTick();
      await renderPdfPage();
      return;
    }
    loading.value = false;
  } catch (err) {
    if (disposed) {
      return;
    }
    error.value = err instanceof Error ? err.message : i18n("preview.loadFailed");
    loading.value = false;
  }
}

async function renderPdfPage(): Promise<void> {
  if (!pdfDocument || !pdfCanvas.value) {
    return;
  }
  const sequence = ++renderSequence;
  if (renderTask) {
    try {
      renderTask.cancel();
    } catch {
      // 忽略取消异常
    }
    renderTask = null;
  }
  pdfRendering.value = true;
  try {
    const page = await pdfDocument.getPage(pdfPage.value);
    if (sequence !== renderSequence) {
      return;
    }
    const canvas = pdfCanvas.value;
    if (!canvas) {
      return;
    }
    const ratio = Math.min(window.devicePixelRatio || 1, 2);
    const viewport = page.getViewport({ scale: pdfScale.value * ratio });
    canvas.width = Math.floor(viewport.width);
    canvas.height = Math.floor(viewport.height);
    canvas.style.width = `${Math.floor(viewport.width / ratio)}px`;
    canvas.style.height = `${Math.floor(viewport.height / ratio)}px`;
    const context = canvas.getContext("2d", { alpha: false });
    if (!context) {
      throw new Error(i18n("preview.noCanvas"));
    }
    const task = page.render({ canvasContext: context, viewport });
    renderTask = task;
    await task.promise;
  } catch (err) {
    if (!(err instanceof pdfjsLib.RenderingCancelledException)) {
      error.value = err instanceof Error ? err.message : i18n("preview.renderFailed");
    }
  } finally {
    if (sequence === renderSequence) {
      pdfRendering.value = false;
    }
  }
}

function changePage(delta: number): void {
  const next = Math.max(1, Math.min(pdfPages.value, pdfPage.value + delta));
  if (next === pdfPage.value) {
    return;
  }
  pdfPage.value = next;
  void renderPdfPage();
}

function changeScale(delta: number): void {
  const next = Math.max(0.5, Math.min(3, Math.round((pdfScale.value + delta) * 100) / 100));
  if (next === pdfScale.value) {
    return;
  }
  pdfScale.value = next;
  void renderPdfPage();
}

function resetScale(): void {
  pdfScale.value = 1.15;
  void renderPdfPage();
}

function toggleLineNumbers(): void {
  lineNumbers.value = !lineNumbers.value;
  localStorage.setItem("fasthfs-line-numbers", lineNumbers.value ? "1" : "0");
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === "Escape") {
    emit("close");
    return;
  }
  if (kind === "pdf") {
    if (event.key === "ArrowLeft") {
      changePage(-1);
    } else if (event.key === "ArrowRight") {
      changePage(1);
    }
  }
}

function cleanup(): void {
  disposed = true;
  renderSequence += 1;
  if (renderTask) {
    try {
      renderTask.cancel();
    } catch {
      // 忽略
    }
    renderTask = null;
  }
  if (pdfLoadingTask) {
    void pdfLoadingTask.destroy();
    pdfLoadingTask = null;
  }
  pdfDocument = null;
  releaseObjectUrl();
}

onMounted(() => {
  document.addEventListener("keydown", onKeydown);
  document.body.classList.add("modal-open");
  if (kind === "audio" || kind === "unsupported") {
    loading.value = false;
    return;
  }
  void loadContent();
});

onBeforeUnmount(() => {
  document.removeEventListener("keydown", onKeydown);
  document.body.classList.remove("modal-open");
  cleanup();
});
</script>

<template>
  <Teleport to="body">
    <div class="preview-overlay" role="presentation" @mousedown.self="emit('close')">
      <div class="preview-window" role="dialog" aria-modal="true" :aria-label="entry.name">
        <header class="preview-header">
          <FileIcon :name="entry.name" :is-dir="false" :mime="entry.mime" :size="20" />
          <div class="preview-heading">
            <div class="preview-name" :title="entry.name">{{ entry.name }}</div>
            <div class="preview-meta">{{ metaText }}<span v-if="kindLabel"> · {{ kindLabel }}</span></div>
          </div>
          <a v-if="downloadUrl" class="btn btn-sm" :href="downloadUrl">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <path d="M12 4.5V15M8 11.5l4 3.8 4-3.8" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" />
              <path d="M5 18.5h14" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" />
            </svg>
            {{ i18n("preview.download") }}
          </a>
          <button type="button" class="btn btn-ghost icon-btn" :aria-label="i18n('preview.closeLabel')" :title="i18n('preview.closeTitle')" @click="emit('close')">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none">
              <path d="M6 6l12 12M18 6L6 18" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
            </svg>
          </button>
        </header>

        <div class="preview-body" :class="`body-${kind}`">
          <div v-if="loading" class="preview-state">
            <span class="spinner spinner-lg" />
            <p class="state-title">{{ i18n("preview.loading") }}</p>
          </div>

          <div v-else-if="error" class="preview-state">
            <div class="state-icon state-error" aria-hidden="true">
              <svg width="26" height="26" viewBox="0 0 24 24" fill="none">
                <circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="1.6" />
                <path d="M12 7.5v5.2" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" />
                <circle cx="12" cy="16.4" r="1.05" fill="currentColor" />
              </svg>
            </div>
            <p class="state-title">{{ i18n("preview.loadFailed") }}</p>
            <p class="state-desc">{{ error }}</p>
            <div class="state-actions">
              <button type="button" class="btn" @click="loadContent">{{ i18n("common.retry") }}</button>
              <a v-if="downloadUrl" class="btn btn-primary" :href="downloadUrl">{{ i18n("preview.downloadFile") }}</a>
            </div>
          </div>

          <template v-else-if="kind === 'image'">
            <div class="image-stage">
              <img :src="objectUrl" :alt="entry.name" />
            </div>
          </template>

          <template v-else-if="kind === 'audio'">
            <div class="audio-stage">
              <div class="audio-mark" aria-hidden="true">
                <FileIcon :name="entry.name" :is-dir="false" :mime="entry.mime" :size="54" />
              </div>
              <p class="audio-name">{{ entry.name }}</p>
              <audio :src="url" controls preload="metadata">{{ i18n("preview.audioUnsupported") }}</audio>
            </div>
          </template>

          <template v-else-if="kind === 'pdf'">
            <div class="pdf-stage">
              <canvas ref="pdfCanvas" class="pdf-canvas" />
              <span v-if="pdfRendering" class="pdf-spinner spinner" />
            </div>
          </template>

          <template v-else-if="kind === 'text'">
            <div class="code-scroll" :class="{ 'with-lines': lineNumbers }">
              <div v-if="lineNumbers" class="code-gutter" aria-hidden="true">
                <span v-for="line in lineCount" :key="line">{{ line }}</span>
              </div>
              <pre class="code-block"><code class="hljs" v-html="highlighted"></code></pre>
            </div>
          </template>

          <div v-else class="preview-state">
            <div class="state-icon" aria-hidden="true">
              <FileIcon :name="entry.name" :is-dir="false" :mime="entry.mime" :size="52" />
            </div>
            <p class="state-title">{{ i18n("preview.unsupportedTitle") }}</p>
            <p class="state-desc">{{ i18n("preview.unsupportedDesc") }}</p>
            <div class="state-actions">
              <a v-if="downloadUrl" class="btn btn-primary" :href="downloadUrl">{{ i18n("preview.downloadFile") }}</a>
              <button v-else type="button" class="btn" @click="emit('close')">{{ i18n("common.close") }}</button>
            </div>
          </div>
        </div>

        <footer v-if="!loading && !error && (kind === 'pdf' || kind === 'text')" class="preview-footer">
          <template v-if="kind === 'pdf'">
            <button type="button" class="btn btn-sm" :disabled="pdfPage <= 1" @click="changePage(-1)">{{ i18n("preview.prevPage") }}</button>
            <span class="page-indicator">{{ pdfPage }} / {{ pdfPages }}</span>
            <button type="button" class="btn btn-sm" :disabled="pdfPage >= pdfPages" @click="changePage(1)">{{ i18n("preview.nextPage") }}</button>
            <span class="footer-sep" />
            <button type="button" class="btn btn-sm" :aria-label="i18n('preview.zoomOut')" @click="changeScale(-0.15)">−</button>
            <button type="button" class="btn btn-sm scale-text" :title="i18n('preview.resetZoom')" @click="resetScale">
              {{ Math.round((pdfScale / 1.15) * 100) }}%
            </button>
            <button type="button" class="btn btn-sm" :aria-label="i18n('preview.zoomIn')" @click="changeScale(0.15)">+</button>
          </template>
          <template v-else>
            <button type="button" class="btn btn-sm" :class="{ 'btn-primary': lineNumbers }" @click="toggleLineNumbers">
              {{ i18n("preview.lineNumbers") }}
            </button>
            <span class="footer-info">{{ i18n("preview.lineInfo", { n: lineCount, size: formatSize(entry.size) }) }}</span>
          </template>
        </footer>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.preview-overlay {
  position: fixed;
  inset: 0;
  z-index: 1100;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 22px;
  background: var(--overlay);
  backdrop-filter: blur(4px);
  animation: overlay-in 0.18s ease;
}

.preview-window {
  display: flex;
  flex-direction: column;
  width: min(1180px, 100%);
  height: min(92dvh, 100%);
  background: var(--bg-elev);
  border: 1px solid var(--border);
  border-radius: 16px;
  box-shadow: var(--shadow-lg);
  overflow: hidden;
  animation: card-in 0.2s cubic-bezier(0.2, 0.8, 0.2, 1);
}

.preview-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 11px 13px 11px 16px;
  border-bottom: 1px solid var(--border);
  background: var(--bg-elev);
  flex: none;
}

.preview-heading {
  flex: 1;
  min-width: 0;
}

.preview-name {
  font-size: 13.5px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.preview-meta {
  margin-top: 1px;
  color: var(--text-3);
  font-size: 11.5px;
}

.preview-body {
  flex: 1;
  min-height: 0;
  overflow: auto;
  background: var(--bg);
}

.preview-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  height: 100%;
  min-height: 240px;
  padding: 32px 24px;
  text-align: center;
}

.state-icon {
  display: inline-flex;
  padding: 12px;
  border-radius: 14px;
  background: var(--bg-subtle);
}

.state-error {
  color: var(--danger);
  background: var(--danger-soft);
}

.state-title {
  font-size: 14px;
  font-weight: 600;
}

.state-desc {
  max-width: 420px;
  color: var(--text-2);
  font-size: 12.5px;
  line-height: 1.65;
  word-break: break-word;
}

.state-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 6px;
}

.image-stage {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100%;
  padding: 22px;
}

.image-stage img {
  max-width: 100%;
  max-height: calc(92dvh - 160px);
  border-radius: 6px;
  box-shadow: var(--shadow-md);
  object-fit: contain;
}

.audio-stage {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 14px;
  height: 100%;
  min-height: 300px;
  padding: 32px;
}

.audio-mark {
  display: inline-flex;
  padding: 18px;
  border-radius: 20px;
  background: var(--bg-subtle);
}

.audio-name {
  max-width: 90%;
  font-size: 13.5px;
  font-weight: 600;
  text-align: center;
  word-break: break-all;
}

.audio-stage audio {
  width: min(460px, 100%);
}

.pdf-stage {
  position: relative;
  display: flex;
  justify-content: center;
  min-width: max-content;
  min-height: 100%;
  padding: 20px;
}

.pdf-canvas {
  display: block;
  max-width: none;
  border-radius: 4px;
  box-shadow: var(--shadow-md);
  background: #fff;
}

.pdf-spinner {
  position: sticky;
  bottom: 18px;
  align-self: flex-end;
  margin-left: -28px;
}

.code-scroll {
  display: flex;
  align-items: flex-start;
  min-height: 100%;
  overflow: auto;
  background: var(--code-bg);
  font-family: ui-monospace, "Cascadia Code", "SFMono-Regular", Consolas, "Liberation Mono", monospace;
  font-size: 12.5px;
  line-height: 1.65;
}

.code-gutter {
  position: sticky;
  left: 0;
  z-index: 1;
  display: flex;
  flex-direction: column;
  flex: none;
  min-width: 46px;
  padding: 16px 10px;
  background: var(--code-bg);
  border-right: 1px solid var(--border);
  color: var(--text-3);
  text-align: right;
  user-select: none;
  font-variant-numeric: tabular-nums;
}

.code-gutter span {
  display: block;
  height: 1.65em;
}

.code-block {
  flex: 1;
  margin: 0;
  padding: 16px 18px;
  min-width: max-content;
  tab-size: 4;
  white-space: pre;
}

.code-block code {
  font-family: inherit;
  line-height: inherit;
  white-space: pre;
}

.preview-footer {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: none;
  padding: 8px 14px;
  border-top: 1px solid var(--border);
  background: var(--bg-elev);
  overflow-x: auto;
}

.page-indicator {
  min-width: 64px;
  text-align: center;
  color: var(--text-2);
  font-size: 12.5px;
  font-variant-numeric: tabular-nums;
}

.scale-text {
  min-width: 58px;
  font-variant-numeric: tabular-nums;
}

.footer-sep {
  width: 1px;
  height: 18px;
  margin: 0 6px;
  background: var(--border);
}

.footer-info {
  color: var(--text-3);
  font-size: 12px;
  margin-left: auto;
}

@media (max-width: 720px) {
  .preview-overlay {
    padding: 0;
  }

  .preview-window {
    width: 100%;
    height: 100dvh;
    max-height: none;
    border-radius: 0;
    border: 0;
  }

  .image-stage img {
    max-height: calc(100dvh - 150px);
  }

  .preview-header {
    padding-left: 12px;
  }
}
</style>
