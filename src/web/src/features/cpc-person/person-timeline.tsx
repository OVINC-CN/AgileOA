import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  ArrowRightLeft,
  Check,
  Circle,
  Link,
  Pencil,
  RefreshCw,
  SkipForward,
  TriangleAlert,
  X,
} from 'lucide-react';
import { useState } from 'react';

import { ErrorMessage } from '@/components/error-message';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Drawer } from '@/components/ui/drawer';
import { Label } from '@/components/ui/label';
import { SearchSelect } from '@/components/ui/search-select';
import { nodeStatuses, personKey, timelineStatusLabel } from '@/features/cpc-person/api';
import {
  bindPersonTimeline,
  type PersonTimelineNode,
  personTimelineOptions,
} from '@/features/cpc-person/timeline-api';
import { PersonTimelineNodeDetails } from '@/features/cpc-person/timeline-node-details';
import { PersonTimelineNodeEditor } from '@/features/cpc-person/timeline-node-editor';
import { TimelineTemplateSelect } from '@/features/cpc-person/timeline-template-select';
import {
  cancelTimelineTransfer,
  timelineTransferOptions,
  timelineTransferReviewOptions,
} from '@/features/cpc-person/timeline-transfer-api';
import {
  TimelineTransferDrawer,
  type TimelineTransferState,
} from '@/features/cpc-person/timeline-transfer-drawer';
import { TimelineTransferRejectDrawer } from '@/features/cpc-person/timeline-transfer-reject-drawer';
import { BaselineNodeBadge } from '@/features/cpc-timeline/baseline-node-badge';
import { TimelineNodeGroups } from '@/features/cpc-timeline/node-groups';
import { ApiError } from '@/lib/api';
import { useApiAction } from '@/lib/api-action';
import { formatIdentity } from '@/lib/format';
import { cn } from '@/lib/utils';

