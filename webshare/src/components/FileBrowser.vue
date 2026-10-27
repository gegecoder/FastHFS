<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from "vue";
import type { Entry, ListResult, SortField, SortOrder, ViewMode } from "../types";
import { formatSize, formatTime } from "../utils";
import { i18n } from "../i18n";
import FileIcon from "./FileIcon.vue";

const props = defineProps<{
  list: ListResult | null;
  entries: Entry[];
  viewMode: ViewMode;
  sortField: SortField;
  sortOrder: SortOrder;
  loading: boolean;
  previewable: boolean;
  canZip: boolean;
}>();

const emit = defineEmits<{
  open: [entry: Entry];
  preview: [entry: Entry];
  download: [entry: Entry];
  share: [entry: Entry];
  rename: [entry: Entry];
  remove: [entry: Entry];
  denied: [entry: Entry];
  sort: [field: SortField];
  toggleOrder: [];
  zip: [];
}>();

const openMenu = ref<string | null>(null);

function toggleMenu(name: string, event: MouseEvent): void {
  event.stopPropagation();
  openMenu.value = openMenu.value === name ? null : name;
}

function closeMenu(): void {
  openMenu.value = null;
}

function selectEntry(entry: Entry): void {
  closeMenu();
  if (entry.isDir) {
    emit("open", entry);
    return;
  }
  if (props.previewable) {
    emit("preview", entry);
  } else if (props.list?.canDownload) {
    emit("download", entry);
  } else {
    emit("denied", entry);
  }
}

function run(action: "preview" | "download" | "share" | "rename" | "remove", entry: Entry, event: MouseEvent): void {
  event.stopPropagation();
  openMenu.value = null;
  switch (action) {
    case "preview":
      emit("preview", entry);
      break;
    case "download":
      emit("download", entry);
      break;
    case "share":
      emit("share", entry);
      break;
    case "rename":
      emit("rename", entry);
      break;
    case "remove":
      emit("remove", entry);
      break;
  }
}

function onSortChange(event: Event): void {
  const target = event.target as HTMLSelectElement;
  emit("sort", target.value as SortField);
}

function headerSort(field: SortField): void {
  emit("sort", field);
}

const canDownload = () => props.list?.canDownload ?? false;
const canDelete = () => props.list?.canDelete ?? false;
const canShare = () => props.list?.canShare ?? false;

function arrow(field: SortField): "asc" | "desc" | null {
  return props.sortField === field ? props.sortOrder : null;
}

onMounted(() => document.addEventListener("click", closeMenu));
onBeforeUnmount(() => document.removeEventListener("click", closeMenu));
</script>

