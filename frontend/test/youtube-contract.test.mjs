import assert from "node:assert/strict";
import { createServer } from "vite";

const vite = await createServer({ root: "frontend", server: { middlewareMode: true, ws: false } });
try {
  const { extractSupportedURL, getSiteIdFromURL } = await vite.ssrLoadModule("/src/utils/url.ts");
  const { codecLabel, getUniqueVideoStreams, getBestAudioStream, audioKey, resolveVideoContainer } =
    await vite.ssrLoadModule("/src/utils/media.ts");
  const { parseErrorMessage } = await vite.ssrLoadModule("/src/utils/parseError.ts");

  // 1. YouTube URL 与仿冒域名校验
  assert.equal(getSiteIdFromURL("https://www.youtube.com/watch?v=aqz-KE-bpKQ"), "youtube");
  assert.equal(getSiteIdFromURL("https://youtube.com/watch?v=aqz-KE-bpKQ&list=PL123"), "youtube");
  assert.equal(getSiteIdFromURL("https://m.youtube.com/watch?v=aqz-KE-bpKQ"), "youtube");
  assert.equal(getSiteIdFromURL("https://youtu.be/aqz-KE-bpKQ?si=abc"), "youtube");
  assert.equal(getSiteIdFromURL("https://www.youtube.com/shorts/aqz-KE-bpKQ"), "youtube");
  assert.equal(extractSupportedURL("推荐观看 https://youtu.be/aqz-KE-bpKQ！很精彩"), "https://youtu.be/aqz-KE-bpKQ");
  for (const bad of [
    "https://youtube.com.evil.org/watch?v=aqz-KE-bpKQ",
    "https://fakeyoutu.be/aqz-KE-bpKQ",
    "https://user:pass@www.youtube.com/watch?v=aqz-KE-bpKQ",
    "https://www.youtube.com/playlist?list=PL1234567890",
    "https://www.youtube.com/watch?v=short",
  ]) {
    assert.equal(getSiteIdFromURL(bad), "", `should reject ${bad}`);
  }

  // 2. VP9 编码识别、QualityGroupKey 分组与 HDR/帧率隔离
  assert.equal(codecLabel("vp9"), "VP9");
  assert.equal(codecLabel("vp09.00.51.08"), "VP9");
  assert.equal(codecLabel("opus"), "Opus");
  const streams = [
    { id: 303, quality_group_key: "1080p-60fps-SDR", stream_key: "303", codec: "vp9", bandwidth: 4652000, quality_label: "1080P 60帧" },
    { id: 299, quality_group_key: "1080p-60fps-SDR", stream_key: "299", codec: "avc1.64002A", bandwidth: 6900000, quality_label: "1080P 60帧" },
    { id: 399, quality_group_key: "1080p-60fps-SDR", stream_key: "399", codec: "av01.0.09M.08", bandwidth: 4400000, quality_label: "1080P 60帧" },
    { id: 335, quality_group_key: "1080p-60fps-HDR10", stream_key: "335", codec: "vp9.2", bandwidth: 5500000, quality_label: "1080P 60帧 HDR10" },
    { id: 137, quality_group_key: "1080p-30fps-SDR", stream_key: "137", codec: "avc1.640028", bandwidth: 3200000, quality_label: "1080P" },
  ];
  const groups = getUniqueVideoStreams(streams);
  assert.equal(groups.length, 3);
  assert.equal(groups[0].groupKey, "1080p-60fps-SDR");
  assert.deepEqual(groups[0].streams.map((s) => codecLabel(s.codec)), ["AVC", "VP9", "AV1"]);
  assert.equal(groups[1].groupKey, "1080p-60fps-HDR10");
  assert.equal(groups[2].groupKey, "1080p-30fps-SDR");

  // 3. AudioStream StreamKey 区分 140 与 140-drc
  const audios = [
    { id: 140, stream_key: "140", codec: "mp4a.40.2", kind: "standard", bandwidth: 129000 },
    { id: 140, stream_key: "140-drc", codec: "mp4a.40.2", kind: "drc", bandwidth: 129000 },
    { id: 251, stream_key: "251", codec: "opus", kind: "standard", bandwidth: 131000 },
  ];
  assert.notEqual(audioKey(audios[0]), audioKey(audios[1]));
  assert.equal(getBestAudioStream(audios)?.stream_key, "140");

  // 4. 容器决策与 Go 端 1:1 对齐
  assert.equal(resolveVideoContainer("avc1.64002A", "mp4a.40.2"), "mp4");
  assert.equal(resolveVideoContainer("av01.0.13M.08", "mp4a.40.2"), "mp4");
  assert.equal(resolveVideoContainer("vp9", "opus"), "webm");
  assert.equal(resolveVideoContainer("avc1.64002A", "opus"), "mkv");
  assert.equal(resolveVideoContainer("vp9", "mp4a.40.2"), "mkv");
  assert.equal(resolveVideoContainer("av01.0.13M.08", "opus"), "mkv");
  assert.equal(resolveVideoContainer("vp9"), "webm");
  assert.equal(resolveVideoContainer("avc1.64002A"), "mp4");

  // 5. YouTube 专属错误分类
  assert.match(parseErrorMessage(new Error("youtube: 未找到解析组件 yt-dlp")), /yt-dlp/);
  assert.match(parseErrorMessage(new Error("youtube: 未找到兼容的 JavaScript 运行时")), /JavaScript/);
  assert.match(parseErrorMessage(new Error("YouTube 解析超时")), /超时/);
  assert.match(parseErrorMessage(new Error("该视频受年龄限制")), /年龄限制/);
  assert.match(parseErrorMessage(new Error("暂不支持直播流")), /直播/);
  assert.match(parseErrorMessage(new Error("人机校验")), /人机校验/);
  console.log("YouTube 前端契约测试（URL / 分组 / 容器 / 错误提示）通过");
} finally {
  await vite.close();
}
