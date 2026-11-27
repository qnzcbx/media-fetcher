import assert from "node:assert/strict";
import { createSSRApp, h } from "vue";
import { renderToString } from "vue/server-renderer";
import { createServer } from "vite";

const vite = await createServer({ root: "frontend", server: { middlewareMode: true, ws: false } });
try {
  const { extractSupportedURL, getSiteIdFromURL } = await vite.ssrLoadModule("/src/utils/url.ts");
  const { codecLabel, getUniqueVideoStreams, pickVideoStreamForGroup, getBestAudioStream, audioKey, resolveVideoContainer } =
    await vite.ssrLoadModule("/src/utils/media.ts");
  const { default: VideoQualityGrid } = await vite.ssrLoadModule("/src/components/media/VideoQualityGrid.vue");
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

  // 2.5 画质切换保留当前编码并更新新分组的准确 StreamKey（含 VideoQualityGrid 组件回归）
  const ytSwitchStreams = [
    { id: 315, quality_group_key: "2160p-60fps-SDR", stream_key: "315", codec: "vp9", bandwidth: 17174000, quality_label: "4K 60帧" },
    { id: 401, quality_group_key: "2160p-60fps-SDR", stream_key: "401", codec: "av01.0.13M.08", bandwidth: 8981000, quality_label: "4K 60帧" },
    { id: 299, quality_group_key: "1080p-60fps-SDR", stream_key: "299", codec: "avc1.64002A", bandwidth: 6933000, quality_label: "1080P 60帧" },
    { id: 303, quality_group_key: "1080p-60fps-SDR", stream_key: "303", codec: "vp9", bandwidth: 4652000, quality_label: "1080P 60帧" },
    { id: 399, quality_group_key: "1080p-60fps-SDR", stream_key: "399", codec: "av01.0.09M.08", bandwidth: 4492000, quality_label: "1080P 60帧" },
    { id: 335, quality_group_key: "1080p-60fps-HDR10", stream_key: "335", codec: "vp9.2", bandwidth: 5500000, quality_label: "1080P 60帧 HDR10" },
    { id: 298, quality_group_key: "720p-60fps-SDR", stream_key: "298", codec: "avc1.4D4020", bandwidth: 4206000, quality_label: "720P 60帧" },
  ];
  const ytGroups = getUniqueVideoStreams(ytSwitchStreams);
  const [group4k, group1080, group1080Hdr, group720] = ytGroups;
  // 1080P 代表轨是 AVC(299)，但当前编码为 VP9 时应保留 VP9 并切换至新分组的 303
  assert.equal(group1080.stream.stream_key, "299");
  assert.equal(pickVideoStreamForGroup(group1080, "vp9").stream_key, "303");
  // 不同 level 的同族编码（AV1 4K -> 1080P）应保留 AV1 并更新为 399
  assert.equal(pickVideoStreamForGroup(group1080, "av01.0.13M.08").stream_key, "399");
  // 同族 VP9 跨 SDR/HDR（vp9 -> vp9.2）应匹配 335
  assert.equal(pickVideoStreamForGroup(group1080Hdr, "vp9").stream_key, "335");
  // 新画质不含当前编码（VP9 -> 仅 AVC 的 720P）时回退代表轨 298
  assert.equal(pickVideoStreamForGroup(group720, "vp9").stream_key, "298");
  // Bilibili（无 stream_key）切换画质时保留 HEVC 家族并更新画质 ID
  const biliGroups = getUniqueVideoStreams([
    { id: 80, codec: "avc1.640032", bandwidth: 3000000, quality_label: "1080P" },
    { id: 80, codec: "hev1.1.6.L120.90", bandwidth: 2500000, quality_label: "1080P" },
    { id: 64, codec: "avc1.64001F", bandwidth: 1800000, quality_label: "720P" },
    { id: 64, codec: "hev1.1.6.L93.90", bandwidth: 1500000, quality_label: "720P" },
  ]);
  assert.equal(pickVideoStreamForGroup(biliGroups[1], "hev1.1.6.L120.90").codec, "hev1.1.6.L93.90");
  assert.equal(pickVideoStreamForGroup(biliGroups[1], "hev1.1.6.L120.90").id, 64);

  // 验证 VideoQualityGrid.vue SFC 可正常渲染分组卡片并按 selectedVideoGroupKey 高亮当前画质
  const html = await renderToString(
    createSSRApp({
      render: () =>
        h(VideoQualityGrid, {
          videoStreams: ytGroups,
          previewMap: {},
          coverUrl: "",
          selectedVideoID: 303,
          selectedVideoGroupKey: group1080.groupKey,
          selectedVideoCodec: "vp9",
        }),
    }),
  );
  assert.match(html, /4K 60帧/);
  assert.match(html, /1080P 60帧/);
  assert.match(html, /720P 60帧/);
  assert.equal((html.match(/media-card active/g) ?? []).length, 1);

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
