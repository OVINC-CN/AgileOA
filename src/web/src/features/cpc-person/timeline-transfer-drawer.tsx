import { useQueryClient } from '@tanstack/react-query';
import { ArrowRightLeft, Eye, RefreshCw } from 'lucide-react';
import { useState } from 'react';

import { Button } from '@/components/ui/button';
import { Drawer } from '@/components/ui/drawer';
import { Label } from '@/components/ui/label';
import { SearchSelect } from '@/components/ui/search-select';
import { personKey } from '@/features/cpc-person/api';
import { PersonTimelineNodeDetails } from '@/features/cpc-person/timeline-node-details';
import {
  previewTimelineTransfer,
  requestTimelineTransfer,
  reviewTimelineTransfer,
  type TimelineTransferOptions,
  type TimelineTransferPreview,
  timelineTransferReviewOptions,
} from '@/features/cpc-person/timeline-transfer-api';
import { timelineValueLabel } from '@/features/cpc-person/timeline-values';
import { BaselineNodeBadge } from '@/features/cpc-timeline/baseline-node-badge';
import { TimelineNodeGroups } from '@/features/cpc-timeline/node-groups';
import { useApiAction } from '@/lib/api-action';
import { cn } from '@/lib/utils';

export type TimelineTransferState =
  | { mode: 'request'; options: TimelineTransferOptions }
  | { mode: 'review'; preview: TimelineTransferPreview };

