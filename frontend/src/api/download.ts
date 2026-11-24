import { DesktopService, events } from "../mygo";

// 重新导出从 mygo.ts 获取的 DownloadParams 类型，为了更好的隔离
// 实际上我们可以直接使用参数接口
export interface DownloadParams {
  URL: string;
  AccountID: string;
  OutputDir: string;
  VideoID: number;
  VideoCodec: string;
  AudioID: number;
  SelectedImage: number[];
}

export const startDownload = async (params: any) => {
  return await DesktopService.startDownload(params);
};

export const cancelDownload = async () => {
  return await DesktopService.cancelDownload();
};

export const onProgress = (callback: (info: any) => void) => {
  events.progress.on(callback);
};

export const offProgress = (_callback: (info: any) => void) => {
  // MyGo events do not have an off method by default in this version
  // We can leave this empty or handle it differently if needed
};
