import { afterEach, expect, it } from 'vitest';

import { getConfig } from '@/lib/config';
import { uploadAccept, validateFileSize, validateUploadFile } from '@/lib/file-upload';
import { readSpreadsheetImport } from '@/lib/spreadsheet';

const original = window.AGILE_WORKSPACE_CONFIG;
afterEach(() => {
  window.AGILE_WORKSPACE_CONFIG = original;
});

it('所有文件入口共用 5 MB 上限，业务上限更大时也不能绕过', async () => {
  const limit = 5 * 1024 * 1024;
  const exact = new File([new Uint8Array(limit)], '材料.pdf', { type: 'application/pdf' });
  expect(() => {
    validateFileSize(exact);
  }).not.toThrow();
  const oversized = new File([new Uint8Array(limit + 1)], '导入.xlsx');
  expect(() => {
    validateFileSize(oversized, limit * 2);
  }).toThrow('单个文件不能超过 5 MB');
  await expect(readSpreadsheetImport(oversized, limit * 2)).rejects.toThrow(
    '单个文件不能超过 5 MB',
  );
});

it('默认只允许 PDF 和图片，缺少 MIME 时回退文件名，配置可收紧类型', () => {
  expect(uploadAccept()).toContain('image/*');
  expect(() => {
    validateUploadFile(new File(['fixture'], '材料.pdf'));
  }).not.toThrow();
  expect(() => {
    validateUploadFile(new File(['fixture'], '照片.png', { type: 'image/png' }));
  }).not.toThrow();
  expect(() => {
    validateUploadFile(new File(['fixture'], '材料.txt'));
  }).toThrow('文件类型不在允许上传的范围内');
  window.AGILE_WORKSPACE_CONFIG = {
    ...getConfig(),
    fileAllowedContentTypes: ['image/png'],
    fileContentTypeGroups: { 图片: ['image/png'] },
  };
  expect(uploadAccept()).toBe('image/png');
  expect(() => {
    validateUploadFile(new File(['fixture'], '材料.pdf'));
  }).toThrow('文件类型不在允许上传的范围内');
});

it('空文件在读取和上传之前被拒绝', () => {
  expect(() => {
    validateFileSize(new File([], '导入.json'));
  }).toThrow('文件不能为空');
});
