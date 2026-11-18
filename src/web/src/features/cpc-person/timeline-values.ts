import { z } from 'zod';

import type { TimelineValues } from '@/features/cpc-person/timeline-api';
import type { TimelineNode } from '@/features/cpc-timeline/model';
import { formatValue } from '@/lib/format';

export function parsePersonTimelineValues(
  node: TimelineNode,
  values: TimelineValues,
): TimelineValues {
  const result: TimelineValues = {};
  for (const field of node.fields) {
    // 引用由服务端实时解析，缺失时也无需填写或提交。
    if (field.student_field) continue;
    let value = values[field.id];
    if (typeof value === 'string') value = value.trim();
    if (value === undefined || value === null || value === '') {
      if (field.required) throw new Error(`请填写“${field.name}”`);
      continue;
    }
    switch (field.type) {
      case 'number':
        if (typeof value === 'string') value = Number(value);
        if (typeof value !== 'number' || !Number.isFinite(value)) {
          throw new Error(`“${field.name}”须填写有效数字`);
        }
        break;
      case 'boolean':
        if (typeof value !== 'boolean') throw new Error(`请选择“${field.name}”`);
        break;
      case 'date':
        if (!z.iso.date().safeParse(value).success || String(value).startsWith('0000')) {
          throw new Error(`“${field.name}”须填写有效日期`);
        }
        break;
      case 'text':
      case 'textarea': {
        const text = typeof value === 'string' ? value : '';
        const controls = field.type === 'textarea' ? text.replace(/[\n\r\t]/gu, '') : text;
        if (
          typeof value !== 'string' ||
          Array.from(text).length > 1000 ||
          /\p{Cc}/u.test(controls)
        ) {
          throw new Error(`“${field.name}”须填写最多 1000 字符的有效文本`);
        }
      }
    }
    result[field.id] = value;
  }
  return result;
}

export function timelineValueLabel(value: TimelineValues[string] | undefined) {
  if (typeof value === 'boolean') return value ? '是' : '否';
  return formatValue(value);
}
