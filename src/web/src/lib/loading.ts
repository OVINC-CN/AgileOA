import { useEffect, useState } from 'react';

import { loadingExitDelayMs } from '@/lib/motion';

// 加载立即显示，结束后延迟退出；新请求会取消上一次的退出计时。
export function useLoadingDelay(active: boolean) {
  const [visible, setVisible] = useState(active);
  if (active && !visible) setVisible(true);

  useEffect(() => {
    if (active || !visible) return;

    const timeout = window.setTimeout(() => {
      setVisible(false);
    }, loadingExitDelayMs);
    return () => {
      window.clearTimeout(timeout);
    };
  }, [active, visible]);

  return active || visible;
}