<template>
  <div class="file-browser">
    <div class="browser-toolbar">
      <span class="entry-count">
        {{ i18n("browser.count", { n: entries.length }) }}
        <span v-if="loading" class="loading-label">· {{ i18n("browser.refreshing") }}</span>
      </span>
      <div class="toolbar-right">
        <span class="sort-label">{{ i18n("browser.sort") }}</span>
        <select class="input sort-select" :value="sortField" :aria-label="i18n('browser.sortField')" @change="onSortChange">
          <option value="name">{{ i18n("browser.name") }}</option>
          <option value="size">{{ i18n("browser.size") }}</option>
          <option value="modTime">{{ i18n("browser.modTime") }}</option>
        </select>
        <button
          type="button"
          class="btn btn-ghost icon-btn"
          :title="sortOrder === 'asc' ? i18n('browser.orderAscTip') : i18n('browser.orderDescTip')"
          :aria-label="i18n('browser.toggleOrder')"
          @click="emit('toggleOrder')"
        >
          <svg v-if="sortOrder === 'asc'" width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path d="M12 19V5M6.5 10.5L12 5l5.5 5.5" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" />
          </svg>
          <svg v-else width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path d="M12 5v14M6.5 13.5L12 19l5.5-5.5" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" />
          </svg>
        </button>
        <button v-if="canZip" type="button" class="btn btn-sm zip-button" @click="emit('zip')">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path d="M4.5 6.8A1.8 1.8 0 0 1 6.3 5h4l1.7 2h5.7a1.8 1.8 0 0 1 1.8 1.8v8.4a1.8 1.8 0 0 1-1.8 1.8H6.3a1.8 1.8 0 0 1-1.8-1.8z" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" />
            <path d="M12 10.2v2M10.8 12.2h2.4v1.3h-2.4zM12 13.5v1.9" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" />
          </svg>
          <span class="zip-text">{{ i18n("browser.zip") }}</span>
        </button>
      </div>
    </div>

    <div v-if="loading && !list" class="skeleton-list" aria-hidden="true">
      <div v-for="index in 7" :key="index" class="skeleton-row">
        <span class="skeleton skeleton-icon" />
        <span class="skeleton skeleton-name" />
        <span class="skeleton skeleton-meta" />
      </div>
    </div>

    <div v-else-if="entries.length === 0" class="empty-state">
      <div class="empty-icon" aria-hidden="true">
        <svg width="34" height="34" viewBox="0 0 24 24" fill="none">
          <path
            d="M3.2 7.2A2.2 2.2 0 0 1 5.4 5h4.1l2 2.3h7.1a2.2 2.2 0 0 1 2.2 2.2v8.3a2.2 2.2 0 0 1-2.2 2.2H5.4a2.2 2.2 0 0 1-2.2-2.2z"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linejoin="round"
          />
          <path d="M9 13.5h6" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
        </svg>
      </div>
      <p class="empty-title">{{ entries.length === 0 && list?.entries.length ? i18n("browser.noMatchTitle") : i18n("browser.emptyTitle") }}</p>
      <p class="empty-desc">
        {{ entries.length === 0 && list?.entries.length ? i18n("browser.noMatchDesc") : list?.canUpload ? i18n("browser.emptyUploadDesc") : i18n("browser.emptyDesc") }}
      </p>
      <slot name="empty-action" />
    </div>

    <template v-else>
      <div v-if="viewMode === 'list'" class="list-view" :class="{ refreshing: loading }" role="table">
        <div class="list-head" role="row">
          <button type="button" class="head-cell name-cell" @click="headerSort('name')">
            {{ i18n("browser.name") }}
            <span v-if="arrow('name')" class="sort-arrow" aria-hidden="true">
              <svg v-if="arrow('name') === 'asc'" width="12" height="12" viewBox="0 0 24 24" fill="none">
                <path d="M12 18V6M7 11l5-5 5 5" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
              </svg>
              <svg v-else width="12" height="12" viewBox="0 0 24 24" fill="none">
                <path d="M12 6v12M7 13l5 5 5-5" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
              </svg>
            </span>
          </button>
          <button type="button" class="head-cell size-cell" @click="headerSort('size')">
            {{ i18n("browser.size") }}
            <span v-if="arrow('size')" class="sort-arrow" aria-hidden="true">
              <svg v-if="arrow('size') === 'asc'" width="12" height="12" viewBox="0 0 24 24" fill="none">
                <path d="M12 18V6M7 11l5-5 5 5" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
              </svg>
              <svg v-else width="12" height="12" viewBox="0 0 24 24" fill="none">
                <path d="M12 6v12M7 13l5 5 5-5" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
              </svg>
            </span>
          </button>
          <button type="button" class="head-cell time-cell" @click="headerSort('modTime')">
            {{ i18n("browser.modTime") }}
            <span v-if="arrow('modTime')" class="sort-arrow" aria-hidden="true">
              <svg v-if="arrow('modTime') === 'asc'" width="12" height="12" viewBox="0 0 24 24" fill="none">
                <path d="M12 18V6M7 11l5-5 5 5" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
              </svg>
              <svg v-else width="12" height="12" viewBox="0 0 24 24" fill="none">
                <path d="M12 6v12M7 13l5 5 5-5" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
              </svg>
            </span>
          </button>
          <span class="head-cell actions-cell" aria-hidden="true" />
        </div>

        <div class="list-body">
          <div
            v-for="entry in entries"
            :key="entry.name"
            class="file-row"
            role="row"
            tabindex="0"
            :title="entry.name"
            @click="selectEntry(entry)"
            @keydown.enter="selectEntry(entry)"
          >
            <div class="name-cell">
              <FileIcon :name="entry.name" :is-dir="entry.isDir" :mime="entry.mime" :size="22" />
              <div class="name-text">
                <span class="entry-name" :class="{ 'is-dir': entry.isDir }">{{ entry.name }}</span>
                <span class="mobile-meta">
                  {{ entry.isDir ? i18n("browser.folder") : formatSize(entry.size) }}
                  <template v-if="!entry.isDir"> · {{ formatTime(entry.modTime) }}</template>
                </span>
              </div>
            </div>
            <div class="size-cell">{{ entry.isDir ? "—" : formatSize(entry.size) }}</div>
            <div class="time-cell">{{ formatTime(entry.modTime) }}</div>
            <div class="actions-cell" @click.stop>
              <div class="inline-actions">
                <button
                  v-if="!entry.isDir && previewable"
                  type="button"
                  class="btn btn-ghost icon-btn row-icon"
                  :title="i18n('browser.preview')"
                  :aria-label="i18n('browser.preview')"
                  @click="run('preview', entry, $event)"
                >
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
                    <path d="M2.8 12s3.4-5.6 9.2-5.6S21.2 12 21.2 12s-3.4 5.6-9.2 5.6S2.8 12 2.8 12z" stroke="currentColor" stroke-width="1.6" />
                    <circle cx="12" cy="12" r="2.6" stroke="currentColor" stroke-width="1.6" />
                  </svg>
                </button>
                <button
                  v-if="canDownload()"
                  type="button"
                  class="btn btn-ghost icon-btn row-icon"
                  :title="entry.isDir ? i18n('browser.zip') : i18n('browser.download')"
                  :aria-label="entry.isDir ? i18n('browser.zip') : i18n('browser.download')"
                  @click="run('download', entry, $event)"
                >
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
                    <path d="M12 4.5V15M8 11.5l4 3.8 4-3.8" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" />
                    <path d="M5 18.5h14" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" />
                  </svg>
                </button>
                <button
                  v-if="canShare()"
                  type="button"
                  class="btn btn-ghost icon-btn row-icon"
                  :title="i18n('browser.createShare')"
                  :aria-label="i18n('browser.createShare')"
                  @click="run('share', entry, $event)"
                >
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
                    <circle cx="17.5" cy="5.5" r="2.3" stroke="currentColor" stroke-width="1.6" />
                    <circle cx="6.5" cy="12" r="2.3" stroke="currentColor" stroke-width="1.6" />
                    <circle cx="17.5" cy="18.5" r="2.3" stroke="currentColor" stroke-width="1.6" />
                    <path d="M8.6 10.8l6.8-4M8.6 13.2l6.8 4" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" />
                  </svg>
                </button>
              </div>
              <button
                type="button"
                class="btn btn-ghost icon-btn row-icon"
                :title="i18n('browser.more')"
                :aria-label="i18n('browser.more')"
                @click="toggleMenu(entry.name, $event)"
              >
                <svg width="17" height="17" viewBox="0 0 24 24" fill="none">
                  <circle cx="12" cy="5.5" r="1.5" fill="currentColor" />
                  <circle cx="12" cy="12" r="1.5" fill="currentColor" />
                  <circle cx="12" cy="18.5" r="1.5" fill="currentColor" />
                </svg>
              </button>
              <div v-if="openMenu === entry.name" class="entry-menu" @click.stop>
                <button
                  v-if="!entry.isDir && previewable"
                  type="button"
                  class="menu-item"
                  @click="run('preview', entry, $event)"
                >
                  {{ i18n("browser.preview") }}
                </button>
                <button v-if="canDownload()" type="button" class="menu-item" @click="run('download', entry, $event)">
                  {{ entry.isDir ? i18n("browser.zip") : i18n("browser.download") }}
                </button>
                <button v-if="canShare()" type="button" class="menu-item" @click="run('share', entry, $event)">{{ i18n("browser.createShare") }}</button>
                <template v-if="canDelete()">
                  <span class="menu-sep" />
                  <button type="button" class="menu-item" @click="run('rename', entry, $event)">{{ i18n("browser.rename") }}</button>
                  <button type="button" class="menu-item danger" @click="run('remove', entry, $event)">{{ i18n("common.delete") }}</button>
                </template>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-else class="grid-view" :class="{ refreshing: loading }">
        <div
          v-for="entry in entries"
          :key="entry.name"
          class="file-card"
          tabindex="0"
          :title="entry.name"
          @click="selectEntry(entry)"
          @keydown.enter="selectEntry(entry)"
        >
          <div class="card-top">
            <FileIcon :name="entry.name" :is-dir="entry.isDir" :mime="entry.mime" :size="38" />
            <button
              type="button"
              class="btn btn-ghost icon-btn card-menu-btn"
              :title="i18n('browser.more')"
              :aria-label="i18n('browser.more')"
              @click="toggleMenu(entry.name, $event)"
            >
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
                <circle cx="12" cy="5.5" r="1.4" fill="currentColor" />
                <circle cx="12" cy="12" r="1.4" fill="currentColor" />
                <circle cx="12" cy="18.5" r="1.4" fill="currentColor" />
              </svg>
            </button>
          </div>
          <div class="card-name" :class="{ 'is-dir': entry.isDir }">{{ entry.name }}</div>
          <div class="card-meta">
            {{ entry.isDir ? i18n("browser.folder") : formatSize(entry.size) }} · {{ formatTime(entry.modTime) }}
          </div>
          <div v-if="openMenu === entry.name" class="entry-menu card-menu" @click.stop>
            <button v-if="!entry.isDir && previewable" type="button" class="menu-item" @click="run('preview', entry, $event)">{{ i18n("browser.preview") }}</button>
            <button v-if="canDownload()" type="button" class="menu-item" @click="run('download', entry, $event)">
              {{ entry.isDir ? i18n("browser.zip") : i18n("browser.download") }}
            </button>
            <button v-if="canShare()" type="button" class="menu-item" @click="run('share', entry, $event)">{{ i18n("browser.createShare") }}</button>
            <template v-if="canDelete()">
              <span class="menu-sep" />
              <button type="button" class="menu-item" @click="run('rename', entry, $event)">{{ i18n("browser.rename") }}</button>
              <button type="button" class="menu-item danger" @click="run('remove', entry, $event)">{{ i18n("common.delete") }}</button>
            </template>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.file-browser {
  min-width: 0;
}

