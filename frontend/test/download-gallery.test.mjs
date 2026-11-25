import assert from "node:assert/strict";
import { createServer } from "vite";

const vite = await createServer({ root: "frontend", server: { middlewareMode: true, ws: false } });
try {
  const { createDownloads, progressPercent } = await vite.ssrLoadModule("/src/composables/useDownloads.ts");
  let latest;
  let refuse = true;
  const api = {
    start: async (params) => {
      latest = params.TaskID;
      if (refuse) throw new Error("拒绝启动");
    },
    cancel: async () => {},
    history: async () => [],
  };
  const state = createDownloads(api);
  const media = { siteId: "lofter", title: "图片", images: [1, 2, 3].map((index) => ({ index, url: String(index) })) };
  const params = { URL: "https://example.lofter.com/post/abc_def", AccountID: "", OutputDir: "", VideoID: -1, AudioID: -1, VideoCodec: "", SelectedImage: [1, 2] };
  const event = (stage, itemId = "", taskId = latest, extra = {}) => ({ taskId, itemId, stage, downloaded: 0, total: -1, message: "", finalPath: "", ...extra });

  assert.equal(await state.doDownload(params, media, {}), false);
  assert.equal(state.isDownloading.value, false);
  assert.equal(state.cards.value.length, 0);
  assert.equal(state.error.value, "拒绝启动");
  refuse = false;

  assert.equal(await state.doDownload(params, media, {}), true);
  const first = latest;
  assert.deepEqual(state.cards.value.map((card) => card.resourceId), [1, 2]);
  state.handleProgress(event("images", "1"));
  assert.equal(progressPercent(state.cards.value[0].progress), undefined);
  assert.equal(progressPercent(event("muxing", "1", latest, { total: 20, downloaded: 20 })), undefined);
  assert.equal(progressPercent(event("images", "1", latest, { total: 20, downloaded: 10 })), 50);
  state.handleProgress(event("done", "1", latest, { record: { ...state.cards.value[0], status: "done", finalPath: "one.png" } }));
  state.handleProgress(event("item-error", "2"));
  state.handleProgress(event("finished"));
  assert.deepEqual(state.cards.value.map((card) => card.status), ["done", "failed"]);

  assert.equal(await state.doDownload({ ...params, SelectedImage: [3] }, media, {}), true);
  state.handleProgress(event("cancelled", "", first));
  assert.equal(state.isDownloading.value, true);
  await state.doCancel();
  assert.equal(state.cards.value[0].status, "cancelling");
  assert.equal(state.isDownloading.value, true);
  state.handleProgress(event("cancelled"));
  assert.equal(state.isDownloading.value, false);
  assert.deepEqual(state.cards.value.map((card) => card.status), ["cancelled", "done", "failed"]);

  let fast;
  fast = createDownloads({ ...api, start: async (request) => {
    fast.handleProgress(event("cancelled", "", request.TaskID));
  } });
  assert.equal(await fast.doDownload({ ...params, SelectedImage: [1] }, media, {}), true);
  assert.equal(fast.isDownloading.value, false);
  assert.equal(fast.cards.value[0].status, "cancelled");

  // 批量移除：部分失败时保留失败记录，并跳过活动任务。
  const removeState = createDownloads({
    start: async () => {},
    cancel: async () => {},
    history: async () => [],
    remove: async (id) => {
      if (id === "task-2") throw new Error("拒绝移除");
    },
  });
  removeState.cards.value = [
    { id: "task-1", status: "done" },
    { id: "task-2", status: "done" },
    { id: "task-3", status: "running" },
  ];
  await removeState.removeMultiple(["task-1", "task-2", "task-3", "missing"]);
  assert.deepEqual(removeState.cards.value.map((card) => card.id), ["task-2", "task-3"]);
  assert.equal(removeState.error.value, "部分记录移除失败，请重试");

  // 批量移除期间重复提交只执行一次。
  let calls = 0;
  const guardState = createDownloads({
    start: async () => {},
    cancel: async () => {},
    history: async () => [],
    remove: async () => {
      calls++;
      await new Promise((resolve) => setTimeout(resolve, 10));
    },
  });
  guardState.cards.value = [{ id: "guard-1", status: "done" }];
  await Promise.all([guardState.removeMultiple(["guard-1"]), guardState.removeMultiple(["guard-1"])]);
  assert.equal(calls, 1);
  assert.equal(guardState.cards.value.length, 0);

  for (const status of ["queued", "running", "cancelling"]) {
    guardState.cards.value = [{ id: "active", status }];
    await guardState.act({ id: "active", status: "done" }, "remove");
    assert.equal(guardState.cards.value.length, 1);
  }
  assert.equal(calls, 1);
  guardState.cards.value = [{ id: "active", status: "done" }];
  await guardState.act(guardState.cards.value[0], "remove");
  assert.equal(calls, 2);
  assert.equal(guardState.cards.value.length, 0);

  // 选择资格：活动任务不能单选，全选也只覆盖可移除记录。
  const { useGallerySelection } = await vite.ssrLoadModule("/src/composables/useGallerySelection.ts");
  const selection = useGallerySelection();
  const doneCard = { id: "select-1", status: "done" };
  const runningCard = { id: "select-2", status: "running" };
  selection.toggleSelection(runningCard);
  assert.equal(selection.selectedIds.value.has("select-2"), false);
  selection.toggleSelection(doneCard);
  assert.equal(selection.selectedIds.value.has("select-1"), true);
  selection.toggleSelection(doneCard);
  assert.equal(selection.selectedIds.value.has("select-1"), false);
  selection.selectAll([doneCard, runningCard, { id: "select-3", status: "queued" }]);
  assert.deepEqual([...selection.selectedIds.value], ["select-1"]);
  assert.deepEqual(selection.collectSelectableIds([doneCard, runningCard]), ["select-1"]);

  // 阶段文案统一：封装与转码在详情列表与封面遮罩使用同一映射。
  const { isProcessingStage, stageLabel } = await vite.ssrLoadModule("/src/utils/downloadStatus.ts");
  assert.equal(stageLabel("transcoding"), "转码中");
  assert.equal(stageLabel("remuxing"), "封装中");
  assert.equal(stageLabel("video"), "下载视频");
  assert.equal(isProcessingStage("transcoding"), true);
  assert.equal(isProcessingStage("video"), false);

  console.log("下载图库任务状态回归通过");
} finally {
  await vite.close();
}
