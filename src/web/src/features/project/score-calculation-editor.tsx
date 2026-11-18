import { ArrowDown, ArrowUp, Plus, Trash2 } from 'lucide-react';
import { useId, useRef } from 'react';

import { InRuleEditor } from '@/components/in-rule-editor';
import { AutoTextarea } from '@/components/ui/auto-textarea';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Label } from '@/components/ui/label';
import { SearchSelect } from '@/components/ui/search-select';
import {
  newScoreRule,
  rankingDimensionFields,
  type ScoreCalculation,
  scoreConditionFields,
  scoreReference,
  type ScoreRule,
} from '@/features/project/score-calculation';
import { ScoreCalculationPreview } from '@/features/project/score-calculation-preview';
import { getConfig } from '@/lib/config';

export function ScoreCalculationEditor({
  workspaceID,
  projectID,
  value,
  names,
  onChange,
  disabled = false,
}: {
  workspaceID: string;
  projectID?: number | undefined;
  value: ScoreCalculation;
  names: string[];
  onChange: (value: ScoreCalculation) => void;
  disabled?: boolean;
}) {
  const config = getConfig();
  const move = (index: number, offset: number) => {
    const rules = [...value.rules];
    const target = index + offset;
    const current = rules[index];
    const next = rules[target];
    if (!current || !next) return;
    rules[index] = next;
    rules[target] = current;
    onChange({ ...value, rules });
  };
  return (
    <div className="space-y-5">
      <div className="space-y-3">
        <label className="flex items-center gap-2 text-sm">
          <Checkbox
            checked={value.enabled}
            disabled={disabled}
            onChange={(event) => {
              const enabled = event.target.checked;
              onChange({
                ...value,
                enabled,
                ranking_dimensions: enabled ? value.ranking_dimensions : [],
              });
            }}
          />
          启用总分计算
        </label>
        {value.enabled ? (
          <div className="space-y-4">
            <p className="text-xs text-muted-foreground">
              总分按公式计算，学生申报和审核编辑不再手填总分、排名。
            </p>
            <fieldset className="space-y-2" disabled={disabled}>
              <legend className="mb-2 text-sm">排名维度</legend>
              <div className="flex flex-wrap gap-4">
                {rankingDimensionFields.map(({ value: field, label }) => (
                  <label key={field} className="flex items-center gap-2 text-sm">
                    <Checkbox
                      checked={value.ranking_dimensions.includes(field)}
                      onChange={(event) => {
                        onChange({
                          ...value,
                          ranking_dimensions: event.target.checked
                            ? [...value.ranking_dimensions, field]
                            : value.ranking_dimensions.filter((item) => item !== field),
                        });
                      }}
                    />
                    {label}
                  </label>
                ))}
              </div>
            </fieldset>
          </div>
        ) : null}
      </div>
      {value.enabled ? (
        <div className="space-y-4 border-t border-border pt-5">
          <p className="text-xs leading-5 text-muted-foreground">
            条件按完整文本匹配，可填写多个候选值，并组合多个条件。
            按顺序使用首个匹配的条件组；未匹配或计算失败时无法保存、提交。
          </p>
          {value.rules.map((rule, index) => (
            <section
              key={index}
              aria-label={`分数计算第 ${String(index + 1)} 组`}
              className="min-w-0 space-y-4 rounded-xl border border-border p-4"
            >
              <div className="flex items-center justify-between gap-2">
                <h3 className="text-sm font-medium">第 {String(index + 1)} 组</h3>
                <div className="flex items-center gap-1">
                  <Button
                    type="button"
                    size="icon-sm"
                    variant="ghost"
                    aria-label={`上移第 ${String(index + 1)} 组`}
                    disabled={disabled || index === 0}
                    onClick={() => {
                      move(index, -1);
                    }}
                  >
                    <ArrowUp aria-hidden="true" />
                  </Button>
                  <Button
                    type="button"
                    size="icon-sm"
                    variant="ghost"
                    aria-label={`下移第 ${String(index + 1)} 组`}
                    disabled={disabled || index === value.rules.length - 1}
                    onClick={() => {
                      move(index, 1);
                    }}
                  >
                    <ArrowDown aria-hidden="true" />
                  </Button>
                  <Button
                    type="button"
                    size="icon-sm"
                    variant="ghost"
                    aria-label={`删除第 ${String(index + 1)} 组`}
                    disabled={disabled}
                    onClick={() => {
                      onChange({
                        ...value,
                        rules: value.rules.filter((_, position) => position !== index),
                      });
                    }}
                  >
                    <Trash2 aria-hidden="true" />
                  </Button>
                </div>
              </div>
              <ScoreRuleFields
                rule={rule}
                names={names}
                disabled={disabled}
                onChange={(next) => {
                  onChange({
                    ...value,
                    rules: value.rules.map((item, position) => (position === index ? next : item)),
                  });
                }}
              />
            </section>
          ))}
          <Button
            type="button"
            variant="outline"
            disabled={disabled || value.rules.length >= config.projectScoreMaxRules}
            onClick={() => {
              onChange({ ...value, rules: [...value.rules, newScoreRule()] });
            }}
          >
            <Plus aria-hidden="true" />
            添加一组
          </Button>
          <ScoreCalculationPreview
            workspaceID={workspaceID}
            projectID={projectID}
            calculation={value}
            names={names}
            disabled={disabled}
          />
        </div>
      ) : null}
    </div>
  );
}

function ScoreRuleFields({
  rule,
  names,
  disabled,
  onChange,
}: {
  rule: ScoreRule;
  names: string[];
  disabled: boolean;
  onChange: (value: ScoreRule) => void;
}) {
  const config = getConfig();
  const id = useId();
  const formula = useRef<HTMLTextAreaElement>(null);
  return (
    <>
      <div className="space-y-2">
        <p className="text-sm">条件</p>
        <InRuleEditor
          rule={rule.condition}
          fields={scoreConditionFields}
          limits={{
            maxDepth: config.projectScoreRuleMaxDepth,
            maxNodes: config.projectScoreRuleMaxNodes,
            maxValues: config.projectScoreRuleMaxValues,
          }}
          readOnly={disabled}
          onChange={(condition) => {
            onChange({ ...rule, condition });
          }}
        />
      </div>
      <div className="space-y-2">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <Label htmlFor={id}>公式</Label>
          <SearchSelect
            label="插入子项"
            placeholder="插入子项"
            value=""
            options={names.map((name) => ({ value: name, label: name }))}
            disabled={disabled || names.length === 0}
            onValueChange={(name) => {
              if (!name || !formula.current) return;
              const start = formula.current.selectionStart;
              const end = formula.current.selectionEnd;
              const reference = scoreReference(name);
              onChange({
                ...rule,
                formula: rule.formula.slice(0, start) + reference + rule.formula.slice(end),
              });
              queueMicrotask(() => {
                formula.current?.focus();
                formula.current?.setSelectionRange(
                  start + reference.length,
                  start + reference.length,
                );
              });
            }}
          />
        </div>
        <AutoTextarea
          id={id}
          ref={formula}
          rows={3}
          disabled={disabled}
          spellCheck={false}
          maxLength={config.projectScoreFormulaMaxLength}
          placeholder="例如：MAX([子项A] * 0.6, 10)"
          value={rule.formula}
          onChange={(event) => {
            onChange({ ...rule, formula: event.target.value });
          }}
        />
        <p className="text-xs text-muted-foreground">支持子项分数、+ - * /、括号、MAX、MIN。</p>
      </div>
    </>
  );
}
