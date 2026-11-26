import { computed, onMounted, onUnmounted, ref } from "vue";
import { downloadAPI } from "../api/download";
import type { DownloadParams, DownloadRecord, MediaInfoResponse, ProgressInfo } from "../mygo";
import { isActiveStatus, isProcessingStage } from "../utils/downloadStatus";

type Request = Omit<DownloadParams, "TaskID">;
export type GalleryCard = DownloadRecord & { preview?: string; progress?: ProgressInfo; fileState?: string; canRecycle?: boolean; retry?: Request };

export function createDownloads(api = downloadAPI) {
  const cards = ref<GalleryCard[]>([]);
  const error = ref("");
  const removing = ref(false);
  const recycleConfirmation = ref<{ ids: string[]; names: string[]; skipped: number }>();
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
      void refreshFileStates().catch(() => {});
      return;
    }

    const card = task.cards.find((item) => item.id === task.id + "-" + event.itemId);
    if (!card || card.status === "done" || card.status === "failed") return;
    if (event.stage === "done" && event.record) {
      Object.assign(card, event.record, { retry: undefined, progress: undefined,
        preview: event.record.thumbnail ? undefined : card.preview });
      if (event.message) error.value = event.message;
    } else if (event.stage === "item-error") {
      if (event.message) error.value = event.message;
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
      params = { ...params, OutputDir: await api.validateDirectory(params.OutputDir) };
      for (const card of task.cards) if (card.retry) card.retry.OutputDir = params.OutputDir;
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

  let stateGeneration = 0;
  const refreshFileStates = async () => {
    if (!api.fileStates) return;
    const generation = ++stateGeneration;
    const result = await api.fileStates(cards.value.filter(card => card.status === "done").map(card => card.id));
    if (generation !== stateGeneration) return;
    for (const state of result ?? []) {
      const card = cards.value.find(card => card.id === state.id);
      if (card) { card.fileState = state.status; card.canRecycle = state.canRecycle; }
    }
  };
  const recycleMultiple = async (ids: string[]) => {
    if (removing.value) return;
    await refreshFileStates();
    const targets = cards.value.filter(card => ids.includes(card.id) && card.status === "done" && card.canRecycle);
    if (!targets.length) { error.value = "没有可安全删除的文件"; return; }
    const skipped = ids.length - targets.length;
    recycleConfirmation.value = { ids: targets.map(card => card.id), names: targets.map(card => card.finalPath.split(/[\\/]/).pop() || card.title), skipped };
  };
  const confirmRecycle = async () => {
    const confirmation = recycleConfirmation.value;
    if (!confirmation || removing.value) return;
    recycleConfirmation.value = undefined;
    removing.value = true;
    ++stateGeneration;
    try {
      const results = await api.recycle(confirmation.ids);
      const removed = results.filter(item => item.status === "recycled").map(item => item.id);
      cards.value = cards.value.filter(card => !removed.includes(card.id));
      error.value = results.filter(item => item.status !== "recycled").map(item => item.message).filter(Boolean).join("；");
    } catch { error.value = "文件操作失败，请重试"; }
    finally { removing.value = false; await refreshFileStates(); }
  };

  const loadHistory = async () => {
    try {
      const history = await api.history();
      const known = new Set(cards.value.map((card) => card.id));
      cards.value = [...cards.value, ...(history ?? []).filter((card) => !known.has(card.id))];
      await refreshFileStates();
    } catch {
      error.value = "无法读取下载历史";
    }
  };

  const removeMultiple = async (ids: string[]) => {
    if (removing.value) return;
    // 只处理当前存在且允许移除的记录，活动任务跳过。
    const targets = cards.value.filter((card) => ids.includes(card.id) && !isActiveStatus(card.status));
    if (!targets.length) return;
    removing.value = true;
    error.value = "";
    const removed: string[] = [];
    try {
      for (const card of targets) {
        try {
          await api.remove(card.id);
          removed.push(card.id);
        } catch {
          // 单条失败保留记录，等待用户重试。
        }
      }
      // 仅移除真正成功的记录，避免重启后失败记录重新出现。
      cards.value = cards.value.filter((card) => !removed.includes(card.id));
      if (removed.length < targets.length) {
        error.value = "部分记录移除失败，请重试";
      }
    } finally {
      removing.value = false;
    }
  };

  const act = async (card: GalleryCard, action: string) => {
    try {
      error.value = "";
      if (action === "file" || action === "folder") await refreshFileStates();
      if (action === "retry" && card.retry) {
        if (await start(card.retry, [card])) cards.value = cards.value.filter((item) => item.id !== card.id);
      } else if (action === "recycle") {
        await recycleMultiple([card.id]);
      } else if (action === "remove") {
        await removeMultiple([card.id]);
      } else if (action === "copy") {
        await navigator.clipboard.writeText(card.sourceUrl);
      } else {
        await api.open(card.id, action);
      }
    } catch {
      error.value = "操作失败，请检查下载记录或本地文件";
    }
  };

  return { recycleConfirmation, confirmRecycle, refreshFileStates, recycleMultiple, cards, error, isDownloading, isRemoving: removing, handleProgress, doDownload, doCancel, loadHistory, act, removeMultiple };
}

export const progressPercent = (progress?: ProgressInfo) =>
  progress && !isProcessingStage(progress.stage) && progress.total > 0
    ? Math.floor(Math.max(0, Math.min(1, progress.downloaded / progress.total)) * 100)
    : undefined;

export const useDownloads = () => {
  const state = createDownloads();
  let unsubscribe: (() => void) | undefined;
  let lastFocus = 0;
  const focus = () => { if (Date.now() - lastFocus < 2000) return; lastFocus = Date.now(); void state.refreshFileStates().catch(() => {}); };
  onMounted(() => {
    window.addEventListener("focus", focus);
    unsubscribe = downloadAPI.subscribe(state.handleProgress);
    void state.loadHistory();
  });
  onUnmounted(() => { unsubscribe?.(); window.removeEventListener("focus", focus); });
  return state;
};
