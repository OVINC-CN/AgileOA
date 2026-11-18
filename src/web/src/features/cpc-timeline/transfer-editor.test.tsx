import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';

import { newNode, type TimelineTransfer } from '@/features/cpc-timeline/model';
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

function Harness() {
  const [transfers, setTransfers] = useState<TimelineTransfer[]>([
    { target_template_id: 2, node_mappings: [] },
  ]);
  return (
    <TemplateTransferEditor
      workspaceID={workspaceID}
      templateID={1}
      templateName="申请模板"
      nodes={[source]}
      transfers={transfers}
      onChange={setTransfers}
    />
  );
}

function setup() {
  return render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <Harness />
    </QueryClientProvider>,
  );
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

it('基础输入外观一致，转移配置隐藏底部预览，返回节点配置后恢复', async () => {
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <TemplateEditor workspaceID={workspaceID} original={null} actions={[]} onClose={vi.fn()} />
    </QueryClientProvider>,
  );
  const user = userEvent.setup();
  for (const name of ['模板名称', '模板说明']) {
    expect(screen.getByRole('textbox', { name })).toHaveClass(
      'h-8',
      'min-h-8',
      'py-1',
      'leading-[22px]',
    );
  }
  expect(screen.getByRole('button', { name: '预览' })).toBeVisible();
  await user.click(screen.getByRole('tab', { name: '转移配置' }));
  expect(screen.queryByRole('button', { name: '预览' })).not.toBeInTheDocument();
  const dialog = within(screen.getByRole('dialog'));
  expect(dialog.getByRole('button', { name: '取消' })).toBeVisible();
  expect(dialog.getByRole('button', { name: '保存' })).toBeVisible();
  await user.click(screen.getByRole('tab', { name: '节点配置' }));
  expect(screen.getByRole('button', { name: '预览' })).toBeVisible();
});
