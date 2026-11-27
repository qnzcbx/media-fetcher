import type { AudioStream, VideoStream } from "../mygo";

// 同画质分组，代表轨道用于卡片显示，streams 保留全部编码。
export type UniqueVideoStream = {
  groupKey: string;
  streams: VideoStream[];
  index: number;
  stream: VideoStream;
};

export const codecLabel = (codec: string) =>
  /^(avc|h264)/i.test(codec)
    ? "AVC"
    : /^(hev|hvc|h265)/i.test(codec)
      ? "HEVC"
      : /^vp0?9/i.test(codec)
        ? "VP9"
        : /^vp0?8/i.test(codec)
          ? "VP8"
          : /^av01/i.test(codec)
            ? "AV1"
            : /^opus/i.test(codec)
              ? "Opus"
              : /^(mp4a|aac)/i.test(codec)
                ? "AAC"
                : codec || "未知编码";

const rank = (codec: string) => ({ AVC: 0, HEVC: 1, VP9: 2, AV1: 3 }[codecLabel(codec)] ?? 4);

export const videoGroupKey = (stream: VideoStream) => stream.quality_group_key || String(stream.id);

export const videoStreamKey = (stream: VideoStream) => stream.stream_key || `${stream.id}|${stream.codec}`;

export const getUniqueVideoStreams = (streams: VideoStream[]): UniqueVideoStream[] => {
  const groups = new Map<string, VideoStream[]>();
  for (const stream of streams ?? []) {
    const key = videoGroupKey(stream);
    const group = groups.get(key) ?? [];
    group.push(stream);
    groups.set(key, group);
  }
  return [...groups.entries()].map(([groupKey, group], index) => {
    group.sort((a, b) => rank(a.codec) - rank(b.codec) || b.bandwidth - a.bandwidth);
    return { groupKey, index, stream: group[0]!, streams: group };
  });
};

// 切换画质分组时，优先保留当前编码（先精确匹配再按编码家族匹配），不可用时回退该分组代表轨道。
export const pickVideoStreamForGroup = (group: UniqueVideoStream, currentCodec: string): VideoStream => {
  if (currentCodec) {
    const exact = group.streams.find((stream) => stream.codec === currentCodec);
    if (exact) return exact;
    const family = codecLabel(currentCodec);
    const sameFamily = group.streams.find((stream) => codecLabel(stream.codec) === family);
    if (sameFamily) return sameFamily;
  }
  return group.stream;
};

export const getBestAudioStream = (streams: AudioStream[]): AudioStream | undefined => {
  if (!streams || streams.length === 0) return undefined;
  const bestAudio = [...streams].sort(
    (a, b) => Number(!/^(mp4a|aac)/i.test(a.codec)) - Number(!/^(mp4a|aac)/i.test(b.codec)) || b.bandwidth - a.bandwidth,
  )[0];
  return bestAudio;
};

export const audioKey = (audio: AudioStream) => audio.stream_key || `${audio.id}|${audio.codec}|${audio.kind ?? ""}`;

// 仅用于展示分组；默认音轨仍由兼容性选择函数决定。
export const sortAudioStreams = (streams: AudioStream[]): AudioStream[] => {
  const order = (audio: AudioStream) => (audio.kind === "hires" ? 0 : audio.kind === "dolby" ? 1 : 2);
  return [...(streams ?? [])].sort((a, b) => order(a) - order(b) || b.bandwidth - a.bandwidth);
};

// 与 Go 端 downloader.ResolveVideoContainer 保持一致的无损容器预览决策。
const normalizeVideoFamily = (codec: string) => {
  const c = (codec ?? "").trim().toLowerCase();
  if (c.startsWith("avc") || c.startsWith("h264") || c === "x264") return "avc";
  if (c.startsWith("hev") || c.startsWith("hvc") || c.startsWith("h265") || c === "hevc") return "hevc";
  if (c.startsWith("av01") || c === "av1") return "av1";
  if (c.startsWith("vp09") || c === "vp9") return "vp9";
  if (c.startsWith("vp08") || c === "vp8") return "vp8";
  if (c === "mpeg4" || c.startsWith("mp4v")) return "mpeg4";
  return "";
};

const normalizeAudioFamily = (codec: string) => {
  const c = (codec ?? "").trim().toLowerCase();
  if (c === "mp4a.40.34" || c === "mp3") return "mp3";
  if (c.startsWith("mp4a") || c === "aac") return "aac";
  if (c.startsWith("opus")) return "opus";
  if (c.startsWith("vorbis")) return "vorbis";
  if (c.startsWith("flac")) return "flac";
  if (c === "ec-3" || c === "eac3" || c.startsWith("ec-3")) return "eac3";
  if (c === "ac-3" || c === "ac3" || c.startsWith("ac-3")) return "ac3";
  return "";
};

export const resolveVideoContainer = (videoCodec: string, audioCodec?: string): "mp4" | "webm" | "mkv" => {
  const v = normalizeVideoFamily(videoCodec);
  if (!audioCodec) {
    return v === "vp9" || v === "vp8" ? "webm" : "mp4";
  }
  const a = normalizeAudioFamily(audioCodec);
  if (v === "vp9" || v === "vp8") {
    return a === "opus" || a === "vorbis" ? "webm" : "mkv";
  }
  if (a === "opus" || a === "vorbis") {
    return "mkv";
  }
  return "mp4";
};
