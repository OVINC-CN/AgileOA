import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';

import { timelineTransferTargetsOptions } from '@/features/cpc-timeline/api';
import {
  newNode,
  type TimelineInput,
  type TimelineNode,
  type TimelineTransfer,
} from '@/features/cpc-timeline/model';
import { TemplateEditor } from '@/features/cpc-timeline/template-editor';
import { TemplateTransferEditor } from '@/features/cpc-timeline/transfer-editor';

const workspaceID = 'w7m2k9p4s6n8c3x5';
const source = { ...newNode(), id: 'node_source', name: '申请材料', group_name: '申请阶段' };
source.fields.push({ id: 'note', name: '备注', type: 'text', required: false });
source.fields.push({
  id: 'birth',
  name: '出生日期',
  type: 'date',
  required: false,
  student_field: 'birth_date',
});
const target = { ...newNode(), id: 'node_target', name: '接收材料', group_name: '接收阶段' };
target.fields.push({ id: 'note', name: '说明', type: 'text', required: false });
const otherTarget = { ...newNode(), id: 'node_other', name: '后续核对' };
const captureMethods = ['setPointerCapture', 'hasPointerCapture', 'releasePointerCapture'] as const;
const captureDescriptors = new Map<string, PropertyDescriptor | undefined>();
let hitDescriptor: PropertyDescriptor | undefined;

beforeEach(() => {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe = vi.fn();
      disconnect = vi.fn();
    },
  );
  vi.stubGlobal(
    'requestAnimationFrame',
    vi.fn(() => 1),
  );
  vi.stubGlobal('cancelAnimationFrame', vi.fn());
  vi.stubGlobal(
    'PointerEvent',
    class extends MouseEvent {
      pointerId: number;
      pointerType: string;
      constructor(type: string, init: PointerEventInit = {}) {
        super(type, init);
        this.pointerId = init.pointerId ?? 1;
        this.pointerType = init.pointerType ?? 'mouse';
      }
    },
  );
  for (const name of captureMethods) {
    captureDescriptors.set(name, Object.getOwnPropertyDescriptor(HTMLElement.prototype, name));
    Object.defineProperty(HTMLElement.prototype, name, {
      configurable: true,
      value: name === 'hasPointerCapture' ? vi.fn(() => true) : vi.fn(),
    });
  }
  hitDescriptor = Object.getOwnPropertyDescriptor(document, 'elementFromPoint');
  vi.stubGlobal(
    'fetch',
    vi.fn<typeof globalThis.fetch>(() =>
      Promise.resolve(
        new Response(
          JSON.stringify({
            trace: '1'.repeat(32),
            error_msg: '',
            data: [{ id: 2, name: '接收模板', definition: { nodes: [target, otherTarget] } }],
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        ),
      ),
    ),
  );
});

afterEach(() => {
  cleanup();
  for (const name of captureMethods) {
    const descriptor = captureDescriptors.get(name);
    if (descriptor) Object.defineProperty(HTMLElement.prototype, name, descriptor);
    else Reflect.deleteProperty(HTMLElement.prototype, name);
  }
  if (hitDescriptor) Object.defineProperty(document, 'elementFromPoint', hitDescriptor);
  else Reflect.deleteProperty(document, 'elementFromPoint');
  vi.unstubAllGlobals();
});

function Harness({
  initialTransfers,
  nodes = [source],
}: {
  initialTransfers?: TimelineTransfer[] | undefined;
  nodes?: TimelineNode[];
}) {
  const [transfers, setTransfers] = useState<TimelineTransfer[]>([
    ...(initialTransfers ?? [{ target_template_id: 2, node_mappings: [] }]),
  ]);
  return (
    <TemplateTransferEditor
      workspaceID={workspaceID}
      templateID={1}
      templateName="申请模板"
      nodes={nodes}
      transfers={transfers}
      onChange={setTransfers}
    />
  );
}

