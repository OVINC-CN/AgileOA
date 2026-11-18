import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { Plus, RotateCw, Search, ShieldCheck } from 'lucide-react';
import { useState } from 'react';
import { useSearchParams } from 'react-router';

import { ErrorMessage } from '@/components/error-message';
import { FilterField, FilterPanel } from '@/components/filter-panel';
import { ManagementPageHeader } from '@/components/management-page-header';
import { ManagementTable } from '@/components/management-table';
import { SearchResetButton } from '@/components/search-reset-button';
import { TableActionButton } from '@/components/table-action-button';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { SearchSelect } from '@/components/ui/search-select';
import { ActionEditor } from '@/features/access/action-editor';
import {
  type AccessUser,
  accessUsersOptions,
  actionCatalogOptions,
  userActionsOptions,
} from '@/features/access/api';
import { UserGrantPicker } from '@/features/access/user-grant-picker';
import { useCurrentUser } from '@/features/user/api';
import { workspaceEnumOptions, workspaceIDSchema } from '@/features/workspace/api';
import { workspaceQueryParam } from '@/features/workspace/navigation';
import { useApiAction, useInitialLoading } from '@/lib/api-action';
import { formatIdentity } from '@/lib/format';
import { useTablePagination } from '@/lib/pagination';
import { recordTimeColumns } from '@/lib/record-time-columns';

const empty: AccessUser[] = [];

