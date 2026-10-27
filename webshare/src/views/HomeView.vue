<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { api, downloadUrl, errorMessage, tusEndpoint, zipUrl } from "../api";
import { useTheme } from "../composables/useTheme";
import { useToast } from "../composables/useToast";
import { useUploader } from "../composables/useUploader";
import type { AuthInfo, Entry, ListResult, MountOption, SortField, SortOrder, ViewMode } from "../types";
import { filterEntries, joinRel, sortEntries, triggerDownload } from "../utils";
import { i18n } from "../i18n";
import BreadcrumbNav from "../components/BreadcrumbNav.vue";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import FileBrowser from "../components/FileBrowser.vue";
import InputDialog from "../components/InputDialog.vue";
import LoginDialog from "../components/LoginDialog.vue";
import PreviewModal from "../components/PreviewModal.vue";
import ShareCreateDialog from "../components/ShareCreateDialog.vue";
import UploadPanel from "../components/UploadPanel.vue";

const VIEW_KEY = "fasthfs-webshare-view";
const MOUNT_KEY = "fasthfs-webshare-mount";

const { theme, toggleTheme } = useTheme();
const toast = useToast();

const auth = ref<AuthInfo | null>(null);
const mounts = ref<MountOption[]>([]);
const authRequired = ref(false);
const loadingMounts = ref(true);
const mountError = ref("");
const currentMount = ref("");
const list = ref<ListResult | null>(null);
const loadingList = ref(false);
const listError = ref("");
const path = ref("");
const search = ref("");
const viewMode = ref<ViewMode>(localStorage.getItem(VIEW_KEY) === "grid" ? "grid" : "list");
const sortField = ref<SortField>("name");
const sortOrder = ref<SortOrder>("asc");

const showLogin = ref(false);
const showUploadMenu = ref(false);
const showUserMenu = ref(false);
const inputDialog = ref<{ mode: "mkdir" | "rename"; entry?: Entry } | null>(null);
const deleteTarget = ref<Entry | null>(null);
const deleteBusy = ref(false);
const shareTarget = ref<Entry | null>(null);
const previewTarget = ref<Entry | null>(null);

const fileInput = ref<HTMLInputElement | null>(null);
const folderInput = ref<HTMLInputElement | null>(null);
let refreshTimer = 0;
let listRequestId = 0;

const uploader = useUploader({
  concurrency: 3,
  onComplete: () => scheduleRefresh(),
  onError: (task) =>
    toast.error(i18n("home.uploadFailed", { name: task.name, error: task.error })),
});

const loginRequired = computed(() => authRequired.value && !auth.value?.authenticated);
const currentMountOption = computed(() => mounts.value.find((item) => item.alias === currentMount.value) ?? null);
const canUpload = computed(() => list.value?.canUpload ?? false);
const rootLabel = computed(() => list.value?.mountName || currentMountOption.value?.name || currentMount.value || "FastHFS");
const permissionBadge = computed(() => {
  if (auth.value?.authenticated) {
    return i18n("home.loggedIn");
  }
  switch (auth.value?.anonymousMode) {
    case "readonly":
      return i18n("home.anonymousReadonly");
    case "upload":
      return i18n("home.anonymousUpload");
    default:
      return i18n("home.anonymousAccess");
  }
});
const userInitial = computed(() => (auth.value?.username ? auth.value.username.slice(0, 1).toUpperCase() : "U"));

const entries = computed(() => sortEntries(filterEntries(list.value?.entries ?? [], search.value), sortField.value, sortOrder.value));

const previewDownloadUrl = computed(() => {
  if (!previewTarget.value || !currentMount.value || !list.value?.canDownload) {
    return null;
  }
  return downloadUrl(currentMount.value, joinRel(path.value, previewTarget.value.name), false);
});

const previewUrl = computed(() => {
  if (!previewTarget.value || !currentMount.value) {
    return "";
  }
  return downloadUrl(currentMount.value, joinRel(path.value, previewTarget.value.name), true);
});

