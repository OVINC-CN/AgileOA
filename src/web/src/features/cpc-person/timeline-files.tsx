import type { ReactNode } from 'react';

import { AttachmentFilename } from '@/components/attachment-filename';
import {
  type PersonTimelineAttachment,
  personTimelineDownload,
  type PersonTimelineNode,
} from '@/features/cpc-person/timeline-api';
import { uploadStateLabels } from '@/lib/file-upload';

export function PersonTimelineFiles({
  workspaceID,
  personID,
  node,
  editing,
  renderAction,
  downloadable = true,
}: {
  workspaceID: string;
  personID: number;
  node: PersonTimelineNode;
  editing: boolean;
  renderAction?: (file: PersonTimelineAttachment) => ReactNode;
  downloadable?: boolean;
}) {
  const groups = [
    { label: '模板附件', files: node.template_attachments, template: true },
    { label: '人员附件', files: node.attachments, template: false },
  ].filter((group) => group.files.length > 0);
  if (!groups.length) return null;
  return (
    <div className="space-y-3 text-sm">
      {groups.map((group) => (
        <section key={group.label} className="space-y-1">
          <h4 className="text-xs font-medium text-muted-foreground">{group.label}</h4>
          <ul className="space-y-1">
            {group.files.map((file) => (
              <li key={file.id} className="flex min-w-0 items-center gap-2">
                <AttachmentFilename
                  filename={file.filename}
                  className="flex-1"
                  downloadURL={
                    downloadable && file.state === 'ready'
                      ? personTimelineDownload(
                          workspaceID,
                          personID,
                          node.id,
                          file.id,
                          group.template,
                          editing,
                        )
                      : undefined
                  }
                />
                {file.state !== 'ready' ? (
                  <span className="shrink-0 text-xs text-muted-foreground">
                    {uploadStateLabels[file.state]}
                  </span>
                ) : null}
                <div className="flex shrink-0 items-center gap-1">
                  {!group.template && 'person_id' in file ? renderAction?.(file) : null}
                </div>
              </li>
            ))}
          </ul>
        </section>
      ))}
    </div>
  );
}
