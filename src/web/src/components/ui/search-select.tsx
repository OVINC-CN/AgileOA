import { Check, ChevronsUpDown, Filter, Maximize2, Minimize2, Plus, RotateCcw } from 'lucide-react';
import { Popover } from 'radix-ui';
import { Fragment, useId, useMemo, useRef, useState } from 'react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Tooltip } from '@/components/ui/tooltip';
import { useApiAction } from '@/lib/api-action';
import { sortSelectOptions } from '@/lib/select-options';
import { cn } from '@/lib/utils';

const groupLabels: Record<string, string> = {
  pinned: '特殊选项',
  create: '新建',
  numeric: '0–9',
};

interface Option {
  value: string;
  label: string;
  pinned?: boolean;
}

interface Choice {
  value: string | null;
  label: string;
  initial: string;
  create?: boolean;
}

interface SharedProps {
  label: string;
  placeholder?: string;
  options: Option[];
  optionSeparator?: { beforeValue: string | null; label: string };
  loading?: boolean;
  disabled?: boolean;
  required?: boolean;
  className?: string;
  labelTooltip?: boolean;
  onOpenChange?: (open: boolean) => void;
  onSearchChange?: (keyword: string) => void;
  onSearchReset?: () => Promise<unknown>;
  id?: string;
  allowCreate?: boolean;
  maxLength?: number;
  appearance?: 'default' | 'table-filter';
}

type SearchSelectProps = SharedProps &
  (
    | {
        multiple: true;
        value: string[];
        defaultValue?: string[];
        onValueChange: (value: string[]) => void;
        allLabel?: string;
        showAllOption?: boolean;
      }
    | {
        multiple?: false;
        value: string;
        defaultValue?: string;
        onValueChange: (value: string) => void;
        allLabel?: never;
        showAllOption?: never;
      }
  );

