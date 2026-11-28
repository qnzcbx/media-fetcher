import { brands, type BrandName } from "../assets/brands";
import { getSiteIdFromURL } from "./url";

export interface SourceBrand {
  id: BrandName;
  name: string;
}

const isBrandName = (value: string): value is BrandName =>
  Object.prototype.hasOwnProperty.call(brands, value);

// 优先复用记录已有的 siteId；旧记录缺少 siteId 时按合法 sourceUrl 域名推断，无法识别时返回 null 不伪造来源。
export function resolveSourceBrand(
  record: { siteId?: string; sourceUrl?: string } | null | undefined,
): SourceBrand | null {
  if (!record) return null;
  const direct = (record.siteId ?? "").trim().toLowerCase();
  if (direct && isBrandName(direct)) {
    return { id: direct, name: brands[direct].name };
  }
  const rawUrl = (record.sourceUrl ?? "").trim();
  if (rawUrl) {
    const inferred = getSiteIdFromURL(rawUrl);
    if (inferred && isBrandName(inferred)) {
      return { id: inferred, name: brands[inferred].name };
    }
  }
  return null;
}

export function formatFileSize(bytes: number) {
  const units = ["B", "KB", "MB", "GB"];
  let unit = 0;
  while (bytes >= 1024 && unit < units.length - 1) {
    bytes /= 1024;
    unit++;
  }
  return `${Number(bytes.toFixed(unit ? 1 : 0))} ${units[unit]}`;
}

export function formatDuration(seconds?: number) {
  if (!seconds || !Number.isFinite(seconds) || seconds < 0) return "";
  const total = Math.floor(seconds);
  const h = Math.floor(total / 3600);
  const m = Math.floor(total / 60) % 60;
  const s = total % 60;
  return (h ? [h, m, s] : [m, s]).map(value => String(value).padStart(2, "0")).join(":");
}
