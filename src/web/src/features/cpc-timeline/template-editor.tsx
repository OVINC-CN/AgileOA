import { useQueryClient } from '@tanstack/react-query';
import { Eye, Pencil, Save } from 'lucide-react';
import { useId, useRef, useState } from 'react';

import { Button } from '@/components/ui/button';
import { DeleteConfirmation } from '@/components/ui/delete-confirmation';
import { Drawer, type DrawerHandle } from '@/components/ui/drawer';
import { DrawerTabs } from '@/components/ui/drawer-tabs';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { personKey } from '@/features/cpc-person/api';
import {
  saveTimeline,
  timelineAttachmentOptions,
  type TimelineInfo,
  timelineKey,
} from '@/features/cpc-timeline/api';
import { TimelineNodeAttachments } from '@/features/cpc-timeline/attachments';
import { newNode, type TimelineInput } from '@/features/cpc-timeline/model';
import { NodeEditor } from '@/features/cpc-timeline/node-editor';
import { NodeList } from '@/features/cpc-timeline/node-list';
import { TemplatePreview } from '@/features/cpc-timeline/template-preview';
import { TemplateTransferEditor } from '@/features/cpc-timeline/transfer-editor';
import { useApiAction } from '@/lib/api-action';
import { cn } from '@/lib/utils';

