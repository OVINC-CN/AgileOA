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
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Drawer } from '@/components/ui/drawer';
import { Input } from '@/components/ui/input';
import { SearchSelect } from '@/components/ui/search-select';
import { useCurrentUser } from '@/features/user/api';
import {
  deleteWorkspace,
  type Workspace,
  type WorkspaceFilters,
  workspaceListOptions,
} from '@/features/workspace/api';
import { workspaceURL } from '@/features/workspace/navigation';
import { WorkspaceEditor } from '@/features/workspace/workspace-editor';
import { useApiAction, useApiMutation, useInitialLoading } from '@/lib/api-action';
import { useFilterForm } from '@/lib/filter-form';
import { formatValue } from '@/lib/format';
import { useTablePagination } from '@/lib/pagination';
import { recordTimeColumns } from '@/lib/record-time-columns';

const empty: Workspace[] = [];
const defaultFilters: WorkspaceFilters = { keyword: '', status: [] };

function DeleteWorkspace({
  workspace,
  onClose,
  onDeleted,
}: {
  workspace: Workspace;
  onClose: () => void;
  onDeleted: () => void;
}) {
  const client = useQueryClient();
  const mutation = useApiMutation({
    mutationFn: async () => {
      await deleteWorkspace(workspace.id);
      await Promise.all([
        client.invalidateQueries({ queryKey: ['management', 'workspaces'] }),
        client.invalidateQueries({ queryKey: ['management', 'audit'] }),
        client.invalidateQueries({ queryKey: ['workspace'] }),
      ]);
    },
    onSuccess: onDeleted,
  });
  return (
    <Drawer
      title="删除工作空间"
      busy={mutation.isPending}
      description={workspace.name}
      onClose={() => {
        if (!mutation.isPending) onClose();
      }}
      footer={
        <>
          <Button variant="cancel" disabled={mutation.isPending} onClick={onClose}>
            取消
          </Button>
          <Button
            loading={mutation.isPending}
            onClick={() => {
              mutation.mutate();
            }}
          >
            <Trash2 aria-hidden="true" />
            确认删除
          </Button>
        </>
      }
    >
      <div className="space-y-4 p-6 text-sm">
        <p>
          将永久删除工作空间及其中的学生、党建、月度工作等业务数据和关联记录，历史审计保留。空间仍有授权或奖项项目时不可删除。
        </p>
        {mutation.error ? <ErrorMessage>{mutation.error.message}</ErrorMessage> : null}
      </div>
    </Drawer>
  );
}

export function WorkspacesPage() {
  const { data: user } = useCurrentUser();
  const client = useQueryClient();
  const [filters, setFilters] = useState(defaultFilters);
  const { page, pageSize, setPage, setPageSize } = useTablePagination();
  const [editor, setEditor] = useState<{ workspace: Workspace | null } | null>(null);
  const [deleting, setDeleting] = useState<Workspace | null>(null);
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
  const columns: ColumnDef<Workspace>[] = [
    {
      accessorKey: 'name',
      header: '工作空间',
      cell: ({ row }) => (
        <div className="flex items-center gap-3">
          <p className="font-medium">{row.original.name}</p>
          <p className="text-xs text-muted-foreground">{formatValue(row.original.description)}</p>
        </div>
      ),
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
    {
      id: 'actions',
      header: '操作',
      cell: ({ row }) => (
        <div className="flex flex-nowrap gap-1">
          <TableActionButton
            onClick={() => {
              setEditor({ workspace: row.original });
            }}
          >
            <Pencil aria-hidden="true" />
            编辑
          </TableActionButton>
          <TableActionButton asChild>
            <Link to={workspaceURL('/system/permissions', row.original.id)}>
              <ShieldCheck aria-hidden="true" />
              权限
            </Link>
          </TableActionButton>
          <TableActionButton
            onClick={() => {
              setDeleting(row.original);
            }}
          >
            <Trash2 aria-hidden="true" />
            删除
          </TableActionButton>
        </div>
      ),
    },
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
      {deleting ? (
        <DeleteWorkspace
          workspace={deleting}
          onClose={() => {
            setDeleting(null);
          }}
          onDeleted={() => {
            setDeleting(null);
            setNotice(`已删除工作空间 ${deleting.name}`);
            if (list.data?.items.length === 1 && page > 1) setPage(page - 1);
          }}
        />
      ) : null}
    </section>
  );
}
