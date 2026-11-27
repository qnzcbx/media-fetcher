// 内部异常保留在 IPC 错误中，界面只显示与用户操作有关的提示。
export function parseErrorMessage(error: unknown): string {
  const message = error instanceof Error ? error.message : String(error);
  if (message.includes("未找到解析组件 yt-dlp")) return "未找到 YouTube 解析组件 yt-dlp，请按说明配置后重试。";
  if (message.includes("未找到兼容的 JavaScript 运行时") || message.includes("挑战求解失败")) return "未找到兼容的 JavaScript 运行时（QuickJS-NG / Node 22+ / Deno 2.3+）。";
  if (message.includes("解析超时")) return "解析超时，请检查网络连接后重试。";
  if (message.includes("人机校验") || message.includes("访问频率限制")) return "YouTube 触发访问频率限制或人机校验，请稍后重试。";
  if (message.includes("年龄限制")) return "该视频受年龄限制，当前暂不支持登录解析。";
  if (message.includes("私有视频")) return "该视频为私有视频，当前暂不支持下载。";
  if (message.includes("已删除") || message.includes("不可用")) return "该视频已删除或不可用。";
  if (message.includes("暂不支持直播流")) return "暂不支持直播流或待开播内容。";
  if (message.includes("暂不支持的流媒体协议")) return "当前视频仅提供暂不支持的流媒体协议。";
  if (message.includes("试看")) return "当前剧集仅提供试看资源。";
  if (message.includes("凭证无效") || message.includes("凭证失效") || message.includes("已过期")) return "登录已失效，请重新登录。";
  if (message.includes("需要登录")) return "请先登录后重试。";
  if (message.includes("DRM")) return "暂不支持此 DRM 保护资源。";
  if (message.includes("不支持的媒体")) return "暂不支持当前媒体资源。";
  if (message.includes("地区")) return "当前资源受到地区访问限制。";
  if (message.includes("会员") || message.includes("-10403")) return "当前账号没有该资源的访问权限。";
  if (message.includes("不可播放")) return "当前视频不可播放。";
  if (message.includes("下载链接") || message.includes("已失效")) return "媒体下载链接已失效，请重新解析。";
  if (message.includes("网络请求失败") || message.includes("网络异常")) return "网络请求失败，请检查网络连接后重试。";
  return "解析失败，请重试。";
}
