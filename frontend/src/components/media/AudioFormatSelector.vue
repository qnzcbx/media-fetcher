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
        class="segmented-option"
        title="优先保留兼容的原始 AAC 音轨"
        :aria-pressed="modelValue === 'm4a'"
        @click="emit('update:modelValue', 'm4a')"
      >
        M4A
      </button>
      <button
        type="button"
        class="segmented-option"
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




</style>
