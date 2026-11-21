import { CalendarDays } from 'lucide-react';
import { Popover } from 'radix-ui';
import { useId, useRef, useState } from 'react';

import { Button } from '@/components/ui/button';
import { CalendarPanel } from '@/components/ui/calendar-panel';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  calendarDate,
  calendarLabel,
  currentCalendarDate,
  maximumCalendarDate,
  minimumCalendarDate,
} from '@/lib/calendar';
import { showMessage } from '@/lib/messages';
import { cn } from '@/lib/utils';

export function DatePicker({
  id,
  label,
  value,
  onValueChange,
  mode = 'date',
  min = mode === 'date' ? minimumCalendarDate : minimumCalendarDate.slice(0, 7),
  max = mode === 'date' ? maximumCalendarDate : maximumCalendarDate.slice(0, 7),
  required = false,
  disabled = false,
  invalid = false,
  className,
}: {
  id?: string;
  label: string;
  value: string;
  onValueChange: (value: string) => void;
  mode?: 'date' | 'month';
  min?: string;
  max?: string;
  required?: boolean;
  disabled?: boolean;
  invalid?: boolean;
  className?: string;
}) {
  const popupID = useId();
  const inputID = useId();
  const today = currentCalendarDate();
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState(value);
  const panel = useRef<HTMLDivElement>(null);
  const valid = (next: string) => next >= min && next <= max;
  const choose = (next: string) => {
    onValueChange(next);
    setOpen(false);
  };
  const apply = () => {
    const next = draft.trim();
    if (calendarDate(next, mode) && valid(next)) {
      choose(next);
    } else {
      showMessage(
        mode === 'date' ? '请输入有效日期，格式为 YYYY-MM-DD' : '请输入有效月份，格式为 YYYY-MM',
      );
    }
  };
  return (
    <Popover.Root
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (next) {
          setDraft(value);
        }
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
          aria-expanded={open}
          aria-controls={popupID}
          aria-required={required}
          aria-invalid={invalid || undefined}
          disabled={disabled}
          className={cn(
            'w-full justify-between font-normal',
            !value && 'text-muted-foreground',
            className,
          )}
        >
          <span className="truncate">{value ? calendarLabel(value, mode) : `选择${label}`}</span>
          <CalendarDays aria-hidden="true" className="size-4 text-muted-foreground" />
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
          className="glass-panel menu-motion z-50 w-80 max-w-[calc(100vw-2rem)] space-y-3 rounded-xl border bg-popover p-3 shadow-popover"
          onOpenAutoFocus={(event) => {
            event.preventDefault();
            panel.current?.querySelector<HTMLButtonElement>('[data-date][tabindex="0"]')?.focus();
          }}
        >
          <CalendarPanel
            value={value}
            mode={mode}
            min={min}
            max={max}
            today={today}
            onValueChange={choose}
          />
          <div className="space-y-2 border-t border-border pt-3">
            <Label htmlFor={inputID}>{mode === 'date' ? '输入日期' : '输入月份'}</Label>
            <div className="flex gap-2">
              <Input
                id={inputID}
                value={draft}
                autoComplete="off"
                maxLength={mode === 'date' ? 10 : 7}
                placeholder={mode === 'date' ? 'YYYY-MM-DD' : 'YYYY-MM'}
                onChange={(event) => {
                  setDraft(event.target.value);
                }}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') {
                    event.preventDefault();
                    apply();
                  }
                }}
              />
              <Button type="button" variant="outline" onClick={apply}>
                确定
              </Button>
            </div>
          </div>
          <div className="flex justify-between gap-2">
            <Button
              type="button"
              variant="ghost"
              disabled={required || !value}
              onClick={() => {
                choose('');
              }}
            >
              清空
            </Button>
            <Button
              type="button"
              variant="ghost"
              disabled={!valid(today.slice(0, mode === 'date' ? 10 : 7))}
              onClick={() => {
                choose(today.slice(0, mode === 'date' ? 10 : 7));
              }}
            >
              {mode === 'date' ? '今天' : '本月'}
            </Button>
          </div>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}
