import { describe, expect, it } from 'vitest';

import { sortSelectOptions } from '@/lib/select-options';

describe('下拉选项排序', () => {
  it('数字、完整拼音与英文混排，忽略大小写，支持多音字词组且不改动原始选项', () => {
    const options = Object.freeze([
      { value: 'zhang', label: '张三' },
      { value: 'banana', label: 'Banana' },
      { value: 'two', label: '2号' },
      { value: 'one', label: '1号' },
      { value: 'chongyang', label: '重阳' },
      { value: 'chongqing', label: '重庆' },
      { value: 'anhui', label: '安徽' },
      { value: 'lower', label: 'apple' },
      { value: 'upper', label: 'APPLE' },
      { value: 'beijing', label: '北京' },
      { value: 'symbol', label: '☆项目' },
    ]);
    const sorted = sortSelectOptions(options);
    expect(sorted.map(({ value }) => value)).toEqual([
      'one',
      'two',
      'anhui',
      'lower',
      'upper',
      'banana',
      'beijing',
      'chongqing',
      'chongyang',
      'zhang',
      'symbol',
    ]);
    expect(sorted.map(({ initial }) => initial)).toEqual([
      'numeric',
      'numeric',
      'A',
      'A',
      'A',
      'B',
      'B',
      'C',
      'C',
      'Z',
      '#',
    ]);
    expect(options[0]?.value).toBe('zhang');
  });

  it('特殊规则按原有顺序置顶，普通选项兼容全角数字和字母', () => {
    const sorted = sortSelectOptions([
      { value: 'beta', label: 'beta' },
      { value: 'all', label: '全部', pinned: true },
      { value: 'two', label: '２号' },
      { value: 'exclude', label: '不含审计', pinned: true },
      { value: 'alice', label: 'Ａlice' },
    ]);
    expect(sorted.map(({ value }) => value)).toEqual(['all', 'exclude', 'two', 'alice', 'beta']);
    expect(sorted.map(({ initial }) => initial)).toEqual(['pinned', 'pinned', 'numeric', 'A', 'B']);
  });

  it.each([
    {
      label: '月份',
      values: Array.from({ length: 12 }, (_, index) => 12 - index),
      suffix: ' 月',
      expected: Array.from({ length: 12 }, (_, index) => index + 1),
    },
    {
      label: '每页条数',
      values: [100, 20, 5, 50, 10],
      suffix: ' / 页',
      expected: [5, 10, 20, 50, 100],
    },
    {
      label: '年份',
      values: [2028, 2026, 2027, 2024],
      suffix: ' 年',
      expected: [2024, 2026, 2027, 2028],
    },
  ])(
    '$label 按完整数字排序，数字分组不会把十位数排到个位数之前',
    ({ values, suffix, expected }) => {
      const sorted = sortSelectOptions(
        values.map((value) => ({ value: String(value), label: `${String(value)}${suffix}` })),
      );
      expect(sorted.map(({ value }) => value)).toEqual(expected.map(String));
      expect(new Set(sorted.map(({ initial }) => initial))).toEqual(new Set(['numeric']));
    },
  );
});
