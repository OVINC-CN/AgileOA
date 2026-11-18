import { LoaderCircle } from 'lucide-react';
import type { ReactNode } from 'react';

import { useLoadingDelay } from '@/lib/loading';
import { cn } from '@/lib/utils';

export function LoadingIcon({
  className,
  loading = true,
  fallback = null,
}: {
  className?: string;
  loading?: boolean;
  fallback?: ReactNode;
}) {
  const visible = useLoadingDelay(loading);
  if (!visible) return fallback;

  return (
    <LoaderCircle
      aria-hidden="true"
      className={cn('size-4 shrink-0 motion-safe:animate-spin', className)}
    />
  );
}
