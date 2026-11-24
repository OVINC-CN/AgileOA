import { Pencil, Save } from 'lucide-react';
import { useState } from 'react';

import { TableActionButton } from '@/components/table-action-button';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { DeleteConfirmation } from '@/components/ui/delete-confirmation';
import { Drawer } from '@/components/ui/drawer';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Tooltip } from '@/components/ui/tooltip';
import {
  type TimelineFilterPreset,
  timelinePresetNameLength,
} from '@/features/cpc-person/timeline-filter-preset-store';
import { showMessage } from '@/lib/messages';

export type TimelinePresetEditor =
  { mode: 'create'; preset: null } | { mode: 'rename' | 'overwrite'; preset: TimelineFilterPreset };

export function TimelinePresetEditorDrawer({
  editor,
  onSave,
  onClose,
}: {
  editor: TimelinePresetEditor;
  onSave: (name: string, includeValues: boolean) => void;
  onClose: () => void;
}) {
  const [name, setName] = useState(editor.preset?.name ?? '');
  const [includeValues, setIncludeValues] = useState(editor.preset?.includeValues ?? true);
  const titles = {
    create: '保存常用筛选方案',
    rename: '重命名筛选方案',
    overwrite: '覆盖常用筛选方案',
  };
  return (
    <Drawer
      title={titles[editor.mode]}
      description={
        editor.mode === 'rename'
          ? '修改名称，已保存的条件保持不变'
          : editor.mode === 'overwrite'
            ? '使用当前时间线筛选条件替换原方案，保存后无法恢复原条件'
            : '保存当前时间线筛选条件，供当前账号在本工作空间使用'
      }
      onClose={onClose}
      form={{
        noValidate: true,
        onSubmit: (event) => {
          event.preventDefault();
          event.stopPropagation();
          try {
            onSave(name, includeValues);
            onClose();
          } catch (cause) {
            showMessage(cause instanceof Error ? cause.message : '保存方案失败，请重试');
          }
        },
      }}
      footerDismiss={
        <Button type="button" variant="cancel" onClick={onClose}>
          取消
        </Button>
      }
      footer={
        <Button type="submit">
          <Save aria-hidden="true" />
          {editor.mode === 'overwrite' ? '覆盖保存' : '保存'}
        </Button>
      }
    >
      <div className="space-y-6 p-6">
        <div className="space-y-2">
          <Label htmlFor="timeline-preset-name">方案名称 *</Label>
          <Input
            id="timeline-preset-name"
            autoComplete="off"
            maxLength={timelinePresetNameLength}
            value={name}
            onChange={(event) => {
              setName(event.target.value);
            }}
          />
        </div>
        {editor.mode !== 'rename' ? (
          <div className="space-y-2">
            <label className="flex items-center gap-2 text-sm">
              <Checkbox
                checked={includeValues}
                onChange={(event) => {
                  setIncludeValues(event.target.checked);
                }}
              />
              保存筛选值
            </label>
            <p className="text-xs text-muted-foreground">
              取消后仅保存字段和筛选方式，选用方案后再填写筛选值。
            </p>
          </div>
        ) : null}
      </div>
    </Drawer>
  );
}

export function TimelinePresetManagerDrawer({
  presets,
  canOverwrite,
  onEdit,
  onDelete,
  onClose,
}: {
  presets: TimelineFilterPreset[];
  canOverwrite: boolean;
  onEdit: (editor: TimelinePresetEditor) => void;
  onDelete: (preset: TimelineFilterPreset) => void;
  onClose: () => void;
}) {
  return (
    <Drawer
      title="管理常用筛选方案"
      description="方案仅保存在当前浏览器，清除浏览器数据后需要重新保存"
      onClose={onClose}
      footerDismiss={
        <Button type="button" variant="cancel" onClick={onClose}>
          关闭
        </Button>
      }
    >
      <ul className="space-y-3 p-6">
        {presets.length === 0 ? (
          <li className="text-sm text-muted-foreground">暂无常用筛选方案</li>
        ) : null}
        {presets.map((preset) => (
          <li key={preset.id} className="space-y-3 rounded-xl border border-border p-4">
            <div className="min-w-0 space-y-1">
              <Tooltip content={preset.name} onlyOverflow>
                <p className="truncate text-sm font-medium">{preset.name}</p>
              </Tooltip>
              <p className="text-xs text-muted-foreground">
                {preset.conditions.length} 个条件 ·{' '}
                {preset.includeValues ? '包含筛选值' : '不含筛选值'}
              </p>
            </div>
            <div className="flex flex-wrap justify-end gap-1">
              <TableActionButton
                aria-label={`重命名方案 ${preset.name}`}
                onClick={() => {
                  onEdit({ mode: 'rename', preset });
                }}
              >
                <Pencil aria-hidden="true" />
                重命名
              </TableActionButton>
              <TableActionButton
                aria-label={`覆盖方案 ${preset.name}`}
                disabled={!canOverwrite}
                onClick={() => {
                  onEdit({ mode: 'overwrite', preset });
                }}
              >
                <Save aria-hidden="true" />
                覆盖保存
              </TableActionButton>
              <DeleteConfirmation
                target={preset.name}
                onConfirm={() => {
                  onDelete(preset);
                }}
                aria-label={`删除方案 ${preset.name}`}
              />
            </div>
          </li>
        ))}
      </ul>
    </Drawer>
  );
}
