<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { ApiError, api, errorMessage, shareDownloadUrl, shareTusEndpoint, shareZipUrl, withBase } from "../api";
import { useTheme } from "../composables/useTheme";
import { useToast } from "../composables/useToast";
import { useUploader } from "../composables/useUploader";
import type { Entry, ListResult, ShareInfo, SortField, SortOrder, ViewMode } from "../types";
import { baseName, filterEntries, joinRel, sortEntries, triggerDownload } from "../utils";
import { i18n } from "../i18n";
import BreadcrumbNav from "../components/BreadcrumbNav.vue";
import FileBrowser from "../components/FileBrowser.vue";
import PreviewModal from "../components/PreviewModal.vue";
import UploadPanel from "../components/UploadPanel.vue";

const props = defineProps<{ token: string }>();

const VIEW_KEY = "fasthfs-webshare-view";
const homeUrl = withBase("/");
const { theme, toggleTheme } = useTheme();
const toast = useToast();

type Phase = "loading" | "locked" | "ready" | "error";
type StatusKind = "notFound" | "expired" | "offline" | "other";

const info = ref<ShareInfo | null>(null);
const phase = ref<Phase>("loading");
const statusKind = ref<StatusKind>("other");
const statusTitle = ref(i18n("share.cannotAccess"));
const statusMessage = ref("");
const password = ref("");
const unlocking = ref(false);
const unlockError = ref("");

const list = ref<ListResult | null>(null);
const loadingList = ref(false);
const listError = ref("");
const path = ref(readPathFromUrl());
const search = ref("");
const viewMode = ref<ViewMode>(localStorage.getItem(VIEW_KEY) === "grid" ? "grid" : "list");
const sortField = ref<SortField>("name");
const sortOrder = ref<SortOrder>("asc");
const previewTarget = ref<Entry | null>(null);
const collectSubdir = ref("");

const fileInput = ref<HTMLInputElement | null>(null);
const folderInput = ref<HTMLInputElement | null>(null);
let listRequestId = 0;
let sessionRenewing = false;

const uploader = useUploader({
  concurrency: 3,
  onError: (task) =>
    toast.error(i18n("share.uploadFailed", { name: task.name, error: task.error })),
});

const isCollect = computed(() => info.value?.mode === "collect");
const modeLabel = computed(() => {
  switch (info.value?.mode) {
    case "preview":
      return i18n("share.modePreview");
    case "download":
      return i18n("share.modeDownload");
    case "collect":
      return i18n("share.modeCollect");
    default:
      return i18n("share.modeShare");
  }
});
const rootLabel = computed(() => (info.value ? baseName(info.value.path) || info.value.mountName : i18n("share.modeShare")));
const entries = computed(() => sortEntries(filterEntries(list.value?.entries ?? [], search.value), sortField.value, sortOrder.value));
const successCount = computed(() => uploader.tasks.value.filter((task) => task.status === "success").length);
const hasFileTarget = computed(() => {
  if (!info.value || info.value.mode === "collect" || info.value.path === "") {
    return false;
  }
  const items = list.value?.entries ?? [];
  return items.length === 1 && !items[0].isDir && baseName(info.value.path) === items[0].name;
});

// 单文件分享的根就是该文件本身，相对路径必须为空，否则会拼接成 file/file。
function shareRel(entry: Entry): string {
  return hasFileTarget.value ? "" : joinRel(path.value, entry.name);
}

const previewUrl = computed(() => {
  if (!previewTarget.value) {
    return "";
  }
  return shareDownloadUrl(props.token, shareRel(previewTarget.value), true);
});

const previewDownloadUrl = computed(() => {
  if (!previewTarget.value || !list.value?.canDownload) {
    return null;
  }
  return shareDownloadUrl(props.token, shareRel(previewTarget.value), false);
});

function readPathFromUrl(): string {
  const value = new URLSearchParams(window.location.search).get("p") ?? "";
  return value.replace(/^\/+/, "");
}

