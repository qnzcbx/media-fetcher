const YOUTUBE_ID_RE = /^[A-Za-z0-9_-]{11}$/;
const BILIBILI_VIDEO_RE = /^\/video\/BV[0-9A-Za-z]+(?:\/)?$/i;
const BILIBILI_BANGUMI_RE = /^\/bangumi\/play\/(?:ep|ss)\d+(?:\/)?$/i;
const LOFTER_POST_RE = /^\/post\/[^/?#]+(?:\/)?$/i;
const URL_TOKEN_RE = /https?:\/\/[^\s<>"'，。！？；【】（）]+/g;

export const extractSupportedURL = (text: string): string => {
  const links = text.match(URL_TOKEN_RE) ?? [];
  for (const link of links) {
    const candidate = link.replace(/[\]）)]+$/, "");
    if (getSiteIdFromURL(candidate)) return new URL(candidate).href;
  }
  return "";
};

// 按站点清洗追踪参数与片段，与 Go 侧 cleanSource 保持一致。
export const cleanSupportedURL = (rawUrl: string): string => {
  const siteId = getSiteIdFromURL(rawUrl);
  if (!siteId) return "";
  try {
    const parsed = new URL(rawUrl);
    const host = parsed.hostname.toLowerCase();
    const kept = new URLSearchParams();
    if (siteId === "bilibili") {
      const p = parsed.searchParams.get("p");
      if (p && /^[1-9]\d*$/.test(p)) kept.set("p", p);
    } else if (siteId === "youtube") {
      const cleanPath = parsed.pathname.replace(/^\/+|\/+$/g, "");
      if (host !== "youtu.be" && cleanPath === "watch") {
        const v = parsed.searchParams.get("v") ?? "";
        if (YOUTUBE_ID_RE.test(v)) kept.set("v", v);
      }
    }
    parsed.search = kept.toString();
    parsed.hash = "";
    parsed.username = "";
    parsed.password = "";
    return parsed.href;
  } catch {
    return "";
  }
};

// 自动识别仅接受具体媒体资源链接，避免用户复制站点首页时误填入。
export const isAutoDetectableMediaURL = (rawUrl: string): boolean => {
  const siteId = getSiteIdFromURL(rawUrl);
  if (!siteId) return false;
  try {
    const parsed = new URL(rawUrl);
    if (siteId === "youtube") return true;
    if (siteId === "bilibili") {
      return BILIBILI_VIDEO_RE.test(parsed.pathname) || BILIBILI_BANGUMI_RE.test(parsed.pathname);
    }
    if (siteId === "lofter") {
      return LOFTER_POST_RE.test(parsed.pathname);
    }
    return false;
  } catch {
    return false;
  }
};

// 从剪贴板文本或分享文案中提取唯一受支持媒体链接；包含多个不同有效链接时返回空字符串，不擅自选择。
export const extractSingleSupportedURL = (text: string): string => {
  if (!text) return "";
  const links = text.match(URL_TOKEN_RE) ?? [];
  const unique = new Set<string>();
  for (const link of links) {
    const candidate = link.replace(/[\]）)]+$/, "");
    if (!isAutoDetectableMediaURL(candidate)) continue;
    const cleaned = cleanSupportedURL(candidate);
    if (cleaned) unique.add(cleaned);
    if (unique.size > 1) return "";
  }
  return unique.size === 1 ? [...unique][0] : "";
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
