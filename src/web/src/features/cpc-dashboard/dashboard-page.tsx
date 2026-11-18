import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Download, RefreshCw } from 'lucide-react';

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
import { type CPCDashboard, dashboardOptions, exportDashboard } from '@/features/cpc-dashboard/api';
import type { WorkspaceContext } from '@/features/workspace/context';
import { cpcDashboardTabs as tabs } from '@/features/workspace/navigation';
import { ApiError } from '@/lib/api';
import { useApiAction } from '@/lib/api-action';
import { formatDashboardCount, formatValue } from '@/lib/format';
import { useNavigationTabs } from '@/lib/navigation-tabs';
import { saveSpreadsheetFile } from '@/lib/spreadsheet';
import { useTableColumns } from '@/lib/table-columns';

const columns = [
  { key: 'party_branch_name', label: '支部名称' },
  { key: 'full_members', label: '正式党员' },
  { key: 'probationary_members', label: '预备党员' },
  { key: 'total', label: '合计' },
  { key: 'secretaries', label: '支部书记' },
  { key: 'organization_members', label: '组织委员' },
  { key: 'publicity_members', label: '宣传委员' },
  { key: 'discipline_members', label: '纪律委员' },
  { key: 'advisors', label: '导师姓名' },
] as const;

export function DashboardPage(props: WorkspaceContext) {
  return <DashboardWorkspace key={props.workspaceID} {...props} />;
}

function DashboardWorkspace({ workspaceID, actions, refresh: refreshWorkspace }: WorkspaceContext) {
  const canList = actions.includes('cpc.dashboard.list');
  const canExport = actions.includes('cpc.dashboard.export');
  const { selected } = useNavigationTabs(canList ? tabs : []);
  const client = useQueryClient();
  const refresh = useApiAction();
  const download = useApiAction();
  const query = useQuery({ ...dashboardOptions(workspaceID), enabled: canList });
  const denied = query.error instanceof ApiError && [401, 403, 404].includes(query.error.status);
  const data = canList && !denied ? query.data : undefined;

  if (!canList) {
    return <Card className="p-6 text-sm text-muted-foreground">暂无查看数据仪表的权限</Card>;
  }

  return (
    <section aria-labelledby="cpc-dashboard-title" className="min-w-0 space-y-5">
      <header className="flex min-h-8 flex-wrap items-center justify-between gap-3">
        <h1 id="cpc-dashboard-title" className="sr-only">
          数据仪表
        </h1>
        <nav aria-label="面包屑" className="text-xs text-muted-foreground">
          <ol className="flex flex-wrap items-center gap-2">
            <li>党建工作</li>
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
                disabled={!data}
                onClick={() => {
                  void (async () => {
                    const result = await download.run('branches', () =>
                      exportDashboard(workspaceID),
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
          <Button
            variant="outline"
            loading={refresh.pending || query.isPending}
            onClick={() => {
              void refresh.run('refresh', async () => {
                await refreshWorkspace();
                await client.query({ ...dashboardOptions(workspaceID), staleTime: 0 });
              });
            }}
          >
            <RefreshCw aria-hidden="true" />
            刷新
          </Button>
        </div>
      </header>
      <div role="region" id="cpc-dashboard-panel-branches" aria-label={selected?.label}>
        <Card className="gap-0 overflow-hidden py-0">
          <SummaryTable data={data} loading={query.isFetching} />
        </Card>
      </div>
    </section>
  );
}

function SummaryTable({ data, loading }: { data: CPCDashboard | undefined; loading: boolean }) {
  const settings = useTableColumns(
    'cpc-dashboard-summary',
    columns.map((column) => ({ ...column, id: column.key })),
  );
  return (
    <>
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border px-5 py-4">
        <span className="text-xs text-muted-foreground">在编人员</span>
        <TableColumnSettings settings={settings} />
      </div>
      <Table aria-label={tabs[0].label} aria-busy={loading}>
        <TableHeader className="bg-table-header">
          <TableRow className="border-border hover:bg-transparent">
            {settings.visibleColumns.map((column) => (
              <TableHead
                key={column.key}
                scope="col"
                className="h-8 px-5 py-1 text-xs font-medium text-muted-foreground"
              >
                {column.label}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {data?.items.length ? (
            data.items.map((item) => (
              <TableRow
                key={item.party_branch_id}
                className="border-border-subtle hover:bg-table-hover"
              >
                {settings.visibleColumns.map(({ key }) =>
                  key === 'party_branch_name' ? (
                    <TableHead key={key} scope="row" className="h-12 px-5 py-2 font-normal">
                      {formatValue(item[key])}
                    </TableHead>
                  ) : (
                    <TableCell key={key} className="h-12 px-5 py-2">
                      <SummaryValue value={item[key]} />
                    </TableCell>
                  ),
                )}
              </TableRow>
            ))
          ) : loading && !data ? (
            Array.from({ length: 4 }, (_, index) => (
              <TableRow key={index} aria-hidden="true">
                {settings.visibleColumns.map(({ key }) => (
                  <TableCell key={key} className="h-12 px-5 py-2">
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
                {data ? '暂无党支部数据' : '暂无可展示的汇总数据'}
              </TableCell>
            </TableRow>
          )}
        </TableBody>
        {data ? (
          <TableFooter className="border-border bg-table-header">
            <TableRow className="hover:bg-transparent">
              {settings.visibleColumns.map(({ key }) => (
                <TableCell key={key} className="h-8 px-5 py-2 font-normal">
                  {key === 'party_branch_name' ? (
                    '合计'
                  ) : key === 'full_members' ||
                    key === 'probationary_members' ||
                    key === 'total' ? (
                    <SummaryValue value={data.totals[key]} />
                  ) : (
                    formatValue(null)
                  )}
                </TableCell>
              ))}
            </TableRow>
          </TableFooter>
        ) : null}
      </Table>
    </>
  );
}

function SummaryValue({ value }: { value: string | number }) {
  return typeof value === 'number' ? formatDashboardCount(value) : formatValue(value);
}
