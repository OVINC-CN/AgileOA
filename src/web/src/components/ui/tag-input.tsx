import { Plus, X } from 'lucide-react';
import { useId, useRef, useState } from 'react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Tooltip } from '@/components/ui/tooltip';
import { showMessage } from '@/lib/messages';

export function TagInput({
  id,
  label,
  values,
  onValuesChange,
  maxLength,
  maxValues,
  readOnly = false,
}: {
  id: string;
  label: string;
  values: string[];
  onValuesChange: (values: string[]) => void;
  maxLength: number;
  maxValues?: number;
  readOnly?: boolean;
}) {
  const [draft, setDraft] = useState('');
  const input = useRef<HTMLInputElement>(null);
  const messageID = useId();
  const commit = (text: string) => {
    const additions = text
      .split(/[\r\n,，、]/u)
      .map((value) => value.trim())
      .filter(Boolean);
    const next = [...new Set([...values, ...additions])];
    const failure = additions.some(
      (value) => Array.from(value).length > maxLength || /\p{Cc}/u.test(value),
    )
      ? `${label}每个值最多 ${String(maxLength)} 字符，且不能包含控制字符`
      : maxValues !== undefined && next.length > maxValues
        ? `${label}最多添加 ${String(maxValues)} 个值`
        : '';
    input.current?.setCustomValidity(failure);
    if (failure) {
      setDraft(text);
      showMessage(failure, messageID);
      return;
    }
    onValuesChange(next);
    setDraft('');
  };
  return (
    <div className="form-control flex min-h-9 min-w-0 flex-wrap items-center gap-1.5 rounded-lg border px-2 py-1 focus-within:border-ring focus-within:ring-2 focus-within:ring-ring/30 motion-reduce:transition-none">
      {values.map((value, index) => (
        <Badge
          key={`${String(index)}:${value}`}
          variant="outline"
          className="max-w-full gap-1 rounded-md border-primary/15 bg-primary/8 px-2 py-0.5 text-xs font-normal"
        >
          <Tooltip content={value} onlyOverflow>
            <span className="truncate">{value}</span>
          </Tooltip>
          {!readOnly ? (
            <Tooltip content="删除">
              <Button
                type="button"
                variant="cancel"
                size="icon-xs"
                aria-label={`删除候选值 ${value}`}
                className="-mr-1 size-4 rounded border-0 p-0 text-muted-foreground enabled:hover:text-foreground"
                onClick={() => {
                  onValuesChange(values.filter((_, position) => position !== index));
                }}
              >
                <X className="size-3" aria-hidden="true" />
              </Button>
            </Tooltip>
          ) : null}
        </Badge>
      ))}
      {!readOnly ? (
        <div className="flex min-w-28 flex-1 items-center gap-1">
          <Input
            ref={input}
            id={id}
            aria-label={label}
            value={draft}
            className="h-7 min-w-0 border-0 bg-transparent px-1 shadow-none focus-visible:ring-0"
            placeholder="输入后按回车添加"
            onChange={(event) => {
              input.current?.setCustomValidity('');
              const value = event.target.value;
              if (/[,，、]/u.test(value)) commit(value);
              else setDraft(value);
            }}
            onKeyDown={(event) => {
              if (event.key === 'Enter' && !event.nativeEvent.isComposing) {
                event.preventDefault();
                commit(draft);
              }
              if (
                event.key === 'Backspace' &&
                !draft &&
                values.length > 0 &&
                !event.nativeEvent.isComposing
              ) {
                onValuesChange(values.slice(0, -1));
              }
            }}
            onPaste={(event) => {
              event.preventDefault();
              const start = event.currentTarget.selectionStart ?? draft.length;
              const end = event.currentTarget.selectionEnd ?? draft.length;
              commit(
                draft.slice(0, start) + event.clipboardData.getData('text') + draft.slice(end),
              );
            }}
            onBlur={() => {
              if (draft.trim()) commit(draft);
            }}
            onInvalid={(event) => {
              event.preventDefault();
              showMessage(event.currentTarget.validationMessage, messageID);
            }}
          />
          <Tooltip content="添加" disabled={!draft.trim()}>
            <Button
              type="button"
              variant="ghost"
              size="icon-xs"
              aria-label={`添加${label}`}
              disabled={!draft.trim()}
              className="text-accent-foreground hover:bg-accent hover:text-accent-foreground"
              onClick={() => {
                commit(draft);
                input.current?.focus();
              }}
            >
              <Plus className="size-4" aria-hidden="true" />
            </Button>
          </Tooltip>
        </div>
      ) : null}
      {readOnly && values.length === 0 ? (
        <span className="text-xs text-muted-foreground">未设置候选值</span>
      ) : null}
    </div>
  );
}
