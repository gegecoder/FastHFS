import type { Entry, PreviewKind } from "./types";
import { i18n } from "./i18n";

const SIZE_UNITS = ["B", "KB", "MB", "GB", "TB", "PB"];

export function formatSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return bytes === 0 ? "0 B" : "—";
  }
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < SIZE_UNITS.length - 1) {
    value /= 1024;
    unit += 1;
  }
  const digits = value >= 100 || unit === 0 ? 0 : value >= 10 ? 1 : 2;
  return `${value.toFixed(digits)} ${SIZE_UNITS[unit]}`;
}

export function formatTime(unixSeconds: number): string {
  if (!unixSeconds) {
    return "—";
  }
  const date = new Date(unixSeconds * 1000);
  if (Number.isNaN(date.getTime())) {
    return "—";
  }
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(
    date.getMinutes(),
  )}`;
}

export function formatSpeed(bytesPerSecond: number): string {
  if (!Number.isFinite(bytesPerSecond) || bytesPerSecond <= 0) {
    return "0 B/s";
  }
  return `${formatSize(bytesPerSecond)}/s`;
}

export function formatDuration(hours: number): string {
  if (hours <= 0) {
    return i18n("utils.forever");
  }
  if (hours < 24) {
    return i18n("utils.hours", { n: hours });
  }
  const days = hours / 24;
  return Number.isInteger(days) ? i18n("utils.days", { n: days }) : i18n("utils.hours", { n: hours });
}

export function joinRel(base: string, name: string): string {
  const cleanBase = base.replace(/\\/g, "/").replace(/^\/+|\/+$/g, "");
  const cleanName = name.replace(/\\/g, "/").replace(/^\/+|\/+$/g, "");
  if (!cleanBase) {
    return cleanName;
  }
  if (!cleanName) {
    return cleanBase;
  }
  return `${cleanBase}/${cleanName}`;
}

export function parentPath(rel: string): string {
  const clean = rel.replace(/^\/+|\/+$/g, "");
  if (!clean) {
    return "";
  }
  const index = clean.lastIndexOf("/");
  return index < 0 ? "" : clean.slice(0, index);
}

export function splitPath(rel: string): string[] {
  return rel
    .split("/")
    .map((item) => item.trim())
    .filter((item) => item.length > 0);
}

export function baseName(rel: string): string {
  const clean = rel.replace(/\\/g, "/").replace(/\/+$/g, "");
  const index = clean.lastIndexOf("/");
  return index < 0 ? clean : clean.slice(index + 1);
}

export function fileExtension(name: string): string {
  const index = name.lastIndexOf(".");
  if (index <= 0 || index === name.length - 1) {
    return "";
  }
  return name.slice(index + 1).toLowerCase();
}

const TEXT_EXTENSIONS = new Set([
  "txt", "md", "markdown", "log", "csv", "tsv", "json", "jsonc", "xml", "yaml", "yml", "toml", "ini", "conf",
  "cfg", "properties", "env", "gitignore", "dockerignore", "editorconfig",
  "js", "mjs", "cjs", "jsx", "ts", "mts", "cts", "tsx", "vue", "svelte", "css", "scss", "sass", "less", "html",
  "htm", "xhtml", "svg", "py", "pyw", "go", "rs", "java", "kt", "kts", "scala", "c", "h", "cc", "cpp", "cxx",
  "hpp", "hh", "cs", "m", "mm", "swift", "dart", "php", "rb", "pl", "lua", "r", "sh", "bash", "zsh", "fish",
  "ps1", "bat", "cmd", "sql", "graphql", "gql", "proto", "tex", "makefile", "cmake", "gradle", "srt", "vtt",
  "diff", "patch", "nginx", "htaccess",
]);

const AUDIO_EXTENSIONS = new Set(["mp3", "wav", "ogg", "oga", "m4a", "aac", "flac", "opus", "weba"]);

export function previewKind(entry: Pick<Entry, "name" | "mime">): PreviewKind {
  const mime = (entry.mime || "").toLowerCase();
  const ext = fileExtension(entry.name);
  if (mime.startsWith("image/")) {
    return "image";
  }
  if (mime === "application/pdf" || ext === "pdf") {
    return "pdf";
  }
  if (mime.startsWith("audio/") || AUDIO_EXTENSIONS.has(ext)) {
    return "audio";
  }
  if (
    mime.startsWith("text/") ||
    mime.includes("json") ||
    mime.includes("xml") ||
    mime.includes("javascript") ||
    mime.includes("yaml") ||
    mime.includes("x-sh") ||
    TEXT_EXTENSIONS.has(ext)
  ) {
    return "text";
  }
  return "unsupported";
}

export function canPreview(entry: Pick<Entry, "name" | "mime">): boolean {
  return previewKind(entry) !== "unsupported";
}

export function sortEntries(entries: Entry[], field: "name" | "size" | "modTime", order: "asc" | "desc"): Entry[] {
  const direction = order === "asc" ? 1 : -1;
  return [...entries].sort((a, b) => {
    if (a.isDir !== b.isDir) {
      return a.isDir ? -1 : 1;
    }
    if (field === "name") {
      return a.name.localeCompare(b.name, "zh-CN", { numeric: true, sensitivity: "base" }) * direction;
    }
    if (field === "size") {
      if (a.size === b.size) {
        return a.name.localeCompare(b.name, "zh-CN", { numeric: true }) * direction;
      }
      return (a.size - b.size) * direction;
    }
    if (a.modTime === b.modTime) {
      return a.name.localeCompare(b.name, "zh-CN", { numeric: true }) * direction;
    }
    return (a.modTime - b.modTime) * direction;
  });
}

export function filterEntries(entries: Entry[], search: string): Entry[] {
  const keyword = search.trim().toLowerCase();
  if (!keyword) {
    return entries;
  }
  return entries.filter((entry) => entry.name.toLowerCase().includes(keyword));
}

/** 触发浏览器下载（同源，由服务端 Content-Disposition 决定文件名） */
export function triggerDownload(url: string): void {
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.rel = "noopener";
  anchor.style.display = "none";
  document.body.appendChild(anchor);
  anchor.click();
  window.setTimeout(() => anchor.remove(), 1000);
}

export async function copyText(text: string): Promise<boolean> {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text);
      return true;
    }
  } catch {
    // 继续使用回退方案
  }
  try {
    const area = document.createElement("textarea");
    area.value = text;
    area.setAttribute("readonly", "");
    area.style.position = "fixed";
    area.style.top = "-1000px";
    area.style.opacity = "0";
    document.body.appendChild(area);
    area.select();
    const ok = document.execCommand("copy");
    area.remove();
    return ok;
  } catch {
    return false;
  }
}

/** 从 File 读取相对目录（文件夹上传时使用） */
export function relativeParts(file: File): { dir: string; name: string } {
  const rawPath = (file as File & { webkitRelativePath?: string }).webkitRelativePath || "";
  const normalized = rawPath.replace(/\\/g, "/");
  const parts = splitPath(normalized);
  if (parts.length === 0) {
    return { dir: "", name: file.name };
  }
  const name = parts[parts.length - 1];
  return { dir: parts.slice(0, -1).join("/"), name };
}

export function isAbortError(error: unknown): boolean {
  if (error instanceof DOMException && error.name === "AbortError") {
    return true;
  }
  return error instanceof Error && /abort/i.test(error.name);
}
