import type { GraduationStatistic } from '@/features/graduation/api';

// 年份作为横轴，固定系列代码避免专业或类型名称被图表解析为对象路径。
export function buildGraduationSeries(data: GraduationStatistic[]) {
  const records = data.filter((item) => item.graduation_year !== '');
  const categories = [...new Map(records.map((item) => [item.value, item.label])).entries()]
    .sort((a, b) => a[1].localeCompare(b[1], 'zh-CN'))
    .map(([value, label], index) => ({
      value,
      label,
      countKey: `count_${String(index)}`,
      rateKey: `rate_${String(index)}`,
    }));
  const years = [...new Set(records.map((item) => item.graduation_year))].sort(
    (a, b) => Number(a) - Number(b),
  );
  const statisticsByGroup = new Map(
    records.map((item) => [JSON.stringify([item.graduation_year, item.value]), item]),
  );
  const rows = years.map((year) => {
    const row: Record<string, string | number | null> = { year };
    for (const category of categories) {
      const item = statisticsByGroup.get(JSON.stringify([year, category.value]));
      row[category.countKey] = item?.total ?? 0;
      row[category.rateKey] = item && item.total > 0 ? item.employment_rate : null;
    }
    return row;
  });
  return { categories, rows, records, statisticsByGroup };
}
