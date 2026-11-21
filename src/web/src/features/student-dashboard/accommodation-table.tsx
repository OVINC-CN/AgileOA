import type { ReactNode } from 'react';

import { type AppliedFilter, AppliedFilters } from '@/components/applied-filters';
import { TableColumnSettings } from '@/components/table-column-settings';
import { Card } from '@/components/ui/card';
import { SearchSelect } from '@/components/ui/search-select';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Table,
  TableBody,
  TableCell,
  TableFooter,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import type {
  AccommodationData,
  AccommodationFilterOptions,
  AccommodationFilters,
  AccommodationRow,
} from '@/features/student-dashboard/api';
import { useFilterForm } from '@/lib/filter-form';
import { formatDashboardCount, formatValue } from '@/lib/format';
import { useTableColumns } from '@/lib/table-columns';

const columns = [
  { id: 'accommodation_type', label: '住宿类型' },
  { id: 'campus', label: '住宿园区' },
  { id: 'building', label: '宿舍楼' },
  { id: 'room', label: '宿舍号' },
  { id: 'count', label: '人数' },
] as const;

const filters = [
  { key: 'accommodation_type', label: '住宿类型', options: 'accommodation_types' },
  { key: 'campus', label: '住宿园区', options: 'campuses' },
  { key: 'building', label: '宿舍楼', options: 'buildings' },
  { key: 'room', label: '宿舍号', options: 'rooms' },
] as const;

export function AccommodationTable({
  workspaceID,
  data,
  loading,
  options,
  optionsLoading,
  values,
  onApply,
  refreshAction,
}: {
  workspaceID: string;
  data: AccommodationData | undefined;
  loading: boolean;
  options: AccommodationFilterOptions | undefined;
  optionsLoading: boolean;
  values: AccommodationFilters;
  onApply: (values: AccommodationFilters) => Promise<unknown>;
  refreshAction?: ReactNode;
}) {
  const { draft, changeFilters, applyChanges } = useFilterForm(values, onApply, values);
  const settings = useTableColumns(`student-dashboard-accommodation:${workspaceID}`, columns);
  const appliedFilters: AppliedFilter[] = filters.flatMap((field) => {
    const selected = values[field.key];
    if (selected.length === 0) return [];
    return [
      {
        id: field.key,
        label: field.label,
        value: selected
          .map(
            (value) =>
              options?.[field.options].find((option) => option.value === value)?.label ?? value,
          )
          .join('、'),
        onRemove: () => applyChanges({ [field.key]: [] }),
      },
    ];
  });

  return (
    <section aria-label="住宿信息统计" className="space-y-4">
      <Card className="gap-0 overflow-hidden py-0">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border px-5 py-4">
          <span className="text-xs text-muted-foreground">在籍学生</span>
          <div className="flex items-center gap-3">
            <TableColumnSettings settings={settings} />
            {refreshAction}
          </div>
        </div>
        <AppliedFilters
          filters={appliedFilters}
          className="border-b border-border-subtle px-5 py-3"
        />
        <Table pinFirstColumn aria-label="住宿信息表" aria-busy={loading}>
          <TableHeader className="bg-table-header">
            <TableRow className="border-border hover:bg-transparent">
              {settings.visibleColumns.map((column) => {
                const field = filters.find((entry) => entry.key === column.id);
                const fieldOptions = field ? (options?.[field.options] ?? []) : [];
                const matchingValues = field
                  ? (data?.items ?? [])
                      .filter(
                        (item) =>
                          field.key === 'accommodation_type' ||
                          item.accommodation_type.value === 'on_campus',
                      )
                      .map((item) =>
                        field.key === 'accommodation_type'
                          ? item.accommodation_type.value
                          : item[field.key],
                      )
                  : [];
                const headerCount =
                  column.id === 'count' ? (data?.total ?? 0) : new Set(matchingValues).size;
                return (
                  <TableHead
                    key={column.id}
                    scope="col"
                    className="h-8 px-5 py-1 text-xs font-medium text-muted-foreground"
                  >
                    <div className="flex items-center gap-2">
                      <span>
                        {column.label}
                        {headerCount > 0 ? (
                          <span
                            className="font-normal"
                            aria-label={
                              column.id === 'count'
                                ? `总人数${String(headerCount)}人`
                                : `${column.label}共有${String(headerCount)}个唯一选项`
                            }
                          >
                            （{formatDashboardCount(headerCount)}）
                          </span>
                        ) : null}
                      </span>
                      {field ? (
                        <SearchSelect
                          multiple
                          appearance="table-filter"
                          label={field.label}
                          options={fieldOptions}
                          loading={optionsLoading}
                          value={draft[field.key]}
                          onValueChange={(value) => {
                            changeFilters({ [field.key]: value });
                          }}
                        />
                      ) : null}
                    </div>
                  </TableHead>
                );
              })}
            </TableRow>
          </TableHeader>
          <TableBody>
            {data?.items.length ? (
              data.items.map((item) => (
                <TableRow
                  key={JSON.stringify([
                    item.accommodation_type.value,
                    item.campus,
                    item.building,
                    item.room,
                  ])}
                  className="border-border-subtle hover:bg-table-hover"
                >
                  {settings.visibleColumns.map((column) => (
                    <TableCell key={column.id} className="h-12 px-5 py-2">
                      <AccommodationValue column={column.id} item={item} />
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : loading && !data ? (
              Array.from({ length: 4 }, (_, index) => (
                <TableRow key={index} aria-hidden="true">
                  {settings.visibleColumns.map((column) => (
                    <TableCell key={column.id} className="h-12 px-5 py-2">
                      <Skeleton className="h-4 w-16" />
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell
                  colSpan={settings.visibleColumns.length}
                  className="h-30 text-center text-muted-foreground"
                >
                  {data ? '暂无符合条件的在籍学生' : '暂无可展示的住宿数据'}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
          {data ? (
            <TableFooter className="border-border bg-table-header">
              <TableRow className="hover:bg-transparent">
                {settings.visibleColumns.map((column) => (
                  <TableCell key={column.id} className="h-8 px-5 py-2 font-normal">
                    {column.id === 'accommodation_type'
                      ? '合计'
                      : column.id === 'count'
                        ? formatDashboardCount(data.total)
                        : formatValue(null)}
                  </TableCell>
                ))}
              </TableRow>
            </TableFooter>
          ) : null}
        </Table>
      </Card>
    </section>
  );
}

function AccommodationValue({
  column,
  item,
}: {
  column: (typeof columns)[number]['id'];
  item: AccommodationRow;
}) {
  if (column === 'accommodation_type') return item.accommodation_type.label;
  if (column === 'count') return formatDashboardCount(item.count);
  return formatValue(item[column]);
}
