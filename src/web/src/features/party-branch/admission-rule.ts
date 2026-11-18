import { z } from 'zod';

import { getConfig } from '@/lib/config';

export type AdmissionRuleNode =
  | { type: 'group'; logic: 'and' | 'or'; children: AdmissionRuleNode[] }
  | {
      type: 'condition';
      field: 'advisor' | 'major' | 'education_level' | 'student_no';
      operator: 'in';
      values: string[];
    };

export const ruleFieldLabels = {
  advisor: '导师姓名',
  major: '专业',
  education_level: '学历',
  student_no: '学号',
};
const ruleFieldLimits = { advisor: 50, major: 100, education_level: 32, student_no: 32 };

export const admissionRuleNodeSchema: z.ZodType<AdmissionRuleNode> = z.lazy(() =>
  z.discriminatedUnion('type', [
    z.strictObject({
      type: z.literal('group'),
      logic: z.enum(['and', 'or']),
      children: z.array(admissionRuleNodeSchema).min(1),
    }),
    z
      .strictObject({
        type: z.literal('condition'),
        field: z.enum(['advisor', 'major', 'education_level', 'student_no']),
        operator: z.literal('in'),
        values: z
          .array(
            z
              .string()
              .trim()
              .min(1)
              .max(100)
              .refine((value) => !/\p{Cc}/u.test(value)),
          )
          .min(1),
      })
      .refine((node) =>
        node.values.every((value) => Array.from(value).length <= ruleFieldLimits[node.field]),
      ),
  ]),
);

export function ruleValues(values: string[]) {
  return [
    ...new Set(
      values
        .flatMap((value) => value.split(/[\r\n,，、]/u))
        .map((value) => value.trim())
        .filter(Boolean),
    ),
  ];
}

export function ruleSize(node: AdmissionRuleNode | null): { nodes: number; depth: number } {
  if (node === null) return { nodes: 0, depth: 0 };
  if (node.type === 'condition') return { nodes: 1, depth: 1 };
  const children = node.children.map(ruleSize);
  return {
    nodes: 1 + children.reduce((total, child) => total + child.nodes, 0),
    depth: 1 + Math.max(0, ...children.map((child) => child.depth)),
  };
}

export function cleanAdmissionRule(rule: AdmissionRuleNode | null) {
  if (rule === null) return null;
  const config = getConfig();
  let count = 0;
  const clean = (node: AdmissionRuleNode, depth: number): AdmissionRuleNode => {
    count += 1;
    if (depth > config.partyBranchRuleMaxDepth || count > config.partyBranchRuleMaxNodes) {
      throw new Error('准入规则超过层数或节点数量限制');
    }
    if (node.type === 'group') {
      return { ...node, children: node.children.map((child) => clean(child, depth + 1)) };
    }
    const values = ruleValues(node.values);
    if (values.length > config.partyBranchRuleMaxValues) {
      throw new Error('准入条件候选值超过数量限制');
    }
    return { ...node, values };
  };
  const result = admissionRuleNodeSchema.safeParse(clean(rule, 1));
  if (!result.success) {
    throw new Error(
      '请检查准入规则：条件组不能为空，导师姓名最多 50 字符，专业最多 100 字符，学历和学号最多 32 字符，每个条件至少填写一个值',
    );
  }
  return result.data;
}

export function newRuleCondition(): AdmissionRuleNode {
  return { type: 'condition', field: 'advisor', operator: 'in', values: [] };
}

export function newRuleGroup(): AdmissionRuleNode {
  return { type: 'group', logic: 'and', children: [newRuleCondition()] };
}
