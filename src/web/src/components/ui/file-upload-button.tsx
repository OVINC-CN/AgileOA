import { type ComponentProps, useRef, useState } from 'react';

import { UploadButton } from '@/components/ui/upload-button';
import {
  uploadAccept,
  type UploadStatus,
  type UploadUpdate,
  validateUploadFile,
} from '@/lib/file-upload';
import { showMessage } from '@/lib/messages';

type Props = Pick<ComponentProps<'button'>, 'className' | 'disabled' | 'title'> & {
  accept?: string;
  label?: string;
  loading?: boolean;
  onFileSelect: (file: File, updateUpload: UploadUpdate) => void;
};

export function FileUploadButton({
  accept,
  label = '上传附件',
  loading = false,
  disabled = false,
  onFileSelect,
  ...props
}: Props) {
  const input = useRef<HTMLInputElement>(null);
  const [progress, setProgress] = useState<UploadStatus>(null);
  return (
    <>
      <input
        ref={input}
        type="file"
        accept={accept ?? uploadAccept()}
        aria-label={label}
        tabIndex={-1}
        className="sr-only"
        disabled={disabled || loading}
        onChange={(event) => {
          // 选择后立即交给所属业务上传，取消选择不发起请求，并允许重选同一文件。
          const file = event.currentTarget.files?.[0];
          event.currentTarget.value = '';
          if (file && !disabled && !loading) {
            try {
              validateUploadFile(file);
            } catch (error) {
              if (error instanceof Error) showMessage(error.message);
              return;
            }
            setProgress(null);
            onFileSelect(file, setProgress);
          }
        }}
      />
      <UploadButton
        {...props}
        variant="outline"
        size="sm"
        disabled={disabled}
        loading={loading}
        progress={progress}
        onDismiss={() => {
          setProgress(null);
        }}
        onClick={() => {
          input.current?.click();
        }}
      >
        {label === '上传附件' ? '上传' : label}
      </UploadButton>
    </>
  );
}