const inputDialogTitle = computed(() =>
  inputDialog.value?.mode === "rename" ? i18n("home.rename") : i18n("home.newFolder"),
);
const inputDialogLabel = computed(() =>
  inputDialog.value?.mode === "rename" ? i18n("home.newName") : i18n("home.folderName"),
);
const inputDialogConfirm = computed(() =>
  inputDialog.value?.mode === "rename" ? i18n("common.save") : i18n("home.create"),
);
const deleteMessage = computed(() => {
  if (!deleteTarget.value) {
    return "";
  }
  const suffix = deleteTarget.value.isDir ? i18n("home.deleteSuffixDir") : i18n("home.deleteSuffixFile");
  return i18n("home.deleteMessage", { name: deleteTarget.value.name, suffix });
});

async function loadList(target: string): Promise<void> {
  if (!currentMount.value) {
    return;
  }
  const requestId = ++listRequestId;
  loadingList.value = true;
  listError.value = "";
  try {
    const result = await api.list(currentMount.value, target);
    if (requestId !== listRequestId) {
      return;
    }
    list.value = result;
    path.value = result.path;
  } catch (err) {
    if (requestId !== listRequestId) {
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

async function bootstrap(): Promise<void> {
  loadingMounts.value = true;
  mountError.value = "";
  try {
    const [me, mountsResult] = await Promise.all([api.me().catch(() => null), api.mounts()]);
    if (me) {
      auth.value = me;
    }
    authRequired.value = mountsResult.authRequired;
    mounts.value = mountsResult.mounts;
    if (mountsResult.authRequired) {
      showLogin.value = true;
    }
    const stored = localStorage.getItem(MOUNT_KEY);
    const target = mounts.value.find((item) => item.alias === stored) ?? mounts.value[0] ?? null;
    if (target) {
      currentMount.value = target.alias;
      localStorage.setItem(MOUNT_KEY, target.alias);
      await loadList("");
    } else {
      currentMount.value = "";
      list.value = null;
      path.value = "";
    }
  } catch (err) {
    mountError.value = errorMessage(err);
  } finally {
    loadingMounts.value = false;
  }
}

function scheduleRefresh(): void {
  if (refreshTimer) {
    window.clearTimeout(refreshTimer);
  }
  refreshTimer = window.setTimeout(() => {
    refreshTimer = 0;
    if (currentMount.value) {
      void loadList(path.value);
    }
  }, 450);
}

async function selectMount(alias: string): Promise<void> {
  if (!alias || alias === currentMount.value) {
    return;
  }
  currentMount.value = alias;
  localStorage.setItem(MOUNT_KEY, alias);
  path.value = "";
  search.value = "";
  list.value = null;
  await loadList("");
}

function onMountChange(event: Event): void {
  const target = event.target as HTMLSelectElement;
  void selectMount(target.value);
}

async function navigate(target: string): Promise<void> {
  if (target === path.value && list.value) {
    return;
  }
  await loadList(target);
}

async function openEntry(entry: Entry): Promise<void> {
  await navigate(joinRel(path.value, entry.name));
}

function openPreview(entry: Entry): void {
  previewTarget.value = entry;
}

function downloadEntry(entry: Entry): void {
  if (!currentMount.value) {
    return;
  }
  const rel = joinRel(path.value, entry.name);
  if (entry.isDir) {
    triggerDownload(zipUrl(currentMount.value, rel));
  } else {
    triggerDownload(downloadUrl(currentMount.value, rel, false));
  }
}

function zipCurrent(): void {
  if (!currentMount.value) {
    return;
  }
  triggerDownload(zipUrl(currentMount.value, path.value));
}

function denied(): void {
  toast.info(i18n("home.noPreviewDownloadPermission"));
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

function openMkdir(): void {
  if (!canUpload.value) {
    return;
  }
  inputDialog.value = { mode: "mkdir" };
}

function openRename(entry: Entry): void {
  inputDialog.value = { mode: "rename", entry };
}

async function confirmInput(name: string): Promise<void> {
  const dialog = inputDialog.value;
  if (!dialog || !currentMount.value) {
    return;
  }
  try {
    if (dialog.mode === "mkdir") {
      const result = await api.mkdir(currentMount.value, path.value, name);
      toast.success(result.message || i18n("home.folderCreated"));
    } else if (dialog.entry) {
      const result = await api.rename(currentMount.value, joinRel(path.value, dialog.entry.name), name);
      toast.success(result.message || i18n("home.renamed"));
    }
    inputDialog.value = null;
    await loadList(path.value);
  } catch (err) {
    toast.error(errorMessage(err));
  }
}

async function confirmDelete(): Promise<void> {
  const target = deleteTarget.value;
  if (!target || !currentMount.value || deleteBusy.value) {
    return;
  }
  deleteBusy.value = true;
  try {
    await api.remove(currentMount.value, [joinRel(path.value, target.name)]);
    toast.success(i18n("home.deleted"));
    deleteTarget.value = null;
    await loadList(path.value);
  } catch (err) {
    toast.error(errorMessage(err));
  } finally {
    deleteBusy.value = false;
  }
}

function openShare(entry: Entry): void {
  shareTarget.value = entry;
}

function toggleUploadMenu(): void {
  showUploadMenu.value = !showUploadMenu.value;
  showUserMenu.value = false;
}

function toggleUserMenu(): void {
  showUserMenu.value = !showUserMenu.value;
  showUploadMenu.value = false;
}

function chooseFiles(): void {
  showUploadMenu.value = false;
  fileInput.value?.click();
}

function chooseFolder(): void {
  showUploadMenu.value = false;
  folderInput.value?.click();
}

function onFilesPicked(event: Event): void {
  const input = event.target as HTMLInputElement;
  const files = Array.from(input.files ?? []);
  input.value = "";
  if (!files.length) {
    return;
  }
  if (!list.value?.canUpload || !currentMount.value) {
    toast.error(i18n("home.noUploadPermission"));
    return;
  }
  const count = uploader.addFiles(files, {
    endpoint: tusEndpoint(currentMount.value),
    dir: path.value,
  });
  if (count > 0) {
    toast.success(i18n("home.queuedFiles", { count }));
  }
}

async function onLoginSuccess(info: AuthInfo): Promise<void> {
  auth.value = info;
  showLogin.value = false;
  toast.success(i18n("home.welcomeBack", { name: info.username }));
  await bootstrap();
}

async function logout(): Promise<void> {
  showUserMenu.value = false;
  try {
    auth.value = await api.logout();
    toast.success(i18n("home.loggedOut"));
    await bootstrap();
  } catch (err) {
    toast.error(errorMessage(err));
  }
}

function openLogin(): void {
  showLogin.value = true;
}

function onDocumentClick(): void {
  showUploadMenu.value = false;
  showUserMenu.value = false;
}

onMounted(() => {
  document.addEventListener("click", onDocumentClick);
  void bootstrap();
});

onBeforeUnmount(() => {
  document.removeEventListener("click", onDocumentClick);
  if (refreshTimer) {
    window.clearTimeout(refreshTimer);
  }
  uploader.reset();
  listRequestId += 1;
});
</script>

<template>
  <div class="home-view">
    <header class="app-header">
      <div class="header-inner">
        <button type="button" class="brand" :aria-label="i18n('home.backToRoot')" @click="navigate('')">
          <span class="brand-mark" aria-hidden="true">
            <svg width="28" height="28" viewBox="0 0 28 28" fill="none">
              <rect x="1" y="1" width="26" height="26" rx="7" fill="var(--primary)" />
              <path d="M8 8.5h8.5M8 14h6M8 19.5h9.5" stroke="#fff" stroke-width="2.2" stroke-linecap="round" />
            </svg>
          </span>
          <span class="brand-name">FastHFS</span>
        </button>

        <div v-if="mounts.length" class="mount-area">
          <select
            v-if="mounts.length > 1"
            class="input mount-select"
            :value="currentMount"
            :aria-label="i18n('home.switchMount')"
            @change="onMountChange"
            @click.stop
          >
            <option v-for="item in mounts" :key="item.alias" :value="item.alias">{{ item.name }}</option>
          </select>
          <span v-else class="mount-badge" :title="rootLabel">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <path d="M4 7.5A2.5 2.5 0 0 1 6.5 5h3.8l2 2.4h5.2A2.5 2.5 0 0 1 20 9.9v7.6A2.5 2.5 0 0 1 17.5 20h-11A2.5 2.5 0 0 1 4 17.5z" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" />
            </svg>
            {{ rootLabel }}
          </span>
        </div>

        <label class="search-box" @click.stop>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <circle cx="10.8" cy="10.8" r="6.3" stroke="currentColor" stroke-width="1.7" />
            <path d="M15.5 15.5L20 20" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" />
          </svg>
          <input v-model="search" type="text" :placeholder="i18n('home.searchPlaceholder')" autocomplete="off" spellcheck="false" :aria-label="i18n('home.searchLabel')" />
          <button v-if="search" type="button" class="search-clear" :aria-label="i18n('home.clearSearch')" @click="search = ''">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none">
              <path d="M6 6l12 12M18 6L6 18" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" />
            </svg>
          </button>
        </label>

        <div class="header-actions">
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

          <div v-if="canUpload" class="popover-wrap">
            <button type="button" class="btn btn-primary" :title="i18n('home.upload')" @click.stop="toggleUploadMenu">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path d="M12 16.5V5M8 9l4-4 4 4" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
                <path d="M5 17v1.5A1.5 1.5 0 0 0 6.5 20h11A1.5 1.5 0 0 0 19 18.5V17" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
              </svg>
              <span class="btn-label">{{ i18n("home.upload") }}</span>
            </button>
            <div v-if="showUploadMenu" class="popover" @click.stop>
              <button type="button" class="popover-item" @click="chooseFiles">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <path d="M6.5 3.5h6.8l4.2 4.2v12.3a1.5 1.5 0 0 1-1.5 1.5H6.5A1.5 1.5 0 0 1 5 20V5a1.5 1.5 0 0 1 1.5-1.5z" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" />
                  <path d="M13.2 3.7v4h4" stroke="currentColor" stroke-width="1.5" />
                </svg>
                <span>{{ i18n("home.uploadFile") }}</span>
              </button>
              <button type="button" class="popover-item" @click="chooseFolder">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <path d="M3.5 7.2A2.2 2.2 0 0 1 5.7 5h4l2 2.3h6.6a2.2 2.2 0 0 1 2.2 2.2v7.3a2.2 2.2 0 0 1-2.2 2.2H5.7a2.2 2.2 0 0 1-2.2-2.2z" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" />
                  <path d="M12 17.5v-5M9.6 14.8l2.4-2.4 2.4 2.4" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round" />
                </svg>
                <span>{{ i18n("home.uploadFolder") }}</span>
              </button>
            </div>
          </div>

          <button v-if="canUpload" type="button" class="btn" :title="i18n('home.newFolder')" :aria-label="i18n('home.newFolder')" @click="openMkdir">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <path d="M3.5 7.2A2.2 2.2 0 0 1 5.7 5h4l2 2.3h6.6a2.2 2.2 0 0 1 2.2 2.2v7.3a2.2 2.2 0 0 1-2.2 2.2H5.7a2.2 2.2 0 0 1-2.2-2.2z" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" />
              <path d="M12 11v5M9.5 13.5h5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
            </svg>
            <span class="btn-label mkdir-label">{{ i18n("home.newFolder") }}</span>
          </button>

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

          <div v-if="auth?.authenticated" class="popover-wrap">
            <button type="button" class="user-chip" :title="i18n('home.accountMenu')" @click.stop="toggleUserMenu">
              <span class="avatar">{{ userInitial }}</span>
              <span class="user-name">{{ auth.username }}</span>
              <svg class="chevron" width="13" height="13" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path d="M6 9.5l6 6 6-6" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
              </svg>
            </button>
            <div v-if="showUserMenu" class="popover user-popover" @click.stop>
              <div class="user-info">
                <span class="user-info-name">{{ auth.username }}</span>
                <span class="user-info-mode">{{ permissionBadge }}</span>
              </div>
              <span class="popover-sep" />
              <button type="button" class="popover-item" @click="logout">
                <svg width="15" height="15" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <path d="M14 5.5H6.5A1.5 1.5 0 0 0 5 7v10a1.5 1.5 0 0 0 1.5 1.5H14" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
                  <path d="M12.5 12h7M16.5 8.5l3.5 3.5-3.5 3.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
                </svg>
                <span>{{ i18n("home.logout") }}</span>
              </button>
            </div>
          </div>
          <button v-else-if="auth?.canLogin" type="button" class="btn" @click="openLogin">
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <circle cx="12" cy="8.5" r="3.2" stroke="currentColor" stroke-width="1.5" />
              <path d="M5.5 20a6.5 6.5 0 0 1 13 0" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
            </svg>
            <span class="btn-label">{{ i18n("home.login") }}</span>
          </button>
        </div>

        <input ref="fileInput" class="sr-only" type="file" multiple @change="onFilesPicked" />
        <input ref="folderInput" class="sr-only" type="file" multiple webkitdirectory @change="onFilesPicked" />
      </div>
    </header>

    <div v-if="mounts.length" class="subbar">
      <div class="subbar-inner">
        <BreadcrumbNav :root-label="rootLabel" :path="path" @navigate="navigate" />
        <div class="subbar-right">
          <span class="perm-badge">
            <span class="perm-dot" :class="{ online: auth?.authenticated }" />
            {{ permissionBadge }}
          </span>
        </div>
      </div>
    </div>

    <main class="home-main">
      <div v-if="loadingMounts" class="center-state">
        <span class="spinner spinner-lg" />
        <p class="state-desc">{{ i18n("home.loadingMounts") }}</p>
      </div>

      <div v-else-if="mountError" class="center-card">
        <div class="state-icon" aria-hidden="true">
          <svg width="30" height="30" viewBox="0 0 24 24" fill="none">
            <circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="1.5" />
            <path d="M12 7.5v5.2" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
            <circle cx="12" cy="16.4" r="1.05" fill="currentColor" />
          </svg>
        </div>
        <h2 class="state-title">{{ i18n("home.loadFailed") }}</h2>
        <p class="state-desc">{{ mountError }}</p>
        <button type="button" class="btn btn-primary" @click="bootstrap">{{ i18n("common.retry") }}</button>
      </div>

      <div v-else-if="loginRequired" class="center-card">
        <div class="state-icon" aria-hidden="true">
          <svg width="30" height="30" viewBox="0 0 24 24" fill="none">
            <rect x="4.5" y="10.5" width="15" height="9.5" rx="2.2" stroke="currentColor" stroke-width="1.5" />
            <path d="M8 10.5V8a4 4 0 0 1 8 0v2.5" stroke="currentColor" stroke-width="1.5" />
            <circle cx="12" cy="15.2" r="1.2" fill="currentColor" />
          </svg>
        </div>
        <h2 class="state-title">{{ i18n("home.loginRequiredTitle") }}</h2>
        <p class="state-desc">{{ i18n("home.loginRequiredDesc") }}</p>
        <button type="button" class="btn btn-primary" @click="openLogin">{{ i18n("home.login") }}</button>
      </div>

      <div v-else-if="!mounts.length" class="center-card">
        <div class="state-icon" aria-hidden="true">
          <svg width="30" height="30" viewBox="0 0 24 24" fill="none">
            <path d="M3.5 7.2A2.2 2.2 0 0 1 5.7 5h4l2 2.3h6.6a2.2 2.2 0 0 1 2.2 2.2v7.3a2.2 2.2 0 0 1-2.2 2.2H5.7a2.2 2.2 0 0 1-2.2-2.2z" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" />
            <path d="M9.5 13.5h5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
          </svg>
        </div>
        <h2 class="state-title">{{ i18n("home.noMountsTitle") }}</h2>
        <p class="state-desc">{{ i18n("home.noMountsDesc") }}</p>
        <button v-if="auth?.canLogin" type="button" class="btn" @click="openLogin">{{ i18n("home.loginRetry") }}</button>
      </div>

      <div v-else-if="listError" class="center-card">
        <div class="state-icon" aria-hidden="true">
          <svg width="30" height="30" viewBox="0 0 24 24" fill="none">
            <circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="1.5" />
            <path d="M12 7.5v5.2" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
            <circle cx="12" cy="16.4" r="1.05" fill="currentColor" />
          </svg>
        </div>
        <h2 class="state-title">{{ i18n("home.openFailed") }}</h2>
        <p class="state-desc">{{ listError }}</p>
        <button type="button" class="btn btn-primary" @click="loadList(path)">{{ i18n("common.retry") }}</button>
      </div>

      <template v-else>
        <FileBrowser
          :list="list"
          :entries="entries"
          :view-mode="viewMode"
          :sort-field="sortField"
          :sort-order="sortOrder"
          :loading="loadingList"
          :previewable="Boolean(list?.canDownload)"
          :can-zip="Boolean(list?.canDownload)"
          @open="openEntry"
          @preview="openPreview"
          @download="downloadEntry"
          @share="openShare"
          @rename="openRename"
          @remove="deleteTarget = $event"
          @denied="denied"
          @sort="toggleSort"
          @toggle-order="toggleOrder"
          @zip="zipCurrent"
        >
          <template #empty-action>
            <button v-if="canUpload" type="button" class="btn btn-primary" style="margin-top: 8px" @click="chooseFiles">
              {{ i18n("home.uploadFile") }}
            </button>
          </template>
        </FileBrowser>
      </template>
    </main>

    <UploadPanel
      v-if="uploader.tasks.value.length > 0"
      :tasks="uploader.tasks.value"
      :uploading-count="uploader.uploadingCount.value"
      :finished-count="uploader.finishedCount.value"
      :total-progress="uploader.totalProgress.value"
      @cancel="uploader.cancel"
      @retry="uploader.retry"
      @remove="uploader.remove"
      @clear="uploader.clearFinished"
    />

    <LoginDialog v-if="showLogin" :required="loginRequired" @close="showLogin = false" @success="onLoginSuccess" />

    <InputDialog
      v-if="inputDialog"
      :title="inputDialogTitle"
      :label="inputDialogLabel"
      :initial="inputDialog.entry?.name ?? ''"
      :confirm-text="inputDialogConfirm"
      :select-base-name="inputDialog.mode === 'rename'"
      :placeholder="inputDialog.mode === 'mkdir' ? i18n('home.mkdirPlaceholder') : ''"
      @close="inputDialog = null"
      @confirm="confirmInput"
    />

    <ConfirmDialog
      v-if="deleteTarget"
      :title="i18n('home.deleteTitle')"
      :message="deleteMessage"
      :confirm-text="i18n('common.delete')"
      danger
      :busy="deleteBusy"
      @close="deleteTarget = null"
      @confirm="confirmDelete"
    />

    <ShareCreateDialog
      v-if="shareTarget"
      :mount="currentMount"
      :path="joinRel(path, shareTarget.name)"
      :name="shareTarget.name"
      :is-dir="shareTarget.isDir"
      @close="shareTarget = null"
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
.home-view {
  min-height: 100dvh;
  display: flex;
  flex-direction: column;
}

.app-header {
  position: sticky;
  top: 0;
  z-index: 100;
  border-bottom: 1px solid var(--border);
  background: color-mix(in srgb, var(--bg) 86%, transparent);
  backdrop-filter: blur(14px) saturate(1.15);
}

.header-inner {
  display: flex;
  align-items: center;
  gap: 10px;
  max-width: 1240px;
  margin: 0 auto;
  padding: 10px 22px;
}

.brand {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex: none;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--text);
}

.brand-mark {
  display: inline-flex;
}

.brand-name {
  font-size: 15px;
  font-weight: 650;
  letter-spacing: -0.02em;
}

.mount-area {
  flex: none;
}

.mount-select {
  width: 148px;
  height: 34px;
  font-size: 12.5px;
}

.mount-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  max-width: 180px;
  height: 34px;
  padding: 0 10px;
  border: 1px solid var(--border);
  border-radius: 9px;
  color: var(--text-2);
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
}

.mount-badge svg {
  color: var(--icon-folder);
  flex: none;
}

.search-box {
  display: flex;
  align-items: center;
  gap: 7px;
  flex: 1 1 180px;
  max-width: 380px;
  height: 34px;
  padding: 0 10px;
  border: 1px solid var(--border);
  border-radius: 9px;
  background: var(--bg-elev);
  color: var(--text-3);
  transition: border-color 0.15s ease, box-shadow 0.15s ease;
  cursor: text;
}

.search-box:focus-within {
  border-color: var(--primary);
  box-shadow: 0 0 0 3px var(--focus-ring);
}

.search-box svg {
  flex: none;
}

.search-box input {
  flex: 1;
  min-width: 0;
  height: 100%;
  padding: 0;
  border: 0;
  outline: none;
  background: transparent;
  color: var(--text);
  font-size: 13px;
}

.search-box input::placeholder {
  color: var(--text-3);
}

.search-clear {
  display: inline-flex;
  padding: 3px;
  border: 0;
  border-radius: 5px;
  background: transparent;
  color: var(--text-3);
}

.search-clear:hover {
  background: var(--bg-hover);
  color: var(--text);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 5px;
  margin-left: auto;
  flex: none;
}

.segmented {
  display: inline-flex;
  padding: 2px;
  border: 1px solid var(--border);
  border-radius: 9px;
  background: var(--bg-elev);
}

.segment {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 28px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--text-3);
  transition: background-color 0.12s ease, color 0.12s ease;
}

.segment:hover {
  color: var(--text);
}

.segment.active {
  background: var(--bg-active);
  color: var(--text);
}

.popover-wrap {
  position: relative;
}

.popover {
  position: absolute;
  top: calc(100% + 7px);
  right: 0;
  z-index: 200;
  min-width: 158px;
  padding: 4px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--bg-elev);
  box-shadow: var(--shadow-md);
  animation: popover-in 0.13s ease;
}

