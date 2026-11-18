import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';

import type { TimelineAttachment, TimelineInfo } from '@/features/cpc-timeline/api';
import { TimelineNodeAttachments } from '@/features/cpc-timeline/attachments';
import { newNode } from '@/features/cpc-timeline/model';
import { TemplateEditor } from '@/features/cpc-timeline/template-editor';
import { loadingExitDelayMs } from '@/lib/motion';
import { preparedUpload } from '@/test/mock-cos';

const workspaceID = 'w7m2k9p4s6n8c3x5';
const template = { id: 7, name: '虚构模板' };
const node = { id: 'node_first', name: '虚构节点' };
const path = `/v1/workspaces/${workspaceID}/cpc-timeline-templates/7/nodes/node_first/attachments`;
const attachment: TimelineAttachment = {
  id: 11,
  timeline_template_id: 7,
  node_id: node.id,
  category: 'cpc_timeline_node',
  filename: '虚构材料.pdf',
  size: 1024,
  state: 'ready',
  revision: 2,
  created_at: '2026-11-06T00:00:00Z',
  updated_at: '2026-11-06T00:00:00Z',
};
const permissions = ['read', 'upload', 'delete'].map(
  (operation) => `cpc.timeline_template.attachment.${operation}`,
);

function respond(data: unknown) {
  return Response.json({ trace: '4bf92f3577b34da6a3ce929d0e0e4736', error_msg: '', data });
}

function client() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

it('未保存节点直接展示禁用的上传入口并提示先保存，不发起请求', () => {
  const fetch = vi.fn();
  vi.stubGlobal('fetch', fetch);
  render(
    <QueryClientProvider client={client()}>
      <TimelineNodeAttachments
        workspaceID={workspaceID}
        template={template}
        node={node}
        actions={permissions}
        saved={false}
      />
    </QueryClientProvider>,
  );
  expect(screen.getByRole('button', { name: '上传' })).toBeDisabled();
  expect(screen.getByText('请先保存节点')).toBeVisible();
  expect(fetch).not.toHaveBeenCalled();
});

it('详情附件只读，即使拥有全部附件权限也不显示上传或删除', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn(() =>
      Promise.resolve(
        respond({
          items: [attachment],
          total: 1,
          page: 1,
          page_size: 5,
        }),
      ),
    ),
  );
  render(
    <QueryClientProvider client={client()}>
      <TimelineNodeAttachments
        workspaceID={workspaceID}
        template={template}
        node={node}
        actions={permissions}
        readOnly
      />
    </QueryClientProvider>,
  );
  expect(await screen.findByText('虚构材料.pdf')).toBeVisible();
  expect(screen.getByRole('button', { name: '虚构材料.pdf' })).toBeVisible();
  expect(screen.queryByRole('link', { name: /下载/u })).not.toBeInTheDocument();
  expect(screen.queryByLabelText('上传附件')).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: '上传' })).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: '删除' })).not.toBeInTheDocument();
  expect(screen.queryByRole('table')).not.toBeInTheDocument();
});

it('选择文件后直接按节点路径上传，上传期间保护关闭且不推导读取权限', async () => {
  let finish: (response: Response) => void = () => {
    throw new Error('尚未开始上传');
  };
  const pendingResponse = new Promise<Response>((resolve) => {
    finish = resolve;
  });
  const fetch = vi.fn((url: string, options: RequestInit) => {
    if (url.endsWith('/complete')) return Promise.resolve(respond(attachment));
    expect(url).toBe(path);
    expect(options.method).toBe('POST');
    expect(JSON.parse(typeof options.body === 'string' ? options.body : '')).toEqual({
      filename: '虚构材料.pdf',
      size: 7,
      content_type: 'application/pdf',
    });
    return pendingResponse;
  });
  vi.stubGlobal('fetch', fetch);
  const closed = vi.fn();
  const original: TimelineInfo = {
    ...template,
    description: '',
    node_count: 1,
    revision: 1,
    definition: { nodes: [{ ...newNode(), id: node.id, name: node.name }] },
    created_at: '2026-11-06T00:00:00Z',
    updated_at: '2026-11-06T00:00:00Z',
  };
  render(
    <QueryClientProvider client={client()}>
      <TemplateEditor
        workspaceID={workspaceID}
        original={original}
        actions={['cpc.timeline_template.attachment.upload']}
        onClose={closed}
      />
    </QueryClientProvider>,
  );
  expect(fetch).not.toHaveBeenCalled();
  const user = userEvent.setup();
  await user.upload(screen.getByLabelText('上传附件'), new File(['fixture'], '虚构材料.pdf'));
  expect(screen.getByRole('button', { name: '上传' })).toHaveAttribute('aria-busy', 'true');
  expect(screen.getByRole('button', { name: '关闭抽屉' })).toBeDisabled();
  await user.keyboard('{Escape}');
  expect(closed).not.toHaveBeenCalled();
  vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
  await act(async () => {
    finish(respond(preparedUpload()));
    await pendingResponse;
  });
  expect(screen.getByRole('button', { name: '上传' })).toBeEnabled();
  expect(screen.getByRole('button', { name: '关闭抽屉' })).toBeEnabled();

  // 用虚拟时钟推进指示器退出延迟，避免 CI 调度影响真实计时等待。
  await act(async () => {
    await vi.advanceTimersByTimeAsync(loadingExitDelayMs);
  });
  expect(screen.getByRole('button', { name: '上传' })).not.toHaveAttribute('aria-busy', 'true');
  expect(fetch).toHaveBeenCalledTimes(2);
});

