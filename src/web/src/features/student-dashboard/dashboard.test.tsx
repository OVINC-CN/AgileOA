import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';
import { afterEach, expect, it, vi } from 'vitest';

import {
  type AccommodationData,
  type AccommodationFilterOptions,
  dashboardOptions,
  type StudentDashboard,
} from '@/features/student-dashboard/api';
import { DashboardPage } from '@/features/student-dashboard/dashboard-page';
import * as spreadsheet from '@/lib/spreadsheet';
import { PageNavigationHarness } from '@/test/page-navigation-harness';

vi.mock('@/lib/spreadsheet', async (importOriginal) => ({
  ...(await importOriginal<typeof spreadsheet>()),
  saveSpreadsheetFile: vi.fn(),
}));

const workspaceID = 'w7m2k9p4s6n8c3x5';
const base = `/v1/workspaces/${workspaceID}/student-dashboard`;
const data: StudentDashboard = {
  majors: ['会计学', '审计学'],
  items: [
    {
      class_name: '一班',
      major_counts: { 会计学: 2 },
      total: 2,
      monitors: '虚构班长甲,虚构班长乙',
    },
    { class_name: '二班', major_counts: { 审计学: 1 }, total: 1, monitors: '' },
  ],
  totals: { major_counts: { 会计学: 2, 审计学: 1 }, total: 3 },
};
const accommodation: AccommodationData = {
  items: [
    {
      accommodation_type: { value: 'on_campus', label: '校内' },
      campus: '虚构园区',
      building: '1号楼',
      room: '0101',
      count: 2,
    },
    {
      accommodation_type: { value: 'on_campus', label: '校内' },
      campus: '虚构园区',
      building: '1号楼',
      room: '0102',
      count: 1,
    },
    {
      accommodation_type: { value: 'off_campus', label: '校外' },
      campus: '',
      building: '',
      room: '',
      count: 3,
    },
  ],
  total: 6,
};
const filterOptions: AccommodationFilterOptions = {
  accommodation_types: [
    { value: 'on_campus', label: '校内' },
    { value: 'off_campus', label: '校外' },
  ],
  campuses: [{ value: '虚构园区', label: '虚构园区' }],
  buildings: [{ value: '1号楼', label: '1号楼' }],
  rooms: [
    { value: '0101', label: '0101' },
    { value: '0102', label: '0102' },
  ],
};
const initialRequests = [base, `${base}/accommodation?`, `${base}/accommodation/filter-options`];

function dashboardData(path: string) {
  const url = new URL(path, 'https://example.com');
  if (url.pathname.endsWith('/filter-options')) return filterOptions;
  if (url.pathname.endsWith('/accommodation')) {
    const items = accommodation.items.filter((item) =>
      ['accommodation_type', 'campus', 'building', 'room'].every((key) => {
        const selected = url.searchParams.getAll(key);
        const value =
          key === 'accommodation_type'
            ? item.accommodation_type.value
            : item[key as 'campus' | 'building' | 'room'];
        return selected.length === 0 || selected.includes(value);
      }),
    );
    return { items, total: items.reduce((sum, item) => sum + item.count, 0) };
  }
  return data;
}

const respond = (value: unknown, status = 200) =>
  Response.json(
    {
      trace: '4bf92f3577b34da6a3ce929d0e0e4736',
      error_msg: status === 200 ? '' : '没有此工作空间的操作权限',
      data: value,
    },
    { status },
  );

function renderDashboard(actions: string[], entry = `/stu/dashboard?workspace=${workspaceID}`) {
  const client = new QueryClient();
  const view = render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[entry]}>
        <PageNavigationHarness workspaceID={workspaceID} actions={actions}>
          <DashboardPage
            workspaceID={workspaceID}
            workspaceName="虚构空间"
            actions={actions}
            refresh={() => Promise.resolve()}
          />
        </PageNavigationHarness>
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return { ...view, client };
}

