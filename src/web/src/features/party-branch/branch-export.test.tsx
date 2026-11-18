import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';

import { exportPartyBranches, partyBranchListOptions } from '@/features/party-branch/api';
import { BranchPage } from '@/features/party-branch/branch-page';
import * as spreadsheet from '@/lib/spreadsheet';

vi.mock('@/lib/spreadsheet', async (importOriginal) => ({
  ...(await importOriginal<typeof spreadsheet>()),
  saveSpreadsheetFile: vi.fn(),
}));

const workspaceID = 'w7m2k9p4s6n8c3x5';
const base = `/v1/workspaces/${workspaceID}/cpc-party-branches`;
const respond = (data: unknown, status = 200) =>
  Response.json(
    {
      trace: '4bf92f3577b34da6a3ce929d0e0e4736',
      error_msg: status === 200 ? '' : '无权查看当前空间数据',
      data,
    },
    { status },
  );
const file = {
  filename: 'party-branches.xlsx',
  content_type: spreadsheet.xlsxContentType,
  content: 'UEsDBAoAAAAAAA==',
};
const listData = { items: [], total: 0, page: 1, page_size: 5 };

function renderBranches(actions: string[]) {
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <BranchPage
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

it.each([
  ['cpc.party_branch.list', 'cpc.party_branch.import', 'cpc.party_branch.update'],
  ['cpc.party_branch.export'],
])('党支部列表或导出权限不齐全时隐藏导出入口：%j', async (...actions) => {
  vi.stubGlobal(
    'fetch',
    vi.fn(() => Promise.resolve(respond(listData))),
  );
  renderBranches(actions);
  await waitFor(() => {
    expect(screen.getByRole('button', { name: '刷新' })).toBeEnabled();
  });
  expect(screen.queryByRole('button', { name: '导出' })).not.toBeInTheDocument();
});

it('导出沿用已提交关键词，保留草稿并独立显示加载状态', async () => {
  const requests: string[] = [];
  let complete: ((response: Response) => void) | undefined;
  vi.stubGlobal(
    'fetch',
    vi.fn((path: string) => {
      requests.push(path);
      if (path.includes('/export?')) {
        return new Promise<Response>((resolve) => {
          complete = resolve;
        });
      }
      return Promise.resolve(respond(listData));
    }),
  );
  renderBranches(['cpc.party_branch.list', 'cpc.party_branch.export']);
  const user = userEvent.setup();
  const keyword = screen.getByRole('textbox', { name: '简称、全称或管理员' });
  await user.type(keyword, '  虚构支部  {Enter}');
  await waitFor(() => {
    expect(screen.getByRole('button', { name: '搜索' })).toBeEnabled();
  });
  await user.clear(keyword);
  await user.type(keyword, '尚未提交的草稿');
  const download = screen.getByRole('button', { name: '导出' });
  await user.click(download);
  expect(download).toHaveAttribute('aria-busy', 'true');
  expect(download).toBeDisabled();
  expect(screen.getByRole('button', { name: '刷新' })).toBeEnabled();
  const request = new URL(
    requests.find((path) => path.includes('/export?')) ?? '',
    'https://example.com',
  );
  expect(request.pathname).toBe(`${base}/export`);
  expect([...request.searchParams]).toEqual([['keyword', '虚构支部']]);
  expect(keyword).toHaveValue('尚未提交的草稿');
  complete?.(respond(file));
  await waitFor(() => {
    expect(spreadsheet.saveSpreadsheetFile).toHaveBeenCalledWith(file);
    expect(download).not.toHaveAttribute('aria-busy', 'true');
    expect(download).toBeEnabled();
  });
});

it('列表与导出使用相同关键词，导出不携带分页并校验文件契约', async () => {
  const requests: string[] = [];
  vi.stubGlobal(
    'fetch',
    vi.fn((path: string) => {
      requests.push(path);
      return Promise.resolve(respond(path.includes('/export?') ? file : listData));
    }),
  );
  const keyword = '虚构支部 & 0001';
  await exportPartyBranches(workspaceID, keyword);
  await new QueryClient().query(partyBranchListOptions(workspaceID, keyword, 3, 5));
  const exported = new URL(requests[0] ?? '', 'https://example.com').searchParams;
  const listed = new URL(requests[1] ?? '', 'https://example.com').searchParams;
  listed.delete('page');
  listed.delete('page_size');
  expect([...exported]).toEqual([...listed]);
  vi.stubGlobal(
    'fetch',
    vi.fn(() => Promise.resolve(respond({ ...file, filename: '错误文件.csv' }))),
  );
  await expect(exportPartyBranches(workspaceID, keyword)).rejects.toThrow();
});

it('列表授权失效时禁用导出', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn(() => Promise.resolve(respond(null, 403))),
  );
  renderBranches(['cpc.party_branch.list', 'cpc.party_branch.export']);
  await waitFor(() => {
    expect(screen.getByRole('button', { name: '导出' })).toBeDisabled();
  });
  expect(spreadsheet.saveSpreadsheetFile).not.toHaveBeenCalled();
});
