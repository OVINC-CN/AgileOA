import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { RefreshCw, Save } from 'lucide-react';
import { useState } from 'react';
import { afterEach, expect, it, vi } from 'vitest';

import { Button } from '@/components/ui/button';
import { Drawer } from '@/components/ui/drawer';

afterEach(() => {
  vi.unstubAllGlobals();
});

it('右上角显示无边框关闭图标，底部默认显示关闭文字，忙碌时均禁用', async () => {
  const onClose = vi.fn();
  const view = render(
    <Drawer title="详情" onClose={onClose} busy>
      正文
    </Drawer>,
  );
  const close = screen.getByRole('button', { name: '关闭' });
  const headerClose = screen.getByRole('button', { name: '关闭抽屉' });
  expect(close.closest('[data-slot="drawer-footer"]')).toBeInTheDocument();
  expect(headerClose.textContent).toBe('');
  expect(headerClose.querySelector('svg')).toBeVisible();
  expect(headerClose).toHaveClass('border-0');
  expect(close).toHaveClass('border');
  expect(close.querySelector('svg')).toBeNull();
  expect(headerClose).toBeDisabled();
  expect(close).toBeDisabled();
  fireEvent.click(headerClose);
  fireEvent.click(close);
  expect(onClose).not.toHaveBeenCalled();
  view.rerender(
    <Drawer title="详情" onClose={onClose}>
      正文
    </Drawer>,
  );
  fireEvent.click(close);
  await waitFor(() => {
    expect(onClose).toHaveBeenCalledOnce();
  });
  fireEvent.click(headerClose);
  await waitFor(() => {
    expect(onClose).toHaveBeenCalledTimes(2);
  });
});

it('关闭时禁用面板交互，并等面板和遮罩过渡结束后退出', async () => {
  const onClose = vi.fn();
  let finishPanel: (() => void) | undefined;
  let finishOverlay: (() => void) | undefined;
  const panelFinished = new Promise<void>((resolve) => {
    finishPanel = resolve;
  });
  const overlayFinished = new Promise<void>((resolve) => {
    finishOverlay = resolve;
  });
  function Example() {
    const [open, setOpen] = useState(true);
    return open ? (
      <Drawer
        title="编辑记录"
        dismissLabel="取消"
        onClose={() => {
          onClose();
          setOpen(false);
        }}
      >
        正文
      </Drawer>
    ) : null;
  }
  render(<Example />);
  const dialog = screen.getByRole('dialog', { name: '编辑记录' });
  const panel = dialog.querySelector('.drawer-panel');
  const overlay = dialog.previousElementSibling;
  if (!panel || !overlay) throw new Error('缺少抽屉面板或遮罩');
  Object.defineProperty(panel, 'getAnimations', {
    value: () => [{ finished: panelFinished }],
  });
  Object.defineProperty(overlay, 'getAnimations', {
    value: () => [{ finished: overlayFinished }],
  });

  fireEvent.click(within(dialog).getByRole('button', { name: '取消' }));
  expect(dialog).toHaveAttribute('data-closing', 'true');
  expect(overlay).toHaveAttribute('data-closing', 'true');
  expect(panel).toHaveAttribute('inert');
  expect(onClose).not.toHaveBeenCalled();
  await act(() => {
    finishOverlay?.();
    return overlayFinished;
  });
  expect(onClose).not.toHaveBeenCalled();
  await act(() => {
    finishPanel?.();
    return panelFinished;
  });
  expect(onClose).toHaveBeenCalledOnce();
  expect(dialog).not.toBeInTheDocument();
});

it('底部按钮按退出、刷新、业务操作排列', () => {
  const onCancel = vi.fn();
  render(
    <Drawer
      title="编辑记录"
      onClose={vi.fn()}
      footerDismiss={
        <Button variant="cancel" onClick={onCancel}>
          取消
        </Button>
      }
      footerRefresh={
        <Button variant="outline">
          <RefreshCw aria-hidden="true" />
          刷新
        </Button>
      }
      footer={
        <Button>
          <Save aria-hidden="true" />
          保存
        </Button>
      }
    >
      正文
    </Drawer>,
  );
  const cancel = screen.getByRole('button', { name: '取消' });
  const footer = cancel.closest<HTMLElement>('[data-slot="drawer-footer"]');
  if (!footer) throw new Error('缺少抽屉底部操作区');
  expect(
    within(footer)
      .getAllByRole('button')
      .map((button) => button.textContent),
  ).toEqual(['取消', '刷新', '保存']);
  expect(within(footer).getByRole('button', { name: '刷新' }).querySelector('svg')).toBeVisible();
  expect(within(footer).getByRole('button', { name: '保存' }).querySelector('svg')).toBeVisible();
  fireEvent.click(cancel);
  expect(onCancel).toHaveBeenCalledOnce();
});

