import { extractSupportedURL, getSiteIdFromURL } from "./url";
import { getUniqueVideoStreams, getBestAudioStreamId } from "./media";

console.log("Running utils tests...");

// Test URL Extraction
const testCases = [
  {
    input: "【井迪《失控》百万豪装录音棚大声听】 https://www.bilibili.com/video/BV1uYWq6UEFb/?share_source=copy_web&vd_source=123",
    expectedUrl: "https://www.bilibili.com/video/BV1uYWq6UEFb/?share_source=copy_web&vd_source=123",
    expectedSite: "bilibili"
  },
  {
    input: "来看图片 https://duzou85951.lofter.com/post/4d0c4f36_2bf8c0ca0】",
    expectedUrl: "https://duzou85951.lofter.com/post/4d0c4f36_2bf8c0ca0",
    expectedSite: "lofter"
  },
  {
    input: "短链测试 https://b23.tv/abcd123",
    expectedUrl: "https://b23.tv/abcd123",
    expectedSite: "bilibili"
  },
  {
    input: "无效链接 https://example.com",
    expectedUrl: "",
    expectedSite: ""
  }
];

for (const tc of testCases) {
  const extracted = extractSupportedURL(tc.input);
  if (extracted !== tc.expectedUrl) {
    console.error(`URL test failed for "${tc.input}". Expected "${tc.expectedUrl}", got "${extracted}"`);
  }
  const site = getSiteIdFromURL(extracted);
  if (site !== tc.expectedSite) {
    console.error(`Site ID test failed for "${tc.input}". Expected "${tc.expectedSite}", got "${site}"`);
  }
}

// Test Media grouping
const mockVideoStreams: any[] = [
  { id: 112, quality_label: "1080P", codec: "hev1", bandwidth: 2000 },
  { id: 80, quality_label: "1080P", codec: "avc1", bandwidth: 2500 },
  { id: 64, quality_label: "720P", codec: "avc1", bandwidth: 1000 },
];

const unique = getUniqueVideoStreams(mockVideoStreams);
if (unique.length !== 2) {
  console.error(`Media grouping failed. Expected 2 unique streams, got ${unique.length}`);
}
if (unique[0].stream.codec !== "avc1") {
  console.error(`Media grouping failed to prioritize AVC. Got codec ${unique[0].stream.codec}`);
}

const mockAudioStreams: any[] = [
  { id: 30280, quality: "192K", bandwidth: 192000 },
  { id: 30216, quality: "64K", bandwidth: 64000 },
];

const bestAudio = getBestAudioStreamId(mockAudioStreams);
if (bestAudio !== 30280) {
  console.error(`Best audio selection failed. Expected 30280, got ${bestAudio}`);
}

console.log("Utils tests finished.");