export function TimelineTransferDrawer({
  workspaceID,
  personID,
  personLabel,
  transfer,
  onClose,
}: {
  workspaceID: string;
  personID: number;
  personLabel: string | undefined;
  transfer: TimelineTransferState;
  onClose: () => void;
}) {
  const client = useQueryClient();
  const [targetID, setTargetID] = useState('');
  const [preview, setPreview] = useState<TimelineTransferPreview | null>(
    transfer.mode === 'review' ? transfer.preview : null,
  );
  const [stalePreview, setStalePreview] = useState(false);
  const prepare = useApiAction();
  const confirm = useApiAction();
  const busy = prepare.pending || confirm.pending;
  const reviewing = transfer.mode === 'review';
  const choices = transfer.mode === 'request' ? transfer.options : undefined;
  return (
    <Drawer
      title={reviewing ? '审核时间线转移' : '转移人员时间线'}
      description={personLabel}
      truncateDescription
      onClose={onClose}
      busy={busy}
      footer={
        <>
          <Button variant="secondary" disabled={busy} onClick={onClose}>
            取消
          </Button>
          <Button
            variant="outline"
            loading={prepare.pending}
            disabled={
              confirm.pending ||
              (!reviewing &&
                (!targetID || !choices?.targets.some((item) => item.value === targetID)))
            }
            onClick={() => {
              void prepare.run('preview', async () => {
                try {
                  const result = reviewing
                    ? await client.query(timelineTransferReviewOptions(workspaceID, personID))
                    : await previewTimelineTransfer(workspaceID, personID, Number(targetID));
                  setPreview(result);
                  setStalePreview(false);
                } catch (error) {
                  setStalePreview(true);
                  throw error;
                }
              });
            }}
          >
            {reviewing ? <RefreshCw aria-hidden="true" /> : <Eye aria-hidden="true" />}
            {reviewing ? '刷新' : preview ? '重新预览' : '预览转移'}
          </Button>
          {preview ? (
            <Button
              loading={confirm.pending}
              disabled={prepare.pending || stalePreview}
              onClick={() => {
                void confirm.run('confirm', async () => {
                  try {
                    if (reviewing) await reviewTimelineTransfer(workspaceID, personID, preview);
                    else await requestTimelineTransfer(workspaceID, personID, preview);
                  } catch (error) {
                    setStalePreview(true);
                    throw error;
                  }
                  await Promise.all([
                    client.invalidateQueries({ queryKey: personKey(workspaceID) }),
                    client.invalidateQueries({ queryKey: ['management', 'audit'] }),
                    client.invalidateQueries({ queryKey: ['notifications'] }),
                  ]);
                  onClose();
                });
              }}
            >
              <ArrowRightLeft aria-hidden="true" />
              {reviewing ? '通过' : '发起转移'}
            </Button>
          ) : null}
        </>
      }
    >
      <div className="space-y-5 p-6">
        {choices ? (
          <>
            <p className="text-sm wrap-anywhere">当前模板：{choices.source_template_name}</p>
            {choices.targets.length ? (
              <div className="space-y-2">
                <Label>转移到</Label>
                <SearchSelect
                  label="转移目标模板"
                  value={targetID}
                  options={choices.targets}
                  className="w-full"
                  disabled={busy}
                  onValueChange={(value) => {
                    setTargetID(value);
                    setPreview(null);
                    setStalePreview(false);
                  }}
                />
              </div>
            ) : (
              <p role="status" className="text-sm text-muted-foreground">
                当前模板尚未配置转移目标
              </p>
            )}
          </>
        ) : null}
        {preview ? (
          <>
            <div className="space-y-2 border-t pt-4">
              <h2 className="text-sm font-semibold wrap-anywhere">
                转移后预览 · {preview.timeline.template_name}
              </h2>
              <p className="text-xs text-muted-foreground">
                {reviewing
                  ? '审核通过后按以下内容重建时间线，人员附件随映射保留。'
                  : '发起后进入转移中，审核通过后按以下内容重建时间线，人员附件随映射保留。'}
              </p>
              <p
                className={cn(
                  'text-sm',
                  preview.timeline.status === 'abnormal'
                    ? 'text-danger-foreground'
                    : 'text-success-foreground',
                )}
              >
                {preview.timeline.status === 'abnormal' ? '时间线状态：异常' : '时间线状态：正常'}
              </p>
            </div>
            {preview.discarded_fields.length ? (
              <section
                className="space-y-3 rounded-lg border border-warning-border bg-warning p-4"
                aria-label="未映射的填写内容"
              >
                <h3 className="text-sm font-semibold text-warning-foreground">
                  以下已填写内容未映射，转移后不再保留
                </h3>
                <dl className="space-y-2 text-sm">
                  {preview.discarded_fields.map((field, index) => (
                    <div key={index}>
                      <dt className="text-xs wrap-anywhere text-muted-foreground">
                        {field.node_name} · {field.field_name}
                      </dt>
                      <dd className="wrap-anywhere whitespace-pre-wrap">
                        {timelineValueLabel(field.value)}
                      </dd>
                    </div>
                  ))}
                </dl>
              </section>
            ) : null}
            <TimelineNodeGroups
              nodes={preview.timeline.nodes}
              label="转移后的时间线节点"
              renderNode={(node, index) => (
                <li
                  key={node.id}
                  className="relative ml-3 border-l border-border pb-6 pl-7 last:border-transparent"
                >
                  <span className="absolute -left-3 flex size-6 items-center justify-center rounded-full bg-border text-xs text-foreground-secondary">
                    {String(index + 1)}
                  </span>
                  <div
                    className={cn(
                      'space-y-3 rounded-xl border bg-background/80 p-4',
                      node.issues.length ? 'border-danger-border' : 'border-border',
                    )}
                  >
                    <div className="flex flex-wrap items-center gap-2">
                      <h3 className="min-w-0 text-sm font-semibold wrap-anywhere">{node.name}</h3>
                      {node.is_baseline ? <BaselineNodeBadge /> : null}
                      <span className="ml-auto text-xs text-muted-foreground">
                        {node.status_label}
                      </span>
                    </div>
                    <PersonTimelineNodeDetails
                      workspaceID={workspaceID}
                      personID={personID}
                      node={node}
                      nodes={preview.timeline.nodes}
                      editing
                      preview
                    />
                  </div>
                </li>
              )}
            />
          </>
        ) : null}
      </div>
    </Drawer>
  );
}