function setup(initialTransfers?: TimelineTransfer[]) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const view = render(
    <QueryClientProvider client={client}>
      <Harness initialTransfers={initialTransfers} />
    </QueryClientProvider>,
  );
  return { ...view, client };
}

function dragTo(origin: HTMLElement, destination: HTMLElement, pointerType = 'mouse') {
  Object.defineProperty(document, 'elementFromPoint', {
    configurable: true,
    value: vi.fn(() => destination),
  });
  fireEvent.pointerDown(origin, { pointerId: 9, pointerType, button: 0, clientX: 20, clientY: 50 });
  fireEvent.pointerMove(origin, { pointerId: 9, pointerType, clientX: 500, clientY: 80 });
  fireEvent.pointerUp(origin, { pointerId: 9, pointerType, clientX: 500, clientY: 80 });
  fireEvent.click(origin);
}

it('左右完整展示节点、分组和字段，拖拽建立箭头映射，删除字段保留节点关系', async () => {
  setup();
  const user = userEvent.setup();
  const destination = await screen.findByRole('button', { name: '映射到目标字段 接收材料 / 说明' });
  expect(screen.getByRole('button', { name: '映射到目标节点 后续核对' })).toBeVisible();
  expect(screen.getByRole('region', { name: '分组 接收阶段' })).toBeVisible();
  expect(screen.getByRole('button', { name: '拖动来源字段 申请材料 / 出生日期' })).toBeDisabled();
  dragTo(screen.getByRole('button', { name: '拖动来源字段 申请材料 / 备注' }), destination);
  const remove = screen.getByRole('button', { name: '移除字段映射 申请材料 / 备注 → 说明' });
  expect(remove).toBeVisible();
  expect(screen.getByText('已配置 1 组节点映射 · 1 组字段映射')).toBeVisible();
  await user.click(remove);
  expect(screen.getByText('已配置 1 组节点映射 · 0 组字段映射')).toBeVisible();
  expect(screen.getByRole('button', { name: '移除节点映射 申请材料 → 接收材料' })).toBeVisible();
});

it.each(['mouse', 'touch'])('直接拖动字段到右侧建立映射（%s）', async (pointerType) => {
  setup();
  const destination = await screen.findByRole('button', { name: '映射到目标字段 接收材料 / 说明' });
  const origin = screen.getByRole('button', { name: '拖动来源字段 申请材料 / 备注' });
  dragTo(origin, destination, pointerType);
  expect(screen.getByRole('button', { name: '移除字段映射 申请材料 / 备注 → 说明' })).toBeVisible();
});

it('选择式映射排除只读及不兼容字段，支持多目标和删除并保留节点关系', async () => {
  setup();
  const user = userEvent.setup();
  const controls = within(await screen.findByRole('group', { name: '选择式映射' }));
  expect(controls.getByRole('combobox', { name: '映射目标' })).toBeDisabled();
  expect(controls.getByRole('button', { name: '建立映射' })).toBeDisabled();
  await user.click(controls.getByRole('combobox', { name: '映射来源' }));
  expect(screen.queryByRole('option', { name: '1 · 申请材料 / 出生日期' })).not.toBeInTheDocument();
  await user.click(screen.getByRole('option', { name: '1 · 申请材料 / 备注' }));
  await user.click(controls.getByRole('combobox', { name: '映射目标' }));
  expect(screen.queryByRole('option', { name: '1 · 接收材料 / 发生时间' })).not.toBeInTheDocument();
  expect(screen.queryByRole('option', { name: '1 · 接收材料' })).not.toBeInTheDocument();
  await user.click(screen.getByRole('option', { name: '1 · 接收材料 / 说明' }));
  await user.click(controls.getByRole('button', { name: '建立映射' }));
  expect(screen.getByText('已配置 1 组节点映射 · 1 组字段映射')).toBeVisible();
  expect(controls.getByRole('button', { name: '已建立映射' })).toBeDisabled();

  await user.click(controls.getByRole('combobox', { name: '映射来源' }));
  await user.click(screen.getByRole('option', { name: '1 · 申请材料' }));
  expect(controls.getByRole('combobox', { name: '映射目标' })).toHaveTextContent('选择对应的目标');
  await user.click(controls.getByRole('combobox', { name: '映射目标' }));
  await user.click(screen.getByRole('option', { name: '2 · 后续核对' }));
  await user.click(controls.getByRole('button', { name: '建立映射' }));
  expect(screen.getByText('已配置 2 组节点映射 · 1 组字段映射')).toBeVisible();
  await user.click(screen.getByRole('button', { name: '移除字段映射 申请材料 / 备注 → 说明' }));
  expect(screen.getByText('已配置 2 组节点映射 · 0 组字段映射')).toBeVisible();
  await user.click(screen.getByRole('button', { name: '移除节点映射 申请材料 → 后续核对' }));
  expect(screen.getByText('已配置 1 组节点映射 · 0 组字段映射')).toBeVisible();
});

