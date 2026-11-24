<script setup lang="ts">
defineProps<{
  modelValue: string;
}>();

const emit = defineEmits<{
  (e: "update:modelValue", value: string): void;
}>();
</script>

<template>
  <div class="format-options flex-between">
    <span class="format-label">输出格式</span>
    <div class="format-switch" role="group" aria-label="输出格式">
      <button
        type="button"
        class="format-button"
        title="优先保留兼容的原始 AAC 音轨"
        :aria-pressed="modelValue === 'm4a'"
        @click="emit('update:modelValue', 'm4a')"
      >
        M4A
      </button>
      <button
        type="button"
        class="format-button"
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
  @apply min-h-[44px] px-4 rounded-lg;
  gap: 12px;
  background: var(--input-bg);
  box-sizing: border-box;
}

.format-label {
  font-size: 14px;
  color: var(--text-main);
}

.format-switch {
  @apply inline-flex gap-0.5 rounded-lg p-0.5;
  background: var(--card-bg);
}

.format-button {
  @apply border-0 bg-transparent text-muted cursor-pointer transition-colors rounded-md;
  padding: 4px 14px;
  font-size: 13px;
  transition-duration: 180ms;
}

.format-button:hover {
  @apply text-main;
}

.format-button:focus-visible {
  outline: 2px solid var(--focus-ring);
  outline-offset: 1px;
}

.format-button[aria-pressed="true"] {
  background: var(--selection-bg);
  color: var(--selection-text);
  font-weight: 500;
}
</style>
