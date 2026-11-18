import {
  Activity,
  CheckCheck,
  CircleAlert,
  ShieldCheck,
  UserCheck,
  UsersRound,
  UserX,
} from 'lucide-react';
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  type TooltipContentProps,
  XAxis,
  YAxis,
} from 'recharts';

import { ChartCountTick } from '@/components/chart-count-tick';
import { ChartTooltipTable } from '@/components/chart-tooltip-table';
import { MetricCard as Metric } from '@/components/metric-card';
import { TableColumnSettings } from '@/components/table-column-settings';
import { Card } from '@/components/ui/card';
import type { AuditStatistics, UserStatistics } from '@/features/management/api';
import { formatNumber } from '@/lib/format';
import { useTableColumns } from '@/lib/table-columns';

const trendTooltipColumns = [
  { id: 'series', label: '统计项' },
  { id: 'count', label: '数量' },
];
const chartTick = { fontSize: 'var(--font-size-caption)', fill: 'var(--muted-foreground)' };

function TrendTooltip({
  tableID,
  label,
  payload = [],
}: {
  tableID: string;
  label?: TooltipContentProps['label'];
  payload?: TooltipContentProps['payload'];
}) {
  const rows = payload.flatMap(({ dataKey, name, value, color }) =>
    typeof dataKey === 'string' && typeof name === 'string' && typeof value === 'number'
      ? [{ id: dataKey, label: name, color, value, cells: [formatNumber(value)] }]
      : [],
  );

  return (
    <ChartTooltipTable
      tableID={tableID}
      caption={label}
      columns={trendTooltipColumns}
      rows={rows}
    />
  );
}

function Trend({
  tableID,
  title,
  days,
  first,
  second,
  loading,
}: {
  tableID: string;
  title: string;
  days: { date: string; first: number; second: number }[];
  first: string;
  second: string;
  loading: boolean;
}) {
  const tooltipSettings = useTableColumns(tableID, trendTooltipColumns);

  return (
    <Card className="gap-0 p-4 sm:p-5">
      <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-sm font-medium">{title}</h2>
        <div className="flex flex-wrap items-center gap-4 text-xs text-muted-foreground">
          <span className="flex items-center gap-1.5">
            <span className="size-2 rounded-full bg-[var(--chart-1)]" />
            {first}
          </span>
          <span className="flex items-center gap-1.5">
            <span className="size-2 rounded-full bg-[var(--chart-2)]" />
            {second}
          </span>
          <TableColumnSettings settings={tooltipSettings} label="提示字段" />
        </div>
      </div>
      <figure aria-label={title} className="h-60 w-full min-w-0 sm:h-64">
        {days.length ? (
          <ResponsiveContainer width="100%" height="100%" minWidth={0}>
            <LineChart
              data={days}
              margin={{ top: 8, right: 12, bottom: 0, left: 0 }}
              accessibilityLayer
            >
              <CartesianGrid stroke="var(--border-subtle)" vertical={false} />
              <XAxis
                dataKey="date"
                tickFormatter={(value: string) => value.slice(5).replace('-', '/')}
                tick={chartTick}
                axisLine={false}
                tickLine={false}
                minTickGap={28}
                tickMargin={12}
              />
              <YAxis
                allowDecimals={false}
                tick={<ChartCountTick />}
                axisLine={false}
                tickLine={false}
                width={40}
              />
              <Tooltip
                content={<TrendTooltip tableID={tableID} />}
                cursor={{ stroke: 'var(--border)', strokeDasharray: '4 4' }}
              />
              <Line
                name={first}
                type="monotone"
                dataKey="first"
                stroke="var(--chart-1)"
                strokeWidth={2.5}
                dot={false}
                activeDot={{ r: 4 }}
                isAnimationActive={false}
              />
              <Line
                name={second}
                type="monotone"
                dataKey="second"
                stroke="var(--chart-2)"
                strokeWidth={2.5}
                dot={false}
                activeDot={{ r: 4 }}
                isAnimationActive={false}
              />
            </LineChart>
          </ResponsiveContainer>
        ) : !loading ? (
          <p className="flex h-full items-center justify-center text-sm text-muted-foreground">
            暂无统计数据
          </p>
        ) : null}
        <figcaption className="sr-only">
          {days
            .map(
              (day) => `${day.date}，${first}${String(day.first)}，${second}${String(day.second)}`,
            )
            .join('；')}
        </figcaption>
      </figure>
    </Card>
  );
}

export function UserCharts({
  data,
  loading,
}: {
  data: UserStatistics | undefined;
  loading: boolean;
}) {
  return (
    <div className="space-y-5">
      <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
        <Metric
          title="用户总数"
          value={data?.total}
          note={data ? `系统管理员 ${String(data.admins)} 人` : '账号数量'}
          icon={UsersRound}
          valueSize="large"
        />
        <Metric
          title="启用账号"
          value={data?.enabled}
          note="当前允许登录的账号"
          icon={UserCheck}
          valueSize="large"
        />
        <Metric
          title="停用账号"
          value={data?.disabled}
          note="当前已停用的账号"
          icon={UserX}
          valueSize="large"
        />
        <Metric
          title="成功登录"
          value={data?.logins}
          note={data ? `最近 ${String(data.days.length)} 天` : '统计窗口内'}
          icon={Activity}
          valueSize="large"
        />
      </div>
      <Trend
        tableID="user-trend-tooltip"
        title="账号与登录趋势"
        days={
          data?.days.map((day) => ({ date: day.date, first: day.logins, second: day.created })) ??
          []
        }
        first="成功登录"
        second="新增账号"
        loading={loading}
      />
    </div>
  );
}

export function AuditCharts({
  data,
  loading,
}: {
  data: AuditStatistics | undefined;
  loading: boolean;
}) {
  return (
    <div className="space-y-5">
      <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
        <Metric
          title="操作总数"
          value={data?.total}
          note="当前查询窗口"
          icon={Activity}
          valueSize="large"
        />
        <Metric
          title="成功操作"
          value={data?.success}
          note="业务与审计均已提交"
          icon={CheckCheck}
          valueSize="large"
        />
        <Metric
          title="失败操作"
          value={data?.failure}
          note="查询窗口内的失败记录"
          icon={CircleAlert}
          valueSize="large"
        />
        <Metric
          title="被拒绝操作"
          value={data?.denied}
          note="已确认身份的拒绝记录"
          icon={ShieldCheck}
          valueSize="large"
        />
      </div>
      <Trend
        tableID="audit-trend-tooltip"
        title="操作结果趋势"
        days={
          data?.days.map((day) => ({
            date: day.date,
            first: day.failure,
            second: day.success,
          })) ?? []
        }
        first="失败"
        second="成功"
        loading={loading}
      />
    </div>
  );
}
