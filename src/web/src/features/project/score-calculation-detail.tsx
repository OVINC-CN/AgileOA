import { InRuleTree } from '@/components/in-rule-tree';
import {
  rankingDimensionFields,
  type ScoreCalculation,
  scoreConditionFields,
} from '@/features/project/score-calculation';
import { formatValue } from '@/lib/format';

const fieldLabels = Object.fromEntries(
  scoreConditionFields.map(({ value, label }) => [value, label]),
);

export function ScoreCalculationDetail({ value }: { value: ScoreCalculation | null | undefined }) {
  return (
    <div className="space-y-5 text-sm">
      <dl className="grid gap-4 sm:grid-cols-2">
        <div className="space-y-1">
          <dt className="text-xs text-muted-foreground">总分计算</dt>
          <dd>{value?.enabled ? '开启' : '关闭'}</dd>
        </div>
        {value?.enabled ? (
          <div className="space-y-1">
            <dt className="text-xs text-muted-foreground">排名维度</dt>
            <dd>
              {rankingDimensionFields
                .filter(({ value: field }) => value.ranking_dimensions.includes(field))
                .map(({ label }) => label)
                .join('、') || '全奖项统一排名'}
            </dd>
          </div>
        ) : null}
      </dl>
      {value?.rules.length ? (
        <div className="space-y-5 border-t border-border pt-5">
          <p className="text-xs text-muted-foreground">按顺序使用首个匹配的条件组。</p>
          {value.rules.map((rule, index) => (
            <section
              key={index}
              aria-label={`分数计算第 ${String(index + 1)} 组`}
              className="min-w-0 space-y-4 border-b border-border pb-5 last:border-b-0 last:pb-0"
            >
              <h3 className="text-sm font-medium">第 {String(index + 1)} 组</h3>
              <div className="space-y-2">
                <h4 className="text-xs font-normal text-muted-foreground">条件</h4>
                {rule.condition ? (
                  <InRuleTree
                    rule={rule.condition}
                    labels={fieldLabels}
                    label={`第 ${String(index + 1)} 组条件`}
                  />
                ) : (
                  <p>{formatValue(null)}</p>
                )}
              </div>
              <div className="space-y-2">
                <h4 className="text-xs font-normal text-muted-foreground">公式</h4>
                <pre className="rounded-xl border border-border bg-muted/80 p-4 font-mono text-sm leading-6 wrap-anywhere whitespace-pre-wrap">
                  <code>{formatValue(rule.formula)}</code>
                </pre>
              </div>
            </section>
          ))}
        </div>
      ) : (
        <p className="text-muted-foreground">未配置分数计算规则</p>
      )}
    </div>
  );
}
