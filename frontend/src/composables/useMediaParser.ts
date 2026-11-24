import { ref } from "vue";
import { extractMedia } from "../api/media";

export const useMediaParser = () => {
  const isParsing = ref(false);
  const parseError = ref("");
  const mediaInfo = ref<any>(null);

  const parse = async (parsedUrl: string, accountId: string) => {
    isParsing.value = true;
    parseError.value = "";
    mediaInfo.value = null;

    try {
      const res = await extractMedia(parsedUrl, accountId);
      mediaInfo.value = res;
      return res;
    } catch (e: any) {
      parseError.value = e.message || "解析失败";
      return null;
    } finally {
      isParsing.value = false;
    }
  };

  const clearMediaInfo = () => {
    mediaInfo.value = null;
  };

  return {
    isParsing,
    parseError,
    mediaInfo,
    parse,
    clearMediaInfo,
  };
};
