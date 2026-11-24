import { ref, computed } from "vue";
import type { GalleryCard } from "./useDownloads";
import { isActiveStatus } from "../utils/downloadStatus";

// 活动任务（排队、下载中、取消中）不允许加入移除选择。
export const isSelectableCard = (card: GalleryCard) => !isActiveStatus(card.status);

export const selectableCount = (cards: GalleryCard[]) => cards.filter(isSelectableCard).length;

export const useGallerySelection = () => {
  const isSelectionMode = ref(false);
  const selectedIds = ref<Set<string>>(new Set());

  const enterSelectionMode = () => {
    isSelectionMode.value = true;
    selectedIds.value.clear();
  };

  const exitSelectionMode = () => {
    isSelectionMode.value = false;
    selectedIds.value.clear();
  };

  // 单选与全选使用同一套资格判断，避免绕过活动任务保护。
  const toggleSelection = (card: GalleryCard) => {
    if (!isSelectableCard(card)) return;
    if (selectedIds.value.has(card.id)) {
      selectedIds.value.delete(card.id);
    } else {
      selectedIds.value.add(card.id);
    }
  };

  const selectAll = (cards: GalleryCard[]) => {
    const selectable = cards.filter(isSelectableCard);
    const allSelected = selectable.length > 0 && selectable.every((card) => selectedIds.value.has(card.id));

    if (allSelected) {
      selectedIds.value.clear();
    } else {
      selectable.forEach((card) => selectedIds.value.add(card.id));
    }
  };

  // 执行批量移除前再次依据最新卡片状态过滤，防止选择后状态发生变化。
  const collectSelectableIds = (cards: GalleryCard[]) => {
    const byId = new Map(cards.map((card) => [card.id, card]));
    return Array.from(selectedIds.value).filter((id) => {
      const card = byId.get(id);
      return !!card && isSelectableCard(card);
    });
  };

  const selectedCount = computed(() => selectedIds.value.size);

  return {
    isSelectionMode,
    selectedIds,
    enterSelectionMode,
    exitSelectionMode,
    toggleSelection,
    selectAll,
    collectSelectableIds,
    selectedCount,
  };
};
