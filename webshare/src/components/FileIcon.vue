<script setup lang="ts">
import { computed } from "vue";
import { fileExtension } from "../utils";

const props = withDefaults(
  defineProps<{
    name: string;
    isDir: boolean;
    mime?: string;
    size?: number;
  }>(),
  { mime: "", size: 24 },
);

const ARCHIVE_EXTENSIONS = new Set(["zip", "rar", "7z", "tar", "gz", "bz2", "xz", "zst", "tgz", "tbz2"]);
const VIDEO_EXTENSIONS = new Set(["mp4", "m4v", "mov", "webm", "mkv", "avi", "wmv", "flv"]);
const AUDIO_EXTENSIONS = new Set(["mp3", "wav", "ogg", "oga", "m4a", "aac", "flac", "opus", "weba"]);
const IMAGE_EXTENSIONS = new Set(["png", "jpg", "jpeg", "gif", "webp", "bmp", "svg", "ico", "avif", "tif", "tiff"]);

const category = computed(() => {
  if (props.isDir) {
    return "folder";
  }
  const mime = props.mime.toLowerCase();
  const ext = fileExtension(props.name);
  if (mime.startsWith("image/") || IMAGE_EXTENSIONS.has(ext)) {
    return "image";
  }
  if (mime.startsWith("audio/") || AUDIO_EXTENSIONS.has(ext)) {
    return "audio";
  }
  if (mime === "application/pdf" || ext === "pdf") {
    return "pdf";
  }
  if (mime.startsWith("video/") || VIDEO_EXTENSIONS.has(ext)) {
    return "video";
  }
  if (ARCHIVE_EXTENSIONS.has(ext) || mime.includes("zip") || mime.includes("compressed")) {
    return "archive";
  }
  if (
    mime.startsWith("text/") ||
    mime.includes("json") ||
    mime.includes("xml") ||
    mime.includes("javascript") ||
    ["md", "ts", "tsx", "js", "jsx", "vue", "go", "py", "rs", "java", "c", "cpp", "h", "css", "html", "yml", "yaml", "toml", "sh"].includes(ext)
  ) {
    return "text";
  }
  return "file";
});
</script>

<template>
  <span class="file-icon" :class="`file-icon-${category}`" :style="{ width: `${size}px`, height: `${size}px` }">
    <svg v-if="category === 'folder'" :width="size" :height="size" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="M3.2 7.2A2.2 2.2 0 0 1 5.4 5h4.1l2 2.3h7.1a2.2 2.2 0 0 1 2.2 2.2v8.3a2.2 2.2 0 0 1-2.2 2.2H5.4a2.2 2.2 0 0 1-2.2-2.2z"
        fill="currentColor"
      />
      <path d="M3.2 9.6h17.6" stroke="rgba(255,255,255,.35)" stroke-width="1.2" />
    </svg>
    <svg v-else :width="size" :height="size" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="M6.2 3h7.3l4.3 4.3V19a2 2 0 0 1-2 2H6.2a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2z"
        fill="currentColor"
        opacity="0.14"
      />
      <path
        d="M6.2 3h7.3l4.3 4.3V19a2 2 0 0 1-2 2H6.2a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2z"
        stroke="currentColor"
        stroke-width="1.5"
        stroke-linejoin="round"
      />
      <path d="M13.4 3.2v4.2h4.2" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" />
      <g v-if="category === 'image'" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="9.5" cy="11.2" r="1.3" />
        <path d="M6.6 17l3.1-3.4 2.4 2.3 2.2-2.6 3.1 3.7" />
      </g>
      <g v-else-if="category === 'audio'" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
        <path d="M10 16.2V10l5-1.3v6" />
        <circle cx="8.6" cy="16.3" r="1.4" />
        <circle cx="13.6" cy="15" r="1.4" />
      </g>
      <g v-else-if="category === 'video'" fill="currentColor">
        <path d="M10 10.3l4.6 2.9-4.6 2.9z" />
      </g>
      <g v-else-if="category === 'archive'" stroke="currentColor" stroke-width="1.3" stroke-linecap="round">
        <path d="M12 7.3v2M10.7 9.8h2.6v1.5h-2.6zM12 11.8v2M10.7 14.4h2.6v1.4h-2.6z" />
      </g>
      <g v-else-if="category === 'pdf'" fill="currentColor">
        <text x="6.6" y="17.2" font-size="4.6" font-weight="700" font-family="Arial, sans-serif" letter-spacing=".1">PDF</text>
      </g>
      <g v-else stroke="currentColor" stroke-width="1.4" stroke-linecap="round">
        <path d="M7.4 11h5M7.4 13.7h7M7.4 16.4h5.4" />
      </g>
    </svg>
  </span>
</template>

<style scoped>
.file-icon {
  display: inline-flex;
  flex: none;
  color: var(--icon-file);
}

.file-icon-folder {
  color: var(--icon-folder);
}

.file-icon-image {
  color: var(--icon-image);
}

.file-icon-audio {
  color: var(--icon-audio);
}

.file-icon-pdf {
  color: var(--icon-pdf);
}

.file-icon-text {
  color: var(--icon-text);
}

.file-icon-archive {
  color: var(--icon-archive);
}

.file-icon-video {
  color: var(--icon-video);
}
</style>
