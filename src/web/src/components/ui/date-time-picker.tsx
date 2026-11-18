import { CalendarClock } from 'lucide-react';
import { Popover } from 'radix-ui';
import { useEffect, useId, useRef, useState } from 'react';

import { Button } from '@/components/ui/button';
import { CalendarPanel } from '@/components/ui/calendar-panel';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  calendarDate,
  calendarLabel,
  maximumCalendarDate,
  minimumCalendarDate,
} from '@/lib/calendar';
import { getConfig } from '@/lib/config';
import { dateTimeParts, utcDateTime } from '@/lib/date-time';
import { showMessage } from '@/lib/messages';
import { cn } from '@/lib/utils';

const endOfDay = { hour: '23', minute: '59' };

function TimeOptions({
  label,
  count,
  value,
  onValueChange,
}: {
  label: '小时' | '分钟';
  count: number;
  value: string;
  onValueChange: (value: string) => void;
}) {
  const list = useRef<HTMLDivElement>(null);
  const labelID = useId();
  useEffect(() => {
    // 列表打开及选值变化时仅滚动时间列，避免带动日历或外层抽屉。
    const selected = list.current?.querySelector<HTMLElement>('[aria-selected="true"]');
    if (list.current && selected) {
      list.current.scrollTop =
        selected.offsetTop - (list.current.clientHeight - selected.offsetHeight) / 2;
    }
  }, [value]);

  return (
    <div className="min-w-0 space-y-2">
      <div id={labelID} className="text-center text-xs text-muted-foreground">
        {label}
      </div>
      <div
        ref={list}
        role="listbox"
        aria-labelledby={labelID}
        className="relative h-28 space-y-1 overflow-y-auto overscroll-contain p-1 sm:h-64"
      >
        {Array.from({ length: count }, (_, index) => {
          const next = String(index).padStart(2, '0');
          return (
            <Button
              key={next}
              type="button"
              role="option"
              data-time={next}
              aria-label={`${next} ${label === '小时' ? '时' : '分'}`}
              aria-selected={value === next}
              variant={value === next ? 'default' : 'ghost'}
              size="sm"
              className="w-full px-2 font-normal tabular-nums"
              tabIndex={value === next ? 0 : -1}
              onClick={() => {
                onValueChange(next);
              }}
              onKeyDown={(event) => {
                const positions: Record<string, number> = {
                  ArrowUp: index - 1,
                  ArrowDown: index + 1,
                  Home: 0,
                  End: count - 1,
                };
                const target = positions[event.key];
                if (target === undefined) return;
                event.preventDefault();
                const choice = String(Math.max(0, Math.min(count - 1, target))).padStart(2, '0');
                list.current?.querySelector<HTMLButtonElement>(`[data-time="${choice}"]`)?.focus();
                onValueChange(choice);
              }}
            >
              {next}
            </Button>
          );
        })}
      </div>
    </div>
  );
}

