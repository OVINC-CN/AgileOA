import { z } from 'zod';

import { parsePersonTimelineValues } from '@/features/cpc-person/timeline-values';

export const timelineFilterFieldSchema = z.object({
  node_id: z.number().int().positive(),
  field_id: z.string().regex(/^[a-z][a-z0-9_]{0,63}$/u),
  label: z.string(),
  type: z.enum(['text', 'textarea', 'number', 'date', 'boolean']),
});
export type TimelineFilterField = z.infer<typeof timelineFilterFieldSchema>;
export interface TimelineFilterDraft {
  id: string;
  field: TimelineFilterField | null;
  operator: string;
  value: string;
  invalid?: boolean;
}

const emptyOperators = [
  { value: 'empty', label: '为空' },
  { value: 'not_empty', label: '不为空' },
];
export function timelineFilterOperators(type: TimelineFilterField['type']) {
  const equal = { value: 'eq', label: '等于' };
  switch (type) {
    case 'text':
    case 'textarea':
      return [{ value: 'contains', label: '包含' }, equal, ...emptyOperators];
    case 'number':
    case 'date':
      return [
        equal,
        { value: 'gt', label: type === 'date' ? '晚于' : '大于' },
        { value: 'gte', label: type === 'date' ? '不早于' : '大于或等于' },
        { value: 'lt', label: type === 'date' ? '早于' : '小于' },
        { value: 'lte', label: type === 'date' ? '不晚于' : '小于或等于' },
        ...emptyOperators,
      ];
    case 'boolean':
      return [equal, ...emptyOperators];
  }
}

export const isEmptyTimelineOperator = (operator: string) =>
  operator === 'empty' || operator === 'not_empty';

export function serializeTimelineFilters(filters: TimelineFilterDraft[]) {
  // 未选择字段或未填写比较值时不限制结果，空值判断通过独立运算符表达。
  return filters.flatMap(({ field, operator, value, invalid }) => {
    if (invalid) throw new Error('常用方案中的时间线字段已失效，请重新选择字段或删除该条件');
    if (!field || (!isEmptyTimelineOperator(operator) && !value.trim())) return [];
    if (!timelineFilterOperators(field.type).some((item) => item.value === operator)) {
      throw new Error('请选择有效的时间线筛选方式');
    }
    if (isEmptyTimelineOperator(operator)) {
      return [{ node_id: field.node_id, field_id: field.field_id, operator }];
    }
    if (field.type === 'boolean' && value !== 'true' && value !== 'false') {
      throw new Error('请选择时间线筛选值“是”或“否”');
    }
    const definition = {
      id: field.field_id,
      name: field.label,
      type: field.type,
      required: true,
    };
    const values = parsePersonTimelineValues(
      {
        id: 'filter',
        name: '',
        description: '',
        group_name: '',
        is_baseline: false,
        time_field_id: field.field_id,
        fields: [definition],
        rules: [],
      },
      { [field.field_id]: field.type === 'boolean' ? value === 'true' : value },
    );
    return [
      {
        node_id: field.node_id,
        field_id: field.field_id,
        operator,
        value: String(values[field.field_id]),
      },
    ];
  });
}