export function SearchSelect(props: SearchSelectProps) {
  const {
    label,
    options,
    value,
    loading = false,
    disabled = false,
    required = false,
    className = '',
    labelTooltip = false,
    onOpenChange,
    onSearchChange,
    onSearchReset,
    id,
    allowCreate = false,
    maxLength,
    appearance = 'default',
  } = props;
  const listID = useId();
  const [mode, setMode] = useState<'closed' | 'popover' | 'expanded'>('closed');
  const [expandedSide, setExpandedSide] = useState<'top' | 'bottom'>('bottom');
  const [initialValue] = useState(() => value);
  const [keyword, setKeyword] = useState('');
  const [activeIndex, setActiveIndex] = useState(0);
  const [activeInitial, setActiveInitial] = useState('');
  const reset = useApiAction();
  const trigger = useRef<HTMLButtonElement>(null);
  const input = useRef<HTMLInputElement>(null);
  const list = useRef<HTMLDivElement>(null);
  const open = mode !== 'closed';
  const expanded = mode === 'expanded';
  const selectedValues = Array.isArray(value) ? value : [value];
  const defaultValue = props.defaultValue ?? (props.multiple ? [] : initialValue);
  const defaultValues = Array.isArray(defaultValue) ? defaultValue : [defaultValue];
  const atDefault =
    selectedValues.length === defaultValues.length &&
    selectedValues.every((entry) => defaultValues.includes(entry));
  const selectedLabels = selectedValues.map(
    (entry) => options.find((option) => option.value === entry)?.label ?? entry,
  );
  const allLabel = props.allLabel ?? `全部${label}`;
  const showAllOption = props.multiple && props.showAllOption !== false;
  const tableFilter = appearance === 'table-filter';
  const selectionLabel = props.multiple
    ? selectedLabels.length > 0
      ? selectedLabels.join('、')
      : (props.placeholder ?? allLabel)
    : (options.find((option) => option.value === value)?.label ??
      props.placeholder ??
      `选择${label}`);
  const ordered = useMemo(
    () =>
      sortSelectOptions<{ value: string | null; label: string; pinned?: boolean }>([
        ...(showAllOption ? [{ value: null, label: allLabel, pinned: true }] : []),
        ...options.map((option) => ({
          ...option,
          pinned: option.pinned ?? option.value === '',
        })),
      ]),
    [options, showAllOption, allLabel],
  );
  const filtered = ordered.filter(
    (option) =>
      (option.value !== null || keyword.trim() === '') &&
      option.label.toLocaleLowerCase().includes(keyword.trim().toLocaleLowerCase()),
  );
  const newValue = keyword.trim();
  const canCreate =
    allowCreate &&
    newValue !== '' &&
    !options.some(
      (option) =>
        option.label.toLocaleLowerCase() === newValue.toLocaleLowerCase() ||
        option.value === newValue,
    );
  const choices: Choice[] = canCreate
    ? [
        ...filtered,
        { value: newValue, label: `新建“${newValue}”`, initial: 'create', create: true },
      ]
    : filtered;
  const separatorIndex =
    props.optionSeparator && props.optionSeparator.beforeValue !== null
      ? ordered.findIndex((option) => option.value === props.optionSeparator?.beforeValue)
      : -1;
  const separatorBefore =
    separatorIndex < 0
      ? null
      : (choices.find(
          (choice) =>
            ordered.findIndex((option) => option.value === choice.value) >= separatorIndex,
        )?.value ?? null);
  const groups = choices.reduce<
    { initial: string; entries: { option: Choice; index: number }[] }[]
  >((result, option, index) => {
    const previous = result.at(-1);
    if (previous?.initial === option.initial) {
      previous.entries.push({ option, index });
    } else {
      result.push({ initial: option.initial, entries: [{ option, index }] });
    }
    return result;
  }, []);
  const indexGroups = groups.filter(
    (group) => group.initial !== 'create' && group.initial !== 'pinned',
  );
  const currentInitial = indexGroups.some((group) => group.initial === activeInitial)
    ? activeInitial
    : choices[0]?.initial;
  const changeOpen = (next: boolean) => {
    // 两种选择视图共用一次打开状态，切换视图时不清空搜索或中断选项查询。
    setMode(next ? 'popover' : 'closed');
    onOpenChange?.(next);
    setKeyword('');
    onSearchChange?.('');
    setActiveIndex(0);
    setActiveInitial('');
  };
  const isSelected = (next: string | null) =>
    next === null ? selectedValues.length === 0 : selectedValues.includes(next);
  const choose = (next: string | null) => {
    // 多选切换当前选项并保留弹层，全部选项清空当前条件。
    if (props.multiple) {
      props.onValueChange(
        next === null
          ? []
          : selectedValues.includes(next)
            ? selectedValues.filter((entry) => entry !== next)
            : [...selectedValues, next],
      );
      input.current?.focus();
      return;
    }
    if (next === null) return;
    props.onValueChange(next);
    setMode('closed');
    onOpenChange?.(false);
    setKeyword('');
  };
  const move = (index: number) => {
    const next = (index + choices.length) % choices.length;
    if (!Number.isFinite(next)) return;
    setActiveIndex(next);
    setActiveInitial(choices[next]?.initial ?? '');
    list.current
      ?.querySelectorAll<HTMLElement>('[role="option"]')
      [next]?.scrollIntoView({ block: 'nearest' });
  };
  const searchControls = (
    <div className="mb-1 flex shrink-0 gap-1">
      <Input
        ref={input}
        type="search"
        name={`filter-${listID}`}
        autoComplete="off"
        data-1p-ignore="true"
        spellCheck={false}
        maxLength={maxLength}
        aria-label={`搜索${label}`}
        placeholder="搜索"
        value={keyword}
        aria-controls={listID}
        aria-activedescendant={
          choices.length
            ? `${listID}-${String(Math.min(activeIndex, choices.length - 1))}`
            : undefined
        }
        onChange={(event) => {
          setKeyword(event.target.value);
          onSearchChange?.(event.target.value);
          setActiveIndex(0);
          setActiveInitial('');
          if (list.current) list.current.scrollTop = 0;
        }}
        onKeyDown={(event) => {
          if (event.nativeEvent.isComposing) return;
          if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
            event.preventDefault();
            move(activeIndex + (event.key === 'ArrowDown' ? 1 : -1));
          }
          if (event.key === 'Enter') {
            event.preventDefault();
            const option = choices[Math.min(activeIndex, choices.length - 1)];
            if (option) choose(option.value);
          }
        }}
      />
      <Tooltip content="恢复默认选项并清空搜索">
        <Button
          type="button"
          variant="secondary"
          size="icon"
          aria-label={`重置${label}`}
          loading={reset.pending}
          disabled={atDefault && keyword === ''}
          onClick={() => {
            // 重置当前字段的选值与搜索，不改动页面中的其他筛选条件。
            setKeyword('');
            onSearchChange?.('');
            if (!atDefault) {
              if (props.multiple) {
                props.onValueChange([...defaultValues]);
              } else {
                props.onValueChange(defaultValues[0] ?? '');
              }
            }
            setActiveIndex(0);
            setActiveInitial('');
            if (list.current) list.current.scrollTop = 0;
            input.current?.focus();
            if (onSearchReset) void reset.run('reset', onSearchReset);
          }}
        >
          <RotateCcw aria-hidden="true" />
        </Button>
      </Tooltip>
      <Tooltip content={expanded ? '收起选择器' : '展开选择器'}>
        <Button
          type="button"
          variant="secondary"
          size="icon"
          aria-label={`${expanded ? '收起' : '展开'}${label}选择器`}
          aria-expanded={expanded}
          aria-controls={listID}
          onClick={() => {
            // 展开时重新选择空间较大的一侧，兼容移动端键盘改变后的可视区域。
            if (!expanded && trigger.current) {
              const bounds = trigger.current.getBoundingClientRect();
              const viewport = window.visualViewport;
              const top = viewport?.offsetTop ?? 0;
              const bottom = top + (viewport?.height ?? window.innerHeight);
              setExpandedSide(bounds.top - top > bottom - bounds.bottom ? 'top' : 'bottom');
            }
            setMode(expanded ? 'popover' : 'expanded');
            input.current?.focus();
          }}
        >
          {expanded ? <Minimize2 aria-hidden="true" /> : <Maximize2 aria-hidden="true" />}
        </Button>
      </Tooltip>
    </div>
  );
  const renderOption = (option: Choice, index: number) => (
    <Tooltip key={JSON.stringify(option.value)} content={option.label} onlyOverflow={!labelTooltip}>
      <button
        type="button"
        id={`${listID}-${String(index)}`}
        role="option"
        aria-selected={isSelected(option.value)}
        className={cn(
          'flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm transition-colors outline-none hover:bg-secondary focus-visible:ring-2 focus-visible:ring-ring/30 focus-visible:ring-inset aria-selected:bg-accent aria-selected:font-medium aria-selected:text-accent-foreground motion-reduce:transition-none',
          index === activeIndex && 'bg-secondary ring-1 ring-border ring-inset',
          expanded && 'min-h-11 scroll-mt-8',
        )}
        onMouseMove={() => {
          setActiveIndex(index);
        }}
        onClick={() => {
          choose(option.value);
        }}
      >
        {option.create ? (
          <Plus aria-hidden="true" className="size-4 shrink-0" />
        ) : (
          <Check
            aria-hidden="true"
            className={cn('size-4 shrink-0', !isSelected(option.value) && 'opacity-0')}
          />
        )}
        <span data-tooltip-overflow className="min-w-0 truncate">
          {option.label}
        </span>
      </button>
    </Tooltip>
  );
  const renderSeparator = (beforeValue: string | null) =>
    props.optionSeparator && separatorBefore === beforeValue ? (
      <div
        role="separator"
        aria-label={props.optionSeparator.label}
        className="flex items-center gap-2 px-3 py-2 text-xs text-muted-foreground"
      >
        <span aria-hidden="true" className="h-px flex-1 bg-border" />
        <span>{props.optionSeparator.label}</span>
        <span aria-hidden="true" className="h-px flex-1 bg-border" />
      </div>
    ) : null;
  const renderChoice = (option: Choice, index: number) => (
    <Fragment key={option.value ?? '__all__'}>
      {option.value !== null ? renderSeparator(option.value) : null}
      {renderOption(option, index)}
    </Fragment>
  );
  const optionList = (
    <div
      ref={list}
      id={listID}
      role="listbox"
      aria-label={label}
      aria-multiselectable={props.multiple}
      className={cn(
        'min-h-0 min-w-0 overflow-y-auto overscroll-contain',
        expanded ? 'flex-1' : 'max-h-64',
      )}
      onScroll={
        expanded
          ? () => {
              // 只读取当前列表的位置，手动滚动时同步右侧索引。
              const container = list.current;
              if (!container) return;
              let initial = choices[0]?.initial ?? '';
              const top = container.getBoundingClientRect().top;
              container
                .querySelectorAll<HTMLElement>('[data-select-initial]')
                .forEach((heading) => {
                  if (heading.getBoundingClientRect().top <= top + 1) {
                    initial = heading.dataset['selectInitial'] ?? initial;
                  }
                });
              setActiveInitial(initial);
            }
          : undefined
      }
    >
      {expanded
        ? groups.map((group) => (
            <div
              key={group.initial}
              role="group"
              aria-labelledby={`${listID}-group-${group.initial}`}
              className={
                group.initial === 'pinned' ? 'mb-1 border-b border-border pb-1' : undefined
              }
            >
              <div
                id={`${listID}-group-${group.initial}`}
                data-select-initial={
                  ['create', 'pinned'].includes(group.initial) ? undefined : group.initial
                }
                className={cn(
                  'sticky top-0 z-10 flex items-center gap-3 bg-popover px-3 py-2 text-xs font-medium text-muted-foreground backdrop-blur-md',
                  group.initial === 'pinned' && 'sr-only',
                )}
              >
                <span>{groupLabels[group.initial] ?? group.initial}</span>
                <span aria-hidden="true" className="h-px flex-1 bg-border/80" />
              </div>
              {group.entries.map(({ option, index }) => renderChoice(option, index))}
            </div>
          ))
        : choices.map(renderChoice)}
      {renderSeparator(null)}
      {choices.length === 0 ? (
        <p role="status" className="px-3 py-4 text-sm text-muted-foreground">
          没有符合条件的选项
        </p>
      ) : null}
    </div>
  );
  return (
    <Popover.Root open={open} onOpenChange={changeOpen}>
      <Popover.Trigger asChild>
        <Button
          ref={trigger}
          id={id}
          type="button"
          data-1p-ignore="true"
          role="combobox"
          variant={tableFilter ? 'ghost' : 'outline'}
          size={tableFilter ? 'icon-xs' : 'default'}
          className={cn(
            tableFilter
              ? 'size-5 rounded-sm text-muted-foreground data-[active=true]:text-primary'
              : 'w-64 max-w-full min-w-0 shrink justify-between',
            className,
          )}
          data-active={tableFilter && selectedValues.length > 0}
          aria-label={label}
          aria-describedby={tableFilter ? `${listID}-selection` : undefined}
          aria-required={required || undefined}
          aria-expanded={open}
          aria-controls={listID}
          disabled={disabled}
          loading={loading}
        >
          {tableFilter ? (
            <Filter aria-hidden="true" className="size-3" />
          ) : (
            <>
              <Tooltip content={selectionLabel} onlyOverflow={!labelTooltip}>
                <span className="min-w-0 truncate">{selectionLabel}</span>
              </Tooltip>
              <ChevronsUpDown aria-hidden="true" className="size-4 text-muted-foreground" />
            </>
          )}
        </Button>
      </Popover.Trigger>
      {tableFilter ? (
        <span id={`${listID}-selection`} className="sr-only">
          {selectionLabel}
        </span>
      ) : null}
      <Popover.Portal>
        <Popover.Content
          align="end"
          side={expanded ? expandedSide : 'bottom'}
          sideOffset={6}
          collisionPadding={16}
          sticky="always"
          updatePositionStrategy="always"
          className={cn(
            'glass-panel menu-motion z-50 flex flex-col overflow-hidden rounded-xl border bg-popover p-1 shadow-popover',
            'max-h-[min(var(--radix-popover-content-available-height),calc(100dvh-2rem))]',
            'max-w-[min(var(--radix-popover-content-available-width),calc(100vw-2rem))]',
            'min-w-[min(16rem,var(--radix-popover-content-available-width),calc(100vw-2rem))]',
            expanded
              ? [
                  'w-[480px]',
                  'h-[min(32rem,var(--radix-popover-content-available-height),calc(100dvh-2rem))]',
                ]
              : tableFilter
                ? 'w-64'
                : 'w-[var(--radix-popover-trigger-width)]',
          )}
          onOpenAutoFocus={(event) => {
            event.preventDefault();
            input.current?.focus();
          }}
        >
          {searchControls}
          {expanded ? (
            <div className="flex min-h-0 flex-1">
              {optionList}
              {indexGroups.length > 0 ? (
                <nav
                  aria-label={`${label}首字母索引`}
                  className="flex min-h-0 w-10 shrink-0 items-center justify-center border-l border-border-subtle px-1"
                >
                  <div className="flex max-h-full flex-col items-center overflow-y-auto py-1">
                    {indexGroups.map((group) => (
                      <Button
                        key={group.initial}
                        type="button"
                        variant="ghost"
                        size="icon-xs"
                        aria-label={`跳转到 ${groupLabels[group.initial] ?? group.initial}`}
                        aria-controls={`${listID}-group-${group.initial}`}
                        aria-current={currentInitial === group.initial ? 'true' : undefined}
                        className="h-5 w-7 shrink-0 rounded-sm text-xs text-muted-foreground aria-current:bg-primary/10 aria-current:text-primary"
                        onClick={() => {
                          const index = choices.findIndex(
                            (option) => option.initial === group.initial,
                          );
                          setActiveIndex(index);
                          setActiveInitial(group.initial);
                          list.current
                            ?.querySelector<HTMLElement>(`[data-select-initial="${group.initial}"]`)
                            ?.scrollIntoView({ block: 'start' });
                        }}
                      >
                        {groupLabels[group.initial] ?? group.initial}
                      </Button>
                    ))}
                  </div>
                </nav>
              ) : null}
            </div>
          ) : (
            optionList
          )}
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}
