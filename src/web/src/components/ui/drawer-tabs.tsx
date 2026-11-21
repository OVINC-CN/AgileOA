import {
  NavigationButtonGroup,
  NavigationGroupButton,
} from '@/components/ui/navigation-button-group';

export function DrawerTabs({
  id,
  label,
  value,
  items,
  onChange,
  disabled = false,
  className,
}: {
  id: string;
  label: string;
  value: string;
  items: readonly { value: string; label: string }[];
  onChange: (value: string) => void;
  disabled?: boolean;
  className?: string;
}) {
  return (
    <NavigationButtonGroup value={value} role="tablist" aria-label={label} className={className}>
      {items.map((item, index) => (
        <NavigationGroupButton
          key={item.value}
          role="tab"
          id={`${id}-tab-${item.value}`}
          aria-selected={value === item.value}
          aria-controls={`${id}-panel-${item.value}`}
          tabIndex={value === item.value ? 0 : -1}
          disabled={disabled}
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
        </NavigationGroupButton>
      ))}
    </NavigationButtonGroup>
  );
}
