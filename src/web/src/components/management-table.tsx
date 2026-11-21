import { type ColumnDef, flexRender, getCoreRowModel, useReactTable } from '@tanstack/react-table';
import { Inbox } from 'lucide-react';
import type { ReactNode } from 'react';

import { ManagementPagination } from '@/components/management-pagination';
import { TableColumnSettings } from '@/components/table-column-settings';
import { LoadingIcon } from '@/components/ui/loading-icon';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { useApiAction } from '@/lib/api-action';
import { formatValue } from '@/lib/format';
import { useLoadingDelay } from '@/lib/loading';
import { type TableColumn, useTableColumns } from '@/lib/table-columns';
import { cn } from '@/lib/utils';

interface Props<T> {
  tableID: string;
  columns: ColumnDef<T>[];
  data: T[];
  total: number;
  page: number;
  pageSize: number;
  loading: boolean;
  onPage: (page: number) => Promise<unknown>;
  onPageSize: (pageSize: number) => Promise<unknown>;
  emptyMessage?: string;
  emptyAction?: ReactNode;
  refreshAction?: ReactNode;
  title?: string;
  rowID?: (record: T) => string;
  pinnedColumn?: string;
}

function columnOptions<T>(columns: ColumnDef<T>[]): TableColumn[] {
  return columns.flatMap((column) => {
    if ('columns' in column) return columnOptions(column.columns);
    const header = typeof column.header === 'string' ? column.header : undefined;
    const accessor = 'accessorKey' in column ? String(column.accessorKey) : undefined;
    const id = column.id ?? accessor?.replaceAll('.', '_') ?? header;
    return id ? [{ id, label: header ?? id }] : [];
  });
}

export function ManagementTable<T>({
  tableID,
  columns,
  data,
  total,
  page,
  pageSize,
  loading,
  onPage,
  onPageSize,
  emptyMessage,
  emptyAction,
  refreshAction,
  title,
  rowID,
  pinnedColumn,
}: Props<T>) {
  const settings = useTableColumns(tableID, columnOptions(columns));
  const table = useReactTable({
    data,
    columns,
    defaultColumn: {
      cell: ({ getValue }) => formatValue(getValue<string | number | boolean | null>()),
    },
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    getRowId: (record, index) => rowID?.(record) ?? String(index),
    state: {
      columnVisibility: Object.fromEntries(
        settings.columns.map((column) => [column.id, settings.isVisible(column.id)]),
      ),
    },
  });
  const pagination = useApiAction();
  const visibleLoading = useLoadingDelay(loading);
  return (
    <>
      <div className="flex flex-wrap items-center justify-end gap-3 border-b border-border px-4 py-3 sm:px-5">
        {title ? (
          <div className="mr-auto flex min-w-0 items-center gap-2">
            <h2 className="text-base font-medium">{title}</h2>
            {loading ? <LoadingIcon /> : null}
          </div>
        ) : null}
        <TableColumnSettings settings={settings} />
        {refreshAction}
      </div>
      <Table aria-label={title} aria-busy={loading || pagination.pending}>
        <TableHeader className="bg-table-header">
          {table.getHeaderGroups().map((group) => (
            <TableRow key={group.id} className="border-border hover:bg-transparent">
              {group.headers.map((header) => (
                <TableHead
                  key={header.id}
                  scope="col"
                  colSpan={header.colSpan}
                  className={cn(
                    'h-8 px-4 py-1 text-xs font-medium text-muted-foreground',
                    header.column.id === pinnedColumn && 'table-pinned left-0',
                    ['actions', 'operations'].includes(header.column.id) &&
                      'table-pinned-end right-0',
                  )}
                >
                  {flexRender(header.column.columnDef.header, header.getContext())}
                </TableHead>
              ))}
            </TableRow>
          ))}
        </TableHeader>
        <TableBody>
          {table.getRowModel().rows.length ? (
            table.getRowModel().rows.map((row) => (
              <TableRow key={row.id} className="border-border-subtle hover:bg-table-hover">
                {row.getVisibleCells().map((cell) => (
                  <TableCell
                    key={cell.id}
                    className={cn(
                      'h-12 px-4 py-2',
                      cell.column.id === pinnedColumn && 'table-pinned left-0',
                      ['actions', 'operations'].includes(cell.column.id) &&
                        'table-pinned-end right-0',
                    )}
                  >
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </TableCell>
                ))}
              </TableRow>
            ))
          ) : (
            <TableRow>
              <TableCell colSpan={Math.max(settings.visibleColumns.length, 1)} className="h-30">
                {!visibleLoading && emptyMessage ? (
                  <div className="flex flex-col items-center gap-3 text-muted-foreground">
                    <Inbox className="size-7 opacity-50" aria-hidden="true" />
                    <p>{emptyMessage}</p>
                    {emptyAction}
                  </div>
                ) : null}
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
      <ManagementPagination
        total={total}
        page={page}
        pageSize={pageSize}
        loading={loading || pagination.pending}
        onPage={(value) => pagination.run('page', () => onPage(value))}
        onPageSize={(value) => pagination.run('size', () => onPageSize(value))}
      />
    </>
  );
}