it('节点切换后隔离旧附件，未完成或删除中的附件不提供下载', async () => {
  const queryClient = client();
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) =>
      url.includes('/node_second/')
        ? new Promise<Response>(() => {
            // 保持另一节点的请求未完成，检查旧附件不会沿用。
          })
        : Promise.resolve(
            respond({
              items: [
                attachment,
                { ...attachment, id: 12, state: 'pending' },
                { ...attachment, id: 13, state: 'deleting' },
              ],
              total: 3,
              page: 1,
              page_size: 5,
            }),
          ),
    ),
  );
  const view = render(
    <QueryClientProvider client={queryClient}>
      <TimelineNodeAttachments
        workspaceID={workspaceID}
        template={template}
        node={node}
        actions={['cpc.timeline_template.attachment.read']}
      />
    </QueryClientProvider>,
  );
  await screen.findAllByText('虚构材料.pdf');
  expect(screen.getAllByRole('button', { name: '虚构材料.pdf' })).toHaveLength(1);
  view.rerender(
    <QueryClientProvider client={queryClient}>
      <TimelineNodeAttachments
        workspaceID={workspaceID}
        template={template}
        node={{ id: 'node_second', name: '另一节点' }}
        actions={['cpc.timeline_template.attachment.read']}
      />
    </QueryClientProvider>,
  );
  expect(screen.queryByText('虚构材料.pdf')).not.toBeInTheDocument();
});

it('编辑和预览直接展示附件列表，返回编辑后继续保护有附件的节点', async () => {
  const first = { ...newNode(), id: node.id, name: node.name };
  const original: TimelineInfo = {
    ...template,
    description: '',
    node_count: 2,
    revision: 1,
    definition: { nodes: [first, newNode()] },
    created_at: '2026-11-06T00:00:00Z',
    updated_at: '2026-11-06T00:00:00Z',
  };
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) =>
      Promise.resolve(
        respond({
          items: url.includes(`/nodes/${node.id}/`) ? [attachment] : [],
          total: url.includes(`/nodes/${node.id}/`) ? 1 : 0,
          page: 1,
          page_size: 1,
        }),
      ),
    ),
  );
  render(
    <QueryClientProvider client={client()}>
      <TemplateEditor
        workspaceID={workspaceID}
        original={original}
        actions={permissions}
        onClose={vi.fn()}
      />
    </QueryClientProvider>,
  );
  const user = userEvent.setup();
  const baseline = screen.getByRole('checkbox', { name: '基准节点' });
  expect(baseline).not.toBeChecked();
  await user.click(baseline);
  expect(await screen.findByText('虚构材料.pdf')).toBeVisible();
  await user.click(screen.getByRole('button', { name: '预览' }));
  expect(screen.getByText('基准节点')).toBeVisible();
  expect(await screen.findByText('虚构材料.pdf')).toBeVisible();
  expect(screen.queryByRole('button', { name: /节点.*附件/u })).not.toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: '继续编辑' }));
  expect(screen.getByRole('checkbox', { name: '基准节点' })).toBeChecked();
  await user.click(screen.getByRole('button', { name: '删除节点 1' }));
  expect(await screen.findByRole('alert')).toHaveTextContent('节点仍有模板附件');
  expect(screen.getByRole('button', { name: '1. 虚构节点' })).toBeVisible();
});
