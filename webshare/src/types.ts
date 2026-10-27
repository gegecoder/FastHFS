export type AnonymousMode = "deny" | "readonly" | "upload";

export interface AuthInfo {
  authenticated: boolean;
  username: string;
  anonymousMode: AnonymousMode;
  canLogin: boolean;
}

export interface MountOption {
  alias: string;
  name: string;
  allowBrowse: boolean;
  allowDownload: boolean;
  allowUpload: boolean;
}

export interface MountsResult {
  authRequired: boolean;
  anonymous: AnonymousMode;
  mounts: MountOption[];
}

export interface Entry {
  name: string;
  isDir: boolean;
  size: number;
  modTime: number;
  mime: string;
}

export interface ListResult {
  mount: string;
  mountName: string;
  path: string;
  parent: string;
  entries: Entry[];
  canUpload: boolean;
  canDelete: boolean;
  canDownload: boolean;
  canShare: boolean;
}

export interface APIResult {
  success: boolean;
  message: string;
}

export interface LoginRequest {
  username: string;
  password: string;
}

export type ShareMode = "preview" | "download" | "collect";

export interface Share {
  id?: string;
  token: string;
  mountAlias?: string;
  mountName: string;
  path: string;
  mode: ShareMode;
  hasPassword: boolean;
  expiresAt: number;
  maxVisits: number;
  visits: number;
  createdAt?: number;
}

export interface ShareInfo {
  token: string;
  mountName: string;
  path: string;
  mode: ShareMode;
  hasPassword: boolean;
  expiresAt: number;
  maxVisits: number;
  visits: number;
  authenticated: boolean;
}

export interface ShareCreated {
  share: Share;
  url: string;
}

export interface ShareCreateRequest {
  mount: string;
  path: string;
  password: string;
  expiresIn: number;
  maxVisits: number;
  mode: ShareMode;
}

export type SortField = "name" | "size" | "modTime";
export type SortOrder = "asc" | "desc";
export type ViewMode = "list" | "grid";

export type PreviewKind = "image" | "text" | "pdf" | "audio" | "unsupported";
