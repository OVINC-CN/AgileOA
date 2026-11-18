import { expect, it } from 'vitest';

import { newNode, parseTimelineInput, type TimelineInput } from '@/features/cpc-timeline/model';
import {
  connectTransferMapping,
  transferItemID,
  transferMappingItems,
  transferMappingLinks,
  transferMappingProblem,
} from '@/features/cpc-timeline/transfer-mapping';

function input(): TimelineInput {
  const first = newNode();
  first.id = 'node_first';
  const firstField = first.fields[0];
  if (!firstField) throw new Error('缺少来源字段');
  firstField.id = first.time_field_id = 'date';
  const second = newNode();
  second.id = 'node_second';
  const secondField = second.fields[0];
  if (!secondField) throw new Error('缺少来源字段');
  secondField.id = second.time_field_id = 'date';
  return {
    name: '来源模板',
    description: '',
    definition: { nodes: [first, second] },
    transfers: [
      {
        target_template_id: 2,
        node_mappings: [
          {
            source_node_id: first.id,
            target_node_id: 'node_target',
            fields: [{ source_field_id: 'date', target_field_id: 'first_date' }],
          },
          {
            source_node_id: first.id,
            target_node_id: 'node_other',
            fields: [{ source_field_id: 'date', target_field_id: 'date' }],
          },
          {
            source_node_id: second.id,
            target_node_id: 'node_target',
            fields: [{ source_field_id: 'date', target_field_id: 'second_date' }],
          },
        ],
      },
    ],
  };
}

it('兼容旧模板并允许多个目标和多对多节点、字段映射', () => {
  const value = input();
  expect(parseTimelineInput(value).transfers?.[0]?.node_mappings).toHaveLength(3);
  value.transfers?.push({
    target_template_id: 3,
    node_mappings: [{ source_node_id: 'node_first', target_node_id: 'node_other', fields: [] }],
  });
  expect(parseTimelineInput(value).transfers).toHaveLength(2);
  delete value.transfers;
  expect(() => parseTimelineInput(value)).not.toThrow();
});

it('拒绝多个来源覆盖同一目标字段及只读引用字段映射', () => {
  const value = input();
  const target = value.transfers?.[0]?.node_mappings[2]?.fields[0];
  if (!target) throw new Error('缺少目标字段映射');
  target.target_field_id = 'first_date';
  expect(() => parseTimelineInput(value)).toThrow('同一目标字段');
  const reference = input();
  const field = reference.definition.nodes[0]?.fields[0];
  if (!field) throw new Error('缺少来源字段');
  field.student_field = 'birth_date';
  expect(() => parseTimelineInput(reference)).toThrow('引用字段');
});

it('字段拖入自动建立节点映射，重复连接不增加关系，支持分发和合并', () => {
  const source = input().definition.nodes;
  const target = structuredClone(source);
  const firstTarget = target[0];
  if (!firstTarget) throw new Error('缺少目标节点');
  firstTarget.fields.push({ id: 'other_date', name: '补充日期', type: 'date', required: false });
  const item = (nodes: typeof source, nodeID: string, fieldID: string) => {
    const value = transferMappingItems(nodes).find(
      (entry) => entry.id === transferItemID(nodeID, fieldID),
    );
    if (!value) throw new Error('缺少映射字段');
    return value;
  };
  const from = item(source, 'node_first', 'date');
  const to = item(target, 'node_first', 'date');
  const first = connectTransferMapping([], from, to);
  expect(first).toEqual([
    {
      source_node_id: 'node_first',
      target_node_id: 'node_first',
      fields: [{ source_field_id: 'date', target_field_id: 'date' }],
    },
  ]);
  expect(connectTransferMapping(first, from, to)).toBe(first);
  const distributed = connectTransferMapping(first, from, item(target, 'node_second', 'date'));
  const merged = connectTransferMapping(
    distributed,
    item(source, 'node_second', 'date'),
    item(target, 'node_first', 'other_date'),
  );
  expect(merged).toHaveLength(3);
  expect(transferMappingLinks(merged)).toHaveLength(6);
  expect(() => connectTransferMapping(merged, item(source, 'node_second', 'date'), to)).toThrow(
    '已有来源',
  );
  expect(first[0]?.fields).toHaveLength(1);
});

it('拖拽校验拒绝节点字段混连、类型不一致及只读引用字段', () => {
  const nodes = input().definition.nodes;
  const first = nodes[0];
  if (!first) throw new Error('缺少节点');
  first.fields.push({ id: 'count', name: '数量', type: 'number', required: false });
  first.fields.push({
    id: 'birth',
    name: '出生日期',
    type: 'date',
    required: false,
    student_field: 'birth_date',
  });
  const items = transferMappingItems(nodes);
  const node = items.find((item) => item.kind === 'node');
  const date = items.find((item) => item.kind === 'field' && item.field.id === 'date');
  const count = items.find((item) => item.kind === 'field' && item.field.id === 'count');
  const reference = items.find((item) => item.kind === 'field' && item.field.id === 'birth');
  expect(transferMappingProblem(node, date, [])).toContain('节点请映射到节点');
  expect(transferMappingProblem(date, count, [])).toContain('相同类型');
  expect(reference?.disabled).toBe(true);
  expect(transferMappingProblem(reference, date, [])).toContain('引用字段');
  expect(transferMappingProblem(date, reference, [])).toContain('引用字段');
});
