import { Calculator, ChevronDown, ChevronUp } from 'lucide-react';
import { type KeyboardEvent, useEffect, useState } from 'react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { previewScoreCalculation } from '@/features/project/api';
import {
  type ScoreCalculation,
  scoreConditionFields,
  type ScoreDimensions,
} from '@/features/project/score-calculation';
import { cleanScoreCalculation } from '@/features/project/score-calculation';
import { getConfig } from '@/lib/config';
import { formatScore } from '@/lib/format';
import { showMessage } from '@/lib/messages';

export function ScoreCalculationPreview({
  workspaceID,
  projectID,
  calculation,
  names,
  disabled,
}: {
  workspaceID: string;
  projectID?: number | undefined;
  calculation: ScoreCalculation;
  names: string[];
  disabled: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [dimensions, setDimensions] = useState<ScoreDimensions>({
    grade: '',
    education_level: '',
    major: '',
    class_name: '',
  });
  const [scores, setScores] = useState<Record<string, number | null>>({});
  const [result, setResult] = useState<{
    input: string;
    matched_rule: number;
    score: number;
  } | null>(null);
  const delay = getConfig().projectScorePreviewDelayMs;
  const input = JSON.stringify({ workspaceID, projectID, calculation, names, dimensions, scores });
  useEffect(() => {
    if (!open || disabled || !Object.values(dimensions).some((value) => value.trim())) return;
    try {
      cleanScoreCalculation(calculation);
    } catch {
      return;
    }
    let active = true;
    const timeout = window.setTimeout(() => {
      setResult(null);
      void previewScoreCalculation(workspaceID, projectID, calculation, names, dimensions, scores)
        .then((next) => {
          if (active) setResult({ input, ...next });
        })
        .catch((cause: unknown) => {
          if (active) showMessage(cause instanceof Error ? cause.message : '分数试算失败');
        });
    }, delay);
    return () => {
      active = false;
      window.clearTimeout(timeout);
    };
  }, [
    input,
    open,
    disabled,
    delay,
    workspaceID,
    projectID,
    calculation,
    names,
    dimensions,
    scores,
  ]);
  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Enter' && !event.nativeEvent.isComposing) event.preventDefault();
  };
  const currentResult = result?.input === input ? result : null;
  return (
    <section aria-label="分数试算" className="space-y-4 border-t border-border pt-4">
      <Button
        type="button"
        variant="outline"
        disabled={disabled}
        aria-label="分数试算"
        className="w-full justify-between"
        aria-expanded={open}
        onClick={() => {
          setOpen((current) => !current);
        }}
      >
        <span className="inline-flex items-center gap-2">
          <Calculator aria-hidden="true" />
          分数试算
          {open ? <ChevronUp aria-hidden="true" /> : <ChevronDown aria-hidden="true" />}
        </span>
        <span role="status" aria-label="试算总分" className="ml-auto tabular-nums">
          {currentResult ? formatScore(currentResult.score) : '-'}
        </span>
      </Button>
      {open ? (
        <fieldset disabled={disabled} className="space-y-4">
          <div className="grid gap-3 sm:grid-cols-2">
            {scoreConditionFields.map((field) => (
              <label key={field.value} className="space-y-2">
                <span className="text-sm">{field.label}</span>
                <Input
                  onKeyDown={onKeyDown}
                  aria-label={`试算${field.label}`}
                  maxLength={field.max_length}
                  value={dimensions[field.value]}
                  onChange={(event) => {
                    setDimensions((current) => ({ ...current, [field.value]: event.target.value }));
                  }}
                />
              </label>
            ))}
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            {names.map((name) => (
              <label key={name} className="min-w-0 space-y-2">
                <span className="block text-sm wrap-anywhere">{name}</span>
                <Input
                  type="number"
                  step="any"
                  onKeyDown={onKeyDown}
                  aria-label={`试算子项分数：${name}`}
                  value={scores[name] ?? ''}
                  onChange={(event) => {
                    setScores((current) => ({
                      ...current,
                      [name]: event.target.value === '' ? null : Number(event.target.value),
                    }));
                  }}
                />
              </label>
            ))}
          </div>
          <p className="text-xs text-muted-foreground">未填写的子项分数按 0 分计算。</p>
        </fieldset>
      ) : null}
    </section>
  );
}
