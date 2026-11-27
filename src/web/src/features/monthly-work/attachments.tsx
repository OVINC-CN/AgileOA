import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { RefreshCw } from 'lucide-react';
import { useMemo } from 'react';

import { AttachmentFilename } from '@/components/attachment-filename';
import { ErrorMessage } from '@/components/error-message';
import { ManagementTable } from '@/components/management-table';
import { Button } from '@/components/ui/button';
import { DeleteConfirmation } from '@/components/ui/delete-confirmation';
import { Drawer } from '@/components/ui/drawer';
import { FileUploadButton } from '@/components/ui/file-upload-button';
import {
  deleteWorkAttachment,
  uploadWorkAttachment,
  type WorkAttachment,
  workAttachmentDownload,
  workAttachmentOptions,
  workKey,
  type WorkRecord,
} from '@/features/monthly-work/api';
import { workAction, type WorkCategory } from '@/features/monthly-work/category';
import { ApiError } from '@/lib/api';
import { useApiAction } from '@/lib/api-action';
import { uploadHint, uploadStateLabels, validateUploadFile } from '@/lib/file-upload';
import { useTablePagination } from '@/lib/pagination';
import { recordTimeColumns } from '@/lib/record-time-columns';

interface Props {
  workspaceID: string;
  category?: WorkCategory;
  record: WorkRecord;
  actions: string[];
  isOwner: boolean;
  onClose: () => void;
}

export function WorkAttachments(props: Props) {
  return (
    <AttachmentPanel
      key={`${props.workspaceID}:${props.category ?? 'student'}:${String(props.record.id)}`}
      {...props}
    />
  );
}

function AttachmentPanel({
  workspaceID,
  category = 'student',
  record,
  actions,
  isOwner,
  onClose,
}: Props) {
  const can = (operation: string) => actions.includes(workAction(category, operation));
  const client = useQueryClient();
  const pagination = useTablePagination();
  const upload = useApiAction();
  const remove = useApiAction();
  const refresh = useApiAction();
  const canRead = can('attachment.read');
  const canUpload = isOwner && can('attachment.upload');
  const canDelete = isOwner && can('attachment.delete');
  const options = workAttachmentOptions(
    workspaceID,
    record.id,
    pagination.page,
    pagination.pageSize,
    category,
  );
  const query = useQuery({ ...options, enabled: canRead, placeholderData: keepPreviousData });
  const denied = query.error instanceof ApiError && [401, 403, 404].includes(query.error.status);
  const data = denied || !canRead ? undefined : query.data;
  const invalidate = () => client.invalidateQueries({ queryKey: workKey(workspaceID, category) });
  const renderActions = (item: WorkAttachment) => {
    return (
      <div className="flex items-center gap-1">
        {canDelete ? (
          <DeleteConfirmation
            target={item.filename}
            onConfirm={async () => {
              const result = await remove.run(String(item.id), async () => {
                try {
                  await deleteWorkAttachment(workspaceID, item, category);
                } finally {
                  await invalidate();
                }
              });
              return result.ok;
            }}
            disabled={remove.pending}
          />
        ) : null}
      </div>
    );
  };
  // 保持单元格组件稳定，避免附件刷新时关闭正在查看的预览。
  const columns = useMemo<ColumnDef<WorkAttachment>[]>(
    () => [
      {
        accessorKey: 'filename',
        header: '文件名',
        cell: ({ row }) => (
          <AttachmentFilename
            filename={row.original.filename}
            className="max-w-64"
            downloadURL={
              row.original.state === 'ready'
                ? workAttachmentDownload(workspaceID, row.original.id, category)
                : undefined
            }
          />
        ),
      },
      {
        accessorKey: 'size',
        header: '大小',
        cell: ({ row }) => `${(row.original.size / 1024).toFixed(1)} KB`,
      },
      {
        accessorKey: 'state',
        header: '状态',
        cell: ({ row }) => uploadStateLabels[row.original.state],
      },
      ...recordTimeColumns<WorkAttachment>((record) => record),
      ...(canDelete ? [{ id: 'actions', header: '操作' }] : []),
    ],
    [canDelete, category, workspaceID],
  );
  return (
    <Drawer
      title="月度工作附件"
      description={`${record.date} · ${record.work_item}`}
      busy={upload.pending || remove.pending}
      onClose={onClose}
      footerRefresh={
        canRead ? (
          <Button
            type="button"
            variant="outline"
            loading={refresh.pending || query.isPending}
            onClick={() => {
              void refresh.run('refresh', async () => {
                const result = await query.refetch();
                if (result.error) throw result.error;
              });
            }}
          >
            <RefreshCw aria-hidden="true" />
            刷新
          </Button>
        ) : null
      }
    >
      <div className="space-y-4 p-6">
        {canUpload ? (
          <div className="space-y-2">
            <FileUploadButton
              loading={upload.pending}
              disabled={remove.pending}
              onFileSelect={(file, updateUpload) => {
                void upload.run('upload', async () => {
                  validateUploadFile(file);
                  try {
                    await uploadWorkAttachment(
                      workspaceID,
                      record.id,
                      file,
                      category,
                      updateUpload,
                    );
                  } finally {
                    await invalidate();
                  }
                  updateUpload('complete');
                });
              }}
            />
            <p className="text-xs text-muted-foreground">{uploadHint()}</p>
          </div>
        ) : null}
        {query.error ? <ErrorMessage>{query.error.message}</ErrorMessage> : null}
        {canRead && (query.isPending || (data?.total ?? 0) > 0) ? (
          <div className="border-y border-border">
            <ManagementTable
              tableID={`monthly-work-attachments:${category}`}
              pinnedColumn="filename"
              emptyMessage="暂无附件"
              rowActions={renderActions}
              rowID={(record) => String(record.id)}
              columns={columns}
              data={data?.items ?? []}
              total={data?.total ?? 0}
              page={pagination.page}
              pageSize={pagination.pageSize}
              loading={query.isFetching}
              onPage={async (page) => {
                pagination.setPage(page);
                return client.query({
                  ...workAttachmentOptions(
                    workspaceID,
                    record.id,
                    page,
                    pagination.pageSize,
                    category,
                  ),
                  staleTime: 0,
                });
              }}
              onPageSize={async (size) => {
                pagination.setPageSize(size);
                return client.query({
                  ...workAttachmentOptions(workspaceID, record.id, 1, size, category),
                  staleTime: 0,
                });
              }}
            />
          </div>
        ) : null}
      </div>
    </Drawer>
  );
}