afterEach(() => {
  localStorage.clear();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

it('学生、党建仪表和导出权限不替代学生仪表查看权限', () => {
  const fetchMock = vi.fn();
  vi.stubGlobal('fetch', fetchMock);
  renderDashboard(['student.list', 'cpc.dashboard.list', 'student.dashboard.export']);
  expect(screen.getByText('暂无查看数据仪表的权限')).toBeVisible();
  expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
  expect(screen.queryByRole('table')).not.toBeInTheDocument();
  expect(fetchMock).not.toHaveBeenCalled();
});

it('导航子菜单默认展示专业表并支持切换，人数按班级和专业汇总且保留空值及合计', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn((path: string) => Promise.resolve(respond(dashboardData(path)))),
  );
  renderDashboard(['student.dashboard.list']);
  const summaryTable = await screen.findByRole('table', { name: '学生专业表' });
  expect(summaryTable).toHaveAttribute('data-pin-first-column');
  const table = within(summaryTable);
  await table.findByRole('rowheader', { name: '一班' });
  expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
  const tabs = within(screen.getByRole('group', { name: '数据仪表子菜单' }));
  expect(tabs.getByRole('link', { name: '学生专业表', current: 'page' })).toBeVisible();
  expect(tabs.getByRole('link', { name: '住宿信息表' })).not.toHaveAttribute('aria-current');
  expect(screen.getByRole('navigation', { name: '面包屑' })).toHaveTextContent(
    /^学生工作\/数据仪表\/学生专业表$/,
  );
  expect(tabs.getByRole('link', { name: '学生专业表' })).toHaveAttribute(
    'href',
    `/stu/dashboard?workspace=${workspaceID}&tab=majors`,
  );
  expect(tabs.getByRole('link', { name: '住宿信息表' })).toHaveAttribute(
    'href',
    `/stu/dashboard?workspace=${workspaceID}&tab=accommodation`,
  );
  expect(screen.getByText('在籍学生')).toBeVisible();
  expect(screen.queryByRole('table', { name: '住宿信息表' })).not.toBeInTheDocument();
  expect(table.getAllByRole('columnheader').map((cell) => cell.textContent)).toEqual([
    '班级',
    '会计学（2）',
    '审计学（1）',
    '总数（3）',
    '班长',
  ]);
  const rows = table.getAllByRole('row');
  const [, first, second, footer] = rows;
  if (!first || !second || !footer) throw new Error('缺少班级或合计行');
  expect(
    within(first)
      .getAllByRole('cell')
      .map((cell) => cell.textContent),
  ).toEqual(['2', '', '2', '虚构班长甲,虚构班长乙']);
  expect(within(second).getAllByRole('cell').at(-1)).toHaveTextContent('-');
  expect(
    within(footer)
      .getAllByRole('cell')
      .map((cell) => cell.textContent),
  ).toEqual(['合计', '2', '1', '3', '-']);
  expect(screen.queryByRole('button', { name: '导出' })).not.toBeInTheDocument();
  expect(dashboardOptions(workspaceID).queryKey).not.toEqual(
    dashboardOptions('x8n3m0q5t7p9d4y6').queryKey,
  );

  await userEvent.click(tabs.getByRole('link', { name: '住宿信息表' }));
  expect(screen.getByRole('region', { name: '住宿信息表' })).toBeVisible();
  expect(screen.getByRole('navigation', { name: '面包屑' })).toHaveTextContent(
    /^学生工作\/数据仪表\/住宿信息表$/,
  );
  expect(screen.queryByRole('table', { name: '学生专业表' })).not.toBeInTheDocument();
  await screen.findByRole('cell', { name: '0102' });
  await userEvent.click(tabs.getByRole('link', { name: '学生专业表' }));
  expect(tabs.getByRole('link', { name: '学生专业表' })).toHaveFocus();
  expect(screen.getByRole('region', { name: '学生专业表' })).toBeVisible();
  expect(screen.getByRole('navigation', { name: '面包屑' })).toHaveTextContent(
    /^学生工作\/数据仪表\/学生专业表$/,
  );
  expect(screen.queryByRole('table', { name: '住宿信息表' })).not.toBeInTheDocument();
});