it('模板更新使选择失效时恢复占位提示，禁止提交并保留已配置映射', async () => {
  const { client, rerender } = setup();
  const user = userEvent.setup();
  const controls = within(await screen.findByRole('group', { name: '选择式映射' }));
  dragTo(
    screen.getByRole('button', { name: '拖动来源节点 申请材料' }),
    screen.getByRole('button', { name: '映射到目标节点 后续核对' }),
  );
  await user.click(controls.getByRole('combobox', { name: '映射来源' }));
  await user.click(screen.getByRole('option', { name: '1 · 申请材料 / 备注' }));
  await user.click(controls.getByRole('combobox', { name: '映射目标' }));
  await user.click(screen.getByRole('option', { name: '1 · 接收材料 / 说明' }));
  expect(controls.getByRole('button', { name: '建立映射' })).toBeEnabled();

  // 模拟重新查询后的目标定义，已移除的字段不能残留为可提交选择。
  act(() => {
    client.setQueryData(timelineTransferTargetsOptions(workspaceID, 'update', '', 2).queryKey, [
      {
        id: 2,
        name: '接收模板',
        definition: {
          nodes: [
            { ...target, fields: target.fields.filter((field) => field.id !== 'note') },
            otherTarget,
          ],
        },
      },
    ]);
  });
  await waitFor(() => {
    expect(controls.getByRole('combobox', { name: '映射目标' })).toHaveTextContent(
      '选择对应的目标',
    );
  });
  expect(controls.getByRole('button', { name: '建立映射' })).toBeDisabled();
  rerender(
    <QueryClientProvider client={client}>
      <Harness
        nodes={[
          {
            ...source,
            fields: source.fields.map((field) =>
              field.id === 'note' ? { ...field, type: 'date', student_field: 'birth_date' } : field,
            ),
          },
        ]}
      />
    </QueryClientProvider>,
  );
  expect(controls.getByRole('combobox', { name: '映射来源' })).toHaveTextContent(
    '选择来源节点或字段',
  );
  expect(controls.getByRole('combobox', { name: '映射目标' })).toBeDisabled();
  await user.click(controls.getByRole('button', { name: '建立映射' }));
  expect(screen.getByText('已配置 1 组节点映射 · 0 组字段映射')).toBeVisible();
});

