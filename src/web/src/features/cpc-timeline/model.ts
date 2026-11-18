import { z } from 'zod';

import { getConfig } from '@/lib/config';

export const fieldTypes = [
  { value: 'text', label: '单行文本' },
  { value: 'textarea', label: '多行文本' },
  { value: 'number', label: '数字' },
  { value: 'date', label: '日期' },
  { value: 'boolean', label: '是否' },
] as const;
export const studentFieldOptions = [
  { value: 'birth_date', label: '出生日期' },
  { value: 'adulthood_date', label: '成年日期（出生日期加 18 年）' },
] as const;
export const offsetUnits = [
  { value: 'years', label: '年' },
  { value: 'months', label: '月' },
  { value: 'days', label: '天' },
  { value: 'workdays', label: '工作日' },
] as const;

const text = (limit: number, required = false, multiline = false) =>
  z
    .string()
    .trim()
    .min(required ? 1 : 0)
    .max(limit)
    .refine((value) => !/\p{Cc}/u.test(multiline ? value.replace(/[\n\r\t]/gu, '') : value));
const idSchema = z.string().regex(/^[a-z][a-z0-9_]{0,63}$/u);
export const offsetSchema = z.strictObject({
  years: z.number().int(),
  months: z.number().int(),
  days: z.number().int(),
  workdays: z.number().int(),
});
const fieldSchema = z
  .strictObject({
    id: idSchema,
    name: text(100, true),
    type: z.enum(['text', 'textarea', 'number', 'date', 'boolean']),
    required: z.boolean(),
    student_field: z.enum(['', 'birth_date', 'adulthood_date']).optional(),
  })
  .refine((field) => !field.student_field || field.type === 'date', {
    message: '引用学生信息的字段必须为日期类型',
    path: ['student_field'],
  });
const ruleSchema = z.strictObject({
  id: idSchema,
  reference_node_id: idSchema,
  min_offset: offsetSchema.nullable(),
  max_offset: offsetSchema.nullable(),
});
export const nodeSchema = z.strictObject({
  numeric_id: z.number().int().positive().optional(),
  id: idSchema,
  name: text(100, true),
  description: text(1000, false, true),
  group_name: text(100).default(''),
  is_baseline: z.boolean().default(false),
  time_field_id: idSchema,
  fields: z.array(fieldSchema).nonempty(),
  rules: z.array(ruleSchema),
});
export const definitionSchema = z.strictObject({ nodes: z.array(nodeSchema).nonempty() });
export const transferFieldSchema = z.strictObject({
  source_field_id: idSchema,
  target_field_id: idSchema,
});
export const transferNodeSchema = z.strictObject({
  source_node_id: idSchema,
  target_node_id: idSchema,
  fields: z.array(transferFieldSchema),
});
export const transferSchema = z.strictObject({
  target_template_id: z.number().int().positive(),
  node_mappings: z.array(transferNodeSchema).nonempty(),
});
export const inputSchema = z.strictObject({
  name: text(100, true),
  description: text(1000, false, true),
  definition: definitionSchema,
  transfers: z.array(transferSchema).optional(),
});
export type TimelineInput = z.infer<typeof inputSchema>;
export type TimelineNode = z.infer<typeof nodeSchema>;
export type TimelineField = z.infer<typeof fieldSchema>;
export type TimelineRule = z.infer<typeof ruleSchema>;
export type TimeOffset = z.infer<typeof offsetSchema>;
export type TimelineTransfer = z.infer<typeof transferSchema>;
export type TransferNodeMapping = z.infer<typeof transferNodeSchema>;

export function newID(prefix: string) {
  return `${prefix}_${crypto.randomUUID().replaceAll('-', '')}`;
}

export function newNode(): TimelineNode {
  const timeID = newID('field');
  return {
    id: newID('node'),
    name: '新节点',
    description: '',
    group_name: '',
    is_baseline: false,
    time_field_id: timeID,
    fields: [{ id: timeID, name: '发生时间', type: 'date', required: true }],
    rules: [],
  };
}

export const zeroOffset = (): TimeOffset => ({ years: 0, months: 0, days: 0, workdays: 0 });

