import { useEffect } from 'react';

import { getConfig } from '@/lib/config';
import { showMessage } from '@/lib/messages';
import { randomPollIntervalMs } from '@/lib/polling';

export function PageUpdateDetector() {
  useEffect(() => {
    const config = getConfig();
    let baseline: string | null = null;
    let pending: AbortController | null = null;
    let stopped = false;
    let nextCheck: number;

    const check = async (scheduled: boolean) => {
      // 所有页面共用首次有效 ETag，请求未结束时跳过本轮。
      if (stopped || pending) return;
      const request = new AbortController();
      pending = request;
      const timeout = window.setTimeout(() => {
        request.abort();
      }, config.pageUpdateCheckTimeoutMs);

      try {
        // 只请求页面路径，定时检测附加日志标识。
        const url = new URL(window.location.href);
        url.search = '';
        url.hash = '';
        if (scheduled) url.searchParams.set('poll', 'page-update');
        const response = await fetch(url.href, {
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
        window.clearTimeout(nextCheck);
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

    // 每轮独立抽取间隔，分散多个页面的定时请求。
    const scheduleCheck = () => {
      nextCheck = window.setTimeout(() => {
        scheduleCheck();
        void check(true);
      }, randomPollIntervalMs(config.pageUpdateCheckIntervalMs));
    };
    scheduleCheck();
    void check(false);

    // 卸载时取消请求和定时器，避免过期响应触发提示。
    return () => {
      stopped = true;
      window.clearTimeout(nextCheck);
      pending?.abort();
    };
  }, []);

  return null;
}
