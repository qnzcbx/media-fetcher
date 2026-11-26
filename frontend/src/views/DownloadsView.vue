<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { ChevronLeft } from "@lucide/vue";
import type { GalleryCard } from "../composables/useDownloads";
import DownloadListItem from "../components/download/DownloadListItem.vue";
import GallerySelectionBar from "../components/download/GallerySelectionBar.vue";
import { selectableCount, useGallerySelection } from "../composables/useGallerySelection";

const props = defineProps<{ cards: GalleryCard[]; busy: boolean; error: string }>();
const emit = defineEmits<{
  action: [card: GalleryCard, action: string];
  cancel: [];
  back: [];
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
  isSelectionMode,
  selectedIds,
  enterSelectionMode,
  exitSelectionMode,
  toggleSelection,
  selectAll,
  collectSelectableIds,
  selectedCount,
} = useGallerySelection();

watch(() => props.cards.map(card => card.id), (ids) => {
  for (const id of selectedIds.value) if (!ids.includes(id)) selectedIds.value.delete(id);
});
watch(filter, () => {
  exitSelectionMode();
});

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
  <div class="downloads-view">
    <div class="content-wrapper">
      <header class="header">
        <div class="left">
          <button class="icon-btn" @click="emit('back')"><ChevronLeft :size="24" /></button>
          <h1>我的下载</h1>
        </div>
        <div class="right">
          <button
            class="btn-text"
            @click="isSelectionMode ? exitSelectionMode() : enterSelectionMode()"
          >
            {{ isSelectionMode ? "取消选择" : "选择" }}
          </button>
        </div>
      </header>

      <div class="filters">
        <nav>
          <button
            v-for="item in filters"
            :key="item.id"
            class="filter-tab"
            :class="{ active: filter === item.id }"
            @click="filter = item.id"
          >
            {{ item.name }}
            <div class="indicator" v-if="filter === item.id"></div>
          </button>
        </nav>
      </div>

      <div class="list-container">
        <p v-if="error" class="error-msg" role="alert">{{ error }}</p>

        <div class="list">
          <DownloadListItem
            v-for="card in filtered"
            :key="card.id"
            :card="card"
            :busy="busy"
            :isSelectionMode="isSelectionMode"
            :isSelected="selectedIds.has(card.id)"
            @action="(item: GalleryCard, action: string) => emit('action', item, action)"
            @cancel="emit('cancel')"
            @toggleSelect="toggleSelection(card)"
          />
          <div v-if="filtered.length === 0" class="empty-state">没有记录</div>
        </div>
      </div>

      <GallerySelectionBar :busy="busy"
      @recycle="emit('recycleSelected', [...selectedIds])"
        v-if="isSelectionMode"
        class="selection-bar-footer"
        :count="selectedCount"
        :totalSelectable="selectableCount(filtered)"
        @remove="handleRemove"
        @selectAll="selectAll(filtered)"
      />
    </div>
  </div>
</template>

<style scoped>
.downloads-view {
  flex: 1;
  display: flex;
  width: 100%;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  background: var(--bg-color);
}

.content-wrapper {
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
  width: 100%;
  max-width: 1200px;
  height: 100%;
  min-height: 0;
  margin: 0 auto;
  padding: 0 20px;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 24px 0 16px;
  flex-shrink: 0;
}
.header .left {
  display: flex;
  align-items: center;
  gap: 12px;
}
.icon-btn {
  background: transparent;
  border: none;
  cursor: pointer;
  color: var(--text-main);
  padding: 4px;
  display: flex;
  align-items: center;
  margin-left: -8px;
}
.header h1 {
  font-size: 20px;
  font-weight: 500;
  margin: 0;
}
.btn-text {
  background: transparent;
  border: none;
  color: var(--primary);
  font-size: 15px;
  cursor: pointer;
}

.filters {
  margin-bottom: 24px;
  flex-shrink: 0;
}
.filters nav {
  display: flex;
  gap: 24px;
}
.filter-tab {
  background: transparent;
  border: none;
  padding: 0 0 12px 0;
  font-size: 15px;
  color: var(--text-sub);
  cursor: pointer;
  position: relative;
}
.filter-tab.active {
  color: var(--text-main);
  font-weight: 500;
}
.indicator {
  position: absolute;
  bottom: 0;
  left: 50%;
  transform: translateX(-50%);
  width: 20px;
  height: 3px;
  background: var(--primary);
  border-radius: 3px 3px 0 0;
}

.list-container {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  width: 100%;
  padding-bottom: 16px;
}
.list {
  background: var(--card-bg);
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 2px 8px var(--shadow-color);
}
.list > *:last-child {
  border-bottom: none;
}
.empty-state {
  padding: 40px;
  text-align: center;
  color: var(--text-sub);
  font-size: 14px;
}

.error-msg {
  color: var(--danger);
  text-align: center;
  margin-bottom: 16px;
  font-size: 14px;
}

.selection-bar-footer {
  position: static;
  flex-shrink: 0;
  margin: 0;
  border-radius: 0;
}
</style>
