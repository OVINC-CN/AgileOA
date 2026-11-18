import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { Search, ShieldPlus } from 'lucide-react';
import { useState } from 'react';

import { ErrorMessage } from '@/components/error-message';
import { FilterField, FilterPanel } from '@/components/filter-panel';
import { ManagementTable } from '@/components/management-table';
import { SearchResetButton } from '@/components/search-reset-button';
import { TableActionButton } from '@/components/table-action-button';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Drawer } from '@/components/ui/drawer';
import { Input } from '@/components/ui/input';
import { type AccessUser, accessUsersOptions } from '@/features/access/api';
import { useApiAction } from '@/lib/api-action';
import { formatIdentity } from '@/lib/format';
import { useTablePagination } from '@/lib/pagination';
import { recordTimeColumns } from '@/lib/record-time-columns';

export function UserGrantPicker({
  workspaceID,
  workspaceName,
  onSelect,
  onClose,
}: {
  workspaceID: string;
  workspaceName: string;
  onSelect: (user: AccessUser) => Promise<void>;
  onClose: () => void;
}) {
  const client = useQueryClient();
  const [keyword, setKeyword] = useState('');
  const [search, setSearch] = useState('');
  const { page, pageSize, setPage, setPageSize } = useTablePagination();
  const selection = useApiAction();
  const searchAction = useApiAction();
  const users = useQuery({
    ...accessUsersOptions(workspaceID, search, page, pageSize, 'ungranted'),
    placeholderData: keepPreviousData,
  });
  const applySearch = async (value: string) => {
    setSearch(value);
    setPage(1);
    await client.query({
      ...accessUsersOptions(workspaceID, value, 1, pageSize, 'ungranted'),
      staleTime: 0,
    });
  };
  const columns: ColumnDef<AccessUser>[] = [
    {
      accessorKey: 'username',
      header: '用户',
      cell: ({ row }) => formatIdentity(row.original.display_name, row.original.username),
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
    ...recordTimeColumns<AccessUser>((record) => record),
    {
      id: 'actions',
      header: '操作',
      cell: ({ row }) => (
        <TableActionButton
          loading={selection.isPending(String(row.original.id))}
          disabled={selection.pending || searchAction.pending || users.isPlaceholderData}
          onClick={() => {
            void selection.run(String(row.original.id), () => onSelect(row.original));
          }}
        >
          <ShieldPlus aria-hidden="true" />
          选择用户
        </TableActionButton>
      ),
    },
  ];

  return (
    <Drawer
      title="添加授权"
      description={workspaceName}
      busy={selection.pending}
      onClose={onClose}
      footer={
        <Button variant="secondary" disabled={selection.pending} onClick={onClose}>
          取消
        </Button>
      }
    >
      <div className="space-y-4 p-5">
        <p className="text-sm text-muted-foreground">选择尚未授权的用户，再配置操作权限。</p>
        <FilterPanel
          onSubmit={(event) => {
            event.preventDefault();
            void searchAction.run('search', () => applySearch(keyword.trim()));
          }}
          actions={
            <>
              <SearchResetButton
                loading={searchAction.isPending('reset')}
                disabled={selection.pending || searchAction.pending || users.isPending}
                onClick={() => {
                  setKeyword('');
                  void searchAction.run('reset', () => applySearch(''));
                }}
              />
              <Button
                type="submit"
                loading={searchAction.isPending('search') || users.isPending}
                disabled={selection.pending || searchAction.pending}
              >
                <Search aria-hidden="true" />
                搜索
              </Button>
            </>
          }
        >
          <div className="sm:col-span-2 lg:col-span-4">
            <FilterField label="用户信息" htmlFor="grant-user-keyword">
              <Input
                id="grant-user-keyword"
                placeholder="用户名或显示名称"
                value={keyword}
                onChange={(event) => {
                  setKeyword(event.target.value);
                }}
              />
            </FilterField>
          </div>
        </FilterPanel>
        {users.error ? <ErrorMessage>{users.error.message}</ErrorMessage> : null}
        <Card className="gap-0 overflow-hidden border-border py-0">
          <ManagementTable
            tableID="system-permission-candidates"
            columns={columns}
            data={users.data?.items ?? []}
            total={users.data?.total ?? 0}
            page={page}
            pageSize={pageSize}
            loading={users.isPending}
            onPage={async (value) => {
              setPage(value);
              await client.query({
                ...accessUsersOptions(workspaceID, search, value, pageSize, 'ungranted'),
                staleTime: 0,
              });
            }}
            onPageSize={async (value) => {
              setPageSize(value);
              await client.query({
                ...accessUsersOptions(workspaceID, search, 1, value, 'ungranted'),
                staleTime: 0,
              });
            }}
            emptyMessage="没有符合条件的未授权用户"
          />
        </Card>
      </div>
    </Drawer>
  );
}
