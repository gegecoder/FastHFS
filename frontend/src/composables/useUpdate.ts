// 版本更新检测：关于页手动检测与启动时静默检测共用同一份模块级状态
import { computed, ref } from "vue";
import * as Api from "../../bindings/FastHFS/app";
import type { Info as UpdateInfo } from "../../bindings/FastHFS/backend/update/models";
import { i18n } from "../i18n";

type CheckStatus = "idle" | "checking" | "latest" | "available" | "none" | "error";

const updateInfo = ref<UpdateInfo | null>(null);
const checking = ref(false);
const showUpdateModal = ref(false);
const checkStatus = ref<CheckStatus>("idle");
const checkError = ref("");

const checkMessage = computed(() => {
  switch (checkStatus.value) {
    case "checking":
      return i18n("update.statusChecking");
    case "latest":
      return i18n("update.statusLatest");
    case "available":
      return i18n("update.statusAvailable", { version: updateInfo.value?.latestVersion ?? "" });
    case "none":
      return i18n("update.statusNone");
    case "error":
      return checkError.value || i18n("update.statusFailed");
    default:
      return "";
  }
});

// 检测更新：silent=true 用于启动时静默检测，失败或已是最新时不保留状态文案；
// 发现新版本时自动弹出更新窗口
export async function checkForUpdate(silent = false): Promise<UpdateInfo | null> {
  if (checking.value) return updateInfo.value;
  checking.value = true;
  checkStatus.value = "checking";
  checkError.value = "";
  try {
    const res = await Api.CheckForUpdate();
    updateInfo.value = res ?? null;
    if (res && res.msg === "ok") {
      if (res.hasUpdate) {
        checkStatus.value = "available";
        showUpdateModal.value = true;
      } else if (res.noRelease) {
        checkStatus.value = "none";
      } else {
        checkStatus.value = "latest";
      }
    } else {
      checkStatus.value = "error";
      checkError.value = res?.msg || "";
    }
    return updateInfo.value;
  } catch (error) {
    checkStatus.value = "error";
    checkError.value = error instanceof Error ? error.message : String(error);
    return null;
  } finally {
    checking.value = false;
    if (silent && checkStatus.value !== "available") {
      checkStatus.value = "idle";
      checkError.value = "";
    }
  }
}

export function useUpdate() {
  return {
    updateInfo,
    checking,
    showUpdateModal,
    checkStatus,
    checkMessage,
    checkForUpdate,
  };
}
