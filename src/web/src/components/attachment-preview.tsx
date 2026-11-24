import {
  ChevronLeft,
  ChevronRight,
  FileQuestion,
  Maximize2,
  RefreshCw,
  RotateCcw,
  RotateCw,
  ZoomIn,
  ZoomOut,
} from 'lucide-react';
import { useEffect, useRef, useState } from 'react';

import { Button } from '@/components/ui/button';
import { LoadingIcon } from '@/components/ui/loading-icon';
import { SearchSelect } from '@/components/ui/search-select';
import {
  attachmentPreviewErrorMessage,
  type AttachmentPreviewPosition,
  type AttachmentPreviewViewer,
  createAttachmentPreviewViewer,
  emptyPosition,
  maximumImageZoom,
  minimumImageZoom,
} from '@/lib/attachment-preview';
import { attachmentPreviewKind } from '@/lib/file-preview';
import { showMessage } from '@/lib/messages';
import { cn } from '@/lib/utils';

type PreviewAction =
  'previous' | 'next' | 'sheet' | 'zoomIn' | 'zoomOut' | 'fitWidth' | 'fitPage' | 'rotate';

export function AttachmentPreview({ file, contentType }: { file: File; contentType: string }) {
  const kind = attachmentPreviewKind(contentType);
  const container = useRef<HTMLDivElement>(null);
  const viewer = useRef<AttachmentPreviewViewer | null>(null);
  const fail = useRef<(error: unknown) => void>(() => undefined);
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<'loading' | 'ready' | 'error'>('loading');
  const [position, setPosition] = useState<AttachmentPreviewPosition>(emptyPosition);
  const [action, setAction] = useState<PreviewAction | null>(null);

  useEffect(() => {
    const host = container.current;
    if (!host) return;
    const controller = new AbortController();
    let failed = false;
    const isActive = () => !controller.signal.aborted && !failed;
    let current: AttachmentPreviewViewer | undefined;
    // 解析错误和后续翻页错误使用同一提示，保留外层已缓存的原文件供下载。
    const reportError = (error: unknown) => {
      if (!isActive()) return;
      failed = true;
      current?.destroy();
      current = undefined;
      viewer.current = null;
      setStatus('error');
      setAction(null);
      showMessage(attachmentPreviewErrorMessage(error));
    };
    fail.current = reportError;
    void (async () => {
      try {
        const created = await createAttachmentPreviewViewer(kind, host, (value) => {
          if (isActive()) setPosition(value);
        });
        if (!isActive()) {
          created.destroy();
          return;
        }
        current = created;
        viewer.current = created;
        await created.load(file, contentType);
        if (!isActive()) return;
        setStatus('ready');
      } catch (error) {
        reportError(error);
      }
    })();

    // 关闭、切换附件或重新预览时销毁解析 Worker、画布和事件监听。
    return () => {
      controller.abort();
      current?.destroy();
      viewer.current = null;
      host.replaceChildren();
    };
  }, [file, contentType, kind, attempt]);

  async function operate(
    name: PreviewAction,
    operation: (current: AttachmentPreviewViewer) => void | Promise<void>,
  ) {
    const current = viewer.current;
    if (!current || action) return;
    setAction(name);
    try {
      await operation(current);
    } catch (error) {
      if (viewer.current === current) fail.current(error);
    } finally {
      if (viewer.current === current) setAction(null);
    }
  }

  const disabled = status !== 'ready' || action !== null;
  return (
    <div
      className="flex min-h-0 w-full flex-col"
      aria-busy={status === 'loading' || action !== null}
    >
      <div className="flex shrink-0 flex-wrap items-center gap-2 border-b border-border-subtle bg-popover px-4 py-2">
        {kind === 'xlsx' ? (
          <SearchSelect
            label="工作表"
            required
            value={String(position.index)}
            options={(position.names ?? []).map((label, index) => ({
              value: String(index),
              label,
            }))}
            className="w-full min-w-0 sm:w-64"
            disabled={disabled}
            loading={action === 'sheet'}
            onValueChange={(value) => {
              void operate('sheet', (current) => current.goTo(Number(value)));
            }}
          />
        ) : kind === 'image' ? (
          <span className="text-xs text-muted-foreground tabular-nums" aria-live="polite">
            {Math.round((position.zoom ?? 1) * 100)}%
          </span>
        ) : (
          <>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={disabled || position.index === 0}
              loading={action === 'previous'}
              onClick={() => {
                void operate('previous', (current) => current.goTo(position.index - 1));
              }}
            >
              <ChevronLeft aria-hidden="true" />
              上一页
            </Button>
            <span className="text-xs text-muted-foreground tabular-nums" aria-live="polite">
              {position.total ? `${String(position.index + 1)} / ${String(position.total)}` : '-'}
              {kind === 'pptx' ? ' 张' : ' 页'}
            </span>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={disabled || position.index + 1 >= position.total}
              loading={action === 'next'}
              onClick={() => {
                void operate('next', (current) => current.goTo(position.index + 1));
              }}
            >
              下一页
              <ChevronRight aria-hidden="true" />
            </Button>
          </>
        )}
        <div className="flex items-center gap-2 sm:ml-auto">
          <Button
            type="button"
            variant="outline"
            size="icon-sm"
            aria-label="缩小"
            disabled={disabled || (kind === 'image' && position.zoom === minimumImageZoom)}
            loading={action === 'zoomOut'}
            onClick={() => {
              void operate('zoomOut', (current) => current.zoomOut());
            }}
          >
            <ZoomOut aria-hidden="true" />
          </Button>
          <Button
            type="button"
            variant="outline"
            size="icon-sm"
            aria-label="放大"
            disabled={disabled || (kind === 'image' && position.zoom === maximumImageZoom)}
            loading={action === 'zoomIn'}
            onClick={() => {
              void operate('zoomIn', (current) => current.zoomIn());
            }}
          >
            <ZoomIn aria-hidden="true" />
          </Button>
          {kind === 'image' || kind === 'pdf' ? (
            <Button
              type="button"
              variant="outline"
              size="sm"
              aria-label="顺时针旋转 90 度"
              disabled={disabled}
              loading={action === 'rotate'}
              onClick={() => {
                void operate('rotate', (current) => current.rotate());
              }}
            >
              <RotateCw aria-hidden="true" />
              旋转
            </Button>
          ) : null}
          {kind !== 'xlsx' && position.canFitPage !== false ? (
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={disabled}
              loading={action === 'fitPage'}
              onClick={() => {
                void operate('fitPage', (current) => current.fitPage());
              }}
            >
              {kind === 'image' ? (
                <RotateCcw aria-hidden="true" />
              ) : (
                <Maximize2 aria-hidden="true" />
              )}
              {kind === 'image' ? '重置' : '适应整页'}
            </Button>
          ) : null}
          {kind !== 'image' ? (
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={disabled}
              loading={action === 'fitWidth'}
              onClick={() => {
                void operate('fitWidth', (current) => current.fitWidth());
              }}
            >
              <Maximize2 aria-hidden="true" />
              适应宽度
            </Button>
          ) : null}
        </div>
      </div>
      <div className="relative min-h-0 flex-1">
        <div
          ref={container}
          role="region"
          aria-label="附件预览内容"
          title={file.name}
          className={cn(
            'h-full w-full overflow-auto bg-background outline-none focus-visible:ring-2 focus-visible:ring-ring/30',
            status !== 'ready' && 'invisible',
          )}
        />
        {status !== 'ready' ? (
          <div className="absolute inset-0 flex flex-col items-center justify-center gap-3 p-6 text-sm text-muted-foreground">
            {status === 'loading' ? (
              <>
                <LoadingIcon className="size-6" />
                正在加载预览
              </>
            ) : (
              <>
                <FileQuestion className="size-8" aria-hidden="true" />
                无法显示预览，可下载原文件查看
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => {
                    setStatus('loading');
                    setAction(null);
                    setPosition(emptyPosition);
                    setAttempt((value) => value + 1);
                  }}
                >
                  <RefreshCw aria-hidden="true" />
                  重新预览
                </Button>
              </>
            )}
          </div>
        ) : null}
      </div>
    </div>
  );
}
