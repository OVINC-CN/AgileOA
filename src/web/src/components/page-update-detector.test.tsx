import { act, cleanup, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { PageUpdateDetector } from '@/components/page-update-detector';
import { getConfig } from '@/lib/config';

const originalURL = window.location.href;
const originalConfig = window.AGILE_WORKSPACE_CONFIG;

function pageResponse(etag?: string, status = 200) {
  return new Response(null, {
    status,
    headers: etag ? { ETag: etag } : {},
  });
}

async function advanceTime(milliseconds: number) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(milliseconds);
  });
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.spyOn(Math, 'random').mockReturnValue(0.2);
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  window.history.replaceState(null, '', originalURL);
  window.AGILE_WORKSPACE_CONFIG = originalConfig;
});

describe('页面更新检测', () => {
  it('进入时建立基准，每轮重新抽取检测间隔，跨页面变化后只显示一次常驻提示', async () => {
    const random = vi.mocked(Math.random).mockReturnValue(0);
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(pageResponse('"initial"'))
      .mockResolvedValueOnce(pageResponse('"initial"'))
      .mockResolvedValue(pageResponse('"updated"'));
    vi.stubGlobal('fetch', fetchMock);
    window.history.replaceState(null, '', '/stu/roster?workspace=fixture');
    render(<PageUpdateDetector />);
    await advanceTime(0);
    expect(fetchMock).toHaveBeenCalledExactlyOnceWith(
      window.location.href,
      expect.objectContaining({ method: 'GET', cache: 'no-store' }),
    );
    expect(fetchMock.mock.calls[0]?.[1]?.signal).toBeInstanceOf(AbortSignal);
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    await advanceTime(49999);
    expect(fetchMock).toHaveBeenCalledOnce();

    random.mockReturnValue(0.999999);
    window.history.replaceState(null, '', '/cpc/roster?workspace=another');
    await advanceTime(1);
    expect(fetchMock).toHaveBeenLastCalledWith(window.location.href, expect.any(Object));
    await advanceTime(99999);
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledTimes(2);
    await advanceTime(1);
    expect(screen.getByRole('alert')).toHaveTextContent('发现新版本');
    expect(screen.getByRole('button', { name: '一键刷新' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '关闭消息' })).not.toBeInTheDocument();
    await advanceTime(180000);
    expect(screen.getAllByRole('alert')).toHaveLength(1);
    expect(fetchMock).toHaveBeenCalledTimes(3);
  });

  it('网络错误、失败响应和缺少 ETag 不建立基准，也不打断后续检测', async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockRejectedValueOnce(new TypeError('Network error'))
      .mockResolvedValueOnce(pageResponse('"error"', 503))
      .mockResolvedValueOnce(pageResponse())
      .mockResolvedValueOnce(pageResponse('"initial"'))
      .mockResolvedValueOnce(pageResponse('"initial"'))
      .mockResolvedValueOnce(pageResponse('"updated"'));
    vi.stubGlobal('fetch', fetchMock);
    render(<PageUpdateDetector />);
    await advanceTime(0);
    await advanceTime(240000);
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    await advanceTime(60000);
    expect(screen.getByRole('alert')).toHaveTextContent('发现新版本');
  });

  it('请求进行中不重复发送，超时后恢复检测，卸载会取消请求并忽略迟到的变化', async () => {
    window.AGILE_WORKSPACE_CONFIG = { ...getConfig(), pageUpdateCheckTimeoutMs: 90000 };
    const signals: AbortSignal[] = [];
    let finish: ((response: Response) => void) | undefined;
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockImplementationOnce((_url, options) => {
        return new Promise<Response>((_resolve, reject) => {
          const signal = options?.signal;
          if (!signal) throw new Error('检测请求缺少取消信号');
          signals.push(signal);
          signal.addEventListener('abort', () => {
            reject(new DOMException('Request aborted', 'AbortError'));
          });
        });
      })
      .mockResolvedValueOnce(pageResponse('"initial"'))
      .mockImplementationOnce((_url, options) => {
        const signal = options?.signal;
        if (!signal) throw new Error('检测请求缺少取消信号');
        signals.push(signal);
        return new Promise<Response>((resolve) => {
          finish = resolve;
        });
      });
    vi.stubGlobal('fetch', fetchMock);
    const view = render(<PageUpdateDetector />);
    await advanceTime(60000);
    expect(fetchMock).toHaveBeenCalledOnce();
    await advanceTime(30000);
    expect(signals[0]?.aborted).toBe(true);
    await advanceTime(90000);
    expect(fetchMock).toHaveBeenCalledTimes(3);
    view.unmount();
    expect(signals[1]?.aborted).toBe(true);
    await act(async () => {
      if (!finish) throw new Error('未发起后续检测请求');
      finish(pageResponse('"updated"'));
      await vi.advanceTimersByTimeAsync(120000);
    });
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledTimes(3);
  });
});