function shareUrlFor(target: string): string {
  const base = withBase(`/s/${encodeURIComponent(props.token)}`);
  return target ? `${base}?p=${encodeURIComponent(target)}` : base;
}

function setStatus(error: unknown): void {
  const apiError = error instanceof ApiError ? error : null;
  const message = errorMessage(error);
  if (apiError?.status === 404) {
    statusKind.value = "notFound";
    statusTitle.value = i18n("share.notFoundTitle");
    statusMessage.value = i18n("share.notFoundDesc");
  } else if (apiError?.status === 410) {
    statusKind.value = "expired";
    statusTitle.value = i18n("share.expiredTitle");
    statusMessage.value = i18n("share.expiredDesc");
  } else if (apiError?.status === 0) {
    statusKind.value = "offline";
    statusTitle.value = i18n("share.noServerTitle");
    statusMessage.value = message;
  } else {
    statusKind.value = "other";
    statusTitle.value = i18n("share.cannotAccess");
    statusMessage.value = message;
  }
  phase.value = "error";
}

async function init(): Promise<void> {
  try {
    info.value = await api.shareInfo(props.token);
  } catch (err) {
    setStatus(err);
    return;
  }
  await ensureSession();
}

async function ensureSession(): Promise<void> {
  const current = info.value;
  if (!current) {
    return;
  }
  if (current.authenticated) {
    phase.value = "ready";
    if (current.mode !== "collect") {
      await loadShareList(path.value);
    }
    return;
  }
  if (current.hasPassword) {
    phase.value = "locked";
    return;
  }
  try {
    await api.shareUnlock(props.token, "");
    current.authenticated = true;
    phase.value = "ready";
    if (current.mode !== "collect") {
      await loadShareList(path.value);
    }
  } catch (err) {
    const apiError = err instanceof ApiError ? err : null;
    if (apiError?.status === 403) {
      phase.value = "locked";
    } else {
      setStatus(err);
    }
  }
}

async function submitPassword(): Promise<void> {
  if (unlocking.value) {
    return;
  }
  if (!password.value) {
    unlockError.value = i18n("share.passwordRequired");
    return;
  }
  unlocking.value = true;
  unlockError.value = "";
  try {
    await api.shareUnlock(props.token, password.value);
    if (info.value) {
      info.value.authenticated = true;
    }
    password.value = "";
    phase.value = "ready";
    if (info.value?.mode !== "collect") {
      await loadShareList(path.value);
    }
  } catch (err) {
    const apiError = err instanceof ApiError ? err : null;
    if (apiError?.status === 410 || apiError?.status === 404) {
      setStatus(err);
    } else {
      unlockError.value = errorMessage(err);
    }
  } finally {
    unlocking.value = false;
  }
}

async function loadShareList(target: string): Promise<void> {
  const requestId = ++listRequestId;
  loadingList.value = true;
  listError.value = "";
  try {
    const result = await api.shareList(props.token, target);
    if (requestId !== listRequestId) {
      return;
    }
    list.value = result;
    path.value = result.path;
  } catch (err) {
    if (requestId !== listRequestId) {
      return;
    }
    const apiError = err instanceof ApiError ? err : null;
    if (apiError?.status === 403 && apiError.code === "share_locked") {
      if (info.value) {
        info.value.authenticated = false;
      }
      if (!sessionRenewing && info.value && !info.value.hasPassword) {
        sessionRenewing = true;
        try {
          await api.shareUnlock(props.token, "");
          if (info.value) {
            info.value.authenticated = true;
          }
          await loadShareList(target);
        } catch (unlockErr) {
          const unlockApiError = unlockErr instanceof ApiError ? unlockErr : null;
          if (unlockApiError?.status === 410 || unlockApiError?.status === 404) {
            setStatus(unlockErr);
          } else {
            phase.value = "locked";
          }
        } finally {
          sessionRenewing = false;
        }
        return;
      }
      phase.value = "locked";
      return;
    }
    list.value = null;
    listError.value = errorMessage(err);
  } finally {
    if (requestId === listRequestId) {
      loadingList.value = false;
    }
  }
}

