import { computed, onMounted, onUnmounted, ref } from "vue";
import { downloadAPI } from "../api/download";
import type { DownloadParams, DownloadRecord, MediaInfoResponse, ProgressInfo } from "../mygo";

type Request = Omit<DownloadParams, "TaskID">;
export type GalleryCard = DownloadRecord & { preview?: string; progress?: ProgressInfo; retry?: Request };

export function createDownloads(api = downloadAPI) {
  const cards = ref<GalleryCard[]>([]);
  const error = ref("");
  const active = ref<{ id: string; accepted: boolean; cancelling: boolean; terminal: boolean; cards: GalleryCard[] }>();
  const isDownloading = computed(() => !!active.value);

  const handleProgress = (event: ProgressInfo) => {
    const task = active.value;
    if (!task || task.terminal || event.taskId !== task.id) return;

    if (!event.itemId) {
      task.terminal = true;
      for (const card of task.cards) {
        if (card.status === "done" || card.status === "failed") continue;
        card.status = event.stage === "cancelled" ? "cancelled" : "failed";
      }
      if (event.message) error.value = event.message;
      if (task.accepted) active.value = undefined;
      return;
    }

    const card = task.cards.find((item) => item.id === task.id + "-" + event.itemId);
    if (!card || card.status === "done" || card.status === "failed") return;
    if (event.stage === "done" && event.record) {
      Object.assign(card, event.record, { retry: undefined, progress: undefined,
        preview: event.record.thumbnail ? undefined : card.preview });
      if (event.message) error.value = event.message;
    } else if (event.stage === "item-error") {
      card.status = "failed";
    } else {
      card.progress = event;
      if (!task.cancelling) card.status = "running";
    }
  };

  const start = async (params: Request, seeds: Pick<GalleryCard, "kind" | "title" | "siteId" | "preview" | "resourceId">[]) => {
    if (active.value || !seeds.length) return false;
    error.value = "";
    const id = Array.from(crypto.getRandomValues(new Uint8Array(16)), (byte) => byte.toString(16).padStart(2, "0")).join("");
    const createdAt = new Date().toISOString();
    active.value = { id, accepted: false, terminal: false, cancelling: false, cards: seeds.map((seed) => ({
      ...seed, id: id + "-" + (seed.kind === "image" ? seed.resourceId : "media"),
      taskId: id, sourceUrl: "", finalPath: "", thumbnail: "", createdAt, completedAt: "",
      status: "queued", codec: params.VideoCodec,
      retry: { ...params, SelectedImage: seed.kind === "image" ? [seed.resourceId] : [] },
    })) };
    const task = active.value;
    try {
      await api.start({ ...params, TaskID: id });
      task.accepted = true;
      cards.value = [...task.cards, ...cards.value];
      if (task.terminal) active.value = undefined;
      return true;
    } catch (cause) {
      active.value = undefined;
      error.value = cause instanceof Error ? cause.message : "无法启动下载，请重试";
      return false;
    }
  };

  const doDownload = (params: Request, media: MediaInfoResponse, previews: Record<string, string>) => {
    const shared = { siteId: media.siteId, title: media.title };
    return start(params, media.images?.length
      ? media.images.filter((image) => params.SelectedImage.includes(image.index)).map((image) => ({
          ...shared, kind: "image", resourceId: image.index, preview: previews[image.url],
        }))
      : [{ ...shared, kind: params.VideoID < 0 ? "audio" : "video",
          resourceId: params.VideoID < 0 ? params.AudioID : params.VideoID, preview: previews[media.coverUrl] }]);
  };

  const doCancel = async () => {
    const task = active.value;
    if (!task?.accepted || task.cancelling) return;
    task.cancelling = true;
    for (const card of task.cards) {
      if (card.status === "running" || card.status === "queued") card.status = "cancelling";
    }
    try {
      await api.cancel();
    } catch {
      if (active.value === task) {
        task.cancelling = false;
        for (const card of task.cards) if (card.status === "cancelling") card.status = card.progress ? "running" : "queued";
        error.value = "无法取消下载，请重试";
      }
    }
  };

  const loadHistory = async () => {
    try {
      const history = await api.history();
      const known = new Set(cards.value.map((card) => card.id));
      cards.value = [...cards.value, ...(history ?? []).filter((card) => !known.has(card.id))];
    } catch {
      error.value = "无法读取下载历史";
    }
  };

  const removeMultiple = async (ids: string[]) => {
    try {
      error.value = "";
      let hasError = false;
      for (const id of ids) {
        try {
          await api.remove(id);
        } catch {
          hasError = true;
        }
      }
      cards.value = cards.value.filter((item) => !ids.includes(item.id));
      if (hasError) {
        error.value = "部分记录移除失败，请重试";
      }
    } catch {
      error.value = "操作失败，请检查下载记录";
    }
  };

  const act = async (card: GalleryCard, action: string) => {
    try {
      error.value = "";
      if (action === "retry" && card.retry) {
        if (await start(card.retry, [card])) cards.value = cards.value.filter((item) => item.id !== card.id);
      } else if (action === "remove") {
        await api.remove(card.id);
        cards.value = cards.value.filter((item) => item.id !== card.id);
      } else if (action === "copy") {
        await navigator.clipboard.writeText(card.sourceUrl);
      } else {
        await api.open(card.id, action);
      }
    } catch {
      error.value = "操作失败，请检查下载记录或本地文件";
    }
  };

  return { cards, error, isDownloading, handleProgress, doDownload, doCancel, loadHistory, act, removeMultiple };
}

export const progressPercent = (progress?: ProgressInfo) =>
  progress && !["muxing","remuxing","transcoding"].includes(progress.stage) && progress.total > 0
    ? Math.floor(Math.max(0, Math.min(1, progress.downloaded / progress.total)) * 100)
    : undefined;

export const useDownloads = () => {
  const state = createDownloads();
  let unsubscribe: (() => void) | undefined;
  onMounted(() => {
    unsubscribe = downloadAPI.subscribe(state.handleProgress);
    void state.loadHistory();
  });
  onUnmounted(() => unsubscribe?.());
  return state;
};
