import { type ComponentPropsWithoutRef, useCallback, useLayoutEffect, useRef } from 'react';
import { Link, type To } from 'react-router';

import { buttonVariants } from '@/components/ui/button';
import { cn } from '@/lib/utils';

export function NavigationMotion({
  as: Container = 'div',
  value,
  children,
  className,
  ...props
}: ComponentPropsWithoutRef<'div'> & { as?: 'div' | 'nav'; value: string }) {
  const container = useRef<HTMLElement>(null);
  const indicator = useRef<HTMLSpanElement>(null);
  const setContainer = useCallback((element: HTMLElement | null) => {
    container.current = element;
  }, []);

  // 以实际控件尺寸定位高亮，兼容横向导航、纵向侧栏及侧栏折叠。
  useLayoutEffect(() => {
    const root = container.current;
    const highlight = indicator.current;
    if (!root || !highlight) return;

    const selected = root.querySelector<HTMLElement>(
      '[aria-current="page"], [role="tab"][aria-selected="true"]',
    );
    const measure = () => {
      highlight.hidden = !selected;
      if (!selected) return;

      // 先集中读取尺寸，再写入样式，避免切换时反复触发布局计算。
      const { offsetWidth, offsetHeight, offsetLeft, offsetTop } = selected;
      const x = String(offsetLeft);
      const y = String(offsetTop);
      highlight.style.width = `${String(offsetWidth)}px`;
      highlight.style.height = `${String(offsetHeight)}px`;
      highlight.style.transform = `translate(${x}px, ${y}px)`;
    };
    measure();
    const frame = window.requestAnimationFrame(() => {
      highlight.dataset['ready'] = 'true';
    });
    const observer = typeof ResizeObserver === 'function' ? new ResizeObserver(measure) : undefined;
    observer?.observe(root);
    if (selected) observer?.observe(selected);
    window.addEventListener('resize', measure);

    // 清理尺寸监听，避免已卸载导航继续测量。
    return () => {
      window.cancelAnimationFrame(frame);
      observer?.disconnect();
      window.removeEventListener('resize', measure);
    };
  }, [value]);

  return (
    <Container {...props} ref={setContainer} className={cn('navigation-motion', className)}>
      <span ref={indicator} aria-hidden="true" className="navigation-motion-indicator" />
      {children}
    </Container>
  );
}

export function NavigationTabs({
  id,
  label,
  value,
  items,
  onChange,
}: {
  id: string;
  label: string;
  value: string | undefined;
  items: readonly { value: string; label: string; to: To }[];
  onChange?: () => void;
}) {
  if (items.length === 0) return null;

  return (
    <NavigationMotion
      value={value ?? ''}
      role="tablist"
      aria-label={label}
      className="hidden max-w-full gap-1 overflow-x-auto rounded-lg border border-border bg-background/60 p-1 backdrop-blur-xl lg:inline-flex"
    >
      {items.map((item, index) => (
        <Link
          key={item.value}
          to={item.to}
          preventScrollReset
          role="tab"
          id={`${id}-tab-${item.value}`}
          aria-selected={value === item.value}
          aria-controls={`${id}-panel-${item.value}`}
          tabIndex={value === item.value ? 0 : -1}
          className={cn(
            buttonVariants({ variant: 'ghost' }),
            'hover:bg-transparent aria-selected:text-accent-foreground motion-reduce:transition-none',
          )}
          onClick={(event) => {
            // 修饰键和中键沿用链接的浏览器行为，不修改当前页面的业务状态。
            if (
              event.defaultPrevented ||
              event.button !== 0 ||
              event.metaKey ||
              event.ctrlKey ||
              event.altKey ||
              event.shiftKey
            )
              return;
            onChange?.();
          }}
          onKeyDown={(event) => {
            // 保留浏览器快捷键，普通方向键、首尾键和空格用于切换 Tab。
            if (event.metaKey || event.ctrlKey || event.altKey || event.shiftKey) return;
            if (event.key === ' ') {
              event.preventDefault();
              event.currentTarget.click();
              return;
            }
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
            const target = document.getElementById(`${id}-tab-${next.value}`);
            target?.focus();
            target?.click();
          }}
        >
          {item.label}
        </Link>
      ))}
    </NavigationMotion>
  );
}
