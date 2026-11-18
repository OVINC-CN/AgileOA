import { ArrowRight, GripVertical, LocateFixed } from 'lucide-react';
import {
  type ComponentProps,
  createContext,
  type PointerEvent,
  type ReactNode,
  useCallback,
  useContext,
  useId,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { createPortal } from 'react-dom';

import { Button } from '@/components/ui/button';
import { type MappingScrollLink, useMappingScroll } from '@/components/ui/mapping-scroll';
import { showMessage } from '@/lib/messages';
import { cn } from '@/lib/utils';

export interface MappingItem {
  id: string;
  label: string;
  disabled: boolean;
}

export interface MappingLink {
  id: string;
  sourceID: string;
  targetID: string;
}

interface MappingContext {
  highlightedSource: string;
  locatedTarget: string;
  dragging: string;
  hovered: string;
  sourceItems: ReadonlyMap<string, MappingItem>;
  targetItems: ReadonlyMap<string, MappingItem>;
  register: (side: 'source' | 'target', id: string, element: HTMLButtonElement | null) => void;
  locate: (side: 'source' | 'target', id: string, sourceID: string) => void;
  start: (id: string, event: PointerEvent<HTMLButtonElement>) => void;
  cancelDrag: () => boolean;
  problem: (sourceID: string, targetID: string) => string | null;
}

const Context = createContext<MappingContext | null>(null);

function useMappingBoard() {
  const context = useContext(Context);
  if (!context) throw new Error('映射控件必须放在映射面板内');
  return context;
}

export function DragMappingBoard({
  sourceTitle,
  targetTitle,
  source,
  target,
  sourceItems,
  targetItems,
  links,
  scrollLinks,
  syncScroll,
  validate,
  onConnect,
}: {
  sourceTitle: ReactNode;
  targetTitle: ReactNode;
  source: ReactNode;
  target: ReactNode;
  sourceItems: MappingItem[];
  targetItems: MappingItem[];
  links: MappingLink[];
  scrollLinks: MappingScrollLink[];
  syncScroll: boolean;
  validate: (sourceID: string, targetID: string) => string | null;
  onConnect: (sourceID: string, targetID: string) => void;
}) {
  const root = useRef<HTMLDivElement>(null);
  const canvas = useRef<HTMLDivElement>(null);
  const anchors = useRef({
    source: new Map<string, HTMLButtonElement>(),
    target: new Map<string, HTMLButtonElement>(),
  });
  const {
    source: sourcePanel,
    target: targetPanel,
    synchronize,
    ignore,
  } = useMappingScroll({ enabled: syncScroll, links: scrollLinks, anchors });
  const drag = useRef<{
    id: string;
    pointerID: number;
    startX: number;
    startY: number;
    x: number;
    y: number;
    active: boolean;
    capture: HTMLButtonElement;
  } | null>(null);
  const suppressClick = useRef(false);
  const frame = useRef(0);
  const [highlightedSource, setHighlightedSource] = useState('');
  const [dragging, setDragging] = useState('');
  const [locatedTarget, setLocatedTarget] = useState('');
  const [hovered, setHovered] = useState('');
  const [cursor, setCursor] = useState<{ x: number; y: number } | null>(null);
  const [announcement, setAnnouncement] = useState('');
  const [paths, setPaths] = useState<
    { id: string; sourceID: string; d: string; clipped: boolean }[]
  >([]);
  const marker = useId().replaceAll(':', '');
  const sourceLookup = useMemo(
    () => new Map(sourceItems.map((item) => [item.id, item])),
    [sourceItems],
  );
  const targetLookup = useMemo(
    () => new Map(targetItems.map((item) => [item.id, item])),
    [targetItems],
  );

  const measure = useCallback(() => {
    const bounds = canvas.current?.getBoundingClientRect();
    if (!bounds) return;
    const endpoint = (element: HTMLElement) => {
      const rectangle = element.getBoundingClientRect();
      const viewport = element
        .closest<HTMLElement>('[data-mapping-scroll]')
        ?.getBoundingClientRect();
      if (!viewport || viewport.height <= 0) return null;
      const center = rectangle.top + rectangle.height / 2;
      const visible = center >= viewport.top && center <= viewport.bottom;
      const inset = Math.min(8, viewport.height / 2);
      const y = visible
        ? center
        : Math.max(viewport.top + inset, Math.min(viewport.bottom - inset, center));
      return {
        rectangle,
        visible,
        y: y - bounds.top,
      };
    };

    // 一端滚出可视区域时，将连线保留在该侧的上下边缘，避免映射关系消失。
    const next = links.flatMap((link) => {
      const left = anchors.current.source.get(link.sourceID);
      const right = anchors.current.target.get(link.targetID);
      if (!left || !right) return [];
      const from = endpoint(left);
      const to = endpoint(right);
      if (!from || !to || (!from.visible && !to.visible)) return [];
      const x1 = from.rectangle.right - bounds.left;
      const x2 = to.rectangle.left - bounds.left;
      const bend = Math.max(24, (x2 - x1) / 2);
      return [
        {
          id: link.id,
          sourceID: link.sourceID,
          clipped: !from.visible || !to.visible,
          d: [
            `M ${String(x1)} ${String(from.y)}`,
            `C ${String(x1 + bend)} ${String(from.y)},`,
            `${String(x2 - bend)} ${String(to.y)},`,
            `${String(x2 - 3)} ${String(to.y)}`,
          ].join(' '),
        },
      ];
    });
    setPaths((before) => (JSON.stringify(before) === JSON.stringify(next) ? before : next));
  }, [links]);

  useLayoutEffect(() => {
    const element = canvas.current;
    if (!element) return;
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    for (const side of ['source', 'target'] as const) {
      for (const anchor of anchors.current[side].values()) observer.observe(anchor);
    }
    window.addEventListener('resize', measure);
    return () => {
      observer.disconnect();
      window.removeEventListener('resize', measure);
    };
  }, [measure, sourceItems, targetItems]);

  useLayoutEffect(
    () => () => {
      cancelAnimationFrame(frame.current);
    },
    [],
  );

  const register = useCallback<MappingContext['register']>((side, id, element) => {
    if (element) anchors.current[side].set(id, element);
    else anchors.current[side].delete(id);
  }, []);

  const connect = useCallback(
    (sourceID: string, targetID: string) => {
      const sourceItem = sourceItems.find((item) => item.id === sourceID);
      const targetItem = targetItems.find((item) => item.id === targetID);
      if (!sourceItem || !targetItem) return;
      const error = validate(sourceID, targetID);
      if (error) {
        showMessage(error);
        setAnnouncement(error);
        return;
      }
      onConnect(sourceID, targetID);
      setHighlightedSource(sourceID);
      setLocatedTarget(targetID);
      setAnnouncement(`已建立映射：${sourceItem.label} → ${targetItem.label}`);
    },
    [onConnect, sourceItems, targetItems, validate],
  );

  const locate = useCallback<MappingContext['locate']>(
    (side, id, sourceID) => {
      const element = anchors.current[side].get(id);
      const item = (side === 'source' ? sourceLookup : targetLookup).get(id);
      if (!element || !item) return;

      // 只定位已配置的映射，不触发新增映射；即时滚动也适用于减少动态效果的设置。
      setHighlightedSource(sourceID);
      setLocatedTarget(side === 'target' ? id : '');
      element.scrollIntoView({ block: 'center', inline: 'nearest', behavior: 'instant' });
      element.focus({ preventScroll: true });
      ignore(side, element.closest<HTMLElement>('[data-mapping-scroll]'));
      measure();
      setAnnouncement(`已定位${side === 'source' ? '来源' : '目标'}：${item.label}`);
    },
    [ignore, measure, sourceLookup, targetLookup],
  );

  const cancel = useCallback(() => {
    const active = drag.current;
    drag.current = null;
    cancelAnimationFrame(frame.current);
    if (active?.capture.hasPointerCapture(active.pointerID)) {
      active.capture.releasePointerCapture(active.pointerID);
    }
    setCursor(null);
    setDragging('');
    setHovered('');
    setHighlightedSource('');
    setLocatedTarget('');
  }, []);

  const cancelDrag = useCallback(() => {
    if (!drag.current) return false;
    suppressClick.current = drag.current.active;
    cancel();
    setAnnouncement('已取消拖拽');
    return true;
  }, [cancel]);

  const hitTarget = (x: number, y: number) => {
    const element = document.elementFromPoint(x, y)?.closest<HTMLElement>('[data-mapping-target]');
    return element && root.current?.contains(element)
      ? (element.dataset['mappingTarget'] ?? '')
      : '';
  };

  const autoScroll = () => {
    const active = drag.current;
    if (!active?.active) return;
    const panel = document
      .elementFromPoint(active.x, active.y)
      ?.closest<HTMLElement>('[data-mapping-scroll]');
    if (panel && root.current?.contains(panel)) {
      const bounds = panel.getBoundingClientRect();
      if (active.y < bounds.top + 36) panel.scrollTop -= 12;
      if (active.y > bounds.bottom - 36) panel.scrollTop += 12;
    }
    const bounds = root.current?.getBoundingClientRect();
    if (bounds && root.current) {
      if (active.x < bounds.left + 36) root.current.scrollLeft -= 12;
      if (active.x > bounds.right - 36) root.current.scrollLeft += 12;
    }
    setHovered(hitTarget(active.x, active.y));
    frame.current = requestAnimationFrame(autoScroll);
  };

  const start = useCallback<MappingContext['start']>((id, event) => {
    if (event.button !== 0 || drag.current || event.currentTarget.matches(':disabled')) return;
    setLocatedTarget('');
    event.currentTarget.focus();
    suppressClick.current = false;
    event.currentTarget.setPointerCapture(event.pointerId);
    drag.current = {
      id,
      pointerID: event.pointerId,
      startX: event.clientX,
      startY: event.clientY,
      x: event.clientX,
      y: event.clientY,
      active: false,
      capture: event.currentTarget,
    };
  }, []);
  const context = useMemo<MappingContext>(
    () => ({
      highlightedSource,
      locatedTarget,
      dragging,
      hovered,
      sourceItems: sourceLookup,
      targetItems: targetLookup,
      register,
      locate,
      start,
      cancelDrag,
      problem: validate,
    }),
    [
      highlightedSource,
      locatedTarget,
      dragging,
      hovered,
      sourceLookup,
      targetLookup,
      register,
      locate,
      start,
      cancelDrag,
      validate,
    ],
  );
  const activeLabel = sourceLookup.get(dragging)?.label;
  const scroll = (side: 'source' | 'target', panel: HTMLDivElement) => {
    if (!drag.current?.active) synchronize(side, panel);
    measure();
  };
  return (
    <Context.Provider value={context}>
      <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-2">
        <p role="status" aria-live="polite" className="sr-only">
          {announcement}
        </p>
        <div
          ref={root}
          className="min-h-0 flex-1 overflow-x-auto rounded-xl border border-border bg-background/50"
          onClickCapture={(event) => {
            if (!suppressClick.current) return;
            event.preventDefault();
            event.stopPropagation();
            suppressClick.current = false;
          }}
          onPointerMove={(event) => {
            const active = drag.current;
            if (event.pointerId !== active?.pointerID) return;
            active.x = event.clientX;
            active.y = event.clientY;
            if (
              !active.active &&
              Math.hypot(active.x - active.startX, active.y - active.startY) < 5
            )
              return;
            event.preventDefault();
            if (!active.active) {
              active.active = true;
              setDragging(active.id);
              setHighlightedSource(active.id);
              frame.current = requestAnimationFrame(autoScroll);
            }
            setCursor({ x: active.x, y: active.y });
            setHovered(hitTarget(active.x, active.y));
          }}
          onPointerUp={(event) => {
            const active = drag.current;
            if (event.pointerId !== active?.pointerID) return;
            if (active.active) {
              suppressClick.current = true;
              const targetID = hitTarget(event.clientX, event.clientY);
              cancel();
              if (targetID) connect(active.id, targetID);
            } else cancel();
          }}
          onPointerCancel={(event) => {
            if (event.pointerId === drag.current?.pointerID) cancel();
          }}
          onLostPointerCapture={(event) => {
            if (event.pointerId === drag.current?.pointerID) cancel();
          }}
          onScroll={measure}
        >
          <div
            ref={canvas}
            className="relative grid h-full min-w-[680px] grid-cols-[minmax(0,1fr)_64px_minmax(0,1fr)]"
          >
            <div className="flex min-h-0 min-w-0 flex-col">
              <div className="shrink-0 border-b border-border bg-background/70 px-4 py-3">
                {sourceTitle}
              </div>
              <div
                ref={sourcePanel}
                data-mapping-scroll="source"
                onScroll={(event) => {
                  scroll('source', event.currentTarget);
                }}
                className="min-h-0 flex-1 overflow-y-auto overscroll-contain p-4"
              >
                {source}
              </div>
            </div>
            <div className="flex items-start justify-center border-x border-border bg-muted/60 pt-4 text-muted-foreground">
              <ArrowRight aria-hidden="true" className="size-5" />
            </div>
            <div className="flex min-h-0 min-w-0 flex-col">
              <div className="shrink-0 border-b border-border bg-background/70 px-4 py-3">
                {targetTitle}
              </div>
              <div
                ref={targetPanel}
                data-mapping-scroll="target"
                onScroll={(event) => {
                  scroll('target', event.currentTarget);
                }}
                className="min-h-0 flex-1 overflow-y-auto overscroll-contain p-4"
              >
                {target}
              </div>
            </div>
            <svg
              aria-hidden="true"
              className="pointer-events-none absolute inset-0 z-10 size-full overflow-hidden text-primary"
            >
              <defs>
                <marker
                  id={marker}
                  markerWidth="7"
                  markerHeight="7"
                  refX="6"
                  refY="3.5"
                  orient="auto"
                >
                  <path d="M 0 0 L 7 3.5 L 0 7 Z" fill="currentColor" />
                </marker>
              </defs>
              {paths.map((path) => (
                <path
                  key={path.id}
                  d={path.d}
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.5"
                  strokeDasharray={path.clipped ? '4 4' : undefined}
                  markerEnd={`url(#${marker})`}
                  opacity={!highlightedSource || highlightedSource === path.sourceID ? 0.65 : 0.12}
                />
              ))}
            </svg>
          </div>
        </div>
        <p className="min-h-4 shrink-0 text-xs wrap-anywhere text-muted-foreground">
          {dragging && activeLabel
            ? `正在拖拽：${activeLabel}，放到右侧目标以建立映射；按 Esc 取消。`
            : '拖动左侧节点或字段到右侧建立映射；点击映射名称定位另一侧，虚线表示一端在可视区域外。'}
        </p>
      </div>
      {cursor && activeLabel
        ? createPortal(
            <div
              style={{ left: cursor.x + 16, top: cursor.y + 16 }}
              className="glass-panel pointer-events-none fixed z-[70] flex max-w-72 items-center gap-2 rounded-lg border border-primary/40 bg-popover px-3 py-2 text-sm shadow-popover"
            >
              <GripVertical aria-hidden="true" className="size-4 shrink-0 text-primary" />
              <span className="truncate">{activeLabel}</span>
              <ArrowRight aria-hidden="true" className="size-4 shrink-0 text-primary" />
            </div>,
            document.body,
          )
        : null}
    </Context.Provider>
  );
}

export function MappingSourceButton({
  itemID,
  className,
  ...props
}: Omit<ComponentProps<typeof Button>, 'onClick' | 'onKeyDown'> & { itemID: string }) {
  const context = useMappingBoard();
  const { register } = context;
  const ref = useCallback(
    (element: HTMLButtonElement | null) => {
      register('source', itemID, element);
    },
    [register, itemID],
  );
  const disabled = props.disabled === true || context.sourceItems.get(itemID)?.disabled === true;
  return (
    <Button
      {...props}
      ref={ref}
      type="button"
      variant="ghost"
      disabled={disabled}
      tabIndex={-1}
      className={cn(
        'h-auto w-full min-w-0 cursor-grab touch-none justify-start rounded-lg border border-transparent px-2 py-2 text-left whitespace-normal active:cursor-grabbing motion-reduce:transition-none',
        context.highlightedSource === itemID && 'border-primary/60 bg-primary/5',
        context.dragging === itemID && 'opacity-50',
        className,
      )}
      onPointerDown={(event) => {
        context.start(itemID, event);
      }}
      onKeyDown={(event) => {
        if (event.key === 'Escape' && context.cancelDrag()) {
          event.preventDefault();
          event.stopPropagation();
        }
      }}
    />
  );
}

export function MappingTargetButton({
  itemID,
  className,
  ...props
}: Omit<ComponentProps<typeof Button>, 'onClick' | 'onKeyDown'> & { itemID: string }) {
  const context = useMappingBoard();
  const { register } = context;
  const ref = useCallback(
    (element: HTMLButtonElement | null) => {
      register('target', itemID, element);
    },
    [register, itemID],
  );
  const disabled = props.disabled === true || context.targetItems.get(itemID)?.disabled === true;
  const problem = context.dragging ? context.problem(context.dragging, itemID) : null;
  const compatible = !!context.dragging && !problem && !disabled;
  return (
    <Button
      {...props}
      ref={ref}
      type="button"
      variant="ghost"
      disabled={disabled}
      tabIndex={-1}
      data-mapping-target={itemID}
      aria-disabled={disabled || (!!context.dragging && !!problem)}
      className={cn(
        'h-auto w-full min-w-0 justify-start rounded-lg border border-transparent px-2 py-2 text-left whitespace-normal motion-reduce:transition-none',
        compatible && 'border-dashed border-primary/40 bg-primary/5',
        context.hovered === itemID &&
          (compatible
            ? 'border-solid border-primary bg-primary/10 ring-2 ring-primary/20'
            : 'border-danger-border bg-danger'),
        context.locatedTarget === itemID && 'border-primary bg-primary/10 ring-2 ring-primary/20',
        className,
      )}
    />
  );
}

export function MappingLocateButton({
  side,
  itemID,
  sourceID,
  className,
  children,
  ...props
}: ComponentProps<typeof Button> & {
  side: 'source' | 'target';
  itemID: string;
  sourceID: string;
}) {
  const context = useMappingBoard();
  const items = side === 'source' ? context.sourceItems : context.targetItems;
  return (
    <Button
      {...props}
      type="button"
      variant="ghost"
      disabled={!items.has(itemID)}
      className={cn(
        'h-6 min-w-0 flex-1 shrink justify-start px-1 text-xs font-normal hover:text-primary has-[>svg]:px-1',
        className,
      )}
      onClick={() => {
        context.locate(side, itemID, sourceID);
      }}
    >
      {children}
      <LocateFixed aria-hidden="true" className="size-3 shrink-0 text-primary" />
    </Button>
  );
}