it('可见视口缩小时同步收缩滚动区域，并跟随可见区域偏移', () => {
  const viewport = Object.assign(new EventTarget(), { height: 660, offsetTop: 0 });
  vi.stubGlobal('visualViewport', viewport);
  render(
    <Drawer
      title="导航菜单"
      description="虚构空间"
      size="full"
      scrollMode="viewport"
      onClose={vi.fn()}
    >
      <p>末尾菜单</p>
    </Drawer>,
  );
  const dialog = screen.getByRole('dialog', { name: '导航菜单' });
  expect(dialog).toHaveStyle({ height: '660px', top: '0px' });

  act(() => {
    viewport.height = 530;
    viewport.offsetTop = 40;
    viewport.dispatchEvent(new Event('resize'));
  });
  expect(dialog).toHaveStyle({ height: '530px', top: '40px' });

  act(() => {
    viewport.offsetTop = 56;
    viewport.dispatchEvent(new Event('scroll'));
  });
  expect(dialog).toHaveStyle({ height: '530px', top: '56px' });
});

it('缺少可见视口接口时使用窗口高度，表单抽屉同样随窗口调整', () => {
  vi.stubGlobal('visualViewport', undefined);
  vi.stubGlobal('innerHeight', 740);
  render(
    <Drawer title="编辑记录" description="虚构表单" onClose={vi.fn()} form={{ onSubmit: vi.fn() }}>
      <p>表单内容</p>
    </Drawer>,
  );
  const dialog = screen.getByRole('dialog', { name: '编辑记录' });
  expect(dialog).toHaveStyle({ height: '740px', top: '0px' });

  act(() => {
    vi.stubGlobal('innerHeight', 580);
    window.dispatchEvent(new Event('resize'));
  });
  expect(dialog).toHaveStyle({ height: '580px', top: '0px' });
});

it('标题与标识行仅在实际溢出时各自提示完整文本', () => {
  vi.useFakeTimers();
  try {
    const title = '记录名称 (001234567890)';
    const description = '关联名称 (009876543210)';
    render(
      <Drawer title={title} description={description} truncateDescription onClose={vi.fn()}>
        <p>正文</p>
      </Drawer>,
    );
    const heading = screen.getByRole('heading', { name: title });
    const subtitle = screen.getByText(description);
    const dialog = screen.getByRole('dialog', { name: title });
    expect(dialog).toHaveAccessibleDescription(description);

    Object.defineProperties(heading, {
      clientWidth: { value: 200, configurable: true },
      scrollWidth: { value: 200 },
    });
    Object.defineProperties(subtitle, {
      clientWidth: { value: 100 },
      scrollWidth: { value: 200 },
    });

    fireEvent.pointerMove(heading, { pointerType: 'mouse' });
    act(() => {
      vi.advanceTimersByTime(350);
    });
    expect(screen.queryByRole('tooltip', { hidden: true })).not.toBeInTheDocument();
    fireEvent.pointerOut(heading, { relatedTarget: document.body });

    Object.defineProperty(heading, 'clientWidth', { value: 100 });
    fireEvent.pointerMove(heading, { pointerType: 'mouse' });
    act(() => {
      vi.advanceTimersByTime(350);
    });
    expect(screen.getByRole('tooltip', { hidden: true })).toHaveTextContent(title);
    fireEvent.pointerOut(heading, { relatedTarget: document.body });

    fireEvent.pointerMove(subtitle, { pointerType: 'mouse' });
    act(() => {
      vi.advanceTimersByTime(350);
    });
    expect(screen.getByRole('tooltip', { hidden: true })).toHaveTextContent(description);
    const close = screen.getByRole('button', { name: '关闭抽屉' });
    expect(close).toBeVisible();
    expect(close).toHaveAttribute('data-variant', 'cancel');
    expect(close).toHaveClass('bg-transparent', 'disabled:bg-muted');
  } finally {
    vi.useRealTimers();
  }
});
