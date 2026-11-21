import type { ColumnDef } from '@tanstack/react-table';
import { Pencil, Plug, ShieldCheck, Trash2, Unplug } from 'lucide-react';
import { type ReactNode, useMemo } from 'react';

import { ManagementTable } from '@/components/management-table';
import { TableActionButton } from '@/components/table-action-button';
import { TableTimestamp } from '@/components/table-timestamp';
import type { UserRow } from '@/features/management/api';
import { recordTimeColumns } from '@/lib/record-time-columns';

export function UserTable({
  data,
  total,
  page,
  pageSize,
  loading,
  refreshAction,
  currentID,
  onPage,
  onPageSize,
  onEdit,
  onDelete,
  onRevoke,
  onAPIToken,
}: {
  data: UserRow[];
  total: number;
  page: number;
  pageSize: number;
  loading: boolean;
  refreshAction?: ReactNode;
  currentID: number;
  onPage: (page: number) => Promise<unknown>;
  onPageSize: (pageSize: number) => Promise<unknown>;
  onEdit: (user: UserRow) => void;
  onDelete: (user: UserRow) => void;
  onRevoke: (user: UserRow) => void;
  onAPIToken: (user: UserRow) => void;
}) {
  const columns = useMemo<ColumnDef<UserRow>[]>(
    () => [
      { accessorKey: 'id', header: 'ID' },
      {
        accessorKey: 'username',
        header: '用户',
        cell: ({ row }) => (
          <p className="font-medium">
            {row.original.username}
            {row.original.id === currentID ? (
              <span className="ml-2 text-xs font-normal text-muted-foreground">当前账号</span>
            ) : null}
          </p>
        ),
      },
      { accessorKey: 'display_name', header: '显示名称' },
      { accessorKey: 'email', header: '邮箱' },
      {
        accessorKey: 'password_login_enabled',
        header: '密码登录',
        cell: ({ row }) => (
          <span
            className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs ${row.original.password_login_enabled ? 'bg-success text-success-foreground' : 'bg-danger text-danger-foreground'}`}
          >
            <span
              className={`size-1.5 rounded-full ${row.original.password_login_enabled ? 'bg-success-indicator' : 'bg-danger-indicator'}`}
            />
            {row.original.password_login_enabled ? '允许' : '禁止'}
          </span>
        ),
      },
      {
        accessorKey: 'is_admin',
        header: '账号类型',
        cell: ({ row }) => (
          <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
            {row.original.is_admin ? (
              <ShieldCheck className="size-3.5 text-primary" aria-hidden="true" />
            ) : null}
            {row.original.is_admin ? '系统管理员' : '普通用户'}
          </span>
        ),
      },
      {
        accessorKey: 'is_enabled',
        header: '状态',
        cell: ({ row }) => (
          <span
            className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs ${row.original.is_enabled ? 'bg-success text-success-foreground' : 'bg-secondary text-muted-foreground'}`}
          >
            <span
              className={`size-1.5 rounded-full ${row.original.is_enabled ? 'bg-success-indicator' : 'bg-muted-foreground'}`}
            />
            {row.original.is_enabled ? '启用' : '停用'}
          </span>
        ),
      },
      {
        accessorKey: 'last_login_at',
        header: '最近登录',
        cell: ({ row }) => <TableTimestamp value={row.original.last_login_at} />,
      },
      ...recordTimeColumns<UserRow>((record) => record),
      {
        id: 'actions',
        header: '操作',
        cell: ({ row }) => (
          <div className="flex flex-nowrap gap-1">
            <TableActionButton
              onClick={() => {
                onEdit(row.original);
              }}
            >
              <Pencil aria-hidden="true" />
              编辑
            </TableActionButton>
            <TableActionButton
              aria-label={`配置 ${row.original.username} 的 MCP`}
              onClick={() => {
                onAPIToken(row.original);
              }}
            >
              <Plug aria-hidden="true" />
              MCP
            </TableActionButton>
            <TableActionButton
              aria-label={`撤销 ${row.original.username} 的会话`}
              disabled={row.original.id === currentID}
              onClick={() => {
                onRevoke(row.original);
              }}
            >
              <Unplug aria-hidden="true" />
              撤销会话
            </TableActionButton>
            <TableActionButton
              aria-label={`删除用户 ${row.original.username}`}
              disabled={row.original.id === currentID}
              onClick={() => {
                onDelete(row.original);
              }}
            >
              <Trash2 aria-hidden="true" />
              删除
            </TableActionButton>
          </div>
        ),
      },
    ],
    [currentID, onEdit, onDelete, onRevoke, onAPIToken],
  );
  return (
    <ManagementTable
      refreshAction={refreshAction}
      tableID="system-users"
      columns={columns}
      data={data}
      total={total}
      page={page}
      pageSize={pageSize}
      loading={loading}
      onPage={onPage}
      onPageSize={onPageSize}
      emptyMessage="没有符合条件的用户"
    />
  );
}
