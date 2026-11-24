import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { Eye, Search } from 'lucide-react';
import { useCallback, useMemo, useState } from 'react';

import { ErrorMessage } from '@/components/error-message';
import { FilterField, FilterPanel } from '@/components/filter-panel';
import { AuditCharts } from '@/components/management-charts';
import { ManagementPageHeader } from '@/components/management-page-header';
import { ManagementTable } from '@/components/management-table';
import { RefreshButton } from '@/components/refresh-button';
import { SearchResetButton } from '@/components/search-reset-button';
import { TableActionButton } from '@/components/table-action-button';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { DateTimePicker } from '@/components/ui/date-time-picker';
import { Input } from '@/components/ui/input';
import { SearchSelect } from '@/components/ui/search-select';
import {
  auditDetailOptions,
  type AuditFilters,
  auditListOptions,
  type AuditRow,
  auditStatisticsOptions,
  defaultAuditFilters,
  excludeAuditAction,
} from '@/features/management/api';
import { AuditDetail } from '@/features/management/audit-detail';
import {
  auditIdentity,
  auditObjectLabel,
  auditWorkspaceLabel,
} from '@/features/management/audit-labels';
import { useApiAction, useInitialLoading } from '@/lib/api-action';
import { useFilterForm } from '@/lib/filter-form';
import { formatDate, formatIdentity, formatValue } from '@/lib/format';
import { useTablePagination } from '@/lib/pagination';

const emptyRows: AuditRow[] = [];
const midnight = { hour: '00', minute: '00' };
const systemOptions = [{ value: 'system', label: '系统', pinned: true }];

