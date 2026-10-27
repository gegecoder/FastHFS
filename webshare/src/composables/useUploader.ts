import { computed, ref } from "vue";
import { Upload } from "tus-js-client";
import { i18n } from "../i18n";
import { joinRel, relativeParts } from "../utils";

export type UploadStatus = "queued" | "uploading" | "success" | "error" | "canceled";

export interface UploadTask {
  id: number;
  file: File;
  name: string;
  size: number;
  dir: string;
  endpoint: string;
  sent: number;
  total: number;
  progress: number;
  speed: number;
  status: UploadStatus;
  error: string;
  upload: Upload | null;
}

export interface AddFilesOptions {
  endpoint: string;
  dir: string;
}

interface UploaderOptions {
  concurrency?: number;
  onComplete?: (task: UploadTask) => void;
  onError?: (task: UploadTask) => void;
}

const CHUNK_SIZE = 8 * 1024 * 1024;

export function useUploader(options: UploaderOptions = {}) {
  const tasks = ref<UploadTask[]>([]);
  const concurrency = Math.max(1, options.concurrency ?? 3);
  let sequence = 0;

  const activeCount = computed(() => tasks.value.filter((task) => task.status === "uploading").length);
  const uploadingCount = computed(
    () => tasks.value.filter((task) => task.status === "uploading" || task.status === "queued").length,
  );
  const finishedCount = computed(
    () => tasks.value.filter((task) => task.status === "success" || task.status === "error").length,
  );
  const totalProgress = computed(() => {
    const total = tasks.value.reduce((sum, task) => sum + task.total, 0);
    if (total <= 0) {
      return 0;
    }
    const sent = tasks.value.reduce((sum, task) => sum + task.sent, 0);
    return Math.min(100, (sent / total) * 100);
  });

  function pump(): void {
    while (activeCount.value < concurrency) {
      const next = tasks.value.find((task) => task.status === "queued");
      if (!next) {
        break;
      }
      startTask(next);
    }
  }

  function startTask(task: UploadTask): void {
    task.status = "uploading";
    task.error = "";
    task.speed = 0;
    task.sent = Math.min(task.sent, task.size);
    task.progress = task.size > 0 ? (task.sent / task.size) * 100 : 0;

    let lastTime = 0;
    let lastSent = 0;
    let smoothedSpeed = 0;

    const metadata: Record<string, string> = { filename: task.name };
    if (task.dir) {
      metadata.dir = task.dir;
    }

    const upload = new Upload(task.file, {
      endpoint: task.endpoint,
      metadata,
      retryDelays: [0, 1000, 3000, 5000],
      chunkSize: CHUNK_SIZE,
      removeFingerprintOnSuccess: true,
      onProgress: (sent, total) => {
        if (task.status === "canceled") {
          return;
        }
        const now = Date.now();
        if (lastTime === 0) {
          lastTime = now;
          lastSent = sent;
        } else if (now - lastTime >= 300) {
          const instant = (sent - lastSent) / ((now - lastTime) / 1000);
          smoothedSpeed = smoothedSpeed > 0 ? smoothedSpeed * 0.5 + instant * 0.5 : instant;
          task.speed = smoothedSpeed;
          lastTime = now;
          lastSent = sent;
        }
        task.sent = sent;
        task.total = total || task.size;
        task.progress = task.total > 0 ? Math.min(100, (sent / task.total) * 100) : 0;
      },
      onSuccess: () => {
        task.sent = task.total;
        task.progress = 100;
        task.speed = 0;
        task.status = "success";
        task.upload = null;
        options.onComplete?.(task);
        pump();
      },
      onError: (error) => {
        if (task.status === "canceled") {
          return;
        }
        task.upload = null;
        task.speed = 0;
        task.status = "error";
        task.error = describeUploadError(error);
        options.onError?.(task);
        pump();
      },
    });

    task.upload = upload;
    upload.start();
  }

  function describeUploadError(error: Error): string {
    const original = (error as { originalResponse?: { getStatus?: () => number } }).originalResponse;
    const status = original && typeof original.getStatus === "function" ? original.getStatus() : 0;
    switch (status) {
      case 401:
        return i18n("upload.errUnauthorized");
      case 403:
        return i18n("upload.errForbidden");
      case 404:
        return i18n("upload.errNotFound");
      case 409:
        return i18n("upload.errConflict");
      case 410:
        return i18n("upload.errGone");
      case 413:
        return i18n("upload.errTooLarge");
      case 429:
        return i18n("upload.errRateLimited");
      default:
        break;
    }
    if (status >= 400) {
      return i18n("upload.errHttp", { status });
    }
    return error.message && error.message !== "tus: failed to upload chunk"
      ? error.message
      : i18n("upload.errNetwork");
  }

  function addFiles(files: File[], opts: AddFilesOptions): number {
    if (!files.length) {
      return 0;
    }
    const next: UploadTask[] = [];
    for (const file of files) {
      const parts = relativeParts(file);
      next.push({
        id: ++sequence,
        file,
        name: parts.name || file.name,
        size: file.size,
        dir: joinRel(opts.dir, parts.dir),
        endpoint: opts.endpoint,
        sent: 0,
        total: file.size,
        progress: 0,
        speed: 0,
        status: "queued",
        error: "",
        upload: null,
      });
    }
    tasks.value = [...tasks.value, ...next];
    pump();
    return next.length;
  }

  function cancel(task: UploadTask): void {
    const active = task.status === "uploading" || task.status === "queued";
    if (!active) {
      return;
    }
    task.status = "canceled";
    task.speed = 0;
    if (task.upload) {
      const upload = task.upload;
      task.upload = null;
      void upload.abort(true);
    }
    pump();
  }

  function retry(task: UploadTask): void {
    if (task.status !== "error" && task.status !== "canceled") {
      return;
    }
    task.status = "queued";
    task.error = "";
    pump();
  }

  function remove(task: UploadTask): void {
    if (task.status === "uploading" || task.status === "queued") {
      cancel(task);
    }
    tasks.value = tasks.value.filter((item) => item.id !== task.id);
  }

  function clearFinished(): void {
    tasks.value = tasks.value.filter((task) => task.status === "uploading" || task.status === "queued");
  }

  function reset(): void {
    for (const task of tasks.value) {
      cancel(task);
    }
    tasks.value = [];
  }

  return {
    tasks,
    activeCount,
    uploadingCount,
    finishedCount,
    totalProgress,
    addFiles,
    cancel,
    retry,
    remove,
    clearFinished,
    reset,
  };
}