export function parseTimelineInput(value: unknown): TimelineInput {
  // 校验固定协议及引用，不接受人员发生时间或强制规则模式。
  const parsed = inputSchema.safeParse(value);
  if (!parsed.success) throw new Error('请检查模板、节点、字段和规则的必填项、类型及长度');
  const input = parsed.data;
  for (const node of input.definition.nodes) delete node.numeric_id;
  const limits = getConfig();
  const nodes = input.definition.nodes;
  if (nodes.length > limits.timelineMaxNodes) throw new Error('节点数超过限制');
  const ids = new Set(nodes.map((node) => node.id));
  if (ids.size !== nodes.length) throw new Error('节点标识重复');
  for (const node of nodes) {
    if (
      node.fields.length > limits.timelineMaxFieldsPerNode ||
      node.rules.length > limits.timelineMaxRulesPerNode
    ) {
      throw new Error(`“${node.name}”的字段或规则数量超过限制`);
    }
    const time = node.fields.find((field) => field.id === node.time_field_id);
    if (time?.type !== 'date' || !time.required) {
      throw new Error(`“${node.name}”必须指定一个必填日期字段作为发生时间`);
    }
    if (
      new Set(node.fields.map((field) => field.id)).size !== node.fields.length ||
      new Set(node.fields.map((field) => field.name)).size !== node.fields.length
    ) {
      throw new Error(`“${node.name}”的字段标识和名称不能重复`);
    }
    if (new Set(node.rules.map((rule) => rule.id)).size !== node.rules.length) {
      throw new Error('同一节点内的规则标识不能重复');
    }
    for (const rule of node.rules) {
      if (rule.reference_node_id === node.id || !ids.has(rule.reference_node_id)) {
        throw new Error(`“${node.name}”的规则须选择模板内的其他节点`);
      }
      if (!rule.min_offset && !rule.max_offset) throw new Error('每条规则至少配置一个时间边界');
      for (const offset of [rule.min_offset, rule.max_offset]) {
        if (
          offset &&
          Object.values(offset).some((part) => Math.abs(part) > limits.timelineMaxOffset)
        ) {
          throw new Error('相对时间偏移超过限制');
        }
      }
      const min = rule.min_offset;
      const max = rule.max_offset;
      if (isReversed(min, max)) {
        throw new Error('最早时间不能晚于最晚时间，请检查偏移范围');
      }
    }
  }
  const targets = new Set<number>();
  for (const transfer of input.transfers ?? []) {
    if (targets.has(transfer.target_template_id)) throw new Error('转移目标模板不能重复');
    targets.add(transfer.target_template_id);
    const pairs = new Set<string>();
    const targetFields = new Set<string>();
    for (const mapping of transfer.node_mappings) {
      const source = nodes.find((node) => node.id === mapping.source_node_id);
      const key = JSON.stringify([mapping.source_node_id, mapping.target_node_id]);
      if (!source || pairs.has(key)) throw new Error('转移来源节点不存在或节点映射重复');
      pairs.add(key);
      for (const field of mapping.fields) {
        const definition = source.fields.find((item) => item.id === field.source_field_id);
        if (!definition || definition.student_field) {
          throw new Error('转移字段必须存在且不能包含引用字段');
        }
        const targetKey = JSON.stringify([mapping.target_node_id, field.target_field_id]);
        if (targetFields.has(targetKey)) throw new Error('同一目标字段只能配置一个来源');
        targetFields.add(targetKey);
      }
    }
  }
  if (new TextEncoder().encode(JSON.stringify(input)).length > limits.timelineMaxDefinitionBytes) {
    throw new Error('模板定义超过大小限制');
  }
  return input;
}

export function moveNode(nodes: TimelineNode[], activeID: string, overID: string) {
  const from = nodes.findIndex((node) => node.id === activeID);
  const to = nodes.findIndex((node) => node.id === overID);
  if (from < 0 || to < 0 || from === to) return nodes;
  const next = [...nodes];
  const [node] = next.splice(from, 1);
  if (!node) return nodes;
  next.splice(to, 0, node);
  return next;
}

function isReversed(min: TimeOffset | null, max: TimeOffset | null) {
  if (!min || !max) return false;
  if (min.years === max.years && min.months === max.months) {
    if (min.workdays === max.workdays) return min.days > max.days;
    if (min.days === max.days) return min.workdays > max.workdays;
  }
  return (
    min.days === max.days &&
    min.workdays === max.workdays &&
    min.years * 12 + min.months > max.years * 12 + max.months
  );
}

export function removeNode(nodes: TimelineNode[], id: string) {
  const reference = nodes.find(
    (node) => node.id !== id && node.rules.some((rule) => rule.reference_node_id === id),
  );
  if (reference) throw new Error(`“${reference.name}”的规则仍引用此节点，请先处理引用`);
  return nodes.filter((node) => node.id !== id);
}
