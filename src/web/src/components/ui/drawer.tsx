import { X } from 'lucide-react';
import { Dialog } from 'radix-ui';
import {
  type ComponentProps,
  type ReactElement,
  type ReactNode,
  type Ref,
  type SubmitEventHandler,
  useCallback,
  useImperativeHandle,
  useLayoutEffect,
  useRef,
  useState,
} from 'react';

import { Button } from '@/components/ui/button';
import { Tooltip } from '@/components/ui/tooltip';
import { setMessageContainer } from '@/lib/messages';
import { cn } from '@/lib/utils';

export interface DrawerHandle {
  close: (afterClose?: () => void) => void;
}

interface Props {
  ref?: Ref<DrawerHandle>;
  title: string;
  description?: string | undefined;
  truncateDescription?: boolean;
  headerNavigation?: ReactNode;
  footerNavigation?: ReactNode;
  children: ReactNode;
  footerDismiss?: ReactElement | undefined;
  dismissLabel?: string;
  footerRefresh?: ReactNode;
  footer?: ReactNode;
  form?: {
    onSubmit: SubmitEventHandler<HTMLFormElement>;
    noValidate?: boolean;
  };
  onClose: () => void;
  busy?: boolean;
  size?: 'default' | 'full';
  scrollMode?: 'content' | 'viewport';
  contentClassName?: string;
  closeLabel?: string;
  onCloseAutoFocus?: ComponentProps<typeof Dialog.Content>['onCloseAutoFocus'];
}