function navigate(target: string): void {
  if (target === path.value && list.value) {
    return;
  }
  if (target !== path.value) {
    history.pushState({ p: target }, "", shareUrlFor(target));
  }
  path.value = target;
  void loadShareList(target);
}

function onPopState(): void {
  const target = readPathFromUrl();
  if (target === path.value) {
    return;
  }
  path.value = target;
  if (phase.value === "ready" && info.value?.mode !== "collect") {
    void loadShareList(target);
  }
}

async function openEntry(entry: Entry): Promise<void> {
  if (!entry.isDir) {
    return;
  }
  navigate(joinRel(path.value, entry.name));
}

function openPreview(entry: Entry): void {
  previewTarget.value = entry;
}

function downloadEntry(entry: Entry): void {
  if (!list.value?.canDownload) {
    toast.info(i18n("share.previewOnlyToast"));
    return;
  }
  const rel = shareRel(entry);
  if (entry.isDir) {
    triggerDownload(shareZipUrl(props.token, rel));
  } else {
    triggerDownload(shareDownloadUrl(props.token, rel, false));
  }
}

function zipCurrent(): void {
  if (!list.value?.canDownload) {
    return;
  }
  triggerDownload(shareZipUrl(props.token, path.value));
}

function denied(): void {
  toast.info(i18n("share.previewOnlyToast"));
}

function setView(mode: ViewMode): void {
  viewMode.value = mode;
  localStorage.setItem(VIEW_KEY, mode);
}

function toggleSort(field: SortField): void {
  if (sortField.value === field) {
    sortOrder.value = sortOrder.value === "asc" ? "desc" : "asc";
    return;
  }
  sortField.value = field;
  sortOrder.value = field === "name" ? "asc" : "desc";
}

function toggleOrder(): void {
  sortOrder.value = sortOrder.value === "asc" ? "desc" : "asc";
}

function normalizedSubdir(): string | null {
  const value = collectSubdir.value.trim().replace(/\\/g, "/").replace(/^\/+|\/+$/g, "");
  if (!value) {
    return "";
  }
  const parts = value.split("/").filter((part) => part.length > 0);
  if (parts.some((part) => part === "." || part === "..")) {
    toast.error(i18n("share.invalidSubdir"));
    return null;
  }
  return parts.join("/");
}

function addCollectFiles(files: File[]): void {
  if (!files.length) {
    return;
  }
  const dir = normalizedSubdir();
  if (dir === null) {
    return;
  }
  const count = uploader.addFiles(files, { endpoint: shareTusEndpoint(props.token), dir });
  if (count > 0) {
    toast.success(i18n("share.queuedFiles", { count }));
  }
}

function chooseCollectFiles(): void {
  fileInput.value?.click();
}

function chooseCollectFolder(): void {
  folderInput.value?.click();
}

function onCollectPicked(event: Event): void {
  const input = event.target as HTMLInputElement;
  const files = Array.from(input.files ?? []);
  input.value = "";
  addCollectFiles(files);
}

function onDrop(event: DragEvent): void {
  const files = Array.from(event.dataTransfer?.files ?? []);
  addCollectFiles(files);
}

function retryInit(): void {
  phase.value = "loading";
  void init();
}

function retryList(): void {
  void loadShareList(path.value);
}

onMounted(() => {
  window.addEventListener("popstate", onPopState);
  void init();
});

onBeforeUnmount(() => {
  window.removeEventListener("popstate", onPopState);
  uploader.reset();
  listRequestId += 1;
});
</script>

