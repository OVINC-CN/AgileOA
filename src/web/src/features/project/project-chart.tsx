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
import { ChartDataTable } from '@/components/chart-data-table';
import { ChartTooltipTable } from '@/components/chart-tooltip-table';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Tooltip as TextTooltip } from '@/components/ui/tooltip';
import type { Statistic } from '@/features/project/api';
import { buildProjectSeries } from '@/features/project/series';
import { formatNumber } from '@/lib/format';

const colors = [
  'var(--chart-1)',
  'var(--chart-2)',
  'var(--chart-3)',
  'var(--chart-4)',
  'var(--chart-5)',
];
const tooltipTableID = 'project-statistics-tooltip';
const tooltipColumns = [
  { id: 'project', label: '奖项' },
  { id: 'count', label: '人数' },
];

function ProjectTooltip({
  label,
  payload = [],
}: {
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
      tableID={tooltipTableID}
      caption={label === undefined ? undefined : `${String(label)} 年`}
      columns={tooltipColumns}
      rows={rows}
      fixedColumns
    />
  );
}

export function ProjectChart({ data }: { data: Statistic[] }) {
  const { names, rows, years, yearDomain } = buildProjectSeries(data);
  const [hidden, setHidden] = useState<string[]>([]);
  return (
    <Card
      className="gap-3 rounded-xl border-border p-4 shadow-panel"
      aria-label="各奖项年度获奖人数"
    >
      {names.length ? (
        <>
          <figure
            className="h-44 w-full min-w-0 sm:h-48"
            aria-label="按项目归属年份统计，每个奖项一条折线"
          >
            <ResponsiveContainer width="100%" height="100%" minWidth={0}>
              <LineChart
                data={rows}
                margin={{ top: 16, right: 16, left: 16, bottom: 4 }}
                accessibilityLayer
              >
                <CartesianGrid stroke="var(--border-subtle)" vertical={false} />
                <XAxis
                  dataKey="year"
                  type="number"
                  domain={yearDomain}
                  ticks={years}
                  allowDecimals={false}
                  axisLine={false}
                  tickLine={false}
                  tick={{ fontSize: 'var(--font-size-caption)', fill: 'var(--muted-foreground)' }}
                  tickMargin={12}
                  minTickGap={24}
                />
                <YAxis
                  allowDecimals={false}
                  domain={[0, 'dataMax + 10']}
                  padding={{ top: 20, bottom: 12 }}
                  width={48}
                  axisLine={false}
                  tickLine={false}
                  tick={<ChartCountTick />}
                  label={{
                    value: '人数',
                    position: 'insideTopLeft',
                    offset: 0,
                    fontSize: 'var(--font-size-caption)',
                    fill: 'var(--muted-foreground)',
                  }}
                />
                <Tooltip
                  content={<ProjectTooltip />}
                  cursor={{ stroke: 'var(--border)', strokeDasharray: '4 4' }}
                />
                {names.map((name, index) => (
                  <Line
                    key={name}
                    type="monotone"
                    dataKey={`project_${String(index)}`}
                    name={name}
                    stroke={colors[index % colors.length] ?? 'var(--chart-1)'}
                    strokeWidth={2.5}
                    strokeLinecap="round"
                    dot={{ r: 3, strokeWidth: 2, fill: 'var(--background)' }}
                    activeDot={{ r: 5, strokeWidth: 2, stroke: 'var(--background)' }}
                    hide={hidden.includes(name)}
                    isAnimationActive={false}
                  />
                ))}
              </LineChart>
            </ResponsiveContainer>
          </figure>
          <div role="group" aria-label="奖项图例" className="flex flex-wrap justify-end gap-1.5">
            {names.map((name, index) => (
              <TextTooltip key={name} content={name} onlyOverflow>
                <Button
                  type="button"
                  variant="ghost"
                  size="xs"
                  aria-pressed={!hidden.includes(name)}
                  className={`max-w-full gap-2 rounded-lg px-2.5 text-xs font-normal ${hidden.includes(name) ? 'text-muted-foreground' : 'bg-background/65 text-foreground ring-1 ring-border'}`}
                  onClick={() => {
                    setHidden((current) =>
                      current.includes(name)
                        ? current.filter((item) => item !== name)
                        : [...current, name],
                    );
                  }}
                >
                  <span
                    aria-hidden="true"
                    className="h-0.5 w-4 shrink-0 rounded-full"
                    style={{
                      backgroundColor: colors[index % colors.length],
                      opacity: hidden.includes(name) ? 0.35 : 1,
                    }}
                  />
                  <span data-tooltip-overflow className="min-w-0 truncate">
                    {name}
                  </span>
                </Button>
              </TextTooltip>
            ))}
          </div>
        </>
      ) : (
        <p className="py-8 text-center text-sm text-muted-foreground">暂无奖项统计</p>
      )}
      <ChartDataTable
        caption="奖项年度人数数据"
        columns={[
          { id: 'year', label: '归属年份' },
          { id: 'project', label: '奖项' },
          { id: 'count', label: '人数' },
        ]}
        rows={data.map((item) => ({
          id: JSON.stringify([item.year, item.name]),
          cells: [item.year, item.name, item.count],
        }))}
      />
    </Card>
  );
}
