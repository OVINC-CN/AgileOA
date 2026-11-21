import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ValueType, Workbook } from 'exceljs';
import { afterEach, expect, it, vi } from 'vitest';

import { ChartDataTable } from '@/components/chart-data-table';
import { ChartTooltipTable } from '@/components/chart-tooltip-table';
import { xlsxContentType } from '@/lib/spreadsheet';

afterEach(() => {
  localStorage.removeItem('table-hidden-columns:project-statistics-data');
  localStorage.removeItem('table-hidden-columns:fixture-chart-tooltip');
});

it('固定提示字段不受历史隐藏偏好影响', () => {
  localStorage.setItem('table-hidden-columns:fixture-chart-tooltip', '["count"]');
  render(
    <ChartTooltipTable
      tableID="fixture-chart-tooltip"
      caption="2026 年"
      columns={[
        { id: 'project', label: '奖项' },
        { id: 'count', label: '人数' },
      ]}
      rows={[{ id: 'one', label: '虚构奖项', color: undefined, value: 2, cells: ['2'] }]}
      fixedColumns
    />,
  );
  expect(screen.getAllByRole('columnheader').map((header) => header.textContent)).toEqual([
    '奖项',
    '人数',
  ]);
  expect(screen.getByRole('cell', { name: '2' })).toBeVisible();
});

it('图表数据固定展示全部字段，下载 XLSX 以文本保留数值、前导零及公式前缀', async () => {
  localStorage.setItem('table-hidden-columns:project-statistics-data', '["project"]');
  const createObjectURL = vi.fn<(file: Blob) => string>(() => 'blob:fixture');
  vi.stubGlobal(
    'URL',
    class extends URL {
      static override createObjectURL = createObjectURL;
      static override revokeObjectURL = vi.fn();
    },
  );
  const fetch = vi.fn();
  vi.stubGlobal('fetch', fetch);
  const download = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (
    this: HTMLAnchorElement,
  ) {
    expect(this.download).toBe('奖项年度人数数据.xlsx');
  });
  render(
    <ChartDataTable
      caption="奖项年度人数数据"
      columns={[
        { id: 'year', label: '归属年份' },
        { id: 'project', label: '奖项' },
        { id: 'count', label: '人数' },
      ]}
      rows={[
        { id: '1', cells: [2026, '奖项,"一"\n补充说明', 3] },
        { id: '2', cells: [2026, ' =1+1', 0] },
        { id: '3', cells: ['2026-11', '00001', '2026-11-06'] },
      ]}
    />,
  );
  const user = userEvent.setup();
  await user.click(screen.getByText('查看人数数据'));
  expect(screen.queryByRole('button', { name: '列显示' })).not.toBeInTheDocument();
  expect(screen.getAllByRole('columnheader').map((header) => header.textContent)).toEqual([
    '归属年份',
    '奖项',
    '人数',
  ]);
  expect(screen.getAllByRole('row')[1]).toHaveTextContent('2026奖项,"一" 补充说明3');
  await user.click(screen.getByRole('button', { name: '下载 XLSX' }));
  await waitFor(() => {
    expect(download).toHaveBeenCalledOnce();
  });
  expect(fetch).not.toHaveBeenCalled();

  const blob = createObjectURL.mock.lastCall?.[0];
  if (!blob) throw new Error('未生成 XLSX');
  expect(blob.type).toBe(xlsxContentType);
  const content = await new Promise<ArrayBuffer>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      if (reader.result instanceof ArrayBuffer) resolve(reader.result);
      else reject(new Error('XLSX 内容不是二进制'));
    };
    reader.onerror = () => {
      reject(reader.error ?? new Error('无法读取 XLSX'));
    };
    reader.readAsArrayBuffer(blob);
  });
  const book = new Workbook();
  await book.xlsx.load(content);
  const sheet = book.worksheets[0];
  if (!sheet) throw new Error('缺少数据工作表');
  expect(sheet.getSheetValues()).toEqual([
    undefined,
    [undefined, '归属年份', '奖项', '人数'],
    [undefined, '2026', '奖项,"一"\n补充说明', '3'],
    [undefined, '2026', ' =1+1', '0'],
    [undefined, '2026-11', '00001', '2026-11-06'],
  ]);
  sheet.eachRow((row) => {
    row.eachCell((cell) => {
      expect(cell.type).toBe(ValueType.String);
      expect(cell.numFmt).toBe('@');
      expect(cell.formula).toBeUndefined();
    });
  });
});
