import { DesktopService, events } from "../mygo";

export const downloadAPI = {
  validateDirectory: DesktopService.validateDownloadDirectory,
  fileStates: DesktopService.getDownloadFileStates,
  recycle: DesktopService.recycleDownloadFiles,
  start: DesktopService.startDownload,
  cancel: DesktopService.cancelDownload,
  history: DesktopService.downloadHistory,
  duration: DesktopService.downloadDuration,
  thumbnail: DesktopService.galleryThumbnail,
  open: DesktopService.openDownloadRecord,
  remove: DesktopService.removeDownloadRecord,
  subscribe: events.progress.on,
};
