import { ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';

import { Button } from '@/components/ui/button';
import {
  calendarDate,
  calendarKey,
  calendarLabel,
  currentCalendarDate,
  maximumCalendarDate,
  minimumCalendarDate,
  shiftCalendarMonth,
} from '@/lib/calendar';
import { cn } from '@/lib/utils';

export function CalendarPanel({
  value,
  onValueChange,
  mode = 'date',
  min = mode === 'date' ? minimumCalendarDate : minimumCalendarDate.slice(0, 7),
  max = mode === 'date' ? maximumCalendarDate : maximumCalendarDate.slice(0, 7),
  today = currentCalendarDate(),
}: {
  value: string;
  onValueChange: (value: string) => void;
  mode?: 'date' | 'month';
  min?: string;
  max?: string;
  today?: string;
}) {
  const selected = calendarDate(value, mode) ? value : today.slice(0, mode === 'date' ? 10 : 7);
  const initial = selected < min ? min : selected > max ? max : selected;
  const [view, setView] = useState({ value, month: initial.slice(0, 7), focused: initial });
  // 选值变化时同步日历视图，手工输入和快捷选择共用相同规则。
  const month = view.value === value ? view.month : initial.slice(0, 7);
  const focused = view.value === value ? view.focused : initial;
  const panel = useRef<HTMLDivElement>(null);
  const keyboardFocus = useRef(false);
  const year = Number(month.slice(0, 4));
  const first = calendarDate(month, 'month');
  const leading = first ? (first.getUTCDay() + 6) % 7 : 0;
  const days = new Date(Date.UTC(year, Number(month.slice(5, 7)), 0)).getUTCDate();
  const firstAvailable = mode === 'date' ? `${month}-01` : `${String(year)}-01`;
  const firstEnabled = firstAvailable < min ? min : firstAvailable;
  const valid = (next: string) => next >= min && next <= max;
  const showMonth = (next: string) => {
    setView({ value, month: next, focused });
  };
  const moveMonth = (amount: number) => {
    const next = shiftCalendarMonth(month, amount);
    return next >= min.slice(0, 7) && next <= max.slice(0, 7) ? next : month;
  };
  const focus = (next: string) => {
    if (!valid(next)) return;
    keyboardFocus.current = true;
    setView({ value, month: next.slice(0, 7), focused: next });
  };
  useEffect(() => {
    // 日历内的方向键移动焦点，跨月后等待新日期渲染。
    if (keyboardFocus.current) {
      panel.current?.querySelector<HTMLButtonElement>(`[data-date="${focused}"]`)?.focus();
      keyboardFocus.current = false;
    }
  }, [focused, month]);

  return (
    <div ref={panel} className="min-w-0 space-y-3">
      <div className="flex items-center justify-between gap-1">
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          aria-label="上一年"
          disabled={moveMonth(-12) === month}
          onClick={() => {
            showMonth(moveMonth(-12));
          }}
        >
          <ChevronsLeft aria-hidden="true" />
        </Button>
        {mode === 'date' ? (
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            aria-label="上个月"
            disabled={moveMonth(-1) === month}
            onClick={() => {
              showMonth(moveMonth(-1));
            }}
          >
            <ChevronLeft aria-hidden="true" />
          </Button>
        ) : null}
        <span aria-live="polite" className="flex-1 text-center text-sm font-medium">
          {mode === 'date' ? calendarLabel(month, 'month') : `${String(year)}年`}
        </span>
        {mode === 'date' ? (
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            aria-label="下个月"
            disabled={moveMonth(1) === month}
            onClick={() => {
              showMonth(moveMonth(1));
            }}
          >
            <ChevronRight aria-hidden="true" />
          </Button>
        ) : null}
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          aria-label="下一年"
          disabled={moveMonth(12) === month}
          onClick={() => {
            showMonth(moveMonth(12));
          }}
        >
          <ChevronsRight aria-hidden="true" />
        </Button>
      </div>
      {mode === 'date' ? (
        <div
          className="grid grid-cols-7 text-center text-xs text-muted-foreground"
          aria-hidden="true"
        >
          {['一', '二', '三', '四', '五', '六', '日'].map((day) => (
            <span key={day} className="py-1">
              {day}
            </span>
          ))}
        </div>
      ) : null}
      <div
        role="group"
        aria-label={mode === 'date' ? '选择日期' : '选择月份'}
        className={cn('grid gap-1', mode === 'date' ? 'grid-cols-7' : 'grid-cols-3')}
      >
        {mode === 'date'
          ? Array.from({ length: leading }, (_, index) => <span key={`blank-${String(index)}`} />)
          : null}
        {Array.from({ length: mode === 'date' ? days : 12 }, (_, index) => {
          const next =
            mode === 'date'
              ? `${month}-${String(index + 1).padStart(2, '0')}`
              : `${String(year)}-${String(index + 1).padStart(2, '0')}`;
          const outside =
            mode === 'date' ? focused.slice(0, 7) !== month : focused.slice(0, 4) !== String(year);
          const active = focused === next || (outside && next === firstEnabled);
          return (
            <Button
              key={next}
              type="button"
              data-date={next}
              variant={value === next ? 'default' : 'ghost'}
              className={cn(
                'w-full px-0 text-xs',
                value !== next && next === today && 'ring-1 ring-input',
              )}
              aria-label={calendarLabel(next, mode)}
              aria-pressed={value === next}
              disabled={!valid(next)}
              tabIndex={active ? 0 : -1}
              onFocus={() => {
                setView({ value, month, focused: next });
              }}
              onClick={() => {
                onValueChange(next);
              }}
              onKeyDown={(event) => {
                const offsets: Record<string, number> = {
                  ArrowLeft: -1,
                  ArrowRight: 1,
                  ArrowUp: mode === 'date' ? -7 : -3,
                  ArrowDown: mode === 'date' ? 7 : 3,
                };
                const offset = offsets[event.key];
                if (offset !== undefined) {
                  event.preventDefault();
                  if (mode === 'month') {
                    focus(shiftCalendarMonth(next, offset));
                  } else {
                    const date = calendarDate(next);
                    if (date) {
                      date.setUTCDate(date.getUTCDate() + offset);
                      focus(calendarKey(date));
                    }
                  }
                }
              }}
            >
              {mode === 'date' ? String(index + 1) : `${String(index + 1)}月`}
            </Button>
          );
        })}
      </div>
    </div>
  );
}
