<script setup lang="ts">
import { ref, computed } from "vue";
import { Music, X } from "@lucide/vue";
import VideoQualityGrid from "./VideoQualityGrid.vue";
import AudioSelector from "./AudioSelector.vue";
import ImageSelectionGrid from "./ImageSelectionGrid.vue";

const props = defineProps<{
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
  showUnlockCard?: boolean;
  parseError?: string;
  downloadBusy?: boolean;
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "startDownload", mode: string, format: string): void;

  (e: "update:selectedVideo", id: number, codec: string): void;
  (e: "update:selectedAudio", id: number): void;
  (e: "toggleImage", idx: number): void;
  (e: "toggleAllImages", totalImages: any[]): void;
  (e: "unlock"): void;
}>();
const mode = ref("complete");
const format = ref("m4a");
const available = computed(() => [
  { id: "complete", label: "完整视频", enabled: props.selectedVideoID >= 0 && props.selectedAudioID >= 0 },
  { id: "audio", label: "仅音频", enabled: props.selectedAudioID >= 0 },
  { id: "video", label: "仅视频", enabled: props.selectedVideoID >= 0 },
]);
const chosen = computed(() => available.value.find(item => item.id === mode.value && item.enabled)
  ?? available.value.find(item => item.enabled));
const images = computed(() => !!props.mediaInfo.images?.length);
const cover = computed(() => {
  const value = props.previewMap[props.mediaInfo.coverUrl];
  return value?.startsWith("data:image/") ? value : "";
});
</script>

<template>
  <div class="modal-overlay">
    <div class="resource-modal">
      <div class="rmodal-header">
        <h3>选择资源</h3>
        <button class="icon-btn" @click="$emit('close')"><X class="icon" /></button>
      </div>
      <div class="rmodal-body">
        <p v-if="parseError" class="error-msg" role="alert">{{ parseError }}</p>
        <!-- Videos -->
        <div v-if="!images">
          <div class="mode-picker" role="group" aria-label="下载模式">
            <button v-for="item in available" :key="item.id" :disabled="!item.enabled" :aria-pressed="chosen?.id === item.id" @click="mode = item.id">{{ item.label }}</button>
          </div>
          <div v-if="chosen?.id === 'audio'" class="audio-cover">
            <img v-if="cover" :src="cover" alt="" />
            <Music v-else :size="24" />
            <span>音频资源</span>
          </div>
          <VideoQualityGrid
            v-if="chosen?.id !== 'audio'"
            :videoStreams="uniqueVideoStreams"
            :previewMap="previewMap"
            :coverUrl="mediaInfo.coverUrl"
            :selectedVideoID="selectedVideoID"
            :selectedVideoCodec="selectedVideoCodec"
            :showUnlockCard="showUnlockCard"
            @select="(id, codec) => $emit('update:selectedVideo', id, codec)"
            @unlock="$emit('unlock')"
          />

          <AudioSelector
            v-if="chosen?.id !== 'video' && selectedAudioID >= 0"
            :audioStreams="mediaInfo.audioStreams"
            :modelValue="selectedAudioID"
            @update:modelValue="(val) => $emit('update:selectedAudio', val)"
          />
          <div v-if="chosen?.id === 'audio'" class="mode-picker" role="group" aria-label="输出格式">
            <span>输出格式</span>
            <button title="优先保留 AAC 音轨" :aria-pressed="format === 'm4a'" @click="format = 'm4a'">M4A</button>
            <button title="通过 FFmpeg 转码为 MP3" :aria-pressed="format === 'mp3'" @click="format = 'mp3'">MP3</button>
          </div>
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
          @click="emit('startDownload', images ? 'images' : chosen?.id ?? '', !images && chosen?.id === 'audio' ? format : '')"
          :disabled="downloadBusy || (images ? !selectedImages.length : !chosen)"
        >
          {{ !images && chosen?.id === "audio" ? "下载 " + format.toUpperCase() : "下载" }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.mode-picker { display:flex; align-items:center; gap:8px; margin:0 0 14px; }
.mode-picker button { border:1px solid var(--border); color:inherit; background:var(--input-bg); border-radius:8px; padding:6px 13px; cursor:pointer; }
.mode-picker button[aria-pressed="true"] { background:var(--primary); color:#202427; }
.mode-picker button:disabled { opacity:.45; cursor:default; }
.audio-cover { display:flex; align-items:center; gap:14px; margin-bottom:16px; }
.audio-cover img { width:75px; height:75px; object-fit:cover; border-radius:10px; }

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