<template>
  <div class="share-view">
    <header class="share-header">
      <a class="brand" :href="homeUrl" :aria-label="i18n('share.backHomeLabel')">
        <span class="brand-mark" aria-hidden="true">
          <svg width="28" height="28" viewBox="0 0 28 28" fill="none">
            <rect x="1" y="1" width="26" height="26" rx="7" fill="var(--primary)" />
            <path d="M8 8.5h8.5M8 14h6M8 19.5h9.5" stroke="#fff" stroke-width="2.2" stroke-linecap="round" />
          </svg>
        </span>
        <span class="brand-name">FastHFS</span>
      </a>
      <div class="share-meta" v-if="info">
        <span class="mode-chip">{{ modeLabel }}</span>
        <span class="mount-name" :title="info.mountName">{{ info.mountName }}</span>
      </div>
      <div class="header-actions">
        <button
          type="button"
          class="btn btn-ghost icon-btn"
          :title="theme === 'dark' ? i18n('theme.toLight') : i18n('theme.toDark')"
          :aria-label="theme === 'dark' ? i18n('theme.toLight') : i18n('theme.toDark')"
          @click="toggleTheme"
        >
          <svg v-if="theme === 'dark'" width="17" height="17" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <circle cx="12" cy="12" r="4" stroke="currentColor" stroke-width="1.6" />
            <path d="M12 3v2.2M12 18.8V21M3 12h2.2M18.8 12H21M5.8 5.8l1.6 1.6M16.6 16.6l1.6 1.6M18.2 5.8l-1.6 1.6M7.4 16.6l-1.6 1.6" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
          </svg>
          <svg v-else width="17" height="17" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path d="M20 14.2A8.2 8.2 0 0 1 9.8 4a8.4 8.4 0 1 0 10.2 10.2z" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round" />
          </svg>
        </button>
      </div>
    </header>

    <div v-if="phase === 'ready' && !isCollect" class="share-subbar">
      <div class="subbar-inner">
        <BreadcrumbNav :root-label="rootLabel" :path="path" @navigate="navigate" />
        <div class="subbar-tools">
          <label class="search-box">
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <circle cx="10.8" cy="10.8" r="6.3" stroke="currentColor" stroke-width="1.7" />
              <path d="M15.5 15.5L20 20" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" />
            </svg>
            <input v-model="search" type="text" :placeholder="i18n('share.searchPlaceholder')" autocomplete="off" spellcheck="false" :aria-label="i18n('share.searchLabel')" />
          </label>
          <div class="segmented" role="group" :aria-label="i18n('view.switch')">
            <button
              type="button"
              class="segment"
              :class="{ active: viewMode === 'list' }"
              :title="i18n('view.list')"
              :aria-label="i18n('view.list')"
              @click="setView('list')"
            >
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path d="M5 7h14M5 12h14M5 17h14" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
              </svg>
            </button>
            <button
              type="button"
              class="segment"
              :class="{ active: viewMode === 'grid' }"
              :title="i18n('view.grid')"
              :aria-label="i18n('view.grid')"
              @click="setView('grid')"
            >
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <rect x="4" y="4" width="6.4" height="6.4" rx="1.2" stroke="currentColor" stroke-width="1.4" />
                <rect x="13.6" y="4" width="6.4" height="6.4" rx="1.2" stroke="currentColor" stroke-width="1.4" />
                <rect x="4" y="13.6" width="6.4" height="6.4" rx="1.2" stroke="currentColor" stroke-width="1.4" />
                <rect x="13.6" y="13.6" width="6.4" height="6.4" rx="1.2" stroke="currentColor" stroke-width="1.4" />
              </svg>
            </button>
          </div>
        </div>
      </div>
    </div>

    <main class="share-main">
      <div v-if="phase === 'loading'" class="center-state">
        <span class="spinner spinner-lg" />
        <p class="state-desc">{{ i18n("share.opening") }}</p>
      </div>

      <div v-else-if="phase === 'error'" class="share-card status-card">
        <div class="status-icon" aria-hidden="true">
          <svg v-if="statusKind === 'expired'" width="34" height="34" viewBox="0 0 24 24" fill="none">
            <circle cx="12" cy="12" r="8.7" stroke="currentColor" stroke-width="1.5" />
            <path d="M12 7.6V12l3 2" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" />
          </svg>
          <svg v-else width="34" height="34" viewBox="0 0 24 24" fill="none">
            <path d="M9.2 14.8l5.6-5.6M8 7.2l8 8" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" />
            <circle cx="12" cy="12" r="8.7" stroke="currentColor" stroke-width="1.5" />
          </svg>
        </div>
        <h1 class="card-title">{{ statusTitle }}</h1>
        <p class="card-desc">{{ statusMessage }}</p>
        <div class="card-actions">
          <button type="button" class="btn" @click="retryInit">{{ i18n("common.retry") }}</button>
          <a class="btn btn-primary" :href="homeUrl">{{ i18n("share.backHome") }}</a>
        </div>
      </div>

      <div v-else-if="phase === 'locked'" class="share-card lock-card">
        <div class="lock-icon" aria-hidden="true">
          <svg width="30" height="30" viewBox="0 0 24 24" fill="none">
            <rect x="4.5" y="10.5" width="15" height="9.5" rx="2.2" stroke="currentColor" stroke-width="1.6" />
            <path d="M8 10.5V8a4 4 0 0 1 8 0v2.5" stroke="currentColor" stroke-width="1.6" />
            <circle cx="12" cy="15.2" r="1.2" fill="currentColor" />
          </svg>
        </div>
        <h1 class="card-title">{{ i18n("share.lockedTitle") }}</h1>
        <p class="card-desc">{{ i18n("share.lockedDesc", { mount: info?.mountName ?? "" }) }}</p>
        <form class="lock-form" @submit.prevent="submitPassword">
          <input
            v-model="password"
            class="input"
            type="password"
            autocomplete="off"
            :placeholder="i18n('share.passwordPlaceholder')"
            autofocus
            @input="unlockError = ''"
          />
          <button type="submit" class="btn btn-primary" :disabled="unlocking">
            <span v-if="unlocking" class="spinner" style="width: 14px; height: 14px" />
            {{ unlocking ? i18n("share.verifying") : i18n("share.enter") }}
          </button>
        </form>
        <p v-if="unlockError" class="form-error lock-error">{{ unlockError }}</p>
      </div>

      <template v-else-if="phase === 'ready'">
        <section v-if="isCollect" class="collect-layout">
          <div class="collect-card">
            <div class="collect-head">
              <div class="collect-icon" aria-hidden="true">
                <svg width="32" height="32" viewBox="0 0 24 24" fill="none">
                  <path d="M3.5 7.2A2.2 2.2 0 0 1 5.7 5h4l2 2.3h6.6a2.2 2.2 0 0 1 2.2 2.2v7.3a2.2 2.2 0 0 1-2.2 2.2H5.7a2.2 2.2 0 0 1-2.2-2.2z" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" />
                  <path d="M12 16.8v-5.4M9.5 13.9l2.5-2.5 2.5 2.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
                </svg>
              </div>
              <h1 class="card-title">{{ i18n("share.collectTitle") }}</h1>
              <p class="card-desc">
                {{ i18n("share.collectDesc", { mount: info?.mountName ?? "" }) }}
              </p>
            </div>

            <label class="form-row collect-subdir">
              <span class="form-label">{{ i18n("share.subdirLabel") }}</span>
              <input v-model="collectSubdir" class="input" type="text" :placeholder="i18n('share.subdirPlaceholder')" />
            </label>

            <div
              class="drop-zone"
              @dragenter.prevent
              @dragover.prevent
              @drop.prevent="onDrop"
              @click="chooseCollectFiles"
            >
              <div class="drop-icon" aria-hidden="true">
                <svg width="30" height="30" viewBox="0 0 24 24" fill="none">
                  <path d="M12 16.5V5M8 9l4-4 4 4" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" />
                  <path d="M5 16.5v2A1.5 1.5 0 0 0 6.5 20h11a1.5 1.5 0 0 0 1.5-1.5v-2" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" />
                </svg>
              </div>
              <p class="drop-title">{{ i18n("share.dropTitle") }}</p>
              <p class="drop-desc">{{ i18n("share.dropDesc") }}</p>
              <div class="drop-actions">
                <button type="button" class="btn btn-primary" @click.stop="chooseCollectFiles">{{ i18n("share.chooseFiles") }}</button>
                <button type="button" class="btn" @click.stop="chooseCollectFolder">{{ i18n("share.chooseFolder") }}</button>
              </div>
            </div>

            <div v-if="successCount > 0" class="collect-success">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="1.6" />
                <path d="M8.2 12.4l2.6 2.6 5-5.4" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
              </svg>
              {{ i18n("share.collectSuccess", { count: successCount }) }}
            </div>

            <p class="collect-note">{{ i18n("share.collectNote", { dir: collectSubdir.trim() ? ` / ${collectSubdir.trim()}` : "" }) }}</p>

            <input ref="fileInput" class="sr-only" type="file" multiple @change="onCollectPicked" />
            <input ref="folderInput" class="sr-only" type="file" multiple webkitdirectory @change="onCollectPicked" />
          </div>
        </section>

        <template v-else>
          <div v-if="listError" class="center-card">
            <div class="state-icon" aria-hidden="true">
              <svg width="30" height="30" viewBox="0 0 24 24" fill="none">
                <circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="1.5" />
                <path d="M12 7.5v5.2" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
                <circle cx="12" cy="16.4" r="1.05" fill="currentColor" />
              </svg>
            </div>
            <h2 class="state-title">{{ i18n("share.listErrorTitle") }}</h2>
            <p class="state-desc">{{ listError }}</p>
            <button type="button" class="btn btn-primary" @click="retryList">{{ i18n("common.retry") }}</button>
          </div>

          <template v-else-if="hasFileTarget">
            <div class="single-file-card">
              <div class="single-file-main">
                <div class="single-file-name">{{ entries[0]?.name }}</div>
                <div class="single-file-desc">
                  {{ info?.mountName }} · {{ modeLabel }}
                  <template v-if="list?.canDownload"> · {{ i18n("share.downloadable") }}</template>
                  <template v-else> · {{ i18n("share.previewOnly") }}</template>
                </div>
              </div>
              <div class="single-file-actions">
                <button type="button" class="btn" @click="entries[0] && openPreview(entries[0])">{{ i18n("share.previewOnline") }}</button>
                <button
                  v-if="list?.canDownload"
                  type="button"
                  class="btn btn-primary"
                  @click="entries[0] && downloadEntry(entries[0])"
                >
                  {{ i18n("share.downloadFile") }}
                </button>
              </div>
            </div>
            <p v-if="info?.mode === 'preview'" class="preview-only-note">
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="1.5" />
                <path d="M12 11v5" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" />
                <circle cx="12" cy="7.9" r="1.05" fill="currentColor" />
              </svg>
              {{ i18n("share.previewOnlyNote") }}
            </p>
          </template>

          <template v-else>
            <FileBrowser
              :list="list"
              :entries="entries"
              :view-mode="viewMode"
              :sort-field="sortField"
              :sort-order="sortOrder"
              :loading="loadingList"
              :previewable="true"
              :can-zip="info?.mode === 'download'"
              @open="openEntry"
              @preview="openPreview"
              @download="downloadEntry"
              @denied="denied"
              @sort="toggleSort"
              @toggle-order="toggleOrder"
              @zip="zipCurrent"
            />
            <p v-if="info?.mode === 'preview'" class="preview-only-note">
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="1.5" />
                <path d="M12 11v5" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" />
                <circle cx="12" cy="7.9" r="1.05" fill="currentColor" />
              </svg>
              {{ i18n("share.previewOnlyNote") }}
            </p>
          </template>
        </template>
      </template>
    </main>

    <UploadPanel
      v-if="uploader.tasks.value.length > 0 || (phase === 'ready' && isCollect)"
      :tasks="uploader.tasks.value"
      :uploading-count="uploader.uploadingCount.value"
      :finished-count="uploader.finishedCount.value"
      :total-progress="uploader.totalProgress.value"
      @cancel="uploader.cancel"
      @retry="uploader.retry"
      @remove="uploader.remove"
      @clear="uploader.clearFinished"
    />

    <PreviewModal
      v-if="previewTarget"
      :entry="previewTarget"
      :url="previewUrl"
      :download-url="previewDownloadUrl"
      @close="previewTarget = null"
    />
  </div>