@keyframes popover-in {
  from {
    opacity: 0;
    transform: translateY(-3px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.popover-item {
  display: flex;
  align-items: center;
  gap: 9px;
  width: 100%;
  padding: 8px 9px;
  border: 0;
  border-radius: 7px;
  background: transparent;
  color: var(--text);
  font-size: 12.5px;
  text-align: left;
  white-space: nowrap;
}

.popover-item svg {
  flex: none;
  color: var(--text-2);
}

.popover-item:hover {
  background: var(--bg-hover);
}

.popover-sep {
  display: block;
  height: 1px;
  margin: 4px 3px;
  background: var(--border);
}

.user-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 34px;
  padding: 0 7px 0 4px;
  border: 1px solid transparent;
  border-radius: 9px;
  background: transparent;
  color: var(--text);
  font-size: 12.5px;
}

.user-chip:hover {
  background: var(--bg-hover);
}

.avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 25px;
  height: 25px;
  border-radius: 7px;
  background: var(--primary-soft);
  color: var(--primary);
  font-size: 11.5px;
  font-weight: 650;
  flex: none;
}

.user-name {
  max-width: 90px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.chevron {
  color: var(--text-3);
}

.user-popover {
  min-width: 176px;
}

.user-info {
  display: flex;
  flex-direction: column;
  gap: 1px;
  padding: 7px 9px 8px;
}

.user-info-name {
  font-size: 12.5px;
  font-weight: 600;
}

.user-info-mode {
  color: var(--text-3);
  font-size: 11px;
}

.subbar {
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
  min-height: 40px;
}

.subbar-right {
  margin-left: auto;
  flex: none;
}

.perm-badge {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: var(--text-3);
  font-size: 11px;
  white-space: nowrap;
}

.perm-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--text-3);
}

