import { useQueryClient } from '@tanstack/react-query';
import { FileDown } from 'lucide-react';
import { useState } from 'react';

import { Button } from '@/components/ui/button';
import { Drawer } from '@/components/ui/drawer';
import { FileInput } from '@/components/ui/file-input';
import { Label } from '@/components/ui/label';
import { UploadButton } from '@/components/ui/upload-button';
import { importPeople, personKey, personTemplate, savePersonFile } from '@/features/cpc-person/api';
import { useApiAction } from '@/lib/api-action';
import { getConfig } from '@/lib/config';
import type { UploadStatus } from '@/lib/file-upload';
import { readSpreadsheetImport, spreadsheetAccept } from '@/lib/spreadsheet';

export function PersonTransfer({
  workspaceID,
  onClose,
}: {
  workspaceID: string;
  onClose: () => void;
}) {
  const [file, setFile] = useState<File | null>(null);
  const [progress, setProgress] = useState<UploadStatus>(null);
  const [created, setCreated] = useState<number | null>(null);
  const [fileKey, setFileKey] = useState(0);
  const template = useApiAction();
  const upload = useApiAction();
  const client = useQueryClient();
  return (
    <Drawer
      title="导入党建人员"
      description="按学号关联已有学生，仅新增党建记录，任一行有误则整批不导入"
      busy={template.pending || upload.pending}
      onClose={onClose}
      footer={
        <UploadButton
          loading={upload.pending}
          progress={progress}
          onDismiss={() => {
            setProgress(null);
          }}
          disabled={!file || template.pending}
          onClick={() => {
            void upload.run('import', async () => {
              setProgress(null);
              if (!file) return;
              const result = await importPeople(
                workspaceID,
                await readSpreadsheetImport(file, getConfig().cpcPersonMaxFileBytes),
                setProgress,
              );
              await client.invalidateQueries({ queryKey: personKey(workspaceID) });
              setCreated(result.created);
              setProgress('complete');
              setFile(null);
              setFileKey((key) => key + 1);
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
            disabled={upload.pending}
            onClick={() => {
              void template.run('template', async () => {
                savePersonFile(await personTemplate(workspaceID));
              });
            }}
          >
            <FileDown aria-hidden="true" />
            下载模板
          </Button>
        </span>
        <p className="text-xs text-muted-foreground">
          时间线模板填写当前空间的完整模板名称，每名新增人员必须绑定。状态可自定义，留空默认为“在编”。所属党支部由准入规则自动确定，基础信息请在学生信息中维护。
        </p>
        <div className="space-y-2">
          <Label htmlFor="cpc-file">XLSX 文件</Label>
          <FileInput
            key={fileKey}
            id="cpc-file"
            file={file}
            accept={spreadsheetAccept}
            disabled={upload.pending}
            onFileChange={(nextFile) => {
              setFile(nextFile);
              setCreated(null);
            }}
          />
        </div>
        {created !== null ? <p role="status">已导入 {String(created)} 名人员</p> : null}
      </div>
    </Drawer>
  );
}
