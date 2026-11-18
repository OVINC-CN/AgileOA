import { type LucideIcon, PanelLeftClose } from 'lucide-react';
import { type ReactNode, useId, useState } from 'react';
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
  const navigationID = useId();
  const location = useLocation();
  return (
    <div
      className={cn(
        'grid items-start gap-4 lg:gap-6',
        collapsed ? 'lg:grid-cols-[56px_minmax(0,1fr)]' : 'lg:grid-cols-[160px_minmax(0,1fr)]',
      )}
    >
      <aside aria-label={label} className="hidden lg:sticky lg:top-20 lg:block">
        <NavigationMotion
          as="nav"
          value={location.pathname + location.search}
          id={navigationID}
          aria-label={`${label}导航`}
          className="flex flex-col gap-1"
        >
          {items.map(({ to, label, icon: Icon, children: subItems }) => {
            const hasActiveChild = subItems?.some((item) => item.isActive) ?? false;

            return (
              <div key={to} className="contents">
                <Tooltip content={label} onlyOverflow={!collapsed}>
                  <NavLink
                    to={to}
                    aria-label={label}
                    aria-current={subItems?.length ? false : 'page'}
                    data-current-child={hasActiveChild || undefined}
                    className={cn(
                      navigationItemClass,
                      'relative z-10',
                      'aria-[current=page]:font-medium aria-[current=page]:text-accent-foreground',
                      'data-[current-child=true]:font-medium',
                      collapsed && 'lg:justify-center lg:gap-0',
                    )}
                  >
                    <Icon className="size-4 shrink-0" aria-hidden="true" />
                    <span
                      data-tooltip-overflow
                      className={cn('truncate', collapsed && 'lg:hidden')}
                    >
                      {label}
                    </span>
                  </NavLink>
                </Tooltip>
                {hasActiveChild ? (
                  <div
                    role="group"
                    aria-label={`${label}子菜单`}
                    className={cn(
                      'ml-6 flex flex-col gap-1 border-l border-border pl-2',
                      collapsed && 'lg:ml-0 lg:border-0 lg:pl-0',
                    )}
                  >
                    {subItems?.map((item) => (
                      <Tooltip key={item.id} content={item.label} onlyOverflow={!collapsed}>
                        <Link
                          id={item.id}
                          to={item.to}
                          preventScrollReset
                          aria-label={item.label}
                          aria-current={item.isActive ? 'page' : undefined}
                          className={cn(
                            navigationItemClass,
                            'relative z-10 h-8 text-xs',
                            'aria-[current=page]:font-medium aria-[current=page]:text-accent-foreground',
                            collapsed && 'lg:justify-center lg:px-1',
                          )}
                        >
                          <span
                            data-tooltip-overflow
                            className={cn('truncate', collapsed && 'lg:hidden')}
                          >
                            {item.label}
                          </span>
                          {collapsed ? (
                            <span aria-hidden="true" className="hidden lg:inline">
                              {item.label.slice(0, 2)}
                            </span>
                          ) : null}
                        </Link>
                      </Tooltip>
                    ))}
                  </div>
                ) : null}
              </div>
            );
          })}
          <Tooltip content={collapsed ? '展开侧栏' : '折叠侧栏'}>
            <Button
              type="button"
              variant="ghost"
              aria-label={collapsed ? '展开侧栏' : '折叠侧栏'}
              aria-expanded={!collapsed}
              aria-controls={navigationID}
              className={cn(
                navigationItemClass,
                'mt-3 hidden justify-start lg:flex',
                collapsed && 'justify-center gap-0',
              )}
              onClick={() => {
                setCollapsed(!collapsed);
              }}
            >
              <PanelLeftClose
                aria-hidden="true"
                className={cn(
                  'size-4 shrink-0 transition-transform motion-reduce:transition-none',
                  collapsed && 'rotate-180',
                )}
              />
              <span className={cn('truncate', collapsed && 'hidden')}>
                {collapsed ? '展开侧栏' : '折叠侧栏'}
              </span>
            </Button>
          </Tooltip>
        </NavigationMotion>
      </aside>
      <div className="min-w-0">{children}</div>
    </div>
  );
}
