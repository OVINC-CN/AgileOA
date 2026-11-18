import { useState } from 'react';
import {
  Bar,
  CartesianGrid,
  ComposedChart,
  Line,
  ResponsiveContainer,
  Tooltip,
  type TooltipContentProps,
  XAxis,
  YAxis,
} from 'recharts';

import { ChartCountTick } from '@/components/chart-count-tick';
import { ChartDataTable } from '@/components/chart-data-table';
import { ChartTooltipTable } from '@/components/chart-tooltip-table';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Tooltip as TextTooltip } from '@/components/ui/tooltip';
import type { Dimension, GraduationStatistic } from '@/features/graduation/api';
import { buildGraduationSeries } from '@/features/graduation/series';
import { emptyValueLabel, formatNumber } from '@/lib/format';

const colors = [
  'var(--chart-1)',
  'var(--chart-2)',
  'var(--chart-3)',
  'var(--chart-4)',
  'var(--chart-5)',
];
const percentage = (value: number) => `${(value * 100).toFixed(1)}%`;
const tooltipColumns = {
  major: [
    { id: 'major', label: '专业' },
    { id: 'count', label: '人数' },
    { id: 'employment_rate', label: '就业率' },
  ],
  type: [
    { id: 'type', label: '去向类型' },
    { id: 'count', label: '人数' },
  ],
};
const tooltipTableID = (dimension: Dimension) => `graduation-statistics-${dimension}-tooltip`;

function GraduationTooltip({
  categories,
  dimension,
  statisticsByGroup,
  label,
  payload = [],
}: {
  categories: ReturnType<typeof buildGraduationSeries>['categories'];
  dimension: Dimension;
  statisticsByGroup: ReturnType<typeof buildGraduationSeries>['statisticsByGroup'];
  label?: TooltipContentProps['label'];
  payload?: TooltipContentProps['payload'];
}) {
  const byMajor = dimension === 'major';
  const points = new Map(payload.map((item) => [item.dataKey, item]));
  const groups = categories.flatMap((category) => {
    const point = points.get(category.countKey);
    if (typeof point?.value !== 'number') return [];
    const rateValue = points.get(category.rateKey)?.value;
    const rate = typeof rateValue === 'number' ? rateValue : undefined;
    const cells = [formatNumber(point.value)];
    if (byMajor) cells.push(rate === undefined ? emptyValueLabel : percentage(rate));
    return [
      {
        id: category.value,
        label: category.label,
        color: point.color,
        value: byMajor ? (rate ?? -1) : point.value,
        count: point.value,
        cells,
      },
    ];
  });
  const total = groups.reduce((sum, group) => sum + group.count, 0);
  const totals = [formatNumber(total)];
  if (byMajor) {
    const year = String(label);
    const employed = groups.reduce((sum, group) => {
      const statistic = statisticsByGroup.get(JSON.stringify([year, group.id]));
      return sum + (statistic?.employed ?? 0);
    }, 0);
    totals.push(total > 0 ? percentage(employed / total) : emptyValueLabel);
  }

  return (
    <ChartTooltipTable
      tableID={tooltipTableID(dimension)}
      caption={label}
      columns={tooltipColumns[dimension]}
      rows={groups}
      footer={{ label: '总数', cells: totals }}
      fixedColumns
    />
  );
}

