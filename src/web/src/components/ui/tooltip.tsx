import { Tooltip as TooltipPrimitive } from 'radix-ui';
import { type ReactElement, useRef, useState } from 'react';

export function Tooltip({
  children,
  content,
  onlyOverflow = false,
}: {
  children: ReactElement;
  content: string;
  onlyOverflow?: boolean;
}) {
  const trigger = useRef<HTMLElement>(null);
  const [open, setOpen] = useState(false);
  return (
    <TooltipPrimitive.Provider delayDuration={300}>
      <TooltipPrimitive.Root
        open={open}
        onOpenChange={(next) => {
          // 在实际悬停或聚焦时测量，适应触发元素的宽度及内容变化。
          const text =
            trigger.current?.querySelector<HTMLElement>('[data-tooltip-overflow]') ??
            trigger.current;
          setOpen(
            next && (!onlyOverflow || (text !== null && text.scrollWidth > text.clientWidth)),
          );
        }}
      >
        <TooltipPrimitive.Trigger
          ref={(element) => {
            trigger.current = element;
          }}
          asChild
        >
          {children}
        </TooltipPrimitive.Trigger>
        <TooltipPrimitive.Portal>
          <TooltipPrimitive.Content
            sideOffset={6}
            className="glass-panel z-[60] max-w-[min(36rem,calc(100vw-2rem))] rounded-lg border bg-popover px-3 py-2 text-sm leading-relaxed wrap-anywhere whitespace-normal text-popover-foreground shadow-popover"
          >
            {content}
          </TooltipPrimitive.Content>
        </TooltipPrimitive.Portal>
      </TooltipPrimitive.Root>
    </TooltipPrimitive.Provider>
  );
}
