import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { Pencil, Plus, Search, ShieldCheck, Trash2 } from 'lucide-react';
import { useState } from 'react';
import { Link } from 'react-router';

import { ErrorMessage } from '@/components/error-message';
import { FilterField, FilterPanel } from '@/components/filter-panel';
import { ManagementPageHeader } from '@/components/management-page-header';
import { ManagementTable } from '@/components/management-table';
import { RefreshButton } from '@/components/refresh-button';
import { SearchResetButton } from '@/components/search-reset-button';
import { TableActionButton } from '@/components/table-action-button';
import { TableActionsMenu } from '@/components/table-actions-menu';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { SearchSelect } from '@/components/ui/search-select';
import { Tooltip } from '@/components/ui/tooltip';
import { useCurrentUser } from '@/features/user/api';
import {
  deleteWorkspace,
  type Workspace,
  type WorkspaceFilters,
  workspaceListOptions,
} from '@/features/workspace/api';
import { workspaceURL } from '@/features/workspace/navigation';
import { WorkspaceEditor } from '@/features/workspace/workspace-editor';
import { useApiAction, useInitialLoading } from '@/lib/api-action';
import { useFilterForm } from '@/lib/filter-form';
import { formatValue } from '@/lib/format';
import { useTablePagination } from '@/lib/pagination';
import { recordTimeColumns } from '@/lib/record-time-columns';

const empty: Workspace[] = [];
const defaultFilters: WorkspaceFilters = { keyword: '', status: [] };