.browser-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 0 2px 10px;
  flex-wrap: wrap;
}

.entry-count {
  color: var(--text-2);
  font-size: 12.5px;
  font-variant-numeric: tabular-nums;
}

.loading-label {
  color: var(--text-3);
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: 4px;
}

.sort-label {
  margin-right: 2px;
  color: var(--text-3);
  font-size: 12px;
}

.sort-select {
  height: 30px;
  padding: 0 28px 0 9px;
  font-size: 12.5px;
  border-radius: 7px;
}

.zip-button {
  margin-left: 6px;
}

/* ---------- 列表 ---------- */
.list-view {
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--bg-elev);
  overflow: visible;
}

.list-head {
  border-radius: 12px 12px 0 0;
  overflow: hidden;
}

.list-body {
  border-radius: 0 0 12px 12px;
}

.list-body .file-row:last-child {
  border-radius: 0 0 12px 12px;
}

.list-head,
.file-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 100px 150px 132px;
  align-items: center;
}

.list-head {
  padding: 0 8px 0 14px;
  border-bottom: 1px solid var(--border);
  background: var(--bg-subtle);
}

.head-cell {
  display: flex;
  align-items: center;
  gap: 3px;
  height: 34px;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--text-3);
  font-size: 11.5px;
  font-weight: 500;
  text-align: left;
  cursor: pointer;
}

