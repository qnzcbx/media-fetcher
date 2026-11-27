<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { PopoverRoot, PopoverTrigger, PopoverPortal, PopoverContent, PopoverClose, SelectRoot, SelectTrigger, SelectValue, SelectPortal, SelectContent, SelectViewport, SelectItem, SelectItemText } from "reka-ui";
import { X, RotateCcw, SlidersHorizontal, ChevronUp, ChevronDown, LoaderCircle } from "@lucide/vue";
import VideoQualityGrid from "./VideoQualityGrid.vue";
import AudioQualityGrid from "./AudioQualityGrid.vue";
import AudioSelector from "./AudioSelector.vue";
import AudioFormatSelector from "./AudioFormatSelector.vue";
import DownloadModeSelector from "./DownloadModeSelector.vue";
import ImageSelectionGrid from "./ImageSelectionGrid.vue";
import type { Episode, ImageItem, MediaInfoResponse } from "../../mygo";
import { codecLabel, audioKey, getBestAudioStream, resolveVideoContainer, videoStreamKey } from "../../utils/media";
import type { UniqueVideoStream } from "../../utils/media";
import type { AudioFormat } from "../../utils/preferences";

type DownloadMode = { id: string; label: string; enabled: boolean };

const props = defineProps<{
  mediaInfo: MediaInfoResponse;
  episodes?: Episode[];
  selectedEpisodeURL?: string;
  episodeBusy?: boolean;
  previewMap: Record<string, string>;

  // 选择状态
  selectedVideoID: number;
  selectedVideoCodec: string;
  selectedVideoGroupKey?: string;
  selectedVideoKey?: string;
  selectedAudioID: number;
  selectedAudioKey: string;
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
  (e: "retryEpisode"): void;
  (e: "selectEpisode", url: string): void;
  (e: "startDownload", mode: string, format: string): void;

  (e: "update:selectedVideo", id: number, codec: string, groupKey: string, streamKey: string): void;
  (e: "update:selectedAudio", id: number, key: string): void;
  (e: "toggleImage", idx: number): void;
  (e: "toggleAllImages", totalImages: ImageItem[]): void;
  (e: "unlock"): void;
}>();

const mode = ref("complete");
const advancedOpen = ref(false);
watch(() => [mode.value, props.mediaInfo], () => advancedOpen.value = false);
const resetDefaults = () => {
  const group = currentGroup.value;
  const video = codecs.value[0];
  if (video && group) emit("update:selectedVideo", video.id, video.codec, group.groupKey, video.stream_key ?? "");
  const audio = getBestAudioStream(props.mediaInfo.audioStreams ?? []);
  if (audio) emit("update:selectedAudio", audio.id, audioKey(audio));
};
const format = ref<AudioFormat>(props.defaultAudioFormat ?? "m4a");

const currentGroup = computed(() =>
  props.uniqueVideoStreams.find((group) =>
    props.selectedVideoGroupKey ? group.groupKey === props.selectedVideoGroupKey : group.stream.id === props.selectedVideoID,
  ),
);
const codecs = computed(() => currentGroup.value?.streams ?? []);
const isVideoSelected = (stream: UniqueVideoStream["stream"]) =>
  props.selectedVideoKey ? stream.stream_key === props.selectedVideoKey : props.selectedVideoCodec === stream.codec;
