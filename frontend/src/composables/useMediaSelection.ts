import { ref } from "vue";
import type { ImageItem, MediaInfoResponse } from "../mygo";
import { audioKey, getUniqueVideoStreams, getBestAudioStream } from "../utils/media";

export const useMediaSelection = () => {
  const selectedVideoID = ref(-1);
  const selectedVideoCodec = ref("");
  const selectedVideoGroupKey = ref("");
  const selectedVideoKey = ref("");
  const selectedAudioID = ref(-1);
  const selectedAudioKey = ref("");
  const selectedImages = ref<number[]>([]);

  // 初始化真实可用的默认选择：最高兼容画质、最高带宽兼容音轨、全部图片。
  const initSelection = (mediaInfo: MediaInfoResponse) => {
    selectedVideoID.value = -1;
    selectedVideoCodec.value = "";
    selectedVideoGroupKey.value = "";
    selectedVideoKey.value = "";
    selectedAudioID.value = -1;
    selectedAudioKey.value = "";
    selectedImages.value = [];

    if (!mediaInfo) return;

    const uniques = getUniqueVideoStreams(mediaInfo.videoStreams || []);
    if (uniques.length > 0) {
      selectedVideoID.value = uniques[0].stream.id;
      selectedVideoCodec.value = uniques[0].stream.codec;
      selectedVideoGroupKey.value = uniques[0].groupKey;
      selectedVideoKey.value = uniques[0].stream.stream_key ?? "";
    }

    if (mediaInfo.audioStreams && mediaInfo.audioStreams.length > 0) {
      const audio = getBestAudioStream(mediaInfo.audioStreams)!;
      selectedAudioID.value = audio.id;
      selectedAudioKey.value = audioKey(audio);
    }

    if (mediaInfo.images && mediaInfo.images.length > 0) {
      selectedImages.value = mediaInfo.images.map((img) => img.index);
    }
  };

  const toggleImage = (idx: number) => {
    if (selectedImages.value.includes(idx)) {
      selectedImages.value = selectedImages.value.filter((i) => i !== idx);
    } else {
      selectedImages.value.push(idx);
    }
  };

  const toggleAllImages = (totalImages: ImageItem[]) => {
    if (!totalImages) return;
    if (selectedImages.value.length === totalImages.length) {
      selectedImages.value = [];
    } else {
      selectedImages.value = totalImages.map((img) => img.index);
    }
  };

  return {
    selectedVideoID,
    selectedVideoCodec,
    selectedVideoGroupKey,
    selectedVideoKey,
    selectedAudioID,
    selectedAudioKey,
    selectedImages,
    initSelection,
    toggleImage,
    toggleAllImages,
  };
};