export function AuditPage() {
  const client = useQueryClient();
  const [filters, setFilters] = useState<AuditFilters>(defaultAuditFilters);
  const { page, pageSize, setPage, setPageSize } = useTablePagination();
  const [selected, setSelected] = useState<AuditRow | null>(null);
  const searchAction = useApiAction();
  const detailAction = useApiAction();
  const list = useQuery({
    ...auditListOptions(filters, page, pageSize),
    placeholderData: keepPreviousData,
  });
  const stats = useQuery({ ...auditStatisticsOptions(filters), placeholderData: keepPreviousData });
  const initialLoading = useInitialLoading(
    (list.isPending && !list.isFetched) || (stats.isPending && !stats.isFetched),
  );
  const actionOptions = [
    ...new Map(
      [{ value: excludeAuditAction, label: '不含审计' }, ...(list.data?.actions ?? [])].map(
        (option) => [option.value, option],
      ),
    ).values(),
  ].map((option) => ({ ...option, pinned: option.value === excludeAuditAction }));
  const applyFilters = async (values: AuditFilters) => {
    const next = {
      ...values,
      user_id: values.user_id.trim(),
      request_id: values.request_id.trim(),
    };
    // 所有查询复用同一份提交条件；编号文本不做模糊或前缀匹配。
    setPage(1);
    setFilters(next);
    await Promise.allSettled([
      client.query({ ...auditListOptions(next, 1, pageSize), staleTime: 0 }),
      client.query({ ...auditStatisticsOptions(next), staleTime: 0 }),
    ]);
  };
  const { draft, setDraft, changeFilters, hasUnappliedChanges } = useFilterForm(
    defaultAuditFilters,
    applyFilters,
    filters,
  );
  const getDetailAction = useCallback(
    (row: AuditRow, entry: 'id' | 'action') => {
      const key = `${entry}:${row.event_id}`;
      return {
        loading: detailAction.isPending(key),
        disabled: detailAction.pending,
        onClick: () => {
          void detailAction
            .run(key, () => client.query({ ...auditDetailOptions(row), staleTime: 0 }))
            .then((result) => {
              if (result.ok) setSelected(row);
            });
        },
      };
    },
    [client, detailAction],
  );
  const columns = useMemo<ColumnDef<AuditRow>[]>(
    () => [
      { accessorKey: 'id', header: 'ID' },
      {
        id: 'user',
        header: '操作用户',
        cell: ({ row }) => {
          const actor = auditIdentity(row.original.snapshot_user_info);
          return (
            <span>
              {formatIdentity(actor.name, actor.username)}
              {row.original.user_identify_type === 2 ? '（学生）' : ''}
            </span>
          );
        },
      },
      { accessorKey: 'user_id', header: '用户 ID' },
      {
        id: 'user_tenant',
        header: '用户租户',
        cell: ({ row }) =>
          auditWorkspaceLabel(row.original.user_identify_tenant_id, list.data?.user_tenants),
      },
      {
        id: 'workspace',
        header: '工作空间',
        cell: ({ row }) => auditWorkspaceLabel(row.original.workspace_id, list.data?.workspaces),
      },
      {
        id: 'action',
        header: '操作',
        cell: ({ row }) =>
          row.original.event_content ??
          list.data?.actions.find((item) => item.value === row.original.action_id)?.label ??
          row.original.action_id,
      },
      {
        id: 'instance',
        header: '对象',
        cell: ({ row }) => (
          <span>
            {auditObjectLabel(row.original.resource_type_id)} /{' '}
            {formatValue(row.original.instance_id)}
          </span>
        ),
      },
      {
        id: 'result',
        header: '操作结果',
        cell: ({ row }) => (
          <span
            className={`rounded-full border px-2 py-1 text-xs ${row.original.result_code === 0 ? 'border-success-border bg-success text-success-foreground' : 'border-danger-border bg-danger text-danger-foreground'}`}
          >
            {list.data?.result_codes.find((item) => item.value === String(row.original.result_code))
              ?.label ?? String(row.original.result_code)}
          </span>
        ),
      },
      {
        id: 'access_type',
        header: '操作来源',
        cell: ({ row }) => (row.original.access_type === 1 ? 'Web' : 'MCP'),
      },
      { accessorKey: 'request_id', header: '请求 ID' },
      {
        accessorKey: 'start_time',
        header: '开始时间',
        cell: ({ row }) => (
          <span className="text-xs text-muted-foreground tabular-nums">
            {formatDate(new Date(row.original.start_time).toISOString())}
          </span>
        ),
      },
      {
        accessorKey: 'end_time',
        header: '结束时间',
        cell: ({ row }) => (
          <span className="text-xs text-muted-foreground tabular-nums">
            {formatDate(new Date(row.original.end_time).toISOString())}
          </span>
        ),
      },
      {
        id: 'detail',
        header: '操作',
        cell: ({ row }) => (
          <TableActionButton
            aria-label={`查看${row.original.event_content ?? row.original.action_id}详情`}
            {...getDetailAction(row.original, 'action')}
          >
            <Eye aria-hidden="true" />
            详情
          </TableActionButton>
        ),
      },
    ],
    [getDetailAction, list.data],
  );
  return (
    <section className="space-y-6" aria-labelledby="audit-title">
      <ManagementPageHeader id="audit-title" title="审计日志" />
      <AuditCharts data={stats.data} loading={stats.isPending} />
      {stats.error ? <ErrorMessage>{stats.error.message}</ErrorMessage> : null}
      <FilterPanel
        hasUnappliedChanges={hasUnappliedChanges}
        onSubmit={(event) => {
          event.preventDefault();
          void searchAction.run('search', () => applyFilters(draft));
        }}
        more={
          <>
            <FilterField label="操作结果">
              <SearchSelect
                multiple
                allLabel="全部结果"
                label="操作结果"
                className="w-full"
                value={draft.result_code}
                options={list.data?.result_codes ?? [{ value: '0', label: '成功' }]}
                onValueChange={(result_code) => {
                  changeFilters({ result_code });
                }}
              />
            </FilterField>
            <FilterField label="操作来源">
              <SearchSelect
                multiple
                allLabel="全部来源"
                label="操作来源"
                className="w-full"
                value={draft.access_type}
                options={[
                  { value: '1', label: 'Web' },
                  { value: '2', label: 'MCP' },
                ]}
                onValueChange={(access_type) => {
                  changeFilters({ access_type });
                }}
              />
            </FilterField>
            <FilterField label="对象 ID" htmlFor="audit-instance-id">
              <Input
                id="audit-instance-id"
                aria-label="对象 ID"
                placeholder="完整对象编号"
                maxLength={128}
                value={draft.instance_id}
                onChange={(event) => {
                  setDraft({ ...draft, instance_id: event.target.value });
                }}
              />
            </FilterField>
            <FilterField label="请求 ID" htmlFor="audit-request-id">
              <Input
                id="audit-request-id"
                aria-label="请求 ID"
                placeholder="完整 Trace ID"
                maxLength={64}
                value={draft.request_id}
                onChange={(event) => {
                  setDraft({ ...draft, request_id: event.target.value });
                }}
              />
            </FilterField>
            <FilterField label="开始时间" htmlFor="audit-start-time">
              <DateTimePicker
                id="audit-start-time"
                label="开始时间"
                defaultTime={midnight}
                value={draft.start_time || null}
                onValueChange={(value) => {
                  changeFilters({ start_time: value ?? '' });
                }}
              />
            </FilterField>
            <FilterField label="结束时间（不含）" htmlFor="audit-end-time">
              <DateTimePicker
                id="audit-end-time"
                label="结束时间"
                defaultTime={midnight}
                value={draft.end_time || null}
                onValueChange={(value) => {
                  changeFilters({ end_time: value ?? '' });
                }}
              />
            </FilterField>
          </>
        }
        actions={
          <>
            <SearchResetButton
              loading={searchAction.isPending('reset')}
              disabled={initialLoading || searchAction.pending}
              onClick={() => {
                void searchAction.run('reset', async () => {
                  setDraft(defaultAuditFilters);
                  await applyFilters(defaultAuditFilters);
                });
              }}
            />
            <Button
              type="submit"
              loading={
                searchAction.isPending('search') || (!searchAction.pending && initialLoading)
              }
              disabled={searchAction.pending}
            >
              <Search aria-hidden="true" />
              搜索
            </Button>
          </>
        }
      >
        <FilterField label="用户租户">
          <SearchSelect
            multiple
            allLabel="全部用户租户"
            label="用户租户"
            className="w-full"
            value={draft.user_identify_tenant_id}
            options={(list.data?.user_tenants ?? systemOptions).map((option) => ({
              ...option,
              pinned: option.value === 'system',
            }))}
            onValueChange={(user_identify_tenant_id) => {
              changeFilters({ user_identify_tenant_id });
            }}
          />
        </FilterField>
        <FilterField label="用户 ID" htmlFor="audit-user-id">
          <Input
            id="audit-user-id"
            aria-label="用户 ID"
            inputMode="numeric"
            placeholder="完整用户编号"
            maxLength={20}
            value={draft.user_id}
            onChange={(event) => {
              setDraft({ ...draft, user_id: event.target.value });
            }}
          />
        </FilterField>
        <FilterField label="工作空间">
          <SearchSelect
            multiple
            allLabel="全部空间"
            label="工作空间"
            className="w-full"
            value={draft.workspace_id}
            options={(list.data?.workspaces ?? systemOptions).map((option) => ({
              ...option,
              pinned: option.value === 'system',
            }))}
            onValueChange={(workspace_id) => {
              changeFilters({ workspace_id });
            }}
          />
        </FilterField>
        <FilterField label="操作">
          <SearchSelect
            multiple
            allLabel="全部操作"
            label="操作"
            className="w-full"
            value={draft.action_id}
            options={actionOptions}
            onValueChange={(action_id) => {
              if (action_id.length > 1 && action_id.includes(excludeAuditAction)) {
                changeFilters({
                  action_id: draft.action_id.includes(excludeAuditAction)
                    ? action_id.filter((value) => value !== excludeAuditAction)
                    : [excludeAuditAction],
                });
                return;
              }
              changeFilters({ action_id });
            }}
          />
        </FilterField>
      </FilterPanel>
      <Card className="gap-0 overflow-hidden border-border py-0">
        {list.error ? <ErrorMessage>{list.error.message}</ErrorMessage> : null}
        <ManagementTable
          refreshAction={
            <RefreshButton
              onRefresh={() => client.invalidateQueries({ queryKey: ['management', 'audit'] })}
            />
          }
          tableID="system-audit"
          pinnedColumn="id"
          pinnedEndColumn="detail"
          primaryColumn={{ id: 'id', getAction: (row) => getDetailAction(row, 'id') }}
          columns={columns}
          data={list.data?.items ?? emptyRows}
          total={list.data?.total ?? 0}
          page={page}
          pageSize={pageSize}
          loading={list.isPending}
          onPage={async (next) => {
            setPage(next);
            await client.query({ ...auditListOptions(filters, next, pageSize), staleTime: 0 });
          }}
          onPageSize={async (size) => {
            setPageSize(size);
            await client.query({ ...auditListOptions(filters, 1, size), staleTime: 0 });
          }}
          emptyMessage="没有符合条件的审计记录"
        />
      </Card>
      {selected ? (
        <AuditDetail
          row={selected}
          actionName={
            list.data?.actions.find((item) => item.value === selected.action_id)?.label ?? '操作'
          }
          workspaceLabel={auditWorkspaceLabel(selected.workspace_id, list.data?.workspaces)}
          onClose={() => {
            setSelected(null);
          }}
        />
      ) : null}
    </section>
  );
}
