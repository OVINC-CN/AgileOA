import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Paperclip, Save } from 'lucide-react';
import { useId, useState } from 'react';

import { ErrorMessage } from '@/components/error-message';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { DateTimePicker } from '@/components/ui/date-time-picker';
import { Drawer } from '@/components/ui/drawer';
import { DrawerSteps } from '@/components/ui/drawer-steps';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { SearchSelect } from '@/components/ui/search-select';
import { TagInput } from '@/components/ui/tag-input';
import { Textarea } from '@/components/ui/textarea';
import {
  type Project,
  projectFilterOptions,
  type ProjectInput,
  projectKey,
  saveProject,
} from '@/features/project/api';
import { ApprovalStepEditor } from '@/features/project/approval-step-editor';
import { AttachmentManager } from '@/features/project/attachments';
import { cleanScoreCalculation, emptyScoreCalculation } from '@/features/project/score-calculation';
import { ScoreCalculationEditor } from '@/features/project/score-calculation-editor';
import { ApiError } from '@/lib/api';
import { useApiMutation } from '@/lib/api-action';

export function ProjectEditor({
  workspaceID,
  project,
  actions,
  onClose,
}: {
  workspaceID: string;
  project: Project | null;
  actions: string[];
  onClose: () => void;
}) {
  const [original] = useState(project);
  const [values, setValues] = useState<ProjectInput>(
    project ?? {
      name: '',
      year: new Date().getFullYear(),
      description: '',
      detail_names: [],
      allow_self_apply: false,
      self_apply_deadline: null,
      approval_steps: [],
    },
  );
  const [tab, setTab] = useState('main');
  const tabsID = useId();
  const [attachments, setAttachments] = useState(false);
  const [namesOpen, setNamesOpen] = useState(false);
  const canList = actions.includes('project.list');
  const names = useQuery({
    ...projectFilterOptions(workspaceID, false),
    enabled: canList && namesOpen,
  });
  const namesDenied =
    names.error instanceof ApiError && [401, 403, 404].includes(names.error.status);
  const nameOptions = [...(canList && !namesDenied ? (names.data?.names ?? []) : [])];
  if (values.name && !nameOptions.some((item) => item.value === values.name)) {
    nameOptions.push({ value: values.name, label: values.name });
  }
  const client = useQueryClient();
  const mutation = useApiMutation({
    mutationFn: async () => {
      if (!values.name.trim()) {
        setTab('main');
        throw new Error('请填写项目名称');
      }
      if (!Number.isInteger(values.year) || values.year < 1900 || values.year > 9999) {
        setTab('main');
        throw new Error('归属年份须为 1900 至 9999 的整数');
      }
      if (values.allow_self_apply && !values.self_apply_deadline) {
        setTab('application');
        throw new Error('允许学生自行申报时必须填写截止时间');
      }
      try {
        cleanScoreCalculation(values.score_calculation);
      } catch (error) {
        setTab('scoring');
        throw error;
      }
      const result = await saveProject(workspaceID, original, values);
      await client.invalidateQueries({ queryKey: projectKey(workspaceID) });
      return result;
    },
    onSuccess: onClose,
  });
  return (
    <>
      <Drawer
        title={project ? '编辑项目' : '新建项目'}
        description="维护项目名称、归属年份、说明及评分子项配置"
        busy={mutation.isPending}
        contentClassName="flex flex-col overflow-hidden"
        onClose={onClose}
        headerNavigation={
          <DrawerSteps
            id={tabsID}
            label="项目内容分类"
            value={tab}
            items={[
              { value: 'main', label: '主要信息' },
              { value: 'application', label: '申报设置' },
              { value: 'scoring', label: '分数计算' },
            ]}
            disabled={mutation.isPending}
            onChange={setTab}
          />
        }
        footer={
          <>
            <Button
              type="button"
              variant="secondary"
              disabled={mutation.isPending}
              onClick={onClose}
            >
              取消
            </Button>
            <Button type="submit" loading={mutation.isPending}>
              <Save aria-hidden="true" />
              保存
            </Button>
          </>
        }
        form={{
          noValidate: true,
          onSubmit: (event) => {
            event.preventDefault();
            mutation.mutate();
          },
        }}
      >
        <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain">
          <section
            role="tabpanel"
            id={`${tabsID}-panel-scoring`}
            aria-labelledby={`${tabsID}-tab-scoring`}
            hidden={tab !== 'scoring'}
            className="p-6"
          >
            <ScoreCalculationEditor
              workspaceID={workspaceID}
              projectID={original?.id}
              value={values.score_calculation ?? emptyScoreCalculation()}
              names={values.detail_names}
              disabled={mutation.isPending}
              onChange={(score_calculation) => {
                setValues((current) => ({ ...current, score_calculation }));
              }}
            />
          </section>
          <section
            role="tabpanel"
            id={`${tabsID}-panel-main`}
            aria-labelledby={`${tabsID}-tab-main`}
            hidden={tab !== 'main'}
          >
            <fieldset disabled={mutation.isPending} className="grid gap-5 p-6 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="project-name">项目名称</Label>
                <SearchSelect
                  id="project-name"
                  label="项目名称"
                  className="w-full"
                  options={nameOptions}
                  allowCreate
                  maxLength={100}
                  value={values.name}
                  onValueChange={(name) => {
                    setValues((current) => ({ ...current, name }));
                  }}
                  onOpenChange={setNamesOpen}
                  loading={names.isFetching}
                />
                {canList && names.error ? <ErrorMessage>{names.error.message}</ErrorMessage> : null}
              </div>
              <div className="space-y-2">
                <Label htmlFor="project-year">归属年份</Label>
                <Input
                  id="project-year"
                  required
                  type="number"
                  min={1900}
                  max={9999}
                  value={values.year || ''}
                  onChange={(event) => {
                    setValues((current) => ({ ...current, year: Number(event.target.value) }));
                  }}
                />
              </div>
              <div className="space-y-2 sm:col-span-2">
                <Label htmlFor="project-description">项目说明</Label>
                <Textarea
                  id="project-description"
                  rows={6}
                  maxLength={10000}
                  value={values.description}
                  onChange={(event) => {
                    setValues((current) => ({ ...current, description: event.target.value }));
                  }}
                />
              </div>
              <div className="space-y-2 sm:col-span-2">
                <Label htmlFor="project-detail-names">可选子项名称</Label>
                <TagInput
                  id="project-detail-names"
                  label="可选子项名称"
                  values={values.detail_names}
                  maxLength={100}
                  onValuesChange={(detail_names) => {
                    setValues((current) => ({ ...current, detail_names }));
                  }}
                />
                <p className="text-xs text-muted-foreground">
                  输入后按回车添加，每项最多 100
                  个字符。参与评分只能选择已配置的名称，已被引用的名称不能移除。
                </p>
              </div>
              <div className="space-y-3 border-t border-border pt-5 sm:col-span-2">
                <div className="flex items-center justify-between">
                  <Label>项目附件</Label>
                  {actions.includes('project.attachment.read') ||
                  actions.includes('project.attachment.upload') ? (
                    <Button
                      type="button"
                      variant="outline"
                      disabled={!original}
                      onClick={() => {
                        setAttachments(true);
                      }}
                    >
                      <Paperclip aria-hidden="true" />
                      管理附件
                    </Button>
                  ) : null}
                </div>
                {original ? null : (
                  <p className="text-xs text-muted-foreground">创建项目后可上传附件</p>
                )}
              </div>
            </fieldset>
          </section>
          <section
            role="tabpanel"
            id={`${tabsID}-panel-application`}
            aria-labelledby={`${tabsID}-tab-application`}
            hidden={tab !== 'application'}
          >
            <fieldset disabled={mutation.isPending} className="p-6">
              <div className="space-y-4">
                <label className="flex items-center gap-2 text-sm">
                  <Checkbox
                    checked={values.allow_self_apply}
                    onChange={(event) => {
                      const allow_self_apply = event.target.checked;
                      setValues((current) => ({ ...current, allow_self_apply }));
                    }}
                  />
                  允许学生自行申报
                </label>
                {values.allow_self_apply ? (
                  <>
                    <div className="space-y-2">
                      <Label htmlFor="project-self-apply-deadline" className="font-normal">
                        自行申报截止时间
                      </Label>
                      <DateTimePicker
                        id="project-self-apply-deadline"
                        label="自行申报截止时间"
                        value={values.self_apply_deadline}
                        required
                        disabled={mutation.isPending}
                        onValueChange={(self_apply_deadline) => {
                          setValues((current) => ({ ...current, self_apply_deadline }));
                        }}
                      />
                    </div>
                    <ApprovalStepEditor
                      workspaceID={workspaceID}
                      steps={values.approval_steps}
                      onChange={(approval_steps) => {
                        setValues((current) => ({ ...current, approval_steps }));
                      }}
                    />
                    <p className="text-xs text-muted-foreground">
                      调整审批级别、处理人或签署方式后，未完成申报将清空有效签署并从第一级重新审批；仅调整编辑开关保留进度和签署，草稿仍可编辑，已通过申报保持只读。
                    </p>
                  </>
                ) : null}
              </div>
            </fieldset>
          </section>
        </div>
      </Drawer>
      {attachments && original ? (
        <AttachmentManager
          workspaceID={workspaceID}
          projectID={original.id}
          actions={actions}
          onClose={() => {
            setAttachments(false);
          }}
        />
      ) : null}
    </>
  );
}
