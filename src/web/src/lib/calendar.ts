import { getConfig } from '@/lib/config';

export const minimumCalendarDate = '1900-01-01';
export const maximumCalendarDate = '9999-12-31';

export function currentCalendarDate() {
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone: getConfig().displayTimeZone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).formatToParts(new Date());
  return ['year', 'month', 'day']
    .map((type) => parts.find((part) => part.type === type)?.value ?? '')
    .join('-');
}

export function calendarDate(value: string, mode: 'date' | 'month' = 'date') {
  const source = mode === 'month' ? `${value}-01` : value;
  if (!/^\d{4}-\d{2}-\d{2}$/u.test(source)) return null;
  const date = new Date(`${source}T00:00:00Z`);
  return Number.isNaN(date.getTime()) || date.toISOString().slice(0, 10) !== source ? null : date;
}

export function calendarKey(date: Date) {
  return date.toISOString().slice(0, 10);
}

export function shiftCalendarMonth(month: string, amount: number) {
  const date = calendarDate(month, 'month');
  if (!date) return month;
  date.setUTCMonth(date.getUTCMonth() + amount);
  return calendarKey(date).slice(0, 7);
}

export function calendarLabel(value: string, mode: 'date' | 'month' = 'date') {
  const [year = '', month = '', day = ''] = value.split('-');
  return `${year}年${String(Number(month))}月${mode === 'date' ? `${String(Number(day))}日` : ''}`;
}
