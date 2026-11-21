import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';
import { RefreshCw, Trash2 } from 'lucide-react';
import { useEffect, useState } from 'react';

import { AttachmentFilename } from '@/components/attachment-filename';
import { ErrorMessage } from '@/components/error-message';
import { ManagementPagination } from '@/components/management-pagination';
import { TableActionButton } from '@/components/table-action-button';
import { Button } from '@/components/ui/button';
import { Drawer } from '@/components/ui/drawer';
import { FileUploadButton } from '@/components/ui/file-upload-button';
import {
  deleteTimelineAttachment,
  type TimelineAttachment,
  timelineAttachmentDownload,
  timelineAttachmentKey,
  timelineAttachmentOptions,
  type TimelineSummary,
  uploadTimelineAttachment,
} from '@/features/cpc-timeline/api';
import type { TimelineNode } from '@/features/cpc-timeline/model';
import { ApiError } from '@/lib/api';
import { useApiAction } from '@/lib/api-action';
import { getConfig } from '@/lib/config';
import { uploadStateLabels } from '@/lib/file-upload';
import { useLoadingDelay } from '@/lib/loading';
import { showMessage } from '@/lib/messages';
import { useTablePagination } from '@/lib/pagination';

interface Props {
  workspaceID: string;
  template: Pick<TimelineSummary, 'id' | 'name'> | null;
  node: Pick<TimelineNode, 'id' | 'name'>;
  actions: string[];
  saved?: boolean;
  onBusyChange?: (busy: boolean) => void;
  readOnly?: boolean;
}

export function TimelineNodeAttachments(props: Props) {
  return (
    <AttachmentPanel
      key={`${props.workspaceID}:${String(props.template?.id ?? 0)}:${props.node.id}`}
      {...props}
    />
  );
}

