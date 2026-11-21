import { cva, type VariantProps } from 'class-variance-authority';
import { Slot } from 'radix-ui';
import { type AriaAttributes, Children, type ComponentProps, isValidElement } from 'react';

import { LoadingIcon } from '@/components/ui/loading-icon';
import { useLoadingDelay } from '@/lib/loading';
import { cn } from '@/lib/utils';

const buttonVariants = cva(
  "inline-flex shrink-0 items-center justify-center gap-2 rounded-lg text-sm font-medium whitespace-nowrap transition-colors outline-none focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/30 disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 motion-reduce:transition-none dark:aria-invalid:ring-destructive/40 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        default: 'bg-primary text-primary-foreground hover:bg-primary/90',
        destructive:
          'bg-destructive text-primary-foreground hover:bg-destructive/90 focus-visible:ring-destructive/20 dark:bg-destructive/60 dark:focus-visible:ring-destructive/40',
        outline: 'form-control border text-foreground hover:bg-muted',
        cancel:
          'border border-input bg-transparent text-foreground enabled:hover:border-[var(--input-hover)] disabled:bg-muted',
        secondary: 'border border-border bg-secondary text-secondary-foreground hover:bg-muted',
        ghost: 'text-foreground-secondary hover:bg-secondary hover:text-foreground',
        link: 'text-accent-foreground underline-offset-4 hover:underline',
      },
      size: {
        default: 'h-9 px-3 py-0',
        xs: "h-6 gap-1 rounded-md px-2 text-xs [&_svg:not([class*='size-'])]:size-3",
        sm: 'h-8 px-3 py-0',
        lg: 'h-10 px-4 py-0',
        icon: 'size-9',
        'icon-xs': "size-6 rounded-md [&_svg:not([class*='size-'])]:size-3",
        'icon-sm': 'size-8',
        'icon-lg': 'size-10',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  },
);

function Button({
  className,
  variant = 'default',
  size = 'default',
  asChild = false,
  loading = false,
  disabled,
  children,
  ...props
}: ComponentProps<'button'> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean;
    loading?: boolean;
  }) {
  const Comp = asChild ? Slot.Root : 'button';
  const visibleLoading = useLoadingDelay(loading);
  const content = Children.toArray(children);
  const icon = content.findIndex(
    (child) =>
      isValidElement<{ 'aria-hidden'?: AriaAttributes['aria-hidden'] }>(child) &&
      (child.props['aria-hidden'] === true || child.props['aria-hidden'] === 'true'),
  );

  return (
    <Comp
      data-slot="button"
      data-variant={variant}
      data-size={size}
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
      disabled={loading ? true : disabled}
      aria-busy={visibleLoading ? true : props['aria-busy']}
    >
      {asChild ? (
        children
      ) : (
        <>
          {visibleLoading && icon < 0 ? <LoadingIcon /> : null}
          {content.map((child, index) =>
            visibleLoading && index === icon ? <LoadingIcon key={index} /> : child,
          )}
        </>
      )}
    </Comp>
  );
}

export { Button, buttonVariants };
