import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';

import { emptyFilters, exportGraduations, graduationListOptions } from '@/features/graduation/api';
import { GraduationPage } from '@/features/graduation/graduation-page';
import * as spreadsheet from '@/lib/spreadsheet';

vi.mock('@/lib/spreadsheet', async (importOriginal) => ({
  ...(await importOriginal<typeof spreadsheet>()),
  saveSpreadsheetFile: vi.fn(),
}));

const workspaceID = 'w7m2k9p4s6n8c3x5';
const respond = (data: unknown) =>
  Response.json({ trace: '4bf92f3577b34da6a3ce929d0e0e4736', error_msg: '', data });
const file = {
  filename: '毕业去向.xlsx',
  content_type: spreadsheet.xlsxContentType,
  content: 'UEsDBAoAAAAAAA==',
};
const listData = { items: [], total: 0, page: 1, page_size: 5 };
const options = { education_levels: [], majors: [], grades: [], graduation_years: [], types: [] };

function renderGraduations(actions: string[]) {
  render(
    <QueryClientProvider client={new QueryClient()}>
      <GraduationPage
        workspaceID={workspaceID}
        workspaceName="虚构空间"
        actions={actions}
        refresh={() => Promise.resolve()}
      />
    </QueryClientProvider>,
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

it('查看及导入权限不能替代毕业去向导出权限', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn((path: string) =>
      Promise.resolve(respond(path.includes('/filter-options') ? options : listData)),
    ),
  );
  renderGraduations(['graduation.list', 'graduation.import']);
  await waitFor(() => {
    expect(screen.getByRole('button', { name: '刷新' })).toBeEnabled();
  });
  expect(screen.queryByRole('button', { name: '导出' })).not.toBeInTheDocument();
});

it('导出使用已提交的待就业筛选，保留关键词草稿并独立加载', async () => {
  let complete: ((value: Response) => void) | undefined;
  const requests: string[] = [];
  vi.stubGlobal(
    'fetch',
    vi.fn((path: string) => {
      requests.push(path);
      if (path.includes('/export?'))
        return new Promise<Response>((resolve) => {
          complete = resolve;
        });
      return Promise.resolve(respond(path.includes('/filter-options') ? options : listData));
    }),
  );
  renderGraduations(['graduation.list', 'graduation.export']);
  const user = userEvent.setup();
  await user.click(screen.getByRole('combobox', { name: '类型' }));
  await user.click(await screen.findByRole('option', { name: '待就业' }));
  await user.keyboard('{Escape}');
  await user.type(screen.getByRole('textbox', { name: '学号或姓名' }), '未提交草稿');
  const download = screen.getByRole('button', { name: '导出' });
  await user.click(download);
  expect(download).toHaveAttribute('aria-busy', 'true');
  expect(screen.getByRole('button', { name: '刷新' })).toBeEnabled();
  const request = new URL(
    requests.find((path) => path.includes('/export?')) ?? '',
    'https://example.com',
  );
  expect(request.pathname).toBe(`/v1/workspaces/${workspaceID}/graduation-destinations/export`);
  expect([...request.searchParams]).toEqual([['type', '']]);
  complete?.(respond(file));
  await waitFor(() => {
    expect(spreadsheet.saveSpreadsheetFile).toHaveBeenCalledWith(file);
    expect(download).not.toHaveAttribute('aria-busy', 'true');
    expect(download).toBeEnabled();
  });
});

it('导出与列表共用多选及显式空类型，全部类型省略参数，并验证文件契约', async () => {
  const requests: string[] = [];
  vi.stubGlobal(
    'fetch',
    vi.fn((path: string) => {
      requests.push(path);
      return Promise.resolve(respond(path.includes('/export?') ? file : listData));
    }),
  );
  const filters = {
    ...emptyFilters,
    keyword: '0001',
    education_level: ['本科', '硕士'],
    major: ['专业甲', '专业乙'],
    grade: ['2025级', '2026级'],
    graduation_year: ['2026', '2027'],
    type: ['', 'custom_type'],
  };
  await exportGraduations(workspaceID, filters);
  await new QueryClient().query(graduationListOptions(workspaceID, filters, 3, 5));
  const exported = new URL(requests[0] ?? '', 'https://example.com').searchParams;
  const listed = new URL(requests[1] ?? '', 'https://example.com').searchParams;
  listed.delete('page');
  listed.delete('page_size');
  expect([...exported]).toEqual([...listed]);
  expect(exported.getAll('type')).toEqual(['', 'custom_type']);
  await exportGraduations(workspaceID, emptyFilters);
  expect(new URL(requests[2] ?? '', 'https://example.com').searchParams.has('type')).toBe(false);
  vi.stubGlobal(
    'fetch',
    vi.fn(() => Promise.resolve(respond({ ...file, filename: '错误文件.csv' }))),
  );
  await expect(exportGraduations(workspaceID, filters)).rejects.toThrow();
});
