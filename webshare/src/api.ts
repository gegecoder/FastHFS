import type {
  APIResult,
  AuthInfo,
  ListResult,
  LoginRequest,
  MountsResult,
  ShareCreated,
  ShareCreateRequest,
  ShareInfo,
} from "./types";
import { i18n } from "./i18n";

export class ApiError extends Error {
  status: number;
  code?: string;

  constructor(message: string, status = 0, code?: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

declare global {
  interface Window {
    /** 服务端注入的部署子路径（反向代理场景），空字符串表示根路径 */
    __FASTHFS_BASE__?: string;
  }
}

const RAW_BASE = typeof window === "undefined" ? "" : (window.__FASTHFS_BASE__ ?? "");
/** 规范化后的部署子路径，例如 /files；根路径时为 "" */
export const BASE_PATH = RAW_BASE === "/" ? "" : RAW_BASE.replace(/\/+$/, "");

/** 给根绝对路径加上部署子路径前缀 */
export function withBase(pathname: string): string {
  return `${BASE_PATH}${pathname}`;
}

/** HTTP 状态码 → 本地化兜底文案 */
const STATUS_KEYS: Record<number, string> = {
  400: "api.badRequest",
  401: "api.unauthorized",
  403: "api.forbidden",
  404: "api.notFound",
  409: "api.conflict",
  410: "api.gone",
  413: "api.tooLarge",
  429: "api.tooMany",
  500: "api.serverError",
  502: "api.unavailable",
  503: "api.unavailable",
};

/** 服务端错误码 → 本地化文案（优先于服务端 message，避免英文界面出现中文） */
const CODE_KEYS: Record<string, string> = {
  bad_request: "api.codes.bad_request",
  login_required: "api.codes.login_required",
  invalid_credentials: "api.codes.invalid_credentials",
  rate_limited: "api.codes.rate_limited",
  mount_not_found: "api.codes.mount_not_found",
  internal_error: "api.codes.internal_error",
  forbidden: "api.codes.forbidden",
  invalid_path: "api.codes.invalid_path",
  not_found: "api.codes.not_found",
  not_a_dir: "api.codes.not_a_dir",
  read_failed: "api.codes.read_failed",
  is_dir: "api.codes.is_dir",
  invalid_name: "api.codes.invalid_name",
  exists: "api.codes.exists",
  mkdir_failed: "api.codes.mkdir_failed",
  rename_failed: "api.codes.rename_failed",
  delete_failed: "api.codes.delete_failed",
  invalid_mode: "api.codes.invalid_mode",
  invalid_password: "api.codes.invalid_password",
  share_not_found: "api.codes.share_not_found",
  share_expired: "api.codes.share_expired",
  share_locked: "api.codes.share_locked",
  mount_missing: "api.codes.mount_missing",
  share_invalid: "api.codes.share_invalid",
  share_required: "api.codes.share_required",
  collect_mode: "api.codes.collect_mode",
  preview_only: "api.codes.preview_only",
  forbidden_type: "api.codes.forbidden_type",
  invalid_dir: "api.codes.invalid_dir",
  network_error: "api.codes.network_error",
};

async function parseErrorResponse(res: Response): Promise<ApiError> {
  let message = "";
  let code = "";
  try {
    const data = (await res.json()) as { error?: string; code?: string };
    message = typeof data.error === "string" ? data.error : "";
    code = typeof data.code === "string" ? data.code : "";
  } catch {
    // 非 JSON 错误响应
  }
  const codeKey = code ? CODE_KEYS[code] : undefined;
  if (codeKey) {
    return new ApiError(i18n(codeKey), res.status, code);
  }
  if (res.status === 401 && !message) {
    message = i18n("api.unauthorized");
  }
  const statusKey = STATUS_KEYS[res.status];
  const fallback = statusKey ? i18n(statusKey) : i18n("api.requestFailed", { status: res.status });
  return new ApiError(message || fallback, res.status, code);
}

export function errorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    return error.message;
  }
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return i18n("api.unknown");
}

interface RequestOptions {
  method?: string;
  body?: unknown;
}