export function PersonTimelineDrawer({
  workspaceID,
  person,
  canUpdate,
  canReview = false,
  onClose,
}: {
  workspaceID: string;
  person: { id: number; name: string; student_no: string };
  canUpdate: boolean;
  canReview?: boolean;
  onClose: () => void;
}) {
  const client = useQueryClient();
  const [templateID, setTemplateID] = useState<number | null>(null);
  const [nodeStatus, setNodeStatus] = useState('');
  const [abnormalOnly, setAbnormalOnly] = useState(false);
  const [baselineOnly, setBaselineOnly] = useState(false);
  const [transfer, setTransfer] = useState<TimelineTransferState | null>(null);
  const [rejectRevision, setRejectRevision] = useState<number | null>(null);
  const [editing, setEditing] = useState<{
    node: PersonTimelineNode;
    templateRevision: number;
  } | null>(null);
  const options = personTimelineOptions(workspaceID, person.id, canUpdate, !canUpdate && canReview);
  const query = useQuery(options);
  const bind = useApiAction();
  const refresh = useApiAction();
  const prepareTransfer = useApiAction();
  const prepareReview = useApiAction();
  const cancelTransfer = useApiAction();
  const busy =
    bind.pending ||
    refresh.pending ||
    prepareTransfer.pending ||
    prepareReview.pending ||
    cancelTransfer.pending;
  const denied = query.error instanceof ApiError && [401, 403, 404].includes(query.error.status);
  const data = denied ? undefined : query.data;
  const canEdit = canUpdate && data?.can_update && data.status !== 'transferring';
  const isVisible = (node: PersonTimelineNode) =>
    (!nodeStatus || node.status === nodeStatus) &&
    (!abnormalOnly || node.issues.length > 0) &&
    (!baselineOnly || node.is_baseline);
  const visibleNodes = data?.nodes.filter(isVisible) ?? [];
  const invalidate = async () => {
    await Promise.all([
      client.invalidateQueries({ queryKey: personKey(workspaceID) }),
      client.invalidateQueries({ queryKey: ['management', 'audit'] }),
      client.invalidateQueries({ queryKey: ['notifications'] }),
    ]);
  };
  return (
    <Drawer
      title="人员时间线"
      description={
        person.name && person.student_no
          ? formatIdentity(person.name, person.student_no)
          : `人员编号：${String(person.id)}`
      }
      busy={busy || (editing !== null && !!canEdit) || transfer !== null || rejectRevision !== null}
      onClose={onClose}
      footer={
        <>
          <Button
            variant="outline"
            loading={refresh.pending || query.isPending}
            disabled={busy}
            onClick={() => {
              void refresh.run('refresh', async () => {
                await client.query({
                  ...options,
                  staleTime: 0,
                });
              });
            }}
          >
            <RefreshCw aria-hidden="true" />
            刷新
          </Button>
          {!data?.template_id && canEdit ? (
            <Button
              loading={bind.pending}
              disabled={!templateID || busy}
              onClick={() => {
                void bind.run('bind', async () => {
                  if (!templateID) return;
                  await bindPersonTimeline(
                    workspaceID,
                    person.id,
                    templateID,
                    data.person_revision,
                  );
                  await invalidate();
                });
              }}
            >
              <Link aria-hidden="true" />
              绑定模板
            </Button>
          ) : null}
          {data?.template_id && data.has_transfer_targets && canEdit ? (
            <Button
              loading={prepareTransfer.pending}
              disabled={busy}
              onClick={() => {
                void prepareTransfer.run('prepare', async () => {
                  const choices = await client.query(
                    timelineTransferOptions(workspaceID, person.id),
                  );
                  if (!choices.targets.length) throw new Error('当前模板尚未配置转移目标');
                  setTransfer({ mode: 'request', options: choices });
                });
              }}
            >
              <ArrowRightLeft aria-hidden="true" />
              转移时间线
            </Button>
          ) : null}
          {data?.status === 'transferring' && data.can_cancel_transfer && canUpdate ? (
            <Button
              variant="outline"
              loading={cancelTransfer.pending}
              disabled={busy}
              onClick={() => {
                void cancelTransfer.run('cancel', async () => {
                  await cancelTimelineTransfer(workspaceID, person.id, data.person_revision);
                  await invalidate();
                });
              }}
            >
              <X aria-hidden="true" />
              取消
            </Button>
          ) : null}
          {data?.status === 'transferring' && data.can_review_transfer && canReview ? (
            <Button
              variant="outline"
              disabled={busy}
              onClick={() => {
                setRejectRevision(data.person_revision);
              }}
            >
              <X aria-hidden="true" />
              驳回
            </Button>
          ) : null}
          {data?.status === 'transferring' && data.can_review_transfer && canReview ? (
            <Button
              loading={prepareReview.pending}
              disabled={busy}
              onClick={() => {
                void prepareReview.run('review', async () => {
                  const preview = await client.query(
                    timelineTransferReviewOptions(workspaceID, person.id),
                  );
                  setTransfer({ mode: 'review', preview });
                });
              }}
            >
              <Check aria-hidden="true" />
              审核
            </Button>
          ) : null}
        </>
      }
    >
      <div className="space-y-5 p-6">
        <ErrorMessage>{query.error?.message}</ErrorMessage>
        {data && !data.template_id ? (
          <div className="space-y-4">
            <p className="text-sm text-muted-foreground">尚未绑定时间线模板</p>
            {canEdit ? (
              <TimelineTemplateSelect
                workspaceID={workspaceID}
                purpose="update"
                value={templateID}
                onChange={setTemplateID}
                disabled={bind.pending}
              />
            ) : null}
          </div>
        ) : null}
        {data?.template_id ? (
          <>
            <div className="space-y-1">
              <h2 className="text-sm font-semibold wrap-anywhere">{data.template_name}</h2>
              <p
                className={
                  data.status === 'abnormal'
                    ? 'flex items-center gap-2 font-semibold text-danger-foreground'
                    : data.status === 'transferring'
                      ? 'text-sm text-warning-foreground'
                      : 'text-sm text-success-foreground'
                }
              >
                {data.status === 'abnormal' ? (
                  <TriangleAlert aria-hidden="true" className="size-4" />
                ) : null}
                时间线状态：{timelineStatusLabel(data.status)}
              </p>
              {data.status === 'transferring' ? (
                <p className="text-sm wrap-anywhere text-warning-foreground">
                  正在转移到“{data.transfer_target_name}
                  ”，等待审核。取消转移后可继续编辑人员和节点。
                </p>
              ) : null}
              <p className="text-xs text-muted-foreground">填写发生时间后显示校验结果。</p>
              <p className="text-xs text-muted-foreground">
                已完成 {String(data.nodes.filter((node) => node.status === 'completed').length)} /{' '}
                {String(data.nodes.length)}
                {data.nodes.some((node) => node.status === 'skipped')
                  ? ` · 已跳过 ${String(data.nodes.filter((node) => node.status === 'skipped').length)}`
                  : ''}
              </p>
            </div>
            <div className="flex flex-wrap items-end gap-4" role="group" aria-label="节点筛选">
              <div className="w-full space-y-2 sm:w-48">
                <Label htmlFor="person-timeline-node-status">节点状态</Label>
                <SearchSelect
                  id="person-timeline-node-status"
                  label="节点状态"
                  className="w-full"
                  value={nodeStatus}
                  options={[{ value: '', label: '全部状态' }, ...nodeStatuses]}
                  onValueChange={setNodeStatus}
                />
              </div>
              <label className="flex h-8 items-center gap-2 text-sm">
                <Checkbox
                  checked={abnormalOnly}
                  onChange={(event) => {
                    setAbnormalOnly(event.target.checked);
                  }}
                />
                仅看异常节点
              </label>
              <label className="flex h-8 items-center gap-2 text-sm">
                <Checkbox
                  checked={baselineOnly}
                  onChange={(event) => {
                    setBaselineOnly(event.target.checked);
                  }}
                />
                仅看基准节点
              </label>
            </div>
            {visibleNodes.length === 0 ? (
              <p role="status" className="py-6 text-center text-sm text-muted-foreground">
                没有符合筛选条件的节点
              </p>
            ) : null}
            <TimelineNodeGroups
              nodes={data.nodes}
              label="人员时间线节点"
              isVisible={isVisible}
              renderNode={(node, index) => (
                <li
                  key={node.id}
                  className="relative ml-3 border-l border-border pb-6 pl-7 last:border-transparent"
                >
                  <span
                    className={cn(
                      'absolute -left-3 flex size-6 items-center justify-center rounded-full text-xs',
                      node.status === 'completed'
                        ? 'bg-success-foreground text-background'
                        : 'bg-border text-foreground-secondary',
                    )}
                  >
                    {String(index + 1)}
                  </span>
                  <div
                    className={cn(
                      'space-y-3 rounded-xl border p-4',
                      node.issues.length
                        ? 'border-danger-border bg-danger/60'
                        : 'border-border bg-background/80',
                    )}
                  >
                    <TimelineNodeHeader
                      node={node}
                      position={index + 1}
                      onEdit={
                        canEdit && !busy
                          ? () => {
                              setEditing({ node, templateRevision: data.template_revision });
                            }
                          : undefined
                      }
                    />
                    <PersonTimelineNodeDetails
                      workspaceID={workspaceID}
                      personID={person.id}
                      node={node}
                      nodes={data.nodes}
                      editing={canUpdate}
                    />
                  </div>
                </li>
              )}
            />
          </>
        ) : null}
      </div>
      {data && editing && canEdit ? (
        <PersonTimelineNodeEditor
          workspaceID={workspaceID}
          personID={person.id}
          original={editing.node}
          current={data.nodes.find((node) => node.id === editing.node.id) ?? editing.node}
          nodes={data.nodes}
          templateRevision={editing.templateRevision}
          invalidate={invalidate}
          onClose={() => {
            setEditing(null);
          }}
        />
      ) : null}
      {transfer ? (
        <TimelineTransferDrawer
          workspaceID={workspaceID}
          person={person}
          transfer={transfer}
          onClose={() => {
            setTransfer(null);
          }}
        />
      ) : null}
      {rejectRevision !== null ? (
        <TimelineTransferRejectDrawer
          workspaceID={workspaceID}
          person={person}
          revision={rejectRevision}
          invalidate={invalidate}
          onClose={() => {
            setRejectRevision(null);
          }}
        />
      ) : null}
    </Drawer>
  );
}

