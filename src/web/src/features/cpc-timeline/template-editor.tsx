import { useQueryClient } from '@tanstack/react-query';
import { Eye, Pencil, Save, TriangleAlert } from 'lucide-react';
import { useId, useState } from 'react';

import { Button } from '@/components/ui/button';
import { Drawer } from '@/components/ui/drawer';
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

const basicFieldClassName = 'h-8 min-h-8 py-1 leading-[22px]';

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
  const [preview, setPreview] = useState(false);
  const [section, setSection] = useState<'nodes' | 'transfers'>('nodes');
  const [checkingNode, setCheckingNode] = useState(false);
  const [confirmingDeletion, setConfirmingDeletion] = useState(false);
  const [attachmentBusy, setAttachmentBusy] = useState(false);
  const tabsID = useId();
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
  const submit = (confirmed: boolean) => {
    void save.run('save', async () => {
      await saveTimeline(workspaceID, original, values, confirmed);
      await Promise.all([
        client.invalidateQueries({ queryKey: timelineKey(workspaceID) }),
        client.invalidateQueries({ queryKey: personKey(workspaceID) }),
      ]);
      onClose();
    });
  };
  const busy = save.pending || checkingNode || confirmingDeletion || attachmentBusy;
  return (
    <Drawer
      title={original ? '编辑时间线模板' : '新增时间线模板'}
      description="配置有序节点、自定义字段和非阻断时间提示规则"
      onClose={onClose}
      busy={busy}
      size="full"
      contentClassName="overflow-hidden"
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
      form={{
        noValidate: true,
        onSubmit: (event) => {
          event.preventDefault();
          if (busy) return;
          if (removedNodes.length) setConfirmingDeletion(true);
          else submit(false);
        },
      }}
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
          <Button type="button" variant="secondary" disabled={busy} onClick={onClose}>
            取消
          </Button>
          <Button type="submit" loading={save.pending} disabled={checkingNode || attachmentBusy}>
            <Save aria-hidden="true" />
            保存
          </Button>
        </>
      }
    >
      <fieldset
        role="tabpanel"
        id={`${tabsID}-panel-${section}`}
        aria-labelledby={`${tabsID}-tab-${section}`}
        disabled={busy}
        className="flex h-full min-h-0 min-w-0 flex-col"
      >
        {section === 'nodes' || removedNodes.length > 0 ? (
          <div className="shrink-0 space-y-3 border-b border-border-subtle p-4 sm:px-6">
            {section === 'nodes' ? (
              <div className="grid gap-3 sm:grid-cols-[minmax(180px,1fr)_minmax(0,2fr)]">
                <Label className="block space-y-2">
                  <span>模板名称</span>
                  <Input
                    aria-label="模板名称"
                    className={basicFieldClassName}
                    required
                    maxLength={100}
                    value={values.name}
                    onChange={(event) => {
                      setValues({ ...values, name: event.target.value });
                    }}
                  />
                </Label>
                <Label className="block space-y-2">
                  <span>模板说明</span>
                  <Textarea
                    aria-label="模板说明"
                    rows={1}
                    className={basicFieldClassName}
                    maxLength={1000}
                    value={values.description}
                    onChange={(event) => {
                      setValues({ ...values, description: event.target.value });
                    }}
                  />
                </Label>
              </div>
            ) : null}
            {removedNodes.length ? (
              <p className="rounded-lg border border-warning-border bg-warning p-3 text-sm font-medium text-warning-foreground">
                保存将同步删除所有绑定人员的 {String(removedNodes.length)}{' '}
                个对应节点及填写内容，无法恢复。
              </p>
            ) : null}
          </div>
        ) : null}
        {section === 'transfers' ? (
          <section aria-label="模板转移配置" className="min-h-0 flex-1 overflow-hidden p-4 sm:p-6">
            <div className="mx-auto h-full min-h-0 max-w-7xl">
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
            className="min-h-0 flex-1 overflow-y-auto overscroll-contain p-4 sm:p-6"
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
          <div className="grid min-h-0 flex-1 grid-rows-[minmax(0,0.7fr)_minmax(0,1.3fr)] overflow-hidden sm:grid-cols-[minmax(240px,300px)_minmax(0,1fr)] sm:grid-rows-1">
            <section
              aria-label="时间线节点"
              className="min-h-0 min-w-0 border-b border-border-subtle bg-muted/50 p-4 sm:border-r sm:border-b-0 sm:p-5"
            >
              <NodeList
                nodes={nodes}
                selectedID={selected?.id ?? ''}
                onSelect={setSelectedID}
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
            <section
              key={selected?.id}
              aria-label="节点配置"
              className="min-h-0 min-w-0 overflow-y-auto overscroll-contain p-4 sm:p-6"
            >
              {selected ? (
                <div className="mx-auto w-full max-w-5xl">
                  <NodeEditor
                    node={selected}
                    nodes={nodes}
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
          </div>
        )}
      </fieldset>
      {confirmingDeletion ? (
        <Drawer
          title="确认同步删除人员节点"
          description="此操作影响所有绑定本模板的人员，无法恢复"
          busy={save.pending}
          onClose={() => {
            setConfirmingDeletion(false);
          }}
          footer={
            <>
              <Button
                type="button"
                variant="secondary"
                disabled={save.pending}
                onClick={() => {
                  setConfirmingDeletion(false);
                }}
              >
                返回检查
              </Button>
              <Button
                type="button"
                variant="destructive"
                loading={save.pending}
                onClick={() => {
                  submit(true);
                }}
              >
                <TriangleAlert aria-hidden="true" />
                确认删除并保存
              </Button>
            </>
          }
        >
          <div className="space-y-4 p-6 text-sm">
            <p className="font-semibold text-danger-foreground">
              以下节点、所有绑定人员的对应节点、填写内容及人员附件记录将同步删除：
            </p>
            <ul className="list-disc space-y-1 pl-5">
              {removedNodes.map((node) => (
                <li key={node.id} className="wrap-anywhere">
                  {node.name}
                </li>
              ))}
            </ul>
            <p>节点的模板附件须先删除；保存后将一并移除关联的人员填写内容和附件记录，无法恢复。</p>
          </div>
        </Drawer>
      ) : null}
    </Drawer>
  );
}
