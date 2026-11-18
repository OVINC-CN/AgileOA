import { Plus, X } from 'lucide-react';
import { useId, useRef, useState } from 'react';

import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
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
    <div className="form-control flex min-h-11 min-w-0 flex-wrap items-center gap-2 rounded-lg border p-2 focus-within:border-ring focus-within:ring-2 focus-within:ring-ring/30 motion-reduce:transition-none">
      {values.map((value, index) => (
        <Badge
          key={`${String(index)}:${value}`}
          variant="outline"
          className="max-w-full gap-1.5 rounded-md border-primary/15 bg-primary/8 px-2 py-1 text-xs font-normal"
        >
          <span className="truncate" title={value}>
            {value}
          </span>
          {!readOnly ? (
            <button
              type="button"
              aria-label={`删除候选值 ${value}`}
              title="删除"
              className="-mr-1 shrink-0 rounded p-0.5 text-muted-foreground transition-colors outline-none hover:bg-background/80 hover:text-foreground focus-visible:outline-2 focus-visible:outline-ring"
              onClick={() => {
                onValuesChange(values.filter((_, position) => position !== index));
              }}
            >
              <X className="size-3" aria-hidden="true" />
            </button>
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
          <button
            type="button"
            aria-label={`添加${label}`}
            title="添加"
            disabled={!draft.trim()}
            className="rounded-md p-1 text-primary outline-none hover:bg-primary/10 focus-visible:outline-2 focus-visible:outline-ring disabled:opacity-30"
            onClick={() => {
              commit(draft);
              input.current?.focus();
            }}
          >
            <Plus className="size-4" aria-hidden="true" />
          </button>
        </div>
      ) : null}
      {readOnly && values.length === 0 ? (
        <span className="text-xs text-muted-foreground">未设置候选值</span>
      ) : null}
    </div>
  );
}
