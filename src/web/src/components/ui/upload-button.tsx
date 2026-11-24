import { Upload, X } from 'lucide-react';
import { Popover } from 'radix-ui';
import { type ComponentProps, useRef } from 'react';

import { Button } from '@/components/ui/button';
import type { UploadStatus } from '@/lib/file-upload';

export function UploadButton({
  children,
  loading = false,
  progress,
  onDismiss,
  ...props
}: Omit<ComponentProps<typeof Button>, 'asChild' | 'ref' | 'type'> & {
  progress: UploadStatus;
  onDismiss: () => void;
}) {
  const anchor = useRef<HTMLSpanElement>(null);
  const complete = progress === 'complete';
  const percent = complete ? 100 : progress;
  const status = complete
    ? '上传完成'
    : percent === null
      ? '准备上传'
      : percent === 100
        ? '已上传，正在处理'
        : '正在上传';
  return (
    <Popover.Root
      open={loading || complete}
      onOpenChange={(open) => {
        if (open || !complete || loading) return;
        const button = anchor.current?.querySelector<HTMLButtonElement>('button:enabled');
        (button ?? anchor.current)?.focus();
        onDismiss();
      }}
    >
      <Popover.Anchor asChild>
        <span ref={anchor} tabIndex={-1} className="inline-flex shrink-0">
          <Button {...props} type="button" loading={loading}>
            <Upload aria-hidden="true" />
            {children}
          </Button>
        </span>
      </Popover.Anchor>
      <Popover.Portal>
        <Popover.Content
          role="group"
          aria-label="文件上传状态"
          side="top"
          sideOffset={6}
          collisionPadding={16}
          className="glass-panel menu-motion z-[60] w-48 space-y-1.5 rounded-lg border bg-popover px-3 py-2 text-popover-foreground shadow-popover"
          onOpenAutoFocus={(event) => {
            event.preventDefault();
          }}
          onCloseAutoFocus={(event) => {
            event.preventDefault();
          }}
          onInteractOutside={(event) => {
            // 完成进度保留在原入口，操作页面其他控件时不关闭。
            event.preventDefault();
          }}
          onEscapeKeyDown={(event) => {
            if (loading) event.preventDefault();
          }}
        >
          <div className="flex min-h-6 items-center gap-2 text-xs">
            <span role="status" className="flex-1">
              {status}
            </span>
            {percent === null ? null : <span className="tabular-nums">{percent}%</span>}
            {complete && !loading ? (
              <Popover.Close asChild>
                <Button type="button" variant="ghost" size="icon-xs" aria-label="关闭上传进度">
                  <X aria-hidden="true" />
                </Button>
              </Popover.Close>
            ) : null}
          </div>
          <div
            role="progressbar"
            aria-label="文件上传进度"
            aria-valuenow={percent ?? undefined}
            aria-valuetext={percent === null ? status : `${String(percent)}%，${status}`}
            className="h-1.5 overflow-hidden rounded-full bg-muted"
          >
            <div
              className="h-full rounded-full bg-primary"
              style={{ width: `${String(percent ?? 0)}%` }}
            />
          </div>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}
