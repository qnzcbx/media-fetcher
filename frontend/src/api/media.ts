import { DesktopService } from "../mygo";

export const extractMedia = async (url: string, accountId: string) => {
  return await DesktopService.extract(url, accountId);
};

export const fetchPreviewBase64 = async (imgUrl: string, siteId: string) => {
  return await DesktopService.fetchPreviewBase64(imgUrl, siteId);
};
