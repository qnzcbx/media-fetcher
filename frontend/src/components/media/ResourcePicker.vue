<script setup lang="ts">
import { ref, computed } from "vue";
import { X } from "@lucide/vue";
import VideoQualityGrid from "./VideoQualityGrid.vue";
import AudioQualityGrid from "./AudioQualityGrid.vue";
import AudioSelector from "./AudioSelector.vue";
import AudioFormatSelector from "./AudioFormatSelector.vue";
import DownloadModeSelector from "./DownloadModeSelector.vue";
import ImageSelectionGrid from "./ImageSelectionGrid.vue";
import type { Episode, ImageItem, MediaInfoResponse } from "../../mygo";
import type { UniqueVideoStream } from "../../utils/media";
import type { AudioFormat } from "../../utils/preferences";

type DownloadMode = { id: string; label: string; enabled: boolean };

const props = defineProps<{
  mediaInfo: MediaInfoResponse;
  episodes?: Episode[];
  episodeBusy?: boolean;
  previewMap: Record<string, string>;

  // 选择状态
  selectedVideoID: number;
  selectedVideoCodec: string;
  selectedAudioID: number;
  selectedImages: number[];

  // 已按画质去重的视频流
  uniqueVideoStreams: UniqueVideoStream[];
  showUnlockCard?: boolean;
  parseError?: string;
  downloadBusy?: boolean;
  // 设置中的默认音频格式只影响新打开的选择器，单次修改不回写偏好。
  defaultAudioFormat?: AudioFormat;
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "selectEpisode", url: string): void;
  (e: "startDownload", mode: string, format: string): void;

  (e: "update:selectedVideo", id: number, codec: string): void;
  (e: "update:selectedAudio", id: number): void;
  (e: "toggleImage", idx: number): void;
  (e: "toggleAllImages", totalImages: ImageItem[]): void;
  (e: "unlock"): void;
}>();

const mode = ref("complete");
const format = ref<AudioFormat>(props.defaultAudioFormat ?? "m4a");

const available = computed<DownloadMode[]>(() => [
  { id: "complete", label: "完整视频", enabled: props.selectedVideoID >= 0 && props.selectedAudioID >= 0 },
  { id: "audio", label: "仅音频", enabled: props.selectedAudioID >= 0 },
  { id: "video", label: "仅视频", enabled: props.selectedVideoID >= 0 },
]);
const chosen = computed(
  () =>
    available.value.find((item) => item.id === mode.value && item.enabled) ??
    available.value.find((item) => item.enabled),
);

// 只有图片资源的站点（LOFTER）不展示下载模式与音频选项。
const isImages = computed(
  () =>
    !!props.mediaInfo.images?.length &&
    !props.mediaInfo.videoStreams?.length &&
    !props.mediaInfo.audioStreams?.length,
);
const cover = computed(() => {
  const value = props.previewMap[props.mediaInfo.coverUrl];
  return value?.startsWith("data:image/") ? value : "";
});

const downloadLabel = computed(() => {
  if (isImages.value) return "下载";
  switch (chosen.value?.id) {
    case "audio":
      return format.value === "mp3" ? "下载 MP3" : "下载 M4A";
    default:
      return "下载 MP4";
  }
});

const canDownload = computed(() =>
  !props.episodeBusy && (isImages.value ? props.selectedImages.length > 0 : !!chosen.value),
);

const startDownload = () => {
  const current = isImages.value ? "images" : (chosen.value?.id ?? "");
  emit("startDownload", current, current === "audio" ? format.value : "");
};
</script>

