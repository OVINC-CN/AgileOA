import { Download, FileQuestion, RefreshCw } from 'lucide-react';
import { useEffect, useState } from 'react';

import { ImagePreview } from '@/components/image-preview';
import { OfficePreview } from '@/components/office-preview';
import { Button } from '@/components/ui/button';
import { Drawer } from '@/components/ui/drawer';
import { LoadingIcon } from '@/components/ui/loading-icon';
import { Tooltip } from '@/components/ui/tooltip';
import { ApiError } from '@/lib/api';
import { downloadFile } from '@/lib/download';
import { officePreviewKind, previewContentType, readPreviewFile } from '@/lib/file-preview';
import { showMessage } from '@/lib/messages';
import { cn } from '@/lib/utils';

export function FilePreviewLink({
  filename,
  downloadURL,
  className,
}: {
  filename: string;
  downloadURL: string;
  className?: string | undefined;
}) {
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const close = () => {
    setOpen(false);
    setLoading(false);
  };
  return (
    <>
      <Tooltip content={filename} onlyOverflow>
        <Button
          type="button"
          variant="link"
          size="sm"
          className={cn(
            'h-auto min-w-0 shrink justify-start rounded-sm p-0 text-left text-[length:inherit] leading-[inherit] font-normal text-inherit has-[>svg]:px-0',
            className,
          )}
          aria-label={filename}
          aria-haspopup="dialog"
          loading={loading}
          onClick={() => {
            setLoading(true);
            setOpen(true);
          }}
        >
          <span data-tooltip-overflow className="min-w-0 truncate">
            {filename}
          </span>
        </Button>
      </Tooltip>
      {open ? (
        <PreviewWindow
          key={downloadURL}
          filename={filename}
          downloadURL={downloadURL}
          onClose={close}
          onLoading={setLoading}
        />
      ) : null}
    </>
  );
}

type PreviewState =
  | { status: 'loading' | 'unsupported' | 'error'; blob?: Blob }
  | { status: 'ready'; blob: Blob; url: string; type: string };

function PreviewWindow({
  filename,
  downloadURL,
  onClose,
  onLoading,
}: {
  filename: string;
  downloadURL: string;
  onClose: () => void;
  onLoading: (loading: boolean) => void;
}) {
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState<PreviewState>({ status: 'loading' });
  const storedName = state.blob instanceof File ? state.blob.name : filename;
  const officeKind = state.status === 'ready' ? officePreviewKind(state.type) : null;
  useEffect(() => {
    const controller = new AbortController();
    let objectURL: string | undefined;
    let storedBlob: Blob | undefined;
    void (async () => {
      try {
        const blob = await readPreviewFile(downloadURL, controller.signal);
        storedBlob = blob;
        const type = await previewContentType(blob, blob.name);
        if (controller.signal.aborted) return;
        if (!type) {
          setState({ status: 'unsupported', blob });
          return;
        }
        objectURL = URL.createObjectURL(new Blob([blob], { type }));
        setState({ status: 'ready', blob, url: objectURL, type });
      } catch (error) {
        if (controller.signal.aborted) return;
        if (!(error instanceof ApiError)) showMessage('文件读取失败，请重试');
        setState(storedBlob ? { status: 'unsupported', blob: storedBlob } : { status: 'error' });
      } finally {
        if (!controller.signal.aborted) onLoading(false);
      }
    })();
    return () => {
      // 关闭、切换文件或工作空间后中止读取，并释放预览使用的对象 URL。
      controller.abort();
      if (objectURL) URL.revokeObjectURL(objectURL);
    };
  }, [downloadURL, attempt, onLoading]);
  return (
    <Drawer
      size="full"
      title="文件预览"
      description={storedName}
      onClose={onClose}
      contentClassName="flex overflow-hidden bg-muted/70"
      footer={
        <>
          <Button
            type="button"
            variant="outline"
            disabled={!state.blob}
            onClick={() => {
              if (!state.blob) return;
              try {
                downloadFile(storedName, state.blob);
              } catch {
                showMessage('文件保存失败，请重试');
              }
            }}
          >
            <Download aria-hidden="true" />
            下载
          </Button>
          <Button type="button" variant="secondary" onClick={onClose}>
            关闭
          </Button>
        </>
      }
    >
      {state.status === 'ready' ? (
        officeKind ? (
          <OfficePreview
            key={state.url}
            source={state.url}
            filename={storedName}
            kind={officeKind}
          />
        ) : state.type === 'application/pdf' ? (
          <iframe title={storedName} src={state.url} className="h-full w-full border-0" />
        ) : (
          <ImagePreview
            key={state.url}
            source={state.url}
            filename={storedName}
            onError={() => {
              setState({ status: 'unsupported', blob: state.blob });
            }}
          />
        )
      ) : (
        <div className="flex w-full flex-col items-center justify-center gap-3 p-6 text-sm text-muted-foreground">
          {state.status === 'loading' ? (
            <>
              <LoadingIcon className="size-6" />
              正在加载预览
            </>
          ) : state.status === 'error' ? (
            <Button
              type="button"
              variant="outline"
              onClick={() => {
                setState({ status: 'loading' });
                onLoading(true);
                setAttempt((value) => value + 1);
              }}
            >
              <RefreshCw aria-hidden="true" />
              重新加载
            </Button>
          ) : (
            <>
              <FileQuestion className="size-8" aria-hidden="true" />
              暂不支持预览此文件类型，支持 PDF、图片及 Office 文档
            </>
          )}
        </div>
      )}
    </Drawer>
  );
}
