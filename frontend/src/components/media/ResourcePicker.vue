<script setup lang="ts">
import { X } from "@lucide/vue";
import VideoQualityGrid from "./VideoQualityGrid.vue";
import AudioSelector from "./AudioSelector.vue";
import ImageSelectionGrid from "./ImageSelectionGrid.vue";

defineProps<{
  mediaInfo: any;
  previewMap: Record<string, string>;

  // Selection states
  selectedVideoID: number;
  selectedVideoCodec: string;
  selectedAudioID: number;
  selectedImages: number[];

  // computed properties or helpers can be passed or we can compute inside?
  // We'll pass the stream unique list directly
  uniqueVideoStreams: any[];
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "startDownload"): void;

  (e: "update:selectedVideo", id: number, codec: string): void;
  (e: "update:selectedAudio", id: number): void;
  (e: "toggleImage", idx: number): void;
  (e: "toggleAllImages", totalImages: any[]): void;
}>();
</script>

<template>
  <div class="modal-overlay">
    <div class="resource-modal">
      <div class="rmodal-header">
        <h3>选择资源</h3>
        <button class="icon-btn" @click="$emit('close')"><X class="icon" /></button>
      </div>
      <div class="rmodal-body">
        <!-- Videos -->
        <div v-if="mediaInfo.videoStreams && mediaInfo.videoStreams.length > 0">
          <VideoQualityGrid
            :videoStreams="uniqueVideoStreams"
            :previewMap="previewMap"
            :coverUrl="mediaInfo.coverUrl"
            :selectedVideoID="selectedVideoID"
            :selectedVideoCodec="selectedVideoCodec"
            @select="(id, codec) => $emit('update:selectedVideo', id, codec)"
          />

          <AudioSelector
            :audioStreams="mediaInfo.audioStreams"
            :modelValue="selectedAudioID"
            @update:modelValue="(val) => $emit('update:selectedAudio', val)"
          />
        </div>

        <!-- Images -->
        <div
          v-if="
            mediaInfo.images &&
            mediaInfo.images.length > 0 &&
            (!mediaInfo.videoStreams || mediaInfo.videoStreams.length === 0)
          "
        >
          <ImageSelectionGrid
            :images="mediaInfo.images"
            :previewMap="previewMap"
            :selectedImages="selectedImages"
            @toggle="(idx) => $emit('toggleImage', idx)"
            @toggleAll="$emit('toggleAllImages', mediaInfo.images)"
          />
        </div>
      </div>

      <div class="rmodal-footer">
        <button
          class="btn-primary"
          style="width: 100%; justify-content: center"
          @click="$emit('startDownload')"
          :disabled="selectedImages.length === 0 && !mediaInfo.videoStreams?.length"
        >
          下载
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.resource-modal {
  background: var(--card-bg);
  border-radius: 16px;
  width: 680px;
  max-width: 90vw;
  max-height: 85vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1);
}

.rmodal-header {
  padding: 20px 24px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px solid var(--border);
}

.rmodal-header h3 {
  margin: 0;
  font-size: 18px;
}

.rmodal-body {
  padding: 24px;
  overflow-y: auto;
  flex: 1;
}

.rmodal-footer {
  padding: 20px 24px;
  border-top: 1px solid var(--border);
}

.icon-btn {
  background: none;
  border: none;
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--text-main);
  cursor: pointer;
  padding: 6px 12px;
  border-radius: 6px;
  transition: background 0.2s;
}

.icon-btn:hover {
  background: var(--input-bg);
}
</style>
