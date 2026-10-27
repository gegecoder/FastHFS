<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { Delete, Edit, FolderOpened, Plus } from "@element-plus/icons-vue";
import { ElMessageBox } from "element-plus";
import * as Api from "../../bindings/FastHFS/app";
import type { Mount } from "../../bindings/FastHFS/backend/models/models";
import { refreshMounts, state } from "../store";
import { guard, toastErr, toastOk } from "../ui";
import { i18n } from "../i18n";
import PageHeader from "../components/PageHeader.vue";

const dialogVisible = ref(false);
const saving = ref(false);
const editingId = ref("");

const form = reactive({
  name: "",
  alias: "",
  root: "",
  allowBrowse: true,
  allowDownload: true,
  allowUpload: false,
  allowDelete: false,
  ignoreRules: "",
});

const title = computed(() => (editingId.value ? i18n("mounts.editTitle") : i18n("mounts.addTitle")));

onMounted(refreshMounts);

function resetForm() {
  editingId.value = "";
  form.name = "";
  form.alias = "";
  form.root = "";
  form.allowBrowse = true;
  form.allowDownload = true;
  form.allowUpload = false;
  form.allowDelete = false;
  form.ignoreRules = "";
}

function openAdd() {
  resetForm();
  dialogVisible.value = true;
}

function openEdit(row: Mount) {
  editingId.value = row.id;
  form.name = row.name;
  form.alias = row.alias;
  form.root = row.root;
  form.allowBrowse = row.allowBrowse;
  form.allowDownload = row.allowDownload;
  form.allowUpload = row.allowUpload;
  form.allowDelete = row.allowDelete;
  form.ignoreRules = (row.ignoreRules || []).join("\n");
  dialogVisible.value = true;
}

async function pickDir() {
  const picked = await guard(() => Api.PickDirectory(i18n("mounts.pickDirTitle")));
  if (picked) {
    form.root = picked;
    const parts = picked.replace(/[\\/]+$/, "").split(/[\\/]/);
    if (!form.name.trim()) {
      form.name = parts[parts.length - 1] || i18n("mounts.defaultName");
    }
    if (!form.alias.trim()) {
      const base = (parts[parts.length - 1] || "share")
        .toLowerCase()
        .replace(/[^a-z0-9._-]+/g, "-")
        .replace(/^-+|-+$/g, "");
      form.alias = base || `share${state.mounts.length + 1}`;
    }
  }
}

function parseRules(): string[] {
  return form.ignoreRules
    .split(/[\n,]+/)
    .map((s) => s.trim())
    .filter(Boolean);
}

async function save() {
  if (saving.value) return;
  if (!form.name.trim()) return toastErr(i18n("mounts.errName"));
  if (!/^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/.test(form.alias.trim())) {
    return toastErr(i18n("mounts.errAlias"));
  }
  if (!form.root.trim()) return toastErr(i18n("mounts.errRoot"));
  if (!form.allowBrowse && !form.allowDownload && !form.allowUpload && !form.allowDelete) {
    return toastErr(i18n("mounts.errPerms"));
  }

  const payload: Mount = {
    id: editingId.value,
    name: form.name.trim(),
    alias: form.alias.trim(),
    root: form.root.trim(),
    allowBrowse: form.allowBrowse,
    allowDownload: form.allowDownload,
    allowUpload: form.allowUpload,
    allowDelete: form.allowDelete,
    ignoreRules: parseRules(),
    createdAt: 0,
  };

  saving.value = true;
  try {
    if (editingId.value) {
      const updated = await Api.UpdateMount(payload);
      if (updated) {
        await refreshMounts();
        dialogVisible.value = false;
        toastOk(i18n("mounts.updated"));
      }
    } else {
      const created = await Api.AddMount(payload);
      if (created) {
        await refreshMounts();
        dialogVisible.value = false;
        toastOk(i18n("mounts.added"));
      }
    }
  } catch (error) {
    toastErr(error);
  } finally {
    saving.value = false;
  }
}

async function remove(row: Mount) {
  try {
    await ElMessageBox.confirm(
      i18n("mounts.deleteConfirm", { name: row.name, alias: row.alias }),
      i18n("mounts.deleteTitle"),
      {
        type: "warning",
        confirmButtonText: i18n("common.delete"),
        cancelButtonText: i18n("common.cancel"),
      },
    );
  } catch {
    return;
  }
  const result = await guard(() => Api.DeleteMount(row.id));
  if (result?.success) await refreshMounts();
}

const permList = (row: Mount) => {
  const out: string[] = [];
  if (row.allowBrowse) out.push(i18n("mounts.permBrowse"));
  if (row.allowDownload) out.push(i18n("mounts.permDownload"));
  if (row.allowUpload) out.push(i18n("mounts.permUpload"));
  if (row.allowDelete) out.push(i18n("mounts.permDelete"));
  return out;
};
</script>

