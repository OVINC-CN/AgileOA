import { Link2, Plus, X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import type { WorkLink } from '@/features/monthly-work/api';
import { getConfig } from '@/lib/config';

export function LinksEditor({
  value,
  onChange,
}: {
  value: WorkLink[];
  onChange: (links: WorkLink[]) => void;
}) {
  const update = (index: number, changes: Partial<WorkLink>) => {
    onChange(value.map((link, position) => (index === position ? { ...link, ...changes } : link)));
  };
  return (
    <section className="space-y-3 border-t border-border pt-5" aria-label="外部链接">
      <div className="flex items-center justify-between gap-3">
        <h2 className="text-base leading-5 font-medium">外部链接</h2>
        <Button
          type="button"
          variant="outline"
          disabled={value.length >= getConfig().monthlyWorkMaxLinks}
          onClick={() => {
            onChange([...value, { name: '', url: '' }]);
          }}
        >
          <Plus aria-hidden="true" />
          添加链接
        </Button>
      </div>
      {value.length ? (
        <ol className="space-y-3">
          {value.map((link, index) => (
            <li key={index} className="glass-panel space-y-3 rounded-lg border p-3">
              <div className="flex items-center justify-between">
                <h3 className="inline-flex items-center gap-2 text-sm leading-5 font-medium">
                  <Link2 className="size-4 text-muted-foreground" aria-hidden="true" />
                  链接 {String(index + 1)}
                </h3>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  aria-label={`移除链接 ${String(index + 1)}`}
                  onClick={() => {
                    onChange(value.filter((_, position) => position !== index));
                  }}
                >
                  <X aria-hidden="true" />
                </Button>
              </div>
              <div className="space-y-2">
                <Label htmlFor={`work-link-name-${String(index)}`}>链接名称（可选）</Label>
                <Input
                  id={`work-link-name-${String(index)}`}
                  maxLength={100}
                  value={link.name}
                  onChange={(event) => {
                    update(index, { name: event.target.value });
                  }}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor={`work-link-url-${String(index)}`}>链接地址</Label>
                <Input
                  id={`work-link-url-${String(index)}`}
                  type="url"
                  required
                  placeholder="https://"
                  maxLength={2048}
                  value={link.url}
                  onChange={(event) => {
                    update(index, { url: event.target.value });
                  }}
                />
              </div>
            </li>
          ))}
        </ol>
      ) : (
        <p className="text-xs text-muted-foreground">暂无外部链接</p>
      )}
    </section>
  );
}
