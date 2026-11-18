import { useQueryClient } from '@tanstack/react-query';
import { FileDown, Upload } from 'lucide-react';
import { useState } from 'react';

import { Button } from '@/components/ui/button';
import { Drawer } from '@/components/ui/drawer';
import { FileInput } from '@/components/ui/file-input';
import { Label } from '@/components/ui/label';
import { importTimeline, timelineImportTemplate, timelineKey } from '@/features/cpc-timeline/api';
import { useApiAction } from '@/lib/api-action';
import { downloadFile } from '@/lib/download';
import { validateFileSize } from '@/lib/file-upload';

export function TemplateImport({
  workspaceID,
  onClose,
}: {
  workspaceID: string;
  onClose: () => void;
}) {
  const [file, setFile] = useState<File | null>(null);
  const template = useApiAction();
  const upload = useApiAction();
  const client = useQueryClient();
  return (
    <Drawer
      title="导入时间线模板"
      description="通过 JSON 文件新增一个模板，同名记录不会被覆盖"
      busy={template.pending || upload.pending}
      onClose={onClose}
      footer={
        <Button
          disabled={!file || template.pending}
          loading={upload.pending}
          onClick={() => {
            void upload.run('import', async () => {
              if (!file) return;
              validateFileSize(file);
              if (!/\.json$/iu.test(file.name)) throw new Error('请选择 JSON 文件');
              let value: unknown;
              try {
                value = JSON.parse((await file.text()).replace(/^\uFEFF/u, ''));
              } catch {
                throw new Error('请上传有效的 JSON 模板文件');
              }
              await importTimeline(workspaceID, value);
              await client.invalidateQueries({ queryKey: timelineKey(workspaceID) });
              onClose();
            });
          }}
        >
          <Upload aria-hidden="true" />
          导入
        </Button>
      }
    >
      <div className="space-y-5 p-6">
        <span className="inline-flex items-center gap-2">
          <Button
            variant="outline"
            loading={template.pending}
            disabled={upload.pending}
            onClick={() => {
              void template.run('download', async () => {
                const result = await timelineImportTemplate(workspaceID);
                downloadFile(
                  result.filename,
                  new Blob([result.content], { type: result.content_type }),
                );
              });
            }}
          >
            <FileDown aria-hidden="true" />
            下载导入格式
          </Button>
        </span>
        <p className="text-xs leading-5 text-muted-foreground">
          每个节点须包含一个必填日期字段作为发生时间。节点、字段和规则标识使用英文小写字母开头，可包含数字及下划线；规则引用同模板内的其他节点。
        </p>
        <div className="space-y-2">
          <Label htmlFor="timeline-template-file">JSON 文件</Label>
          <FileInput
            id="timeline-template-file"
            aria-label="JSON 文件"
            file={file}
            accept=".json,application/json"
            disabled={upload.pending}
            onFileChange={(nextFile) => {
              setFile(nextFile);
            }}
          />
        </div>
      </div>
    </Drawer>
  );
}