<template>
  <div class="modal-overlay">
    <div class="resource-modal" :class="{ 'is-media': !isImages }">
      <div class="rmodal-header">
        <h3>选择资源</h3>
        <button class="icon-btn" aria-label="关闭" @click="emit('close')">
          <X :size="20" />
        </button>
      </div>

      <div class="rmodal-body">
        <p v-if="mediaInfo.episodeTitle" class="episode-title">{{ mediaInfo.title }}</p>
        <p v-if="parseError" class="error-msg" role="alert">{{ parseError }}</p>

        <label v-if="episodes?.length" class="episode-selector">
          <span>剧集</span>
          <select :key="`${mediaInfo.sourceUrl}:${episodeBusy}`" :value="mediaInfo.episodeTitle ? mediaInfo.sourceUrl : ''" :disabled="episodeBusy || downloadBusy"
            @change="emit('selectEpisode', ($event.target as HTMLSelectElement).value)">
            <option value="" disabled>选择剧集</option>
            <option v-for="episode in episodes" :key="episode.id" :value="episode.url">{{ episode.title }}</option>
          </select>
        </label>
        <template v-if="!isImages && (mediaInfo.videoStreams?.length || mediaInfo.audioStreams?.length)">
          <DownloadModeSelector
            class="mode-selector"
            :modelValue="chosen?.id ?? ''"
            :options="available"
            @update:modelValue="mode = $event"
          />

          <div class="resource-content">
            <AudioQualityGrid
              v-if="chosen?.id === 'audio'"
              :audioStreams="mediaInfo.audioStreams"
              :cover="cover"
              :selectedAudioID="selectedAudioID"
              @select="(id) => emit('update:selectedAudio', id)"
            />
            <VideoQualityGrid
              v-else
              :videoStreams="uniqueVideoStreams"
              :previewMap="previewMap"
              :coverUrl="mediaInfo.coverUrl"
              :selectedVideoID="selectedVideoID"
              :selectedVideoCodec="selectedVideoCodec"
              :showUnlockCard="showUnlockCard"
              @select="(id, codec) => emit('update:selectedVideo', id, codec)"
              @unlock="emit('unlock')"
            />
          </div>

          <div v-if="chosen?.id === 'complete' && selectedAudioID >= 0" class="resource-options">
            <AudioSelector
              :audioStreams="mediaInfo.audioStreams"
              :modelValue="selectedAudioID"
              @update:modelValue="(val) => emit('update:selectedAudio', val)"
            />
          </div>
          <div v-else-if="chosen?.id === 'audio'" class="resource-options">
            <AudioFormatSelector v-model="format" />
          </div>
        </template>

        <div v-else-if="isImages" class="resource-content">
          <ImageSelectionGrid
            :images="mediaInfo.images"
            :previewMap="previewMap"
            :selectedImages="selectedImages"
            @toggle="(idx) => emit('toggleImage', idx)"
            @toggleAll="emit('toggleAllImages', mediaInfo.images)"
          />
        </div>
      </div>

      <div class="rmodal-footer">
        <button class="btn-primary download-btn" @click="startDownload" :disabled="downloadBusy || !canDownload">
          {{ downloadLabel }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.episode-title { margin: 0; font-size: 14px; color: var(--text-main); flex-shrink: 0; }
.episode-selector { display: flex; align-items: center; gap: 12px; flex-shrink: 0; }
.episode-selector select { flex: 1; min-width: 0; padding: 8px; border: 1px solid var(--border); border-radius: 8px; background: var(--input-bg); color: var(--text-main); font: inherit; }
.resource-modal {
  background: var(--card-bg);
  border-radius: 16px;
  width: 680px;
  max-width: 90vw;
  max-height: 85vh;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
  box-shadow: 0 8px 24px var(--shadow-color);
}

/* 三种下载模式使用同一固定内容高度，避免切换时弹窗位移。 */
.resource-modal.is-media {
  height: min(640px, calc(100vh - 64px));
}

.rmodal-header {
  flex-shrink: 0;
  padding: 18px 24px;
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
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 18px 24px;
}

.mode-selector {
  flex-shrink: 0;
}

/* 唯一滚动容器，滚动条出现时不再推动卡片布局。 */
.resource-content {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  scrollbar-gutter: stable;
  padding-right: 4px;
}

.resource-options {
  flex-shrink: 0;
}

.rmodal-footer {
  flex-shrink: 0;
  padding: 16px 24px;
  border-top: 1px solid var(--border);
}

.download-btn {
  width: 100%;
  justify-content: center;
}

.error-msg {
  flex-shrink: 0;
  margin: 0;
  color: var(--error-text);
  background: var(--error-bg);
  padding: 10px 12px;
  border-radius: 8px;
  font-size: 13px;
}

.icon-btn {
  background: none;
  border: none;
  display: flex;
  align-items: center;
  color: var(--text-main);
  cursor: pointer;
  padding: 6px 10px;
  border-radius: 6px;
  transition: background 0.2s;
}

.icon-btn:hover {
  background: var(--input-bg);
}
</style>