.head-cell:hover {
  color: var(--text);
}

.size-cell,
.time-cell {
  font-size: 11.5px;
  color: var(--text-3);
}

.file-row {
  padding: 0 8px 0 14px;
  min-height: 46px;
  border-bottom: 1px solid var(--border);
  cursor: pointer;
  transition: background-color 0.12s ease;
}

.file-row:last-child {
  border-bottom: 0;
}

.file-row:hover {
  background: var(--bg-hover);
}

.file-row:focus-visible {
  outline-offset: -2px;
}

.file-row .name-cell {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  padding: 7px 8px 7px 0;
}

.name-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.entry-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
}

.entry-name.is-dir {
  font-weight: 550;
}

.mobile-meta {
  display: none;
  color: var(--text-3);
  font-size: 11px;
}

.file-row .size-cell,
.file-row .time-cell {
  color: var(--text-2);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.actions-cell {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0;
}

.inline-actions {
  display: flex;
  align-items: center;
  opacity: 0;
  transition: opacity 0.12s ease;
}

.file-row:hover .inline-actions,
.file-row:focus-within .inline-actions {
  opacity: 1;
}

.row-icon {
  width: 30px;
  height: 30px;
  color: var(--text-2);
}

.sep {
  width: 1px;
  height: 18px;
  margin: 0 2px;
  background: var(--border);
}

/* ---------- 网格 ---------- */
.grid-view {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(168px, 1fr));
  gap: 10px;
}

