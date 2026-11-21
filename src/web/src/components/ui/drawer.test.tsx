import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';

import { Drawer } from '@/components/ui/drawer';

afterEach(() => {
  vi.unstubAllGlobals();
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
