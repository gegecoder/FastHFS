import { readonly, ref, watch } from "vue";

export type Theme = "light" | "dark";

const STORAGE_KEY = "fasthfs-webshare-theme";

function initialTheme(): Theme {
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored === "light" || stored === "dark") {
    return stored;
  }
  if (window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches) {
    return "dark";
  }
  return "light";
}

const theme = ref<Theme>(initialTheme());

function applyTheme(value: Theme): void {
  const root = document.documentElement;
  root.dataset.theme = value;
  root.style.colorScheme = value;
}

applyTheme(theme.value);

watch(theme, (value) => {
  localStorage.setItem(STORAGE_KEY, value);
  applyTheme(value);
});

export function useTheme() {
  function toggleTheme(): void {
    theme.value = theme.value === "dark" ? "light" : "dark";
  }

  function setTheme(value: Theme): void {
    theme.value = value;
  }

  return { theme: readonly(theme), toggleTheme, setTheme };
}
