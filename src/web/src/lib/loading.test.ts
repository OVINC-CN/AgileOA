import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { useLoadingDelay } from '@/lib/loading';
import { loadingExitDelayMs } from '@/lib/motion';

beforeEach(() => {
  vi.useFakeTimers();
});
afterEach(() => {
  vi.useRealTimers();
});

describe('加载指示器退出延迟', () => {
  it('开始时立即显示，结束后完整保留 360 毫秒', () => {
    const { result, rerender } = renderHook(({ active }) => useLoadingDelay(active), {
      initialProps: { active: false },
    });
    expect(result.current).toBe(false);
    rerender({ active: true });
    expect(result.current).toBe(true);
    rerender({ active: false });
    expect(result.current).toBe(true);
    act(() => {
      vi.advanceTimersByTime(loadingExitDelayMs - 1);
    });
    expect(result.current).toBe(true);
    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(result.current).toBe(false);
  });

  it('退出期间的新请求取消旧计时，并从最后一次完成重新计算延迟', () => {
    const { result, rerender } = renderHook(({ active }) => useLoadingDelay(active), {
      initialProps: { active: true },
    });
    rerender({ active: false });
    act(() => {
      vi.advanceTimersByTime(loadingExitDelayMs / 2);
    });
    rerender({ active: true });
    act(() => {
      vi.advanceTimersByTime(loadingExitDelayMs);
    });
    expect(result.current).toBe(true);
    rerender({ active: false });
    act(() => {
      vi.advanceTimersByTime(loadingExitDelayMs - 1);
    });
    expect(result.current).toBe(true);
    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(result.current).toBe(false);
  });

  it('组件卸载后取消退出计时', () => {
    const { rerender, unmount } = renderHook(({ active }) => useLoadingDelay(active), {
      initialProps: { active: true },
    });
    rerender({ active: false });
    expect(vi.getTimerCount()).toBe(1);
    unmount();
    expect(vi.getTimerCount()).toBe(0);
  });
});
