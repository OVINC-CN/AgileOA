import { useSyncExternalStore } from 'react';

import type { CurrentUser } from '@/features/user/api';

const storageKey = 'watermark.hidden_user_id';
const changeEvent = 'watermark:changed';

function hiddenUserID() {
  // 浏览器禁用会话存储时仍正常显示页面和水印。
  try {
    return sessionStorage.getItem(storageKey);
  } catch {
    return null;
  }
}

function subscribe(listener: () => void) {
  window.addEventListener(changeEvent, listener);
  window.addEventListener('storage', listener);
  return () => {
    window.removeEventListener(changeEvent, listener);
    window.removeEventListener('storage', listener);
  };
}

export function useWatermarkHidden(user: CurrentUser | null | undefined) {
  const owner = useSyncExternalStore(subscribe, hiddenUserID);
  return user?.is_admin === true && owner === String(user.id);
}

export function setWatermarkHidden(user: CurrentUser, hidden: boolean) {
  // 调用方须刷新当前身份，设置时仍检查管理员，防止旧菜单绕过限制。
  if (!user.is_admin) throw new Error('仅管理员可以设置水印');
  try {
    if (hidden) sessionStorage.setItem(storageKey, String(user.id));
    else sessionStorage.removeItem(storageKey);
  } catch {
    throw new Error('无法保存水印设置，请检查浏览器设置');
  }
  window.dispatchEvent(new Event(changeEvent));
}

export function clearWatermarkPreference() {
  // 清理失败不影响退出或重新登录；读取不可用时默认显示水印。
  try {
    sessionStorage.removeItem(storageKey);
  } catch {
    return;
  }
  window.dispatchEvent(new Event(changeEvent));
}
