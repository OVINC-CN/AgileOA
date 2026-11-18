import { useQueryClient } from '@tanstack/react-query';
import { FileDown, Upload } from 'lucide-react';
import { useState } from 'react';

import { Button } from '@/components/ui/button';
import { Drawer } from '@/components/ui/drawer';
import { FileInput } from '@/components/ui/file-input';
import { Label } from '@/components/ui/label';
import {
  importPartyBranches,
  partyBranchKey,
  partyBranchTemplate,
} from '@/features/party-branch/api';
import { useApiAction } from '@/lib/api-action';
import { getConfig } from '@/lib/config';
import { readSpreadsheetImport, saveSpreadsheetFile, spreadsheetAccept } from '@/lib/spreadsheet';

export function BranchTransfer({
  workspaceID,
  onClose,
}: {
  workspaceID: string;
  onClose: () => void;
}) {
  const [file, setFile] = useState<File | null>(null);
  const [created, setCreated] = useState<number | null>(null);
  const [fileKey, setFileKey] = useState(0);
  const template = useApiAction();
  const upload = useApiAction();
  const client = useQueryClient();
  return (
    <Drawer
      title="导入党支部"
      description="按固定 XLSX 模板新增党支部，任一行有误则整批不导入"
      busy={upload.pending || template.pending}
      onClose={onClose}
      footer={
        <Button
          loading={upload.pending}
          disabled={!file || template.pending}
          onClick={() => {
            void upload.run('import', async () => {
              if (!file) return;
              const result = await importPartyBranches(
                workspaceID,
                await readSpreadsheetImport(file, getConfig().partyBranchMaxFileBytes),
              );
              await client.invalidateQueries({ queryKey: partyBranchKey(workspaceID) });
              setCreated(result.created);
              setFile(null);
              setFileKey((current) => current + 1);
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
            disabled={upload.pending}
            loading={template.pending}
            onClick={() => {
              void template.run('template', async () => {
                const result = await partyBranchTemplate(workspaceID);
                saveSpreadsheetFile(result);
              });
            }}
          >
            <FileDown aria-hidden="true" />
            下载模板
          </Button>
        </span>
        <p className="text-xs leading-5 text-muted-foreground">
          简称、全称必填，分别在当前空间内唯一。管理员填写用户登录账号，可留空。
          优先级填写整数，数字越大越优先，留空时为 0。编号由系统自动生成。已有记录不会被覆盖。
        </p>
        <div className="space-y-2">
          <Label htmlFor="branch-file">XLSX 文件</Label>
          <FileInput
            key={fileKey}
            id="branch-file"
            file={file}
            accept={spreadsheetAccept}
            disabled={upload.pending}
            onFileChange={(nextFile) => {
              setFile(nextFile);
              setCreated(null);
            }}
          />
        </div>
        {created !== null ? <p role="status">已导入 {String(created)} 条党支部记录</p> : null}
      </div>
    </Drawer>
  );
}