</template>

<style scoped>
.share-view {
  display: flex;
  flex-direction: column;
  min-height: 100dvh;
}

.share-header {
  position: sticky;
  top: 0;
  z-index: 100;
  display: flex;
  align-items: center;
  gap: 10px;
  max-width: 1240px;
  width: 100%;
  margin: 0 auto;
  padding: 10px 22px;
}

.brand {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex: none;
  color: var(--text);
  text-decoration: none;
}

.brand:hover {
  text-decoration: none;
}

.brand-mark {
  display: inline-flex;
}

.brand-name {
  font-size: 15px;
  font-weight: 650;
  letter-spacing: -0.02em;
}

.share-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  margin-left: 4px;
}

.mode-chip {
  display: inline-flex;
  align-items: center;
  height: 23px;
  padding: 0 8px;
  border-radius: 6px;
  background: var(--primary-soft);
  color: var(--primary);
  font-size: 11.5px;
  font-weight: 600;
  white-space: nowrap;
}

.mount-name {
  color: var(--text-3);
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-left: auto;
}

.share-subbar {
  border-top: 1px solid var(--border);
  border-bottom: 1px solid var(--border);
  background: var(--bg-elev);
}

.subbar-inner {
  display: flex;
  align-items: center;
  gap: 12px;
  max-width: 1240px;
  margin: 0 auto;
  padding: 5px 18px;
  min-height: 42px;
}

