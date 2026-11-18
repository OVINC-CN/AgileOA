import { useQueryClient } from '@tanstack/react-query';
import { FileDown, Upload } from 'lucide-react';
import { useState } from 'react';

import { Button } from '@/components/ui/button';
import { Drawer } from '@/components/ui/drawer';
import { FileInput } from '@/components/ui/file-input';
import { Label } from '@/components/ui/label';
import { graduationKey, graduationTemplate, importGraduations } from '@/features/graduation/api';
import { saveStudentFile, studentKey } from '@/features/student/api';
import { useApiAction } from '@/lib/api-action';
import { getConfig } from '@/lib/config';
import { readSpreadsheetImport, spreadsheetAccept } from '@/lib/spreadsheet';

export function GraduationTransfer({
  workspaceID,
  onClose,
}: {
  workspaceID: string;
  onClose: () => void;
}) {
  const [file, setFile] = useState<File | null>(null);
  const [created, setCreated] = useState<number | null>(null);
  const template = useApiAction();
  const upload = useApiAction();
  const client = useQueryClient();
  return (
    <Drawer
      title="导入毕业去向"
      description="按当前空间学号绑定尚未填写的学生，任一行有误则整批不导入"
      busy={upload.pending || template.pending}
      onClose={onClose}
      footer={
        <Button
          loading={upload.pending}
          disabled={!file || template.pending}
          onClick={() => {
            void upload.run('import', async () => {
              if (!file) return;
              const result = await importGraduations(
                workspaceID,
                await readSpreadsheetImport(file, getConfig().studentMaxFileBytes),
              );
              await Promise.all([
                client.invalidateQueries({ queryKey: graduationKey(workspaceID) }),
                client.invalidateQueries({ queryKey: studentKey(workspaceID) }),
              ]);
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
            disabled={upload.pending}
            onClick={() => {
              void template.run('template', async () => {
                saveStudentFile(await graduationTemplate(workspaceID));
              });
            }}
          >
            <FileDown aria-hidden="true" />
            下载模板
          </Button>
        </span>
        <p className="text-xs leading-5 text-muted-foreground">
          表头为学号、毕业年份、类型、毕业去向。毕业年份独立填写，类型留空为待就业，也可选择或新建名称。已填写的去向请在列表中编辑。
        </p>
        <div className="space-y-2">
          <Label htmlFor="graduation-file">XLSX 文件</Label>
          <FileInput
            id="graduation-file"
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
