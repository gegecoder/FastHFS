<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { Delete, Key, Plus, User } from "@element-plus/icons-vue";
import { ElMessageBox } from "element-plus";
import * as Api from "../../bindings/FastHFS/app";
import { refreshUsers, state } from "../store";
import { fmtTime, guard, toastErr, toastOk } from "../ui";
import { i18n } from "../i18n";
import PageHeader from "../components/PageHeader.vue";

const dialogVisible = ref(false);
const saving = ref(false);
const form = reactive({ username: "", password: "", confirm: "" });

const resetVisible = ref(false);
const resetting = ref(false);
const resetTarget = ref<{ id: string; username: string } | null>(null);
const resetForm = reactive({ password: "", confirm: "" });

onMounted(refreshUsers);

function openAdd() {
  form.username = "";
  form.password = "";
  form.confirm = "";
  dialogVisible.value = true;
}

async function save() {
  if (saving.value) return;
  const username = form.username.trim();
  if (!username) return toastErr(i18n("users.errUsername"));
  if (username.length > 64) return toastErr(i18n("users.errUsernameTooLong"));
  if (form.password.length < 4 || form.password.length > 128) {
    return toastErr(i18n("users.errPassword"));
  }
  if (form.password !== form.confirm) return toastErr(i18n("users.errMismatch"));

  saving.value = true;
  try {
    const result = await Api.AddUser(username, form.password);
    if (result?.success) {
      dialogVisible.value = false;
      await refreshUsers();
      toastOk(result.message || i18n("users.created"));
    } else {
      toastErr(result?.message || i18n("users.createFailed"));
    }
  } catch (error) {
    toastErr(error);
  } finally {
    saving.value = false;
  }
}

function openReset(row: { id: string; username: string }) {
  resetTarget.value = { id: row.id, username: row.username };
  resetForm.password = "";
  resetForm.confirm = "";
  resetVisible.value = true;
}

async function saveReset() {
  if (resetting.value || !resetTarget.value) return;
  if (resetForm.password.length < 4 || resetForm.password.length > 128) {
    return toastErr(i18n("users.errPassword"));
  }
  if (resetForm.password !== resetForm.confirm) return toastErr(i18n("users.errMismatch"));

  resetting.value = true;
  try {
    const result = await Api.ResetUserPassword(resetTarget.value.id, resetForm.password);
    if (result?.success) {
      resetVisible.value = false;
      toastOk(result.message || i18n("users.resetOk"));
    } else {
      toastErr(result?.message || i18n("users.resetFailed"));
    }
  } catch (error) {
    toastErr(error);
  } finally {
    resetting.value = false;
  }
}

async function remove(id: string, username: string) {
  try {
    await ElMessageBox.confirm(i18n("users.deleteConfirm", { name: username }), i18n("users.deleteTitle"), {
      type: "warning",
      confirmButtonText: i18n("common.delete"),
      cancelButtonText: i18n("common.cancel"),
    });
  } catch {
    return;
  }
  const result = await guard(() => Api.DeleteUser(id));
  if (result?.success) await refreshUsers();
}

function initial(name: string) {
  return (name || "?").slice(0, 1).toUpperCase();
}
</script>

