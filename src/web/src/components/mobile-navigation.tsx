import { Check, type LucideIcon, Menu } from 'lucide-react';
import { useEffect, useId, useRef, useState } from 'react';
import { Link } from 'react-router';

import { Button } from '@/components/ui/button';
import { Drawer } from '@/components/ui/drawer';
import { cn } from '@/lib/utils';

export interface MobileNavigationItem {
  id: string;
  label: string;
  to: string;
  icon?: LucideIcon;
  isActive?: boolean;
  children?: readonly MobileNavigationItem[];
}

export function MobileNavigation({
  items,
  workspaceName,
  disabled = false,
}: {
  items: readonly MobileNavigationItem[];
  workspaceName: string;
  disabled?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const navigationID = useId();
  const trigger = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    // 与 lg 断点保持一致，放大到桌面后释放全屏菜单和滚动锁。
    if (typeof window.matchMedia !== 'function') return;
    const desktop = window.matchMedia('(min-width: 64rem)');
    const closeOnDesktop = (event: MediaQueryListEvent) => {
      if (event.matches) setOpen(false);
    };
    desktop.addEventListener('change', closeOnDesktop);
    return () => {
      desktop.removeEventListener('change', closeOnDesktop);
    };
  }, []);

  if (!items.length) return null;

  return (
    <>
      <Button
        ref={trigger}
        type="button"
        variant="ghost"
        size="sm"
        className="shrink-0 gap-1.5 px-2 text-foreground-secondary lg:hidden"
        aria-label="打开导航菜单"
        aria-haspopup="dialog"
        aria-expanded={open}
        aria-controls={open ? navigationID : undefined}
        disabled={disabled}
        onClick={() => {
          setOpen(true);
        }}
      >
        <Menu aria-hidden="true" />
        菜单
      </Button>
      {open ? (
        <Drawer
          title="导航菜单"
          description={workspaceName}
          closeLabel="关闭导航菜单"
          size="full"
          scrollMode="viewport"
          onClose={() => {
            setOpen(false);
          }}
          onCloseAutoFocus={(event) => {
            event.preventDefault();
            trigger.current?.focus();
          }}
        >
          <nav
            id={navigationID}
            aria-label="全部导航"
            className="px-4 pt-4 pb-[calc(1rem+env(safe-area-inset-bottom,0px))]"
          >
            <MenuLevel
              items={items}
              depth={0}
              onNavigate={() => {
                setOpen(false);
              }}
            />
          </nav>
        </Drawer>
      ) : null}
    </>
  );
}

function MenuLevel({
  items,
  depth,
  onNavigate,
}: {
  items: readonly MobileNavigationItem[];
  depth: number;
  onNavigate: () => void;
}) {
  return (
    <ul className={cn(depth === 0 ? 'space-y-4' : 'ml-5 space-y-1 border-l border-border pl-3')}>
      {items.map(({ id, label, to, icon: Icon, isActive, children }) => {
        const current = isActive && !children?.length;
        const link = (
          <Link
            to={to}
            aria-current={current ? 'page' : undefined}
            className={cn(
              'flex min-h-9 min-w-0 items-center gap-2 rounded-lg px-3 py-2',
              'text-foreground-secondary transition-colors hover:bg-muted hover:text-foreground',
              'focus-visible:outline-2 focus-visible:outline-ring motion-reduce:transition-none',
              depth === 0 ? 'text-base font-medium' : depth === 1 ? 'text-sm' : 'text-xs',
              isActive && 'font-medium text-foreground',
              current && 'bg-accent text-accent-foreground',
            )}
            onClick={(event) => {
              // 新标签页和浏览器快捷操作不关闭当前菜单。
              if (
                event.defaultPrevented ||
                event.button !== 0 ||
                event.metaKey ||
                event.ctrlKey ||
                event.altKey ||
                event.shiftKey
              )
                return;
              onNavigate();
            }}
          >
            {Icon ? <Icon className="size-4 shrink-0" aria-hidden="true" /> : null}
            <span className="min-w-0 flex-1 wrap-anywhere">{label}</span>
            {current ? <Check className="size-4 shrink-0" aria-hidden="true" /> : null}
          </Link>
        );

        return (
          <li
            key={id}
            className={cn(
              depth === 0 && 'border-b border-border-subtle pb-4 last:border-0 last:pb-0',
            )}
          >
            {depth === 0 ? <h2>{link}</h2> : link}
            {children?.length ? (
              <MenuLevel items={children} depth={depth + 1} onNavigate={onNavigate} />
            ) : null}
          </li>
        );
      })}
    </ul>
  );
}
