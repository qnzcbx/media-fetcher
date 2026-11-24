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
  display: flex;
  gap: 2px;
  padding: 3px;
  border-radius: 10px;
  background: var(--input-bg);
}

.mode-selector button {
  flex: 1;
  border: 0;
  background: transparent;
  color: var(--text-sub);
  border-radius: 7px;
  padding: 6px 14px;
  font: inherit;
  font-size: 14px;
  cursor: pointer;
  transition: background 0.18s, color 0.18s;
}

.mode-selector button:hover:not(:disabled) {
  color: var(--text-main);
}

.mode-selector button[aria-pressed="true"] {
  background: #caedf6;
  color: #17323c;
  font-weight: 500;
}

.mode-selector button:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.mode-selector button:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 1px;
}
</style>
