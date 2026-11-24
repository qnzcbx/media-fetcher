<script setup lang="ts">
defineProps<{
  modelValue: string;
}>();

const emit = defineEmits<{
  (e: "update:modelValue", value: string): void;
}>();
</script>

<template>
  <div class="format-options flex-between gap-3">
    <span class="text-sm text-main">输出格式</span>
    <div class="format-switch inline-flex gap-0.5 rounded-lg bg-surface p-0.5" role="group" aria-label="输出格式">
      <button
        class="border-0 bg-transparent text-[13px] px-3.5 py-1 rounded-md text-muted cursor-pointer transition-colors duration-[180ms] hover:(text-main) focus-visible:(outline outline-2 outline-offset-1 outline-primary)"
        type="button"
        title="优先保留兼容的原始 AAC 音轨"
        :aria-pressed="modelValue === 'm4a'"
        @click="emit('update:modelValue', 'm4a')"
      >
        M4A
      </button>
      <button
        class="border-0 bg-transparent text-[13px] px-3.5 py-1 rounded-md text-muted cursor-pointer transition-colors duration-[180ms] hover:(text-main) focus-visible:(outline outline-2 outline-offset-1 outline-primary)"
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
  @apply min-h-[44px] px-4 rounded-lg;
  background: var(--input-bg);
  box-sizing: border-box;
}

.format-switch button[aria-pressed="true"] {
  @apply bg-[#caedf6] text-[#17323c] font-medium;
}
</style>
