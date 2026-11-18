import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { DatePicker } from '@/components/ui/date-picker';

describe('自建日期选择器', () => {
  it('使用自建日历，箭头可跨月移动，确认后返回自然日字符串', async () => {
    const change = vi.fn();
    const view = render(<DatePicker label="日期" value="2024-02-29" onValueChange={change} />);
    expect(view.container.querySelector('input[type="date"]')).toBeNull();
    const user = userEvent.setup();
    await user.click(screen.getByRole('combobox', { name: '日期' }));
    const selected = await screen.findByRole('button', { name: '2024年2月29日' });
    expect(selected).toHaveFocus();
    await user.keyboard('{ArrowRight}');
    expect(screen.getByRole('button', { name: '2024年3月1日' })).toHaveFocus();
    await user.keyboard('{Enter}');
    expect(change).toHaveBeenCalledWith('2024-03-01');
    expect(screen.queryByRole('dialog', { name: '日期选择' })).not.toBeInTheDocument();
  });

  it('月份选择使用自建面板，支持选择和清空', async () => {
    const change = vi.fn();
    const view = render(
      <DatePicker label="月份" mode="month" value="2026-11" onValueChange={change} />,
    );
    expect(view.container.querySelector('input[type="month"]')).toBeNull();
    const user = userEvent.setup();
    await user.click(screen.getByRole('combobox', { name: '月份' }));
    await user.click(screen.getByRole('button', { name: '2026年10月' }));
    expect(change).toHaveBeenCalledWith('2026-10');
    await user.click(screen.getByRole('combobox', { name: '月份' }));
    await user.click(screen.getByRole('button', { name: '清空' }));
    expect(change).toHaveBeenLastCalledWith('');
  });

  it('手工输入拒绝不存在的日期，错误仅由顶部 Message 显示', async () => {
    const change = vi.fn();
    render(<DatePicker label="日期" value="2026-02-01" required onValueChange={change} />);
    const user = userEvent.setup();
    await user.click(screen.getByRole('combobox', { name: '日期' }));
    expect(screen.getByRole('button', { name: '清空' })).toBeDisabled();
    const input = screen.getByRole('textbox', { name: '输入日期' });
    await user.clear(input);
    await user.type(input, '2026-02-30');
    await user.click(screen.getByRole('button', { name: '确定' }));
    expect(change).not.toHaveBeenCalled();
    expect(screen.getByRole('alert')).toHaveTextContent('请输入有效日期');
    await user.clear(input);
    await user.type(input, '2026-02-28');
    await user.click(screen.getByRole('button', { name: '确定' }));
    expect(change).toHaveBeenCalledWith('2026-02-28');
  });
});
