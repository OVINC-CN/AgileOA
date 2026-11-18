import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { afterEach, expect, it, vi } from 'vitest';

import { defaultFilters, personFilterParams, type PersonFilters } from '@/features/cpc-person/api';
import { PersonFilterPanel } from '@/features/cpc-person/person-filters';
import type { TimelineFilterDraft } from '@/features/cpc-person/timeline-filter-model';
import {
  prepareTimelinePreset,
  readTimelinePresets,
  timelinePresetStorageKey,
  writeTimelinePresets,
} from '@/features/cpc-person/timeline-filter-preset-store';
import { currentUserKey } from '@/features/user/api';

const workspaceID = 'workspace-one';
const userID = 7;
const storageKey = timelinePresetStorageKey(userID, workspaceID);
const filters: TimelineFilterDraft[] = [
  {
    id: 'score',
    field: { node_id: 7, field_id: 'score', label: '虚构节点 / 分数', type: 'number' },
    operator: 'eq',
    value: '0',
  },
  {
    id: 'confirmed',
    field: { node_id: 7, field_id: 'confirmed', label: '虚构节点 / 已确认', type: 'boolean' },
    operator: 'eq',
    value: 'false',
  },
];
const fields = filters.flatMap(({ field }) => (field ? [field] : []));

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});

function setup(initial = filters, available = fields) {
  const apply = vi.fn<(values: PersonFilters) => Promise<void>>(() => Promise.resolve());
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  client.setQueryData(currentUserKey, {
    id: userID,
    username: 'example',
    display_name: '虚构账号',
    email: '',
    is_admin: false,
    password_login_enabled: true,
    password_change_required: false,
  });
  vi.stubGlobal(
    'fetch',
    vi.fn(() =>
      Promise.resolve(
        new Response(
          JSON.stringify({
            trace: '4bf92f3577b34da6a3ce929d0e0e4736',
            error_msg: '',
            data: available,
          }),
        ),
      ),
    ),
  );
  function Form() {
    const [draft, setDraft] = useState<PersonFilters>({
      ...defaultFilters,
      keyword: '原关键词',
      timeline_filters: initial,
    });
    return (
      <PersonFilterPanel
        workspaceID={workspaceID}
        draft={draft}
        setDraft={setDraft}
        changeFilters={(changes) => {
          setDraft((current) => ({ ...current, ...changes }));
        }}
        onApply={(values) => {
          personFilterParams(values);
          return apply(values);
        }}
      />
    );
  }
  render(
    <QueryClientProvider client={client}>
      <Form />
    </QueryClientProvider>,
  );
  return { apply, user: userEvent.setup() };
}

