import { type LucideIcon, PanelLeftClose } from 'lucide-react';
import { Popover } from 'radix-ui';
import { type ReactNode, useId, useLayoutEffect, useRef, useState } from 'react';
import { Link, NavLink, useLocation } from 'react-router';

import { NavigationMotion } from '@/components/navigation-motion';
import { Button } from '@/components/ui/button';
import { Tooltip } from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';

export interface NavigationItem {
  to: string;
  label: string;
  icon: LucideIcon;
  children?: {
    id: string;
    to: string;
    label: string;
    isActive: boolean;
  }[];
}
const navigationItemClass =
  'group flex h-9 min-w-0 flex-none items-center gap-2 rounded-lg px-3 text-sm font-normal text-foreground transition-colors hover:bg-muted focus-visible:outline-2 focus-visible:outline-ring motion-reduce:transition-none';

function CollapsedNavigationItem({ item }: { item: NavigationItem }) {
  const [open, setOpen] = useState(false);
  const Icon = item.icon;
  const active = item.children?.some((child) => child.isActive) ?? false;

  return (
    <Popover.Root open={open} onOpenChange={setOpen}>
      <Tooltip content={item.label}>
        <Popover.Trigger asChild>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            aria-label={item.label}
            aria-current={active ? 'page' : undefined}
            className={cn(
              navigationItemClass,
              'relative z-10 ml-2.5 size-9 justify-center gap-0 px-0',
              'aria-[current=page]:text-accent-foreground',
            )}
          >
            <Icon className="size-4 shrink-0" aria-hidden="true" />
          </Button>
        </Popover.Trigger>
      </Tooltip>
      <Popover.Portal>
        <Popover.Content
          aria-label={`${item.label}子菜单`}
          side="right"
          align="start"
          sideOffset={8}
          collisionPadding={16}
          className="glass-panel menu-motion z-50 max-h-[var(--radix-popover-content-available-height)] w-56 max-w-[calc(100vw-2rem)] overflow-y-auto rounded-xl border bg-popover p-1 shadow-popover"
        >
          <p className="px-3 py-2 text-xs text-muted-foreground">{item.label}</p>
          <nav aria-label={`${item.label}子菜单导航`} className="space-y-1">
            {item.children?.map((child) => (
              <Link
                key={child.id}
                to={child.to}
                preventScrollReset
                aria-current={child.isActive ? 'page' : undefined}
                className={cn(
                  navigationItemClass,
                  'aria-[current=page]:bg-accent aria-[current=page]:font-medium',
                  'aria-[current=page]:text-accent-foreground',
                )}
                onClick={(event) => {
                  // 保留修饰键与新标签页操作，普通跳转后关闭菜单。
                  if (
                    event.defaultPrevented ||
                    event.button !== 0 ||
                    event.metaKey ||
                    event.ctrlKey ||
                    event.altKey ||
                    event.shiftKey
                  )
                    return;
                  setOpen(false);
                }}
              >
                <span className="min-w-0 flex-1 wrap-anywhere">{child.label}</span>
              </Link>
            ))}
          </nav>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}

export function SideNavigationLayout({
  label,
  items,
  children,
}: {
  label: string;
  items: NavigationItem[];
  children: ReactNode;
}) {
  const [collapsed, setCollapsed] = useState(false);
  const toggle = useRef<HTMLButtonElement>(null);
  const restoreToggleFocus = useRef(false);
  const navigationID = useId();
  const location = useLocation();

  useLayoutEffect(() => {
    // 折叠与展开后将键盘焦点保留在同一操作入口。
    if (!restoreToggleFocus.current) return;
    toggle.current?.focus();
    restoreToggleFocus.current = false;
  }, [collapsed]);

  const toggleButton = (
    <Button
      ref={toggle}
      type="button"
      variant="ghost"
      aria-label={collapsed ? '展开侧栏' : '折叠侧栏'}
      aria-expanded={!collapsed}
      aria-controls={navigationID}
      className={cn(
        navigationItemClass,
        'side-navigation-item relative gap-0 px-0',
        collapsed ? 'ml-2.5 size-9' : 'w-full',
      )}
      onClick={() => {
        restoreToggleFocus.current = true;
        setCollapsed(!collapsed);
      }}
    >
      <PanelLeftClose
        aria-hidden="true"
        className={cn(
          'side-navigation-icon size-4 shrink-0 transition-transform motion-reduce:transition-none',
          collapsed && 'rotate-180',
        )}
      />
      <span aria-hidden={collapsed} className="side-navigation-label truncate">
        折叠侧栏
      </span>
    </Button>
  );
  return (
    <div
      data-collapsed={collapsed}
      className="side-navigation-layout grid flex-1 items-stretch gap-4 lg:gap-6"
    >
      <aside aria-label={label} className="hidden min-w-0 lg:flex lg:flex-col">
        <NavigationMotion
          as="nav"
          value={location.pathname + location.search}
          id={navigationID}
          aria-label={`${label}导航`}
          className="flex min-h-0 flex-col gap-1 overflow-x-hidden overflow-y-auto lg:sticky lg:top-20 lg:max-h-[calc(100dvh-9rem)]"
        >
          {items.map(({ to, label, icon: Icon, children: subItems }) => {
            const hasActiveChild = subItems?.some((item) => item.isActive) ?? false;

            if (collapsed && subItems?.length) {
              return (
                <CollapsedNavigationItem
                  key={to}
                  item={{ to, label, icon: Icon, children: subItems }}
                />
              );
            }

            const link = (
              <NavLink
                to={to}
                aria-label={label}
                aria-current={subItems?.length ? false : 'page'}
                data-current-child={hasActiveChild || undefined}
                className={cn(
                  navigationItemClass,
                  'side-navigation-item relative z-10 gap-0 px-0',
                  'aria-[current=page]:font-medium aria-[current=page]:text-accent-foreground',
                  'data-[current-child=true]:font-medium',
                  collapsed && 'ml-2.5 size-9',
                )}
              >
                <Icon className="side-navigation-icon size-4 shrink-0" aria-hidden="true" />
                <span aria-hidden={collapsed} className="side-navigation-label truncate">
                  {label}
                </span>
              </NavLink>
            );
            return (
              <div key={to} className="contents">
                <Tooltip content={label} disabled={!collapsed}>
                  {link}
                </Tooltip>
                {hasActiveChild && !collapsed ? (
                  <div
                    role="group"
                    aria-label={`${label}子菜单`}
                    className="ml-6 flex flex-col gap-1 border-l border-border pl-2"
                  >
                    {subItems?.map((item) => (
                      <Link
                        key={item.id}
                        id={item.id}
                        to={item.to}
                        preventScrollReset
                        aria-label={item.label}
                        aria-current={item.isActive ? 'page' : undefined}
                        className={cn(
                          navigationItemClass,
                          'relative z-10 h-8 text-xs',
                          'aria-[current=page]:font-medium aria-[current=page]:text-accent-foreground',
                        )}
                      >
                        <span className="truncate">{item.label}</span>
                      </Link>
                    ))}
                  </div>
                ) : null}
              </div>
            );
          })}
        </NavigationMotion>
        <div className="mt-auto shrink-0 overflow-hidden border-t border-border-subtle bg-background py-3 lg:sticky lg:bottom-0">
          <Tooltip content="展开侧栏" disabled={!collapsed}>
            {toggleButton}
          </Tooltip>
        </div>
      </aside>
      <div className="min-w-0">{children}</div>
    </div>
  );
}
