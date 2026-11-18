import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { DateTimePicker } from '@/components/ui/date-time-picker';

describe('日期时间选择器', () => {
  it('单个入口在同一弹层选择日期和时分，跨月及键盘操作后一次回传 UTC', async () => {
    const change = vi.fn();
    const view = render(
      <DateTimePicker label="截止时间" value="2024-02-29T15:59:00Z" onValueChange={change} />,
    );
    const trigger = screen.getByRole('combobox', { name: '截止时间' });
    expect(screen.getAllByRole('combobox')).toHaveLength(1);
    expect(trigger).toHaveTextContent('2024年2月29日 23:59');
    expect(view.container.querySelector('input')).toBeNull();
    const user = userEvent.setup();
    await user.click(trigger);
    const picker = within(screen.getByRole('dialog', { name: '截止时间选择' }));
    expect(picker.getByRole('button', { name: '2024年2月29日' })).toHaveFocus();
    await user.keyboard('{ArrowRight}{Enter}');
    expect(picker.getByRole('textbox', { name: '输入日期' })).toHaveValue('2024-03-01');
    await user.click(picker.getByRole('option', { name: '00 时' }));
    await user.keyboard('{ArrowUp}');
    expect(picker.getByRole('option', { name: '00 时' })).toHaveFocus();
    await user.click(picker.getByRole('option', { name: '59 分' }));
    await user.keyboard('{Home}{ArrowDown}');
    expect(picker.getByRole('option', { name: '01 分' })).toHaveFocus();
    expect(picker.getByRole('option', { name: '01 分' })).toHaveAttribute('aria-selected', 'true');
    expect(change).not.toHaveBeenCalled();
    await user.click(picker.getByRole('button', { name: '确定' }));
    expect(change).toHaveBeenCalledExactlyOnceWith('2024-02-29T16:01:00.000Z');
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
  });

  it.each(['取消', 'Escape', '外部'] as const)(
    '%s关闭放弃草稿，重新打开读取当前值',
    async (dismiss) => {
      const change = vi.fn();
      const view = render(
        <>
          <DateTimePicker label="截止时间" value="2026-12-31T15:59:00Z" onValueChange={change} />
          <button type="button">外部</button>
        </>,
      );
      const user = userEvent.setup();
      const trigger = screen.getByRole('combobox', { name: '截止时间' });
      await user.click(trigger);
      await user.click(screen.getByRole('button', { name: '2026年12月30日' }));
      await user.click(screen.getByRole('option', { name: '12 时' }));
      if (dismiss === 'Escape') {
        await user.keyboard('{Escape}');
      } else {
        await user.click(screen.getByRole('button', { name: dismiss }));
      }
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      expect(change).not.toHaveBeenCalled();
      await user.click(trigger);
      expect(screen.getByRole('textbox', { name: '输入日期' })).toHaveValue('2026-12-31');
      expect(screen.getByRole('option', { name: '23 时' })).toHaveAttribute(
        'aria-selected',
        'true',
      );
      await user.keyboard('{Escape}');
      view.rerender(
        <DateTimePicker label="截止时间" value="2027-01-01T00:01:00Z" onValueChange={change} />,
      );
      await user.click(screen.getByRole('combobox', { name: '截止时间' }));
      expect(screen.getByRole('textbox', { name: '输入日期' })).toHaveValue('2027-01-01');
      expect(screen.getByRole('option', { name: '08 时' })).toHaveAttribute(
        'aria-selected',
        'true',
      );
      expect(screen.getByRole('option', { name: '01 分' })).toHaveAttribute(
        'aria-selected',
        'true',
      );
    },
  );

  it.each([
    { time: undefined, hour: '23', minute: '59', expected: '2026-12-31T23:59:00.000Z' },
    {
      time: { hour: '00', minute: '00' },
      hour: '00',
      minute: '00',
      expected: '2026-12-31T00:00:00.000Z',
    },
  ])(
    '空值采用默认 $hour:$minute，并遵循显式 UTC 时区',
    async ({ time, hour, minute, expected }) => {
      const change = vi.fn();
      render(
        <DateTimePicker
          label="时间"
          value={null}
          timeZone="UTC"
          {...(time ? { defaultTime: time } : {})}
          onValueChange={change}
        />,
      );
      const user = userEvent.setup();
      await user.click(screen.getByRole('combobox', { name: '时间' }));
      expect(screen.getByRole('button', { name: '清空' })).toBeDisabled();
      expect(screen.getByRole('option', { name: `${hour} 时` })).toHaveAttribute(
        'aria-selected',
        'true',
      );
      expect(screen.getByRole('option', { name: `${minute} 分` })).toHaveAttribute(
        'aria-selected',
        'true',
      );
      await user.type(screen.getByRole('textbox', { name: '输入日期' }), '2026-12-31');
      expect(screen.getByRole('button', { name: '2026年12月31日' })).toHaveAttribute(
        'aria-pressed',
        'true',
      );
      await user.keyboard('{Enter}');
      expect(change).toHaveBeenCalledExactlyOnceWith(expected);
    },
  );

  it('不存在的日期和夏令时跳变时间由 Message 提示，修正后才能确认', async () => {
    const change = vi.fn();
    render(
      <DateTimePicker
        label="时间"
        value="2026-03-08T06:30:00Z"
        timeZone="America/New_York"
        onValueChange={change}
      />,
    );
    const user = userEvent.setup();
    await user.click(screen.getByRole('combobox', { name: '时间' }));
    const date = screen.getByRole('textbox', { name: '输入日期' });
    await user.clear(date);
    await user.type(date, '2026-02-30');
    await user.click(screen.getByRole('button', { name: '确定' }));
    expect(screen.getByRole('alert')).toHaveTextContent('请输入有效日期');
    expect(change).not.toHaveBeenCalled();
    await user.clear(date);
    await user.type(date, '2026-03-08');
    await user.click(screen.getByRole('option', { name: '02 时' }));
    await user.click(screen.getByRole('button', { name: '确定' }));
    expect(
      screen
        .getAllByRole('alert')
        .some((alert) => alert.textContent.includes('所选时间在当前时区不存在')),
    ).toBe(true);
    expect(change).not.toHaveBeenCalled();
    expect(screen.getByRole('dialog', { name: '时间选择' })).toBeVisible();
    await user.click(screen.getByRole('option', { name: '03 时' }));
    await user.click(screen.getByRole('button', { name: '确定' }));
    expect(change).toHaveBeenCalledExactlyOnceWith('2026-03-08T07:30:00.000Z');
  });

  it('可选值可以清空，必填与禁用状态阻止相应操作', async () => {
    const change = vi.fn();
    const view = render(
      <DateTimePicker
        id="deadline"
        label="时间"
        value="2026-12-31T15:59:00Z"
        onValueChange={change}
      />,
    );
    const user = userEvent.setup();
    expect(screen.getByRole('combobox', { name: '时间' })).toHaveAttribute('id', 'deadline');
    await user.click(screen.getByRole('combobox', { name: '时间' }));
    await user.click(screen.getByRole('button', { name: '清空' }));
    expect(change).toHaveBeenCalledExactlyOnceWith(null);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    view.rerender(
      <DateTimePicker label="时间" value="2026-12-31T15:59:00Z" required onValueChange={change} />,
    );
    await user.click(screen.getByRole('combobox', { name: '时间' }));
    expect(screen.getByRole('button', { name: '清空' })).toBeDisabled();
    view.rerender(
      <DateTimePicker label="时间" value="2026-12-31T15:59:00Z" disabled onValueChange={change} />,
    );
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: '时间' })).toBeDisabled();
    await user.click(screen.getByRole('combobox', { name: '时间' }));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(change).toHaveBeenCalledOnce();
  });

  it('此刻及今天标记按指定时区计算，确认后才提交当前分钟', async () => {
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(new Date('2026-01-01T00:30:45Z'));
    try {
      const change = vi.fn();
      render(
        <DateTimePicker
          label="时间"
          value={null}
          timeZone="America/New_York"
          onValueChange={change}
        />,
      );
      const user = userEvent.setup();
      await user.click(screen.getByRole('combobox', { name: '时间' }));
      expect(screen.getByRole('button', { name: '2025年12月31日' })).toHaveFocus();
      await user.click(screen.getByRole('button', { name: '此刻' }));
      expect(screen.getByRole('textbox', { name: '输入日期' })).toHaveValue('2025-12-31');
      expect(screen.getByRole('option', { name: '19 时' })).toHaveAttribute(
        'aria-selected',
        'true',
      );
      expect(screen.getByRole('option', { name: '30 分' })).toHaveAttribute(
        'aria-selected',
        'true',
      );
      expect(change).not.toHaveBeenCalled();
      await user.click(screen.getByRole('button', { name: '确定' }));
      expect(change).toHaveBeenCalledExactlyOnceWith('2026-01-01T00:30:00.000Z');
    } finally {
      vi.useRealTimers();
    }
  });
});
