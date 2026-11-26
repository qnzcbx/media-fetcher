import assert from "node:assert/strict";
import { createServer } from "vite";

const vite = await createServer({ root: "frontend", server: { middlewareMode: true, ws: false } });
try {
  const { createDownloads, progressPercent } = await vite.ssrLoadModule("/src/composables/useDownloads.ts");
  let latest;
  let refuse = true;
  const api = {
    validateDirectory: async (path) => path || "C:\\Downloads",
    start: async (params) => {
      assert.equal(params.OutputDir, "C:\\Downloads");
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
  assert.equal(state.cards.value[0].retry.OutputDir, "C:\\Downloads");
  params.OutputDir = "C:\\Changed";
  assert.equal(state.cards.value[0].retry.OutputDir, "C:\\Downloads");
  params.OutputDir = "";
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
    validateDirectory: async (path) => path || "C:\\Downloads",
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
  assert.equal(removeState.managementError.value, "部分记录移除失败，请重试");

  // 批量移除期间重复提交只执行一次。
  let calls = 0;
  const guardState = createDownloads({
    validateDirectory: async (path) => path || "C:\\Downloads",
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
    await guardState.removeMultiple(["active"]);
    assert.equal(guardState.cards.value.length, 1);
  }
  assert.equal(calls, 1);
  guardState.cards.value = [{ id: "active", status: "done" }];
  await guardState.removeMultiple(["active"]);
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


  let recycleCalls = 0;
  const files = createDownloads({
    fileStates: async ids => ids.map(id => ({ id, status: "available", canRecycle: true })),
    prepareDeletion: async ids => ({token:"plan",targets:ids.filter(id=>id!=="missing").map(id=>({id,path:"C:\\"+id,name:id})),skipped:ids.includes("missing")?1:0}),
    executeDeletion: async () => { recycleCalls++; return [{id:"ok",status:"recycled"},{id:"bad",status:"failed",message:"占用"}]; },
    remove: async () => {},
  });
  files.cards.value = [{id:"ok",status:"done",finalPath:"C:\\ok"},{id:"bad",status:"done",finalPath:"C:\\bad"},{id:"missing",status:"done"},{id:"active",status:"running"}];
  await files.recycleMultiple(["ok","bad","missing","active"]);
  assert.deepEqual(files.recycleConfirmation.value.ids,["ok","bad","missing"]);
  assert.equal(files.recycleConfirmation.value.deleteFiles,true);
  files.recycleConfirmation.value = undefined;
  assert.equal(recycleCalls,0);
  await files.recycleMultiple(["ok","bad"]);
  await files.confirmRecycle();
  assert.equal(recycleCalls,1);
  assert.deepEqual(files.cards.value.map(card=>card.id),["bad","missing","active"]);
  await files.recycleMultiple(["bad"]);
  files.recycleConfirmation.value.deleteFiles=false;
  await files.confirmRecycle();
  assert.equal(recycleCalls,1);
  assert.deepEqual(files.cards.value.map(card=>card.id),["missing","active"]);
  assert.equal(progressPercent(event("video","media",latest,{overallDownloaded:180,overallTotal:200})),90);
  assert.equal(progressPercent(event("audio","media",latest,{overallDownloaded:190,overallTotal:200})),95);
  assert.equal(progressPercent(event("publishing","media",latest,{overallDownloaded:200,overallTotal:200})),undefined);
  console.log("下载图库任务状态回归通过");
} finally {
  await vite.close();
}
