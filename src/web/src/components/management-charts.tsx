import {
  Activity,
  CheckCheck,
  CircleAlert,
  ShieldCheck,
  UserCheck,
  UsersRound,
  UserX,
} from 'lucide-react';
import { useState } from 'react';
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
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import type { AuditDimension, AuditStatistics, UserStatistics } from '@/features/management/api';
import { formatNumber } from '@/lib/format';

const trendTooltipColumns = [
  { id: 'series', label: '统计项' },
  { id: 'count', label: '数量' },
];
const chartTick = { fontSize: 'var(--font-size-caption)', fill: 'var(--muted-foreground)' };
const chartColors = [
  'var(--chart-1)',
  'var(--chart-2)',
  'var(--chart-3)',
  'var(--chart-4)',
  'var(--chart-5)',
] as const;
const loginSeries = [
  { value: 'logins', label: '成功登录', color: chartColors[0] },
  { value: 'created', label: '新增账号', color: chartColors[1] },
];
const resultSeries = [
  { value: 'failure', label: '失败', color: chartColors[0] },
  { value: 'success', label: '成功', color: chartColors[1] },
];
const auditTrendTitles = {
  result: '操作结果趋势',
  workspace: '空间操作趋势',
  action: '各项操作趋势',
};

interface TrendSeries {
  value: string;
  label: string;
  color: string;
  dash?: string;
}

type ManagementTrendView = AuditDimension | 'users';
const trendCountKey = (index: number) => `counts[${String(index)}]`;

function TrendTooltip({
  tableID,
  showTotal,
  lastRowID,
  label,
  payload = [],
}: {
  tableID: string;
  showTotal: boolean;
  lastRowID?: string | undefined;
  label?: TooltipContentProps['label'];
  payload?: TooltipContentProps['payload'];
}) {
  const rows = payload.flatMap(({ dataKey, name, value, color }) =>
    typeof dataKey === 'string' && typeof name === 'string' && typeof value === 'number'
      ? [{ id: dataKey, label: name, color, value, cells: [formatNumber(value)] }]
      : [],
  );
  const total = rows.reduce((sum, row) => sum + row.value, 0);
  const caption =
    showTotal && label !== undefined ? `${String(label)} · 总数 ${formatNumber(total)}` : label;

  return (
    <ChartTooltipTable
      tableID={tableID}
      caption={caption}
      columns={trendTooltipColumns}
      rows={rows}
      fixedColumns
      lastRowID={lastRowID}
    />
  );
}

function Trend({
  tooltipTableID,
  title,
  days,
  series,
  loading,
  view,
}: {
  tooltipTableID: string;
  title: string;
  days: { date: string; counts: number[] }[];
  series: TrendSeries[];
  loading: boolean;
  view: ManagementTrendView;
}) {
  const [hiddenByView, setHiddenByView] = useState<Partial<Record<ManagementTrendView, string[]>>>(
    {},
  );
  const hidden = hiddenByView[view] ?? [];
  const otherIndex = series.findIndex(({ value }) => value === 'other');
  return (
    <Card className="gap-0 p-4 sm:p-5">
      <h2 className="mb-5 text-sm font-medium">{title}</h2>
      <figure aria-label={title} className="h-60 w-full min-w-0 sm:h-64">
        {days.length && series.length ? (
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
                content={
                  <TrendTooltip
                    tableID={tooltipTableID}
                    showTotal={view !== 'users'}
                    lastRowID={otherIndex < 0 ? undefined : trendCountKey(otherIndex)}
                  />
                }
                cursor={{ stroke: 'var(--border)', strokeDasharray: '4 4' }}
                isAnimationActive={false}
              />
              {series.map(({ value, label, color, dash }, index) => (
                <Line
                  key={value}
                  name={label}
                  type="monotone"
                  dataKey={trendCountKey(index)}
                  stroke={color}
                  strokeDasharray={dash ?? 'none'}
                  strokeWidth={2.5}
                  dot={days.length === 1 ? { r: 3 } : false}
                  activeDot={{ r: 4 }}
                  hide={hidden.includes(value)}
                  isAnimationActive={false}
                />
              ))}
            </LineChart>
          </ResponsiveContainer>
        ) : !loading ? (
          <p className="flex h-full items-center justify-center text-sm text-muted-foreground">
            暂无统计数据
          </p>
        ) : null}
        <figcaption className="sr-only">
          {series.length
            ? days
                .map((day) =>
                  [
                    day.date,
                    ...series.map(({ label }, index) => `${label}${String(day.counts[index])}`),
                  ].join('，'),
                )
                .join('；')
            : null}
        </figcaption>
      </figure>
      {series.length ? (
        <div role="group" aria-label="趋势图例" className="mt-3 flex flex-wrap justify-end gap-1.5">
          {series.map(({ value, label, color, dash }) => {
            const visible = !hidden.includes(value);
            return (
              <div
                key={value}
                className={value === 'other' ? 'flex w-full justify-end' : 'max-w-full min-w-0'}
              >
                <Button
                  type="button"
                  variant="ghost"
                  size="xs"
                  aria-pressed={visible}
                  className={`h-auto min-h-6 max-w-full gap-2 px-2.5 py-1 text-left text-xs font-normal whitespace-normal ${visible ? 'bg-background/65 text-foreground ring-1 ring-border' : 'text-muted-foreground'}`}
                  onClick={() => {
                    setHiddenByView({
                      ...hiddenByView,
                      [view]: visible
                        ? [...hidden, value]
                        : hidden.filter((item) => item !== value),
                    });
                  }}
                >
                  <svg aria-hidden="true" className="h-3 w-4 shrink-0" viewBox="0 0 16 12">
                    <path
                      d="M0 6h16"
                      stroke={color}
                      strokeWidth={2}
                      strokeDasharray={dash}
                      opacity={visible ? 1 : 0.35}
                    />
                  </svg>
                  <span className="min-w-0 wrap-anywhere">{label}</span>
                </Button>
              </div>
            );
          })}
        </div>
      ) : null}
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
        tooltipTableID="user-trend-tooltip"
        title="账号与登录趋势"
        days={
          data?.days.map(({ date, logins, created }) => ({ date, counts: [logins, created] })) ?? []
        }
        series={loginSeries}
        loading={loading}
        view="users"
      />
    </div>
  );
}

export function AuditCharts({
  data,
  loading,
  dimension = 'result',
}: {
  data: AuditStatistics | undefined;
  loading: boolean;
  dimension?: AuditDimension;
}) {
  const grouped = dimension !== 'result';
  const days = grouped
    ? (data?.trend?.days ?? [])
    : (data?.days.map(({ date, failure, success }) => ({ date, counts: [failure, success] })) ??
      []);
  const series = grouped
    ? (data?.trend?.series.map((item, index) => ({
        ...item,
        color:
          item.value === 'other'
            ? 'var(--muted-foreground)'
            : (chartColors[index % chartColors.length] ?? chartColors[0]),
        dash: index >= chartColors.length ? '8 4' : 'none',
      })) ?? [])
    : resultSeries;
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
        tooltipTableID="audit-trend-tooltip"
        title={auditTrendTitles[dimension]}
        days={days}
        series={series}
        loading={loading}
        view={dimension}
      />
    </div>
  );
}