export function GraduationChart({
  data,
  dimension,
}: {
  data: GraduationStatistic[];
  dimension: Dimension;
}) {
  const { categories, rows, records, statisticsByGroup } = buildGraduationSeries(data);
  const [hidden, setHidden] = useState<string[]>([]);
  const byMajor = dimension === 'major';
  const countMaximum = Math.max(
    0,
    ...rows.map((row) =>
      categories.reduce(
        (total, category) =>
          total + (hidden.includes(category.value) ? 0 : Number(row[category.countKey] ?? 0)),
        0,
      ),
    ),
  );
  return (
    <Card className="gap-3 rounded-xl border-border p-4 shadow-panel" aria-label="毕业去向统计">
      {rows.length ? (
        <>
          <figure
            className="h-44 w-full min-w-0 sm:h-48"
            aria-label={
              dimension === 'major'
                ? '按毕业年份展示各专业堆叠人数和就业率折线'
                : '按毕业年份展示去向类型堆叠人数'
            }
          >
            <ResponsiveContainer width="100%" height="100%" minWidth={0}>
              <ComposedChart
                data={rows}
                margin={{ top: 16, right: 4, left: 0, bottom: 4 }}
                accessibilityLayer
              >
                <CartesianGrid stroke="var(--border-subtle)" vertical={false} />
                <XAxis
                  dataKey="year"
                  axisLine={false}
                  tickLine={false}
                  tick={{ fontSize: 'var(--font-size-caption)', fill: 'var(--muted-foreground)' }}
                  tickMargin={12}
                  minTickGap={24}
                />
                <YAxis
                  yAxisId="count"
                  allowDecimals={false}
                  domain={[0, countMaximum]}
                  padding={{ top: 20, bottom: 12 }}
                  axisLine={false}
                  tickLine={false}
                  width={48}
                  tick={<ChartCountTick />}
                  label={{
                    value: '人数',
                    position: 'insideTopLeft',
                    offset: 0,
                    fontSize: 'var(--font-size-caption)',
                    fill: 'var(--muted-foreground)',
                  }}
                />
                {dimension === 'major' ? (
                  <YAxis
                    yAxisId="rate"
                    orientation="right"
                    domain={[0, 1]}
                    ticks={[0, 0.25, 0.5, 0.75, 1]}
                    tickFormatter={(value: number) => `${String(Math.round(value * 100))}%`}
                    axisLine={false}
                    tickLine={false}
                    width={54}
                    tick={{ fontSize: 'var(--font-size-caption)', fill: 'var(--muted-foreground)' }}
                  />
                ) : null}
                <Tooltip
                  content={
                    <GraduationTooltip
                      categories={categories}
                      dimension={dimension}
                      statisticsByGroup={statisticsByGroup}
                    />
                  }
                  cursor={{ fill: 'var(--muted)', opacity: 0.35 }}
                />
                {categories.map((category, index) => (
                  <Bar
                    key={category.countKey}
                    yAxisId="count"
                    dataKey={category.countKey}
                    name={category.label}
                    stackId="students"
                    fill={colors[index % colors.length] ?? 'var(--chart-1)'}
                    maxBarSize={48}
                    hide={hidden.includes(category.value)}
                    isAnimationActive={false}
                  />
                ))}
                {dimension === 'major'
                  ? categories.map((category, index) => (
                      <Line
                        key={category.rateKey}
                        yAxisId="rate"
                        type="linear"
                        dataKey={category.rateKey}
                        name={`${category.label}就业率`}
                        stroke={colors[index % colors.length] ?? 'var(--chart-1)'}
                        strokeWidth={2.5}
                        strokeDasharray="5 3"
                        strokeLinecap="round"
                        dot={{ r: 3, strokeWidth: 2, fill: 'var(--background)' }}
                        activeDot={{ r: 5, strokeWidth: 2, stroke: 'var(--background)' }}
                        hide={hidden.includes(category.value)}
                        isAnimationActive={false}
                      />
                    ))
                  : null}
              </ComposedChart>
            </ResponsiveContainer>
          </figure>
          <div
            role="group"
            aria-label="毕业去向图例"
            className="flex flex-wrap justify-end gap-1.5"
          >
            {categories.map((category, index) => (
              <TextTooltip key={category.value} content={category.label} onlyOverflow>
                <Button
                  type="button"
                  variant="ghost"
                  size="xs"
                  aria-pressed={!hidden.includes(category.value)}
                  className={`max-w-full gap-2 rounded-lg px-2.5 text-xs font-normal ${hidden.includes(category.value) ? 'text-muted-foreground' : 'bg-background/65 text-foreground ring-1 ring-border'}`}
                  onClick={() => {
                    setHidden((current) =>
                      current.includes(category.value)
                        ? current.filter((value) => value !== category.value)
                        : [...current, category.value],
                    );
                  }}
                >
                  <span
                    aria-hidden="true"
                    className="size-2.5 shrink-0 rounded-xs"
                    style={{
                      backgroundColor: colors[index % colors.length],
                      opacity: hidden.includes(category.value) ? 0.35 : 1,
                    }}
                  />
                  <span data-tooltip-overflow className="min-w-0 truncate">
                    {category.label}
                  </span>
                </Button>
              </TextTooltip>
            ))}
          </div>
          <ChartDataTable
            caption={`毕业去向统计数据（按${byMajor ? '专业' : '去向类型'}）`}
            columns={[
              { id: 'graduation_year', label: '毕业年份' },
              { id: dimension, label: byMajor ? '专业' : '去向类型' },
              { id: 'total', label: '学生人数' },
              { id: 'employed', label: '非待就业人数' },
              { id: 'pending', label: '待就业人数' },
              ...(byMajor ? [{ id: 'employment_rate', label: '就业率' }] : []),
            ]}
            rows={records.map((item) => ({
              id: JSON.stringify([item.graduation_year, item.value]),
              cells: [
                item.graduation_year,
                item.label,
                item.total,
                item.employed,
                item.pending,
                ...(byMajor ? [percentage(item.employment_rate)] : []),
              ],
            }))}
          />
        </>
      ) : (
        <p className="py-8 text-center text-sm text-muted-foreground">暂无毕业去向统计</p>
      )}
    </Card>
  );
}