function TimelineNodeHeader({
  node,
  position,
  onEdit,
}: {
  node: PersonTimelineNode;
  position: number;
  onEdit: (() => void) | undefined;
}) {
  const title = `${String(position)} · ${node.name}`;
  const content = (
    <>
      <span className="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
        <span className="min-w-0 wrap-anywhere">{title}</span>
        {onEdit ? (
          <Pencil aria-hidden="true" className="size-3.5 shrink-0 text-muted-foreground" />
        ) : null}
        {node.is_baseline ? <BaselineNodeBadge /> : null}
      </span>
      <span
        className={cn(
          'inline-flex shrink-0 items-center gap-1 rounded-full px-2 py-1 text-xs font-medium whitespace-nowrap',
          node.status === 'completed'
            ? 'bg-success text-success-foreground'
            : node.status === 'skipped'
              ? 'bg-secondary text-foreground-secondary'
              : 'bg-warning text-warning-foreground',
        )}
      >
        {node.status === 'completed' ? (
          <Check aria-hidden="true" className="size-3.5" />
        ) : node.status === 'skipped' ? (
          <SkipForward aria-hidden="true" className="size-3.5" />
        ) : (
          <Circle aria-hidden="true" className="size-3.5" />
        )}
        {node.status_label}
      </span>
    </>
  );
  return (
    <h3 aria-label={title} className="text-sm font-semibold">
      {onEdit ? (
        <Button
          type="button"
          variant="ghost"
          className="h-auto w-full min-w-0 justify-between gap-2 p-0 text-left font-semibold whitespace-normal hover:bg-transparent"
          aria-label={`编辑节点 ${node.name}`}
          onClick={onEdit}
        >
          {content}
        </Button>
      ) : (
        <span className="flex flex-wrap items-center justify-between gap-2">{content}</span>
      )}
    </h3>
  );
}