<template>
  <div class="page">
    <PageHeader :title="i18n('users.title')" :desc="i18n('users.desc')" />
    <main class="content">
      <div class="page-stack">
        <section class="panel">
          <div class="panel-head">
            <div>
              <h3>{{ i18n("users.panelTitle") }}</h3>
              <p class="sub">{{ i18n("users.panelSub") }}</p>
            </div>
            <el-button type="primary" :icon="Plus" @click="openAdd">{{ i18n("users.add") }}</el-button>
          </div>

          <el-table v-if="state.users.length" :data="state.users" row-key="id" style="width: 100%">
            <el-table-column :label="i18n('users.colAccount')" min-width="220">
              <template #default="{ row }">
                <div class="user-cell">
                  <span class="avatar">{{ initial(row.username) }}</span>
                  <div>
                    <strong>{{ row.username }}</strong>
                    <span class="muted">{{ i18n("users.createdAt", { time: fmtTime(row.createdAt) }) }}</span>
                  </div>
                </div>
              </template>
            </el-table-column>
            <el-table-column :label="i18n('users.colLastLogin')" width="170">
              <template #default="{ row }">
                <span v-if="row.lastLoginAt">{{ fmtTime(row.lastLoginAt) }}</span>
                <span v-else class="muted">{{ i18n("users.neverLoggedIn") }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="i18n('users.colActions')" width="200" align="right">
              <template #default="{ row }">
                <el-button text size="small" :icon="Key" @click="openReset(row)">
                  {{ i18n("users.reset") }}
                </el-button>
                <el-button text size="small" type="danger" :icon="Delete" @click="remove(row.id, row.username)">
                  {{ i18n("common.delete") }}
                </el-button>
              </template>
            </el-table-column>
          </el-table>
          <div v-else class="empty">
            <el-icon class="empty-icon"><User /></el-icon>
            <p>{{ i18n("users.empty") }}</p>
            <el-button type="primary" plain :icon="Plus" @click="openAdd">{{ i18n("users.emptyAction") }}</el-button>
          </div>
        </section>

        <el-dialog v-model="dialogVisible" :title="i18n('users.dialogTitle')" width="440px" :close-on-click-modal="false">
          <el-form label-position="top">
            <el-form-item :label="i18n('users.username')">
              <el-input
                v-model="form.username"
                :placeholder="i18n('users.usernamePlaceholder')"
                maxlength="64"
                @keyup.enter="save"
              />
            </el-form-item>
            <el-form-item :label="i18n('users.password')">
              <el-input
                v-model="form.password"
                type="password"
                show-password
                :placeholder="i18n('users.passwordPlaceholder')"
                maxlength="128"
              />
            </el-form-item>
            <el-form-item :label="i18n('users.confirmPassword')">
              <el-input
                v-model="form.confirm"
                type="password"
                show-password
                :placeholder="i18n('users.confirmPlaceholder')"
                maxlength="128"
                @keyup.enter="save"
              />
            </el-form-item>
          </el-form>
          <template #footer>
            <el-button @click="dialogVisible = false">{{ i18n("common.cancel") }}</el-button>
            <el-button type="primary" :loading="saving" @click="save">{{ i18n("common.create") }}</el-button>
          </template>
        </el-dialog>

        <el-dialog v-model="resetVisible" :title="i18n('users.resetTitle')" width="440px" :close-on-click-modal="false">
          <p class="reset-hint">{{ i18n("users.resetFor", { name: resetTarget?.username || "" }) }}</p>
          <el-form label-position="top">
            <el-form-item :label="i18n('users.password')">
              <el-input
                v-model="resetForm.password"
                type="password"
                show-password
                :placeholder="i18n('users.passwordPlaceholder')"
                maxlength="128"
              />
            </el-form-item>
            <el-form-item :label="i18n('users.confirmPassword')">
              <el-input
                v-model="resetForm.confirm"
                type="password"
                show-password
                :placeholder="i18n('users.confirmPlaceholder')"
                maxlength="128"
                @keyup.enter="saveReset"
              />
            </el-form-item>
          </el-form>
          <template #footer>
            <el-button @click="resetVisible = false">{{ i18n("common.cancel") }}</el-button>
            <el-button type="primary" :loading="resetting" @click="saveReset">{{ i18n("common.save") }}</el-button>
          </template>
        </el-dialog>
      </div>
    </main>
  </div>
</template>

<style scoped>
.user-cell {
  display: flex;
  align-items: center;
  gap: 11px;
}

.avatar {
  width: 34px;
  height: 34px;
  flex: 0 0 34px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  font-size: 13px;
  font-weight: 600;
  color: var(--primary);
  background: var(--primary-soft);
  border: 1px solid color-mix(in srgb, var(--primary) 22%, var(--border));
}

.user-cell strong {
  display: block;
  font-size: 13px;
}

.user-cell span.muted {
  display: block;
  font-size: 11.5px;
  margin-top: 1px;
}

.empty-icon {
  font-size: 30px;
  color: var(--muted);
  margin-bottom: 6px;
}

.reset-hint {
  margin: 0 0 12px;
  font-size: 12.5px;
  line-height: 1.6;
  color: var(--muted);
}
</style>
