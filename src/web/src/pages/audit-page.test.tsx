import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { AuditRow, AuditStatistics } from '@/features/management/api';
import { AuditPage } from '@/pages/audit-page';

vi.mock('@/components/management-charts', () => ({
  AuditCharts: ({ data }: { data: AuditStatistics | undefined }) => (
    <div role="status" aria-label="审计统计">
      {data?.total ?? 0}
    </div>
  ),
}));

const workspaceID = 'w7m2k9p4s6n8c3x5';
const deletedWorkspaceID = 't4f8a2n6k9c3p5s7';
const row: AuditRow = {
  id: '9223372036854775810',
  event_id: '12f8f7ba823d4a8eac12b5db99db71b4',
  event_date: '2026-11-03',
  workspace_id: workspaceID,
  event_content: '编辑党建人员',
  action_id: 'cpc.person.update',
  request_id: '23f8f7ba823d4a8eac12b5db99db71b4',
  user_id: '1',
  user_identify_type: 1,
  user_identify_tenant_id: 'system',
  start_time: Date.parse('2026-11-03T01:00:00Z'),
  end_time: Date.parse('2026-11-03T01:00:00.012Z'),
  access_type: 1,
  access_source_ip: '192.0.2.1',
  access_user_agent: 'fixture',
  resource_type_id: 'cpc_person',
  instance_id: '7',
  instance_name: null,
  result_code: 0,
  result_content: '',
  snapshot_user_info: { id: 1, username: 'fixture', display_name: '虚构用户', is_admin: true },
};

function renderAudit(keepPending = false, includeRecord = false) {
  const paths: string[] = [];
  vi.stubGlobal(
    'fetch',
    vi.fn((path: string) => {
      paths.push(path);
      const parameters = new URL(path, 'https://example.com').searchParams;
      if (keepPending && parameters.get('action_id') !== 'exclude_audit') {
        return new Promise<Response>(() => undefined);
      }
      const data = path.includes(`/logs/${row.event_id}?`)
        ? {
            ...row,
            instance_origin_data: { party_position: '', revision: 1 },
            instance_data: { party_position: '书记', revision: 2 },
            has_sensitive_snapshot: true,
            extend_data: { method: 'PUT', path: '/v1/workspaces/:workspace_id/cpc-people/:id' },
          }
        : path.includes('/statistics?')
          ? { total: 0, success: 0, failure: 0, denied: 0, days: [] }
          : {
              items: includeRecord ? [row] : [],
              total: includeRecord ? 1 : 0,
              page: 1,
              page_size: 5,
              start_time: Date.parse('2026-10-21T00:00:00Z'),
              end_time: Date.parse('2026-11-04T00:00:00Z'),
              user_tenants: [
                { value: 'system', label: '系统' },
                { value: workspaceID, label: `示例空间（${workspaceID}）` },
              ],
              result_codes: [
                { value: '0', label: '成功' },
                { value: '403', label: '403 无权限' },
              ],
              actions: [
                { value: 'exclude_audit', label: '不含审计' },
                { value: 'audit.read', label: '查看审计日志' },
                { value: 'audit.statistics', label: '查看审计统计' },
                { value: 'audit.detail', label: '查看审计详情' },
                { value: 'cpc.person.update', label: '编辑党建人员' },
              ],
              workspaces: [
                { value: 'system', label: '系统' },
                { value: workspaceID, label: `示例空间（${workspaceID}）` },
                { value: deletedWorkspaceID, label: `已删除工作空间（${deletedWorkspaceID}）` },
              ],
            };
      return Promise.resolve(
        new Response(
          JSON.stringify({
            trace: '4bf92f3577b34da6a3ce929d0e0e4736',
            error_msg: '',
            data,
          }),
        ),
      );
    }),
  );
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: 60_000 },
      mutations: { retry: false },
    },
  });
  render(
    <MemoryRouter>
      <QueryClientProvider client={client}>
        <AuditPage />
      </QueryClientProvider>
    </MemoryRouter>,
  );
  return paths;
}

