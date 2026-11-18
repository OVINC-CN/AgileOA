import { useQuery } from '@tanstack/react-query';

import { homeCalendarOptions } from '@/features/home/api';
import { calendarDate } from '@/lib/calendar';

const millisecondsPerDay = 24 * 60 * 60 * 1000;
const countdownWindowDays = 7;
const holidayIllustrations = [
  '元旦',
  '春节',
  '清明节',
  '劳动节',
  '端午节',
  '中秋节',
  '国庆节',
] as const;

interface CalendarDay {
  date: string;
  holidays: string[];
}

// 各年份按节日名称复用已提供的静态资源，未知名称不拼接资源路径。
export function holidayIllustration(date: string, holidays: string[]) {
  // 校验日期，避免无效日历数据引用图片。
  if (!calendarDate(date)) return null;

  // 仅匹配已提供图片的节日名称。
  const illustration = holidayIllustrations.find((name) =>
    holidays.some((holiday) => holiday.replace(/假期$/u, '') === name),
  );
  return illustration
    ? `/extra-assets/imgs/holidays/${encodeURIComponent(illustration)}.png`
    : null;
}

// 节假日只在临近首日时显示倒计时，使用纯日期差兼容跨年、闰年和夏令时。
export function holidayCountdown(date: string, next: CalendarDay | null | undefined) {
  const today = calendarDate(date);
  const start = next ? calendarDate(next.date) : null;
  if (!today || !start || !next) return null;

  const days = (start.getTime() - today.getTime()) / millisecondsPerDay;
  return days > 0 && days <= countdownWindowDays ? { days, holidays: next.holidays } : null;
}

// 假期内优先展示当天主题，普通日期才展示临近假期的倒计时背景。
export function holidayBackground(
  date: string,
  holidays: string[],
  next: CalendarDay | null | undefined,
) {
  if (holidays.length > 0) return holidayIllustration(date, holidays);

  const countdown = holidayCountdown(date, next);
  return countdown && next ? holidayIllustration(next.date, next.holidays) : null;
}

export function useHomeCalendar(date: string) {
  const calendar = useQuery(homeCalendarOptions(date));
  const next = calendar.data?.next;
  const holidays = calendar.data?.holidays ?? [];
  const countdown = holidayCountdown(date, next);
  return {
    holidays,
    countdown,
    illustration: holidayBackground(date, holidays, next),
  };
}
