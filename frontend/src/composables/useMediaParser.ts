import type { MediaInfoResponse } from "../mygo";
import { shallowRef, ref } from "vue";
import { extractMedia } from "../api/media";

export const useMediaParser = () => {
  const isParsing = ref(false);
  const parseError = ref("");
  const mediaInfo = shallowRef<MediaInfoResponse | null>(null);

  const parse = async (parsedUrl: string, accountId: string, canApply = () => true) => {
    isParsing.value = true;
    parseError.value = "";

    try {
      const res = await extractMedia(parsedUrl, accountId);
      if (!canApply()) return null;
      mediaInfo.value = res;
      return res;
    } catch (e: any) {
      if (canApply()) parseError.value = e.message || "解析失败";
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
