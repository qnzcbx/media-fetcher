import { ref } from "vue";
import { fetchPreviewBase64 } from "../api/media";
import type { MediaInfoResponse } from "../mygo";

export const useMediaPreview = () => {
  const previewMap = ref<Record<string, string>>({});
  let generation = 0;

  const loadPreviews = async (media: MediaInfoResponse) => {
    const current = ++generation;
    previewMap.value = {};
    const urls = new Set([media.coverUrl, ...(media.images ?? []).map((item) => item.url)]);
    await Promise.all([...urls].filter(Boolean).map(async (url) => {
      try {
        const preview = await fetchPreviewBase64(url, media.siteId);
        if (generation === current) previewMap.value[url] = preview;
      } catch {
        if (generation === current) previewMap.value[url] = "error";
      }
    }));
  };

  return { previewMap, loadPreviews };
};
