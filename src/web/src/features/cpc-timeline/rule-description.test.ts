import { expect, it } from 'vitest';

import {
  newNode,
  type TimelineRule,
  type TimeOffset,
  zeroOffset,
} from '@/features/cpc-timeline/model';
import {
  describeTimelineRule,
  timelineRuleRangeLabel,
} from '@/features/cpc-timeline/rule-description';

it.each<{
  min: TimeOffset | null;
  max: TimeOffset | null;
  label: string;
  sentence: string;
}>([
  { min: { ...zeroOffset(), days: 1 }, max: null, label: '最早时间', sentence: '1天后' },
  { min: null, max: { ...zeroOffset(), days: -10 }, label: '最晚时间', sentence: '10天前' },
  {
    min: { ...zeroOffset(), days: -10 },
    max: { ...zeroOffset(), days: 20 },
    label: '时间范围',
    sentence: '10天前至20天后',
  },
  { min: zeroOffset(), max: zeroOffset(), label: '时间范围', sentence: '当天' },
  {
    min: { years: 1, months: 2, days: -7, workdays: 5 },
    max: null,
    label: '最早时间',
    sentence: '1年2个月后再提前7天再延后5个工作日',
  },
])('单侧、双侧和复合偏移使用肯定句并保留范围含义：$sentence', ({ min, max, label, sentence }) => {
  const node = { ...newNode(), name: '材料核对' };
  const reference = { ...newNode(), name: '提交申请' };
  const rule: TimelineRule = {
    id: 'rule_range',
    reference_node_id: reference.id,
    min_offset: min,
    max_offset: max,
  };
  expect(describeTimelineRule(node, rule, [node, reference])).toBe(
    `材料核对应该在提交申请的${sentence}。`,
  );
  expect(timelineRuleRangeLabel(rule)).toBe(label);

  reference.name = '更新后的申请';
  expect(describeTimelineRule(node, rule, [node, reference])).toBe(
    `材料核对应该在更新后的申请的${sentence}。`,
  );
});

it('未填写的偏移保持待填写提示，不将其解释为当天', () => {
  const node = { ...newNode(), name: '材料核对' };
  const reference = newNode();
  expect(
    describeTimelineRule(
      node,
      {
        id: 'rule_range',
        reference_node_id: reference.id,
        min_offset: { ...zeroOffset(), days: Number.NaN },
        max_offset: null,
      },
      [node, reference],
    ),
  ).toBe('材料核对的时间偏移待填写完整。');
});
