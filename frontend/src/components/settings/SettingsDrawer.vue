<script setup lang="ts">
import { X } from "@lucide/vue";
import BaseSheet from "../common/BaseSheet.vue";
import DownloadDirectorySetting from "./DownloadDirectorySetting.vue";
import RuntimeStatusSection from "./RuntimeStatusSection.vue";
import ThemeSelector from "./ThemeSelector.vue";
import type { AudioFormat, ThemePreference } from "../../utils/preferences";

const AUDIO_FORMATS: AudioFormat[] = ["original", "m4a", "mp3"];

defineProps<{
  open: boolean;
  downloadDirectory: string;
  theme: ThemePreference;
  defaultAudioFormat: AudioFormat;
  autoDetectClipboard: boolean;
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "update:downloadDirectory", value: string): void;
  (e: "update:theme", value: ThemePreference): void;
  (e: "update:defaultAudioFormat", value: AudioFormat): void;
  (e: "update:autoDetectClipboard", value: boolean): void;
}>();
</script>

<template>
  <BaseSheet
    :open="open"
    side="right"
    width="400px"
    title="设置"
    @update:open="(value) => !value && emit('close')"
  >
    <header class="settings-header">
      <h2>设置</h2>
      <button class="icon-btn" aria-label="关闭" @click="emit('close')">
        <X :size="20" />
      </button>
    </header>

    <div class="settings-content">
      <section class="settings-section">
        <h3>外观</h3>
        <ThemeSelector
          :modelValue="theme"
          @update:modelValue="(value) => emit('update:theme', value)"
        />
      </section>

      <div class="settings-divider"></div>

      <section class="settings-section">
        <h3>下载偏好</h3>
        <div class="format-row media-parameter-row">
          <span>默认音频格式</span>
          <div class="format-switch media-parameter-segments" role="group" aria-label="默认音频格式">
            <button
              v-for="format in AUDIO_FORMATS"
              :key="format"
              type="button"
              class="segmented-option"
              :aria-pressed="defaultAudioFormat === format"
              @click="emit('update:defaultAudioFormat', format)"
            >
              {{ format === "original" ? "原始" : format.toUpperCase() }}
            </button>
          </div>
        </div>
        <DownloadDirectorySetting
          :modelValue="downloadDirectory"
          @update:modelValue="(value) => emit('update:downloadDirectory', value)"
        />
      </section>

      <div class="settings-divider"></div>

      <section class="settings-section">
        <h3>剪贴板</h3>
        <div class="toggle-row">
          <div class="toggle-copy">
            <span class="toggle-title">自动识别剪贴板链接</span>
            <p class="toggle-desc">
              仅在进入或返回首页且输入框为空时读取本地剪贴板，提取单个受支持链接填入输入框；不自动解析、不记录剪贴板原文。
            </p>
          </div>
          <button
            type="button"
            role="switch"
            class="toggle-switch"
            :class="{ 'toggle-switch--on': autoDetectClipboard }"
            :aria-checked="autoDetectClipboard"
            aria-label="自动识别剪贴板链接"
            @click="emit('update:autoDetectClipboard', !autoDetectClipboard)"
          >
            <span class="toggle-thumb"></span>
          </button>
        </div>
      </section>

      <div class="settings-divider"></div>

      <RuntimeStatusSection :open="open" />
    </div>
  </BaseSheet>
</template>

<style scoped>
.settings-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px 24px;
  background: var(--card-bg);
  border-bottom: 1px solid var(--border);
}

.settings-header h2 {
  font-size: 20px;
  margin: 0;
  font-weight: 500;
}

.icon-btn {
  background: none;
  border: none;
  color: var(--text-main);
  cursor: pointer;
  display: flex;
  align-items: center;
  padding: 4px;
  border-radius: 6px;
}

.icon-btn:hover {
  background: var(--hover-bg);
}

.icon-btn:focus-visible {
  outline: 2px solid var(--focus-ring);
  outline-offset: 1px;
}

.settings-content {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
}

.settings-section h3 {
  margin: 0 0 14px;
  font-size: 14px;
  font-weight: 500;
  color: var(--text-sub);
}

.settings-section + .settings-section {
  margin-top: 24px;
}

.settings-divider {
  height: 1px;
  margin: 24px 0;
  background: var(--border);
}

.toggle-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.toggle-copy {
  flex: 1;
  min-width: 0;
}

.toggle-title {
  display: block;
  font-size: 14px;
  font-weight: 500;
  color: var(--text-main);
}

.toggle-desc {
  margin: 6px 0 0;
  font-size: 12px;
  line-height: 1.5;
  color: var(--text-assist);
}

.toggle-switch {
  position: relative;
  flex-shrink: 0;
  width: 42px;
  height: 24px;
  margin-top: 2px;
  padding: 2px;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: var(--input-bg);
  cursor: pointer;
  transition: background-color 0.18s ease, border-color 0.18s ease;
}

.toggle-switch--on {
  background: var(--primary);
  border-color: var(--primary-hover);
}

.toggle-switch:focus-visible {
  outline: 2px solid var(--focus-ring);
  outline-offset: 2px;
}

.toggle-thumb {
  display: block;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: var(--card-bg);
  box-shadow: 0 1px 2px var(--shadow-color);
  transform: translateX(0);
  transition: transform 0.18s ease;
}

.toggle-switch--on .toggle-thumb {
  transform: translateX(18px);
}

</style>
