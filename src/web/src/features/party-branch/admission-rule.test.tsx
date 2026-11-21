import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { z } from 'zod';

import {
  type AdmissionRuleNode,
  admissionRuleNodeSchema,
  cleanAdmissionRule,
} from '@/features/party-branch/admission-rule';
import type { AdmissionRuleInfo, PartyBranch } from '@/features/party-branch/api';
import { BranchEditor } from '@/features/party-branch/branch-editor';
import { BranchPage } from '@/features/party-branch/branch-page';
import { getConfig } from '@/lib/config';

const workspaceID = 'w7m2k9p4s6n8c3x5';
const base = `/v1/workspaces/${workspaceID}/cpc-party-branches`;
const original: AdmissionRuleInfo = {
  id: 1,
  revision: 3,
  rule: null,
  updated_at: '2026-11-05T00:00:00Z',
  fields: [
    { value: 'advisor', label: '导师姓名', max_length: 50 },
    { value: 'major', label: '专业', max_length: 100 },
    { value: 'education_level', label: '学历', max_length: 32 },
    { value: 'student_no', label: '学号', max_length: 32 },
  ],
  logics: [
    { value: 'and', label: '全部满足（AND）' },
    { value: 'or', label: '任一满足（OR）' },
  ],
  operators: [{ value: 'in', label: '包含于（in）' }],
};
const branch: PartyBranch = {
  id: 1,
  revision: 3,
  full_name: '中共虚构支部委员会',
  short_name: '虚构党支部',
  priority: 100,
  secretary_id: null,
  secretary_name: '',
  secretary_username: '',
  created_at: '2026-11-05T00:00:00Z',
  updated_at: '2026-11-05T00:00:00Z',
};
const nested: AdmissionRuleNode = {
  type: 'group',
  logic: 'and',
  children: [
    { type: 'condition', field: 'advisor', operator: 'in', values: ['张三', '王五'] },
    {
      type: 'group',
      logic: 'or',
      children: [{ type: 'condition', field: 'major', operator: 'in', values: ['会计', '审计'] }],
    },
  ],
};
const requestSchema = z.strictObject({
  full_name: z.string(),
  short_name: z.string(),
  secretary_id: z.number().nullable(),
  priority: z.number().int(),
  revision: z.number(),
  admission_rule: z.object({ rule: admissionRuleNodeSchema.nullable() }),
});
function respond(data: unknown, status = 200, error = '') {
  return Response.json(
    { trace: '4bf92f3577b34da6a3ce929d0e0e4736', error_msg: error, data },
    { status },
  );
}
function createClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}
function renderEditor(rule: AdmissionRuleNode | null, closed = vi.fn()) {
  render(
    <QueryClientProvider client={createClient()}>
      <BranchEditor
        workspaceID={workspaceID}
        branch={branch}
        admissionRule={{ ...original, rule }}
        onClose={closed}
      />
    </QueryClientProvider>,
  );
  return closed;
}
afterEach(() => {
  vi.unstubAllGlobals();
});

