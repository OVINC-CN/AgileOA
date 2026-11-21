import { useLayoutEffect, useRef } from 'react';

import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

export function DrawerSteps({
  id,
  label,
  value,
  items,
  onChange,
  disabled = false,
}: {
  id: string;
  label: string;
  value: string;
  items: readonly { value: string; label: string }[];
  onChange: (value: string) => void;
  disabled?: boolean;
}) {
  const activeIndex = items.findIndex((item) => item.value === value);
  const navigation = useRef<HTMLDivElement>(null);
  useLayoutEffect(() => {
    // 键盘切换后将焦点移到当前步骤，提交错误由表单定位到对应字段。
    if (navigation.current?.contains(document.activeElement)) {
      document.getElementById(`${id}-tab-${value}`)?.focus();
    }
  }, [id, value]);
  return (
    <div
      ref={navigation}
      role="tablist"
      aria-label={label}
      className="relative mx-auto grid w-full max-w-xl gap-1"
      style={{ gridTemplateColumns: `repeat(${String(items.length)}, minmax(0, 1fr))` }}
    >
      {items.length > 1 ? (
        <span
          aria-hidden="true"
          className="absolute top-4 h-px bg-border"
          style={{ left: `${String(50 / items.length)}%`, right: `${String(50 / items.length)}%` }}
        />
      ) : null}
      {items.map((item, index) => (
        <Button
          key={item.value}
          type="button"
          variant="ghost"
          role="tab"
          id={`${id}-tab-${item.value}`}
          aria-label={item.label}
          aria-selected={value === item.value}
          aria-controls={`${id}-panel-${item.value}`}
          tabIndex={value === item.value ? 0 : -1}
          disabled={disabled}
          className="relative h-auto min-w-0 flex-col gap-2 px-0 py-1 font-normal aria-selected:text-accent-foreground"
          onClick={() => {
            onChange(item.value);
          }}
          onKeyDown={(event) => {
            if (disabled || event.metaKey || event.ctrlKey || event.altKey || event.shiftKey)
              return;
            const next =
              event.key === 'ArrowRight'
                ? items[Math.min(index + 1, items.length - 1)]
                : event.key === 'ArrowLeft'
                  ? items[Math.max(index - 1, 0)]
                  : event.key === 'Home'
                    ? items[0]
                    : event.key === 'End'
                      ? items.at(-1)
                      : undefined;
            if (!next) return;
            event.preventDefault();
            onChange(next.value);
          }}
        >
          <span
            aria-hidden="true"
            className={cn(
              'flex size-6 shrink-0 items-center justify-center rounded-full border bg-popover text-xs tabular-nums',
              index === activeIndex && 'border-primary bg-accent text-accent-foreground',
            )}
          >
            {index + 1}
          </span>
          <span className="text-xs leading-4">{item.label}</span>
        </Button>
      ))}
    </div>
  );
}
