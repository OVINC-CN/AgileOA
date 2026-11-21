import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';
import { afterEach, expect, it, vi } from 'vitest';

import { ProjectPage } from '@/features/project/project-page';
import * as spreadsheet from '@/lib/spreadsheet';

vi.mock('@/lib/spreadsheet', async (original) => ({
  ...(await original<typeof spreadsheet>()),
  saveSpreadsheetFile: vi.fn(),
}));

const workspaceID = 'w7m2k9p4s6n8c3x5';
const respond = (data: unknown) =>
  Response.json({
    trace: '4bf92f3577b34da6a3ce929d0e0e4736',
    error_msg: '',
    data,
  });
const list = { items: [], total: 0, page: 1, page_size: 5 };
const filters = {
  names: [{ value: '优秀学生', label: '优秀学生' }],
  years: [{ value: '2026', label: '2026 年' }],
  education_levels: [],
  majors: [],
  grades: [],
};

function renderApplications(actions: string[]) {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter initialEntries={[`/stu/projects?workspace=${workspaceID}&tab=applications`]}>
        <ProjectPage
          workspaceID={workspaceID}
          workspaceName="虚构空间"
          actions={actions}
          canReviewApplications
          refresh={() => Promise.resolve()}
        />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

it('查看权限或审批节点身份不自动提供填报导出入口', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn((path: string) =>
      Promise.resolve(respond(path.includes('/filter-options') ? filters : list)),
    ),
  );
  renderApplications(['project.application.list']);
  await screen.findByText('暂无符合条件的申报单');
  expect(screen.queryByRole('button', { name: '导出' })).not.toBeInTheDocument();
});

it('导出使用已提交的项目、年份、审批状态筛选，忽略关键词草稿和分页，独立显示加载', async () => {
  const requests: string[] = [];
  let complete: ((value: Response) => void) | undefined;
  vi.stubGlobal(
    'fetch',
    vi.fn((path: string) => {
      requests.push(path);
      if (path.includes('/export?'))
        return new Promise<Response>((resolve) => {
          complete = resolve;
        });
      return Promise.resolve(respond(path.includes('/filter-options') ? filters : list));
    }),
  );
  renderApplications(['project.application.list', 'project.application.export']);
  await screen.findByText('暂无符合条件的申报单');
  const user = userEvent.setup();
  await user.type(screen.getByPlaceholderText('学号或姓名'), '未提交关键词');
  await user.click(screen.getByRole('combobox', { name: '项目名称' }));
  await user.click(await screen.findByRole('option', { name: '优秀学生' }));
  await user.keyboard('{Escape}');
  await user.click(screen.getByRole('combobox', { name: '归属年份' }));
  await user.click(await screen.findByRole('option', { name: '2026 年' }));
  await user.keyboard('{Escape}');
  await user.click(screen.getByRole('combobox', { name: '审批状态' }));
  await user.click(await screen.findByRole('option', { name: '已驳回' }));
  await user.keyboard('{Escape}');
  expect(screen.getByRole('status', { name: '筛选生效提示' })).toHaveTextContent('有未应用条件');
  const latestList = requests.filter((path) => path.includes('/project-applications?')).at(-1);
  expect(new URL(latestList ?? '', 'https://example.com').searchParams.has('student')).toBe(false);
  const button = screen.getByRole('button', { name: '导出' });
  await user.click(button);
  await user.click(await screen.findByRole('menuitem', { name: '导出填报明细' }));
  expect(button).toHaveAttribute('aria-busy', 'true');
  expect(screen.getByRole('button', { name: '刷新' })).toBeEnabled();
  const url = new URL(
    requests.find((path) => path.includes('/export?')) ?? '',
    'https://example.com',
  );
  expect(url.pathname).toBe(`/v1/workspaces/${workspaceID}/project-applications/export`);
  expect(url.searchParams.getAll('project_name')).toEqual(['优秀学生']);
  expect(url.searchParams.getAll('year')).toEqual(['2026']);
  expect(url.searchParams.getAll('status')).toEqual(
    expect.arrayContaining(['pending_mine', 'rejected']),
  );
  expect(url.searchParams.has('student')).toBe(false);
  expect(url.searchParams.has('page')).toBe(false);
  expect(url.searchParams.has('page_size')).toBe(false);
  const file = {
    filename: '填报明细.xlsx',
    content_type: spreadsheet.xlsxContentType,
    content: 'UEsDBAoAAAAAAA==',
  };
  complete?.(respond(file));
  await waitFor(() => {
    expect(spreadsheet.saveSpreadsheetFile).toHaveBeenCalledWith(file);
    expect(button).toBeEnabled();
    expect(button).not.toHaveAttribute('aria-busy', 'true');
  });
});
