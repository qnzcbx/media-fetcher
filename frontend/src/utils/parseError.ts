// 内部异常保留在 IPC 错误中，界面只显示与用户操作有关的提示。
export function parseErrorMessage(error: unknown): string {
  const message = error instanceof Error ? error.message : String(error);
  if (message.includes('试看')) return '当前剧集仅提供试看资源。';
  if (message.includes('凭证无效') || message.includes('凭证失效') || message.includes('已过期')) return '登录已失效，请重新登录。';
  if (message.includes('需要登录')) return '请先登录后重试。';
  if (message.includes('DRM')) return '暂不支持此 DRM 资源。';
  if (message.includes('不支持的媒体')) return '暂不支持当前媒体资源。';
  if (message.includes('地区')) return '当前资源受到访问限制。';
  if (message.includes('会员') || message.includes('-10403')) return '当前账号没有该剧集的访问权限。';
  if (message.includes('不可播放')) return '当前剧集不可播放。';
  if (message.includes('网络请求失败')) return '网络请求失败，请重试。';
  return '解析失败，请重试。';
}