const isDefault = computed(() => {
  const audio = getBestAudioStream(props.mediaInfo.audioStreams ?? []);
  const firstVideo = codecs.value[0];
  return !!firstVideo && isVideoSelected(firstVideo) && !!audio && props.selectedAudioKey === audioKey(audio);
});
const selectedAudio = computed(() => props.mediaInfo.audioStreams?.find((audio) => audioKey(audio) === props.selectedAudioKey));
const available = computed<DownloadMode[]>(() => [
  { id: "complete", label: "完整视频", enabled: (props.selectedVideoID >= 0 || !!props.selectedVideoKey) && (props.selectedAudioID >= 0 || !!props.selectedAudioKey) },
  { id: "audio", label: "仅音频", enabled: props.selectedAudioID >= 0 || !!props.selectedAudioKey },
  { id: "video", label: "仅视频", enabled: props.selectedVideoID >= 0 || !!props.selectedVideoKey },
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

const videoContainer = computed(() =>
  resolveVideoContainer(props.selectedVideoCodec, chosen.value?.id === "complete" ? selectedAudio.value?.codec : undefined),
);
const videoContainerLabel = computed(() =>
  videoContainer.value === "webm" ? "WebM" : videoContainer.value === "mkv" ? "MKV" : "MP4",
);

const downloadLabel = computed(() => {
  if (isImages.value) return "下载";
  switch (chosen.value?.id) {
    case "audio":
      return format.value === "original" ? "下载原始音频" : format.value === "mp3" ? "下载 MP3" : "下载 M4A";
    default:
      return `下载 ${videoContainerLabel.value}`;
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
    <div class="resource-modal" :class="{ 'is-media': !isImages, 'is-pgc': !!episodes?.length || !!mediaInfo.seasonTitle }">
      <div class="rmodal-header">
        <h3>选择资源</h3>
        <button class="icon-btn" aria-label="关闭" @click="emit('close')">
          <X :size="20" />
        </button>
      </div>

      <div class="rmodal-body">
        <p v-if="mediaInfo.episodeTitle" class="episode-title">{{ mediaInfo.title }}</p>
        <p v-if="parseError" class="error-msg" role="alert">{{ parseError }}</p>

        <div v-if="episodes?.length" class="episode-selector">
          <span>剧集</span>
          <SelectRoot :modelValue="selectedEpisodeURL || undefined" :disabled="downloadBusy"
            @update:modelValue="(value) => value && emit('selectEpisode', String(value))">
            <SelectTrigger class="episode-trigger" aria-label="剧集">
              <SelectValue placeholder="选择剧集" /><ChevronDown :size="16" />
            </SelectTrigger>
            <SelectPortal><SelectContent class="episode-menu" position="popper" :sideOffset="6">
              <SelectViewport class="episode-viewport">
                <SelectItem v-for="episode in episodes" :key="episode.id" :value="episode.url" class="episode-item">
                  <SelectItemText>{{ episode.title }}</SelectItemText>
                </SelectItem>
              </SelectViewport>
            </SelectContent></SelectPortal>
          </SelectRoot>
        </div>
        <div v-if="episodeBusy" class="episode-status" role="status"><LoaderCircle class="episode-spinner" :size="22" aria-label="正在解析" /></div>
        <button v-else-if="parseError && selectedEpisodeURL" class="btn-secondary episode-retry" @click="emit('retryEpisode')">重试</button>
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
              :selectedAudioKey="selectedAudioKey"
              @select="(id, key) => emit('update:selectedAudio', id, key)"
            />
            <VideoQualityGrid
              v-else
              :videoStreams="uniqueVideoStreams"
              :previewMap="previewMap"
              :coverUrl="mediaInfo.coverUrl"
              :selectedVideoID="selectedVideoID"
              :selectedVideoGroupKey="selectedVideoGroupKey"
              :selectedVideoCodec="selectedVideoCodec"
              :showUnlockCard="showUnlockCard"
              @select="(id, codec, groupKey, streamKey) => emit('update:selectedVideo', id, codec, groupKey, streamKey)"
              @unlock="emit('unlock')"
            />
          </div>

          <div v-if="chosen?.id === 'video' && codecs.length" class="codec-options media-parameter-row">
            <span>视频编码</span>
            <div class="codec-switch media-parameter-segments" role="group" aria-label="视频编码">
              <button v-for="stream in codecs" :key="videoStreamKey(stream)" class="segmented-option" :aria-pressed="isVideoSelected(stream)"
                @click="emit('update:selectedVideo', stream.id, stream.codec, currentGroup?.groupKey ?? '', stream.stream_key ?? '')">{{ codecLabel(stream.codec) }}</button>
            </div>
          </div>
          <p v-if="(chosen?.id === 'complete' || chosen?.id === 'video') && videoContainer !== 'mp4'" class="media-notice">当前编码组合将无损封装为 {{ videoContainerLabel }}，不进行重编码。</p>
          <p v-else-if="chosen?.id === 'complete' && selectedAudio?.kind && selectedAudio.kind !== 'standard'" class="media-notice">高级音轨的 MP4 播放兼容性取决于播放器。</p>
          <p v-if="chosen?.id === 'audio' && format === 'm4a' && selectedAudio && !/^(mp4a|aac)/i.test(selectedAudio.codec)" class="media-notice">将重新编码为 AAC。</p>
          <PopoverRoot v-if="chosen?.id === 'complete'" v-model:open="advancedOpen">
            <PopoverTrigger class="advanced-trigger"><span><SlidersHorizontal :size="16" />高级设置</span><span class="advanced-summary">{{ codecLabel(selectedVideoCodec) }} · {{ selectedAudio?.quality }}</span><ChevronUp v-if="advancedOpen" :size="16" /><ChevronDown v-else :size="16" /></PopoverTrigger>
            <PopoverPortal><PopoverContent side="top" align="start" :sideOffset="8" :collisionPadding="12" class="advanced-panel">
              <div class="advanced-heading"><strong>高级设置</strong><div class="advanced-actions"><button class="icon-btn" title="恢复默认" aria-label="恢复默认" :disabled="isDefault" @click="resetDefaults"><RotateCcw :size="18" /></button><PopoverClose class="icon-btn" title="关闭高级设置" aria-label="关闭高级设置"><X :size="18" /></PopoverClose></div></div>
          <div v-if="codecs.length" class="codec-options media-parameter-row">
            <span>视频编码</span>
            <div class="codec-switch media-parameter-segments" role="group" aria-label="视频编码">
              <button v-for="stream in codecs" :key="videoStreamKey(stream)" class="segmented-option" :aria-pressed="isVideoSelected(stream)"
                @click="emit('update:selectedVideo', stream.id, stream.codec, currentGroup?.groupKey ?? '', stream.stream_key ?? '')">{{ codecLabel(stream.codec) }}</button>
            </div>
          </div>
          <div class="resource-options">
            <AudioSelector
              :audioStreams="mediaInfo.audioStreams"
              :modelValue="selectedAudioID"
              :selectedAudioKey="selectedAudioKey"
              @update:modelValue="(val, key) => emit('update:selectedAudio', val, key)"
            />
          </div>

            </PopoverContent></PopoverPortal>
          </PopoverRoot>
          <div v-if="chosen?.id === 'audio'" class="resource-options">
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
.advanced-trigger { display:flex; align-items:center; gap:12px; min-height:44px; flex-shrink:0; padding:0 16px; border:0; border-radius:8px; background:var(--input-bg); color:var(--text-main); font:inherit; cursor:pointer; }
.advanced-trigger > span:first-child { display:flex; align-items:center; gap:8px; white-space:nowrap; }
.advanced-summary { flex:1; text-align:right; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; color:var(--text-assist); font-size:13px; }
:global(.advanced-panel) { z-index:1100; box-sizing:border-box; width:var(--reka-popover-trigger-width); max-width:calc(100vw - 24px); max-height:var(--reka-popover-content-available-height); overflow:auto; padding:16px; display:flex; flex-direction:column; gap:12px; border:1px solid var(--border); border-radius:12px; background:var(--card-bg); color:var(--text-main); box-shadow:0 6px 20px var(--shadow-color); animation:advanced-in 140ms ease-out; }
.advanced-heading { display:flex; justify-content:space-between; align-items:center; }
.advanced-actions { display:flex; align-items:center; gap:4px; }
.advanced-actions .icon-btn { width:30px; height:30px; padding:6px; justify-content:center; }
.advanced-actions .icon-btn:hover:not(:disabled) { background:var(--input-bg); }
.advanced-actions .icon-btn:disabled { opacity:.35; cursor:default; }
@keyframes advanced-in { from { opacity:0; transform:translateY(4px); } to { opacity:1; transform:translateY(0); } }
.media-notice { margin: 0; font-size: 12px; color: var(--text-assist); }
.episode-title { margin: 0; font-size: 14px; color: var(--text-main); flex-shrink: 0; }
.episode-selector { display: flex; align-items: center; gap: 12px; flex-shrink: 0; }
.episode-trigger { display: flex; justify-content: space-between; align-items: center; cursor: pointer; flex: 1; min-width: 0; padding: 8px; border: 1px solid var(--border); border-radius: 8px; background: var(--input-bg); color: var(--text-main); font: inherit; }
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

.resource-modal.is-pgc { height: min(780px, calc(100vh - 40px)); max-height: calc(100vh - 40px); }
.is-pgc .rmodal-body { overflow-y: auto; }
.is-pgc .resource-content { flex: 1 0 185px; overflow: visible; }
.episode-status { display: flex; justify-content: center; padding: 32px; }
.episode-spinner { animation: episode-spin 1s linear infinite; }
@keyframes episode-spin { to { transform: rotate(360deg); } }
.episode-retry { align-self: center; padding: 8px 18px; border: 1px solid var(--border); border-radius: 8px; background: var(--input-bg); color: var(--text-main); font: inherit; cursor: pointer; }
.episode-menu { z-index: 1000; background: var(--card-bg); color: var(--text-main); border: 1px solid var(--border); border-radius: 8px; min-width: var(--reka-select-trigger-width); max-width: 85vw; box-shadow: 0 8px 24px var(--shadow-color); }
.episode-viewport { max-height: min(320px, var(--reka-select-content-available-height)); padding: 4px; overflow-y: auto; }
.episode-item { padding: 9px 12px; border-radius: 5px; cursor: pointer; outline: none; }
.episode-item[data-highlighted], .episode-item[data-state="checked"] { background: var(--input-bg); }
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
