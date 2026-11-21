import { type ColumnDef, flexRender, getCoreRowModel, useReactTable } from '@tanstack/react-table';
import { Inbox } from 'lucide-react';
import type { ReactNode } from 'react';
import { Link, type To } from 'react-router';

import { ManagementPagination } from '@/components/management-pagination';
import { TableColumnSettings } from '@/components/table-column-settings';
import { Button } from '@/components/ui/button';
import { LoadingIcon } from '@/components/ui/loading-icon';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Tooltip } from '@/components/ui/tooltip';
import { useApiAction } from '@/lib/api-action';
import { formatValue } from '@/lib/format';
import { useLoadingDelay } from '@/lib/loading';
import { type TableColumn, useTableColumns } from '@/lib/table-columns';
import { cn } from '@/lib/utils';

type PrimaryColumnAction =
  { to: To; ariaLabel?: string } | { onClick: () => void; loading?: boolean; disabled?: boolean };

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
  pinnedEndColumn?: string;
  primaryColumn?:
    { id: string; getAction: (record: T) => PrimaryColumnAction | undefined } | undefined;
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

const primaryColumnWidth = 'max-w-[min(8rem,33vw)] sm:max-w-64';

function PrimaryColumnCell({
  label,
  action,
}: {
  label: string;
  action: PrimaryColumnAction | undefined;
}) {
  const text = (
    <span data-tooltip-overflow className="min-w-0 truncate group-aria-busy:opacity-35">
      {label}
    </span>
  );
  const buttonClass = cn(primaryColumnWidth, 'px-0');

  return (
    <Tooltip content={label} onlyOverflow>
      {action === undefined ? (
        <span className={cn(primaryColumnWidth, 'block truncate')}>{label}</span>
      ) : 'to' in action ? (
        <Button asChild variant="link" size="sm" className={buttonClass}>
          <Link to={action.to} aria-label={action.ariaLabel} aria-haspopup="dialog">
            {text}
          </Link>
        </Button>
      ) : (
        <Button
          type="button"
          variant="link"
          size="sm"
          aria-haspopup="dialog"
          className={cn(
            buttonClass,
            'group relative aria-busy:disabled:opacity-100',
            '[&>svg]:absolute [&>svg]:inset-0 [&>svg]:m-auto',
          )}
          loading={action.loading ?? false}
          disabled={action.disabled}
          onClick={action.onClick}
        >
          {text}
        </Button>
      )}
    </Tooltip>
  );
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
  pinnedEndColumn,
  primaryColumn,
}: Props<T>) {
  const settings = useTableColumns(tableID, columnOptions(columns));
  const endPinnedColumns = pinnedEndColumn ? [pinnedEndColumn] : ['actions', 'operations'];
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
                    endPinnedColumns.includes(header.column.id) && 'table-pinned-end right-0',
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
                      endPinnedColumns.includes(cell.column.id) && 'table-pinned-end right-0',
                    )}
                  >
                    {primaryColumn?.id === cell.column.id ? (
                      <PrimaryColumnCell
                        label={formatValue(cell.getValue<string | number | boolean | null>())}
                        action={primaryColumn.getAction(row.original)}
                      />
                    ) : (
                      flexRender(cell.column.columnDef.cell, cell.getContext())
                    )}
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
