<script setup lang="ts">
import { Trash2 } from "@lucide/vue";
import { computed } from "vue";

const props = withDefaults(
  defineProps<{
    busy?: boolean;
    count: number;
    totalSelectable: number;
    hideSelectAll?: boolean;
  }>(),
  {
    hideSelectAll: false,
  },
);

const emit = defineEmits<{
  (e: "remove"): void;
  (e: "recycle"): void;
  (e: "selectAll"): void;
}>();

const allSelected = computed(
  () => props.totalSelectable > 0 && props.count === props.totalSelectable,
);
</script>

<template>
  <div class="selection-bar">
    <div class="selection-bar-inner">
      <div class="left">
        <span>已选择 {{ count }} 项</span>
        <button v-if="!hideSelectAll" class="btn-text" @click="$emit('selectAll')">
          {{ allSelected ? "取消全选" : "全选" }}
        </button>
      </div>
      
      <div class="right">
        <button
          class="trash"
          :disabled="count === 0 || busy"
          title="删除"
          aria-label="删除"
          @click="$emit('recycle')"
        >
          <Trash2 :size="18" />
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.trash {
  display: grid;
  place-items: center;
  padding: 8px;
  border: 0;
  background: none;
  color: var(--text-assist);
  cursor: pointer;
}
.trash:hover {
  color: var(--danger);
}
.trash:focus-visible {
  outline: 2px solid var(--focus-ring);
}
.trash:disabled {
  opacity: 0.4;
  cursor: default;
}
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
</style>
