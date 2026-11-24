<script setup lang="ts">
import { CheckCircle } from "@lucide/vue";

const props = defineProps<{
  images: any[];
  previewMap: Record<string, string>;
  selectedImages: number[];
}>();

const emit = defineEmits<{
  (e: "toggle", idx: number): void;
  (e: "toggleAll"): void;
}>();

const isAllSelected = () => {
  return props.selectedImages.length === props.images.length;
};
</script>

<template>
  <div class="image-selection-wrapper">
    <div class="image-header">
      <label class="custom-checkbox" @click.prevent="$emit('toggleAll')">
        <input type="checkbox" :checked="isAllSelected()" />
        <span>全部 ({{ selectedImages.length }}/{{ images.length }})</span>
      </label>
    </div>
    <div class="video-grid">
      <div
        v-for="img in images"
        :key="img.index"
        class="video-card"
        :class="{ active: selectedImages.includes(img.index) }"
        @click="$emit('toggle', img.index)"
      >
        <div class="video-cover image-cover">
          <img v-if="previewMap[img.url]" :src="previewMap[img.url]" />
          <div v-else class="preview-placeholder">加载中...</div>
          <div class="check" v-if="selectedImages.includes(img.index)">
            <CheckCircle :size="20" />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.image-header {
  margin-bottom: 16px;
}

.custom-checkbox {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  user-select: none;
}

.video-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
  gap: 16px;
  margin-bottom: 24px;
}

.video-card {
  background: var(--card-bg);
  border-radius: 8px;
  border: 2px solid transparent;
  cursor: pointer;
  transition: all 0.2s;
  overflow: hidden;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.05);
}

.video-card.active {
  border-color: var(--primary);
}

.video-cover {
  width: 100%;
  aspect-ratio: 1;
  background: var(--input-bg);
  position: relative;
}

.video-cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.preview-placeholder {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-sub);
  font-size: 12px;
}

.check {
  position: absolute;
  top: 8px;
  right: 8px;
  color: var(--primary);
  background: #fff;
  border-radius: 50%;
  display: flex;
}
</style>
