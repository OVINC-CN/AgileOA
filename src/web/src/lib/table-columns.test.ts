import { act, renderHook } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';

import { useTableColumns } from '@/lib/table-columns';

const columns = [
  { id: 'name', label: '姓名' },
  { id: 'email', label: '邮箱' },
];

afterEach(() => {
  vi.restoreAllMocks();
  localStorage.clear();
});

it('仅保存隐藏标识，重新挂载及字段改名、排序后仍生效，新增字段默认显示', () => {
  const first = renderHook(() => useTableColumns('fixture-columns', columns));
  act(() => {
    first.result.current.setVisible('email', false);
  });
  expect(localStorage.getItem('table-hidden-columns:fixture-columns')).toBe('["email"]');
  first.unmount();

  const next = renderHook(() =>
    useTableColumns('fixture-columns', [
      { id: 'major', label: '专业' },
      { id: 'email', label: '联系邮箱' },
      { id: 'name', label: '姓名' },
    ]),
  );
  expect(next.result.current.visibleColumns.map((column) => column.id)).toEqual(['major', 'name']);
  expect(localStorage.getItem('table-hidden-columns:fixture-columns')).toBe('["email"]');
  act(() => {
    next.result.current.showAll();
  });
  expect(localStorage.getItem('table-hidden-columns:fixture-columns')).toBeNull();
  expect(next.result.current.visibleColumns).toHaveLength(3);
});

it('切换表格标识立即隔离配置，同标识实例及其他标签页同步选择', () => {
  const first = renderHook(() => useTableColumns('fixture-shared', columns));
  const second = renderHook(({ tableID }) => useTableColumns(tableID, columns), {
    initialProps: { tableID: 'fixture-shared' },
  });
  act(() => {
    first.result.current.setVisible('email', false);
  });
  expect(second.result.current.isVisible('email')).toBe(false);
  second.rerender({ tableID: 'fixture-other' });
  expect(second.result.current.isVisible('email')).toBe(true);
  act(() => {
    second.result.current.setVisible('name', false);
  });
  expect(first.result.current.isVisible('name')).toBe(true);

  act(() => {
    localStorage.setItem('table-hidden-columns:fixture-shared', '["name"]');
    window.dispatchEvent(
      new StorageEvent('storage', { key: 'table-hidden-columns:fixture-shared' }),
    );
  });
  expect(first.result.current.visibleColumns.map((column) => column.id)).toEqual(['email']);
  act(() => {
    localStorage.clear();
    window.dispatchEvent(new StorageEvent('storage', { key: null }));
  });
  expect(first.result.current.visibleColumns).toHaveLength(2);
  expect(second.result.current.visibleColumns).toHaveLength(2);
});

it.each(['{', '{}', '["email",7]', '[""]', '["name","email"]'])(
  '无效或隐藏全部字段的配置 %s 恢复可用展示',
  (value) => {
    localStorage.setItem('table-hidden-columns:fixture-invalid', value);
    const { result } = renderHook(() => useTableColumns('fixture-invalid', columns));
    expect(result.current.visibleColumns).toEqual(columns);
  },
);

it('保留暂时离开目录的隐藏字段，阻止隐藏最后一个可见字段', () => {
  localStorage.setItem('table-hidden-columns:fixture-directory', '["email","old_field"]');
  const { result, rerender } = renderHook(
    ({ fields }) => useTableColumns('fixture-directory', fields),
    { initialProps: { fields: columns } },
  );
  act(() => {
    result.current.setVisible('name', false);
  });
  expect(result.current.visibleColumns.map((column) => column.id)).toEqual(['name']);
  rerender({ fields: [{ id: 'name', label: '姓名' }] });
  rerender({ fields: columns });
  expect(result.current.isVisible('email')).toBe(false);
  expect(localStorage.getItem('table-hidden-columns:fixture-directory')).toBe(
    '["email","old_field"]',
  );
});

it('本地存储不可用时仍可切换字段并恢复全部', () => {
  const denied = () => {
    throw new DOMException('无法访问本地存储', 'SecurityError');
  };
  vi.spyOn(Storage.prototype, 'getItem').mockImplementation(denied);
  vi.spyOn(Storage.prototype, 'setItem').mockImplementation(denied);
  vi.spyOn(Storage.prototype, 'removeItem').mockImplementation(denied);
  const first = renderHook(() => useTableColumns('fixture-unavailable', columns));
  const second = renderHook(() => useTableColumns('fixture-unavailable', columns));
  act(() => {
    first.result.current.setVisible('email', false);
  });
  expect(first.result.current.isVisible('email')).toBe(false);
  expect(second.result.current.isVisible('email')).toBe(false);
  act(() => {
    second.result.current.showAll();
  });
  expect(first.result.current.visibleColumns).toHaveLength(2);
});