it('导出直接下载当前专业表，隐藏专业列后仍导出完整表格并独立显示加载状态', async () => {
  const file = {
    filename: '学生专业表.xlsx',
    content_type: spreadsheet.xlsxContentType,
    content: 'UEsDBAoAAAAAAA==',
  };
  let complete: ((value: Response) => void) | undefined;
  const fetchMock = vi.fn((path: string) =>
    path.endsWith('/export')
      ? new Promise<Response>((resolve) => {
          complete = resolve;
        })
      : Promise.resolve(respond(dashboardData(path))),
  );
  vi.stubGlobal('fetch', fetchMock);
  renderDashboard(['student.dashboard.list', 'student.dashboard.export']);
  await screen.findByRole('rowheader', { name: '一班' });
  const settings = screen.getAllByRole('button', { name: '列显示' })[0];
  if (!settings) throw new Error('缺少专业表字段设置');
  await userEvent.click(settings);
  const drawer = within(screen.getByRole('dialog', { name: '列显示' }));
  await userEvent.click(drawer.getByRole('checkbox', { name: '会计学' }));
  await userEvent.keyboard('{Escape}');
  expect(screen.queryByRole('columnheader', { name: /^会计学/ })).not.toBeInTheDocument();
  const download = screen.getByRole('button', { name: '导出' });
  await userEvent.click(download);
  expect(screen.queryByRole('menu')).not.toBeInTheDocument();
  expect(download).not.toHaveAttribute('aria-haspopup');
  expect(download).toHaveAttribute('aria-busy', 'true');
  expect(screen.getByRole('button', { name: '刷新' })).toBeEnabled();
  complete?.(respond(file));
  await waitFor(() => {
    expect(spreadsheet.saveSpreadsheetFile).toHaveBeenCalledWith(file);
    expect(download).toBeEnabled();
    expect(download).not.toHaveAttribute('aria-busy', 'true');
  });
  expect(fetchMock.mock.calls.map(([path]) => path)).toEqual([
    ...initialRequests,
    `${base}/export`,
  ]);
});

it('刷新被拒绝后隐藏旧空间汇总并禁用导出', async () => {
  let denied = false;
  vi.stubGlobal(
    'fetch',
    vi.fn((path: string) =>
      Promise.resolve(denied ? respond(null, 403) : respond(dashboardData(path))),
    ),
  );
  renderDashboard(['student.dashboard.list', 'student.dashboard.export']);
  await screen.findByRole('rowheader', { name: '一班' });
  denied = true;
  await userEvent.click(screen.getByRole('button', { name: '刷新' }));
  await waitFor(() => {
    expect(screen.queryByRole('rowheader', { name: '一班' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '导出' })).toBeDisabled();
  });
});

