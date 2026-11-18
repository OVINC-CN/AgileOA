import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, renderHook, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';

import { useApiAction, useApiMutation, useInitialLoading } from '@/lib/api-action';

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

describe('请求执行状态', () => {
  it('立即请求并阻止重复操作，请求完成后立即释放业务状态', async () => {
    const request = deferred<string>();
    const operation = vi.fn(() => request.promise);
    const { result } = renderHook(useApiAction);
    let completion: Promise<unknown> | undefined;
    act(() => {
      completion = result.current.run('search', operation);
    });
    expect(operation).toHaveBeenCalledOnce();
    expect(result.current.isPending('search')).toBe(true);
    const duplicate = vi.fn(() => Promise.resolve());
    expect(await result.current.run('refresh', duplicate)).toEqual({ ok: false });
    expect(duplicate).not.toHaveBeenCalled();
    await act(async () => {
      request.resolve('结果');
      await completion;
    });
    expect(result.current.pending).toBe(false);
    expect(await completion).toEqual({ ok: true, data: '结果' });
  });

  it('失败立即退出并保留错误，允许再次操作', async () => {
    const { result } = renderHook(useApiAction);
    await act(async () => {
      await result.current.run('save', () => Promise.reject(new Error('保存失败')));
    });
    expect(result.current.pending).toBe(false);
    expect(result.current.error?.message).toBe('保存失败');
    await act(async () => {
      await result.current.run('save', () => Promise.resolve());
    });
    expect(result.current.pending).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('页面离开后不再通知成功', async () => {
    const request = deferred<undefined>();
    const { result, unmount } = renderHook(useApiAction);
    const completion = result.current.run('workspace', () => request.promise);
    unmount();
    request.resolve(undefined);
    expect(await completion).toEqual({ ok: false });
  });

  it('保存等待相关缓存更新，完成后没有额外延迟', async () => {
    const client = new QueryClient();
    const request = deferred<undefined>();
    const cacheUpdate = deferred<undefined>();
    const close = vi.fn();
    const { result, unmount } = renderHook(
      () =>
        useApiMutation({
          mutationFn: async () => {
            await request.promise;
            await cacheUpdate.promise;
          },
          onSuccess: close,
        }),
      {
        wrapper: ({ children }: { children: ReactNode }) => (
          <QueryClientProvider client={client}>{children}</QueryClientProvider>
        ),
      },
    );
    act(() => {
      result.current.mutate();
    });
    await act(async () => {
      request.resolve(undefined);
      await request.promise;
    });
    await waitFor(() => {
      expect(result.current.isPending).toBe(true);
    });
    expect(close).not.toHaveBeenCalled();
    await act(async () => {
      cacheUpdate.resolve(undefined);
      await cacheUpdate.promise;
    });
    await waitFor(() => {
      expect(result.current.isPending).toBe(false);
    });
    expect(close).toHaveBeenCalledOnce();
    unmount();
    client.clear();
  });

  it('首次读取状态直接跟随请求状态', () => {
    const { result, rerender } = renderHook(({ active }) => useInitialLoading(active), {
      initialProps: { active: true },
    });
    expect(result.current).toBe(true);
    rerender({ active: false });
    expect(result.current).toBe(false);
  });
});
