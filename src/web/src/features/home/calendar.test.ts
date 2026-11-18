import { describe, expect, it } from 'vitest';

import { holidayBackground, holidayCountdown, holidayIllustration } from '@/features/home/calendar';

describe('首页节假日倒计时', () => {
  const newYear = { date: '2027-01-01', holidays: ['元旦假期'] };

  it('跨年时从假期首日前第七天开始展示', () => {
    expect(holidayCountdown('2026-12-25', newYear)).toEqual({
      days: 7,
      holidays: ['元旦假期'],
    });
    expect(holidayCountdown('2026-12-31', newYear)?.days).toBe(1);
  });

  it('超过七天、已经开始或没有后续资料时留白', () => {
    expect(holidayCountdown('2026-12-24', newYear)).toBeNull();
    expect(holidayCountdown('2027-01-01', newYear)).toBeNull();
    expect(holidayCountdown('2027-01-02', newYear)).toBeNull();
    expect(holidayCountdown('2026-12-25', null)).toBeNull();
    expect(holidayCountdown('2026-12-25', undefined)).toBeNull();
  });

  it('按公历日期差处理闰年，不依赖浏览器本地时区', () => {
    expect(
      holidayCountdown('2028-02-28', { date: '2028-03-01', holidays: ['示例节日'] })?.days,
    ).toBe(2);
    expect(holidayCountdown('无效日期', newYear)).toBeNull();
  });
});

describe('首页节假日插图资源', () => {
  it('节日名称带有或省略假期后缀时匹配同一张图片', () => {
    expect(holidayIllustration('2026-10-01', ['国庆节假期'])).toBe(
      `/extra-assets/imgs/holidays/${encodeURIComponent('国庆节')}.png`,
    );
    expect(holidayIllustration('2026-10-01', ['国庆节'])).toBe(
      holidayIllustration('2026-10-01', ['国庆节假期']),
    );
  });

  it.each([
    ['01-01', '元旦'],
    ['02-05', '春节'],
    ['04-03', '清明节'],
    ['05-01', '劳动节'],
    ['06-09', '端午节'],
    ['09-15', '中秋节'],
    ['10-01', '国庆节'],
  ])('各年份 %s 的%s假期复用同一张图片', (monthDay, name) => {
    const illustration = `/extra-assets/imgs/holidays/${encodeURIComponent(name)}.png`;
    for (const year of [2026, 2027, 2028]) {
      const date = `${String(year)}-${monthDay}`;
      expect(holidayIllustration(date, [`${name}假期`])).toBe(illustration);
      expect(holidayBackground(date, [`${name}假期`], null)).toBe(illustration);
    }
  });

  it('未提供插图的节日或无效日期不引用缺失资源', () => {
    expect(holidayIllustration('2026-10-01', ['其他节日'])).toBeNull();
    expect(holidayIllustration('无效日期', ['国庆节假期'])).toBeNull();
  });
});

describe('首页假期背景展示时段', () => {
  const nationalDay = { date: '2026-10-01', holidays: ['国庆节假期'] };
  const illustration = `/extra-assets/imgs/holidays/${encodeURIComponent('国庆节')}.png`;

  it('假期前七天开始显示，超过七天时隐藏', () => {
    expect(holidayBackground('2026-09-23', [], nationalDay)).toBeNull();
    expect(holidayBackground('2026-09-24', [], nationalDay)).toBe(illustration);
    expect(holidayBackground('2026-09-30', [], nationalDay)).toBe(illustration);
  });

  it.each(['2026-10-01', '2026-10-04', '2026-10-07'])(
    '国庆假期内 %s 持续显示，不依赖后续假期资料',
    (date) => {
      expect(holidayBackground(date, ['国庆节假期'], null)).toBe(illustration);
      expect(holidayCountdown(date, nationalDay)).toBeNull();
    },
  );

  it('假期结束且没有后续倒计时时隐藏图片', () => {
    expect(holidayBackground('2026-10-08', [], null)).toBeNull();
  });

  it('跨年倒计时按下一假期名称匹配通用图片', () => {
    expect(
      holidayBackground('2026-12-25', [], { date: '2027-01-01', holidays: ['元旦假期'] }),
    ).toBe(`/extra-assets/imgs/holidays/${encodeURIComponent('元旦')}.png`);
  });

  it('2027 年假期重叠倒计时时优先展示当天假期的图片', () => {
    expect(
      holidayBackground('2027-01-01', ['元旦假期'], {
        date: '2027-01-05',
        holidays: ['春节假期'],
      }),
    ).toBe(`/extra-assets/imgs/holidays/${encodeURIComponent('元旦')}.png`);
  });

  it('当天属于其他假期且没有图片时，不借用临近国庆的背景', () => {
    expect(holidayBackground('2026-09-25', ['其他节日'], nationalDay)).toBeNull();
  });
});