it('切换到双栏时关闭小屏选择弹层，恢复窄屏后保留选择和已配置映射', async () => {
  let wide = false;
  const listeners = new Set<() => void>();
  vi.stubGlobal('matchMedia', () => ({
    get matches() {
      return wide;
    },
    addEventListener: (_type: string, listener: () => void) => {
      listeners.add(listener);
    },
    removeEventListener: (_type: string, listener: () => void) => {
      listeners.delete(listener);
    },
  }));
  setup();
  const user = userEvent.setup();
  const controls = within(await screen.findByRole('group', { name: '选择式映射' }));
  dragTo(
    screen.getByRole('button', { name: '拖动来源节点 申请材料' }),
    screen.getByRole('button', { name: '映射到目标节点 后续核对' }),
  );
  await user.click(controls.getByRole('combobox', { name: '映射来源' }));
  await user.click(screen.getByRole('option', { name: '1 · 申请材料 / 备注' }));
  await user.click(controls.getByRole('combobox', { name: '映射目标' }));
  expect(screen.getByRole('searchbox', { name: '搜索映射目标' })).toBeVisible();
  act(() => {
    wide = true;
    listeners.forEach((listener) => {
      listener();
    });
  });
  expect(screen.queryByRole('group', { name: '选择式映射' })).not.toBeInTheDocument();
  expect(screen.queryByRole('searchbox', { name: '搜索映射目标' })).not.toBeInTheDocument();
  act(() => {
    wide = false;
    listeners.forEach((listener) => {
      listener();
    });
  });
  const restored = within(screen.getByRole('group', { name: '选择式映射' }));
  expect(restored.getByRole('combobox', { name: '映射来源' })).toHaveTextContent('申请材料 / 备注');
  expect(restored.getByRole('combobox', { name: '映射目标' })).toHaveAttribute(
    'aria-expanded',
    'false',
  );
  expect(screen.getByText('已配置 1 组节点映射 · 0 组字段映射')).toBeVisible();
});

it('先选择目标再启用拖拽，未完成的目标不能重复添加，移除后保留原映射', async () => {
  setup([]);
  const user = userEvent.setup();
  const origin = screen.getByRole('button', { name: '拖动来源字段 申请材料 / 备注' });
  expect(origin).toHaveTextContent('备注');
  expect(origin).toBeDisabled();
  expect(screen.queryByRole('button', { name: '关闭联动滚动' })).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: '添加转移目标' })).not.toBeInTheDocument();

  await user.click(screen.getByRole('combobox', { name: '转移目标模板' }));
  await user.click(await screen.findByRole('option', { name: '接收模板' }));
  const destination = await screen.findByRole('button', { name: '映射到目标字段 接收材料 / 说明' });
  const activeOrigin = screen.getByRole('button', { name: '拖动来源字段 申请材料 / 备注' });
  expect(activeOrigin).toBeEnabled();
  dragTo(activeOrigin, destination);
  expect(screen.getByText('已配置 1 组节点映射 · 1 组字段映射')).toBeVisible();
  await user.click(screen.getByRole('button', { name: '关闭联动滚动' }));
  expect(screen.getByRole('button', { name: '开启联动滚动' })).toHaveAttribute(
    'aria-pressed',
    'false',
  );

  await user.click(screen.getByRole('button', { name: '添加转移目标' }));
  expect(screen.getByRole('button', { name: '添加转移目标' })).toBeDisabled();
  expect(screen.getByRole('button', { name: '拖动来源字段 申请材料 / 备注' })).toBeDisabled();
  await user.click(screen.getByRole('button', { name: '移除当前目标' }));
  expect(screen.getByRole('button', { name: '添加转移目标' })).toBeEnabled();
  expect(screen.getByText('已配置 1 组节点映射 · 1 组字段映射')).toBeVisible();
  expect(screen.getByRole('button', { name: '开启联动滚动' })).toHaveAttribute(
    'aria-pressed',
    'false',
  );
});

