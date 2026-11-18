import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useId, useState } from 'react';

import { ErrorMessage } from '@/components/error-message';
import { Label } from '@/components/ui/label';
import { SearchSelect } from '@/components/ui/search-select';
import { personTimelineTemplateOptions } from '@/features/cpc-person/timeline-api';
import { ApiError } from '@/lib/api';

interface TemplateOption {
  value: string;
  label: string;
}

export function TimelineTemplateSelect({
  workspaceID,
  purpose,
  value,
  selectedOption,
  onChange,
  disabled = false,
}: {
  workspaceID: string;
  purpose: 'create' | 'update' | 'list';
  value: number | null;
  selectedOption?: TemplateOption | null;
  onChange: (value: number | null, option: TemplateOption | null) => void;
  disabled?: boolean;
}) {
  const id = useId();
  const [open, setOpen] = useState(false);
  const [keyword, setKeyword] = useState('');
  const [selected, setSelected] = useState<TemplateOption | null>(null);
  const client = useQueryClient();
  const query = useQuery({
    ...personTimelineTemplateOptions(workspaceID, purpose, keyword),
    enabled: open && !disabled,
  });
  const denied = query.error instanceof ApiError && [401, 403, 404].includes(query.error.status);
  const options = denied ? [] : [...(query.data ?? [])];
  const choice = selectedOption ?? selected;
  if (
    !denied &&
    choice?.value === String(value) &&
    !options.some((item) => item.value === choice.value)
  ) {
    // 保留搜索选中的模板名称，收起筛选后重新展开仍能显示当前条件。
    options.push(choice);
  }
  const filtering = purpose === 'list';
  if (filtering) options.unshift({ value: '', label: '全部时间线模板' });
  return (
    <div className={filtering ? 'space-y-1.5' : 'space-y-2'}>
      {filtering ? (
        <span className="text-xs text-muted-foreground">时间线模板</span>
      ) : (
        <Label htmlFor={id}>时间线模板 *</Label>
      )}
      <SearchSelect
        id={id}
        label="时间线模板"
        className="w-full"
        options={options}
        value={value ? String(value) : ''}
        defaultValue=""
        disabled={disabled || denied}
        loading={query.isFetching}
        onOpenChange={setOpen}
        onSearchChange={setKeyword}
        onSearchReset={async () => {
          setKeyword('');
          await client.query(personTimelineTemplateOptions(workspaceID, purpose, ''));
        }}
        onValueChange={(id) => {
          const option = options.find((item) => item.value === id) ?? null;
          setSelected(option);
          onChange(id ? Number(id) : null, option);
        }}
      />
      <ErrorMessage>{query.error?.message}</ErrorMessage>
      {!filtering ? (
        <p className="text-xs text-muted-foreground">
          绑定后为此人员建立待处理节点，模板不可更换。
        </p>
      ) : null}
    </div>
  );
}