export function Drawer({
  ref,
  title,
  description,
  truncateDescription = false,
  headerNavigation,
  footerNavigation,
  children,
  footerDismiss,
  dismissLabel = '关闭',
  footerRefresh,
  footer,
  form,
  onClose,
  busy = false,
  size = 'default',
  scrollMode = 'content',
  contentClassName,
  closeLabel = '关闭抽屉',
  onCloseAutoFocus,
}: Props) {
  const viewportScroll = scrollMode === 'viewport';
  const container = useRef<HTMLDivElement>(null);
  const overlay = useRef<HTMLDivElement>(null);
  const afterExit = useRef<(() => void) | null>(null);
  const [closing, setClosing] = useState(false);

  const close = useCallback(
    (afterClose = onClose) => {
      if (afterExit.current) return;
      afterExit.current = afterClose;
      setClosing(true);
    },
    [onClose],
  );
  useImperativeHandle(ref, () => ({ close }), [close]);

  useLayoutEffect(() => {
    if (!closing) return;

    const panel = container.current?.querySelector(':scope > .drawer-panel');
    const transitions = [overlay.current, panel].flatMap((element) =>
      element && typeof element.getAnimations === 'function' ? element.getAnimations() : [],
    );
    // 面板和遮罩退出后再释放焦点和滚动锁定。
    let mounted = true;
    void Promise.allSettled(transitions.map((transition) => transition.finished)).then(() => {
      if (!mounted) return;
      const afterClose = afterExit.current;
      afterExit.current = null;
      setClosing(false);
      afterClose?.();
    });
    return () => {
      mounted = false;
    };
  }, [closing]);

  const fitViewport = useCallback(() => {
    const element = container.current;
    if (!element) return;

    // 使用实际可见高度，避免地址栏或键盘遮住抽屉滚动区的底部。
    const viewport = window.visualViewport;
    const height = viewport?.height ?? window.innerHeight;
    const top = viewport?.offsetTop ?? 0;
    if (height <= 0) return;
    element.style.height = `${String(height)}px`;
    element.style.top = `${String(top)}px`;
  }, []);
  const setContainer = useCallback(
    (element: HTMLDivElement | null) => {
      container.current = element;
      if (!element) return;

      // Portal 可能晚于抽屉首次布局挂载，在节点就绪时立即同步视口。
      fitViewport();
      const clearMessageContainer = setMessageContainer(element);
      return () => {
        // 保留消息容器的清理，关闭内层抽屉后恢复上一层或页面消息。
        clearMessageContainer?.();
        container.current = null;
      };
    },
    [fitViewport],
  );

  useLayoutEffect(() => {
    fitViewport();
    const viewport = window.visualViewport;
    window.addEventListener('resize', fitViewport);
    viewport?.addEventListener('resize', fitViewport);
    viewport?.addEventListener('scroll', fitViewport);

    // 清理可视区域监听，嵌套抽屉分别维护自己的尺寸。
    return () => {
      window.removeEventListener('resize', fitViewport);
      viewport?.removeEventListener('resize', fitViewport);
      viewport?.removeEventListener('scroll', fitViewport);
    };
  }, [fitViewport]);

  const panelClassName = cn(
    'drawer-panel glass-panel flex flex-col border-l border-border bg-popover shadow-drawer outline-none',
    viewportScroll
      ? 'relative ml-auto min-h-full'
      : 'absolute inset-y-0 right-0 min-h-0 overflow-hidden',
    size === 'full' ? 'w-full max-w-none min-w-0' : 'w-full max-w-[600px]',
  );
  const descriptionElement = (
    <Dialog.Description
      className={cn('text-xs leading-4 text-muted-foreground', truncateDescription && 'truncate')}
    >
      {description}
    </Dialog.Description>
  );
  const content = (
    <>
      <div
        className={cn(
          'flex shrink-0 items-center justify-between gap-3 border-b border-border-subtle px-4 py-3 sm:px-6',
          viewportScroll && 'sticky top-0 z-10 bg-popover backdrop-blur-xl',
        )}
      >
        <div className="min-w-0 flex-1 space-y-0.5 wrap-anywhere">
          <Tooltip content={title} onlyOverflow>
            <Dialog.Title
              data-slot="drawer-title"
              className="truncate text-xl leading-6 font-medium"
            >
              {title}
            </Dialog.Title>
          </Tooltip>
          {description &&
            (truncateDescription ? (
              <Tooltip content={description} onlyOverflow>
                {descriptionElement}
              </Tooltip>
            ) : (
              descriptionElement
            ))}
        </div>
        <Dialog.Close asChild>
          <Button
            type="button"
            variant="cancel"
            size="icon-sm"
            className="border-0"
            aria-label={closeLabel}
            disabled={busy || closing}
          >
            <X aria-hidden="true" />
          </Button>
        </Dialog.Close>
      </div>
      {headerNavigation ? (
        <div className="shrink-0 border-b border-border-subtle px-5 py-4">{headerNavigation}</div>
      ) : null}
      <div
        className={cn(
          viewportScroll ? 'shrink-0' : 'min-h-0 flex-1 overflow-y-auto overscroll-contain',
          contentClassName,
        )}
      >
        {children}
      </div>
      <div
        data-slot="drawer-footer"
        className={cn(
          'mt-auto flex shrink-0 flex-wrap items-center justify-end border-t border-border-subtle px-4 sm:px-6',
          footerNavigation
            ? 'gap-2 pt-2 pb-[max(0.5rem,env(safe-area-inset-bottom,0px))]'
            : 'gap-2 pt-3 pb-[max(0.75rem,env(safe-area-inset-bottom,0px))] sm:gap-3',
        )}
      >
        {footerNavigation ? (
          <div data-slot="drawer-footer-navigation" className="mr-auto max-w-full min-w-0">
            {footerNavigation}
          </div>
        ) : null}
        <div className="ml-auto flex max-w-full flex-wrap items-center justify-end gap-2 sm:gap-3">
          {footerDismiss ?? (
            <Dialog.Close asChild>
              <Button type="button" variant="cancel" disabled={busy || closing}>
                {dismissLabel}
              </Button>
            </Dialog.Close>
          )}
          {footerRefresh}
          {footer}
        </div>
      </div>
    </>
  );
  return (
    <Dialog.Root
      open
      onOpenChange={(open) => {
        if (!open && !busy) close();
      }}
    >
      <Dialog.Portal>
        <Dialog.Overlay
          ref={overlay}
          data-closing={closing || undefined}
          className="drawer-overlay fixed inset-0 z-50 bg-overlay/20 backdrop-blur-sm"
        />
        <Dialog.Content
          {...(description ? {} : { 'aria-describedby': undefined })}
          ref={setContainer}
          data-closing={closing || undefined}
          onCloseAutoFocus={onCloseAutoFocus}
          style={{ pointerEvents: viewportScroll ? 'auto' : 'none' }}
          className={cn(
            'drawer-root fixed inset-x-0 top-0 z-50 h-dvh outline-none',
            viewportScroll
              ? 'touch-pan-y touch-pinch-zoom scroll-pb-[calc(1rem+env(safe-area-inset-bottom,0px))] overflow-x-hidden overflow-y-auto overscroll-y-contain'
              : 'overflow-hidden',
          )}
        >
          {form ? (
            <form
              {...form}
              inert={closing}
              style={{ pointerEvents: 'auto' }}
              className={panelClassName}
            >
              {content}
            </form>
          ) : (
            <div inert={closing} style={{ pointerEvents: 'auto' }} className={panelClassName}>
              {content}
            </div>
          )}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
