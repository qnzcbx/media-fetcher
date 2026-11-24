import { computed, onUnmounted, ref, watch } from "vue";
import {
  DEFAULT_PREFERENCES,
  PREFERENCES_STORAGE_KEY,
  parsePreferences,
  resolveTheme,
  type AudioFormat,
  type Preferences,
  type ResolvedTheme,
  type ThemePreference,
} from "../utils/preferences";

const SYSTEM_DARK_QUERY = "(prefers-color-scheme: dark)";

const readStoredPreferences = (): Preferences => {
  if (typeof localStorage === "undefined") return { ...DEFAULT_PREFERENCES };
  try {
    return parsePreferences(localStorage.getItem(PREFERENCES_STORAGE_KEY));
  } catch {
    return { ...DEFAULT_PREFERENCES };
  }
};

// 主题必须挂在 document 根节点，Teleport 到 body 的浮层才能继承。
export const applyResolvedTheme = (theme: ResolvedTheme) => {
  if (typeof document !== "undefined") {
    document.documentElement.dataset.theme = theme;
  }
};

const systemPrefersDark = () =>
  typeof window !== "undefined" && typeof window.matchMedia === "function"
    ? window.matchMedia(SYSTEM_DARK_QUERY).matches
    : false;

// 在 Vue 首次渲染前恢复主题，减少启动时的浅色闪烁。
export const initTheme = () => {
  applyResolvedTheme(resolveTheme(readStoredPreferences().theme, systemPrefersDark()));
};

// 由 App 层唯一创建，避免各组件重复注册系统主题监听器。
export const usePreferences = () => {
  const preferences = ref<Preferences>(readStoredPreferences());

  const media =
    typeof window !== "undefined" && typeof window.matchMedia === "function"
      ? window.matchMedia(SYSTEM_DARK_QUERY)
      : null;
  const systemDark = ref(media?.matches ?? false);

  const resolvedTheme = computed(() => resolveTheme(preferences.value.theme, systemDark.value));

  const onSystemChange = (event: MediaQueryListEvent) => {
    systemDark.value = event.matches;
  };
  media?.addEventListener("change", onSystemChange);

  watch(resolvedTheme, (theme) => applyResolvedTheme(theme), { immediate: true });

  watch(
    preferences,
    (value) => {
      if (typeof localStorage === "undefined") return;
      try {
        localStorage.setItem(PREFERENCES_STORAGE_KEY, JSON.stringify(value));
      } catch {
        // 存储不可用时仅影响持久化，不阻断主题切换。
      }
    },
    { deep: true },
  );

  onUnmounted(() => media?.removeEventListener("change", onSystemChange));

  const setTheme = (theme: ThemePreference) => {
    preferences.value = { ...preferences.value, theme };
  };

  const setDefaultAudioFormat = (format: AudioFormat) => {
    preferences.value = { ...preferences.value, defaultAudioFormat: format };
  };

  return { preferences, resolvedTheme, setTheme, setDefaultAudioFormat };
};