it('保存、恢复、重命名、覆盖与确认删除共用本地方案，重置不删除且恢复只修改时间线草稿', async () => {
  const { apply, user } = setup();
  await user.click(screen.getByRole('button', { name: '更多筛选' }));
  await user.click(screen.getByRole('button', { name: '保存当前' }));
  let editor = within(screen.getByRole('dialog', { name: '保存常用筛选方案' }));
  await user.type(editor.getByLabelText(/方案名称/), '日常核对');
  await user.click(editor.getByRole('button', { name: '保存' }));
  expect(apply).not.toHaveBeenCalled();
  expect(readTimelinePresets(storageKey).presets).toHaveLength(1);

  await user.click(screen.getByRole('button', { name: '重置' }));
  expect(readTimelinePresets(storageKey).presets).toHaveLength(1);
  await user.type(screen.getByRole('searchbox', { name: '学号或姓名' }), '新关键词');
  const callsBeforeRestore = apply.mock.calls.length;
  await user.click(screen.getByRole('combobox', { name: '常用筛选方案' }));
  await user.click(screen.getByRole('option', { name: '日常核对' }));
  await waitFor(() => {
    expect(screen.getByRole('textbox', { name: '筛选值 1' })).toHaveValue('0');
  });
  expect(screen.getByRole('combobox', { name: '筛选值 2' })).toHaveTextContent('否');
  expect(screen.getByRole('searchbox', { name: '学号或姓名' })).toHaveValue('新关键词');
  expect(apply).toHaveBeenCalledTimes(callsBeforeRestore);
  await user.click(screen.getByRole('button', { name: '搜索' }));
  await waitFor(() => {
    expect(apply).toHaveBeenCalledTimes(callsBeforeRestore + 1);
  });
  const submitted = apply.mock.lastCall?.[0];
  expect(submitted?.keyword).toBe('新关键词');
  expect(submitted?.status).toEqual(defaultFilters.status);
  expect(submitted && personFilterParams(submitted).getAll('timeline_filter')).toEqual([
    JSON.stringify({ node_id: 7, field_id: 'score', operator: 'eq', value: '0' }),
    JSON.stringify({ node_id: 7, field_id: 'confirmed', operator: 'eq', value: 'false' }),
  ]);

  await user.click(screen.getByRole('button', { name: '管理方案' }));
  await user.click(screen.getByRole('button', { name: '重命名方案 日常核对' }));
  editor = within(screen.getByRole('dialog', { name: '重命名筛选方案' }));
  await user.clear(editor.getByLabelText(/方案名称/));
  await user.type(editor.getByLabelText(/方案名称/), '新的名称');
  await user.click(editor.getByRole('button', { name: '保存' }));
  expect(readTimelinePresets(storageKey).presets[0]?.name).toBe('新的名称');
  let manager = within(screen.getByRole('dialog', { name: '管理常用筛选方案' }));
  await user.click(manager.getByRole('button', { name: '关闭' }));

  await user.clear(screen.getByRole('textbox', { name: '筛选值 1' }));
  await user.type(screen.getByRole('textbox', { name: '筛选值 1' }), '20');
  await user.click(screen.getByRole('button', { name: '管理方案' }));
  await user.click(screen.getByRole('button', { name: '覆盖方案 新的名称' }));
  editor = within(screen.getByRole('dialog', { name: '覆盖常用筛选方案' }));
  await user.click(editor.getByRole('button', { name: '覆盖保存' }));
  expect(readTimelinePresets(storageKey).presets[0]?.conditions[0]?.value).toBe('20');

  await user.click(screen.getByRole('button', { name: '删除方案 新的名称' }));
  let confirmation = within(screen.getByRole('dialog', { name: '删除常用筛选方案' }));
  await user.click(confirmation.getByRole('button', { name: '取消' }));
  expect(readTimelinePresets(storageKey).presets).toHaveLength(1);
  await user.click(screen.getByRole('button', { name: '删除方案 新的名称' }));
  confirmation = within(screen.getByRole('dialog', { name: '删除常用筛选方案' }));
  await user.click(confirmation.getByRole('button', { name: '删除方案' }));
  expect(readTimelinePresets(storageKey).presets).toEqual([]);
  manager = within(screen.getByRole('dialog', { name: '管理常用筛选方案' }));
  expect(manager.getByText('暂无常用筛选方案')).toBeInTheDocument();
  await user.click(manager.getByRole('button', { name: '关闭' }));
  expect(screen.getByRole('textbox', { name: '筛选值 1' })).toHaveValue('20');
});

it('失效字段保留并阻断搜索，不能通过重复选择旧选项清除失效标记', async () => {
  const preset = {
    id: '2a72d10a-b2c0-49f6-81dc-2b54f297aa00',
    ...prepareTimelinePreset('旧方案', true, filters),
  };
  writeTimelinePresets(storageKey, [preset]);
  const { apply, user } = setup([], []);
  await user.click(screen.getByRole('button', { name: '更多筛选' }));
  await user.click(screen.getByRole('combobox', { name: '常用筛选方案' }));
  await user.click(screen.getByRole('option', { name: '旧方案' }));
  await waitFor(() => {
    expect(screen.getByRole('combobox', { name: '时间线字段 1' })).toHaveTextContent('字段已失效');
  });
  await user.click(screen.getByRole('button', { name: '搜索' }));
  expect(apply).not.toHaveBeenCalled();
  await user.click(screen.getByRole('combobox', { name: '时间线字段 1' }));
  await user.click(screen.getByRole('option', { name: /分数（字段已失效）/ }));
  await user.click(screen.getByRole('button', { name: '搜索' }));
  expect(apply).not.toHaveBeenCalled();
  await user.click(screen.getByRole('button', { name: '删除时间线条件 2' }));
  await user.click(screen.getByRole('button', { name: '删除时间线条件 1' }));
  await user.click(screen.getByRole('button', { name: '搜索' }));
  await waitFor(() => {
    expect(apply).toHaveBeenCalledTimes(1);
  });
});
