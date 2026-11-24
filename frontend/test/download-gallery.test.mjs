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
  console.log("下载图库任务状态回归通过");
} finally {
  await vite.close();
}
