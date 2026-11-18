import { type ComponentProps, useCallback, useLayoutEffect, useRef } from 'react';

import { Textarea } from '@/components/ui/textarea';
import { cn } from '@/lib/utils';

export function AutoTextarea({ className, value, ref, ...props }: ComponentProps<'textarea'>) {
  const input = useRef<HTMLTextAreaElement>(null);
  const setInputRef = useCallback(
    (element: HTMLTextAreaElement | null) => {
      input.current = element;
      if (typeof ref === 'function') {
        const cleanup = ref(element);
        if (typeof cleanup === 'function')
          return () => {
            input.current = null;
            cleanup();
          };
      } else if (ref) ref.current = element;
      return undefined;
    },
    [ref],
  );
  useLayoutEffect(() => {
    const element = input.current;
    if (!element) return;
    const resize = () => {
      if (element.getBoundingClientRect().width === 0) return;
      element.style.height = 'auto';
      element.style.height = `${String(element.scrollHeight)}px`;
    };
    resize();
    let width = element.getBoundingClientRect().width;
    const observer =
      typeof ResizeObserver === 'function'
        ? new ResizeObserver(() => {
            const next = element.getBoundingClientRect().width;
            if (next !== width) {
              width = next;
              resize();
            }
          })
        : undefined;
    observer?.observe(element);
    window.addEventListener('resize', resize);
    return () => {
      observer?.disconnect();
      window.removeEventListener('resize', resize);
    };
  }, [value]);
  return (
    <Textarea
      {...props}
      ref={setInputRef}
      value={value}
      className={cn('min-h-16 overflow-hidden text-sm', className)}
    />
  );
}
