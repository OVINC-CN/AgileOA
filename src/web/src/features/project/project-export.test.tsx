import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';
import { afterEach, expect, it, vi } from 'vitest';

import {
  emptyFilters,
  exportRecords,
  participationListOptions,
  type ProjectExportType,
  statisticsOptions,
} from '@/features/project/api';
import { ProjectPage } from '@/features/project/project-page';
import * as spreadsheet from '@/lib/spreadsheet';

vi.mock('@/lib/spreadsheet', async (importOriginal) => ({
  ...(await importOriginal<typeof spreadsheet>()),
  saveSpreadsheetFile: vi.fn(),
}));

const workspaceID = 'w7m2k9p4s6n8c3x5';
const respond = (data: unknown) =>
  Response.json({ trace: '4bf92f3577b34da6a3ce929d0e0e4736', error_msg: '', data });
const filenames = { projects: '奖项.xlsx', students: '获奖学生.xlsx', details: '获奖明细.xlsx' };
const file = (type: ProjectExportType) => ({
  filename: filenames[type],
  content_type: spreadsheet.xlsxContentType,
  content: 'UEsDBAoAAAAAAA==',
});
const listData = { items: [], total: 0, page: 1, page_size: 5 };
const options = {
  names: [{ value: '虚构项目', label: '虚构项目' }],
  years: [{ value: '2026', label: '2026 年' }],
  education_levels: [
    { value: '硕士', label: '硕士' },
    { value: '博士', label: '博士' },
  ],
  majors: [{ value: '虚构专业', label: '虚构专业' }],
  grades: [{ value: '2026级', label: '2026级' }],
};

function renderProjects(actions: string[], tab = 'projects') {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter initialEntries={[`/stu/projects?workspace=${workspaceID}&tab=${tab}`]}>
        <ProjectPage
          workspaceID={workspaceID}
          workspaceName="虚构空间"
          actions={actions}
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

it.each([
  { tab: 'projects', actions: [], labels: [] },
  { tab: 'projects', actions: ['project.export'], labels: ['导出奖项'] },
  { tab: 'projects', actions: ['project.participation.export'], labels: [] },
  {
    tab: 'projects',
    actions: ['project.export', 'project.participation.export'],
    labels: ['导出奖项'],
  },
  { tab: 'participations', actions: [], labels: [] },
  { tab: 'participations', actions: ['project.export'], labels: [] },
  {
    tab: 'participations',
    actions: ['project.participation.export'],
    labels: ['导出获奖学生', '导出获奖明细'],
  },
  {
    tab: 'participations',
    actions: ['project.export', 'project.participation.export'],
    labels: ['导出获奖学生', '导出获奖明细'],
  },
])('按当前 Tab 和独立权限显示导出子菜单：$tab $actions', async ({ tab, actions, labels }) => {
  vi.stubGlobal(
    'fetch',
    vi.fn((path: string) =>
      Promise.resolve(respond(path.includes('/filter-options') ? options : listData)),
    ),
  );
  renderProjects(['project.list', 'project.participation.list', 'project.import', ...actions], tab);
  await waitFor(() => {
    expect(screen.getByRole('button', { name: '刷新' })).toBeEnabled();
  });
  if (!labels.length) {
    expect(screen.queryByRole('button', { name: '导出' })).not.toBeInTheDocument();
    return;
  }
  await userEvent.click(screen.getByRole('button', { name: '导出' }));
  expect(screen.getAllByRole('menuitem').map((item) => item.textContent)).toEqual(labels);
});

it.each([
  { type: 'projects' as const, label: '导出奖项', resource: 'projects', format: null },
  {
    type: 'students' as const,
    label: '导出获奖学生',
    resource: 'project-participations',
    format: 'basic',
  },
  {
    type: 'details' as const,
    label: '导出获奖明细',
    resource: 'project-participations',
    format: 'full',
  },
])('$label 使用已提交筛选且独立加载', async ({ type, label, resource, format }) => {
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
      if (path.includes('/projects/statistics?')) return Promise.resolve(respond([]));
      return Promise.resolve(respond(path.includes('/filter-options') ? options : listData));
    }),
  );
  renderProjects([
    type === 'projects' ? 'project.list' : 'project.participation.list',
    'project.export',
    'project.participation.export',
    'project.statistics',
  ]);
  const user = userEvent.setup();
  await user.click(screen.getByRole('combobox', { name: '项目名称' }));
  await user.click(await screen.findByRole('option', { name: '虚构项目' }));
  await user.click(screen.getByRole('combobox', { name: '归属年份' }));
  await user.click(await screen.findByRole('option', { name: '2026 年' }));
  await user.keyboard('{Escape}');
  if (type !== 'projects') {
    for (const [label, values] of [
      ['学历', ['硕士', '博士']],
      ['专业', ['虚构专业']],
      ['年级', ['2026级']],
    ] as const) {
      await user.click(screen.getByRole('combobox', { name: label }));
      for (const value of values) {
        await user.click(await screen.findByRole('option', { name: value }));
      }
      await user.keyboard('{Escape}');
    }
    await user.type(screen.getByRole('textbox', { name: '学号或姓名' }), '未提交草稿');
  }
  const download = screen.getByRole('button', { name: '导出' });
  await user.click(download);
  await user.click(await screen.findByRole('menuitem', { name: label }));
  expect(download).toHaveAttribute('aria-busy', 'true');
  expect(screen.getByRole('button', { name: '刷新' })).toBeEnabled();
  const request = new URL(
    requests.find((path) => path.includes('/export?')) ?? '',
    'https://example.com',
  );
  expect(request.pathname).toBe(`/v1/workspaces/${workspaceID}/${resource}/export`);
  expect(request.searchParams.getAll('project_name')).toEqual(['虚构项目']);
  expect(request.searchParams.getAll('year')).toEqual(['2026']);
  expect(request.searchParams.getAll('education_level')).toEqual(
    type === 'projects' ? [] : ['硕士', '博士'],
  );
  expect(request.searchParams.getAll('major')).toEqual(type === 'projects' ? [] : ['虚构专业']);
  expect(request.searchParams.getAll('grade')).toEqual(type === 'projects' ? [] : ['2026级']);
  expect(request.searchParams.get('format')).toBe(format);
  expect(request.searchParams.has('student')).toBe(false);
  expect(request.searchParams.has('page')).toBe(false);
  if (type !== 'projects') {
    const query = new URL(
      requests.filter((path) => path.includes('/project-participations?')).at(-1) ?? '',
      'https://example.com',
    ).searchParams;
    expect(query.getAll('education_level')).toEqual(['硕士', '博士']);
    expect(query.getAll('major')).toEqual(['虚构专业']);
    expect(query.getAll('grade')).toEqual(['2026级']);
    expect(requests.some((path) => path.includes('/projects/statistics?'))).toBe(false);
  }
  complete?.(respond(file(type)));
  await waitFor(() => {
    expect(spreadsheet.saveSpreadsheetFile).toHaveBeenCalledWith(file(type));
    expect(download).toBeEnabled();
    expect(download).not.toHaveAttribute('aria-busy', 'true');
  });
  if (type !== 'projects') {
    await user.click(screen.getByRole('button', { name: '重置' }));
    await waitFor(() => {
      const query = new URL(
        requests.filter((path) => path.includes('/project-participations?')).at(-1) ?? '',
        'https://example.com',
      ).searchParams;
      expect(query.has('education_level')).toBe(false);
      expect(query.has('major')).toBe(false);
      expect(query.has('grade')).toBe(false);
      expect(query.get('page')).toBe('1');
    });
  }
});

