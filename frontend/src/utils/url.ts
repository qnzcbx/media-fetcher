const YOUTUBE_ID_RE = /^[A-Za-z0-9_-]{11}$/;

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
    const parsed = new URL(url);
    const { hostname, protocol, username, password, pathname, searchParams } = parsed;
    if ((protocol !== "https:" && protocol !== "http:") || username || password) return "";
    const host = hostname.toLowerCase();
    if (host === "www.bilibili.com" || host === "bilibili.com" || host === "m.bilibili.com") return "bilibili";
    if (host.endsWith(".lofter.com")) return "lofter";
    if (host === "youtube.com" || host === "www.youtube.com" || host === "m.youtube.com") {
      const cleanPath = pathname.replace(/^\/+|\/+$/g, "");
      if (cleanPath === "watch" && YOUTUBE_ID_RE.test(searchParams.get("v") ?? "")) return "youtube";
      const parts = cleanPath.split("/");
      if (parts.length === 2 && parts[0] === "shorts" && YOUTUBE_ID_RE.test(parts[1] ?? "")) return "youtube";
      return "";
    }
    if (host === "youtu.be") {
      const parts = pathname.replace(/^\/+|\/+$/g, "").split("/");
      if (parts.length === 1 && YOUTUBE_ID_RE.test(parts[0] ?? "")) return "youtube";
      return "";
    }
  } catch {
    return "";
  }
  return "";
};
