import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';

import { SearchSelect } from '@/components/ui/search-select';

describe('自定义下拉选择', () => {
  it('在原下拉内展开与收起，特殊规则置顶且两种视图排序一致，索引只跳转对应分组', async () => {
    const openChange = vi.fn();
    function Example() {
      const [value, setValue] = useState<string[]>(['beijing']);
      return (
        <SearchSelect
          multiple
          label="地区"
          value={value}
          onValueChange={setValue}
          onOpenChange={openChange}
          options={[
            { value: 'beijing', label: '北京' },
            { value: 'beta', label: 'beta' },
            { value: 'symbol', label: '☆地区' },
            { value: 'two', label: '2号' },
            { value: 'anhui', label: '安徽' },
            { value: 'apple', label: 'APPLE' },
            { value: 'exclude', label: '不含停用地区', pinned: true },
          ]}
        />
      );
    }
    render(<Example />);
    const user = userEvent.setup();
    const trigger = screen.getByRole('combobox', { name: '地区' });
    await user.click(trigger);
    const menu = screen.getByRole('dialog');
    const search = screen.getByRole('searchbox');
    const labels = () => screen.getAllByRole('option').map((option) => option.textContent);
    const ordered = ['全部地区', '不含停用地区', '2号', '安徽', 'APPLE', '北京', 'beta', '☆地区'];
    expect(labels()).toEqual(ordered);
    await user.click(screen.getByRole('button', { name: '展开地区选择器' }));
    expect(screen.getAllByRole('dialog')).toEqual([menu]);
    expect(screen.getByRole('searchbox')).toBe(search);
    expect(labels()).toEqual(ordered);
    expect(screen.getByRole('option', { name: '北京' })).toHaveAttribute('aria-selected', 'true');
    expect(openChange).toHaveBeenCalledExactlyOnceWith(true);
    const index = screen.getByRole('navigation', { name: '地区首字母索引' });
    expect(
      within(index)
        .getAllByRole('button')
        .map((button) => button.textContent),
    ).toEqual(['0–9', 'A', 'B', '#']);

    // 测试环境没有布局，模拟分组标题的滚动以验证索引不修改选值。
    const group = screen.getByRole('group', { name: 'B' });
    const heading = document.getElementById(group.getAttribute('aria-labelledby') ?? '');
    const scrollIntoView = vi.fn();
    if (heading) heading.scrollIntoView = scrollIntoView;
    await user.click(within(index).getByRole('button', { name: '跳转到 B' }));
    expect(scrollIntoView).toHaveBeenCalledExactlyOnceWith({ block: 'start' });
    expect(trigger).toHaveTextContent('北京');
    await user.type(search, 'APP');
    expect(labels()).toEqual(['APPLE']);
    expect(
      within(index)
        .getAllByRole('button')
        .map((button) => button.textContent),
    ).toEqual(['A']);
    await user.click(screen.getByRole('button', { name: '收起地区选择器' }));
    expect(screen.getByRole('dialog')).toBe(menu);
    expect(search).toHaveValue('APP');
    expect(trigger).toHaveTextContent('北京');
    expect(screen.queryByRole('navigation')).not.toBeInTheDocument();
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
    expect(openChange.mock.calls).toEqual([[true], [false]]);
  });

  it('月份按一月至十二月排序，展开后使用同一顺序和单个数字分组', async () => {
    const change = vi.fn();
    render(
      <SearchSelect
        multiple
        label="月份"
        value={[]}
        options={Array.from({ length: 12 }, (_, index) => ({
          value: String(12 - index),
          label: `${String(12 - index)} 月`,
        }))}
        onValueChange={change}
      />,
    );
    const user = userEvent.setup();
    const expected = [
      '全部月份',
      ...Array.from({ length: 12 }, (_, index) => `${String(index + 1)} 月`),
    ];
    await user.click(screen.getByRole('combobox', { name: '月份' }));
    expect(screen.getAllByRole('option').map((option) => option.textContent)).toEqual(expected);
    await user.click(screen.getByRole('button', { name: '展开月份选择器' }));
    expect(screen.getAllByRole('option').map((option) => option.textContent)).toEqual(expected);
    expect(screen.getAllByRole('group', { name: '0–9' })).toHaveLength(1);
    expect(screen.getByRole('button', { name: '跳转到 0–9' })).toBeVisible();
    await user.click(screen.getByRole('option', { name: '11 月' }));
    expect(change).toHaveBeenCalledExactlyOnceWith(['11']);
  });

  it.each([0, 12])('普通枚举只有 %s 项时同样展示展开入口', async (count) => {
    const change = vi.fn();
    render(
      <SearchSelect
        multiple
        label="编号"
        value={[]}
        options={[
          { value: 'exclude', label: '不含停用编号', pinned: true },
          { value: '', label: '未分类' },
          ...Array.from({ length: count }, (_, index) => ({
            value: String(count - index),
            label: String(count - index),
          })),
        ]}
        onValueChange={change}
      />,
    );
    const user = userEvent.setup();
    await user.click(screen.getByRole('combobox', { name: '编号' }));
    const expand = screen.getByRole('button', { name: '展开编号选择器' });
    const expected = [
      '全部编号',
      '不含停用编号',
      '未分类',
      ...Array.from({ length: count }, (_, index) => String(index + 1)),
    ];
    expect(screen.getAllByRole('option').map((option) => option.textContent)).toEqual(expected);
    expect(expand).toBeVisible();
    expect(screen.getByRole('searchbox').parentElement?.nextElementSibling).toBe(expand);
    await user.click(expand);
    expect(screen.getAllByRole('option').map((option) => option.textContent)).toEqual(expected);
    expect(screen.getByRole('button', { name: '收起编号选择器' })).toBeVisible();
    if (count > 0) {
      expect(screen.getAllByRole('group', { name: '0–9' })).toHaveLength(1);
      await user.click(screen.getByRole('option', { name: '12' }));
      expect(change).toHaveBeenCalledExactlyOnceWith(['12']);
    }
  });

  it('选项仅在文本被截断时悬停显示完整提示，仍可正常选择', async () => {
    const change = vi.fn();
    const label = '用于验证完整显示的较长工作空间名称';
    render(
      <SearchSelect
        label="工作空间"
        value=""
        options={[{ value: 'workspace', label }]}
        onValueChange={change}
      />,
    );
    const user = userEvent.setup();
    await user.click(screen.getByRole('combobox', { name: '工作空间' }));
    const option = screen.getByRole('option', { name: label });
    const text = option.querySelector('[data-tooltip-overflow]');

    // 测试环境没有实际布局，等待悬停后的宽度测量完成再检查是否显示提示。
    const scrollWidth = vi.fn(() => 100);
    Object.defineProperty(text, 'clientWidth', { configurable: true, value: 100 });
    Object.defineProperty(text, 'scrollWidth', { configurable: true, get: scrollWidth });
    await user.hover(option);
    await waitFor(() => {
      expect(scrollWidth).toHaveBeenCalled();
    });
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();

    await user.unhover(option);
    scrollWidth.mockReturnValue(300);
    await user.hover(option);
    expect(await screen.findByRole('tooltip')).toHaveTextContent(label);
    await user.click(option);
    expect(change).toHaveBeenCalledExactlyOnceWith('workspace');
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });

  it('多选保留弹层和搜索，支持键盘勾选、取消与恢复全部，空值可独立选择', async () => {
    function Example() {
      const [value, setValue] = useState<string[]>([]);
      return (
        <SearchSelect
          multiple
          label="类型"
          value={value}
          onValueChange={setValue}
          options={[
            { value: '', label: '待就业' },
            { value: 'employed', label: '就业' },
          ]}
        />
      );
    }
    render(<Example />);
    const user = userEvent.setup();
    const trigger = screen.getByRole('combobox', { name: '类型' });
    await user.click(trigger);
    expect(screen.getByRole('listbox')).toHaveAttribute('aria-multiselectable', 'true');
    await user.click(screen.getByRole('option', { name: '待就业' }));
    const search = screen.getByRole('searchbox');
    // 测试环境没有布局滚动实现，单独模拟目标选项以校验键盘导航的滚动调用。
    const scrollIntoView = vi.fn();
    screen.getByRole('option', { name: '就业' }).scrollIntoView = scrollIntoView;
    await user.type(search, '就业{ArrowDown}{Enter}');
    expect(scrollIntoView).toHaveBeenCalledExactlyOnceWith({ block: 'nearest' });
    expect(search).toHaveValue('就业');
    expect(screen.getByRole('option', { name: '待就业' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('option', { name: '就业' })).toHaveAttribute('aria-selected', 'true');
    expect(trigger).toHaveTextContent('待就业、就业');
    await user.click(screen.getByRole('option', { name: '待就业' }));
    expect(trigger).toHaveTextContent('就业');
    await user.clear(search);
    await user.click(screen.getByRole('option', { name: '全部类型' }));
    expect(trigger).toHaveTextContent('全部类型');
    expect(screen.getByRole('option', { name: '就业' })).toHaveAttribute('aria-selected', 'false');
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
    await user.click(trigger);
    expect(screen.getByRole('option', { name: '全部类型' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
  });

  it('使用按钮、可搜索选项和键盘选择，不触发系统下拉', async () => {
    const change = vi.fn();
    const searchChange = vi.fn();
    render(
      <SearchSelect
        label="工作空间状态"
        value=""
        options={[
          { value: '', label: '全部状态' },
          { value: 'enabled', label: '启用' },
          { value: 'disabled', label: '停用' },
        ]}
        onValueChange={change}
        onSearchChange={searchChange}
      />,
    );
    expect(screen.getByRole('combobox').tagName).toBe('BUTTON');
    await userEvent.click(screen.getByRole('combobox'));
    const search = screen.getByRole('searchbox', { name: '搜索工作空间状态' });
    expect(search).toHaveAttribute('data-1p-ignore', 'true');
    await userEvent.type(search, '停用');
    expect(screen.queryByRole('option', { name: '启用' })).not.toBeInTheDocument();
    expect(search.parentElement?.nextElementSibling).toBe(
      screen.getByRole('button', { name: '展开工作空间状态选择器' }),
    );
    await userEvent.clear(search);
    expect(search).toHaveValue('');
    expect(search).toHaveFocus();
    expect(screen.getByRole('option', { name: '启用' })).toBeVisible();
    expect(searchChange).toHaveBeenLastCalledWith('');
    expect(change).not.toHaveBeenCalled();
    await userEvent.type(search, '停用{Enter}');
    expect(change).toHaveBeenCalledWith('disabled');
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });

  it('文本维度允许选择已有名称或输入新建', async () => {
    const change = vi.fn();
    render(
      <SearchSelect
        label="专业"
        value=""
        options={[{ value: '虚构专业', label: '虚构专业' }]}
        allowCreate
        maxLength={100}
        onValueChange={change}
      />,
    );
    await userEvent.click(screen.getByRole('combobox'));
    await userEvent.type(screen.getByRole('searchbox'), '新专业');
    await userEvent.click(screen.getByRole('option', { name: '新建“新专业”' }));
    expect(change).toHaveBeenCalledWith('新专业');
  });
});

it('处理人多选可以隐藏全部入口并使用独立占位文案', async () => {
  const changed = vi.fn();
  render(
    <SearchSelect
      multiple
      showAllOption={false}
      label="第二级学生用户"
      placeholder="请选择学生用户"
      value={[]}
      options={[{ value: '7', label: '虚构学生' }]}
      onValueChange={changed}
    />,
  );
  const trigger = screen.getByRole('combobox', { name: '第二级学生用户' });
  expect(trigger).toHaveTextContent('请选择学生用户');
  await userEvent.click(trigger);
  expect(screen.queryByRole('option', { name: /全部/ })).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole('option', { name: '虚构学生' }));
  expect(changed).toHaveBeenCalledWith(['7']);
});
