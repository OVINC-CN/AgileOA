import { z } from 'zod';

import { downloadFile } from '@/lib/download';
import { validateFileSize } from '@/lib/file-upload';

export const xlsxContentType = 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet';
export const spreadsheetAccept = `.xlsx,${xlsxContentType}`;
export const spreadsheetFileSchema = z.object({
  filename: z.string().endsWith('.xlsx'),
  content_type: z.literal(xlsxContentType),
  content: z.base64(),
});

export async function readSpreadsheetImport(file: File, maxBytes: number): Promise<string> {
  // 统一校验格式与大小，避免读取并发送空文件或超限文件。
  if (!/\.xlsx$/iu.test(file.name)) {
    throw new Error('请选择 XLSX 文件');
  }
  validateFileSize(file, maxBytes);

  // XLSX 直接传输二进制的 Base64，避免系统文本编码差异影响中文表头。
  const content = await new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      if (typeof reader.result !== 'string') {
        reject(new Error('文件读取失败，请重新选择'));
        return;
      }
      resolve(reader.result.slice(reader.result.indexOf(',') + 1));
    };
    reader.onerror = () => {
      reject(new Error('文件读取失败，请重新选择'));
    };
    reader.onabort = reader.onerror;
    reader.readAsDataURL(file);
  });
  return content;
}

export function saveSpreadsheetFile(file: z.infer<typeof spreadsheetFileSchema>) {
  const bytes = Uint8Array.from(atob(file.content), (character) => character.charCodeAt(0));
  downloadFile(file.filename, new Blob([bytes], { type: file.content_type }));
}

export async function downloadXLSX(filename: string, records: (string | number)[][]) {
  // 按需加载工作簿组件；数字也写成字符串，并为表头、数据和空列统一设置文本格式。
  const { default: ExcelJS } = await import('exceljs');
  const book = new ExcelJS.Workbook();
  const sheet = book.addWorksheet('数据');
  const columns = Math.max(0, ...records.map((row) => row.length));
  for (let index = 1; index <= columns; index++) {
    const column = sheet.getColumn(index);
    column.numFmt = '@';
    column.width = 20;
  }
  for (const record of records) {
    const row = sheet.addRow(record.map(String));
    row.eachCell({ includeEmpty: true }, (cell) => {
      cell.numFmt = '@';
    });
  }
  const bytes = new Uint8Array(await book.xlsx.writeBuffer());
  downloadFile(filename, new Blob([bytes], { type: xlsxContentType }));
}