.subbar-tools {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-left: auto;
  flex: none;
}

.search-box {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 170px;
  height: 30px;
  padding: 0 9px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg-elev);
  color: var(--text-3);
}

.search-box:focus-within {
  border-color: var(--primary);
}

.search-box input {
  flex: 1;
  min-width: 0;
  border: 0;
  outline: none;
  background: transparent;
  color: var(--text);
  font-size: 12.5px;
}

.segmented {
  display: inline-flex;
  padding: 2px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg-elev);
}

.segment {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 26px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--text-3);
}

.segment:hover {
  color: var(--text);
}

.segment.active {
  background: var(--bg-active);
  color: var(--text);
}

.share-main {
  flex: 1;
  width: 100%;
  max-width: 1240px;
  margin: 0 auto;
  padding: 16px 22px 90px;
}

.center-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 100px 20px;
  color: var(--text-2);
}

.share-card {
  max-width: 430px;
  margin: 70px auto;
  padding: 30px 26px;
  border: 1px solid var(--border);
  border-radius: 18px;
  background: var(--bg-elev);
  box-shadow: var(--shadow-md);
  text-align: center;
}

.status-icon,
.lock-icon {
  display: inline-flex;
  margin-bottom: 12px;
  padding: 14px;
  border-radius: 16px;
  background: var(--bg-subtle);
  color: var(--text-2);
}

