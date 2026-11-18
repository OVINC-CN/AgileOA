import { expect, it } from 'vitest';

import { validateUniqueDetailNames } from '@/features/project/detail-validation';

it('申报和获奖评分拒绝同名子项，包括首尾空格差异', () => {
  expect(() => {
    validateUniqueDetailNames([{ name: '志愿服务' }, { name: ' 志愿服务 ' }]);
  }).toThrow('同一条记录中的子项名称不能重复');
  expect(() => {
    validateUniqueDetailNames([{ name: '志愿服务' }, { name: '社会实践' }]);
  }).not.toThrow();
});
