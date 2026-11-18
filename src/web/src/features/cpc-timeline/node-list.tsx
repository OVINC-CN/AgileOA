import { ArrowDown, ArrowUp, GripVertical, Plus, Trash2 } from 'lucide-react';
import { Fragment, useRef, useState } from 'react';

import { Button } from '@/components/ui/button';
import { Separator } from '@/components/ui/separator';
import { moveNode, newNode, removeNode, type TimelineNode } from '@/features/cpc-timeline/model';
import { useApiAction } from '@/lib/api-action';
import { getConfig } from '@/lib/config';
import { showMessage } from '@/lib/messages';

export function NodeList({
  nodes,
  selectedID,
  onSelect,
  onChange,
  disabled,
  beforeRemove,
}: {
  nodes: TimelineNode[];
  selectedID: string;
  onSelect: (id: string) => void;
  onChange: (nodes: TimelineNode[]) => void;
  disabled: boolean;
  beforeRemove?: (node: TimelineNode) => Promise<void>;
}) {
  const list = useRef<HTMLDivElement>(null);
  const drag = useRef<{ id: string; over: string; pointerID: number } | null>(null);
  const [dragging, setDragging] = useState('');
  const [overID, setOverID] = useState('');
  const [announcement, setAnnouncement] = useState('');
  const remove = useApiAction();
  const move = (id: string, over: string) => {
    const next = moveNode(nodes, id, over);
    if (next === nodes) return;
    onChange(next);
    setAnnouncement(`已移动至第 ${String(next.findIndex((node) => node.id === id) + 1)} 个节点`);
  };
  const stop = (commit: boolean) => {
    const current = drag.current;
    if (commit && current) move(current.id, current.over);
    drag.current = null;
    setDragging('');
    setOverID('');
  };
  return (
    <div className="flex h-full min-h-0 min-w-0 flex-col gap-3">
      <div className="flex shrink-0 flex-wrap items-center justify-between gap-2">
        <h3 className="text-sm font-semibold">节点（{String(nodes.length)}）</h3>
        <Button
          type="button"
          size="sm"
          variant="outline"
          disabled={disabled || nodes.length >= getConfig().timelineMaxNodes}
          onClick={() => {
            const node = newNode();
            onChange([...nodes, node]);
            onSelect(node.id);
          }}
        >
          <Plus aria-hidden="true" />
          添加
        </Button>
      </div>
      <p className="shrink-0 text-xs leading-5 text-muted-foreground">
        拖动手柄调整顺序，也可使用上下移动按钮
      </p>
      <p role="status" aria-live="polite" className="sr-only">
        {announcement}
      </p>
      <div
        ref={list}
        className="min-h-0 flex-1 space-y-2 overflow-y-auto overscroll-contain rounded-lg p-1"
        onPointerMove={(event) => {
          const activeDrag = drag.current;
          if (!activeDrag) return;
          if (event.pointerId !== activeDrag.pointerID || disabled) return;
          const bounds = event.currentTarget.getBoundingClientRect();
          if (event.clientY < bounds.top + 36) event.currentTarget.scrollTop -= 16;
          if (event.clientY > bounds.bottom - 36) event.currentTarget.scrollTop += 16;
          const target = document
            .elementFromPoint(event.clientX, event.clientY)
            ?.closest<HTMLElement>('[data-timeline-node]');
          const id = target?.dataset['timelineNode'];
          if (id && event.currentTarget.contains(target)) {
            activeDrag.over = id;
            setOverID(id);
          }
        }}
        onPointerUp={(event) => {
          if (event.pointerId === drag.current?.pointerID) stop(true);
        }}
        onPointerCancel={(event) => {
          if (event.pointerId === drag.current?.pointerID) stop(false);
        }}
        onLostPointerCapture={(event) => {
          if (event.pointerId === drag.current?.pointerID) stop(false);
        }}
      >
        <ol aria-label="时间线节点排序" className="space-y-2">
          {nodes.map((node, index) => (
            <Fragment key={node.id}>
              {!node.group_name.trim() && nodes[index - 1]?.group_name.trim() ? (
                <li role="presentation">
                  <Separator decorative={false} className="bg-border" />
                </li>
              ) : null}
              <li
                data-timeline-node={node.id}
                className={`rounded-lg border bg-background/75 p-2 ${selectedID === node.id ? 'border-primary/70' : 'border-border'} ${dragging === node.id ? 'opacity-50' : ''} ${dragging && overID === node.id ? 'ring-2 ring-primary/40' : ''}`}
              >
                <div className="flex items-center gap-1">
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    disabled={disabled}
                    className="shrink-0 cursor-grab touch-none active:cursor-grabbing"
                    aria-label={`拖动节点 ${String(index + 1)}，方向键调整顺序`}
                    onPointerDown={(event) => {
                      if (disabled || drag.current || event.button !== 0) return;
                      event.preventDefault();
                      event.currentTarget.focus();
                      list.current?.setPointerCapture(event.pointerId);
                      drag.current = { id: node.id, over: node.id, pointerID: event.pointerId };
                      setDragging(node.id);
                      setOverID(node.id);
                      onSelect(node.id);
                    }}
                    onKeyDown={(event) => {
                      if (event.key === 'Escape' && drag.current) {
                        event.preventDefault();
                        event.stopPropagation();
                        stop(false);
                        return;
                      }
                      if (!['ArrowUp', 'ArrowDown'].includes(event.key)) return;
                      event.preventDefault();
                      const target = nodes[index + (event.key === 'ArrowUp' ? -1 : 1)];
                      if (target) move(node.id, target.id);
                    }}
                  >
                    <GripVertical aria-hidden="true" />
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    className="h-auto min-w-0 flex-1 justify-start rounded px-1 py-2 text-left text-xs wrap-anywhere whitespace-normal focus-visible:outline-primary"
                    aria-pressed={selectedID === node.id}
                    disabled={disabled}
                    onClick={() => {
                      onSelect(node.id);
                    }}
                  >
                    <span className="min-w-0 space-y-1">
                      <span className="block">
                        {String(index + 1)}. {node.name || '未命名节点'}
                      </span>
                      {node.group_name ? (
                        <span className="block text-xs font-normal text-muted-foreground">
                          分组：{node.group_name}
                        </span>
                      ) : null}
                    </span>
                  </Button>
                </div>
                <div className="flex items-center justify-end gap-1">
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    disabled={disabled || index === 0}
                    aria-label={`上移节点 ${String(index + 1)}`}
                    onClick={() => {
                      const previous = nodes[index - 1];
                      if (previous) move(node.id, previous.id);
                    }}
                  >
                    <ArrowUp aria-hidden="true" />
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    disabled={disabled || index === nodes.length - 1}
                    aria-label={`下移节点 ${String(index + 1)}`}
                    onClick={() => {
                      const next = nodes[index + 1];
                      if (next) move(node.id, next.id);
                    }}
                  >
                    <ArrowDown aria-hidden="true" />
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    disabled={disabled || nodes.length === 1}
                    loading={remove.isPending(node.id)}
                    aria-label={`删除节点 ${String(index + 1)}`}
                    onClick={() => {
                      try {
                        const next = removeNode(nodes, node.id);
                        const apply = () => {
                          onChange(next);
                          if (selectedID === node.id && next[0]) onSelect(next[0].id);
                        };
                        if (beforeRemove) {
                          void remove.run(node.id, async () => {
                            await beforeRemove(node);
                            apply();
                          });
                        } else {
                          apply();
                        }
                      } catch (cause) {
                        if (cause instanceof Error) showMessage(cause.message);
                      }
                    }}
                  >
                    <Trash2 aria-hidden="true" />
                  </Button>
                </div>
              </li>
            </Fragment>
          ))}
        </ol>
      </div>
    </div>
  );
}
