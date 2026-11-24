import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { z } from 'zod';

import type { PartyBranch } from '@/features/party-branch/api';
import { BranchEditor } from '@/features/party-branch/branch-editor';
import { BranchPage } from '@/features/party-branch/branch-page';

const workspaceID = 'w7m2k9p4s6n8c3x5';
const base = `/v1/workspaces/${workspaceID}/cpc-party-branches`;
const record: PartyBranch = {
  id: 1,
  full_name: '中共虚构学生第一支部委员会',
  short_name: '虚构第一党支部',
  priority: 100,
  secretary_id: 2,
  secretary_name: '虚构书记',
  secretary_username: 'fixture.secretary',
  revision: 3,
  created_at: '2026-11-05T00:00:00Z',
  updated_at: '2026-11-05T00:00:00Z',
};

function client() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

function respond(data: unknown, status = 200, error = '') {
  return Response.json(
    { trace: '4bf92f3577b34da6a3ce929d0e0e4736', error_msg: error, data },
    { status },
  );
}

const bodySchema = z.strictObject({
  full_name: z.string(),
  short_name: z.string(),
  secretary_id: z.number().nullable(),
  priority: z.number().int(),
  revision: z.number(),
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('党支部', () => {
  it('没有列表权限时隐藏子页及业务按钮，也不请求党支部或用户选项', () => {
    const fetch = vi.fn();
    vi.stubGlobal('fetch', fetch);
    render(
      <QueryClientProvider client={client()}>
        <BranchPage
          workspaceID={workspaceID}
          workspaceName="虚构空间"
          actions={['cpc.party_branch.create']}
          refresh={() => Promise.resolve()}
        />
      </QueryClientProvider>,
    );
    expect(screen.getByText('暂无可查看的数据管理页面')).toBeVisible();
    expect(screen.queryByRole('tab', { name: '党支部' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '新增党支部' })).not.toBeInTheDocument();
    expect(fetch).not.toHaveBeenCalled();
  });

  it('仅有查看和导入权限时不显示重排人员', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(() => Promise.resolve(respond({ items: [], total: 0, page: 1, page_size: 5 }))),
    );
    render(
      <QueryClientProvider client={client()}>
        <BranchPage
          workspaceID={workspaceID}
          workspaceName="虚构空间"
          actions={['cpc.party_branch.list', 'cpc.party_branch.import']}
          refresh={() => Promise.resolve()}
        />
      </QueryClientProvider>,
    );
    expect(await screen.findByRole('button', { name: '导入' })).toBeVisible();
    expect(screen.queryByRole('button', { name: '重排人员' })).not.toBeInTheDocument();
  });

  it('有删除权限时，更多操作中的删除打开党支部确认抽屉', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(() => Promise.resolve(respond({ items: [record], total: 1, page: 1, page_size: 5 }))),
    );
    render(
      <QueryClientProvider client={client()}>
        <BranchPage
          workspaceID={workspaceID}
          workspaceName="虚构空间"
          actions={['cpc.party_branch.list', 'cpc.party_branch.delete']}
          refresh={() => Promise.resolve()}
        />
      </QueryClientProvider>,
    );
    const table = within(screen.getByRole('table'));
    const more = await table.findByRole('button', { name: `更多操作 ${record.short_name}` });
    expect(table.queryByRole('button', { name: '删除' })).not.toBeInTheDocument();
    await userEvent.click(more);
    await userEvent.click(screen.getByRole('menuitem', { name: '删除' }));
    expect(screen.getByRole('menuitem', { name: '确认' })).toHaveAccessibleDescription(
      `确认删除 ${record.full_name} ?`,
    );
  });

  it('编辑权限允许重排，按钮紧邻新增左侧，二次确认前不提交且提交中保护关闭', async () => {
    const submitted: string[] = [];
    let finish: (response: Response) => void = () => {
      throw new Error('尚未提交重排');
    };
    vi.stubGlobal(
      'fetch',
      vi.fn((path: string, options: RequestInit) => {
        if (options.method === 'POST') {
          submitted.push(path);
          return new Promise<Response>((resolve) => {
            finish = resolve;
          });
        }
        return Promise.resolve(respond({ items: [], total: 0, page: 1, page_size: 5 }));
      }),
    );
    render(
      <QueryClientProvider client={client()}>
        <BranchPage
          workspaceID={workspaceID}
          workspaceName="虚构空间"
          actions={[
            'cpc.party_branch.list',
            'cpc.party_branch.import',
            'cpc.party_branch.export',
            'cpc.party_branch.update',
            'cpc.party_branch.create',
          ]}
          refresh={() => Promise.resolve()}
        />
      </QueryClientProvider>,
    );
    const user = userEvent.setup();
    const reorder = await screen.findByRole('button', { name: '重排人员' });
    expect(reorder.nextElementSibling).toBe(screen.getByRole('button', { name: '新增党支部' }));
    await user.click(reorder);
    expect(screen.getByRole('dialog', { name: '确认重排人员' })).toBeVisible();
    expect(submitted).toHaveLength(0);
    await user.click(screen.getByRole('button', { name: '取消' }));
    expect(screen.queryByRole('dialog', { name: '确认重排人员' })).not.toBeInTheDocument();
    expect(submitted).toHaveLength(0);

    await user.click(reorder);
    await user.click(screen.getByRole('button', { name: '确认重排' }));
    expect(submitted).toEqual([`${base}/reorder-people`]);
    expect(screen.getByRole('button', { name: '确认重排' })).toBeDisabled();
    await user.keyboard('{Escape}');
    expect(screen.getByRole('dialog', { name: '确认重排人员' })).toBeVisible();
    finish(respond({ queued: true }));
    await waitFor(() => {
      expect(screen.queryByRole('dialog', { name: '确认重排人员' })).not.toBeInTheDocument();
    });
    expect(submitted).toHaveLength(1);
  });

  it.each([{ original: null }, { original: record }])(
    '新增或编辑不包含专业字段，管理员可以显式清空：$original',
    async ({ original }) => {
      const bodies: z.infer<typeof bodySchema>[] = [];
      vi.stubGlobal(
        'fetch',
        vi.fn((path: string, options: RequestInit) => {
          if (path.endsWith('/secretaries')) {
            return Promise.resolve(respond([{ value: 2, label: '虚构书记(fixture.secretary)' }]));
          }
          if (typeof options.body !== 'string') throw new Error('请求体必须为 JSON 文本');
          const body = bodySchema.parse(JSON.parse(options.body) as unknown);
          bodies.push(body);
          return Promise.resolve(respond({ ...record, ...body, revision: original ? 4 : 1 }));
        }),
      );
      const closed = vi.fn();
      render(
        <QueryClientProvider client={client()}>
          <BranchEditor workspaceID={workspaceID} branch={original} onClose={closed} />
        </QueryClientProvider>,
      );
      const user = userEvent.setup();
      if (!original) {
        await user.type(screen.getByLabelText('党支部全称'), record.full_name);
        await user.type(screen.getByLabelText('党支部简称'), record.short_name);
      } else {
        await user.click(screen.getByRole('combobox', { name: '管理员' }));
        await user.click(screen.getByRole('option', { name: '未配置' }));
      }
      expect(screen.queryByLabelText('专业')).not.toBeInTheDocument();
      await user.click(screen.getByRole('button', { name: '保存' }));
      await waitFor(() => {
        expect(closed).toHaveBeenCalledOnce();
      });
      expect(bodies).toEqual([
        {
          full_name: record.full_name,
          short_name: record.short_name,
          secretary_id: null,
          priority: original?.priority ?? 0,
          revision: original?.revision ?? 0,
        },
      ]);
    },
  );

  it('简称重复时保留表单并通过顶部消息提示，不报告成功', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn((path: string) =>
        Promise.resolve(
          path === `${base}/secretaries`
            ? respond([])
            : respond(null, 409, '当前空间已存在相同全称或简称的党支部'),
        ),
      ),
    );
    const closed = vi.fn();
    render(
      <QueryClientProvider client={client()}>
        <BranchEditor workspaceID={workspaceID} branch={record} onClose={closed} />
      </QueryClientProvider>,
    );
    await userEvent.click(screen.getByRole('button', { name: '保存' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(
      '当前空间已存在相同全称或简称的党支部',
    );
    expect(closed).not.toHaveBeenCalled();
    expect(screen.getByLabelText('党支部简称')).toHaveValue(record.short_name);
  });
});
