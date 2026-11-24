import { ref } from "vue";
import { fetchPreviewBase64 } from "../api/media";

// 单例预览缓存，避免在组件间重复创建或多次请求
const previewMap = ref<Record<string, string>>({});

export const useMediaPreview = () => {
  const getPreview = async (imgUrl: string, siteId: string) => {
    if (previewMap.value[imgUrl]) return; // 已有缓存
    try {
      // 占位状态可以存储为特殊字符串如 'loading'，但为了简单，直接通过没值判断
      const b64 = await fetchPreviewBase64(imgUrl, siteId);
      previewMap.value[imgUrl] = b64;
    } catch (e) {
      console.error("Failed to fetch preview for", imgUrl, e);
      previewMap.value[imgUrl] = "error"; // 简单标记失败
    }
  };

  const loadPreviews = async (media: any) => {
    if (!media) return;
    const siteId = media.siteId || "";
    if (media.coverUrl) {
      getPreview(media.coverUrl, siteId);
    }
    if (media.images) {
      for (const img of media.images) {
        getPreview(img.url, siteId);
      }
    }
  };

  return {
    previewMap,
    loadPreviews,
    getPreview,
  };
};