it('点击和键盘激活不建立映射，Esc 可取消当前拖拽', async () => {
  setup();
  const user = userEvent.setup();
  const destination = await screen.findByRole('button', { name: '映射到目标节点 接收材料' });
  const origin = screen.getByRole('button', { name: '拖动来源节点 申请材料' });
  await user.click(origin);
  await user.click(destination);
  origin.focus();
  await user.keyboard('{ArrowRight}{Enter}');
  destination.focus();
  await user.keyboard('{Enter}');
  expect(screen.getByText('已配置 0 组节点映射 · 0 组字段映射')).toBeVisible();
  expect(origin).not.toHaveAttribute('aria-pressed');
  Object.defineProperty(document, 'elementFromPoint', {
    configurable: true,
    value: vi.fn(() => destination),
  });
  fireEvent.pointerDown(origin, { pointerId: 9, button: 0, clientX: 20, clientY: 50 });
  fireEvent.pointerMove(origin, { pointerId: 9, clientX: 500, clientY: 80 });
  fireEvent.keyDown(origin, { key: 'Escape' });
  fireEvent.pointerUp(origin, { pointerId: 9, clientX: 500, clientY: 80 });
  expect(screen.getByText('已配置 0 组节点映射 · 0 组字段映射')).toBeVisible();
  expect(screen.getByText('已取消拖拽')).toBeInTheDocument();
});

