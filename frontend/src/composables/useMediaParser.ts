import type { MediaInfoResponse } from "../mygo";
import { shallowRef, ref } from "vue";
import { parseErrorMessage } from "../utils/parseError";
import { extractMedia } from "../api/media";

export const useMediaParser = () => {
  const isParsing = ref(false);
  const parseError = ref("");
  const episodeError = ref("");
  const selectedEpisodeURL = ref("");
  let generation = 0;
  const mediaInfo = shallowRef<MediaInfoResponse | null>(null);

  const parse = async (parsedUrl: string, accountId: string, canApply = () => true, episode = false) => {
    const current = ++generation;
    const valid = () => current === generation && canApply();
    isParsing.value = true;
    if (episode) episodeError.value = "";
    else { parseError.value = ""; episodeError.value = ""; selectedEpisodeURL.value = ""; }

    try {
      const res = await extractMedia(parsedUrl, accountId);
      if (!valid()) return null;
      mediaInfo.value = res;
      return res;
    } catch (e: any) {
      if (valid()) (episode ? episodeError : parseError).value = parseErrorMessage(e);
      return null;
    } finally {
      if (current === generation) isParsing.value = false;
    }
  };

  const parseEpisode = async (url: string) => {
    const original = mediaInfo.value;
    if (!original || !original.episodes?.some((ep) => ep.url === url)) return null;
    selectedEpisodeURL.value = url;
    // 弹窗保留季度和账号快照，上一集的可下载资源立即失效。
    mediaInfo.value = { ...original, videoStreams: [], audioStreams: [], images: [], episodeTitle: "", sourceUrl: url };
    return parse(url, original.usedAccountId, () => true, true);
  };

  const clearMediaInfo = () => {
    generation++;
    isParsing.value = false;
    episodeError.value = "";
    selectedEpisodeURL.value = "";
    parseError.value = "";
    mediaInfo.value = null;
  };

  return {
    episodeError, selectedEpisodeURL, parseEpisode,
    isParsing,
    parseError,
    mediaInfo,
    parse,
    clearMediaInfo,
  };
};
