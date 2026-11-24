import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Save } from 'lucide-react';
import { useRef, useState } from 'react';

import { ErrorMessage } from '@/components/error-message';
import { Button } from '@/components/ui/button';
import { Drawer, type DrawerHandle } from '@/components/ui/drawer';
import { Label } from '@/components/ui/label';
import { SearchSelect } from '@/components/ui/search-select';
import {
  type Candidate,
  defaultStatus,
  type Person,
  personCandidateOptions,
  type PersonInput,
  personKey,
  personOptions,
  savePerson,
  stages,
  type StudentInfo,
} from '@/features/cpc-person/api';
import { partyPositionPresets, personStatusPresets } from '@/features/cpc-person/form-presets';
import { TimelineTemplateSelect } from '@/features/cpc-person/timeline-template-select';
import { ApiError } from '@/lib/api';
import { useApiMutation } from '@/lib/api-action';
import { emptyValueLabel, formatIdentity, formatValue } from '@/lib/format';
import { mergeTextFieldOptions } from '@/lib/text-field-options';

const studentLabels = [
  ['student_no', '学号'],
  ['name', '姓名'],
  ['gender', '性别'],
  ['ethnicity', '民族'],
  ['grade', '年级'],
  ['student_status', '学生状态'],
  ['education_level', '学历'],
  ['class_name', '班级'],
  ['major', '专业'],
  ['advisor', '导师'],
  ['phone', '联系电话'],
  ['arrival_month', '来校时间'],
  ['previous_school_or_employer', '毕（肄）业学校及单位'],
  ['previous_graduation_month', '毕（肄）业时间'],
] as const;

export function PersonStudentInfo({ student }: { student: StudentInfo }) {
  return (
    <dl className="grid grid-cols-1 gap-4 text-sm sm:grid-cols-2">
      {studentLabels.map(([key, label]) => (
        <div key={key} className="min-w-0 space-y-1">
          <dt className="text-xs text-muted-foreground">{label}</dt>
          <dd className="wrap-anywhere">{formatValue(student[key])}</dd>
        </div>
      ))}
    </dl>
  );
}

