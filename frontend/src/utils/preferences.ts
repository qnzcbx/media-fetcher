// 非敏感 UI 偏好的纯函数定义，供 localStorage 与单元测试共用。
export type ThemePreference = "light" | "dark" | "system";
export type ResolvedTheme = "light" | "dark";
export type AudioFormat = "m4a" | "mp3";

export type Preferences = {
  downloadDirectory: string;
  theme: ThemePreference;
  defaultAudioFormat: AudioFormat;
};

export const PREFERENCES_STORAGE_KEY = "media-fetcher.preferences.v1";

export const DEFAULT_PREFERENCES: Preferences = {
  downloadDirectory: "",
  theme: "system",
  defaultAudioFormat: "m4a",
};

const THEME_PREFERENCES: readonly ThemePreference[] = ["light", "dark", "system"];
const AUDIO_FORMATS: readonly AudioFormat[] = ["m4a", "mp3"];

const isThemePreference = (value: unknown): value is ThemePreference =>
  typeof value === "string" && THEME_PREFERENCES.includes(value as ThemePreference);

const isAudioFormat = (value: unknown): value is AudioFormat =>
  typeof value === "string" && AUDIO_FORMATS.includes(value as AudioFormat);

// 逐字段校验：无效字段回退默认值，合法字段继续保留。
export const normalizePreferences = (value: unknown): Preferences => {
  const record = (value && typeof value === "object" ? value : {}) as Record<string, unknown>;
  return {
    downloadDirectory: typeof record.downloadDirectory === "string" && !record.downloadDirectory.includes("\0") && (record.downloadDirectory === "" || /^(?:[a-z]:[\\/]|\\\\[^\\]+\\[^\\]+|\/)/i.test(record.downloadDirectory)) ? record.downloadDirectory : "",
    theme: isThemePreference(record.theme) ? record.theme : DEFAULT_PREFERENCES.theme,
    defaultAudioFormat: isAudioFormat(record.defaultAudioFormat)
      ? record.defaultAudioFormat
      : DEFAULT_PREFERENCES.defaultAudioFormat,
  };
};

// 存储内容缺失、损坏或解码失败时安全回退默认值。
export const parsePreferences = (raw: string | null): Preferences => {
  if (!raw) return { ...DEFAULT_PREFERENCES };
  try {
    return normalizePreferences(JSON.parse(raw));
  } catch {
    return { ...DEFAULT_PREFERENCES };
  }
};

// System 模式跟随系统配色，手动模式不受系统变化影响。
export const resolveTheme = (
  preference: ThemePreference,
  systemDark: boolean,
): ResolvedTheme => {
  if (preference === "system") return systemDark ? "dark" : "light";
  return preference;
};
