export type RuntimeSource = "bundled" | "managed" | "system" | "explicit" | "appdata" | "missing" | string;

export interface RuntimeComponentItem {
  key: string;
  name: string;
  available: boolean;
  version: string;
  source: RuntimeSource;
  kind: string;
  detail: string;
  path: string;
  pairPath: string;
}

export interface RuntimeStatusData {
  ytdlp: RuntimeComponentItem;
  jsRuntime: RuntimeComponentItem;
  ffmpeg: RuntimeComponentItem;
}

export function formatRuntimeSource(source: RuntimeSource): string {
  switch (source) {
    case "bundled":
    case "managed":
      return "内置";
    case "system":
      return "系统";
    case "explicit":
    case "appdata":
      return "用户指定";
    default:
      return "未配置";
  }
}

export function formatRuntimeSummary(item: RuntimeComponentItem): string {
  if (!item.available) {
    return item.source === "missing" ? "未找到" : "不可用";
  }
  if (item.key === "jsRuntime" && item.kind) {
    const label =
      item.kind === "deno"
        ? "Deno"
        : item.kind === "node"
          ? "Node.js"
          : item.kind === "quickjs"
            ? "QuickJS-NG"
            : item.kind;
    return item.version ? `${label} ${item.version}` : label;
  }
  return item.version || "已就绪";
}

export function orderedRuntimeItems(data: RuntimeStatusData): RuntimeComponentItem[] {
  return [data.ytdlp, data.jsRuntime, data.ffmpeg];
}
