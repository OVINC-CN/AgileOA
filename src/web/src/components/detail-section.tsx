import { ChevronDown } from 'lucide-react';
import { Collapsible } from 'radix-ui';
import { type ReactNode, type Ref, useId } from 'react';

export function DetailSection({
  title,
  heading: Heading = 'h2',
  open,
  onOpenChange,
  sectionRef,
  children,
}: {
  title: string;
  heading?: 'h2' | 'h3';
  open?: boolean | undefined;
  onOpenChange?: ((open: boolean) => void) | undefined;
  sectionRef?: Ref<HTMLElement> | undefined;
  children: ReactNode;
}) {
  const id = useId();
  return (
    <Collapsible.Root asChild defaultOpen open={open} onOpenChange={onOpenChange}>
      <section ref={sectionRef} aria-labelledby={id} className="py-4">
        <Heading id={id} className={Heading === 'h2' ? 'text-base' : 'text-sm'}>
          <Collapsible.Trigger className="group flex w-full items-center justify-between gap-2 rounded-lg py-1 text-left font-medium text-foreground-secondary hover:text-accent-foreground focus-visible:outline-2 focus-visible:outline-ring">
            <span className="min-w-0 wrap-anywhere">{title}</span>
            <ChevronDown
              className="size-4 shrink-0 transition-transform duration-200 group-data-[state=open]:rotate-180 motion-reduce:transition-none"
              aria-hidden="true"
            />
          </Collapsible.Trigger>
        </Heading>
        <Collapsible.Content className="space-y-4 pt-4">{children}</Collapsible.Content>
      </section>
    </Collapsible.Root>
  );
}
