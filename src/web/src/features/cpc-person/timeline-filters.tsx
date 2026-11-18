import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Plus, Trash2 } from 'lucide-react';
import { useState } from 'react';

import { ErrorMessage } from '@/components/error-message';
import { Button } from '@/components/ui/button';
import { DatePicker } from '@/components/ui/date-picker';
import { Input } from '@/components/ui/input';
import { SearchSelect } from '@/components/ui/search-select';
import { personTimelineFilterFieldOptions } from '@/features/cpc-person/api';
import {
  isEmptyTimelineOperator,
  type TimelineFilterDraft,
  type TimelineFilterField,
  timelineFilterOperators,
} from '@/features/cpc-person/timeline-filter-model';
import { TimelineFilterPresets } from '@/features/cpc-person/timeline-filter-presets';
import { ApiError } from '@/lib/api';
import { showMessage } from '@/lib/messages';

const fieldKey = (field: TimelineFilterField) => `${String(field.node_id)}:${field.field_id}`;

function TimelineCondition({
  workspaceID,
  condition,
  index,
  onChange,
  onRemove,
}: {
  workspaceID: string;
  condition: TimelineFilterDraft;
  index: number;
  onChange: (value: TimelineFilterDraft) => void;
  onRemove: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [keyword, setKeyword] = useState('');
  const client = useQueryClient();
  const query = useQuery({
    ...personTimelineFilterFieldOptions(workspaceID, keyword),
    enabled: open,
  });
  const denied = query.error instanceof ApiError && [401, 403, 404].includes(query.error.status);
  const fields = denied ? [] : [...(query.data ?? [])];
  const { field, operator, value } = condition;
  if (!denied && field && !fields.some((item) => fieldKey(item) === fieldKey(field))) {
    fields.push(field);
  }
  const suffix = String(index + 1);
  const operators = timelineFilterOperators(field?.type ?? 'text');
  return (
    <div className="space-y-3 rounded-xl border border-border p-3">
      <div className="min-w-0 space-y-1.5">
        <span className="text-xs text-muted-foreground">时间线字段</span>
        <SearchSelect
          label={`时间线字段 ${suffix}`}
          labelTooltip
          className="w-full"
          options={fields.map((item) => ({
            value: fieldKey(item),
            label:
              condition.invalid && field && fieldKey(item) === fieldKey(field)
                ? `${item.label}（字段已失效）`
                : item.label,
          }))}
          value={field ? fieldKey(field) : ''}
          defaultValue=""
          loading={query.isFetching}
          disabled={denied}
          onOpenChange={setOpen}
          onSearchChange={setKeyword}
          onSearchReset={async () => {
            setKeyword('');
            await client.query(personTimelineFilterFieldOptions(workspaceID, ''));
          }}
          onValueChange={(key) => {
            // 失效字段仅作为当前值展示，必须选择目录中仍然可用的字段才能解除标记。
            if (key && condition.invalid && !query.data?.some((item) => fieldKey(item) === key)) {
              showMessage('该时间线字段已失效，请搜索并选择可用字段');
              return;
            }
            const selected = fields.find((item) => fieldKey(item) === key) ?? null;
            onChange({
              id: condition.id,
              field: selected,
              operator:
                selected?.type === 'text' || selected?.type === 'textarea' ? 'contains' : 'eq',
              value: '',
            });
          }}
        />
        <ErrorMessage>{query.error?.message}</ErrorMessage>
      </div>
      <div className="grid grid-cols-[minmax(0,1fr)_auto] items-end gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,2fr)_auto]">
        <div className="col-span-2 min-w-0 space-y-1.5 sm:col-span-1">
          <span className="text-xs text-muted-foreground">筛选方式</span>
          <SearchSelect
            label={`筛选方式 ${suffix}`}
            className="w-full"
            options={operators}
            value={operator}
            defaultValue={field?.type === 'text' || field?.type === 'textarea' ? 'contains' : 'eq'}
            disabled={!field}
            onValueChange={(next) => {
              onChange({
                ...condition,
                operator: next,
                value: isEmptyTimelineOperator(next) ? '' : value,
              });
            }}
          />
        </div>
        <div className="min-w-0 space-y-1.5">
          <span className="text-xs text-muted-foreground">筛选值</span>
          {isEmptyTimelineOperator(operator) ? (
            <div className="flex h-8 items-center text-sm text-muted-foreground">无需填写</div>
          ) : field?.type === 'date' ? (
            <DatePicker
              label={`筛选值 ${suffix}`}
              min="0001-01-01"
              value={value}
              onValueChange={(next) => {
                onChange({ ...condition, value: next });
              }}
            />
          ) : field?.type === 'boolean' ? (
            <SearchSelect
              label={`筛选值 ${suffix}`}
              className="w-full"
              options={[
                { value: '', label: '不限' },
                { value: 'true', label: '是' },
                { value: 'false', label: '否' },
              ]}
              value={value}
              onValueChange={(next) => {
                onChange({ ...condition, value: next });
              }}
            />
          ) : (
            <Input
              aria-label={`筛选值 ${suffix}`}
              placeholder={field?.type === 'number' ? '输入数字' : '输入文本'}
              inputMode={field?.type === 'number' ? 'decimal' : 'text'}
              maxLength={1000}
              disabled={!field}
              value={value}
              onChange={(event) => {
                onChange({ ...condition, value: event.target.value });
              }}
            />
          )}
        </div>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          aria-label={`删除时间线条件 ${suffix}`}
          onClick={onRemove}
        >
          <Trash2 aria-hidden="true" />
        </Button>
      </div>
    </div>
  );
}

export function PersonTimelineFilters({
  workspaceID,
  value,
  onChange,
}: {
  workspaceID: string;
  value: TimelineFilterDraft[];
  onChange: (value: TimelineFilterDraft[]) => void;
}) {
  return (
    <div className="space-y-3 border-t border-border pt-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <span className="text-sm font-medium">时间线字段筛选</span>
        <Button
          type="button"
          variant="outline"
          onClick={() => {
            onChange([
              ...value,
              { id: crypto.randomUUID(), field: null, operator: 'contains', value: '' },
            ]);
          }}
        >
          <Plus aria-hidden="true" />
          添加条件
        </Button>
      </div>
      <TimelineFilterPresets workspaceID={workspaceID} value={value} onChange={onChange} />
      {value.map((condition, index) => (
        <TimelineCondition
          key={condition.id}
          workspaceID={workspaceID}
          condition={condition}
          index={index}
          onChange={(next) => {
            onChange(value.map((item) => (item.id === condition.id ? next : item)));
          }}
          onRemove={() => {
            onChange(value.filter((item) => item.id !== condition.id));
          }}
        />
      ))}
      <p className="text-xs text-muted-foreground">
        多个条件需同时满足，设置后点击搜索。未填写筛选值的条件不生效，空值请使用“为空”。
      </p>
    </div>
  );
}
