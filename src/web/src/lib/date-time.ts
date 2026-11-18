import { calendarDate } from '@/lib/calendar';
import { getConfig } from '@/lib/config';

export function dateTimeParts(value: string, timeZone = getConfig().displayTimeZone) {
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hourCycle: 'h23',
  }).formatToParts(new Date(value));
  const fields = Object.fromEntries(parts.map((part) => [part.type, part.value]));
  return {
    date: `${fields['year'] ?? ''}-${fields['month'] ?? ''}-${fields['day'] ?? ''}`,
    hour: fields['hour'] ?? '',
    minute: fields['minute'] ?? '',
  };
}

export function utcDateTime(
  date: string,
  hour: string,
  minute: string,
  timeZone = getConfig().displayTimeZone,
): string | null {
  // 按配置时区解析墙上时间，校验夏令时跳变造成的不存在时间。
  if (!calendarDate(date) || !/^(?:[01]\d|2[0-3])$/u.test(hour) || !/^[0-5]\d$/u.test(minute))
    return null;
  const desired = Date.parse(`${date}T${hour}:${minute}:00Z`);
  let candidate = desired;
  for (let attempt = 0; attempt < 4; attempt++) {
    if (!Number.isFinite(candidate)) return null;
    const value = new Date(candidate).toISOString();
    if (!/^\d{4}-/u.test(value)) return null;
    const displayed = dateTimeParts(value, timeZone);
    const actual = Date.parse(`${displayed.date}T${displayed.hour}:${displayed.minute}:00Z`);
    if (actual === desired) return value;
    candidate += desired - actual;
  }
  return null;
}