export function PersonEditor({
  workspaceID,
  person,
  onClose,
}: {
  workspaceID: string;
  person: Person | null;
  onClose: () => void;
}) {
  const [original] = useState(person);
  const [values, setValues] = useState<PersonInput>(
    person ?? {
      student_id: 0,
      timeline_template_id: null,
      stage: 'applicant',
      status: defaultStatus.value,
      party_position: '',
    },
  );
  const [keyword, setKeyword] = useState('');
  const [studentOpen, setStudentOpen] = useState(false);
  const [positionOpen, setPositionOpen] = useState(false);
  const [statusOpen, setStatusOpen] = useState(false);
  const [selected, setSelected] = useState<Candidate | null>(null);
  const candidates = useQuery({
    ...personCandidateOptions(workspaceID, keyword),
    enabled: !original && studentOpen,
  });
  const positions = useQuery({
    ...personOptions(workspaceID, 'party_position'),
    enabled: positionOpen,
  });
  const statuses = useQuery({
    ...personOptions(workspaceID, 'status'),
    enabled: statusOpen,
  });
  const client = useQueryClient();
  const drawer = useRef<DrawerHandle>(null);
  const mutation = useApiMutation({
    mutationFn: async () => {
      await savePerson(workspaceID, original, values);
      await Promise.all([
        client.invalidateQueries({ queryKey: personKey(workspaceID) }),
        client.invalidateQueries({ queryKey: ['management', 'audit'] }),
      ]);
    },
    onSuccess: () => {
      drawer.current?.close();
    },
  });
  const blocked = [candidates.error, positions.error, statuses.error].some(
    (error) => error instanceof ApiError && [401, 403, 404].includes(error.status),
  );
  const candidateValues = blocked ? [] : [...(candidates.data ?? [])];
  if (!blocked && selected && !candidateValues.some(({ value }) => value === selected.value)) {
    candidateValues.push(selected);
  }
  const positionValues = blocked
    ? []
    : [
        { value: '', label: emptyValueLabel },
        ...mergeTextFieldOptions(partyPositionPresets, positions.data ?? [], values.party_position),
      ];
  const statusValues = blocked
    ? []
    : mergeTextFieldOptions(personStatusPresets, statuses.data ?? [], values.status);
  const student = original ?? selected;
  const canSave =
    !blocked &&
    values.student_id > 0 &&
    (original !== null || values.timeline_template_id !== null);
  return (
    <Drawer
      ref={drawer}
      title={original ? '编辑党建人员' : '新增党建人员'}
      busy={mutation.isPending}
      onClose={onClose}
      truncateDescription={!!original}
      description={
        original ? formatIdentity(original.name, original.student_no) : '从已有学生中选择人员'
      }
      form={{
        onSubmit: (event) => {
          event.preventDefault();
          if (canSave && !mutation.isPending) mutation.mutate();
        },
      }}
      dismissLabel="取消"
      footer={
        <Button type="submit" loading={mutation.isPending} disabled={!canSave}>
          <Save aria-hidden="true" />
          保存
        </Button>
      }
    >
      <fieldset disabled={mutation.isPending} className="space-y-6 p-6">
        <ErrorMessage>
          {candidates.error?.message ?? positions.error?.message ?? statuses.error?.message}
        </ErrorMessage>
        {!original ? (
          <div className="space-y-2">
            <Label htmlFor="cpc-student">关联学生 *</Label>
            <SearchSelect
              id="cpc-student"
              label="关联学生"
              className="w-full"
              options={candidateValues.map(({ value, label }) => ({ value: String(value), label }))}
              value={values.student_id ? String(values.student_id) : ''}
              loading={candidates.isFetching}
              disabled={blocked}
              onOpenChange={setStudentOpen}
              onSearchChange={setKeyword}
              onValueChange={(value) => {
                const next = candidateValues.find((item) => item.value === Number(value)) ?? null;
                setSelected(next);
                setValues((current) => ({
                  ...current,
                  student_id: next?.value ?? 0,
                }));
              }}
            />
          </div>
        ) : null}
        {student && !blocked ? (
          <section className="space-y-4">
            <h2 className="text-sm font-semibold">学生基础信息</h2>
            <PersonStudentInfo student={student} />
          </section>
        ) : null}
        {!original ? (
          <TimelineTemplateSelect
            workspaceID={workspaceID}
            purpose="create"
            value={values.timeline_template_id}
            disabled={blocked}
            onChange={(value) => {
              setValues((previous) => ({ ...previous, timeline_template_id: value }));
            }}
          />
        ) : null}
        <div className="space-y-2">
          <Label htmlFor="cpc-stage">发展阶段 *</Label>
          <SearchSelect
            id="cpc-stage"
            label="发展阶段"
            className="w-full"
            options={[...stages]}
            value={values.stage}
            disabled={blocked}
            onValueChange={(value) => {
              setValues((current) => ({ ...current, stage: value as PersonInput['stage'] }));
            }}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="cpc-status">状态 *</Label>
          <SearchSelect
            id="cpc-status"
            label="状态"
            className="w-full"
            options={statusValues}
            value={values.status}
            maxLength={8}
            allowCreate
            loading={statuses.isFetching}
            disabled={blocked}
            onOpenChange={setStatusOpen}
            onValueChange={(value) => {
              setValues((current) => ({ ...current, status: value }));
            }}
          />
        </div>
        <div className="space-y-2">
          <p className="text-sm font-medium">所属党支部</p>
          <p className="text-sm wrap-anywhere">
            {student?.party_branch_name ?? '选择学生后由准入规则自动确定'}
          </p>
        </div>
        <div className="space-y-2">
          <Label htmlFor="cpc-position">党内职务</Label>
          <SearchSelect
            id="cpc-position"
            label="党内职务"
            className="w-full"
            options={positionValues}
            value={values.party_position}
            maxLength={64}
            allowCreate
            loading={positions.isFetching}
            disabled={blocked}
            onOpenChange={setPositionOpen}
            onValueChange={(value) => {
              setValues((current) => ({ ...current, party_position: value }));
            }}
          />
        </div>
      </fieldset>
    </Drawer>
  );
}
