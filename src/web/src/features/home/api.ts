import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

import { apiGet } from '@/lib/api';

const calendarDaySchema = z.object({
  date: z.iso.date(),
  holidays: z.array(z.string().trim().min(1).max(100)),
});
const calendarSchema = calendarDaySchema.extend({
  next: calendarDaySchema
    .extend({ holidays: calendarDaySchema.shape.holidays.nonempty() })
    .nullable(),
});

// 节日是公开的全局资料，缓存按展示日期隔离，跨日后不保留昨天的结果。
export function homeCalendarOptions(date: string) {
  return queryOptions({
    queryKey: ['home', 'calendar', date] as const,
    queryFn: ({ signal }) =>
      apiGet(
        `/v1/home/calendar?${new URLSearchParams({ date })}`,
        calendarSchema.refine(
          (data) => data.date === date && (data.next === null || data.next.date > date),
        ),
        signal,
      ),
    refetchOnWindowFocus: true,
  });
}
