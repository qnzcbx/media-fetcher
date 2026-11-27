<script setup lang="ts">
import { X } from "@lucide/vue";
import BaseSheet from "../common/BaseSheet.vue";
import DownloadDirectorySetting from "./DownloadDirectorySetting.vue";
import ThemeSelector from "./ThemeSelector.vue";
import type { AudioFormat, ThemePreference } from "../../utils/preferences";

const AUDIO_FORMATS: AudioFormat[] = ["original", "m4a", "mp3"];

defineProps<{
  open: boolean;
  downloadDirectory: string;
  theme: ThemePreference;
  defaultAudioFormat: AudioFormat;
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "update:downloadDirectory", value: string): void;
  (e: "update:theme", value: ThemePreference): void;
  (e: "update:defaultAudioFormat", value: AudioFormat): void;
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
        <div class="format-row">
          <span>默认音频格式</span>
          <div class="format-switch" role="group" aria-label="默认音频格式">
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

.format-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font-size: 14px;
  color: var(--text-main);
}

.format-switch {
  display: inline-flex;
  gap: 2px;
  padding: 2px;
  border-radius: 8px;
  background: var(--input-bg);
}
</style>
