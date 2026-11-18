import { expect, it } from 'vitest';

import { dateTimeParts, utcDateTime } from '@/lib/date-time';

it('日期时间按展示时区与 UTC 双向转换，保留跨日语义', () => {
  expect(utcDateTime('2026-12-31', '23', '59', 'Asia/Shanghai')).toBe('2026-12-31T15:59:00.000Z');
  expect(utcDateTime('2026-12-31', '00', '01', 'Asia/Shanghai')).toBe('2026-12-30T16:01:00.000Z');
  expect(dateTimeParts('2026-12-30T16:01:00Z', 'Asia/Shanghai')).toEqual({
    date: '2026-12-31',
    hour: '00',
    minute: '01',
  });
});

it('拒绝不存在的日期、时间和夏令时跳变时刻', () => {
  expect(utcDateTime('2026-02-30', '23', '59', 'Asia/Shanghai')).toBeNull();
  expect(utcDateTime('2026-12-31', '24', '00', 'Asia/Shanghai')).toBeNull();
  expect(utcDateTime('2026-03-08', '02', '30', 'America/New_York')).toBeNull();
  expect(utcDateTime('2026-03-08', '03', '30', 'America/New_York')).toBe(
    '2026-03-08T07:30:00.000Z',
  );
});