export function PermissionsPage() {
  const { data: currentUser, isPending: currentUserPending } = useCurrentUser();
  const client = useQueryClient();
  const [params, setParams] = useSearchParams();
  const rawID = params.get(workspaceQueryParam) ?? '';
  const workspaceID = workspaceIDSchema.safeParse(rawID).success ? rawID : '';
  const [keyword, setKeyword] = useState('');
  const [search, setSearch] = useState('');
  const { page, pageSize, setPage, setPageSize } = useTablePagination();
  const [editing, setEditing] = useState<{
    workspaceID: string;
    user: AccessUser;
    creating: boolean;
  } | null>(null);
  const [addingWorkspaceID, setAddingWorkspaceID] = useState<string | null>(null);
  const [notice, setNotice] = useState('');
  const searchAction = useApiAction();
  const selectorAction = useApiAction();
  const editAction = useApiAction();
  const enumRetry = useApiAction();
  const workspaces = useQuery({ ...workspaceEnumOptions, enabled: currentUser?.is_admin === true });
  const workspace = workspaces.data?.find((item) => item.value === workspaceID);
  const users = useQuery({
    ...accessUsersOptions(workspaceID, search, page, pageSize),
    enabled: currentUser?.is_admin === true && workspace !== undefined,
    // 仅在同一工作空间的查询条件和页码间保留数据，切换空间时立即隔离旧数据。
    placeholderData: (previous, query) =>
      query?.queryKey[3] === workspaceID ? previous : undefined,
  });
  const initialEnumLoading = useInitialLoading(workspaces.isPending && !workspaces.isFetched);
  const initialUsersLoading = useInitialLoading(
    users.isPending &&
      !users.isFetched &&
      workspace !== undefined &&
      !selectorAction.pending &&
      !enumRetry.pending,
  );
  const applySearch = async (next: string) => {
    setSearch(next);
    setPage(1);
    await client.query({ ...accessUsersOptions(workspaceID, next, 1, pageSize), staleTime: 0 });
  };
  const loadEditor = async (user: AccessUser) => {
    const results = await Promise.allSettled([
      client.query(actionCatalogOptions),
      client.query(userActionsOptions(workspaceID, user.id)),
    ]);
    const failure = results.find((result) => result.status === 'rejected');
    if (failure) {
      throw failure.reason instanceof Error
        ? failure.reason
        : new Error('无法加载操作权限，请重试');
    }
  };
  const columns: ColumnDef<AccessUser>[] = [
    {
      accessorKey: 'username',
      header: '用户',
      cell: ({ row }) => (
        <p className="font-medium">
          {formatIdentity(row.original.display_name, row.original.username)}
        </p>
      ),
    },
    {
      accessorKey: 'is_enabled',
      header: '账号状态',
      cell: ({ row }) => (
        <Badge variant={row.original.is_enabled ? 'secondary' : 'outline'}>
          {row.original.is_enabled ? '启用' : '停用'}
        </Badge>
      ),
    },
    { accessorKey: 'permission_count', header: '已授权操作' },
    ...recordTimeColumns<AccessUser>((record) => record),
    {
      id: 'actions',
      header: '操作',
      cell: ({ row }) => (
        <TableActionButton
          loading={editAction.isPending(`${workspaceID}/${String(row.original.id)}`)}
          disabled={editAction.pending || selectorAction.pending}
          onClick={() => {
            void editAction
              .run(`${workspaceID}/${String(row.original.id)}`, () => loadEditor(row.original))
              .then((result) => {
                if (result.ok) setEditing({ workspaceID, user: row.original, creating: false });
              });
          }}
        >
          <ShieldCheck aria-hidden="true" />
          配置权限
        </TableActionButton>
      ),
    },
  ];
  if (currentUserPending) {
    return (
      <p role="status" className="text-sm text-muted-foreground">
        正在加载权限管理…
      </p>
    );
  }
  if (!currentUser?.is_admin) return <ErrorMessage>仅系统管理员可以访问权限管理。</ErrorMessage>;
  return (
    <section className="space-y-6" aria-labelledby="permissions-title">
      <ManagementPageHeader id="permissions-title" title="权限管理">
        <SearchSelect
          label="工作空间"
          className="w-36 sm:w-44"
          value={workspaceID}
          options={(workspaces.data ?? []).map((option) => ({
            value: option.value,
            label: `${option.label}${option.is_enabled ? '' : '（停用）'}`,
          }))}
          loading={selectorAction.pending || (!enumRetry.pending && initialEnumLoading)}
          disabled={
            workspaces.isError || enumRetry.pending || editAction.pending || searchAction.pending
          }
          onValueChange={(value) => {
            void selectorAction.run('workspace', async () => {
              setParams({ [workspaceQueryParam]: value });
              setPage(1);
              setEditing(null);
              setAddingWorkspaceID(null);
              setNotice('');
              await client.query({
                ...accessUsersOptions(value, search, 1, pageSize),
                staleTime: 0,
              });
            });
          }}
        />
        {workspace && !workspaces.error ? (
          <Button
            disabled={selectorAction.pending || enumRetry.pending || editAction.pending}
            onClick={() => {
              setNotice('');
              setAddingWorkspaceID(workspaceID);
            }}
          >
            <Plus aria-hidden="true" />
            添加授权
          </Button>
        ) : null}
      </ManagementPageHeader>
      {enumRetry.pending || workspaces.error ? (
        <div className="flex items-center gap-3">
          <ErrorMessage>{workspaces.error?.message}</ErrorMessage>
          <Button
            variant="outline"
            loading={enumRetry.pending}
            onClick={() => {
              void enumRetry.run('retry', async () => {
                const result = await workspaces.refetch({ throwOnError: true });
                if (result.data?.some((item) => item.value === workspaceID)) {
                  await client.query({
                    ...accessUsersOptions(workspaceID, search, page, pageSize),
                    staleTime: 0,
                  });
                }
              });
            }}
          >
            <RotateCw aria-hidden="true" />
            重新加载
          </Button>
        </div>
      ) : null}
      {editAction.error ? <ErrorMessage>{editAction.error.message}</ErrorMessage> : null}
      {notice ? (
        <p role="status" className="text-sm wrap-anywhere text-muted-foreground">
          {notice}
        </p>
      ) : null}
      {workspace && !workspaces.error ? (
        <>
          <FilterPanel
            onSubmit={(event) => {
              event.preventDefault();
              void searchAction.run('search', () => applySearch(keyword.trim()));
            }}
            actions={
              <>
                <SearchResetButton
                  loading={searchAction.isPending('reset')}
                  disabled={
                    initialUsersLoading ||
                    searchAction.pending ||
                    selectorAction.pending ||
                    enumRetry.pending
                  }
                  onClick={() => {
                    void searchAction.run('reset', async () => {
                      setKeyword('');
                      await applySearch('');
                    });
                  }}
                />
                <Button
                  type="submit"
                  loading={
                    searchAction.isPending('search') ||
                    (!searchAction.pending && initialUsersLoading)
                  }
                  disabled={searchAction.pending || selectorAction.pending || enumRetry.pending}
                >
                  <Search aria-hidden="true" />
                  搜索
                </Button>
              </>
            }
          >
            <FilterField label="用户信息" htmlFor="permissions-page-keyword">
              <Input
                id="permissions-page-keyword"
                aria-label="搜索用户权限"
                placeholder="用户名或显示名称"
                value={keyword}
                className="w-full"
                onChange={(event) => {
                  setKeyword(event.target.value);
                }}
              />
            </FilterField>
          </FilterPanel>
          <Card className="gap-0 overflow-hidden border-border py-0">
            {users.error ? <ErrorMessage>{users.error.message}</ErrorMessage> : null}
            <ManagementTable
              tableID="system-permissions"
              columns={columns}
              data={users.data?.items ?? empty}
              total={users.data?.total ?? 0}
              page={page}
              pageSize={pageSize}
              loading={users.isPending}
              onPage={async (next) => {
                setPage(next);
                await client.query({
                  ...accessUsersOptions(workspaceID, search, next, pageSize),
                  staleTime: 0,
                });
              }}
              onPageSize={async (size) => {
                setPageSize(size);
                await client.query({
                  ...accessUsersOptions(workspaceID, search, 1, size),
                  staleTime: 0,
                });
              }}
              emptyMessage="没有符合条件的已授权用户"
            />
          </Card>
        </>
      ) : !workspaces.isPending && !workspaces.error ? (
        <p className="text-sm text-muted-foreground">请选择工作空间后配置用户权限。</p>
      ) : null}
      {addingWorkspaceID === workspaceID && workspace ? (
        <UserGrantPicker
          key={workspaceID}
          workspaceID={workspaceID}
          workspaceName={workspace.label}
          onClose={() => {
            setAddingWorkspaceID(null);
          }}
          onSelect={async (user) => {
            await loadEditor(user);
            setAddingWorkspaceID(null);
            setEditing({ workspaceID, user, creating: true });
          }}
        />
      ) : null}
      {editing?.workspaceID === workspaceID && workspace ? (
        <ActionEditor
          key={workspaceID + String(editing.user.id)}
          workspaceID={workspaceID}
          workspaceName={workspace.label}
          user={editing.user}
          creating={editing.creating}
          onClose={() => {
            setEditing(null);
          }}
          onSaved={() => {
            const latest = client.getQueryData(
              accessUsersOptions(workspaceID, search, page, pageSize).queryKey,
            );
            const lastPage = Math.max(1, Math.ceil((latest?.total ?? 0) / pageSize));
            if (page > lastPage) setPage(lastPage);
            setEditing(null);
            setNotice(
              `已${editing.creating ? '添加' : '更新'} ${editing.user.display_name} 的操作权限`,
            );
          }}
        />
      ) : null}
    </section>
  );
}