.lock-icon {
  color: var(--primary);
  background: var(--primary-soft);
}

.card-title {
  font-size: 17px;
  font-weight: 650;
  letter-spacing: -0.01em;
}

.card-desc {
  margin-top: 6px;
  color: var(--text-2);
  font-size: 12.5px;
  line-height: 1.7;
}

.card-actions {
  display: flex;
  justify-content: center;
  gap: 8px;
  margin-top: 18px;
}

.lock-form {
  display: flex;
  gap: 8px;
  margin-top: 18px;
}

.lock-form .input {
  flex: 1;
}

.lock-error {
  margin-top: 10px;
  text-align: left;
}

.collect-layout {
  max-width: 640px;
  margin: 26px auto;
}

.collect-card {
  padding: 26px;
  border: 1px solid var(--border);
  border-radius: 18px;
  background: var(--bg-elev);
  box-shadow: var(--shadow-sm);
}

.collect-head {
  text-align: center;
  margin-bottom: 20px;
}

.collect-icon {
  display: inline-flex;
  margin-bottom: 10px;
  padding: 14px;
  border-radius: 16px;
  background: var(--primary-soft);
  color: var(--primary);
}

.collect-subdir {
  margin-bottom: 14px;
}

.drop-zone {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding: 30px 18px;
  border: 1.5px dashed var(--border-strong);
  border-radius: 14px;
  background: var(--bg-subtle);
  cursor: pointer;
  text-align: center;
  transition: border-color 0.15s ease, background-color 0.15s ease;
}

