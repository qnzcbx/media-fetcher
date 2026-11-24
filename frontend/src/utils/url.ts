/**
 * 提取并规范化支持的站点链接
 * 寻找第一个合法且受支持的 URL，支持带参提取，自动移除结尾混杂中文字符
 */
export const extractSupportedURL = (text: string): string => {
  const match = text.match(/(https?:\/\/[^\s]+)/g);
  if (!match) return "";

  for (let rawUrl of match) {
    // 移除尾部的中文标点或常见闭合符号
    rawUrl = rawUrl.replace(/[\]】）"'\s]+$/, "");

    try {
      const u = new URL(rawUrl);
      const host = u.hostname.toLowerCase();

      if (host.includes("bilibili.com") || host.includes("b23.tv") || host.includes("lofter.com")) {
        return rawUrl;
      }
    } catch (e) {
      // 忽略无效的 URL，继续查找
    }
  }

  return "";
};

export const getSiteIdFromURL = (url: string): string => {
  try {
    const u = new URL(url);
    const host = u.hostname.toLowerCase();
    if (host.includes("bilibili.com") || host.includes("b23.tv")) return "bilibili";
    if (host.includes("lofter.com")) return "lofter";
  } catch (e) {
    //
  }
  return "";
};
