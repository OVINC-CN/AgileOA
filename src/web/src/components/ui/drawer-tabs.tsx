import { NavigationMotion } from '@/components/navigation-motion';
import { Button } from '@/components/ui/button';

export function DrawerTabs({
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
  return (
    <NavigationMotion
      value={value}
      role="tablist"
      aria-label={label}
      className="inline-flex max-w-full gap-1 overflow-x-auto rounded-lg border border-border bg-background/60 p-1 backdrop-blur-xl"
    >
      {items.map((item, index) => (
        <Button
          key={item.value}
          type="button"
          variant="ghost"
          size="sm"
          role="tab"
          id={`${id}-tab-${item.value}`}
          aria-selected={value === item.value}
          aria-controls={`${id}-panel-${item.value}`}
          tabIndex={value === item.value ? 0 : -1}
          disabled={disabled}
          className="px-2 hover:bg-transparent aria-selected:text-accent-foreground"
          onClick={() => {
            onChange(item.value);
          }}
          onKeyDown={(event) => {
            if (disabled || event.metaKey || event.ctrlKey || event.altKey || event.shiftKey)
              return;
            const next =
              event.key === 'ArrowRight'
                ? items[(index + 1) % items.length]
                : event.key === 'ArrowLeft'
                  ? items[(index + items.length - 1) % items.length]
                  : event.key === 'Home'
                    ? items[0]
                    : event.key === 'End'
                      ? items.at(-1)
                      : undefined;
            if (!next) return;
            event.preventDefault();
            onChange(next.value);
            document.getElementById(`${id}-tab-${next.value}`)?.focus();
          }}
        >
          {item.label}
        </Button>
      ))}
    </NavigationMotion>
  );
}