.drop-zone:hover {
  border-color: var(--primary);
  background: var(--primary-soft);
}

.drop-icon {
  display: inline-flex;
  margin-bottom: 6px;
  padding: 11px;
  border-radius: 14px;
  background: var(--bg-elev);
  color: var(--primary);
  box-shadow: var(--shadow-sm);
}

.drop-title {
  font-size: 13.5px;
  font-weight: 600;
}

.drop-desc {
  color: var(--text-3);
  font-size: 11.5px;
}

.drop-actions {
  display: flex;
  gap: 8px;
  margin-top: 12px;
}

.collect-success {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 14px;
  padding: 9px 12px;
  border-radius: 9px;
  background: var(--success-soft);
  color: var(--success);
  font-size: 12.5px;
}

.collect-note {
  margin-top: 12px;
  color: var(--text-3);
  font-size: 11.5px;
  text-align: center;
}

.single-file-card {
  display: flex;
  align-items: center;
  gap: 14px;
  margin: 20px 0;
  padding: 16px 18px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--bg-elev);
}

.single-file-main {
  flex: 1;
  min-width: 0;
}

.single-file-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: none;
}

.single-file-name {
  font-size: 14px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.single-file-desc {
  margin-top: 3px;
  color: var(--text-3);
  font-size: 11.5px;
}

.preview-only-note {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 12px;
  color: var(--text-3);
  font-size: 11.5px;
}

.center-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  max-width: 460px;
  margin: 50px auto;
  padding: 30px 24px;
  border: 1px solid var(--border);
  border-radius: 16px;
  background: var(--bg-elev);
  text-align: center;
}

.state-icon {
  display: inline-flex;
  margin-bottom: 4px;
  padding: 13px;
  border-radius: 15px;
  background: var(--bg-subtle);
  color: var(--text-2);
}

.state-title {
  font-size: 16px;
  font-weight: 650;
}

.state-desc {
  color: var(--text-2);
  font-size: 12.5px;
  line-height: 1.7;
}

@media (max-width: 760px) {
  .share-header {
    padding: 9px 14px;
  }

  .brand-name {
    display: none;
  }

  .share-subbar .subbar-inner {
    flex-wrap: wrap;
    padding: 7px 12px;
    row-gap: 7px;
  }

  .subbar-tools {
    flex: 1 1 100%;
  }

  .search-box {
    flex: 1;
    width: auto;
  }

  .share-main {
    padding: 12px 12px 90px;
  }

  .share-card {
    margin: 36px auto;
    padding: 24px 18px;
  }

  .collect-layout {
    margin: 10px auto;
  }

  .collect-card {
    padding: 18px 16px;
    border-radius: 16px;
  }

  .lock-form {
    flex-direction: column;
  }

  .single-file-card {
    flex-wrap: wrap;
  }

  .single-file-actions {
    width: 100%;
  }

  .single-file-actions .btn {
    flex: 1;
  }
}
</style>
