import { useQueryClient } from '@tanstack/react-query';
import { FileDown } from 'lucide-react';
import { useState } from 'react';

import { Button } from '@/components/ui/button';
import { Drawer } from '@/components/ui/drawer';
import { FileInput } from '@/components/ui/file-input';
import { Label } from '@/components/ui/label';
import { UploadButton } from '@/components/ui/upload-button';
import { importWork, workKey, workTemplate } from '@/features/monthly-work/api';
import type { WorkCategory } from '@/features/monthly-work/category';
import { saveStudentFile } from '@/features/student/api';
import { useApiAction } from '@/lib/api-action';
import { getConfig } from '@/lib/config';
import type { UploadStatus } from '@/lib/file-upload';
import { readSpreadsheetImport, spreadsheetAccept } from '@/lib/spreadsheet';

export function WorkTransfer({
  workspaceID,
  category = 'student',
  onClose,
}: {
  workspaceID: string;
  category?: WorkCategory;
  onClose: () => void;
}) {
  const [file, setFile] = useState<File | null>(null);
  const [progress, setProgress] = useState<UploadStatus>(null);
  const [created, setCreated] = useState<number | null>(null);
  const template = useApiAction();
  const upload = useApiAction();
  const client = useQueryClient();
  return (
    <Drawer
      title="导入月度工作"
      description="使用 XLSX 模板新增记录，任一行有误则整批不导入"
      busy={upload.pending || template.pending}
      onClose={onClose}
      footer={
        <UploadButton
          loading={upload.pending}
          progress={progress}
          onDismiss={() => {
            setProgress(null);
          }}
          disabled={!file}
          onClick={() => {
            void upload.run('import', async () => {
              setProgress(null);
              if (!file) return;
              const result = await importWork(
                workspaceID,
                await readSpreadsheetImport(file, getConfig().monthlyWorkMaxFileBytes),
                category,
                setProgress,
              );
              await client.invalidateQueries({ queryKey: workKey(workspaceID, category) });
              setCreated(result.created);
              setProgress('complete');
              setFile(null);
            });
          }}
        >
          导入
        </UploadButton>
      }
    >
      <div className="space-y-5 p-6">
        <span className="inline-flex items-center gap-2">
          <Button
            variant="outline"
            loading={template.pending}
            onClick={() => {
              void template.run('template', async () => {
                saveStudentFile(await workTemplate(workspaceID, category));
              });
            }}
          >
            <FileDown aria-hidden="true" />
            下载模板
          </Button>
        </span>
        <p className="text-xs leading-5 text-muted-foreground">
          日期使用 YYYY-MM-DD，负责人填写当前空间可用用户的登录账号。外部链接列可留空或填写 JSON
          数组，每项包含 name 和 url。附件在创建记录后上传并关联。
        </p>
        <div className="space-y-2">
          <Label htmlFor="work-file">XLSX 文件</Label>
          <FileInput
            id="work-file"
            file={file}
            accept={spreadsheetAccept}
            disabled={upload.pending}
            onFileChange={(nextFile) => {
              setFile(nextFile);
              setCreated(null);
            }}
          />
        </div>
        {created !== null ? (
          <p role="status" className="text-sm">
            已导入 {String(created)} 条记录
          </p>
        ) : null}
      </div>
    </Drawer>
  );
}
