import assert from "node:assert/strict";
import { createServer } from "vite";

const vite = await createServer({ root: "frontend", server: { middlewareMode: true, ws: false } });
try {
  const { DEFAULT_PREFERENCES, normalizePreferences, parsePreferences, resolveTheme } =
    await vite.ssrLoadModule("/src/utils/preferences.ts");

  // 首次启动与无效存储回退默认设置。
  assert.deepEqual(parsePreferences(null), DEFAULT_PREFERENCES);
  assert.deepEqual(parsePreferences(""), DEFAULT_PREFERENCES);
  assert.deepEqual(parsePreferences("not json"), DEFAULT_PREFERENCES);
  assert.deepEqual(parsePreferences(JSON.stringify({ theme: "blue", defaultAudioFormat: "wav" })), DEFAULT_PREFERENCES);
  assert.deepEqual(parsePreferences(JSON.stringify([])), DEFAULT_PREFERENCES);

  // 合法字段保留，非法字段单独回退。
  assert.deepEqual(
    parsePreferences(JSON.stringify({ theme: "dark", defaultAudioFormat: "mp3" })),
    {
      theme: "dark",
      defaultAudioFormat: "mp3",
      downloadDirectory: "",
      autoDetectClipboard: false,
      clipboardNoticeDismissed: false,
    },
  );
  assert.deepEqual(
    parsePreferences(JSON.stringify({ theme: "dark", defaultAudioFormat: "wav", autoDetectClipboard: true })),
    {
      theme: "dark",
      defaultAudioFormat: "m4a",
      downloadDirectory: "",
      autoDetectClipboard: true,
      clipboardNoticeDismissed: false,
    },
  );
  assert.deepEqual(
    normalizePreferences({ theme: "light", defaultAudioFormat: "mp3", autoDetectClipboard: "yes", clipboardNoticeDismissed: 1 }),
    {
      theme: "light",
      defaultAudioFormat: "mp3",
      downloadDirectory: "",
      autoDetectClipboard: false,
      clipboardNoticeDismissed: false,
    },
  );

  const custom = {
    theme: "dark",
    defaultAudioFormat: "mp3",
    downloadDirectory: "C:\\媒体 文件",
    autoDetectClipboard: true,
    clipboardNoticeDismissed: true,
  };
  assert.deepEqual(parsePreferences(JSON.stringify(custom)), custom);
  for (const downloadDirectory of [22, {}, "relative/path"]) {
    assert.deepEqual(normalizePreferences({ ...custom, downloadDirectory }), { ...custom, downloadDirectory: "" });
  }
  assert.deepEqual(normalizePreferences({ ...custom, downloadDirectory: "" }), { ...custom, downloadDirectory: "" });

  // Light / Dark / System 解析：System 跟随系统，手动模式不受系统影响。
  assert.equal(resolveTheme("light", true), "light");
  assert.equal(resolveTheme("dark", false), "dark");
  assert.equal(resolveTheme("system", true), "dark");
  assert.equal(resolveTheme("system", false), "light");

  console.log("应用偏好与主题解析回归通过");
} finally {
  await vite.close();
}