.file-card {
  position: relative;
  display: flex;
  flex-direction: column;
  min-height: 132px;
  padding: 14px 13px 12px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--bg-elev);
  cursor: pointer;
  transition: background-color 0.14s ease, border-color 0.14s ease, box-shadow 0.14s ease;
}

.file-card:hover {
  background: var(--bg-hover);
  border-color: var(--border-strong);
  box-shadow: var(--shadow-sm);
}

.file-card:focus-visible {
  outline-offset: 1px;
}

.card-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 10px;
}

.card-menu-btn {
  opacity: 0;
  width: 28px;
  height: 28px;
  margin: -4px -5px 0 0;
  color: var(--text-2);
}

.file-card:hover .card-menu-btn,
.file-card:focus-within .card-menu-btn {
  opacity: 1;
}

.card-name {
  overflow: hidden;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  word-break: break-all;
  font-size: 12.5px;
  line-height: 1.4;
}

.card-name.is-dir {
  font-weight: 550;
}

.card-meta {
  margin-top: auto;
  padding-top: 6px;
  color: var(--text-3);
  font-size: 10.5px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* ---------- 菜单 ---------- */
.entry-menu {
  position: absolute;
  top: calc(100% - 2px);
  right: 0;
  z-index: 30;
  min-width: 132px;
  padding: 4px;
  border: 1px solid var(--border);
  border-radius: 9px;
  background: var(--bg-elev);
  box-shadow: var(--shadow-md);
  animation: menu-in 0.12s ease;
}

.card-menu {
  top: 38px;
  right: 8px;
}

@keyframes menu-in {
  from {
    opacity: 0;
    transform: translateY(-3px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.menu-item {
  display: block;
  width: 100%;
  padding: 6px 9px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--text);
  font-size: 12.5px;
  text-align: left;
  white-space: nowrap;
}

.menu-item:hover {
  background: var(--bg-hover);
}

.menu-item.danger {
  color: var(--danger);
}

.menu-item.danger:hover {
  background: var(--danger-soft);
}

.menu-sep {
  display: block;
  height: 1px;
  margin: 4px 3px;
  background: var(--border);
}

/* ---------- 空态 / 骨架 ---------- */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 56px 20px;
  border: 1px dashed var(--border-strong);
  border-radius: 14px;
  text-align: center;
  background: var(--bg-elev);
}

.empty-icon {
  display: inline-flex;
  margin-bottom: 6px;
  padding: 14px;
  border-radius: 16px;
  background: var(--bg-subtle);
  color: var(--text-3);
}

.empty-title {
  font-size: 14px;
  font-weight: 600;
}

.empty-desc {
  color: var(--text-2);
  font-size: 12.5px;
}

.skeleton-list {
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--bg-elev);
  overflow: hidden;
}

.skeleton-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 13px 16px;
  border-bottom: 1px solid var(--border);
}

.skeleton-row:last-child {
  border-bottom: 0;
}

.skeleton {
  display: block;
  height: 12px;
  border-radius: 5px;
  background: var(--bg-subtle);
  animation: shimmer 1.2s ease-in-out infinite alternate;
}

.skeleton-icon {
  width: 22px;
  height: 22px;
  border-radius: 6px;
}

.skeleton-name {
  width: 34%;
}

.skeleton-meta {
  width: 14%;
  margin-left: auto;
}

@keyframes shimmer {
  from {
    opacity: 0.55;
  }
  to {
    opacity: 1;
  }
}

.refreshing {
  opacity: 0.65;
  pointer-events: none;
  transition: opacity 0.15s ease;
}

@media (max-width: 820px) {
  .list-head,
  .file-row {
    grid-template-columns: minmax(0, 1fr) 116px;
  }

  .list-head .size-cell,
  .list-head .time-cell,
  .file-row .size-cell,
  .file-row .time-cell {
    display: none;
  }

  .mobile-meta {
    display: block;
  }

  .inline-actions {
    opacity: 1;
  }

  .file-row {
    min-height: 54px;
  }
}

@media (max-width: 640px) {
  .browser-toolbar {
    padding-bottom: 8px;
  }

  .grid-view {
    grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
    gap: 8px;
  }

  .file-card {
    min-height: 122px;
    padding: 12px;
  }

  .card-menu-btn {
    opacity: 1;
  }

  .zip-text {
    display: none;
  }

  .sort-label {
    display: none;
  }

  .empty-state {
    padding: 44px 16px;
  }
}
</style>
