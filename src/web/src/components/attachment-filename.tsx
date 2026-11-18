import { FilePreviewLink } from '@/components/file-preview';
import { Tooltip } from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';

export function AttachmentFilename({
  filename,
  className,
  downloadURL,
}: {
  filename: string;
  className?: string;
  downloadURL?: string | undefined;
}) {
  if (downloadURL)
    return <FilePreviewLink filename={filename} downloadURL={downloadURL} className={className} />;
  return (
    <Tooltip content={filename} onlyOverflow>
      <span className={cn('block min-w-0 truncate', className)}>{filename}</span>
    </Tooltip>
  );
}
