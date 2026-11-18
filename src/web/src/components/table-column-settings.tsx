import { Columns3, RotateCcw } from 'lucide-react';
import { useId, useState } from 'react';

import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Drawer } from '@/components/ui/drawer';
import { Label } from '@/components/ui/label';
import type { useTableColumns } from '@/lib/table-columns';

export function TableColumnSettings({
  settings,
  label = '显示字段',
}: {
  settings: ReturnType<typeof useTableColumns>;
  label?: string;
}) {
  const [open, setOpen] = useState(false);
  const id = useId();
  return (
    <>
      <Button
        type="button"
        variant="ghost"
        size="sm"
        disabled={settings.columns.length === 0}
        onClick={() => {
          setOpen(true);
        }}
      >
        <Columns3 aria-hidden="true" />
        {label}
      </Button>
      {open ? (
        <Drawer
          title={label}
          description="勾选需要展示的字段，至少保留一个。设置自动保存在当前浏览器。"
          onClose={() => {
            setOpen(false);
          }}
          footer={
            <>
              <Button type="button" variant="outline" onClick={settings.showAll}>
                <RotateCcw aria-hidden="true" />
                恢复全部
              </Button>
              <Button
                type="button"
                onClick={() => {
                  setOpen(false);
                }}
              >
                完成
              </Button>
            </>
          }
        >
          <div className="grid grid-cols-1 gap-1 p-6">
            {settings.columns.map((column) => (
              <Label
                key={column.id}
                htmlFor={`${id}-${column.id}`}
                className="cursor-pointer rounded-lg px-3 py-3 leading-5 hover:bg-muted has-disabled:cursor-default has-disabled:opacity-50"
              >
                <Checkbox
                  id={`${id}-${column.id}`}
                  checked={settings.isVisible(column.id)}
                  disabled={settings.isVisible(column.id) && settings.visibleColumns.length === 1}
                  onChange={(event) => {
                    settings.setVisible(column.id, event.target.checked);
                  }}
                />
                <span className="min-w-0 flex-1 wrap-anywhere">{column.label}</span>
              </Label>
            ))}
          </div>
        </Drawer>
      ) : null}
    </>
  );
}
