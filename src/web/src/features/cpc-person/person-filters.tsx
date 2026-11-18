import { useQuery } from '@tanstack/react-query';
import { Search } from 'lucide-react';
import { type SetStateAction, useState } from 'react';

import { ErrorMessage } from '@/components/error-message';
import { FilterPanel } from '@/components/filter-panel';
import { SearchResetButton } from '@/components/search-reset-button';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { SearchSelect } from '@/components/ui/search-select';
import {
  defaultFilters,
  defaultStatus,
  filterFields,
  nodeStatuses,
  personBranchOptions,
  type PersonFilters,
  personOptions,
  stages,
  timelineStatuses,
} from '@/features/cpc-person/api';
import { PersonTimelineFilters } from '@/features/cpc-person/timeline-filters';
import { TimelineTemplateSelect } from '@/features/cpc-person/timeline-template-select';
import { ApiError } from '@/lib/api';
import { useApiAction } from '@/lib/api-action';

const commonFilterKeys = new Set(['stage', 'party_branch_id', 'timeline_status']);

function Filter({
  workspaceID,
  field,
  value,
  onChange,
}: {
  workspaceID: string;
  field: (typeof filterFields)[number];
  value: string[];
  onChange: (value: string[]) => void;
}) {
  const [open, setOpen] = useState(false);
  const query = useQuery({
    ...(field.key === 'party_branch_id'
      ? personBranchOptions(workspaceID)
      : personOptions(workspaceID, field.key)),
    enabled: open && field.key !== 'stage' && field.key !== 'timeline_status',
  });
  const denied = query.error instanceof ApiError && [401, 403, 404].includes(query.error.status);
  const options = denied ? [] : [...(query.data ?? [])];
  if (
    !denied &&
    field.key === 'status' &&
    !options.some(({ value }) => value === defaultStatus.value)
  ) {
    options.unshift(defaultStatus);
  }
  return (
    <div className="space-y-1.5">
      <span className="text-xs text-muted-foreground">{field.label}</span>
      <SearchSelect
        multiple
        label={field.label}
        className="w-full"
        options={
          field.key === 'stage'
            ? [...stages]
            : field.key === 'timeline_status'
              ? [...timelineStatuses]
              : options
        }
        value={value}
        defaultValue={defaultFilters[field.key]}
        onValueChange={onChange}
        onOpenChange={setOpen}
        loading={open && query.isFetching}
      />
      {query.error ? <ErrorMessage>{query.error.message}</ErrorMessage> : null}
    </div>
  );
}

export function PersonFilterPanel({
  workspaceID,
  draft,
  setDraft,
  changeFilters,
  onApply,
  nodeView = false,
}: {
  workspaceID: string;
  draft: PersonFilters;
  setDraft: (values: SetStateAction<PersonFilters>) => void;
  changeFilters: (changes: Partial<PersonFilters>) => void;
  onApply: (values: PersonFilters) => Promise<unknown>;
  nodeView?: boolean;
}) {
  const search = useApiAction();
  return (
    <FilterPanel
      onSubmit={(event) => {
        event.preventDefault();
        void search.run('search', () => onApply(draft));
      }}
      more={
        <>
          {filterFields
            .filter((field) => !commonFilterKeys.has(field.key))
            .map((field) => (
              <Filter
                key={field.key}
                workspaceID={workspaceID}
                field={field}
                value={draft[field.key]}
                onChange={(value) => {
                  changeFilters({ [field.key]: value });
                }}
              />
            ))}
          {!nodeView ? (
            <TimelineTemplateSelect
              workspaceID={workspaceID}
              purpose="list"
              value={draft.timeline_template ? Number(draft.timeline_template.value) : null}
              selectedOption={draft.timeline_template}
              onChange={(_, option) => {
                changeFilters({ timeline_template: option?.value ? option : null });
              }}
            />
          ) : null}
          <div className="col-span-full">
            <PersonTimelineFilters
              workspaceID={workspaceID}
              value={draft.timeline_filters}
              onChange={(value) => {
                setDraft((current) => ({ ...current, timeline_filters: value }));
              }}
            />
          </div>
        </>
      }
      actions={
        <>
          <SearchResetButton
            disabled={search.pending}
            loading={search.isPending('reset')}
            onClick={() => {
              void search.run('reset', async () => {
                setDraft(defaultFilters);
                await onApply(defaultFilters);
              });
            }}
          />
          <Button type="submit" disabled={search.pending} loading={search.isPending('search')}>
            <Search aria-hidden="true" />
            搜索
          </Button>
        </>
      }
    >
      <label htmlFor="cpc-person-keyword" className="space-y-1.5">
        <span className="text-xs text-muted-foreground">学号或姓名</span>
        <Input
          id="cpc-person-keyword"
          name="cpc-person-filter-keyword"
          type="search"
          autoComplete="off"
          data-1p-ignore="true"
          aria-label="学号或姓名"
          placeholder="搜索"
          maxLength={200}
          value={draft.keyword}
          onChange={(event) => {
            setDraft((current) => ({ ...current, keyword: event.target.value }));
          }}
        />
      </label>
      {filterFields
        .filter((field) => commonFilterKeys.has(field.key))
        .map((field) =>
          nodeView && field.key === 'timeline_status' ? (
            <div key={field.key} className="space-y-1.5">
              <span className="text-xs text-muted-foreground">节点状态</span>
              <SearchSelect
                multiple
                label="节点状态"
                className="w-full"
                options={[...nodeStatuses]}
                value={draft.node_status}
                defaultValue={defaultFilters.node_status}
                onValueChange={(value) => {
                  changeFilters({ node_status: value });
                }}
              />
            </div>
          ) : (
            <Filter
              key={field.key}
              workspaceID={workspaceID}
              field={field}
              value={draft[field.key]}
              onChange={(value) => {
                changeFilters({ [field.key]: value });
              }}
            />
          ),
        )}
    </FilterPanel>
  );
}