<template>
  <div class="page">
    <PageHeader :title="i18n('mounts.title')" :desc="i18n('mounts.desc')" />
    <main class="content">
      <div class="page-stack">
        <section class="panel">
          <div class="panel-head">
            <div>
              <h3>{{ i18n("mounts.panelTitle") }}</h3>
              <p class="sub">{{ i18n("mounts.panelSub") }}</p>
            </div>
            <el-button type="primary" :icon="Plus" @click="openAdd">{{ i18n("mounts.add") }}</el-button>
          </div>

          <el-table v-if="state.mounts.length" :data="state.mounts" row-key="id" style="width: 100%">
            <el-table-column :label="i18n('mounts.colName')" min-width="180">
              <template #default="{ row }">
                <div class="mount-name">
                  <el-icon class="folder-icon"><FolderOpened /></el-icon>
                  <div class="mount-name-text">
                    <strong>{{ row.name }}</strong>
                    <span class="mono muted">{{ row.root }}</span>
                  </div>
                </div>
              </template>
            </el-table-column>
            <el-table-column :label="i18n('mounts.colPath')" width="150">
              <template #default="{ row }">
                <span class="tag-soft blue mono">/{{ row.alias }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="i18n('mounts.colPerms')" min-width="190">
              <template #default="{ row }">
                <div class="tag-list">
                  <span v-for="p in permList(row)" :key="p" class="tag-soft">{{ p }}</span>
                  <span v-if="!permList(row).length" class="tag-soft red">{{ i18n("mounts.noPerms") }}</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column :label="i18n('mounts.colRules')" width="110">
              <template #default="{ row }">
                <span class="muted">{{ i18n("mounts.ruleCount", { n: (row.ignoreRules || []).length }) }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="i18n('mounts.colActions')" width="150" align="right">
              <template #default="{ row }">
                <el-button text size="small" :icon="Edit" @click="openEdit(row)">
                  {{ i18n("common.edit") }}
                </el-button>
                <el-button text size="small" type="danger" :icon="Delete" @click="remove(row)">
                  {{ i18n("common.delete") }}
                </el-button>
              </template>
            </el-table-column>
          </el-table>
          <div v-else class="empty">
            <p>{{ i18n("mounts.empty") }}</p>
            <el-button type="primary" plain :icon="Plus" @click="openAdd">{{ i18n("mounts.emptyAction") }}</el-button>
          </div>
        </section>

        <section class="panel tips-panel">
          <div class="panel-head">
            <h3>{{ i18n("mounts.tipsTitle") }}</h3>
          </div>
          <div class="tips-grid">
            <div>
              <strong>{{ i18n("mounts.permBrowse") }}</strong>
              <p>{{ i18n("mounts.tipBrowse") }}</p>
            </div>
            <div>
              <strong>{{ i18n("mounts.permDownload") }}</strong>
              <p>{{ i18n("mounts.tipDownload") }}</p>
            </div>
            <div>
              <strong>{{ i18n("mounts.permUpload") }}</strong>
              <p>{{ i18n("mounts.tipUpload") }}</p>
            </div>
            <div>
              <strong>{{ i18n("mounts.permDelete") }}</strong>
              <p>{{ i18n("mounts.tipDelete") }}</p>
            </div>
          </div>
          <p class="hint">{{ i18n("mounts.tipsNote") }}</p>
        </section>

        <el-dialog v-model="dialogVisible" :title="title" width="560px" :close-on-click-modal="false">
          <el-form label-position="top">
            <el-form-item :label="i18n('mounts.root')">
              <div class="dir-row">
                <el-input v-model="form.root" :placeholder="i18n('mounts.rootPlaceholder')" readonly />
                <el-button :icon="FolderOpened" @click="pickDir">{{ i18n("mounts.pickDir") }}</el-button>
              </div>
            </el-form-item>
            <el-form-item :label="i18n('mounts.name')">
              <el-input v-model="form.name" :placeholder="i18n('mounts.namePlaceholder')" maxlength="64" />
            </el-form-item>
            <el-form-item :label="i18n('mounts.alias')">
              <el-input v-model="form.alias" :placeholder="i18n('mounts.aliasPlaceholder')">
                <template #prepend>/</template>
              </el-input>
              <div class="hint">
                {{ i18n("mounts.aliasHint", { alias: form.alias || "alias" }) }}
              </div>
            </el-form-item>
            <el-form-item :label="i18n('mounts.permLabel')">
              <div class="switch-grid">
                <label><el-switch v-model="form.allowBrowse" /> {{ i18n("mounts.allowBrowse") }}</label>
                <label><el-switch v-model="form.allowDownload" /> {{ i18n("mounts.allowDownload") }}</label>
                <label><el-switch v-model="form.allowUpload" /> {{ i18n("mounts.allowUpload") }}</label>
                <label><el-switch v-model="form.allowDelete" /> {{ i18n("mounts.allowDelete") }}</label>
              </div>
            </el-form-item>
            <el-form-item :label="i18n('mounts.ignoreRules')">
              <el-input
                v-model="form.ignoreRules"
                type="textarea"
                :rows="4"
                :placeholder="i18n('mounts.ignorePlaceholder')"
              />
            </el-form-item>
          </el-form>
          <template #footer>
            <el-button @click="dialogVisible = false">{{ i18n("common.cancel") }}</el-button>
            <el-button type="primary" :loading="saving" @click="save">{{ i18n("common.save") }}</el-button>
          </template>
        </el-dialog>
      </div>
    </main>
  </div>
</template>

<style scoped>
.mount-name {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.folder-icon {
  color: var(--primary);
  font-size: 18px;
  flex: 0 0 auto;
}

.mount-name-text {
  min-width: 0;
}

.mount-name-text strong {
  display: block;
  font-size: 13px;
}

.mount-name-text span {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 340px;
  font-size: 11.5px;
}

.dir-row {
  display: flex;
  gap: 8px;
  width: 100%;
}

.dir-row .el-input {
  flex: 1;
}

.switch-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  width: 100%;
}

.switch-grid label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--text);
  padding: 8px 10px;
  border: 1px solid var(--border);
  border-radius: 9px;
  background: var(--panel-soft);
}

.tips-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
  margin-bottom: 14px;
}

.tips-grid strong {
  font-size: 13px;
}

.tips-grid p {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--muted);
  line-height: 1.65;
}

.tips-panel {
  box-shadow: none;
}

@media (max-width: 900px) {
  .tips-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
