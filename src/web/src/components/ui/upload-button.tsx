import { Upload, X } from 'lucide-react';
import { Popover } from 'radix-ui';
import { type ComponentProps, useEffect, useRef, useState } from 'react';

import { Button } from '@/components/ui/button';
import type { UploadStatus } from '@/lib/file-upload';

function isElementHidden(element: HTMLElement) {
  if (!element.isConnected) return true;
  if (element.closest('[hidden], [inert], [data-closing="true"]')) return true;
  let current: HTMLElement | null = element;
  while (current) {
    const style = getComputedStyle(current);
    if (style.display === 'none' || style.visibility === 'hidden' || style.opacity === '0') {
      return true;
    }
    current = current.parentElement;
  }
  return false;
}

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
  const onDismissRef = useRef(onDismiss);
  useEffect(() => {
    onDismissRef.current = onDismiss;
  }, [onDismiss]);
  const complete = progress === 'complete';
  const active = loading || complete;
  const [hidden, setHidden] = useState(false);
  if (!active && hidden) setHidden(false);

  useEffect(() => {
    const element = anchor.current;
    if (!active || !element) return;

    let intersected = true;
    const syncVisibility = () => {
      const nextHidden = !intersected || isElementHidden(element);
      setHidden(nextHidden);
      if (nextHidden && complete && !loading) {
        onDismissRef.current();
      }
    };

    // 抽屉关闭、切换分区或按钮滚出可视区时自动隐藏，上传完成后同步关闭浮层。
    const mutation = new MutationObserver(syncVisibility);
    mutation.observe(document.body, {
      subtree: true,
      attributes: true,
      attributeFilter: ['hidden', 'inert', 'data-closing', 'style', 'class'],
    });

    let intersection: IntersectionObserver | undefined;
    if (typeof IntersectionObserver === 'function') {
      intersection = new IntersectionObserver(
        (entries) => {
          const entry = entries.at(-1);
          if (!entry) return;
          intersected = entry.isIntersecting;
          syncVisibility();
        },
        { threshold: 0 },
      );
      intersection.observe(element);
    }

    const frame = requestAnimationFrame(syncVisibility);
    return () => {
      cancelAnimationFrame(frame);
      mutation.disconnect();
      intersection?.disconnect();
    };
  }, [active, complete, loading]);

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
      open={active && !hidden}
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
          hideWhenDetached
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
                <Button
                  type="button"
                  variant="cancel"
                  size="icon-xs"
                  className="border-0 text-muted-foreground enabled:hover:text-foreground"
                  aria-label="关闭上传进度"
                >
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
