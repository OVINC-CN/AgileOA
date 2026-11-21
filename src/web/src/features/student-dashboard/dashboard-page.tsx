import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';
import { Download, RefreshCw } from 'lucide-react';
import { type ReactNode, useState } from 'react';

import { TableColumnSettings } from '@/components/table-column-settings';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
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
import { AccommodationTable } from '@/features/student-dashboard/accommodation-table';
import {
  accommodationFilterOptions,
  type AccommodationFilters,
  accommodationSummaryOptions,
  dashboardOptions,
  defaultAccommodationFilters,
  exportAccommodation,
  exportDashboard,
  type StudentDashboard,
  type StudentDashboardRow,
} from '@/features/student-dashboard/api';
import type { WorkspaceContext } from '@/features/workspace/context';
import { studentDashboardTabs as tabs } from '@/features/workspace/navigation';
import { ApiError } from '@/lib/api';
import { useApiAction } from '@/lib/api-action';
import { formatDashboardCount, formatValue } from '@/lib/format';
import { useNavigationTabs } from '@/lib/navigation-tabs';
import { saveSpreadsheetFile } from '@/lib/spreadsheet';
import { useTableColumns } from '@/lib/table-columns';
import { cn } from '@/lib/utils';

type SummaryColumn = {
  id: string;
  label: string;
} & ({ key: 'class_name' | 'total' | 'monitors' } | { key: 'major_counts'; major: string });

export function DashboardPage(props: WorkspaceContext) {
  return <DashboardWorkspace key={props.workspaceID} {...props} />;
}

function DashboardWorkspace({ workspaceID, actions, refresh: refreshWorkspace }: WorkspaceContext) {
  const canList = actions.includes('student.dashboard.list');
  const canExport = actions.includes('student.dashboard.export');
  const client = useQueryClient();
  const refresh = useApiAction();
  const download = useApiAction();
  const { selected } = useNavigationTabs(canList ? tabs : []);
  const tab = selected?.value ?? tabs[0].value;
  const [filters, setFilters] = useState(defaultAccommodationFilters);
  const query = useQuery({ ...dashboardOptions(workspaceID), enabled: canList });
  const accommodation = useQuery({
    ...accommodationSummaryOptions(workspaceID, filters),
    enabled: canList,
    placeholderData: keepPreviousData,
  });
  const filterOptions = useQuery({ ...accommodationFilterOptions(workspaceID), enabled: canList });
  const denied = [query.error, accommodation.error, filterOptions.error].some(
    (error) => error instanceof ApiError && [401, 403, 404].includes(error.status),
  );
  const data = canList && !denied ? query.data : undefined;
  const accommodationData = canList && !denied ? accommodation.data : undefined;
  const applyAccommodationFilters = async (values: AccommodationFilters) => {
    setFilters(values);
    await client.query({ ...accommodationSummaryOptions(workspaceID, values), staleTime: 0 });
  };

  if (!canList) {
    return <Card className="p-6 text-sm text-muted-foreground">暂无查看数据仪表的权限</Card>;
  }

  const refreshButton = (
    <Button
      type="button"
      size="sm"
      variant="ghost"
      loading={refresh.pending || query.isPending || accommodation.isPending}
      onClick={() => {
        void refresh.run('refresh', async () => {
          await refreshWorkspace();
          await Promise.all([
            client.query({ ...dashboardOptions(workspaceID), staleTime: 0 }),
            client.query({
              ...accommodationSummaryOptions(workspaceID, filters),
              staleTime: 0,
            }),
            client.query({ ...accommodationFilterOptions(workspaceID), staleTime: 0 }),
          ]);
        });
      }}
    >
      <RefreshCw aria-hidden="true" />
      刷新
    </Button>
  );
  return (
    <section aria-labelledby="student-dashboard-title" className="min-w-0 space-y-5">
      <header className="flex min-h-8 flex-wrap items-center justify-between gap-3">
        <h1 id="student-dashboard-title" className="sr-only">
          数据仪表
        </h1>
        <nav aria-label="面包屑" className="text-xs text-muted-foreground">
          <ol className="flex flex-wrap items-center gap-2">
            <li>学生工作</li>
            <li aria-hidden="true">/</li>
            <li aria-current={selected ? undefined : 'page'}>数据仪表</li>
            {selected ? (
              <>
                <li aria-hidden="true">/</li>
                <li aria-current="page">{selected.label}</li>
              </>
            ) : null}
          </ol>
        </nav>
        <div className="ml-auto flex flex-wrap gap-2">
          {canExport ? (
            <span className="inline-flex items-center gap-2">
              <Button
                variant="outline"
                loading={download.pending}
                disabled={tab === 'accommodation' ? !accommodationData : !data}
                onClick={() => {
                  void (async () => {
                    const result = await download.run(tab, () =>
                      tab === 'accommodation'
                        ? exportAccommodation(workspaceID, filters)
                        : exportDashboard(workspaceID),
                    );
                    if (result.ok) saveSpreadsheetFile(result.data);
                  })();
                }}
              >
                <Download aria-hidden="true" />
                导出
              </Button>
            </span>
          ) : null}
        </div>
      </header>
      <div role="region" id={`student-dashboard-panel-${tab}`} aria-label={selected?.label}>
        {tab === 'majors' ? (
          <Card className="gap-0 overflow-hidden py-0">
            <SummaryTable
              refreshAction={refreshButton}
              workspaceID={workspaceID}
              data={data}
              loading={query.isFetching}
            />
          </Card>
        ) : (
          <AccommodationTable
            refreshAction={refreshButton}
            workspaceID={workspaceID}
            data={accommodationData}
            loading={accommodation.isFetching}
            options={!denied ? filterOptions.data : undefined}
            optionsLoading={filterOptions.isFetching}
            values={filters}
            onApply={applyAccommodationFilters}
          />
        )}
      </div>
    </section>
  );
}

