// 通知只跳转到站内绝对路径，兼容没有链接的历史消息。
export function isNotificationURL(value: string) {
  if (value === '') return true;
  if (!/^\/(?!\/)/u.test(value) || /[\s\\\p{Cc}]/u.test(value)) return false;
  try {
    const path = decodeURIComponent(new URL(value, 'https://example.com').pathname);
    return /^\/(?!\/)/u.test(path) && !/[\s\\\p{Cc}]/u.test(path);
  } catch {
    return false;
  }
}
