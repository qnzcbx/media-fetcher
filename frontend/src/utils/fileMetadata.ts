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
