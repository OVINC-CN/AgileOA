import { useEffect } from 'react';

import { getConfig } from '@/lib/config';
import { showMessage } from '@/lib/messages';

export function PageUpdateDetector() {
  useEffect(() => {
    const config = getConfig();
    let baseline: string | null = null;
    let pending: AbortController | null = null;
    let stopped = false;

    const check = async () => {
      // 所有页面共用首次有效 ETag，请求未结束时跳过本轮。
      if (stopped || pending) return;
      const request = new AbortController();
      pending = request;
      const timeout = window.setTimeout(() => {
        request.abort();
      }, config.pageUpdateCheckTimeoutMs);

      try {
        // 每次通过 GET 读取当前页面的 ETag，并绕过浏览器缓存。
        const response = await fetch(window.location.href, {
          method: 'GET',
          cache: 'no-store',
          signal: request.signal,
        });
        if (request.signal.aborted || !response.ok) return;
        const etag = response.headers.get('ETag')?.trim();
        if (!etag) return;
        if (baseline === null) {
          baseline = etag;
          return;
        }
        if (etag === baseline) return;

        // 检测到更新后保留唯一的常驻提示，等待用户刷新。
        stopped = true;
        window.clearInterval(interval);
        showMessage('发现新版本，请刷新页面以使用最新版本', 'page-update', {
          durationMs: 0,
          dismissible: false,
          action: {
            label: '一键刷新',
            onClick: () => {
              window.location.reload();
            },
          },
        });
      } catch {
        // 网络异常与超时不打断使用，下一轮继续检测。
      } finally {
        window.clearTimeout(timeout);
        pending = null;
      }
    };

    const interval = window.setInterval(() => {
      void check();
    }, config.pageUpdateCheckIntervalMs);
    void check();

    // 卸载时取消请求和定时器，避免过期响应触发提示。
    return () => {
      stopped = true;
      window.clearInterval(interval);
      pending?.abort();
    };
  }, []);

  return null;
}
