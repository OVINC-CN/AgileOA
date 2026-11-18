import { useQueryClient } from '@tanstack/react-query';
import { FileDown, Upload } from 'lucide-react';
import { useState } from 'react';

import { Button } from '@/components/ui/button';
import { Drawer } from '@/components/ui/drawer';
import { FileInput } from '@/components/ui/file-input';
import { Label } from '@/components/ui/label';
import { downloadTemplate, importRecords, projectKey } from '@/features/project/api';
import { saveStudentFile } from '@/features/student/api';
import { useApiAction } from '@/lib/api-action';
import { getConfig } from '@/lib/config';
import { readSpreadsheetImport, spreadsheetAccept } from '@/lib/spreadsheet';

export function ProjectTransfer({
  workspaceID,
  participation,
  onClose,
}: {
  workspaceID: string;
  participation: boolean;
  onClose: () => void;
}) {
  const [file, setFile] = useState<File | null>(null);
  const [created, setCreated] = useState<number | null>(null);
  const template = useApiAction();
  const upload = useApiAction();
  const client = useQueryClient();
  return (
    <Drawer
      title={participation ? '导入参与评分' : '导入项目'}
      description="使用 XLSX 模板新增记录，任一行有误则整批不导入"
      busy={upload.pending || template.pending}
      onClose={onClose}
      footer={
        <Button
          loading={upload.pending}
          disabled={!file}
          onClick={() => {
            void upload.run('import', async () => {
              if (!file) return;
              const result = await importRecords(
                workspaceID,
                participation,
                await readSpreadsheetImport(file, getConfig().projectMaxFileBytes),
              );
              await client.invalidateQueries({ queryKey: projectKey(workspaceID) });
              setCreated(result.created);
              setFile(null);
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
            onClick={() => {
              void template.run('template', async () => {
                saveStudentFile(await downloadTemplate(workspaceID, participation));
              });
            }}
          >
            <FileDown aria-hidden="true" />
            下载模板
          </Button>
        </span>
        <p className="text-xs leading-5 text-muted-foreground">
          {participation
            ? '按项目名称、项目归属年份和学号匹配当前空间记录。排名填写正整数，可留空；总分可留空。评分子项列使用 JSON 数组，每项包含 name、description 和可空的 score。name 只能填写该项目已配置的子项名称。'
            : '仅导入项目名称、归属年份、说明和可选子项名称。项目名称和归属年份必填，说明可留空；子项名称填写 JSON 字符串数组，可留空。审批设置、分数计算配置和附件在创建后通过项目编辑维护。'}
        </p>
        <div className="space-y-2">
          <Label htmlFor="project-file">XLSX 文件</Label>
          <FileInput
            id="project-file"
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
