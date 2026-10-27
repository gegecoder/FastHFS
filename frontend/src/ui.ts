import { ElMessage } from "element-plus";
import { i18n } from "./i18n";

/** 统一成功提示 */
export function toastOk(message: string) {
  ElMessage({ type: "success", message, grouping: true });
}

/** 统一错误提示：兼容 Wails 绑定抛出的字符串 / Error */
export function toastErr(error: unknown, fallback = i18n("common.actionFailed")) {
  let message = fallback;
  if (typeof error === "string") {
    message = error;
  } else if (error instanceof Error) {
    message = error.message || fallback;
  } else if (error && typeof error === "object" && "message" in error) {
    message = String((error as { message: unknown }).message || fallback);
  }
  ElMessage({ type: "error", message, grouping: true });
}

/** 执行绑定调用并统一处理异常；传入 successMsg 时成功自动提示 */
export async function guard<T>(task: () => Promise<T>, successMsg?: string): Promise<T | undefined> {
  try {
    const result = await task();
    if (successMsg) toastOk(successMsg);
    return result;
  } catch (error) {
    toastErr(error);
    return undefined;
  }
}

/** Unix 秒 → 本地时间 */
export function fmtTime(seconds?: number | null): string {
  if (!seconds || seconds <= 0) return i18n("common.permanent");
  const d = new Date(seconds * 1000);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

/** 文件大小格式化 */
export function fmtSize(size?: number | null): string {
  const n = Number(size || 0);
  if (n <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = n;
  let i = 0;
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024;
    i++;
  }
  return `${value >= 100 || i === 0 ? value.toFixed(0) : value.toFixed(1)} ${units[i]}`;
}

/** 运行时长 */
export function fmtDuration(startedAt?: number | null): string {
  if (!startedAt) return "—";
  const seconds = Math.max(0, Math.floor(Date.now() / 1000 - startedAt));
  if (seconds < 60) return i18n("common.durationSeconds", { n: seconds });
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return i18n("common.durationMinutes", { n: minutes });
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return i18n("common.durationHours", { h: hours, m: minutes % 60 });
  return i18n("common.durationDays", { d: Math.floor(hours / 24), h: hours % 24 });
}