export function TemplateEditor({
  workspaceID,
  original,
  actions,
  onClose,
}: {
  workspaceID: string;
  original: TimelineInfo | null;
  actions: string[];
  onClose: () => void;
}) {
  const [values, setValues] = useState<TimelineInput>(() =>
    original
      ? {
          name: original.name,
          description: original.description,
          definition: structuredClone(original.definition),
          transfers: structuredClone(original.transfers ?? []),
        }
      : { name: '', description: '', definition: { nodes: [newNode()] } },
  );
  const [selectedID, setSelectedID] = useState(values.definition.nodes[0]?.id ?? '');
  const [focusNodeName, setFocusNodeName] = useState(false);
  const [preview, setPreview] = useState(false);
  const [section, setSection] = useState<'nodes' | 'transfers'>('nodes');
  const [checkingNode, setCheckingNode] = useState(false);
  const [attachmentBusy, setAttachmentBusy] = useState(false);
  const tabsID = useId();
  const drawer = useRef<DrawerHandle>(null);
  const save = useApiAction();
  const client = useQueryClient();
  const nodes = values.definition.nodes;
  const selected = nodes.find((node) => node.id === selectedID) ?? nodes[0];
  const saved = (id: string) => original?.definition.nodes.some((node) => node.id === id) ?? false;
  const renderAttachments = (node: (typeof nodes)[number]) => (
    <TimelineNodeAttachments
      workspaceID={workspaceID}
      template={original}
      node={node}
      actions={actions}
      saved={saved(node.id)}
      onBusyChange={setAttachmentBusy}
    />
  );
  const removedNodes =
    original?.definition.nodes.filter((node) => !nodes.some((item) => item.id === node.id)) ?? [];
  const submit = async (confirmed: boolean) => {
    const result = await save.run('save', async () => {
      await saveTimeline(workspaceID, original, values, confirmed);
      await Promise.all([
        client.invalidateQueries({ queryKey: timelineKey(workspaceID) }),
        client.invalidateQueries({ queryKey: personKey(workspaceID) }),
      ]);
      drawer.current?.close();
    });
    return result.ok;
  };
  const busy = save.pending || checkingNode || attachmentBusy;
  const saveButton = (
    <Button type="submit" loading={save.pending} disabled={checkingNode || attachmentBusy}>
      <Save aria-hidden="true" />
      保存
    </Button>
  );
  return (
    <Drawer
      ref={drawer}
      title={original ? '编辑时间线模板' : '新增时间线模板'}
      description="配置有序节点、自定义字段和非阻断时间提示规则"
      onClose={onClose}
      busy={busy}
      size="full"
      contentClassName={section === 'transfers' ? 'flex flex-col' : 'lg:overflow-hidden'}
      form={{
        noValidate: true,
        onSubmit: (event) => {
          event.preventDefault();
          if (busy) return;
          if (!removedNodes.length) void submit(false);
        },
      }}
      footerNavigation={
        <DrawerTabs
          id={tabsID}
          label="模板配置"
          value={section}
          items={[
            { value: 'nodes', label: '节点配置' },
            { value: 'transfers', label: '转移配置' },
          ]}
          disabled={busy}
          onChange={(value) => {
            if (value === 'nodes' || value === 'transfers') setSection(value);
          }}
        />
      }
      dismissLabel="取消"
      footer={
        <>
          {section === 'nodes' ? (
            <Button
              type="button"
              variant="outline"
              disabled={busy}
              onClick={() => {
                setPreview(!preview);
              }}
            >
              {preview ? <Pencil aria-hidden="true" /> : <Eye aria-hidden="true" />}
              {preview ? '继续编辑' : '预览'}
            </Button>
          ) : null}

          {removedNodes.length ? (
            <DeleteConfirmation
              target={`节点 ${removedNodes.map((node) => node.name).join('、')} 及绑定人员的对应节点`}
              type="submit"
              variant="default"
              size="default"
              label="保存"
              icon={Save}
              disabled={busy}
              onConfirm={() => submit(true)}
            />
          ) : (
            saveButton
          )}
        </>
      }
    >
      <fieldset
        disabled={busy}
        className={cn(
          'flex min-h-full min-w-0 flex-col',
          section === 'nodes'
            ? 'lg:grid lg:h-full lg:min-h-0 lg:grid-cols-[280px_minmax(0,1fr)] lg:grid-rows-[minmax(0,1fr)]'
            : 'flex-1 md:min-h-0',
        )}
      >
        {section === 'nodes' ? (
          <aside
            aria-label="模板信息与节点"
            className="flex min-w-0 shrink-0 flex-col border-b border-border-subtle lg:min-h-0 lg:border-r lg:border-b-0"
          >
            <section aria-label="模板信息" className="shrink-0 p-4 sm:p-5">
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-1">
                <div className="space-y-2">
                  <Label htmlFor={`${tabsID}-name`} required className="leading-5">
                    模板名称
                  </Label>
                  <Input
                    id={`${tabsID}-name`}
                    aria-label="模板名称"
                    placeholder="例如：党员发展时间线"
                    required
                    maxLength={100}
                    value={values.name}
                    onChange={(event) => {
                      setValues({ ...values, name: event.target.value });
                    }}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor={`${tabsID}-description`} className="leading-5">
                    模板说明
                  </Label>
                  <Textarea
                    id={`${tabsID}-description`}
                    aria-label="模板说明"
                    placeholder="说明模板用途或适用范围"
                    rows={3}
                    className="min-h-20"
                    maxLength={1000}
                    value={values.description}
                    onChange={(event) => {
                      setValues({ ...values, description: event.target.value });
                    }}
                  />
                </div>
              </div>
            </section>
            {!preview ? (
              <section
                aria-label="时间线节点"
                className="min-w-0 border-t border-border-subtle p-4 sm:p-5 lg:min-h-0 lg:flex-1"
              >
                <NodeList
                  nodes={nodes}
                  selectedID={selected?.id ?? ''}
                  onSelect={(id) => {
                    setFocusNodeName(!nodes.some((node) => node.id === id));
                    setSelectedID(id);
                  }}
                  disabled={busy}
                  beforeRemove={async (node) => {
                    if (
                      !original ||
                      !saved(node.id) ||
                      !actions.includes('cpc.timeline_template.attachment.read')
                    )
                      return;
                    setCheckingNode(true);
                    try {
                      const attachments = await client.query({
                        ...timelineAttachmentOptions(workspaceID, original.id, node.id, 1, 1),
                        staleTime: 0,
                      });
                      if (attachments.total > 0) {
                        throw new Error('节点仍有模板附件，请先删除该节点的模板附件');
                      }
                    } finally {
                      setCheckingNode(false);
                    }
                  }}
                  onChange={(next) => {
                    setValues({ ...values, definition: { nodes: next } });
                  }}
                />
              </section>
            ) : null}
          </aside>
        ) : null}
        <div
          role="tabpanel"
          id={`${tabsID}-panel-${section}`}
          aria-labelledby={`${tabsID}-tab-${section}`}
          className="flex min-w-0 flex-1 flex-col md:min-h-0"
        >
          {removedNodes.length ? (
            <p className="m-4 shrink-0 rounded-lg border border-warning-border bg-warning p-3 text-sm font-medium text-warning-foreground sm:mx-6">
              保存将同步删除所有绑定人员的 {String(removedNodes.length)}{' '}
              个对应节点及填写内容，无法恢复。
            </p>
          ) : null}
          {section === 'transfers' ? (
            <section
              aria-label="模板转移配置"
              className="flex min-w-0 flex-1 flex-col p-4 sm:p-6 md:min-h-0"
            >
              <div className="mx-auto flex w-full max-w-7xl min-w-0 flex-1 flex-col md:min-h-0">
                <TemplateTransferEditor
                  workspaceID={workspaceID}
                  templateID={original?.id}
                  templateName={values.name}
                  nodes={nodes}
                  transfers={values.transfers ?? []}
                  onChange={(transfers) => {
                    setValues({ ...values, transfers });
                  }}
                />
              </div>
            </section>
          ) : preview ? (
            <section
              aria-label="模板预览"
              className="min-w-0 flex-1 p-4 sm:p-6 lg:min-h-0 lg:overflow-y-auto lg:overscroll-contain"
            >
              <div className="mx-auto max-w-5xl">
                <TemplatePreview
                  nodes={nodes}
                  workspaceID={workspaceID}
                  template={original}
                  actions={actions}
                />
              </div>
            </section>
          ) : (
            <section
              key={selected?.id}
              aria-label="节点配置"
              className="min-w-0 flex-1 p-4 sm:p-6 lg:min-h-0 lg:overflow-y-auto lg:overscroll-contain"
            >
              {selected ? (
                <div className="mx-auto w-full max-w-6xl">
                  <NodeEditor
                    node={selected}
                    nodes={nodes}
                    autoFocusName={focusNodeName}
                    attachments={renderAttachments(selected)}
                    onChange={(node) => {
                      setValues({
                        ...values,
                        definition: {
                          nodes: nodes.map((item) => (item.id === node.id ? node : item)),
                        },
                      });
                    }}
                  />
                </div>
              ) : null}
            </section>
          )}
        </div>
      </fieldset>
    </Drawer>
  );
}
