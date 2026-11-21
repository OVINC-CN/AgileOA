import { act, renderHook } from '@testing-library/react';
import { useState } from 'react';
import { expect, it, vi } from 'vitest';

import { useFilterForm } from '@/lib/filter-form';

const defaults = { keyword: '', status: ['enabled'], date: '' };

it('枚举和日期只更新已应用条件，文字草稿保留到搜索且重置立即恢复默认', async () => {
  const requests = vi.fn().mockResolvedValue(undefined);
  const { result } = renderHook(() => {
    const [applied, setApplied] = useState(defaults);
    const apply = async (values: typeof defaults) => {
      setApplied(values);
      await requests(values);
    };
    return { ...useFilterForm(defaults, apply, applied), applied, apply };
  });
  act(() => {
    result.current.setDraft({ ...defaults, keyword: '未搜索文字' });
  });
  expect(requests).not.toHaveBeenCalled();
  expect(result.current.hasUnappliedChanges).toBe(true);
  act(() => {
    result.current.changeFilters({ status: ['enabled', 'disabled'] });
    result.current.changeFilters({ date: '2026-11-20' });
  });
  expect(requests.mock.calls.at(-1)?.[0]).toEqual({
    keyword: '',
    status: ['enabled', 'disabled'],
    date: '2026-11-20',
  });
  expect(result.current.draft.keyword).toBe('未搜索文字');
  expect(result.current.hasUnappliedChanges).toBe(true);
  await act(() => result.current.apply(result.current.draft));
  expect(result.current.applied.keyword).toBe('未搜索文字');
  expect(result.current.hasUnappliedChanges).toBe(false);
  act(() => {
    result.current.changeFilters({ status: [] });
  });
  expect(requests.mock.calls.at(-1)?.[0]).toMatchObject({ keyword: '未搜索文字', status: [] });
  await act(async () => {
    result.current.setDraft(defaults);
    await result.current.apply(defaults);
  });
  expect(result.current.applied).toEqual(defaults);
  expect(result.current.hasUnappliedChanges).toBe(false);
});

it('连续选择在前次查询未完成时保留全部选项，相同值不重复查询', async () => {
  const complete: (() => void)[] = [];
  const request = vi.fn<(values: typeof defaults) => Promise<unknown>>(
    () =>
      new Promise<void>((resolve) => {
        complete.push(resolve);
      }),
  );
  const { result } = renderHook(() => useFilterForm(defaults, request, defaults));
  act(() => {
    result.current.changeFilters({ status: ['enabled', 'disabled'] });
    result.current.changeFilters({ date: '2026-11-20' });
    result.current.changeFilters({ status: ['enabled', 'disabled'] });
  });
  expect(request).toHaveBeenCalledTimes(2);
  expect(request.mock.calls.at(-1)?.[0]).toMatchObject({
    status: ['enabled', 'disabled'],
    date: '2026-11-20',
  });
  await act(async () => {
    for (const resolve of complete) resolve();
    await Promise.resolve();
  });
});

it('文字草稿已清空时仍可撤销已应用条件，并保留其他未搜索草稿', async () => {
  const initial = { ...defaults, keyword: '已搜索文字' };
  const requests = vi.fn().mockResolvedValue(undefined);
  const { result } = renderHook(() => useFilterForm(initial, requests, initial));
  act(() => {
    result.current.setDraft({ ...initial, keyword: '', date: '2026-11-21' });
  });
  await act(() => result.current.applyChanges({ keyword: '' }));
  expect(requests).toHaveBeenCalledWith({ ...initial, keyword: '' });
  expect(result.current.draft).toEqual({ ...initial, keyword: '', date: '2026-11-21' });
});