.perm-dot.online {
  background: var(--success);
}

.home-main {
  flex: 1;
  width: 100%;
  max-width: 1240px;
  margin: 0 auto;
  padding: 14px 22px 90px;
  min-height: 320px;
}

.center-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 90px 20px;
  color: var(--text-2);
}

.center-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  max-width: 460px;
  margin: 60px auto;
  padding: 30px 24px;
  border: 1px solid var(--border);
  border-radius: 16px;
  background: var(--bg-elev);
  text-align: center;
  box-shadow: var(--shadow-sm);
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

.center-card .btn,
.center-state .btn {
  margin-top: 8px;
}

@media (max-width: 1100px) {
  .mkdir-label {
    display: none;
  }
}

@media (max-width: 900px) {
  .header-inner {
    flex-wrap: wrap;
    padding: 9px 14px;
    row-gap: 8px;
  }

  .brand {
    order: 1;
  }

  .header-actions {
    order: 2;
    margin-left: auto;
  }

  .search-box {
    order: 3;
    flex: 1 1 150px;
    max-width: none;
  }

  .mount-area {
    order: 4;
  }

  .mount-select {
    width: 126px;
  }

  .btn-label {
    display: none;
  }

  .subbar-inner {
    padding: 5px 12px;
  }

  .home-main {
    padding: 12px 14px 90px;
  }
}

@media (max-width: 520px) {
  .brand-name {
    display: none;
  }

  .perm-badge {
    display: none;
  }

  .search-box {
    order: 5;
    flex-basis: 100%;
  }

  .mount-area {
    order: 4;
    margin-left: auto;
  }

  .mount-select {
    width: 118px;
  }

  .user-name {
    display: none;
  }

  .user-chip {
    padding: 0 4px;
  }

  .center-card {
    margin: 30px auto;
    padding: 24px 18px;
  }
}
</style>