it('链接直达住宿表，表头多选立即筛选，导出沿用四项条件并保留宿舍号前导零', async () => {
  const file = {
    filename: '住宿信息表.xlsx',
    content_type: spreadsheet.xlsxContentType,
    content: 'UEsDBAoAAAAAAA==',
  };
  const fetchMock = vi.fn((path: string) =>
    Promise.resolve(
      respond(
        new URL(path, 'https://example.com').pathname.endsWith('/accommodation/export')
          ? file
          : dashboardData(path),
      ),
    ),
  );
  vi.stubGlobal('fetch', fetchMock);
  renderDashboard(
    ['student.dashboard.list', 'student.dashboard.export'],
    `/stu/dashboard?workspace=${workspaceID}&tab=accommodation`,
  );
  const user = userEvent.setup();
  expect(screen.getByRole('link', { name: '住宿信息表', current: 'page' })).toBeVisible();
  const accommodationTable = screen.getByRole('table', { name: '住宿信息表' });
  expect(accommodationTable).toHaveAttribute('data-pin-first-column');
  let table = within(accommodationTable);
  expect(screen.queryByLabelText('已应用筛选')).not.toBeInTheDocument();
  await table.findByRole('cell', { name: '0102' });
  expect(table.getByLabelText('住宿类型共有2个唯一选项')).toHaveTextContent('（2）');
  expect(table.getByLabelText('宿舍号共有2个唯一选项')).toHaveTextContent('（2）');
  expect(table.getAllByRole('row')).toHaveLength(5);
  expect(table.getAllByRole('cell').every((cell) => !cell.hasAttribute('rowspan'))).toBe(true);
  expect(table.getAllByRole('cell', { name: '校内' })).toHaveLength(2);

  for (const [label, option] of [
    ['住宿类型', '校内'],
    ['住宿园区', '虚构园区'],
    ['宿舍楼', '1号楼'],
    ['宿舍号', '0101'],
  ] as const) {
    const trigger = table.getByRole('combobox', { name: label });
    expect(trigger.closest('th')).not.toBeNull();
    await user.click(trigger);
    await user.click(await screen.findByRole('option', { name: option }));
    await user.keyboard('{Escape}');
  }
  await waitFor(() => {
    expect(table.queryByRole('cell', { name: '0102' })).not.toBeInTheDocument();
  });
  expect(table.getByLabelText('住宿类型共有1个唯一选项')).toHaveTextContent('（1）');
  expect(table.getByLabelText('宿舍号共有1个唯一选项')).toHaveTextContent('（1）');
  expect(within(screen.getByLabelText('已应用筛选')).getAllByRole('button')).toHaveLength(4);
  expect(table.getByRole('combobox', { name: '宿舍号' })).toHaveTextContent('1');

  await user.click(screen.getByRole('link', { name: '学生专业表' }));
  await user.click(screen.getByRole('link', { name: '住宿信息表' }));
  table = within(screen.getByRole('table', { name: '住宿信息表' }));
  expect(table.queryByRole('cell', { name: '0102' })).not.toBeInTheDocument();
  expect(table.getByLabelText('宿舍号共有1个唯一选项')).toHaveTextContent('（1）');
  await user.click(screen.getByRole('button', { name: '导出' }));
  expect(screen.queryByRole('menu')).not.toBeInTheDocument();
  await waitFor(() => {
    expect(spreadsheet.saveSpreadsheetFile).toHaveBeenCalledWith(file);
  });
  const exported = fetchMock.mock.calls
    .map(([path]) => new URL(path, 'https://example.com'))
    .find((url) => url.pathname.endsWith('/accommodation/export'));
  expect(exported?.searchParams.getAll('accommodation_type')).toEqual(['on_campus']);
  expect(exported?.searchParams.getAll('campus')).toEqual(['虚构园区']);
  expect(exported?.searchParams.getAll('building')).toEqual(['1号楼']);
  expect(exported?.searchParams.getAll('room')).toEqual(['0101']);
  expect(fetchMock.mock.calls.map(([path]) => path)).not.toContain(`${base}/export`);

  // 隐藏列后仍显示生效条件，并可单独清除，不影响其他筛选。
  await user.click(screen.getByRole('button', { name: '列显示' }));
  await user.click(screen.getByRole('checkbox', { name: '宿舍号' }));
  await user.keyboard('{Escape}');
  expect(table.queryByRole('combobox', { name: '宿舍号' })).not.toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: '移除筛选 宿舍号：0101' }));
  const updated = fetchMock.mock.calls
    .map(([path]) => new URL(path, 'https://example.com'))
    .filter((url) => url.pathname.endsWith('/accommodation'))
    .at(-1);
  expect(updated?.searchParams.getAll('room')).toEqual([]);
  expect(updated?.searchParams.getAll('accommodation_type')).toEqual(['on_campus']);
  expect(updated?.searchParams.getAll('campus')).toEqual(['虚构园区']);
  expect(updated?.searchParams.getAll('building')).toEqual(['1号楼']);

  await user.click(screen.getByRole('button', { name: '列显示' }));
  await user.click(screen.getByRole('button', { name: '恢复全部' }));
  await user.keyboard('{Escape}');
  await table.findByRole('cell', { name: '0102' });
  for (const condition of ['住宿类型：校内', '住宿园区：虚构园区', '宿舍楼：1号楼']) {
    await user.click(screen.getByRole('button', { name: `移除筛选 ${condition}` }));
  }
  expect(screen.queryByLabelText('已应用筛选')).not.toBeInTheDocument();
  await table.findByRole('cell', { name: '0102' });
  expect(table.getByLabelText('宿舍号共有2个唯一选项')).toHaveTextContent('（2）');
  expect(table.getAllByRole('row')).toHaveLength(5);
});
