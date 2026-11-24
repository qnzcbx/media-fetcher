<script setup lang="ts">
import { computed } from "vue";

const props = withDefaults(defineProps<{
  count: number;
  totalSelectable: number;
  hideSelectAll?: boolean;
}>(), {
  hideSelectAll: false,
});

const emit = defineEmits<{
  (e: "remove"): void;
  (e: "selectAll"): void;
}>();

const allSelected = computed(() => props.totalSelectable > 0 && props.count === props.totalSelectable);
</script>

<template>
  <div class="selection-bar">
    <div class="selection-bar-inner">
      <div class="left">
        <span>已选择 {{ count }} 项</span>
        <button v-if="!hideSelectAll" class="btn-text" @click="$emit('selectAll')">{{ allSelected ? '取消全选' : '全选' }}</button>
      </div>
      <div class="right">
        <button class="btn-danger" :disabled="count === 0" @click="$emit('remove')">移除记录</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.selection-bar {
  background: var(--card-bg);
  border-top: 1px solid var(--border);
  box-shadow: 0 -2px 10px var(--shadow-color);
}
.selection-bar-inner {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 20px;
  max-width: 1160px;
  margin: 0 auto;
}
.left {
  display: flex;
  align-items: center;
  gap: 16px;
  font-size: 14px;
}
.right {
  display: flex;
  gap: 12px;
}
.btn-text {
  background: none;
  border: none;
  color: var(--primary);
  cursor: pointer;
  padding: 4px 8px;
}
.btn-text:hover {
  background: var(--input-bg);
  border-radius: 4px;
}
.btn-danger {
  background: var(--card-bg);
  border: 1px solid var(--danger);
  color: var(--danger);
  padding: 6px 16px;
  border-radius: 6px;
  cursor: pointer;
}
.btn-danger:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.btn-danger:hover:not(:disabled) {
  background: var(--danger-bg);
}
</style>
