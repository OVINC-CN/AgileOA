import { z } from 'zod';

import type { InRuleNode } from '@/components/in-rule-editor';
import { getConfig } from '@/lib/config';

export const scoreConditionFields = [
  { value: 'grade', label: '年级', max_length: 100 },
  { value: 'education_level', label: '学历', max_length: 32 },
  { value: 'major', label: '专业', max_length: 100 },
  { value: 'class_name', label: '班级', max_length: 100 },
] as const;

export const rankingDimensionFields = [
  { value: 'grade', label: '年级' },
  { value: 'major', label: '专业' },
  { value: 'class_name', label: '班级' },
] as const;

type ScoreDimensionField = (typeof scoreConditionFields)[number]['value'];
export type ScoreCondition = InRuleNode<ScoreDimensionField>;
export type ScoreDimensions = Record<ScoreDimensionField, string>;

const conditionSchema: z.ZodType<ScoreCondition> = z.lazy(() =>
  z.discriminatedUnion('type', [
    z.strictObject({
      type: z.literal('group'),
      logic: z.enum(['and', 'or']),
      children: z.array(conditionSchema),
    }),
    z.strictObject({
      type: z.literal('condition'),
      field: z.enum(['grade', 'education_level', 'major', 'class_name']),
      operator: z.literal('in'),
      values: z.array(z.string()),
    }),
  ]),
);

export const scoreCalculationSchema = z.object({
  enabled: z.boolean(),
  ranking_dimensions: z.array(z.enum(['grade', 'major', 'class_name'])),
  rules: z.array(z.object({ condition: conditionSchema.nullable(), formula: z.string() })),
});

export type ScoreCalculation = z.infer<typeof scoreCalculationSchema>;
export type ScoreRule = ScoreCalculation['rules'][number];

export const emptyScoreCalculation = (): ScoreCalculation => ({
  enabled: false,
  ranking_dimensions: [],
  rules: [],
});

export const newScoreRule = (): ScoreRule => ({
  condition: { type: 'condition', field: 'grade', operator: 'in', values: [] },
  formula: '',
});

export function cleanScoreCalculation(value: ScoreCalculation | null | undefined) {
  if (value === null || value === undefined) return value;
  const config = getConfig();
  if (!value.enabled && value.ranking_dimensions.length) {
    throw new Error('请先启用总分计算再配置排名维度');
  }
  if (new Set(value.ranking_dimensions).size !== value.ranking_dimensions.length) {
    throw new Error('排名维度不能重复');
  }
  if (value.rules.length > config.projectScoreMaxRules || (value.enabled && !value.rules.length)) {
    throw new Error('请至少配置一组条件和公式，规则组数量不能超过限制');
  }
  const rules = value.rules.map((rule, index) => {
    let count = 0;
    const clean = (node: ScoreCondition | null, depth: number): ScoreCondition => {
      count += 1;
      if (
        !node ||
        depth > config.projectScoreRuleMaxDepth ||
        count > config.projectScoreRuleMaxNodes
      ) {
        throw new Error(`第 ${String(index + 1)} 组条件为空或超过层数、节点数量限制`);
      }
      if (node.type === 'group') {
        if (!node.children.length) throw new Error(`第 ${String(index + 1)} 组的条件组不能为空`);
        return { ...node, children: node.children.map((child) => clean(child, depth + 1)) };
      }
      const values = [...new Set(node.values.map((item) => item.trim()))];
      const maxLength = scoreConditionFields.find(({ value }) => value === node.field)?.max_length;
      if (
        !values.length ||
        values.length > config.projectScoreRuleMaxValues ||
        values.some(
          (item) =>
            !item || !maxLength || Array.from(item).length > maxLength || /\p{Cc}/u.test(item),
        )
      ) {
        throw new Error(`第 ${String(index + 1)} 组的每个条件至少填写一个有效候选值`);
      }
      return { ...node, values };
    };
    const condition = clean(rule.condition, 1);
    const formula = rule.formula.trim();
    if (!formula || Array.from(formula).length > config.projectScoreFormulaMaxLength) {
      throw new Error(`第 ${String(index + 1)} 组的公式为空或超过长度限制`);
    }
    return { condition, formula };
  });
  return {
    ...value,
    ranking_dimensions: rankingDimensionFields
      .filter(({ value: field }) => value.ranking_dimensions.includes(field))
      .map(({ value: field }) => field),
    rules,
  };
}

export function scoreReference(name: string) {
  return `[${name.replaceAll('\\', '\\\\').replaceAll(']', '\\]')}]`;
}
