import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';

import { AttachmentFilename } from '@/components/attachment-filename';
import { downloadFile } from '@/lib/download';
import { previewContentType, readPreviewFile } from '@/lib/file-preview';
import { createOfficePreviewViewer } from '@/lib/office-preview';

vi.mock(import('@/lib/file-preview'), async (importOriginal) => ({
  ...(await importOriginal()),
  readPreviewFile: vi.fn(),
  previewContentType: vi.fn(),
}));
vi.mock(import('@/lib/office-preview'), async (importOriginal) => ({
  ...(await importOriginal()),
  createOfficePreviewViewer: vi.fn(),
}));
vi.mock('@/lib/download', () => ({ downloadFile: vi.fn() }));

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  vi.mocked(readPreviewFile).mockReset();
  vi.mocked(previewContentType).mockReset();
  vi.mocked(downloadFile).mockReset();
  vi.mocked(createOfficePreviewViewer).mockReset();
});

function objectURLs() {
  const create = vi.fn(() => 'blob:fixture-preview');
  const revoke = vi.fn();
  vi.stubGlobal(
    'URL',
    class extends URL {
      static override createObjectURL = create;
      static override revokeObjectURL = revoke;
    },
  );
  return { create, revoke };
}

it('下载复用预览窗口已读取的文件，关闭释放资源，再次打开重新请求下载接口', async () => {
  const user = userEvent.setup();
  const file = new File(['fixture'], '原始材料.pdf', { type: 'application/pdf' });
  vi.mocked(readPreviewFile).mockResolvedValue(file);
  vi.mocked(previewContentType).mockResolvedValue('application/pdf');
  const { revoke } = objectURLs();
  render(<AttachmentFilename filename="材料.pdf" downloadURL="/attachments/7/download" />);
  expect(screen.queryByRole('button', { name: /预览/u })).not.toBeInTheDocument();
  expect(screen.queryByRole('link')).not.toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: '材料.pdf' }));
  const frame = await screen.findByTitle('原始材料.pdf');
  expect(frame).toHaveAttribute('src', 'blob:fixture-preview');
  expect(frame.closest('.drawer-panel')).toHaveClass('w-full', 'max-w-none');
  const dialog = within(screen.getByRole('dialog', { name: '文件预览' }));
  const download = dialog.getByRole('button', { name: '下载' });
  const close = dialog.getByRole('button', { name: '关闭' });
  expect(close.compareDocumentPosition(download) & Node.DOCUMENT_POSITION_FOLLOWING).not.toBe(0);
  await user.click(download);
  await user.click(download);
  expect(downloadFile).toHaveBeenCalledTimes(2);
  expect(downloadFile).toHaveBeenLastCalledWith('原始材料.pdf', file);
  expect(readPreviewFile).toHaveBeenCalledOnce();
  const signal = vi.mocked(readPreviewFile).mock.calls[0]?.[1];
  await user.click(close);
  await waitFor(() => {
    expect(revoke).toHaveBeenCalledWith('blob:fixture-preview');
  });
  expect(signal?.aborted).toBe(true);

  await user.click(screen.getByRole('button', { name: '材料.pdf' }));
  expect(await screen.findByTitle('原始材料.pdf')).toBeVisible();
  expect(readPreviewFile).toHaveBeenCalledTimes(2);
});

it('其他格式提示不支持预览，但仍可保存已经读取的 Blob', async () => {
  const user = userEvent.setup();
  const file = new File(['fixture'], '历史材料.zip');
  vi.mocked(readPreviewFile).mockResolvedValue(file);
  vi.mocked(previewContentType).mockResolvedValue(null);
  render(<AttachmentFilename filename={file.name} downloadURL="/attachments/9/download" />);
  await user.click(screen.getByRole('button', { name: file.name }));
  expect(
    await screen.findByText('暂不支持预览此文件类型，支持 PDF、图片及 Office 文档'),
  ).toBeVisible();
  await user.click(screen.getByRole('button', { name: '下载' }));
  expect(downloadFile).toHaveBeenCalledWith(file.name, file);
  expect(readPreviewFile).toHaveBeenCalledOnce();
});

it('读取完成前禁用下载，关闭时取消请求', async () => {
  const user = userEvent.setup();
  vi.mocked(readPreviewFile).mockReturnValue(new Promise(() => undefined));
  render(<AttachmentFilename filename="材料.pdf" downloadURL="/attachments/7/download" />);
  await user.click(screen.getByRole('button', { name: '材料.pdf' }));
  expect(screen.getByRole('button', { name: '下载' })).toBeDisabled();
  const signal = vi.mocked(readPreviewFile).mock.calls[0]?.[1];
  const close = screen.getByRole('button', { name: '关闭' });
  expect(close).toHaveAttribute('data-variant', 'cancel');
  await user.click(close);
  expect(signal?.aborted).toBe(true);
  expect(downloadFile).not.toHaveBeenCalled();
});

it('没有文件读取入口时，文件名只读且不发起请求', () => {
  render(<AttachmentFilename filename="待完成材料.pdf" />);
  expect(screen.getByText('待完成材料.pdf')).toBeVisible();
  expect(screen.queryByRole('button')).not.toBeInTheDocument();
  expect(readPreviewFile).not.toHaveBeenCalled();
});

it('Office 文档复用已读取文件，预览失败和重试后仍可下载同一原文件', async () => {
  const user = userEvent.setup();
  const file = new File(['fixture'], '原始材料.docx');
  vi.mocked(readPreviewFile).mockResolvedValue(file);
  vi.mocked(previewContentType).mockResolvedValue(
    'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
  );
  const viewer = {
    load: vi.fn().mockRejectedValue({ code: 'unsupported-content' }),
    goTo: vi.fn(),
    fitWidth: vi.fn(),
    zoomIn: vi.fn(),
    zoomOut: vi.fn(),
    destroy: vi.fn(),
  };
  vi.mocked(createOfficePreviewViewer).mockResolvedValue(viewer);
  const { revoke } = objectURLs();
  render(<AttachmentFilename filename="材料.docx" downloadURL="/attachments/10/download" />);
  await user.click(screen.getByRole('button', { name: '材料.docx' }));
  expect(await screen.findByText('无法显示预览，可下载原文件查看')).toBeVisible();
  expect(createOfficePreviewViewer).toHaveBeenCalledWith(
    'docx',
    expect.any(HTMLCanvasElement),
    expect.any(Function),
    expect.any(Function),
  );
  await user.click(screen.getByRole('button', { name: '重新预览' }));
  await waitFor(() => {
    expect(viewer.load).toHaveBeenCalledTimes(2);
  });
  expect(viewer.load).toHaveBeenLastCalledWith('blob:fixture-preview');
  await user.click(screen.getByRole('button', { name: '下载' }));
  expect(downloadFile).toHaveBeenCalledWith(file.name, file);
  expect(readPreviewFile).toHaveBeenCalledOnce();
  await user.click(screen.getByRole('button', { name: '关闭' }));
  expect(revoke).toHaveBeenCalledWith('blob:fixture-preview');
});
