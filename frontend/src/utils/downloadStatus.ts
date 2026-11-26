import type { ProgressInfo } from "../mygo";

// 下载阶段与状态文案统一映射，避免封面遮罩与详细列表出现不同描述。
const STAGE_LABELS: Record<string, string> = {
  video: "下载视频",
  audio: "下载音频",
  images: "下载图片",
  muxing: "合并中",
  remuxing: "封装中",
  transcoding: "转码中",
};

// 这些阶段没有可靠的整体进度，不能虚构百分比。
export const isProcessingStage = (stage?: string) =>
  ["muxing", "remuxing", "transcoding", "publishing"].includes(stage ?? "");

export const stageLabel = (stage?: string) => STAGE_LABELS[stage ?? ""] ?? "下载中";

// 活动任务不可加入移除选择。
export const isActiveStatus = (status?: string) =>
  ["queued", "running", "cancelling"].includes(status ?? "");

export const formatMegabytes = (bytes: number) =>
  `${Math.floor((bytes / 1048576) * 10) / 10} MB`;

// 详细列表的进度副标题：处理阶段使用统一文案，其余展示真实字节进度。
export const progressText = (progress?: ProgressInfo) => {
  if (!progress || isProcessingStage(progress.stage)) return "";
  const total = progress.stage === "images" ? progress.total : progress.overallTotal;
  const downloaded = progress.stage === "images" ? progress.downloaded : progress.overallDownloaded;
  return total > 0 && downloaded < total ? Math.floor(downloaded / total * 100) + "%" : "";
};
