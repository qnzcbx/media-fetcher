import { DesktopService } from "../mygo";

export const getAccounts = async () => {
  return await DesktopService.getAccounts();
};

export const generateBiliQRCode = async () => {
  return await DesktopService.generateBiliQRCode();
};

export const pollBiliQRCode = async (key: string) => {
  return await DesktopService.pollBiliQRCode(key);
};
