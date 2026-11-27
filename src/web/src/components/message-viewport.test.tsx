import { act, fireEvent, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { ErrorMessage } from '@/components/error-message';
import { Drawer } from '@/components/ui/drawer';
import { clearMessages, showMessage } from '@/lib/messages';

describe('顶部 Message', () => {
  it('常驻消息替换旧消息后不自动关闭，操作按钮可用且普通消息仍正常消失', () => {
    vi.useFakeTimers();
    try {
      const action = vi.fn();
      act(() => {
        showMessage('正在检测', 'fixture-update');
        vi.advanceTimersByTime(3000);
        showMessage('发现新版本', 'fixture-update', {
          durationMs: 0,
          dismissible: false,
          action: { label: '一键刷新', onClick: action },
        });
        showMessage('普通异常');
      });
      act(() => {
        vi.advanceTimersByTime(5000);
      });
      expect(screen.getAllByRole('alert')).toHaveLength(1);
      expect(screen.getByRole('alert')).toHaveTextContent('发现新版本');
      expect(screen.queryByRole('button', { name: '关闭消息' })).not.toBeInTheDocument();
      fireEvent.click(screen.getByRole('button', { name: '一键刷新' }));
      expect(action).toHaveBeenCalledOnce();
      act(() => {
        vi.advanceTimersByTime(60000);
      });
      expect(screen.getByRole('alert')).toHaveTextContent('发现新版本');
    } finally {
      act(clearMessages);
      vi.useRealTimers();
    }
  });

  it('消息分别在显示 5 秒后消失，新消息不会延长旧消息时间', () => {
    vi.useFakeTimers();
    try {
      act(() => {
        showMessage('第一条异常');
      });
      act(() => {
        vi.advanceTimersByTime(3000);
        showMessage('第二条异常');
      });
      expect(screen.getAllByRole('alert')).toHaveLength(2);
      act(() => {
        vi.advanceTimersByTime(1999);
      });
      expect(screen.getByText('第一条异常')).toBeInTheDocument();
      act(() => {
        vi.advanceTimersByTime(1);
      });
      expect(screen.queryByText('第一条异常')).not.toBeInTheDocument();
      expect(screen.getByText('第二条异常')).toBeInTheDocument();
      act(() => {
        vi.advanceTimersByTime(3000);
      });
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    } finally {
      act(clearMessages);
      vi.useRealTimers();
    }
  });

  it('同编号消息更新后重新计时，手动关闭会清理定时器', () => {
    vi.useFakeTimers();
    try {
      act(() => {
        showMessage('原异常', 'fixture-message');
      });
      act(() => {
        vi.advanceTimersByTime(3000);
        showMessage('更新异常', 'fixture-message');
      });
      act(() => {
        vi.advanceTimersByTime(2000);
      });
      expect(screen.getByRole('alert')).toHaveTextContent('更新异常');
      fireEvent.click(screen.getByRole('button', { name: '关闭消息' }));
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
      act(() => {
        showMessage('再次显示', 'fixture-message');
        vi.advanceTimersByTime(3000);
      });
      expect(screen.getByRole('alert')).toHaveTextContent('再次显示');
      act(() => {
        vi.advanceTimersByTime(2000);
      });
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    } finally {
      act(clearMessages);
      vi.useRealTimers();
    }
  });
  it('页面异常只在顶部显示，支持关闭且相同错误不重复堆叠', async () => {
    const view = render(
      <form aria-label="验收表单">
        <ErrorMessage>保存失败，请重试</ErrorMessage>
      </form>,
    );
    expect(await screen.findByRole('alert')).toHaveTextContent('保存失败，请重试');
    expect(
      within(screen.getByRole('form', { name: '验收表单' })).queryByRole('alert'),
    ).not.toBeInTheDocument();
    act(() => {
      showMessage('保存失败，请重试');
    });
    expect(screen.getAllByRole('alert')).toHaveLength(1);
    await userEvent.click(screen.getByRole('button', { name: '关闭消息' }));
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    view.rerender(
      <form aria-label="验收表单">
        <ErrorMessage>保存失败，请重试</ErrorMessage>
      </form>,
    );
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('抽屉错误也位于顶部，并保持关闭按钮可操作', async () => {
    render(
      <Drawer title="验收抽屉" description="虚构表单" onClose={vi.fn()}>
        <ErrorMessage>无法保存</ErrorMessage>
      </Drawer>,
    );
    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByRole('region', { name: '消息通知' })).toHaveClass('fixed', 'top-4');
    await userEvent.click(within(dialog).getByRole('button', { name: '关闭消息' }));
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('关闭嵌套抽屉后，顶部消息恢复到上一层抽屉且可关闭', async () => {
    const parent = (nested: boolean) => (
      <Drawer title="外层抽屉" description="虚构表单" onClose={vi.fn()}>
        <p>外层内容</p>
        {nested ? (
          <Drawer title="内层抽屉" description="虚构附件" onClose={vi.fn()}>
            <p>内层内容</p>
          </Drawer>
        ) : null}
      </Drawer>
    );
    const view = render(parent(true));
    act(() => {
      showMessage('嵌套操作失败');
    });
    const inner = screen.getByRole('dialog', { name: '内层抽屉' });
    expect(within(inner).getByRole('alert')).toHaveTextContent('嵌套操作失败');
    view.rerender(parent(false));
    const outer = screen.getByRole('dialog', { name: '外层抽屉' });
    expect(within(outer).getByRole('alert')).toHaveTextContent('嵌套操作失败');
    await userEvent.click(within(outer).getByRole('button', { name: '关闭消息' }));
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('必填校验使用可关闭的顶部中文消息', async () => {
    render(
      <form>
        <label htmlFor="fixture-required">学号</label>
        <input id="fixture-required" required />
      </form>,
    );
    fireEvent.invalid(screen.getByLabelText('学号'));
    expect(await screen.findByRole('alert')).toHaveTextContent('学号：请填写此项');
    await userEvent.click(screen.getByRole('button', { name: '关闭消息' }));
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('支持不同语义类型、固定紧凑宽度适配小屏及超长文本换行', async () => {
    act(() => {
      showMessage('已复制', undefined, { tone: 'success' });
      showMessage('发现新版本，请刷新页面', undefined, { tone: 'info' });
      showMessage('请检查较长的异常提示内容以确保自动换行展示'.repeat(3), undefined, {
        tone: 'warning',
      });
    });

    const viewport = screen.getByRole('region', { name: '消息通知' });
    expect(viewport).toHaveClass('inset-x-4', 'max-w-sm');
    const statuses = screen.getAllByRole('status');
    expect(statuses).toHaveLength(2);
    expect(statuses[0]).toHaveAttribute('data-tone', 'success');
    expect(statuses[0]).toHaveClass('w-full', 'border-success-border');
    expect(statuses[1]).toHaveAttribute('data-tone', 'info');
    expect(statuses[1]).toHaveClass('w-full', 'border-info-border');
    const warning = screen.getByRole('alert');
    expect(warning).toHaveAttribute('data-tone', 'warning');
    expect(warning).toHaveClass('w-full', 'border-warning-border');
    expect(within(warning).getByRole('button', { name: '关闭消息' })).toHaveClass('border-0');
    await userEvent.click(within(warning).getByRole('button', { name: '关闭消息' }));
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });
});
