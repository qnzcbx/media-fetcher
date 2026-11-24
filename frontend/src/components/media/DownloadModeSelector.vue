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
  <div class="mode-selector flex gap-0.5 rounded-[10px] p-[3px] bg-field" role="group" aria-label="下载模式">
    <button
      v-for="item in options"
      :key="item.id"
      class="flex-1 border-0 bg-transparent rounded-[7px] px-3.5 py-1.5 text-[14px] leading-[normal] text-muted cursor-pointer transition-colors duration-[180ms] hover:(text-main) focus-visible:(outline outline-2 outline-offset-1 outline-primary) disabled:(opacity-45 cursor-not-allowed)"
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
/* 选中态依赖 aria-pressed，保留语义化状态选择器，颜色仍来自设计变量。 */
.mode-selector button[aria-pressed="true"] {
  @apply bg-[#caedf6] text-[#17323c] font-medium;
}
</style>
