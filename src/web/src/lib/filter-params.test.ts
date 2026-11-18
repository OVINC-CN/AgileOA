import { describe, expect, it } from 'vitest';

import { filterParams } from '@/lib/filter-params';

describe('多选筛选参数', () => {
  it('重复传递选中值，保留值内的逗号和显式空选项，空数组不限制条件', () => {
    const parameters = filterParams({
      keyword: '',
      major: ['专业甲,方向一', '专业乙'],
      type: ['', 'employed'],
      grade: [],
      page: 2,
    });
    expect(parameters.getAll('major')).toEqual(['专业甲,方向一', '专业乙']);
    expect(parameters.getAll('type')).toEqual(['', 'employed']);
    expect(parameters.has('grade')).toBe(false);
    expect(parameters.has('keyword')).toBe(false);
    expect(parameters.get('page')).toBe('2');
  });
});
