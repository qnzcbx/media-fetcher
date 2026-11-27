<script setup lang="ts">
defineProps<{
  modelValue: string;
}>();

const emit = defineEmits<{
  (e: "update:modelValue", value: string): void;
}>();
</script>

<template>
  <div class="format-options media-parameter-row">
    <span class="format-label">输出格式</span>
    <div class="format-switch media-parameter-segments" role="group" aria-label="输出格式">
      <button type="button" class="segmented-option" :aria-pressed="modelValue === 'original'" @click="emit('update:modelValue', 'original')">原始</button>
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
