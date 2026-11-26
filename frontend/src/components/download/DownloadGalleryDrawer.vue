<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { X, Check } from "@lucide/vue";
import type { GalleryCard } from "../../composables/useDownloads";
import DownloadCard from "./DownloadCard.vue";
import GallerySelectionBar from "./GallerySelectionBar.vue";
import {
  isSelectableCard,
  selectableCount,
  useGallerySelection,
} from "../../composables/useGallerySelection";
import BaseSheet from "../common/BaseSheet.vue";

const props = defineProps<{ cards: GalleryCard[]; busy: boolean; error: string; open: boolean }>();
const emit = defineEmits<{
  action: [card: GalleryCard, action: string];
  cancel: [];
  close: [];
  removeSelected: [ids: string[]];
  recycleSelected: [ids: string[]];
}>();

const filter = ref("");
const filters = computed(() => [
  { id: "", name: "全部" },
  { id: "video", name: "视频" },
  { id: "image", name: "图片" },
  ...(props.cards.some((item) => item.kind === "audio") ? [{ id: "audio", name: "音频" }] : []),
]);
const filtered = computed(() =>
  props.cards.filter((item) => !filter.value || item.kind === filter.value),
);

const {
  selectedIds,
  exitSelectionMode,
  toggleSelection,
  selectAll,
  collectSelectableIds,
  selectedCount,
} = useGallerySelection();

watch(
  () => props.open,
  (val) => {
    if (!val) exitSelectionMode();
  },
);

watch(
  () => props.cards.map((card) => card.id),
  (ids) => {
    for (const id of selectedIds.value) if (!ids.includes(id)) selectedIds.value.delete(id);
  },
);
watch(filter, () => {
  exitSelectionMode();
});

const totalSelectable = computed(() => selectableCount(filtered.value));
const allSelected = computed(
  () => totalSelectable.value > 0 && selectedCount.value === totalSelectable.value,
);

const handleRemove = () => {
  // 执行前按最新状态复核，活动任务不会被移除。
  const ids = collectSelectableIds(props.cards);
  if (!ids.length) {
    exitSelectionMode();
    return;
  }
  emit("recycleSelected", ids);
};
</script>

<template>
  <BaseSheet
    :open="open"
    @update:open="(val) => !val && emit('close')"
    side="right"
    width="500px"
    title="快捷图库"
  >
    <header class="drawer-header">
      <div class="left">
        <h2>我的下载</h2>
      </div>
      <div class="right">
        <button class="btn-text" @click="selectAll(filtered)">
          {{ allSelected ? "取消全选" : "全选" }}
        </button>
        <button class="icon-btn" aria-label="关闭" @click="emit('close')"><X :size="20" /></button>
      </div>
    </header>

    <div class="drawer-content">
      <p v-if="error" role="alert" class="error-text">{{ error }}</p>

      <nav aria-label="资源类型">
        <button
          v-for="item in filters"
          :key="item.id"
          :aria-pressed="filter === item.id"
          @click="filter = item.id"
        >
          {{ item.name }}
        </button>
      </nav>

      <div class="grid">
        <div
          v-for="card in filtered"
          :key="card.id"
          class="card-wrapper"
          :class="{ selected: selectedIds.has(card.id) }"
        >
          <DownloadCard
            hideMore
            :card="card"
            :busy="busy"
            @action="(item, action) => selectedCount === 0 && emit('action', item, action)"
            @cancel="selectedCount === 0 && emit('cancel')"
            :style="selectedCount > 0 ? 'pointer-events: none;' : 'pointer-events: auto;'"
          />
          <div
            class="selection-interaction-layer"
            :style="selectedCount > 0 ? 'pointer-events: auto;' : 'pointer-events: none;'"
            @click="selectedCount > 0 && isSelectableCard(card) && toggleSelection(card)"
          >
            <div
              v-if="isSelectableCard(card)"
              class="check-circle"
              :class="{ selected: selectedIds.has(card.id) }"
              style="pointer-events: auto"
              @click.stop="toggleSelection(card)"
            >
              <Check v-if="selectedIds.has(card.id)" :size="14" color="#fff" strokeWidth="3" />
            </div>
          </div>
        </div>
      </div>
    </div>

    <GallerySelectionBar
      :busy="busy"
      @recycle="emit('recycleSelected', [...selectedIds])"
      v-if="selectedCount > 0"
      :count="selectedCount"
      :totalSelectable="totalSelectable"
      hide-select-all
      @remove="handleRemove"
      style="border-radius: 0; box-shadow: 0 -2px 10px var(--shadow-color)"
    />
  </BaseSheet>
</template>

<style scoped>
.drawer-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px 24px;
  background: var(--card-bg);
  border-bottom: 1px solid var(--border);
}

.drawer-header h2 {
  font-size: 20px;
  margin: 0;
  font-weight: 500;
}

.drawer-header .right {
  display: flex;
  align-items: center;
  gap: 16px;
}

.btn-text {
  background: none;
  border: none;
  color: var(--primary);
  cursor: pointer;
  font-size: 14px;
}

.icon-btn {
  background: none;
  border: none;
  color: var(--text-main);
  cursor: pointer;
  display: flex;
  align-items: center;
}

.drawer-content {
  flex: 1;
  overflow-y: auto;
  padding: 20px 24px;
}

nav {
  display: flex;
  gap: 8px;
  margin-bottom: 20px;
}

nav button {
  border: 0;
  border-radius: 8px;
  background: var(--input-bg);
  color: var(--text-main);
  padding: 6px 14px;
  cursor: pointer;
  font-size: 14px;
}

nav button[aria-pressed="true"] {
  background: var(--selection-bg);
  color: var(--selection-text);
}

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(130px, 1fr));
  gap: 12px;
}

.card-wrapper {
  position: relative;
  cursor: pointer;
  border-radius: 14px;
}

.card-wrapper.selected {
  outline: 3px solid var(--primary);
  outline-offset: -3px;
}

.selection-interaction-layer {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: transparent;
  border-radius: 14px;
  transition: background 0.2s;
}

.card-wrapper.selected .selection-interaction-layer {
  background: var(--selection-soft);
}

.check-circle {
  position: absolute;
  top: 8px;
  right: 8px;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  border: 2px solid rgba(255, 255, 255, 0.8);
  background: rgba(0, 0, 0, 0.3);
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
  z-index: 10;
}

.check-circle.selected {
  background: var(--primary);
  border-color: var(--primary);
}

.check-circle:hover {
  transform: scale(1.05);
  border-color: #fff;
}

.error-text {
  color: var(--danger);
  font-size: 13px;
  margin-bottom: 12px;
}
</style>