export function DateTimePicker({
  id,
  label,
  value,
  disabled = false,
  required = false,
  className,
  timeZone = getConfig().displayTimeZone,
  defaultTime = endOfDay,
  onValueChange,
}: {
  id?: string;
  label: string;
  value: string | null;
  disabled?: boolean;
  required?: boolean;
  className?: string;
  timeZone?: string;
  defaultTime?: { hour: string; minute: string };
  onValueChange: (value: string | null) => void;
}) {
  const popupID = useId();
  const inputID = useId();
  const panel = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  if (disabled && open) setOpen(false);
  const selected = value ? dateTimeParts(value, timeZone) : null;
  const [draft, setDraft] = useState(selected ?? { date: '', ...defaultTime });
  const today = dateTimeParts(new Date().toISOString(), timeZone).date;
  const choose = (next: string | null) => {
    onValueChange(next);
    setOpen(false);
  };
  const apply = () => {
    // 按指定时区校验完整草稿，确认之前不触发业务查询或修改。
    const date = draft.date.trim();
    if (!calendarDate(date) || date < minimumCalendarDate || date > maximumCalendarDate) {
      showMessage('请输入有效日期，格式为 YYYY-MM-DD');
      return;
    }
    const next = utcDateTime(date, draft.hour, draft.minute, timeZone);
    if (!next) {
      showMessage('所选时间在当前时区不存在，请重新选择');
      return;
    }
    choose(next);
  };

  return (
    <Popover.Root
      open={open && !disabled}
      onOpenChange={(next) => {
        if (next) setDraft(selected ?? { date: '', ...defaultTime });
        setOpen(next);
      }}
    >
      <Popover.Trigger asChild>
        <Button
          id={id}
          type="button"
          variant="outline"
          role="combobox"
          aria-label={label}
          aria-haspopup="dialog"
          aria-expanded={open && !disabled}
          aria-controls={popupID}
          aria-required={required}
          disabled={disabled}
          className={cn(
            'w-full justify-between font-normal',
            !selected && 'text-muted-foreground',
            className,
          )}
        >
          <span className="truncate tabular-nums">
            {selected
              ? `${calendarLabel(selected.date)} ${selected.hour}:${selected.minute}`
              : `选择${label}`}
          </span>
          <CalendarClock aria-hidden="true" className="size-4 text-muted-foreground" />
        </Button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content
          ref={panel}
          id={popupID}
          role="dialog"
          aria-label={`${label}选择`}
          align="start"
          sideOffset={6}
          collisionPadding={16}
          className="glass-panel menu-motion z-50 flex max-h-[min(var(--radix-popover-content-available-height),calc(100dvh-2rem))] w-[480px] max-w-[calc(100vw-2rem)] flex-col overflow-hidden rounded-xl border bg-popover shadow-popover"
          onOpenAutoFocus={(event) => {
            event.preventDefault();
            panel.current?.querySelector<HTMLButtonElement>('[data-date][tabindex="0"]')?.focus();
          }}
        >
          <div className="min-h-0 space-y-3 overflow-y-auto overscroll-contain p-3">
            <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_8rem]">
              <CalendarPanel
                value={draft.date}
                today={today}
                onValueChange={(date) => {
                  setDraft((current) => ({ ...current, date }));
                }}
              />
              <div className="grid grid-cols-2 gap-2 border-t border-border pt-3 sm:border-t-0 sm:border-l sm:pt-0 sm:pl-3">
                <TimeOptions
                  label="小时"
                  count={24}
                  value={draft.hour}
                  onValueChange={(hour) => {
                    setDraft((current) => ({ ...current, hour }));
                  }}
                />
                <TimeOptions
                  label="分钟"
                  count={60}
                  value={draft.minute}
                  onValueChange={(minute) => {
                    setDraft((current) => ({ ...current, minute }));
                  }}
                />
              </div>
            </div>
            <div className="space-y-2 border-t border-border pt-3">
              <div className="flex items-center justify-between gap-2">
                <Label htmlFor={inputID}>输入日期</Label>
                <span className="text-xs text-muted-foreground tabular-nums" aria-live="polite">
                  {draft.hour}:{draft.minute}
                </span>
              </div>
              <Input
                id={inputID}
                value={draft.date}
                autoComplete="off"
                maxLength={10}
                placeholder="YYYY-MM-DD"
                onChange={(event) => {
                  const date = event.target.value;
                  setDraft((current) => ({ ...current, date }));
                }}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') {
                    event.preventDefault();
                    apply();
                  }
                }}
              />
            </div>
          </div>
          <div className="flex shrink-0 flex-wrap items-center justify-between gap-2 border-t border-border p-3">
            <div className="flex gap-1">
              <Button
                type="button"
                variant="ghost"
                disabled={required || !value}
                onClick={() => {
                  choose(null);
                }}
              >
                清空
              </Button>
              <Button
                type="button"
                variant="ghost"
                onClick={() => {
                  setDraft(dateTimeParts(new Date().toISOString(), timeZone));
                }}
              >
                此刻
              </Button>
            </div>
            <div className="flex gap-2">
              <Button
                type="button"
                variant="outline"
                onClick={() => {
                  setOpen(false);
                }}
              >
                取消
              </Button>
              <Button type="button" onClick={apply}>
                确定
              </Button>
            </div>
          </div>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}