function AttachmentPanel({
  workspaceID,
  template,
  node,
  actions,
  saved = true,
  onBusyChange,
  readOnly = false,
}: Props) {
  const can = (operation: string) =>
    (!readOnly || operation === 'read') &&
    actions.includes(`cpc.timeline_template.attachment.${operation}`);
  const templateID = template?.id ?? 0;
  const available = templateID > 0 && saved;
  const canRead = can('read') && available;
  const client = useQueryClient();
  const pagination = useTablePagination();
  const [deleting, setDeleting] = useState<TimelineAttachment | null>(null);
  const upload = useApiAction();
  const remove = useApiAction();
  const refresh = useApiAction();
  const query = useQuery({
    ...timelineAttachmentOptions(
      workspaceID,
      templateID,
      node.id,
      pagination.page,
      pagination.pageSize,
    ),
    enabled: canRead,
    placeholderData: keepPreviousData,
  });
  const denied = query.error instanceof ApiError && [401, 403, 404].includes(query.error.status);
  const data = denied || !canRead ? undefined : query.data;
  const loading = useLoadingDelay(query.isFetching);
  const busy = upload.pending || remove.pending || (deleting !== null && can('delete'));
  useEffect(() => {
    onBusyChange?.(busy);
  }, [busy, onBusyChange]);
  useEffect(() => () => onBusyChange?.(false), [onBusyChange]);
  const refreshPage = (page: number, size = pagination.pageSize) =>
    client.query({
      ...timelineAttachmentOptions(workspaceID, templateID, node.id, page, size),
      staleTime: 0,
    });
  const invalidate = async (page: number) => {
    await client.invalidateQueries({
      queryKey: timelineAttachmentKey(workspaceID, templateID, node.id),
      refetchType: 'none',
    });
    if (canRead) await refreshPage(page);
  };
  return (
    <>
      {query.error ? <ErrorMessage>{query.error.message}</ErrorMessage> : null}
      {can('upload') || (data?.total ?? 0) > 0 ? (
        <section aria-label={`${node.name}的附件`} className="min-w-0 space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h4 className="text-sm font-medium">节点附件</h4>
            <div className="flex items-center gap-2">
              {canRead && !readOnly ? (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  disabled={upload.pending || remove.pending}
                  loading={refresh.pending || query.isPending}
                  onClick={() => {
                    void refresh.run('refresh', () => refreshPage(pagination.page));
                  }}
                >
                  <RefreshCw aria-hidden="true" />
                  刷新
                </Button>
              ) : null}
              {can('upload') ? (
                <FileUploadButton
                  disabled={!available || remove.pending}
                  loading={upload.pending}
                  onFileSelect={(file) => {
                    void upload.run('upload', async () => {
                      if (!available) return;
                      if (file.size < 1 || file.size > getConfig().fileMaxUploadBytes) {
                        throw new Error('文件不能为空或超过上传大小限制');
                      }
                      try {
                        await uploadTimelineAttachment(workspaceID, templateID, node.id, file);
                        showMessage('附件上传成功');
                      } finally {
                        pagination.setPage(1);
                        await invalidate(1);
                      }
                    });
                  }}
                />
              ) : null}
            </div>
          </div>
          <div className="space-y-3">
            {can('upload') && !available ? (
              <p className="text-xs text-muted-foreground">保存模板后可上传节点附件</p>
            ) : null}
            {data?.items.length ? (
              <ul className="space-y-1 text-sm" aria-busy={loading}>
                {data.items.map((attachment) => (
                  <li key={attachment.id} className="flex min-w-0 items-center gap-2">
                    <AttachmentFilename
                      filename={attachment.filename}
                      className="flex-1"
                      downloadURL={
                        attachment.state === 'ready'
                          ? timelineAttachmentDownload(workspaceID, attachment.id)
                          : undefined
                      }
                    />
                    {attachment.state !== 'ready' ? (
                      <span className="shrink-0 text-xs text-muted-foreground">
                        {uploadStateLabels[attachment.state]}
                      </span>
                    ) : null}
                    <div className="flex shrink-0 items-center gap-1">
                      {can('delete') ? (
                        <TableActionButton
                          type="button"
                          disabled={upload.pending || remove.pending}
                          loading={remove.isPending(String(attachment.id))}
                          onClick={() => {
                            setDeleting(attachment);
                          }}
                        >
                          <Trash2 aria-hidden="true" />
                          删除
                        </TableActionButton>
                      ) : null}
                    </div>
                  </li>
                ))}
              </ul>
            ) : null}
            {data && data.total > pagination.pageSize ? (
              <ManagementPagination
                total={data.total}
                page={pagination.page}
                pageSize={pagination.pageSize}
                loading={query.isFetching || upload.pending || remove.pending}
                onPage={async (page) => {
                  pagination.setPage(page);
                  return refreshPage(page);
                }}
                onPageSize={async (size) => {
                  pagination.setPageSize(size);
                  return refreshPage(1, size);
                }}
              />
            ) : null}
            {available && !canRead ? (
              <p className="text-xs text-muted-foreground">查看和下载附件需要附件读取权限</p>
            ) : null}
          </div>
        </section>
      ) : null}
      {deleting && can('delete') ? (
        <Drawer
          title="删除时间线节点附件"
          description={deleting.filename}
          busy={remove.pending}
          onClose={() => {
            setDeleting(null);
          }}
          footer={
            <>
              <Button
                type="button"
                variant="secondary"
                disabled={remove.pending}
                onClick={() => {
                  setDeleting(null);
                }}
              >
                取消
              </Button>
              <Button
                type="button"
                variant="destructive"
                loading={remove.pending}
                onClick={() => {
                  void remove.run(String(deleting.id), async () => {
                    const page = Math.max(
                      1,
                      Math.min(
                        pagination.page,
                        Math.ceil(((data?.total ?? 1) - 1) / pagination.pageSize),
                      ),
                    );
                    let removed = false;
                    try {
                      await deleteTimelineAttachment(workspaceID, deleting);
                      removed = true;
                    } finally {
                      const targetPage = removed ? page : pagination.page;
                      pagination.setPage(targetPage);
                      await invalidate(targetPage);
                    }
                    setDeleting(null);
                  });
                }}
              >
                <Trash2 aria-hidden="true" />
                确认删除
              </Button>
            </>
          }
        >
          <p className="p-6 text-sm">将永久删除文件及附件记录，历史操作审计会保留。</p>
        </Drawer>
      ) : null}
    </>
  );
}