function expectAction(paths: string[], action: string | string[]) {
  for (const endpoint of ['/logs?', '/statistics?']) {
    const path = paths.filter((value) => value.includes(endpoint)).at(-1);
    const parameters = new URL(path ?? '', 'https://example.com').searchParams;
    expect(parameters.has('action_id')).toBe(true);
    expect(parameters.getAll('action_id')).toEqual(Array.isArray(action) ? action : [action]);
  }
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('审计操作筛选', () => {
  it('用户、对象、请求编号经搜索精确提交，枚举变化不提交文本草稿', async () => {
    const paths = renderAudit();
    const user = userEvent.setup();
    await waitFor(() => {
      expect(screen.getByRole('button', { name: '搜索' })).toBeEnabled();
    });
    await user.click(screen.getByRole('button', { name: '更多筛选' }));
    const before = paths.length;
    await user.type(screen.getByRole('textbox', { name: '用户 ID' }), '18446744073709551615');
    await user.type(screen.getByRole('textbox', { name: '对象 ID' }), '0000123');
    await user.type(screen.getByRole('textbox', { name: '请求 ID' }), row.request_id);
    expect(paths).toHaveLength(before);
    await user.click(screen.getByRole('combobox', { name: '用户租户' }));
    await user.click(screen.getByRole('option', { name: '系统' }));
    await user.keyboard('{Escape}');
    await waitFor(() => {
      for (const endpoint of ['/logs?', '/statistics?']) {
        const path = paths.filter((value) => value.includes(endpoint)).at(-1);
        const query = new URL(path ?? '', 'https://example.com').searchParams;
        expect(query.getAll('user_identify_tenant_id')).toEqual(['system']);
        for (const key of ['user_id', 'instance_id', 'request_id'])
          expect(query.has(key)).toBe(false);
      }
    });
    expect(screen.getByRole('status', { name: '筛选生效提示' })).toHaveTextContent('有未应用条件');
    await user.click(screen.getByRole('button', { name: '搜索' }));
    await waitFor(() => {
      for (const endpoint of ['/logs?', '/statistics?']) {
        const path = paths.filter((value) => value.includes(endpoint)).at(-1);
        const query = new URL(path ?? '', 'https://example.com').searchParams;
        expect(query.get('user_id')).toBe('18446744073709551615');
        expect(query.get('instance_id')).toBe('0000123');
        expect(query.get('request_id')).toBe(row.request_id);
        expect(query.getAll('user_identify_tenant_id')).toEqual(['system']);
        expect(query.has('workspace_id')).toBe(false);
        expect(query.has('keyword')).toBe(false);
      }
    });
    await user.click(screen.getByRole('button', { name: '重置' }));
    expect(screen.getByRole('textbox', { name: '用户 ID' })).toHaveValue('');
    expect(screen.getByRole('textbox', { name: '对象 ID' })).toHaveValue('');
    expect(screen.getByRole('textbox', { name: '请求 ID' })).toHaveValue('');
    await waitFor(() => {
      expectAction(paths, 'exclude_audit');
    });
  });

  it('来源多选同步用于列表和统计，收起保留条件，重置恢复全部来源', async () => {
    const paths = renderAudit();
    const user = userEvent.setup();
    await waitFor(() => {
      expect(screen.getByRole('button', { name: '搜索' })).toBeEnabled();
    });
    await user.click(screen.getByRole('button', { name: '更多筛选' }));
    expect(screen.getByRole('combobox', { name: '操作来源' })).toHaveTextContent('全部来源');
    await user.click(screen.getByRole('combobox', { name: '操作来源' }));
    await user.click(screen.getByRole('option', { name: 'MCP' }));
    await user.keyboard('{Escape}');
    await waitFor(() => {
      for (const endpoint of ['/logs?', '/statistics?']) {
        const path = paths.filter((value) => value.includes(endpoint)).at(-1);
        expect(
          new URL(path ?? '', 'https://example.com').searchParams.getAll('access_type'),
        ).toEqual(['2']);
      }
    });
    await user.click(screen.getByRole('button', { name: '更多筛选' }));
    expect(screen.queryByRole('combobox', { name: '操作来源' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '更多筛选' }));
    expect(screen.getByRole('combobox', { name: '操作来源' })).toHaveTextContent('MCP');
    await user.click(screen.getByRole('combobox', { name: '操作来源' }));
    await user.click(screen.getByRole('option', { name: 'Web' }));
    await user.keyboard('{Escape}');
    await waitFor(() => {
      for (const endpoint of ['/logs?', '/statistics?']) {
        const path = paths.filter((value) => value.includes(endpoint)).at(-1);
        expect(
          new URL(path ?? '', 'https://example.com').searchParams.getAll('access_type'),
        ).toEqual(['2', '1']);
      }
    });
    await user.click(screen.getByRole('button', { name: '重置' }));
    await waitFor(() => {
      for (const endpoint of ['/logs?', '/statistics?']) {
        const path = paths.filter((value) => value.includes(endpoint)).at(-1);
        expect(new URL(path ?? '', 'https://example.com').searchParams.has('access_type')).toBe(
          false,
        );
      }
    });
    expect(screen.getByRole('combobox', { name: '操作来源' })).toHaveTextContent('全部来源');
  });

  it('空间多选与业务操作同步用于列表和统计，重置恢复全部空间', async () => {
    const paths = renderAudit();
    const user = userEvent.setup();
    await waitFor(() => {
      expect(screen.getByRole('button', { name: '搜索' })).toBeEnabled();
    });
    expect(screen.getByRole('combobox', { name: '工作空间' })).toHaveTextContent('全部空间');
    await user.click(screen.getByRole('combobox', { name: '工作空间' }));
    await user.click(screen.getByRole('option', { name: `示例空间（${workspaceID}）` }));
    await user.click(
      screen.getByRole('option', { name: `已删除工作空间（${deletedWorkspaceID}）` }),
    );
    await user.keyboard('{Escape}');
    await user.click(screen.getByRole('combobox', { name: '操作' }));
    await user.click(screen.getByRole('option', { name: '编辑党建人员' }));
    await user.keyboard('{Escape}');
    await waitFor(() => {
      expectAction(paths, 'cpc.person.update');
      for (const endpoint of ['/logs?', '/statistics?']) {
        const path = paths.filter((value) => value.includes(endpoint)).at(-1);
        expect(
          new URL(path ?? '', 'https://example.com').searchParams.getAll('workspace_id'),
        ).toEqual([workspaceID, deletedWorkspaceID]);
      }
    });
    await user.click(screen.getByRole('button', { name: '重置' }));
    await waitFor(() => {
      expectAction(paths, 'exclude_audit');
      for (const endpoint of ['/logs?', '/statistics?']) {
        const path = paths.filter((value) => value.includes(endpoint)).at(-1);
        expect(new URL(path ?? '', 'https://example.com').searchParams.has('workspace_id')).toBe(
          false,
        );
      }
    });
    expect(screen.getByRole('combobox', { name: '工作空间' })).toHaveTextContent('全部空间');
  });

  it('工作空间与全局可以组合查询', async () => {
    const paths = renderAudit();
    const user = userEvent.setup();
    await waitFor(() => {
      expect(screen.getByRole('button', { name: '搜索' })).toBeEnabled();
    });
    await user.click(screen.getByRole('combobox', { name: '工作空间' }));
    await user.click(screen.getByRole('option', { name: '系统' }));
    await user.click(screen.getByRole('option', { name: `示例空间（${workspaceID}）` }));
    await waitFor(() => {
      for (const endpoint of ['/logs?', '/statistics?']) {
        const path = paths.filter((value) => value.includes(endpoint)).at(-1);
        expect(
          new URL(path ?? '', 'https://example.com').searchParams.getAll('workspace_id'),
        ).toEqual(['system', workspaceID]);
      }
    });
  });

  it('业务记录显示空间与对象名称，详情展示普通变更和受保护快照提示', async () => {
    const paths = renderAudit(false, true);
    const user = userEvent.setup();
    const table = within(await screen.findByRole('table'));
    expect(await table.findByText('党建人员 / 7')).toBeInTheDocument();
    expect(table.getByText(`示例空间（${workspaceID}）`)).toBeInTheDocument();
    expect(table.getByText('Web')).toBeInTheDocument();
    expect(table.getByText('虚构用户(fixture)')).toBeInTheDocument();
    const headers = table.getAllByRole('columnheader');
    const headerNames = headers.map((header) => header.textContent);
    expect(headerNames[headerNames.indexOf('操作结果') + 1]).toBe('操作来源');
    expect(headers[0]).toHaveTextContent('ID');
    expect(headers[0]).toHaveClass('table-pinned');
    expect(headers.at(-1)).toHaveClass('table-pinned-end', 'right-0');
    const idButton = table.getByRole('button', { name: row.id });
    expect(idButton.closest('td')).toHaveClass('table-pinned');
    const detailButton = table.getByRole('button', { name: '查看编辑党建人员详情' });
    expect(detailButton.closest('td')).toHaveClass('table-pinned-end', 'right-0');
    await user.click(idButton);
    const detail = within(await screen.findByRole('dialog', { name: '审计详情' }));
    expect(await detail.findByRole('tabpanel', { name: 'JSON 差异' })).toHaveTextContent(
      '"party_position": "书记"',
    );
    expect(detail.getAllByLabelText('新增').length).toBeGreaterThan(0);
    expect(detail.getAllByLabelText('删除').length).toBeGreaterThan(0);
    await user.click(detail.getByRole('tab', { name: '变更前' }));
    expect(detail.getByLabelText('变更前 JSON')).toHaveTextContent('"revision": 1');
    await user.click(detail.getByRole('tab', { name: '变更后' }));
    expect(detail.getByLabelText('变更后 JSON')).toHaveTextContent('"revision": 2');
    await user.click(detail.getByRole('tab', { name: '事件信息' }));
    for (const title of ['操作与结果', '操作用户', '操作对象', '请求信息', '时间与编号']) {
      expect(detail.getByRole('region', { name: title })).toBeVisible();
    }
    const snapshot = within(detail.getByRole('region', { name: '操作时的用户信息' }));
    expect(snapshot.getByText('用户名')).toBeVisible();
    expect(snapshot.getByText('fixture')).toBeVisible();
    expect(snapshot.getByText('系统管理员')).toBeVisible();
    expect(detail.getByText(`示例空间（${workspaceID}）`)).toBeInTheDocument();
    expect(detail.getByText('Web')).toBeInTheDocument();
    expect(detail.getByText('有')).toBeInTheDocument();
    const detailPath = `/v1/system/audit/logs/${row.event_id}?event_date=${row.event_date}`;
    expect(paths.filter((path) => path === detailPath)).toHaveLength(1);
  });

  it('默认选中不含审计，搜索左侧可重置未提交的用户编号', async () => {
    const paths = renderAudit();
    const user = userEvent.setup();
    const reset = screen.getByRole('button', { name: '重置' });
    const search = screen.getByRole('button', { name: '搜索' });
    expect(screen.getByRole('combobox', { name: '操作' })).toHaveTextContent('不含审计');
    expect(reset.nextElementSibling).toBe(search);
    await waitFor(() => {
      expect(reset).toBeEnabled();
    });
    expectAction(paths, 'exclude_audit');
    const keyword = screen.getByRole('textbox', { name: '用户 ID' });
    const before = paths.length;
    await user.type(keyword, '1234');
    expect(paths).toHaveLength(before);
    await user.click(reset);
    expect(keyword).toHaveValue('');
    await user.click(screen.getByRole('combobox', { name: '操作' }));
    expect(screen.getAllByRole('option', { name: '不含审计' })).toHaveLength(1);
  });

  it('不含审计与具体操作互斥，具体操作仍可多选，面板重置恢复默认', async () => {
    const paths = renderAudit();
    const user = userEvent.setup();
    await waitFor(() => {
      expect(screen.getByRole('button', { name: '搜索' })).toBeEnabled();
    });
    await user.click(screen.getByRole('combobox', { name: '操作' }));
    const excluded = screen.getByRole('option', { name: '不含审计' });
    const personUpdate = screen.getByRole('option', { name: '编辑党建人员' });
    const auditDetail = screen.getByRole('option', { name: '查看审计详情' });
    expect(excluded).toHaveAttribute('aria-selected', 'true');

    await user.click(personUpdate);
    expect(excluded).toHaveAttribute('aria-selected', 'false');
    expect(personUpdate).toHaveAttribute('aria-selected', 'true');
    await waitFor(() => {
      expectAction(paths, 'cpc.person.update');
    });

    await user.click(auditDetail);
    expect(personUpdate).toHaveAttribute('aria-selected', 'true');
    expect(auditDetail).toHaveAttribute('aria-selected', 'true');
    await waitFor(() => {
      expectAction(paths, ['cpc.person.update', 'audit.detail']);
    });

    await user.click(excluded);
    expect(excluded).toHaveAttribute('aria-selected', 'true');
    expect(personUpdate).toHaveAttribute('aria-selected', 'false');
    expect(auditDetail).toHaveAttribute('aria-selected', 'false');
    await waitFor(() => {
      expectAction(paths, 'exclude_audit');
    });

    await user.click(personUpdate);
    await user.keyboard('{Escape}');
    await user.click(screen.getByRole('button', { name: '重置' }));
    await waitFor(() => {
      expectAction(paths, 'exclude_audit');
    });
    await user.click(screen.getByRole('combobox', { name: '操作' }));
    expect(screen.getByRole('option', { name: '不含审计' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    expect(screen.getByRole('option', { name: '编辑党建人员' })).toHaveAttribute(
      'aria-selected',
      'false',
    );
  });

  it('全部操作显式传空值，重置恢复不含审计，列表和统计同步更新', async () => {
    const paths = renderAudit();
    const user = userEvent.setup();
    await waitFor(() => {
      expect(screen.getByRole('button', { name: '搜索' })).toBeEnabled();
    });
    await user.click(screen.getByRole('combobox', { name: '操作' }));
    await user.click(screen.getByRole('option', { name: '全部操作' }));
    await waitFor(() => {
      expectAction(paths, '');
    });
    await waitFor(() => {
      expect(screen.getByRole('button', { name: '重置' })).toBeEnabled();
    });
    await user.click(screen.getByRole('button', { name: '重置' }));
    await waitFor(() => {
      expectAction(paths, 'exclude_audit');
    });
    expect(screen.getByRole('combobox', { name: '操作' })).toHaveTextContent('不含审计');
    await waitFor(() => {
      expect(screen.getByRole('button', { name: '重置' })).toBeEnabled();
    });
    const before = paths.length;
    await user.click(screen.getByRole('combobox', { name: '操作' }));
    await user.click(screen.getByRole('option', { name: '全部操作' }));
    await waitFor(() => {
      expect(paths.length).toBeGreaterThan(before);
      expectAction(paths, '');
    });
  });

  it('前次请求未完成时连续多选立即拉取最新条件的列表与统计', async () => {
    const paths = renderAudit(true);
    const user = userEvent.setup();
    await waitFor(() => {
      expect(screen.getByRole('button', { name: '搜索' })).toBeEnabled();
    });
    await user.click(screen.getByRole('combobox', { name: '操作' }));
    await user.click(screen.getByRole('option', { name: '查看审计详情' }));
    await user.click(screen.getByRole('option', { name: '查看审计统计' }));
    await user.keyboard('{Escape}');
    await user.click(screen.getByRole('button', { name: '更多筛选' }));
    await user.click(screen.getByRole('combobox', { name: '操作结果' }));
    await user.click(screen.getByRole('option', { name: '成功' }));
    await user.click(screen.getByRole('option', { name: '403 无权限' }));
    await user.keyboard('{Escape}');
    await waitFor(() => {
      expectAction(paths, ['audit.detail', 'audit.statistics']);
      for (const endpoint of ['/logs?', '/statistics?']) {
        const path = paths.filter((value) => value.includes(endpoint)).at(-1);
        expect(
          new URL(path ?? '', 'https://example.com').searchParams.getAll('result_code'),
        ).toEqual(['0', '403']);
      }
    });
  });

  it('日期时间确认后一次同步列表和统计，编辑草稿不触发查询', async () => {
    const paths = renderAudit();
    await waitFor(() => {
      expect(screen.getByRole('button', { name: '搜索' })).toBeEnabled();
    });
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: '更多筛选' }));
    for (const [label, value, hour, minute] of [
      ['开始时间', '2026-01-01', '08', '30'],
      ['结束时间', '2026-01-31', '10', '45'],
    ] as const) {
      const before = paths.length;
      await user.click(screen.getByRole('combobox', { name: label }));
      const picker = within(screen.getByRole('dialog', { name: `${label}选择` }));
      const input = picker.getByRole('textbox', { name: '输入日期' });
      await user.clear(input);
      await user.type(input, value);
      await user.click(picker.getByRole('option', { name: `${hour} 时` }));
      await user.click(picker.getByRole('option', { name: `${minute} 分` }));
      expect(paths).toHaveLength(before);
      await user.click(picker.getByRole('button', { name: '确定' }));
      await waitFor(() => {
        expect(paths).toHaveLength(before + 2);
      });
    }
    await waitFor(() => {
      for (const endpoint of ['/logs?', '/statistics?']) {
        const path = paths.filter((value) => value.includes(endpoint)).at(-1);
        const parameters = new URL(path ?? '', 'https://example.com').searchParams;
        expect(parameters.get('start_time')).toBe(String(Date.parse('2026-01-01T08:30:00Z')));
        expect(parameters.get('end_time')).toBe(String(Date.parse('2026-01-31T10:45:00Z')));
        expect(parameters.get('page') ?? '1').toBe('1');
      }
    });
    await user.click(screen.getByRole('button', { name: '更多筛选' }));
    expect(screen.queryByRole('combobox', { name: '开始时间' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '更多筛选' }));
    expect(screen.getByRole('combobox', { name: '开始时间' })).toHaveTextContent(
      '2026年1月1日 08:30',
    );
    expect(screen.getByRole('combobox', { name: '结束时间' })).toHaveTextContent(
      '2026年1月31日 10:45',
    );
  });
});
