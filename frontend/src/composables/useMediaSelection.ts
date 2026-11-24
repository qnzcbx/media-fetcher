import { ref } from "vue";
import { getUniqueVideoStreams, getBestAudioStreamId } from "../utils/media";

export const useMediaSelection = () => {
  const selectedVideoID = ref(-1);
  const selectedVideoCodec = ref("");
  const selectedAudioID = ref(-1);
  const selectedImages = ref<number[]>([]);

  const initSelection = (mediaInfo: any) => {
    selectedVideoID.value = -1;
    selectedVideoCodec.value = "";
    selectedAudioID.value = -1;
    selectedImages.value = [];

    if (!mediaInfo) return;

    const uniques = getUniqueVideoStreams(mediaInfo.videoStreams || []);
    if (uniques.length > 0) {
      selectedVideoID.value = uniques[0].stream.id;
      selectedVideoCodec.value = uniques[0].stream.codec;
    }

    if (mediaInfo.audioStreams && mediaInfo.audioStreams.length > 0) {
      selectedAudioID.value = getBestAudioStreamId(mediaInfo.audioStreams);
    }

    if (mediaInfo.images && mediaInfo.images.length > 0) {
      selectedImages.value = mediaInfo.images.map((img: any) => img.index);
    }
  };

  const toggleImage = (idx: number) => {
    if (selectedImages.value.includes(idx)) {
      selectedImages.value = selectedImages.value.filter((i) => i !== idx);
    } else {
      selectedImages.value.push(idx);
    }
  };

  const toggleAllImages = (totalImages: any[]) => {
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
    selectedAudioID,
    selectedImages,
    initSelection,
    toggleImage,
    toggleAllImages,
  };
};
