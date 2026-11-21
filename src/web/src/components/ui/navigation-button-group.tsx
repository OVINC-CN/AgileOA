import type { ComponentProps } from 'react';

import { NavigationMotion } from '@/components/navigation-motion';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

export function NavigationButtonGroup({
  className,
  ...props
}: Omit<ComponentProps<typeof NavigationMotion>, 'as'>) {
  return (
    <NavigationMotion
      {...props}
      className={cn(
        'inline-flex max-w-full gap-1 overflow-x-auto rounded-lg border border-border bg-background/60 p-1 backdrop-blur-xl',
        className,
      )}
    />
  );
}

export function NavigationGroupButton({
  className,
  ...props
}: Omit<ComponentProps<typeof Button>, 'size' | 'variant'>) {
  return (
    <Button
      type="button"
      variant="ghost"
      size="sm"
      {...props}
      className={cn(
        'px-2 hover:bg-foreground/8 aria-pressed:text-accent-foreground',
        'aria-selected:text-accent-foreground',
        className,
      )}
    />
  );
}