function SummaryTable({
  workspaceID,
  data,
  loading,
  refreshAction,
}: {
  workspaceID: string;
  data: StudentDashboard | undefined;
  loading: boolean;
  refreshAction?: ReactNode;
}) {
  const columns: SummaryColumn[] = [
    { id: 'class_name', key: 'class_name', label: '班级' },
    ...(data?.majors ?? []).map((major): SummaryColumn => ({
      id: `major:${major}`,
      key: 'major_counts',
      major,
      label: formatValue(major),
    })),
    { id: 'total', key: 'total', label: '总数' },
    { id: 'monitors', key: 'monitors', label: '班长' },
  ];
  const settings = useTableColumns(`student-dashboard-summary:${workspaceID}`, columns);

  return (
    <>
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border px-5 py-4">
        <span className="text-xs text-muted-foreground">在籍学生</span>
        <div className="flex items-center gap-3">
          <TableColumnSettings settings={settings} />
          {refreshAction}
        </div>
      </div>
      <Table pinFirstColumn aria-label="学生专业表" aria-busy={loading}>
        <TableHeader className="bg-table-header">
          <TableRow className="border-border hover:bg-transparent">
            {settings.visibleColumns.map((column) => {
              // 表头与表尾共用完整统计，字段显示偏好不改变人数口径。
              const total =
                column.key === 'major_counts'
                  ? (data?.totals.major_counts[column.major] ?? 0)
                  : column.key === 'total'
                    ? (data?.totals.total ?? 0)
                    : 0;
              return (
                <TableHead
                  key={column.id}
                  scope="col"
                  className={cn(
                    'h-8 px-5 py-1 text-xs font-medium text-muted-foreground',
                    (column.key === 'major_counts' || column.key === 'total') && 'text-center',
                  )}
                >
                  {column.label}
                  {total > 0 ? (
                    <span
                      className="mt-1 block font-normal tabular-nums"
                      aria-label={`合计${String(total)}人`}
                    >
                      （{formatDashboardCount(total)}）
                    </span>
                  ) : null}
                </TableHead>
              );
            })}
          </TableRow>
        </TableHeader>
        <TableBody>
          {data?.items.length ? (
            data.items.map((item) => (
              <TableRow key={item.class_name} className="border-border-subtle hover:bg-table-hover">
                {settings.visibleColumns.map((column) =>
                  column.key === 'class_name' ? (
                    <TableHead key={column.id} scope="row" className="h-12 px-5 py-2 font-normal">
                      {formatValue(item.class_name)}
                    </TableHead>
                  ) : (
                    <TableCell
                      key={column.id}
                      className={cn(
                        'h-12 px-5 py-2',
                        (column.key === 'major_counts' || column.key === 'total') && 'text-center',
                      )}
                    >
                      <SummaryValue column={column} item={item} />
                    </TableCell>
                  ),
                )}
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
                {data ? '暂无在籍学生数据' : '暂无可展示的汇总数据'}
              </TableCell>
            </TableRow>
          )}
        </TableBody>
        {data ? (
          <TableFooter className="border-border bg-table-header">
            <TableRow className="hover:bg-transparent">
              {settings.visibleColumns.map((column) => (
                <TableCell
                  key={column.id}
                  className={cn(
                    'h-8 px-5 py-2 font-normal',
                    (column.key === 'major_counts' || column.key === 'total') && 'text-center',
                  )}
                >
                  <SummaryValue
                    column={column}
                    item={{ ...data.totals, class_name: '合计', monitors: '' }}
                  />
                </TableCell>
              ))}
            </TableRow>
          </TableFooter>
        ) : null}
      </Table>
    </>
  );
}

function SummaryValue({ column, item }: { column: SummaryColumn; item: StudentDashboardRow }) {
  if (column.key === 'major_counts') {
    return formatDashboardCount(item.major_counts[column.major] ?? 0);
  }
  if (column.key === 'total') {
    return formatDashboardCount(item.total);
  }
  return formatValue(item[column.key]);
}