describe('党支部准入规则', () => {
  it('选择学号后支持多值录入、预览和保存，保留学号前导零', async () => {
    const submitted: z.infer<typeof requestSchema>[] = [];
    vi.stubGlobal(
      'fetch',
      vi.fn((path: string, options: RequestInit) => {
        if (path.endsWith('/secretaries')) return Promise.resolve(respond([]));
        if (typeof options.body !== 'string') throw new Error('请求体必须为 JSON 文本');
        submitted.push(requestSchema.parse(JSON.parse(options.body) as unknown));
        return Promise.resolve(respond({ ...branch, revision: 4 }));
      }),
    );
    const closed = renderEditor(null);
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: '添加条件' }));
    await user.click(screen.getByRole('combobox', { name: '条件字段' }));
    await user.click(screen.getByRole('option', { name: '学号' }));
    await user.type(
      screen.getByRole('textbox', { name: '学号候选值' }),
      '000001、S20260001、000001{Enter}',
    );
    const preview = within(screen.getByRole('region', { name: '规则预览' }));
    expect(preview.getByText('学号')).toBeVisible();
    expect(preview.getByText('000001')).toBeVisible();
    expect(preview.getByText('S20260001')).toBeVisible();
    await user.click(screen.getByRole('button', { name: '保存' }));
    await waitFor(() => {
      expect(closed).toHaveBeenCalledOnce();
    });
    expect(submitted[0]?.admission_rule.rule).toEqual({
      type: 'condition',
      field: 'student_no',
      operator: 'in',
      values: ['000001', 'S20260001'],
    });
  });

  it('学号规则支持 32 字符边界，拒绝超长值、空值及控制字符', () => {
    const rule = {
      type: 'condition',
      field: 'student_no',
      operator: 'in',
      values: ['0'.repeat(32)],
    } as const;
    expect(cleanAdmissionRule({ ...rule, values: [...rule.values] })?.type).toBe('condition');
    for (const value of ['0'.repeat(33), ' ', '000\u00001']) {
      expect(() => cleanAdmissionRule({ ...rule, values: [value] })).toThrow();
    }
  });

  it('在支部编辑表单添加嵌套 OR 子组，并与基础信息通过一次请求保存', async () => {
    const submitted: z.infer<typeof requestSchema>[] = [];
    const writes: string[] = [];
    vi.stubGlobal(
      'fetch',
      vi.fn((path: string, options: RequestInit) => {
        if (path.endsWith('/secretaries')) return Promise.resolve(respond([]));
        if (typeof options.body !== 'string') throw new Error('请求体必须为 JSON 文本');
        submitted.push(requestSchema.parse(JSON.parse(options.body) as unknown));
        writes.push(path);
        return Promise.resolve(respond({ ...branch, revision: 4 }));
      }),
    );
    const closed = renderEditor(null);
    const user = userEvent.setup();
    await user.clear(screen.getByLabelText('党支部简称'));
    await user.type(screen.getByLabelText('党支部简称'), '新的虚构简称');
    await user.click(screen.getByRole('button', { name: '添加条件组' }));
    await user.type(
      screen.getByRole('textbox', { name: '导师姓名候选值' }),
      '张三{Enter}王五，张三{Enter}',
    );
    await user.click(screen.getByRole('button', { name: '添加子组' }));
    const nestedLogic = screen.getAllByRole('combobox', { name: '组合逻辑' })[1];
    if (!nestedLogic) throw new Error('缺少子组逻辑选择器');
    await user.click(nestedLogic);
    await user.click(screen.getByRole('option', { name: '任一满足（OR）' }));
    const nestedField = screen.getAllByRole('combobox', { name: '条件字段' })[1];
    if (!nestedField) throw new Error('缺少子组条件字段选择器');
    await user.click(nestedField);
    await user.click(screen.getByRole('option', { name: '专业' }));
    await user.type(screen.getByRole('textbox', { name: '专业候选值' }), '会计、审计{Enter}');
    const preview = within(screen.getByRole('region', { name: '规则预览' }));
    expect(preview.getAllByRole('list')).toHaveLength(3);
    expect(preview.getByText('全部满足（AND）')).toBeVisible();
    expect(preview.getByText('任一满足（OR）')).toBeVisible();
    expect(preview.getByText('会计')).toBeVisible();
    await user.click(screen.getByRole('button', { name: '保存' }));
    await waitFor(() => {
      expect(closed).toHaveBeenCalledOnce();
    });
    expect(writes).toEqual([`${base}/1`]);
    expect(submitted).toEqual([
      {
        full_name: branch.full_name,
        short_name: '新的虚构简称',
        secretary_id: null,
        priority: branch.priority,
        revision: 3,
        admission_rule: { rule: nested },
      },
    ]);
  });

  it.each([{ rule: nested }, { rule: null }])(
    '只有查看权限时详情按需展示已保存规则及未配置状态',
    async ({ rule }) => {
      const paths: string[] = [];
      vi.stubGlobal(
        'fetch',
        vi.fn((path: string) => {
          paths.push(path);
          return Promise.resolve(
            respond(
              path.endsWith('/admission-rule')
                ? { ...original, rule }
                : { items: [branch], total: 1, page: 1, page_size: 5 },
            ),
          );
        }),
      );
      render(
        <QueryClientProvider client={createClient()}>
          <BranchPage
            workspaceID={workspaceID}
            workspaceName="虚构空间"
            actions={['cpc.party_branch.list']}
            refresh={() => Promise.resolve()}
          />
        </QueryClientProvider>,
      );
      const table = within(screen.getByRole('table'));
      const name = await table.findByRole('button', { name: branch.short_name });
      const headers = table.getAllByRole('columnheader');
      expect(headers.slice(0, 4).map((header) => header.textContent)).toEqual([
        '党支部简称',
        '党支部全称',
        '优先级',
        '管理员',
      ]);
      expect(headers[0]).toHaveClass('table-pinned', 'left-0');
      expect(name.closest('td')).toHaveClass('table-pinned', 'left-0');
      expect(paths.some((path) => path.endsWith('/admission-rule'))).toBe(false);
      expect(screen.queryByRole('button', { name: '准入规则' })).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: '编辑' })).not.toBeInTheDocument();
      expect(table.queryByRole('button', { name: /^更多操作 /u })).not.toBeInTheDocument();
      await userEvent.click(name);
      expect(await screen.findByRole('dialog', { name: '党支部详情' })).toBeVisible();
      const preview = within(screen.getByRole('region', { name: '规则预览' }));
      if (rule) {
        expect(preview.getAllByRole('list')).toHaveLength(3);
        expect(preview.getByText('全部满足（AND）')).toBeVisible();
        expect(preview.getByText('任一满足（OR）')).toBeVisible();
        expect(preview.getByText('张三')).toBeVisible();
      } else {
        expect(preview.getByText('未配置准入规则')).toBeVisible();
      }
    },
  );

  it('保存冲突保留整个编辑表单，清空规则在同一支部更新请求中提交', async () => {
    const requests: z.infer<typeof requestSchema>[] = [];
    vi.stubGlobal(
      'fetch',
      vi.fn((path: string, options: RequestInit) => {
        if (path.endsWith('/secretaries')) return Promise.resolve(respond([]));
        if (typeof options.body === 'string')
          requests.push(requestSchema.parse(JSON.parse(options.body) as unknown));
        return Promise.resolve(respond(null, 409, '党支部已被修改，请刷新后重试'));
      }),
    );
    const closed = renderEditor(nested);
    await userEvent.click(screen.getByRole('button', { name: '保存' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('党支部已被修改');
    expect(screen.getByLabelText('党支部简称')).toHaveValue(branch.short_name);
    expect(closed).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole('button', { name: '清空规则' }));
    await userEvent.click(screen.getByRole('button', { name: '保存' }));
    await waitFor(() => {
      expect(requests).toHaveLength(2);
    });
    expect(requests[1]?.admission_rule).toEqual({ rule: null });
  });

  it('tag 支持删除，点击保存也保留未按回车的有效候选值', async () => {
    const requests: z.infer<typeof requestSchema>[] = [];
    vi.stubGlobal(
      'fetch',
      vi.fn((path: string, options: RequestInit) => {
        if (path.endsWith('/secretaries')) return Promise.resolve(respond([]));
        if (typeof options.body === 'string')
          requests.push(requestSchema.parse(JSON.parse(options.body) as unknown));
        return Promise.resolve(respond({ ...branch, revision: 4 }));
      }),
    );
    const closed = renderEditor(nested);
    await userEvent.click(screen.getByRole('button', { name: '删除候选值 王五' }));
    await userEvent.type(screen.getByRole('textbox', { name: '导师姓名候选值' }), '李四');
    await userEvent.click(screen.getByRole('button', { name: '保存' }));
    await waitFor(() => {
      expect(closed).toHaveBeenCalledOnce();
    });
    expect(requests[0]?.admission_rule.rule).toEqual({
      ...nested,
      children: [
        { type: 'condition', field: 'advisor', operator: 'in', values: ['张三', '李四'] },
        nested.children[1],
      ],
    });
  });

  it('前端限制层数、节点数和候选值数，拒绝空条件组', () => {
    const config = window.AGILE_WORKSPACE_CONFIG;
    const defaults = getConfig();
    for (const limits of [
      { partyBranchRuleMaxDepth: 2 },
      { partyBranchRuleMaxNodes: 2 },
      { partyBranchRuleMaxValues: 1 },
    ]) {
      window.AGILE_WORKSPACE_CONFIG = { ...defaults, ...limits };
      try {
        expect(() => cleanAdmissionRule(nested)).toThrow();
      } finally {
        window.AGILE_WORKSPACE_CONFIG = config;
      }
    }
    expect(() => cleanAdmissionRule({ type: 'group', logic: 'and', children: [] })).toThrow(
      '条件组不能为空',
    );
    expect(cleanAdmissionRule(null)).toBeNull();
  });
});
