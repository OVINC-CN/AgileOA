import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';

import { OfficePreview } from '@/components/office-preview';
import {
  createOfficePreviewViewer,
  type OfficePreviewPosition,
  type OfficePreviewViewer,
} from '@/lib/office-preview';

vi.mock(import('@/lib/office-preview'), async (importOriginal) => ({
  ...(await importOriginal()),
  createOfficePreviewViewer: vi.fn(),
}));

beforeEach(() => {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe = vi.fn();
      disconnect = vi.fn();
      unobserve = vi.fn();
    },
  );
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.mocked(createOfficePreviewViewer).mockReset();
});

function mockViewer(position: OfficePreviewPosition) {
  const viewer = {
    load: vi.fn<OfficePreviewViewer['load']>(),
    goTo: vi.fn<OfficePreviewViewer['goTo']>(),
    fitWidth: vi.fn<OfficePreviewViewer['fitWidth']>(),
    zoomIn: vi.fn<OfficePreviewViewer['zoomIn']>(),
    zoomOut: vi.fn<OfficePreviewViewer['zoomOut']>(),
    destroy: vi.fn<OfficePreviewViewer['destroy']>(),
  };
  vi.mocked(createOfficePreviewViewer).mockImplementation((_kind, _canvas, onPosition) => {
    viewer.load.mockImplementation(() => {
      onPosition(position);
      return Promise.resolve();
    });
    viewer.goTo.mockImplementation((index) => {
      onPosition({ ...position, index });
      return Promise.resolve();
    });
    return Promise.resolve(viewer);
  });
  return viewer;
}

it.each(['docx', 'pptx'] as const)(
  '%s 使用公共按钮翻页、缩放，卸载时销毁解析资源',
  async (kind) => {
    const user = userEvent.setup();
    const viewer = mockViewer({ index: 0, total: 2 });
    const { unmount } = render(<OfficePreview source="blob:office" filename="材料" kind={kind} />);
    const next = screen.getByRole('button', { name: '下一页' });
    await waitFor(() => expect(next).toBeEnabled());
    expect(screen.getByRole('button', { name: '上一页' })).toBeDisabled();
    expect(viewer.load).toHaveBeenCalledWith('blob:office');
    await user.click(next);
    expect(viewer.goTo).toHaveBeenCalledWith(1);
    expect(next).toBeDisabled();
    await user.click(screen.getByRole('button', { name: '上一页' }));
    expect(viewer.goTo).toHaveBeenLastCalledWith(0);
    await user.click(screen.getByRole('button', { name: '放大' }));
    expect(viewer.zoomIn).toHaveBeenCalledOnce();
    await user.click(screen.getByRole('button', { name: '缩小' }));
    expect(viewer.zoomOut).toHaveBeenCalledOnce();
    await user.click(screen.getByRole('button', { name: '适应宽度' }));
    expect(viewer.fitWidth).toHaveBeenCalledTimes(2);
    unmount();
    expect(viewer.destroy).toHaveBeenCalledOnce();
  },
);

it('Excel 使用公共选择器切换工作表', async () => {
  const user = userEvent.setup();
  const viewer = mockViewer({ index: 0, total: 2, names: ['学生名单', '统计汇总'] });
  render(<OfficePreview source="blob:office" filename="材料.xlsx" kind="xlsx" />);
  const sheets = screen.getByRole('combobox', { name: '工作表' });
  await waitFor(() => expect(sheets).toBeEnabled());
  await user.click(sheets);
  await user.click(screen.getByRole('option', { name: '统计汇总' }));
  expect(viewer.goTo).toHaveBeenCalledWith(1);
  expect(sheets).toHaveTextContent('统计汇总');
  expect(screen.queryByRole('button', { name: '下一页' })).not.toBeInTheDocument();
});

it('关闭时若解析器尚未加载完成，完成后立即销毁且不再读取文件', async () => {
  const viewer = mockViewer({ index: 0, total: 1 });
  let finish: ((value: OfficePreviewViewer) => void) | undefined;
  vi.mocked(createOfficePreviewViewer).mockReturnValue(
    new Promise((resolve) => {
      finish = resolve;
    }),
  );
  const { unmount } = render(
    <OfficePreview source="blob:office" filename="材料.doc" kind="docx" />,
  );
  unmount();
  await act(async () => {
    finish?.(viewer);
    await Promise.resolve();
  });
  expect(viewer.destroy).toHaveBeenCalledOnce();
  expect(viewer.load).not.toHaveBeenCalled();
});

it('后续页面解析失败时给出中文提示并释放资源', async () => {
  const user = userEvent.setup();
  const viewer = mockViewer({ index: 0, total: 2 });
  render(<OfficePreview source="blob:office" filename="材料.ppt" kind="pptx" />);
  await waitFor(() => expect(screen.getByRole('button', { name: '下一页' })).toBeEnabled());
  viewer.goTo.mockRejectedValue({ code: 'unsupported-content' });
  await user.click(screen.getByRole('button', { name: '下一页' }));
  expect(await screen.findByText('文件包含暂不支持的内容，请下载原文件查看')).toBeVisible();
  expect(screen.getByText('无法显示预览，可下载原文件查看')).toBeVisible();
  expect(viewer.destroy).toHaveBeenCalledOnce();
});
