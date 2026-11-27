import assert from "node:assert/strict";
import { createServer } from "vite";

const vite = await createServer({ root: "frontend", server: { middlewareMode: true, ws: false } });
try {
  const { createCredentialState, resolveParseTarget, canUnlock } =
    await vite.ssrLoadModule("/src/composables/useCredentials.ts");
  const saved = { siteId: "bilibili", accountId: "test-account", name: "Test", avatarUrl: "", status: "active", isActive: true };
  let queries = 0;
  let rejectQuery;
  const state = createCredentialState(() => {
    queries++;
    if (queries === 1) return new Promise((_, reject) => { rejectQuery = reject; });
    return Promise.resolve([
      saved,
      { ...saved, siteId: "lofter", status: "expired" },
      { ...saved, siteId: "logged-out", status: "logged_out" },
      { ...saved, siteId: "unknown-status", status: "Valid" },
      { ...saved, siteId: "unselected", isActive: false },
    ]);
  });
  const first = state.ensureInitialized();
  assert.equal(first, state.ensureInitialized());
  await Promise.resolve();
  assert.equal(queries, 1);
  rejectQuery(new Error("IPC unavailable"));
  assert.equal(await first, false);
  assert.equal(state.isInitialized.value, false);
  assert.ok(state.credentialError.value);
  assert.equal(state.getActiveAccountId("bilibili"), "");

  assert.equal(await state.ensureInitialized(), true);
  assert.equal(await state.ensureInitialized(), true);
  assert.equal(queries, 2);
  assert.equal(state.credentialError.value, "");
  assert.equal(state.getActiveAccountId("bilibili"), "test-account");
  for (const site of ["lofter", "logged-out", "unknown-status", "unselected"]) {
    assert.equal(state.getActiveAccountId(site), "");
  }

  const link = "https://www.bilibili.com/video/BV1xx411c7mD?p=2";
  const target = resolveParseTarget(link, state.getActiveAccountId);
  assert.deepEqual(target, { url: link, siteId: "bilibili", accountId: "test-account" });
  assert.deepEqual(resolveParseTarget(`【分享】 ${link}。打开查看`, state.getActiveAccountId), target);
  assert.equal(resolveParseTarget("https://a.lofter.com/post/abc_def", state.getActiveAccountId).accountId, "");
  for (const unsupported of ["https://b23.tv/example", "https://bilibili.com.example.org/video/BV1xx411c7mD"]) {
    assert.equal(resolveParseTarget(unsupported, state.getActiveAccountId).url, "");
  }

  for (const kind of ["ep748435", "ss123"]) {
    const url = `https://www.bilibili.com/bangumi/play/${kind}/`;
    assert.deepEqual(resolveParseTarget(`分享 ${url}`, state.getActiveAccountId),
      { url, siteId: "bilibili", accountId: "test-account" });
  }

  const anonymous = { siteId: "bilibili", isAnonymous: true, usedAccountId: "" };
  assert.equal(canUnlock(anonymous, true, ""), true);
  assert.equal(canUnlock(anonymous, false, ""), false);
  assert.equal(canUnlock(anonymous, true, "test-account"), false);
  assert.equal(canUnlock({ ...anonymous, isAnonymous: false, usedAccountId: "test-account" }, true, ""), false);
  assert.equal(canUnlock({ ...anonymous, siteId: "lofter" }, true, ""), false);
  const { DesktopService } = await vite.ssrLoadModule("/src/mygo.ts");
  const { useMediaParser } = await vite.ssrLoadModule("/src/composables/useMediaParser.ts");
  const parser = useMediaParser();
  DesktopService.extract = async () => anonymous;
  assert.equal(await parser.parse(link, ""), anonymous);
  DesktopService.extract = async () => { throw new Error("授权解析失败"); };
  assert.equal(await parser.parse(link, "test-account"), null);
  assert.equal(parser.mediaInfo.value, anonymous);
  assert.equal(parser.parseError.value, "解析失败，请重试。");
  DesktopService.extract = async () => ({ ...anonymous, isAnonymous: false });
  assert.equal(await parser.parse(link, "test-account", () => false), null);
  assert.equal(parser.mediaInfo.value, anonymous);

  const episodes = [{ id: "1", url: "https://www.bilibili.com/bangumi/play/ep1/", title: "1" },
    { id: "2", url: "https://www.bilibili.com/bangumi/play/ep2/", title: "2" }];
  const firstEpisode = { ...anonymous, episodes, sourceUrl: episodes[0].url, videoStreams: [{id: 80}], audioStreams: [{id: 30280}], images: [] };
  DesktopService.extract = async () => firstEpisode;
  await parser.parse(episodes[0].url, "");
  let rejectEpisode;
  DesktopService.extract = () => new Promise((_, reject) => { rejectEpisode = reject; });
  const failed = parser.parseEpisode(episodes[1].url);
  assert.equal(parser.mediaInfo.value.videoStreams.length, 0);
  assert.equal(parser.selectedEpisodeURL.value, episodes[1].url);
  assert.equal(parser.isParsing.value, true);
  rejectEpisode(new Error("extractor: 接口拒绝: 当前仅能试看"));
  await failed;
  assert.equal(parser.parseError.value, "");
  assert.equal(parser.episodeError.value, "当前剧集仅提供试看资源。");
  assert.equal(parser.mediaInfo.value.audioStreams.length, 0);
  assert.equal(parser.selectedEpisodeURL.value, episodes[1].url);
  DesktopService.extract = async () => firstEpisode;
  assert.equal(await parser.parseEpisode(episodes[0].url), firstEpisode);
  let resolveLate;
  DesktopService.extract = () => new Promise((resolve) => { resolveLate = resolve; });
  const late = parser.parseEpisode(episodes[1].url);
  parser.clearMediaInfo();
  assert.equal(parser.episodeError.value, "");
  DesktopService.extract = async () => anonymous;
  await parser.parse(link, "");
  resolveLate(firstEpisode);
  assert.equal(await late, null);
  assert.equal(parser.mediaInfo.value, anonymous);
  assert.equal(parser.parseError.value, "");

  await vite.ssrLoadModule("/src/utils/test_runner.ts");
  console.log("凭证初始化、站点账号选择与解锁条件回归通过");
} finally {
  await vite.close();
}
