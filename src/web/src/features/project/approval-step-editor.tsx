import { useQuery } from '@tanstack/react-query';
import { ArrowDown, ArrowUp, Plus, Trash2 } from 'lucide-react';
import { useState } from 'react';

import { ErrorMessage } from '@/components/error-message';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Label } from '@/components/ui/label';
import { SearchSelect } from '@/components/ui/search-select';
import type { ApprovalStep } from '@/features/project/api';
import {
  approvalOptions,
  approvalSignModes,
  approverKinds,
} from '@/features/project/application-api';
import { getConfig } from '@/lib/config';
import { showMessage } from '@/lib/messages';

const emptyStep: ApprovalStep = {
  kind: 'applicant',
  sign_mode: 'any',
  allow_edit: true,
  class_positions: [],
  student_ids: [],
  user_ids: [],
  reviewer_label: '申报人',
  reviewer_options: [],
};

export function ApprovalStepEditor({
  workspaceID,
  steps,
  onChange,
}: {
  workspaceID: string;
  steps: ApprovalStep[];
  onChange: (steps: ApprovalStep[]) => void;
}) {
  const [rowKeys, setRowKeys] = useState(() => steps.map(() => crypto.randomUUID()));
  const move = (index: number, delta: number) => {
    const next = [...steps];
    const step = next.splice(index, 1)[0];
    if (step) next.splice(index + delta, 0, step);
    setRowKeys((keys) => {
      const result = [...keys];
      const key = result.splice(index, 1)[0];
      if (key) result.splice(index + delta, 0, key);
      return result;
    });
    onChange(next);
  };
  return (
    <section className="space-y-3" aria-label="审批流配置">
      <div className="flex items-center justify-between gap-3">
        <h2 className="text-sm font-medium">审批流</h2>
        <Button
          type="button"
          variant="outline"
          disabled={steps.length >= getConfig().projectMaxApprovalSteps}
          onClick={() => {
            setRowKeys((keys) => [...keys, crypto.randomUUID()]);
            onChange([...steps, { ...emptyStep }]);
          }}
        >
          <Plus aria-hidden="true" />
          添加审批级别
        </Button>
      </div>
      <p className="text-xs leading-relaxed text-muted-foreground">
        按顺序逐级审批。或签由任一处理人通过，并签需要全部处理人通过；同一学生匹配多个班委时只签一次。任意一级驳回都退回申报人，重新提交后从第一级审批。
        各级可分别设置是否允许编辑申报内容及附件；仅调整编辑开关不重置审批进度。
      </p>
      {steps.map((step, index) => (
        <div
          key={rowKeys[index] ?? index}
          className="space-y-3 rounded-lg border border-border p-4"
        >
          <div className="flex items-center justify-between gap-2">
            <h3 className="text-sm font-medium">第 {String(index + 1)} 级</h3>
            <div className="flex gap-1">
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label={`上移第 ${String(index + 1)} 级`}
                disabled={index === 0}
                onClick={() => {
                  move(index, -1);
                }}
              >
                <ArrowUp aria-hidden="true" />
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label={`下移第 ${String(index + 1)} 级`}
                disabled={index === steps.length - 1}
                onClick={() => {
                  move(index, 1);
                }}
              >
                <ArrowDown aria-hidden="true" />
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label={`删除第 ${String(index + 1)} 级`}
                onClick={() => {
                  setRowKeys((keys) => keys.filter((_, position) => position !== index));
                  onChange(steps.filter((_, position) => position !== index));
                }}
              >
                <Trash2 aria-hidden="true" />
              </Button>
            </div>
          </div>
          <div className={step.kind === 'applicant' ? 'grid gap-3' : 'grid grid-cols-2 gap-3'}>
            <div className="min-w-0 space-y-2">
              <Label htmlFor={`approver-kind-${String(index)}`}>处理人类型</Label>
              <SearchSelect
                id={`approver-kind-${String(index)}`}
                label={`第 ${String(index + 1)} 级处理人类型`}
                options={approverKinds}
                value={step.kind}
                defaultValue={step.kind}
                className="w-full"
                onValueChange={(kind) => {
                  onChange(
                    steps.map((current, position) =>
                      position === index
                        ? {
                            ...emptyStep,
                            kind: kind as ApprovalStep['kind'],
                            allow_edit: current.allow_edit,
                            reviewer_label: '',
                          }
                        : current,
                    ),
                  );
                }}
              />
            </div>
            {step.kind !== 'applicant' ? (
              <div className="min-w-0 space-y-2">
                <Label htmlFor={`approver-sign-${String(index)}`}>签署方式</Label>
                <SearchSelect
                  id={`approver-sign-${String(index)}`}
                  label="签署方式"
                  className="w-full"
                  options={approvalSignModes}
                  value={step.sign_mode}
                  defaultValue={step.sign_mode}
                  onValueChange={(sign_mode) => {
                    onChange(
                      steps.map((current, position) =>
                        position === index
                          ? { ...current, sign_mode: sign_mode as ApprovalStep['sign_mode'] }
                          : current,
                      ),
                    );
                  }}
                />
              </div>
            ) : null}
          </div>
          {step.kind !== 'applicant' ? (
            <ApproverSelector
              key={`${String(index)}:${step.kind}`}
              workspaceID={workspaceID}
              step={step}
              index={index}
              onChange={(next) => {
                onChange(steps.map((current, position) => (position === index ? next : current)));
              }}
            />
          ) : null}
          <label className="flex items-center gap-2 text-sm">
            <Checkbox
              aria-label={`第 ${String(index + 1)} 级允许编辑申报内容`}
              checked={step.allow_edit}
              onChange={(event) => {
                const allow_edit = event.target.checked;
                onChange(
                  steps.map((current, position) =>
                    position === index ? { ...current, allow_edit } : current,
                  ),
                );
              }}
            />
            允许编辑申报内容
          </label>
        </div>
      ))}
      {steps.length === 0 ? (
        <p className="text-sm text-muted-foreground">尚未配置审批节点</p>
      ) : null}
    </section>
  );
}

