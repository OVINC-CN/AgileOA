import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';
import { afterEach, expect, it, vi } from 'vitest';

import { type CPCDashboard, dashboardOptions } from '@/features/cpc-dashboard/api';
import { DashboardPage } from '@/features/cpc-dashboard/dashboard-page';
import * as spreadsheet from '@/lib/spreadsheet';
import { PageNavigationHarness } from '@/test/page-navigation-harness';

vi.mock('@/lib/spreadsheet', async (importOriginal) => ({
  ...(await importOriginal<typeof spreadsheet>()),
  saveSpreadsheetFile: vi.fn(),
}));

const workspaceID = 'w7m2k9p4s6n8c3x5';
const base = `/v1/workspaces/${workspaceID}/cpc-dashboard`;
const counts = { full_members: 2, probationary_members: 0, total: 2 };
const data: CPCDashboard = {
  items: [
    {
      ...counts,
      party_branch_id: 1,
      party_branch_name: '虚构支部',
      secretaries: '虚构书记',
      organization_members: '',
      publicity_members: '虚构委员',
      discipline_members: '',
      advisors: '导师甲,导师乙',
    },
  ],
  totals: counts,
};
const respond = (value: unknown, status = 200) =>
  Response.json(
    {
      trace: '4bf92f3577b34da6a3ce929d0e0e4736',
      error_msg: status === 200 ? '' : '没有此工作空间的操作权限',
      data: value,
    },
    { status },
  );

function renderDashboard(actions: string[]) {
  const client = new QueryClient();
  const view = render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[`/cpc/dashboard?workspace=${workspaceID}`]}>
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
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

it('人员、支部和下载权限不替代仪表查看权限', () => {
  const fetchMock = vi.fn();
  vi.stubGlobal('fetch', fetchMock);
  renderDashboard(['cpc.person.statistics', 'cpc.party_branch.list', 'cpc.dashboard.export']);
  expect(screen.getByText('暂无查看数据仪表的权限')).toBeVisible();
  expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
  expect(screen.queryByRole('table')).not.toBeInTheDocument();
  expect(fetchMock).not.toHaveBeenCalled();
});

it('表格按参考列顺序将零人数留空，保留空值、英文逗号导师及表尾合计', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn(() => Promise.resolve(respond(data))),
  );
  renderDashboard(['cpc.dashboard.list']);
  const table = within(await screen.findByRole('table'));
  await table.findByRole('rowheader', { name: '虚构支部' });
  expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
  const tabs = within(screen.getByRole('group', { name: '数据仪表子菜单' }));
  expect(tabs.getByRole('link', { name: '支部党员表' })).toHaveAttribute(
    'href',
    `/cpc/dashboard?workspace=${workspaceID}&tab=branches`,
  );
  expect(tabs.getByRole('link', { name: '支部党员表', current: 'page' })).toBeVisible();
  expect(screen.getByRole('region', { name: '支部党员表' })).toBeVisible();
  expect(screen.getByRole('navigation', { name: '面包屑' })).toHaveTextContent(
    /^党建工作\/数据仪表\/支部党员表$/,
  );
  expect(screen.getByText('在编人员')).toBeVisible();
  expect(table.getAllByRole('columnheader').map((cell) => cell.textContent)).toEqual([
    '支部名称',
    '中共党员',
    '中共预备党员',
    '合计',
    '支部书记',
    '组织委员',
    '宣传委员',
    '纪律委员',
    '导师姓名',
  ]);
  const rows = table.getAllByRole('row');
  const row = rows[1];
  if (!row) throw new Error('缺少支部数据行');
  expect(row).toHaveTextContent('导师甲,导师乙');
  expect(within(row).getAllByRole('cell')[1]).toBeEmptyDOMElement();
  expect(within(row).getAllByText('-')).toHaveLength(2);
  expect(rows.at(-1)).toHaveTextContent('合计');
  expect(screen.queryByRole('button', { name: '导出' })).not.toBeInTheDocument();
  expect(dashboardOptions(workspaceID).queryKey).not.toEqual(
    dashboardOptions('x8n3m0q5t7p9d4y6').queryKey,
  );
});

it('导出直接下载当前子 Tab 的表，使用独立接口并在请求结束后退出按钮加载状态', async () => {
  const file = {
    filename: '支部党员表.xlsx',
    content_type: spreadsheet.xlsxContentType,
    content: 'UEsDBAoAAAAAAA==',
  };
  let complete: ((value: Response) => void) | undefined;
  const fetchMock = vi.fn((path: string) =>
    path.endsWith('/export')
      ? new Promise<Response>((resolve) => {
          complete = resolve;
        })
      : Promise.resolve(respond(data)),
  );
  vi.stubGlobal('fetch', fetchMock);
  renderDashboard(['cpc.dashboard.list', 'cpc.dashboard.export']);
  await screen.findByRole('rowheader', { name: '虚构支部' });
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
  expect(fetchMock.mock.calls.map(([path]) => path)).toEqual([base, `${base}/export`]);
});

it('刷新被拒绝后立即隐藏此前读取的汇总和下载入口数据', async () => {
  const fetchMock = vi
    .fn()
    .mockResolvedValueOnce(respond(data))
    .mockResolvedValueOnce(respond(null, 403));
  vi.stubGlobal('fetch', fetchMock);
  renderDashboard(['cpc.dashboard.list', 'cpc.dashboard.export']);
  await screen.findByRole('rowheader', { name: '虚构支部' });
  await userEvent.click(screen.getByRole('button', { name: '刷新' }));
  await waitFor(() => {
    expect(screen.queryByRole('rowheader', { name: '虚构支部' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '导出' })).toBeDisabled();
  });
});
