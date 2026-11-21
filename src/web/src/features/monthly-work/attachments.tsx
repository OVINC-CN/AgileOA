import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { RefreshCw, Trash2 } from 'lucide-react';
import { useState } from 'react';

import { AttachmentFilename } from '@/components/attachment-filename';
import { ErrorMessage } from '@/components/error-message';
import { ManagementTable } from '@/components/management-table';
import { TableActionButton } from '@/components/table-action-button';
import { Button } from '@/components/ui/button';
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
  const [deleting, setDeleting] = useState<WorkAttachment | null>(null);
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
  const columns: ColumnDef<WorkAttachment>[] = [
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
    {
      id: 'actions',
      header: '操作',
      cell: ({ row }) => {
        const item = row.original;
        return (
          <div className="flex items-center gap-1">
            {canDelete ? (
              <TableActionButton
                type="button"
                disabled={remove.pending}
                loading={remove.isPending(String(item.id))}
                onClick={() => {
                  setDeleting(item);
                }}
              >
                <Trash2 aria-hidden="true" />
                删除
              </TableActionButton>
            ) : null}
          </div>
        );
      },
    },
  ];
  return (
    <>
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
                onFileSelect={(file) => {
                  void upload.run('upload', async () => {
                    validateUploadFile(file);
                    try {
                      await uploadWorkAttachment(workspaceID, record.id, file, category);
                    } finally {
                      await invalidate();
                    }
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
                columns={columns.filter((column) => column.id !== 'actions' || canDelete)}
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
      {deleting && canDelete ? (
        <Drawer
          title="删除月度工作附件"
          description={deleting.filename}
          busy={remove.pending}
          onClose={() => {
            setDeleting(null);
          }}
          footerDismiss={
            <Button
              type="button"
              variant="cancel"
              disabled={remove.pending}
              onClick={() => {
                setDeleting(null);
              }}
            >
              取消
            </Button>
          }
          footer={
            <Button
              type="button"
              loading={remove.pending}
              onClick={() => {
                void (async () => {
                  const result = await remove.run(String(deleting.id), async () => {
                    try {
                      await deleteWorkAttachment(workspaceID, deleting, category);
                    } finally {
                      await invalidate();
                    }
                  });
                  if (result.ok) setDeleting(null);
                })();
              }}
            >
              <Trash2 aria-hidden="true" />
              确认删除
            </Button>
          }
        >
          <p className="p-6 text-sm">将永久删除文件及其记录。</p>
        </Drawer>
      ) : null}
    </>
  );
}