function ApproverSelector({
  workspaceID,
  step,
  index,
  onChange,
}: {
  workspaceID: string;
  step: ApprovalStep;
  index: number;
  onChange: (step: ApprovalStep) => void;
}) {
  const [keyword, setKeyword] = useState('');
  const query = useQuery(approvalOptions(workspaceID, step.kind, keyword));
  const values =
    step.kind === 'class_position'
      ? step.class_positions
      : (step.kind === 'student' ? step.student_ids : step.user_ids).map(String);
  const [initialValues] = useState(values);
  const options = [...(query.data?.items ?? [])];
  for (const option of step.reviewer_options) {
    if (!options.some((item) => item.value === option.value)) options.push(option);
  }
  for (const value of values) {
    if (!options.some((item) => item.value === value)) options.push({ value, label: value });
  }
  const label = approverKinds.find((kind) => kind.value === step.kind)?.label ?? '处理人';
  return (
    <div className="space-y-2">
      <Label htmlFor={`approver-person-${String(index)}`}>{label}</Label>
      <SearchSelect
        id={`approver-person-${String(index)}`}
        label={`第 ${String(index + 1)} 级${label}`}
        options={options}
        multiple
        showAllOption={false}
        placeholder={`请选择${label}`}
        value={values}
        defaultValue={initialValues}
        className="w-full"
        loading={query.isFetching}
        onSearchChange={setKeyword}
        onValueChange={(selected) => {
          if (selected.length > getConfig().projectMaxApprovalTargets) {
            showMessage(`每级最多选择 ${String(getConfig().projectMaxApprovalTargets)} 个处理人`);
            return;
          }
          onChange({
            ...step,
            class_positions: step.kind === 'class_position' ? selected : [],
            student_ids: step.kind === 'student' ? selected.map(Number) : [],
            user_ids: step.kind === 'user' ? selected.map(Number) : [],
            reviewer_label: selected
              .map((value) => options.find((item) => item.value === value)?.label ?? value)
              .join('、'),
            reviewer_options: selected.map((value) => ({
              value,
              label: options.find((item) => item.value === value)?.label ?? value,
            })),
          });
        }}
      />
      {query.error ? <ErrorMessage>{query.error.message}</ErrorMessage> : null}
    </div>
  );
}
