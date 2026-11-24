import { ref, computed } from "vue";
import type { GalleryCard } from "./useDownloads";

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

  const toggleSelection = (id: string) => {
    if (selectedIds.value.has(id)) {
      selectedIds.value.delete(id);
    } else {
      selectedIds.value.add(id);
    }
  };

  const selectAll = (filteredCards: GalleryCard[]) => {
    // Only selectable cards (not running/cancelling/queued)
    const selectable = filteredCards.filter((c) => !["running", "queued", "cancelling"].includes(c.status));
    const allSelected = selectable.every((c) => selectedIds.value.has(c.id));

    if (allSelected) {
      selectedIds.value.clear();
    } else {
      selectable.forEach((c) => selectedIds.value.add(c.id));
    }
  };

  const selectedCount = computed(() => selectedIds.value.size);

  return {
    isSelectionMode,
    selectedIds,
    enterSelectionMode,
    exitSelectionMode,
    toggleSelection,
    selectAll,
    selectedCount,
  };
};
