import { expect, it } from 'vitest';

import { readSpreadsheetImport, spreadsheetAccept } from '@/lib/spreadsheet';

it('仅接收 XLSX 并将二进制按 Base64 原样发送', async () => {
  const file = new File([new Uint8Array([0x50, 0x4b, 0x03, 0x04, 0x00, 0xff])], '模板.xlsx');
  expect(await readSpreadsheetImport(file, 1024)).toBe('UEsDBAD/');
  expect(spreadsheetAccept).not.toContain('csv');
  await expect(readSpreadsheetImport(new File(['学号,姓名'], '模板.csv'), 1024)).rejects.toThrow(
    '请选择 XLSX 文件',
  );
});
