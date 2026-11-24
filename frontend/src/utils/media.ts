import type { AudioStream, VideoStream } from "../mygo";

// 同一画质只保留一个编码，卡片渲染所需的唯一流信息。
export type UniqueVideoStream = {
  index: number;
  stream: VideoStream;
};

export const getUniqueVideoStreams = (streams: VideoStream[]): UniqueVideoStream[] => {
  if (!streams || streams.length === 0) return [];
  const unique: UniqueVideoStream[] = [];
  const seen = new Set<string>();

  // 按照画质分组，同画质下优先保留 avc 编码（兼容性最好）
  // 为了做到这点，我们可以先排序
  const sorted = [...streams].sort((a, b) => {
    if (a.quality_label === b.quality_label) {
      const isAAvc = a.codec.toLowerCase().includes("avc");
      const isBAvc = b.codec.toLowerCase().includes("avc");
      if (isAAvc && !isBAvc) return -1;
      if (!isAAvc && isBAvc) return 1;
      return b.bandwidth - a.bandwidth;
    }
    return 0; // 不影响原本基于画质的自然顺序
  });

  for (let i = 0; i < sorted.length; i++) {
    const label = sorted[i].quality_label;
    if (!seen.has(label)) {
      seen.add(label);
      unique.push({ index: i, stream: sorted[i] });
    }
  }
  return unique;
};

export const getBestAudioStreamId = (streams: AudioStream[]): number => {
  if (!streams || streams.length === 0) return -1;
  const bestAudio = [...streams].sort((a, b) => b.bandwidth - a.bandwidth)[0];
  return bestAudio.id;
};