it('节点、字段和时间规则在切换配置及预览后保留草稿，保存完整配置', async () => {
  const onClose = vi.fn();
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <TemplateEditor workspaceID={workspaceID} original={null} actions={[]} onClose={onClose} />
    </QueryClientProvider>,
  );
  const user = userEvent.setup();
  expect(screen.getByRole('dialog')).toHaveAccessibleDescription(
    '配置有序节点、自定义字段和非阻断时间提示规则',
  );
  expect(screen.getByRole('heading', { name: '时间校验规则' })).toBeVisible();
  expect(screen.getByRole('button', { name: '添加规则' })).toBeDisabled();
  expect(screen.getByText('添加其他节点后可配置时间提示规则。')).toBeVisible();
  await user.type(screen.getByRole('textbox', { name: '模板名称' }), '学生发展时间线');
  await user.type(screen.getByRole('textbox', { name: '模板说明' }), '材料审核流程');
  await user.click(screen.getByRole('button', { name: '添加' }));
  const nodeName = screen.getByRole('textbox', { name: '节点名称' });
  expect(nodeName).toHaveFocus();
  await user.clear(nodeName);
  await user.type(nodeName, '材料审核');
  await user.click(screen.getByRole('button', { name: '添加字段' }));
  const fieldName = screen.getByRole('textbox', { name: '字段 2 名称' });
  expect(fieldName).toHaveFocus();
  await user.type(fieldName, '审核说明');
  await user.click(screen.getByRole('combobox', { name: '字段 2 类型' }));
  await user.click(screen.getByRole('option', { name: '多行文本' }));
  await user.click(screen.getByRole('checkbox', { name: '必填' }));
  await user.click(screen.getByRole('checkbox', { name: '基准节点' }));
  await user.type(screen.getByRole('textbox', { name: '节点说明' }), '核对申请材料');
  await user.click(screen.getByRole('button', { name: '添加规则' }));
  expect(screen.getByRole('combobox', { name: '规则 1 参考节点' })).toHaveTextContent('新节点');
  const earliest = screen.getByRole('spinbutton', { name: '最早时间偏移天' });
  await user.clear(earliest);
  await user.type(earliest, '-10');
  const latest = within(screen.getByRole('group', { name: '最晚时间（含边界）' }));
  await user.click(latest.getByRole('checkbox', { name: '不限' }));
  await user.clear(latest.getByRole('spinbutton', { name: '最晚时间偏移天' }));
  await user.type(latest.getByRole('spinbutton', { name: '最晚时间偏移天' }), '20');
  await user.click(screen.getByRole('button', { name: '添加规则' }));
  await user.click(screen.getByRole('button', { name: '1. 新节点' }));
  expect(screen.getByRole('textbox', { name: '节点名称' })).toHaveValue('新节点');
  expect(screen.queryByRole('combobox', { name: '规则 1 参考节点' })).not.toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: '2. 材料审核' }));
  expect(screen.getByRole('textbox', { name: '字段 2 名称' })).toHaveValue('审核说明');
  expect(screen.getByRole('button', { name: '预览' })).toBeVisible();
  await user.click(screen.getByRole('tab', { name: '转移配置' }));
  expect(screen.queryByRole('button', { name: '预览' })).not.toBeInTheDocument();
  const dialog = within(screen.getByRole('dialog'));
  expect(dialog.getByRole('button', { name: '取消' })).toBeVisible();
  expect(dialog.getByRole('button', { name: '保存' })).toBeVisible();
  await user.click(screen.getByRole('tab', { name: '节点配置' }));
  expect(screen.getByRole('button', { name: '预览' })).toBeVisible();
  expect(screen.getByRole('textbox', { name: '模板名称' })).toHaveValue('学生发展时间线');
  expect(screen.getByRole('textbox', { name: '模板说明' })).toHaveValue('材料审核流程');
  expect(screen.getByRole('textbox', { name: '节点名称' })).toHaveValue('材料审核');
  expect(screen.getByRole('textbox', { name: '字段 2 名称' })).toHaveValue('审核说明');
  await user.click(screen.getByRole('button', { name: '预览' }));
  expect(screen.getByRole('heading', { name: '材料审核' })).toBeVisible();
  expect(screen.getByRole('region', { name: '材料审核的时间校验规则' })).toHaveTextContent(
    '材料审核应该在新节点的10天前至20天后。',
  );
  await user.click(screen.getByRole('button', { name: '继续编辑' }));
  expect(screen.getByRole('textbox', { name: '字段 2 名称' })).toHaveValue('审核说明');
  expect(screen.getByRole('combobox', { name: '字段 2 类型' })).toHaveTextContent('多行文本');
  expect(screen.getByRole('checkbox', { name: '必填' })).toBeChecked();
  expect(screen.getByRole('checkbox', { name: '基准节点' })).toBeChecked();
  expect(screen.getByRole('textbox', { name: '节点说明' })).toHaveValue('核对申请材料');
  expect(screen.getByRole('combobox', { name: '规则 2 参考节点' })).toHaveTextContent('新节点');
  await user.click(screen.getByRole('button', { name: '删除规则 2' }));
  expect(screen.queryByRole('combobox', { name: '规则 2 参考节点' })).not.toBeInTheDocument();
  expect(screen.getByRole('spinbutton', { name: '最早时间偏移天' })).toHaveValue(-10);
  expect(screen.getByRole('spinbutton', { name: '最晚时间偏移天' })).toHaveValue(20);

  // 校验实际保存请求，确保布局切换没有丢弃节点、字段及规则配置。
  let submitted: TimelineInput | undefined;
  vi.mocked(fetch).mockImplementationOnce((_path, init) => {
    if (typeof init?.body !== 'string') throw new Error('保存请求须使用 JSON 正文');
    submitted = JSON.parse(init.body) as TimelineInput;
    return Promise.resolve(
      Response.json({
        trace: '1'.repeat(32),
        error_msg: '',
        data: {
          ...submitted,
          id: 1,
          node_count: submitted.definition.nodes.length,
          revision: 1,
          created_at: '2026-11-20T00:00:00Z',
          updated_at: '2026-11-20T00:00:00Z',
        },
      }),
    );
  });
  await user.click(screen.getByRole('button', { name: '保存' }));
  await waitFor(() => {
    expect(onClose).toHaveBeenCalledOnce();
  });
  expect(submitted).toMatchObject({ name: '学生发展时间线', description: '材料审核流程' });
  expect(submitted?.definition.nodes).toHaveLength(2);
  expect(submitted?.definition.nodes[1]).toMatchObject({
    name: '材料审核',
    description: '核对申请材料',
    is_baseline: true,
    fields: [
      { name: '发生时间', type: 'date', required: true },
      { name: '审核说明', type: 'textarea', required: true },
    ],
    rules: [
      {
        reference_node_id: submitted?.definition.nodes[0]?.id,
        min_offset: { years: 0, months: 0, days: -10, workdays: 0 },
        max_offset: { years: 0, months: 0, days: 20, workdays: 0 },
      },
    ],
  });
});
