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
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { SearchSelect } from '@/components/ui/search-select';
import {
  type Attachment,
  attachmentDownload,
  attachmentListOptions,
  deleteAttachment,
  projectKey,
  projectOptions,
  uploadAttachment,
} from '@/features/project/api';
import { ApiError } from '@/lib/api';
import { useApiAction } from '@/lib/api-action';
import { uploadHint, uploadStateLabels, validateUploadFile } from '@/lib/file-upload';
import { useTablePagination } from '@/lib/pagination';
import { recordTimeColumns } from '@/lib/record-time-columns';

export function AttachmentManager({
  workspaceID,
  projectID,
  actions,
  onClose,
}: {
  workspaceID: string;
  projectID?: number;
  actions: string[];
  onClose: () => void;
}) {
  const can = (action: string) => actions.includes(action);
  const client = useQueryClient();
  const pagination = useTablePagination();
  const [uploadProjectID, setUploadProjectID] = useState(0);
  const targetProject = projectID ?? uploadProjectID;
  const projects = useQuery({
    ...projectOptions(workspaceID),
    enabled: !projectID && can('project.attachment.upload') && can('project.list'),
  });
  const [deleting, setDeleting] = useState<Attachment | null>(null);
  const upload = useApiAction();
  const remove = useApiAction();
  const refresh = useApiAction();
  const options = attachmentListOptions(
    workspaceID,
    pagination.page,
    pagination.pageSize,
    projectID,
  );
  const query = useQuery({
    ...options,
    enabled: can('project.attachment.read'),
    placeholderData: keepPreviousData,
  });
  const denied = query.error instanceof ApiError && [401, 403, 404].includes(query.error.status);
  const data = denied ? undefined : query.data;
  const invalidate = () => client.invalidateQueries({ queryKey: projectKey(workspaceID) });
  const columns: ColumnDef<Attachment>[] = [
    {
      accessorKey: 'filename',
      header: '文件名',
      cell: ({ row }) => (
        <AttachmentFilename
          filename={row.original.filename}
          className="max-w-64"
          downloadURL={
            row.original.state === 'ready' && can('project.attachment.read')
              ? attachmentDownload(workspaceID, row.original.id)
              : undefined
          }
        />
      ),
    },
    { accessorKey: 'project_id', header: '项目编号' },
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
    ...recordTimeColumns<Attachment>((record) => record),
    {
      id: 'actions',
      header: '操作',
      cell: ({ row }) => {
        const item = row.original;
        return (
          <div className="flex items-center gap-1">
            {can('project.attachment.delete') ? (
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
        title="项目附件"
        busy={upload.pending || remove.pending}
        onClose={onClose}
        footerRefresh={
          <Button
            type="button"
            variant="outline"
            loading={refresh.pending || (query.isPending && can('project.attachment.read'))}
            disabled={!can('project.attachment.read')}
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
        }
      >
        <div className="space-y-4 p-6">
          {!projectID && can('project.attachment.upload') ? (
            <div className="space-y-2">
              <Label htmlFor="upload-project">所属项目</Label>
              {can('project.list') ? (
                <SearchSelect
                  id="upload-project"
                  label="上传项目"
                  value={uploadProjectID ? String(uploadProjectID) : ''}
                  options={(projects.data ?? []).map((item) => ({
                    value: String(item.value),
                    label: item.label,
                  }))}
                  onValueChange={(value) => {
                    setUploadProjectID(Number(value));
                  }}
                  loading={projects.isFetching}
                  className="w-full"
                />
              ) : (
                <Input
                  id="upload-project"
                  type="number"
                  min={1}
                  value={uploadProjectID || ''}
                  onChange={(event) => {
                    setUploadProjectID(Number(event.target.value));
                  }}
                />
              )}
            </div>
          ) : null}
          {projects.error ? <ErrorMessage>{projects.error.message}</ErrorMessage> : null}
          {can('project.attachment.upload') ? (
            <div className="space-y-2">
              <FileUploadButton
                loading={upload.pending}
                disabled={!targetProject || remove.pending}
                onFileSelect={(file, updateUpload) => {
                  void upload.run('upload', async () => {
                    validateUploadFile(file);
                    try {
                      await uploadAttachment(workspaceID, targetProject, file, updateUpload);
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
          {can('project.attachment.read') && (query.isPending || (data?.total ?? 0) > 0) ? (
            <div className="border-y border-border">
              <ManagementTable
                tableID="project-attachments"
                columns={columns.filter(
                  (column) => column.id !== 'actions' || can('project.attachment.delete'),
                )}
                data={data?.items ?? []}
                total={data?.total ?? 0}
                page={pagination.page}
                pageSize={pagination.pageSize}
                loading={query.isFetching}
                onPage={async (page) => {
                  pagination.setPage(page);
                  return client.query({
                    ...attachmentListOptions(workspaceID, page, pagination.pageSize, projectID),
                    staleTime: 0,
                  });
                }}
                onPageSize={async (size) => {
                  pagination.setPageSize(size);
                  return client.query({
                    ...attachmentListOptions(workspaceID, 1, size, projectID),
                    staleTime: 0,
                  });
                }}
              />
            </div>
          ) : null}
        </div>
      </Drawer>
      {deleting ? (
        <Drawer
          title="删除附件"
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
                      await deleteAttachment(workspaceID, deleting);
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
