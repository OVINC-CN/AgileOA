import type { WorkRecord } from '@/features/monthly-work/api';

// 工作日期保持自然日，不转换为浏览器时区中的时间点。
export function groupWorkRecords(records: WorkRecord[]) {
  const groups = new Map<string, WorkRecord[]>();
  for (const record of [...records].sort(
    (first, second) => second.date.localeCompare(first.date) || second.id - first.id,
  )) {
    const month = record.date.slice(0, 7);
    const items = groups.get(month) ?? [];
    items.push(record);
    groups.set(month, items);
  }
  return [...groups].map(([month, items]) => ({ month, items }));
}

export function monthLabel(month: string) {
  const [year, value = ''] = month.split('-');
  return `${year ?? ''} 年 ${String(Number(value))} 月`;
}
