import { useQuery } from '@tanstack/react-query';

import { AttachmentFilename } from '@/components/attachment-filename';
import { ErrorMessage } from '@/components/error-message';
import { RefreshButton } from '@/components/refresh-button';
import { DeleteConfirmation } from '@/components/ui/delete-confirmation';
import { LoadingIcon } from '@/components/ui/loading-icon';
import {
  type WorkAttachment,
  workAttachmentDownload,
  workAttachmentOptions,
} from '@/features/monthly-work/api';
import type { WorkCategory } from '@/features/monthly-work/category';
import { ApiError } from '@/lib/api';
import { uploadStateLabels } from '@/lib/file-upload';

export function WorkAttachmentList({
  workspaceID,
  category,
  workID,
  busy,
  onDelete,
}: {
  workspaceID: string;
  category: WorkCategory;
  workID: number;
  busy: boolean;
  onDelete?: ((file: WorkAttachment) => Promise<boolean>) | undefined;
}) {
  const query = useQuery(workAttachmentOptions(workspaceID, workID, category));
  const denied = query.error instanceof ApiError && [401, 403, 404].includes(query.error.status);
  const files = denied ? [] : (query.data ?? []);

  return (
    <>
      {query.error ? (
        <>
          <ErrorMessage>{query.error.message}</ErrorMessage>
          <RefreshButton label="刷新附件" onRefresh={() => query.refetch({ throwOnError: true })} />
        </>
      ) : null}
      {query.isPending ? (
        <p role="status" className="flex items-center gap-2 text-xs text-muted-foreground">
          <LoadingIcon />
          正在加载附件
        </p>
      ) : null}
      {files.length > 0 ? (
        <ul aria-label="附件列表" className="space-y-1 text-sm">
          {files.map((file) => (
            <li key={file.id} className="flex min-w-0 items-center gap-2">
              <AttachmentFilename
                filename={file.filename}
                className="flex-1"
                downloadURL={
                  file.state === 'ready'
                    ? workAttachmentDownload(workspaceID, file.id, category)
                    : undefined
                }
              />
              {file.state !== 'ready' ? (
                <span className="shrink-0 text-xs text-muted-foreground">
                  {uploadStateLabels[file.state]}
                </span>
              ) : null}
              {onDelete ? (
                <DeleteConfirmation
                  target={file.filename}
                  disabled={busy}
                  onConfirm={() => onDelete(file)}
                />
              ) : null}
            </li>
          ))}
        </ul>
      ) : null}
      {!query.isPending && !query.error && files.length === 0 ? (
        <p className="text-xs text-muted-foreground">暂无附件</p>
      ) : null}
    </>
  );
}
