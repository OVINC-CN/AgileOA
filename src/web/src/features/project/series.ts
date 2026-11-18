import type { Statistic } from '@/features/project/api';

// 项目名称仅作为映射值，使用固定系列代码，避免名称中的点或方括号被图表解析为路径。
export function buildProjectSeries(statistics: Statistic[]) {
  const names = [...new Set(statistics.map((item) => item.name))].sort((a, b) =>
    a.localeCompare(b, 'zh-CN'),
  );
  const years = [...new Set(statistics.map((item) => item.year))].sort((a, b) => a - b);
  const counts = new Map(
    statistics.map((item) => [JSON.stringify([item.year, item.name]), item.count]),
  );
  const rows: Record<string, number>[] = years.map((year) => {
    const row: Record<string, number> = { year };
    names.forEach((name, index) => {
      row[`project_${String(index)}`] = counts.get(JSON.stringify([year, name])) ?? 0;
    });
    return row;
  });
  const yearDomain: [number, number] = [(years[0] ?? 0) - 1, (years.at(-1) ?? 0) + 1];
  return { names, rows, years, yearDomain };
}
