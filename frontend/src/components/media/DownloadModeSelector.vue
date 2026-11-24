<script setup lang="ts">
// 下载模式的分段选择器，仅负责展示与切换。
type DownloadMode = {
  id: string;
  label: string;
  enabled: boolean;
};

defineProps<{
  modelValue: string;
  options: DownloadMode[];
}>();

const emit = defineEmits<{
  (e: "update:modelValue", value: string): void;
}>();
</script>

<template>
  <div class="mode-selector" role="group" aria-label="下载模式">
    <button
      v-for="item in options"
      :key="item.id"
      type="button"
      class="mode-button"
      :disabled="!item.enabled"
      :aria-pressed="modelValue === item.id"
      @click="item.enabled && emit('update:modelValue', item.id)"
    >
      {{ item.label }}
    </button>
  </div>
</template>

<style scoped>
.mode-selector {
  @apply flex gap-0.5 rounded-[10px] p-[3px];
  background: var(--input-bg);
}

.mode-button {
  @apply flex-1 border-0 bg-transparent text-muted cursor-pointer transition-colors;
  border-radius: 7px;
  padding: 6px 14px;
  font-size: 14px;
  line-height: normal;
  transition-duration: 180ms;
}

.mode-button:hover:not(:disabled) {
  @apply text-main;
}

.mode-button:focus-visible {
  outline: 2px solid var(--focus-ring);
  outline-offset: 1px;
}

.mode-button:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.mode-button[aria-pressed="true"] {
  background: var(--selection-bg);
  color: var(--selection-text);
  font-weight: 500;
}
</style>