async function request<T>(url: string, options: RequestOptions = {}): Promise<T> {
  const init: RequestInit = {
    method: options.method ?? "GET",
    credentials: "same-origin",
    headers: { Accept: "application/json" },
  };
  if (options.body !== undefined) {
    init.headers = {
      ...(init.headers as Record<string, string>),
      "Content-Type": "application/json",
    };
    init.body = JSON.stringify(options.body);
  }
  let res: Response;
  try {
    res = await fetch(withBase(url), init);
  } catch {
    throw new ApiError(i18n("api.network"), 0, "network_error");
  }
  if (!res.ok) {
    throw await parseErrorResponse(res);
  }
  const contentType = res.headers.get("content-type") ?? "";
  if (contentType.includes("application/json")) {
    return (await res.json()) as T;
  }
  return undefined as T;
}

function query(params: Record<string, string | number | boolean | undefined>): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === "") {
      continue;
    }
    search.set(key, String(value));
  }
  const text = search.toString();
  return text ? `?${text}` : "";
}

export const api = {
  me(): Promise<AuthInfo> {
    return request<AuthInfo>("/api/auth/me");
  },
  login(payload: LoginRequest): Promise<AuthInfo> {
    return request<AuthInfo>("/api/auth/login", { method: "POST", body: payload });
  },
  logout(): Promise<AuthInfo> {
    return request<AuthInfo>("/api/auth/logout", { method: "POST" });
  },
  mounts(): Promise<MountsResult> {
    return request<MountsResult>("/api/mounts");
  },
  list(mount: string, path: string): Promise<ListResult> {
    return request<ListResult>(`/api/list${query({ mount, path })}`);
  },
  mkdir(mount: string, path: string, name: string): Promise<APIResult> {
    return request<APIResult>("/api/mkdir", { method: "POST", body: { mount, path, name } });
  },
  rename(mount: string, path: string, newName: string): Promise<APIResult> {
    return request<APIResult>("/api/rename", { method: "POST", body: { mount, path, newName } });
  },
  remove(mount: string, paths: string[]): Promise<APIResult> {
    return request<APIResult>("/api/delete", { method: "POST", body: { mount, paths } });
  },
  createShare(payload: ShareCreateRequest): Promise<ShareCreated> {
    return request<ShareCreated>("/api/share", { method: "POST", body: payload });
  },
  shareInfo(token: string): Promise<ShareInfo> {
    return request<ShareInfo>(`/api/s/${encodeURIComponent(token)}/info`);
  },
  shareUnlock(token: string, password: string): Promise<APIResult> {
    return request<APIResult>(`/api/s/${encodeURIComponent(token)}/unlock`, {
      method: "POST",
      body: { password },
    });
  },
  shareList(token: string, path: string): Promise<ListResult> {
    return request<ListResult>(`/api/s/${encodeURIComponent(token)}/list${query({ path })}`);
  },
};

/** 普通下载 / 预览地址 */
export function downloadUrl(mount: string, path: string, inline = false): string {
  return withBase(`/api/dl${query({ mount, path, inline: inline ? "1" : "" })}`);
}

/** 普通目录打包地址 */
export function zipUrl(mount: string, path: string): string {
  return withBase(`/api/zip${query({ mount, path })}`);
}

/** 分享内下载 / 预览地址 */
export function shareDownloadUrl(token: string, path: string, inline = false): string {
  return withBase(`/api/s/${encodeURIComponent(token)}/dl${query({ path, inline: inline ? "1" : "" })}`);
}

/** 分享内打包地址 */
export function shareZipUrl(token: string, path: string): string {
  return withBase(`/api/s/${encodeURIComponent(token)}/zip${query({ path })}`);
}

/** 普通上传 tus creation endpoint */
export function tusEndpoint(mount: string): string {
  return withBase(`/api/tus/create/${encodeURIComponent(mount)}`);
}

/** 分享收集上传 tus creation endpoint */
export function shareTusEndpoint(token: string): string {
  return withBase(`/api/s/${encodeURIComponent(token)}/tus/create`);
}
