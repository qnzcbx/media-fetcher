<script setup lang="ts">
defineProps<{
  modelValue: string;
}>();

const emit = defineEmits<{
  (e: "update:modelValue", value: string): void;
}>();
</script>

<template>
  <div class="format-options">
    <span class="format-label">输出格式</span>
    <div class="format-switch" role="group" aria-label="输出格式">
      <button
        type="button"
        title="优先保留兼容的原始 AAC 音轨"
        :aria-pressed="modelValue === 'm4a'"
        @click="emit('update:modelValue', 'm4a')"
      >
        M4A
      </button>
      <button
        type="button"
        title="通过 FFmpeg 转码为 MP3"
        :aria-pressed="modelValue === 'mp3'"
        @click="emit('update:modelValue', 'mp3')"
      >
        MP3
      </button>
    </div>
  </div>
</template>

<style scoped>
.format-options {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 44px;
  padding: 0 16px;
  background: var(--input-bg);
  border-radius: 8px;
  box-sizing: border-box;
}

.format-label {
  font-size: 14px;
  color: var(--text-main);
}

.format-switch {
  display: inline-flex;
  gap: 2px;
  padding: 2px;
  background: var(--card-bg);
  border-radius: 8px;
}

.format-switch button {
  border: 0;
  background: transparent;
  color: var(--text-sub);
  font: inherit;
  font-size: 13px;
  padding: 4px 14px;
  border-radius: 6px;
  cursor: pointer;
  transition: background 0.18s, color 0.18s;
}

.format-switch button[aria-pressed="true"] {
  background: #caedf6;
  color: #17323c;
  font-weight: 500;
}

.format-switch button:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 1px;
}
</style>