it('学生与明细共用列表筛选，项目导出只发送项目条件，文件名称经过校验', async () => {
  const requests: string[] = [];
  vi.stubGlobal(
    'fetch',
    vi.fn((path: string) => {
      requests.push(path);
      const type = path.includes('/projects/export')
        ? 'projects'
        : path.includes('format=full')
          ? 'details'
          : 'students';
      return Promise.resolve(
        respond(
          path.includes('/export?') ? file(type) : path.includes('/statistics?') ? [] : listData,
        ),
      );
    }),
  );
  const filters = {
    ...emptyFilters,
    project_name: ['项目甲', '项目乙'],
    year: ['2025', '2026'],
    student: '00000001',
    education_level: ['硕士', '博士'],
    major: ['虚构专业甲', '虚构专业乙'],
    grade: ['2025级', '2026级'],
    detail_keyword: '志愿服务',
    project_id: '7',
  };
  await exportRecords(workspaceID, 'students', filters);
  await exportRecords(workspaceID, 'details', filters);
  await new QueryClient().query(participationListOptions(workspaceID, filters, 3, 5));
  await new QueryClient().query(statisticsOptions(workspaceID, filters));
  const queries = requests.map((path) => new URL(path, 'https://example.com').searchParams);
  for (const query of queries) {
    query.delete('page');
    query.delete('page_size');
    query.delete('format');
  }
  expect(queries.map((query) => [...query])).toEqual([
    [...(queries[2] ?? [])],
    [...(queries[2] ?? [])],
    [...(queries[2] ?? [])],
    [...(queries[2] ?? [])],
  ]);
  await exportRecords(workspaceID, 'projects', filters);
  const awards = new URL(requests[4] ?? '', 'https://example.com').searchParams;
  expect([...awards.keys()]).toEqual(['project_name', 'project_name', 'year', 'year']);
  vi.stubGlobal(
    'fetch',
    vi.fn(() => Promise.resolve(respond(file('students')))),
  );
  await expect(exportRecords(workspaceID, 'details', filters)).rejects.toThrow();
});
