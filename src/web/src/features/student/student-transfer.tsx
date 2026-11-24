import { useQueryClient } from '@tanstack/react-query';
import { Download, FileDown } from 'lucide-react';
import { useState } from 'react';

import { ErrorMessage } from '@/components/error-message';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Drawer } from '@/components/ui/drawer';
import { FileInput } from '@/components/ui/file-input';
import { Label } from '@/components/ui/label';
import { UploadButton } from '@/components/ui/upload-button';
import { personKey } from '@/features/cpc-person/api';
import { graduationKey } from '@/features/graduation/api';
import {
  exportStudents,
  importStudents,
  saveStudentFile,
  type StudentFilters,
  studentKey,
  studentTemplate,
} from '@/features/student/api';
import { sensitiveAction } from '@/features/student/fields';
import { useApiAction, useApiMutation } from '@/lib/api-action';
import { getConfig } from '@/lib/config';
import type { UploadStatus } from '@/lib/file-upload';
import { readSpreadsheetImport, spreadsheetAccept } from '@/lib/spreadsheet';

export function StudentTransfer({
  workspaceID,
  filters,
  mode,
  actions,
  onClose,
}: {
  workspaceID: string;
  filters: StudentFilters;
  mode: 'import' | 'export';
  actions: string[];
  onClose: () => void;
}) {
  const [includeSensitive, setIncludeSensitive] = useState(false);
  const [file, setFile] = useState<File | null>(null);
  const [progress, setProgress] = useState<UploadStatus>(null);
  const [created, setCreated] = useState<number | null>(null);
  const template = useApiAction();
  const download = useApiAction();
  const client = useQueryClient();
  const mutation = useApiMutation({
    mutationFn: async () => {
      setProgress(null);
      if (!file) {
        throw new Error('请选择 XLSX 文件');
      }
      const result = await importStudents(
        workspaceID,
        await readSpreadsheetImport(file, getConfig().studentMaxFileBytes),
        setProgress,
      );
      await Promise.all([
        client.invalidateQueries({ queryKey: studentKey(workspaceID) }),
        client.invalidateQueries({ queryKey: personKey(workspaceID) }),
        client.invalidateQueries({ queryKey: graduationKey(workspaceID) }),
        client.invalidateQueries({ queryKey: ['management', 'audit'] }),
      ]);
      return result;
    },
    onSuccess: (result) => {
      setCreated(result.created);
      setProgress('complete');
      setFile(null);
    },
  });
  const busy = template.pending || download.pending || mutation.isPending;
  const error = template.error ?? download.error ?? mutation.error;
  return (
    <Drawer
      title={mode === 'import' ? '导入学生' : '导出学生'}
      description={mode === 'import' ? '使用 XLSX 模板新增学生' : '导出当前筛选条件下的学生'}
      busy={busy}
      onClose={onClose}
      footer={
        mode === 'import' ? (
          <UploadButton
            loading={mutation.isPending}
            progress={progress}
            onDismiss={() => {
              setProgress(null);
            }}
            disabled={!file || template.pending}
            onClick={() => {
              mutation.mutate();
            }}
          >
            导入
          </UploadButton>
        ) : (
          <span className="inline-flex items-center gap-2">
            <Button
              loading={download.pending}
              onClick={() => {
                void download.run('export', async () => {
                  saveStudentFile(await exportStudents(workspaceID, filters, includeSensitive));
                });
              }}
            >
              <Download aria-hidden="true" />
              导出
            </Button>
          </span>
        )
      }
    >
      <div className="space-y-5 p-6">
        <fieldset disabled={busy} className="space-y-3">
          <legend className="mb-3 text-sm font-medium">
            {mode === 'import' ? '模板字段' : '导出字段'}
          </legend>
          <p className="text-xs text-muted-foreground">
            {mode === 'import'
              ? '基本信息和必填身份证号码始终包含，可统一包含其他敏感信息。'
              : '基本信息始终包含，可统一包含敏感信息。'}
          </p>
          {actions.includes(sensitiveAction) ? (
            <label className="flex items-center gap-2 text-sm">
              <Checkbox
                checked={includeSensitive}
                onChange={(event) => {
                  setIncludeSensitive(event.target.checked);
                }}
              />
              包含敏感信息
            </label>
          ) : null}
        </fieldset>
        {mode === 'import' ? (
          <>
            <span className="inline-flex items-center gap-2">
              <Button
                variant="outline"
                loading={template.pending}
                disabled={mutation.isPending}
                onClick={() => {
                  void template.run('template', async () => {
                    saveStudentFile(await studentTemplate(workspaceID, includeSensitive));
                  });
                }}
              >
                <FileDown aria-hidden="true" />
                下载模板
              </Button>
            </span>
            <p className="text-xs leading-5 text-muted-foreground">
              学号、姓名、年级、状态、学历、专业、班级、录取类型、性别、民族、住宿类型、身份证号码必填；导入只新增，任一行有误则整批不导入。文本维度支持已有名称或新文本；是否类及住宿类型使用中文名称或英文代码。XLSX
              模板已将全部列设为文本，填写说明和枚举选项位于单独的工作表。月份使用 yyyy-mm，日期使用
              yyyy-mm-dd。
            </p>
            <div className="space-y-2">
              <Label htmlFor="student-file">XLSX 文件</Label>
              <FileInput
                id="student-file"
                file={file}
                accept={spreadsheetAccept}
                disabled={busy}
                onFileChange={(nextFile) => {
                  setCreated(null);
                  setFile(nextFile);
                }}
              />
            </div>
            {created !== null ? (
              <p role="status" className="text-sm">
                已导入 {String(created)} 条学生记录
              </p>
            ) : null}
          </>
        ) : null}
        {error ? <ErrorMessage>{error.message}</ErrorMessage> : null}
      </div>
    </Drawer>
  );
}