export function WorkspacesPage() {
  const { data: user } = useCurrentUser();
  const client = useQueryClient();
  const [filters, setFilters] = useState(defaultFilters);
  const { page, pageSize, setPage, setPageSize } = useTablePagination();
  const [editor, setEditor] = useState<{ workspace: Workspace | null } | null>(null);
  const [notice, setNotice] = useState('');
  const searchAction = useApiAction();
  const list = useQuery({
    ...workspaceListOptions(filters, page, pageSize),
    enabled: user?.is_admin === true,
    placeholderData: keepPreviousData,
  });
  const initialLoading = useInitialLoading(list.isPending && !list.isFetched);
  const applyFilters = async (values: WorkspaceFilters) => {
    const next = { ...values, keyword: values.keyword.trim() };
    setFilters(next);
    setPage(1);
    await client.query({ ...workspaceListOptions(next, 1, pageSize), staleTime: 0 });
  };
  const { draft, setDraft, changeFilters, hasUnappliedChanges } = useFilterForm(
    defaultFilters,
    applyFilters,
    filters,
  );
  const renderActions = (record: Workspace) => (
    <div className="flex flex-nowrap gap-1">
      <TableActionButton
        onClick={() => {
          setEditor({ workspace: record });
        }}
      >
        <Pencil aria-hidden="true" />
        编辑
      </TableActionButton>
      <TableActionButton asChild>
        <Link to={workspaceURL('/system/permissions', record.id)}>
          <ShieldCheck aria-hidden="true" />
          权限
        </Link>
      </TableActionButton>
      <TableActionsMenu
        label={record.name}
        actions={[
          {
            label: '删除',
            icon: Trash2,
            destructive: true,
            confirmDelete: {
              target: record.name,
              onConfirm: async () => {
                await deleteWorkspace(record.id);
                await Promise.all([
                  client.invalidateQueries({ queryKey: ['management', 'workspaces'] }),
                  client.invalidateQueries({ queryKey: ['management', 'audit'] }),
                  client.invalidateQueries({ queryKey: ['workspace'] }),
                ]);
                setNotice(`已删除工作空间 ${record.name}`);
                if (list.data?.items.length === 1 && page > 1) setPage(page - 1);
              },
            },
          },
        ]}
      />
    </div>
  );
  const columns: ColumnDef<Workspace>[] = [
    { accessorKey: 'name', header: '工作空间' },
    {
      accessorKey: 'description',
      header: '描述',
      cell: ({ row }) => {
        const description = formatValue(row.original.description);
        return (
          <Tooltip content={description} onlyOverflow>
            <span className="block max-w-36 truncate text-xs text-muted-foreground sm:max-w-64">
              {description}
            </span>
          </Tooltip>
        );
      },
    },
    {
      accessorKey: 'is_enabled',
      header: '状态',
      cell: ({ row }) => (
        <Badge variant={row.original.is_enabled ? 'secondary' : 'outline'}>
          {row.original.is_enabled ? '启用' : '停用'}
        </Badge>
      ),
    },
    ...recordTimeColumns<Workspace>((record) => record),
    { id: 'actions', header: '操作' },
  ];
  if (!user?.is_admin) return <ErrorMessage>仅系统管理员可以访问工作空间。</ErrorMessage>;
  return (
    <section className="space-y-6" aria-labelledby="workspaces-title">
      <ManagementPageHeader id="workspaces-title" title="工作空间">
        <Button
          onClick={() => {
            setEditor({ workspace: null });
          }}
        >
          <Plus aria-hidden="true" />
          创建工作空间
        </Button>
      </ManagementPageHeader>
      {notice ? (
        <p role="status" className="text-sm wrap-anywhere text-muted-foreground">
          {notice}
        </p>
      ) : null}
      <FilterPanel
        hasUnappliedChanges={hasUnappliedChanges}
        onSubmit={(event) => {
          event.preventDefault();
          void searchAction.run('search', () => applyFilters(draft));
        }}
        actions={
          <>
            <SearchResetButton
              loading={searchAction.isPending('reset')}
              disabled={initialLoading || searchAction.pending}
              onClick={() => {
                void searchAction.run('reset', async () => {
                  setDraft(defaultFilters);
                  await applyFilters(defaultFilters);
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
        <FilterField label="空间名称" htmlFor="workspaces-page-keyword">
          <Input
            id="workspaces-page-keyword"
            aria-label="搜索工作空间"
            placeholder="工作空间名称"
            className="w-full"
            value={draft.keyword}
            onChange={(event) => {
              setDraft({ ...draft, keyword: event.target.value });
            }}
          />
        </FilterField>
        <FilterField label="工作空间状态">
          <SearchSelect
            multiple
            allLabel="全部状态"
            label="工作空间状态"
            className="w-full"
            options={[
              { value: 'enabled', label: '启用' },
              { value: 'disabled', label: '停用' },
            ]}
            value={draft.status}
            onValueChange={(status) => {
              changeFilters({ status });
            }}
          />
        </FilterField>
      </FilterPanel>
      <Card className="gap-0 overflow-hidden border-border py-0">
        {list.error ? <ErrorMessage>{list.error.message}</ErrorMessage> : null}
        <ManagementTable
          refreshAction={
            <RefreshButton
              onRefresh={() => client.invalidateQueries({ queryKey: ['management', 'workspaces'] })}
            />
          }
          tableID="system-workspaces"
          pinnedColumn="name"
          primaryColumn={{
            id: 'name',
            getAction: (workspace) => ({
              onClick: () => {
                setEditor({ workspace });
              },
            }),
          }}
          rowActions={renderActions}
          rowID={(record) => record.id}
          columns={columns}
          data={list.data?.items ?? empty}
          total={list.data?.total ?? 0}
          page={page}
          pageSize={pageSize}
          loading={list.isPending}
          onPage={async (next) => {
            setPage(next);
            await client.query({
              ...workspaceListOptions(filters, next, pageSize),
              staleTime: 0,
            });
          }}
          onPageSize={async (size) => {
            setPageSize(size);
            await client.query({ ...workspaceListOptions(filters, 1, size), staleTime: 0 });
          }}
          emptyMessage="暂无工作空间"
        />
      </Card>
      {editor ? (
        <WorkspaceEditor
          workspace={editor.workspace}
          onClose={() => {
            setEditor(null);
          }}
          onSaved={(message) => {
            setEditor(null);
            setNotice(message);
          }}
        />
      ) : null}
    </section>
  );
}
