import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';

import { AttachmentPreview } from '@/components/attachment-preview';
import {
  type AttachmentPreviewPosition,
  type AttachmentPreviewViewer,
  createAttachmentPreviewViewer,
} from '@/lib/attachment-preview';

vi.mock(import('@/lib/attachment-preview'), async (importOriginal) => ({
  ...(await importOriginal()),
  createAttachmentPreviewViewer: vi.fn(),
}));

afterEach(() => {
  vi.mocked(createAttachmentPreviewViewer).mockReset();
});

function mockViewer(position: AttachmentPreviewPosition) {
  const viewer = {
    load: vi.fn<AttachmentPreviewViewer['load']>(),
    goTo: vi.fn<AttachmentPreviewViewer['goTo']>(),
    fitWidth: vi.fn<AttachmentPreviewViewer['fitWidth']>(),
    fitPage: vi.fn<AttachmentPreviewViewer['fitPage']>(),
    rotate: vi.fn<AttachmentPreviewViewer['rotate']>(),
    zoomIn: vi.fn<AttachmentPreviewViewer['zoomIn']>(),
    zoomOut: vi.fn<AttachmentPreviewViewer['zoomOut']>(),
    destroy: vi.fn<AttachmentPreviewViewer['destroy']>(),
  };
  vi.mocked(createAttachmentPreviewViewer).mockImplementation((_kind, _host, onPosition) => {
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

it.each([
  ['材料.docx', 'application/vnd.openxmlformats-officedocument.wordprocessingml.document'],
  ['材料.pptx', 'application/vnd.openxmlformats-officedocument.presentationml.presentation'],
  ['材料.pdf', 'application/pdf'],
])('%s 使用公共按钮翻页、缩放，卸载时销毁解析资源', async (filename, contentType) => {
  const user = userEvent.setup();
  const viewer = mockViewer({ index: 0, total: 2 });
  const file = new File(['fixture'], filename, { type: contentType });
  const { unmount } = render(<AttachmentPreview file={file} contentType={contentType} />);
  const next = screen.getByRole('button', { name: '下一页' });
  await waitFor(() => expect(next).toBeEnabled());
  expect(screen.getByRole('button', { name: '上一页' })).toBeDisabled();
  expect(viewer.load).toHaveBeenCalledWith(file, contentType);
  await user.click(next);
  expect(viewer.goTo).toHaveBeenCalledWith(1);
  expect(next).toBeDisabled();
  await user.click(screen.getByRole('button', { name: '上一页' }));
  expect(viewer.goTo).toHaveBeenLastCalledWith(0);
  await user.click(screen.getByRole('button', { name: '放大' }));
  expect(viewer.zoomIn).toHaveBeenCalledOnce();
  await user.click(screen.getByRole('button', { name: '缩小' }));
  expect(viewer.zoomOut).toHaveBeenCalledOnce();
  await user.click(screen.getByRole('button', { name: '适应整页' }));
  expect(viewer.fitPage).toHaveBeenCalledOnce();
  await user.click(screen.getByRole('button', { name: '适应宽度' }));
  expect(viewer.fitWidth).toHaveBeenCalledOnce();
  unmount();
  expect(viewer.destroy).toHaveBeenCalledOnce();
});

it('Excel 使用公共选择器切换工作表', async () => {
  const user = userEvent.setup();
  const viewer = mockViewer({ index: 0, total: 2, names: ['学生名单', '统计汇总'] });
  render(
    <AttachmentPreview file={new File([], '材料.xls')} contentType="application/vnd.ms-excel" />,
  );
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
  let finish: ((value: AttachmentPreviewViewer) => void) | undefined;
  vi.mocked(createAttachmentPreviewViewer).mockReturnValue(
    new Promise((resolve) => {
      finish = resolve;
    }),
  );
  const { unmount } = render(
    <AttachmentPreview file={new File([], '材料.doc')} contentType="application/msword" />,
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
  render(
    <AttachmentPreview
      file={new File([], '材料.ppt')}
      contentType="application/vnd.ms-powerpoint"
    />,
  );
  await waitFor(() => expect(screen.getByRole('button', { name: '下一页' })).toBeEnabled());
  viewer.goTo.mockRejectedValue(new Error('renderer failure'));
  await user.click(screen.getByRole('button', { name: '下一页' }));
  expect(await screen.findByText('文件预览失败，请重试或下载原文件查看')).toBeVisible();
  expect(screen.getByText('无法显示预览，可下载原文件查看')).toBeVisible();
  expect(viewer.destroy).toHaveBeenCalledOnce();
});

it('图片使用公共按钮旋转和重置', async () => {
  const user = userEvent.setup();
  const viewer = mockViewer({ index: 0, total: 0, zoom: 1 });
  render(<AttachmentPreview file={new File([], '材料.png')} contentType="image/png" />);
  const rotate = screen.getByRole('button', { name: '顺时针旋转 90 度' });
  await waitFor(() => expect(rotate).toBeEnabled());
  await user.click(rotate);
  await user.click(screen.getByRole('button', { name: '重置' }));
  expect(viewer.rotate).toHaveBeenCalledOnce();
  expect(viewer.fitPage).toHaveBeenCalledOnce();
});
