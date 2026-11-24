export const extractSupportedURL = (text: string): string => {
  const links = text.match(/https?:\/\/[^\s<>"'，。！？；【】（）]+/g) ?? [];
  for (const link of links) {
    const candidate = link.replace(/[\]）)]+$/, "");
    if (getSiteIdFromURL(candidate)) return new URL(candidate).href;
  }
  return "";
};

export const getSiteIdFromURL = (url: string): string => {
  try {
    const { hostname, protocol, username, password } = new URL(url);
    if ((protocol !== "https:" && protocol !== "http:") || username || password) return "";
    if (hostname === "www.bilibili.com" || hostname === "bilibili.com" || hostname === "m.bilibili.com") return "bilibili";
    if (hostname.endsWith(".lofter.com")) return "lofter";
  } catch {
    return "";
  }
  return "";
};
