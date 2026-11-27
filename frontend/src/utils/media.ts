import type { AudioStream, VideoStream } from "../mygo";

// 同画质分组，代表轨道用于卡片显示，streams 保留全部编码。
export type UniqueVideoStream = {
  streams: VideoStream[];
  index: number;
  stream: VideoStream;
};

export const codecLabel = (codec: string) => /^(avc|h264)/i.test(codec) ? "AVC" : /^(hev|hvc|h265)/i.test(codec) ? "HEVC" : /^av01/i.test(codec) ? "AV1" : codec || "未知编码";
const rank = (codec: string) => ({AVC:0,HEVC:1,AV1:2}[codecLabel(codec)] ?? 3);
export const getUniqueVideoStreams = (streams: VideoStream[]): UniqueVideoStream[] => {
 const groups = new Map<number, VideoStream[]>();
 for(const stream of streams ?? []) {const group=groups.get(stream.id) ?? []; group.push(stream); groups.set(stream.id,group);}
 return [...groups.values()].map((group,index)=>{ group.sort((a,b)=>rank(a.codec)-rank(b.codec)||b.bandwidth-a.bandwidth);return {index,stream:group[0]!,streams:group}; });
};

export const getBestAudioStream = (streams: AudioStream[]): AudioStream | undefined => {
  if (!streams || streams.length === 0) return undefined;
  const bestAudio = [...streams].sort((a, b) => Number(!/^(mp4a|aac)/i.test(a.codec)) - Number(!/^(mp4a|aac)/i.test(b.codec)) || b.bandwidth - a.bandwidth)[0];
  return bestAudio;
};

export const audioKey = (audio: AudioStream) => `${audio.id}|${audio.codec}|${audio.kind ?? ""}`;
