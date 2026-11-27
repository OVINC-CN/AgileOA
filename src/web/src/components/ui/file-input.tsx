import { FolderOpen, X } from 'lucide-react';
import { type ComponentProps, useId, useRef } from 'react';

import { Button } from '@/components/ui/button';
import { Tooltip } from '@/components/ui/tooltip';
import { uploadAccept, validateFileSize, validateUploadFile } from '@/lib/file-upload';
import { showMessage } from '@/lib/messages';
import { cn } from '@/lib/utils';

type Props = Omit<
  ComponentProps<'input'>,
  'type' | 'ref' | 'value' | 'defaultValue' | 'onChange' | 'multiple' | 'children'
> & {
  file: File | null;
  onFileChange: (file: File | null) => void;
};

export function FileInput({
  file,
  onFileChange,
  className,
  id,
  disabled,
  accept,
  ...props
}: Props) {
  const generatedID = useId();
  const input = useRef<HTMLInputElement>(null);
  return (
    <div className={cn('flex min-w-0 flex-wrap items-center gap-2', className)}>
      <input
        {...props}
        accept={accept ?? uploadAccept()}
        ref={input}
        id={id ?? generatedID}
        type="file"
        tabIndex={-1}
        className="sr-only"
        disabled={disabled}
        onChange={(event) => {
          // 文件由表单状态管理，清空底层值以允许再次选择同一个文件。
          const next = event.currentTarget.files?.[0] ?? null;
          event.currentTarget.value = '';
          if (next) {
            try {
              if (accept) validateFileSize(next);
              else validateUploadFile(next);
            } catch (error) {
              if (error instanceof Error) showMessage(error.message);
              return;
            }
          }
          onFileChange(next);
        }}
      />
      <Button
        type="button"
        variant="outline"
        disabled={disabled}
        onClick={() => {
          input.current?.click();
        }}
      >
        <FolderOpen aria-hidden="true" />
        选择文件
      </Button>
      <Tooltip content={file?.name ?? ''} onlyOverflow disabled={!file}>
        <span aria-live="polite" className="min-w-0 flex-1 truncate text-sm text-muted-foreground">
          {file?.name ?? '请选择文件'}
        </span>
      </Tooltip>
      {file ? (
        <Tooltip content="清空已选文件" disabled={disabled === true}>
          <Button
            type="button"
            variant="cancel"
            size="icon-xs"
            className="border-0 text-muted-foreground enabled:hover:text-foreground"
            aria-label="清空已选文件"
            disabled={disabled}
            onClick={() => {
              onFileChange(null);
            }}
          >
            <X aria-hidden="true" />
          </Button>
        </Tooltip>
      ) : null}
    </div>
  );
}
