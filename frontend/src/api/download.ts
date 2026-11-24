import { DesktopService, events } from "../mygo";

export const downloadAPI = {
  start: DesktopService.startDownload,
  cancel: DesktopService.cancelDownload,
  history: DesktopService.downloadHistory,
  thumbnail: DesktopService.galleryThumbnail,
  open: DesktopService.openDownloadRecord,
  remove: DesktopService.removeDownloadRecord,
  subscribe: events.progress.on,
};
