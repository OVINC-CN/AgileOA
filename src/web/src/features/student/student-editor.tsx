import { useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, ArrowRight, RefreshCw, Save, Trash2 } from 'lucide-react';
import { useEffect, useId, useLayoutEffect, useRef, useState } from 'react';

import { ErrorMessage } from '@/components/error-message';
import { Button } from '@/components/ui/button';
import { Drawer } from '@/components/ui/drawer';
import { DrawerSteps } from '@/components/ui/drawer-steps';
import { DrawerTabs } from '@/components/ui/drawer-tabs';
import { FileUploadButton } from '@/components/ui/file-upload-button';
import { personKey } from '@/features/cpc-person/api';
import { graduationKey } from '@/features/graduation/api';
import {
  type AvatarUpload,
  createStudent,
  deleteAvatarUpload,
  deleteStudent,
  emptyOffCampus,
  emptyStudent,
  type OffCampusValues,
  type SensitiveData,
  type Student,
  studentDetailOptions,
  type StudentInput,
  studentKey,
  studentSensitiveOptions,
  StudentValidationError,
  updateStudent,
  updateStudentSensitive,
  uploadStudentAvatar,
} from '@/features/student/api';
import { StudentFieldInput } from '@/features/student/field-input';
import {
  birthDateFromIdentityNumber,
  fieldLabel,
  type StudentField,
  studentFields,
  studentRecordTabs,
  studentSections,
} from '@/features/student/fields';
import { StudentAvatar } from '@/features/student/student-avatar';
import { StudentFieldSections } from '@/features/student/student-field-sections';
import { StudentOffCampusFields } from '@/features/student/student-off-campus';
import { useApiAction, useApiMutation } from '@/lib/api-action';
import { getConfig } from '@/lib/config';
import { uploadStateLabels } from '@/lib/file-upload';
import { formatIdentity } from '@/lib/format';

interface StudentEditorStep {
  value: string;
  label: string;
  group: StudentField['group'];
  sections: readonly string[];
}

const studentCreationSteps: readonly StudentEditorStep[] = [
  { value: 'basic', label: '学籍资料', group: 'basic', sections: ['academic'] },
  {
    value: 'personal',
    label: '个人经历',
    group: 'basic',
    sections: ['personal', 'previous-education', 'advisor'],
  },
  { value: 'contact', label: '联系家庭', group: 'basic', sections: ['contact', 'guardians'] },
  { value: 'accommodation', label: '住宿信息', group: 'basic', sections: ['accommodation'] },
  { value: 'sensitive', label: '敏感信息', group: 'sensitive', sections: [] },
];

export function StudentEditor({
  workspaceID,
  student,
  canQuery,
  canBasic,
  canDetail,
  canSensitive,
  onClose,
}: {
  workspaceID: string;
  student: Student | null;
  canQuery: boolean;
  canBasic: boolean;
  canDetail: boolean;
  canSensitive: boolean;
  onClose: () => void;
}) {
  const sensitive = useQuery({
    ...studentSensitiveOptions(workspaceID, student?.id ?? 0),
    enabled: student !== null && canSensitive,
  });
  const detail = useQuery({
    ...studentDetailOptions(workspaceID, student?.id ?? 0),
    enabled: student !== null && canDetail,
  });
  const retry = useApiAction();
  if (!canBasic && !canSensitive) return null;
  const canReadOffCampus = canBasic && canDetail && student !== null;
  const needsOffCampus = canReadOffCampus && student.accommodation_type === 'off_campus';
  const pendingReads = [
    ...(canSensitive && !sensitive.data ? [sensitive] : []),
    ...(needsOffCampus && !detail.data ? [detail] : []),
  ];
  if (student && pendingReads.length > 0) {
    return (
      <Drawer
        title="编辑学生"
        truncateDescription
        description={formatIdentity(student.name, student.student_no)}
        onClose={onClose}
        footerRefresh={
          <Button
            variant="outline"
            loading={pendingReads.some((read) => read.isFetching) || retry.pending}
            onClick={() => {
              void retry.run('retry', () =>
                Promise.all(pendingReads.map((read) => read.refetch())),
              );
            }}
          >
            <RefreshCw aria-hidden="true" />
            重新读取
          </Button>
        }
      >
        <div className="space-y-4 p-6">
          <ErrorMessage>{pendingReads.find((read) => read.error)?.error?.message}</ErrorMessage>
        </div>
      </Drawer>
    );
  }
  return (
    <StudentEditorForm
      key={student?.id ?? 'new'}
      workspaceID={workspaceID}
      student={needsOffCampus ? (detail.data ?? student) : student}
      canQuery={canQuery}
      canBasic={canBasic}
      avatarURL={canDetail && !detail.isError ? (detail.data?.avatar_url ?? '') : undefined}
      avatarLoading={canDetail && detail.isPending && student !== null}
      avatarUploads={canDetail && !detail.isError ? (detail.data?.avatar_uploads ?? []) : []}
      canSensitive={canSensitive}
      sensitive={sensitive.data}
      canReadOffCampus={canReadOffCampus}
      offCampusDetails={needsOffCampus ? detail.data?.off_campus_details : undefined}
      onClose={onClose}
    />
  );
}

function StudentEditorForm({
  workspaceID,
  student,
  canQuery,
  canBasic,
  avatarURL,
  avatarLoading,
  avatarUploads,
  canSensitive,
  sensitive,
  canReadOffCampus,
  offCampusDetails,
  onClose,
}: {
  workspaceID: string;
  student: Student | null;
  canQuery: boolean;
  canBasic: boolean;
  avatarURL: string | undefined;
  avatarLoading: boolean;
  avatarUploads: AvatarUpload[];
  canSensitive: boolean;
  sensitive: SensitiveData | undefined;
  canReadOffCampus: boolean;
  offCampusDetails: OffCampusValues | undefined;
  onClose: () => void;
}) {
  const [originalStudent] = useState(student);
  const [tab, setTab] = useState(canBasic ? 'basic' : 'sensitive');
  const [invalidField, setInvalidField] = useState<StudentValidationError | null>(null);
  const tabsID = useId();
  const fieldset = useRef<HTMLFieldSetElement>(null);
  const creating = originalStudent === null;
  const tabs: StudentEditorStep[] = (creating ? studentCreationSteps : studentRecordTabs).filter(
    (step) => (step.group === 'basic' ? canBasic : canSensitive),
  );
  const activeTab = tabs.some((item) => item.value === tab) ? tab : (tabs[0]?.value ?? 'basic');
  const activeIndex = tabs.findIndex((item) => item.value === activeTab);
  const finalStep = activeIndex === tabs.length - 1;
  useLayoutEffect(() => {
    const content = fieldset.current?.parentElement;
    if (content) content.scrollTop = 0;
  }, [activeTab]);
  useEffect(() => {
    if (!invalidField) return;
    fieldset.current
      ?.querySelector<HTMLElement>(
        `[data-student-field="${invalidField.field}"] input, [data-student-field="${invalidField.field}"] textarea, [data-student-field="${invalidField.field}"] button[role="combobox"]`,
      )
      ?.focus();
  }, [invalidField, activeTab]);
  const [revision, setRevision] = useState(
    (canBasic ? student?.revision : sensitive?.revision) ?? student?.revision ?? 0,
  );
  const [values, setValues] = useState<StudentInput>(student ?? emptyStudent);
  const [offCampus, setOffCampus] = useState(offCampusDetails ?? emptyOffCampus);
  const showOffCampus =
    canReadOffCampus &&
    (originalStudent?.accommodation_type === 'off_campus' ||
      values.accommodation_type === 'off_campus');
  const fields = studentFields.filter((field) => field.group === 'sensitive');
  const [privateValues, setPrivateValues] = useState<Record<string, string>>(() =>
    Object.fromEntries(sensitive?.values.map((item) => [item.field, item.value]) ?? []),
  );
  const birthDate = canSensitive
    ? birthDateFromIdentityNumber(privateValues['identity_number'] ?? '')
    : (originalStudent?.birth_date ?? '');
  const client = useQueryClient();
  const avatarUpload = useApiAction();
  const avatarMaxUploadBytes = getConfig().fileMaxUploadBytes;
  const invalidate = () =>
    Promise.all([
      client.invalidateQueries({ queryKey: studentKey(workspaceID) }),
      client.invalidateQueries({ queryKey: personKey(workspaceID) }),
      client.invalidateQueries({ queryKey: graduationKey(workspaceID) }),
      client.invalidateQueries({ queryKey: ['management', 'audit'] }),
    ]);
  const locateInvalidField = (cause: StudentValidationError) => {
    setInvalidField(cause);
    const section = studentSections.basic.find((item) => item.fields.includes(cause.field));
    setTab(
      studentFields.find((field) => field.key === cause.field)?.group === 'sensitive'
        ? 'sensitive'
        : (tabs.find((step) => section && step.sections.includes(section.id))?.value ?? 'basic'),
    );
  };
  const mutation = useApiMutation({
    mutationFn: async () => {
      const sensitiveValues = canSensitive
        ? fields.map((field) => ({ field: field.key, value: privateValues[field.key] ?? '' }))
        : undefined;
      const result = !originalStudent
        ? await createStudent(workspaceID, values, sensitiveValues)
        : canBasic
          ? await updateStudent(
              workspaceID,
              { id: originalStudent.id, revision },
              values,
              sensitiveValues,
              showOffCampus ? offCampus : undefined,
            )
          : await updateStudentSensitive(
              workspaceID,
              originalStudent.id,
              revision,
              sensitiveValues ?? [],
            );
      await invalidate();
      return result;
    },
    onSuccess: onClose,
    onError: (cause) => {
      if (!(cause instanceof StudentValidationError)) return;
      locateInvalidField(cause);
    },
  });
  const busy = mutation.isPending || avatarUpload.pending;
  const changeStep = (next: string) => {
    if (busy) return;
    if (!tabs.some((item) => item.value === next)) return;
    // 步骤只切换填写区域，全部草稿在保存时统一校验。
    setTab(next);
  };
  return (
    <Drawer
      title={student ? '编辑学生' : '添加学生'}
      truncateDescription={!!student}
      description={
        student
          ? formatIdentity(student.name, student.student_no)
          : `第 ${String(activeIndex + 1)} / ${String(tabs.length)} 步`
      }
      busy={busy}
      onClose={onClose}
      headerNavigation={
        creating ? (
          <DrawerSteps
            id={tabsID}
            label="添加学生步骤"
            value={activeTab}
            items={tabs}
            onChange={changeStep}
            disabled={busy}
          />
        ) : undefined
      }
      footerNavigation={
        creating ? undefined : (
          <DrawerTabs
            id={tabsID}
            label="学生信息分区"
            value={activeTab}
            items={tabs}
            onChange={changeStep}
            disabled={busy}
          />
        )
      }
      footerDismiss={
        <Button type="button" variant="cancel" disabled={busy} onClick={onClose}>
          取消
        </Button>
      }
      footer={
        <>
          {creating && activeIndex > 0 ? (
            <Button
              type="button"
              variant="outline"
              disabled={busy}
              onClick={() => {
                changeStep(tabs[activeIndex - 1]?.value ?? activeTab);
              }}
            >
              <ArrowLeft aria-hidden="true" />
              上一步
            </Button>
          ) : null}
          <Button type="submit" loading={mutation.isPending} disabled={avatarUpload.pending}>
            {creating && !finalStep ? (
              <ArrowRight aria-hidden="true" />
            ) : (
              <Save aria-hidden="true" />
            )}
            {creating ? (finalStep ? '保存学生' : '下一步') : '保存'}
          </Button>
        </>
      }
      form={{
        noValidate: true,
        onSubmit: (event) => {
          event.preventDefault();
          if (busy) return;
          if (creating && !finalStep) {
            changeStep(tabs[activeIndex + 1]?.value ?? activeTab);
          } else {
            mutation.mutate();
          }
        },
      }}
    >
      <fieldset ref={fieldset} disabled={busy} className="px-5">
        {originalStudent && (avatarURL !== undefined || canBasic) ? (
          <section
            className="flex flex-wrap items-center gap-4 border-b border-border py-5"
            hidden={activeTab !== 'basic'}
          >
            {avatarURL !== undefined ? (
              <StudentAvatar name={originalStudent.name} src={avatarURL} loading={avatarLoading} />
            ) : null}
            {canBasic ? (
              <div className="space-y-2">
                <FileUploadButton
                  label="上传头像"
                  accept="image/jpeg,image/png,image/webp"
                  disabled={mutation.isPending}
                  loading={avatarUpload.pending}
                  onFileSelect={(file, updateUpload) => {
                    void avatarUpload.run('avatar', async () => {
                      // 上传前校验头像大小，避免发送空文件或超限文件。
                      if (file.size < 1) throw new Error('头像文件不能为空');
                      if (file.size > avatarMaxUploadBytes)
                        throw new Error('头像文件超过上传大小限制');

                      // 上传成功后更新版本并刷新相关数据。
                      try {
                        const result = await uploadStudentAvatar(
                          workspaceID,
                          { id: originalStudent.id, revision },
                          file,
                          updateUpload,
                        );
                        setRevision(result.revision);
                      } finally {
                        await invalidate();
                      }
                      updateUpload('complete');
                    });
                  }}
                />
                {avatarUploads.map((upload) => (
                  <div
                    key={upload.id}
                    className="flex items-center gap-2 text-xs text-muted-foreground"
                  >
                    <span className="min-w-0 truncate">{upload.filename}</span>
                    <span className="shrink-0">{uploadStateLabels[upload.state]}</span>
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      disabled={busy}
                      loading={avatarUpload.isPending(`delete:${String(upload.id)}`)}
                      onClick={() => {
                        void avatarUpload.run(`delete:${String(upload.id)}`, async () => {
                          try {
                            await deleteAvatarUpload(workspaceID, originalStudent.id, upload);
                          } finally {
                            await invalidate();
                          }
                        });
                      }}
                    >
                      <Trash2 aria-hidden="true" />
                      删除
                    </Button>
                  </div>
                ))}
                <p className="text-xs text-muted-foreground">
                  支持 JPEG、PNG、WebP，大小不超过 {avatarMaxUploadBytes / (1024 * 1024)}{' '}
                  MiB，上传后立即更新。
                </p>
              </div>
            ) : null}
          </section>
        ) : null}
        {tabs.map((step) => (
          <section
            key={step.value}
            role="tabpanel"
            id={`${tabsID}-panel-${step.value}`}
            aria-labelledby={`${tabsID}-tab-${step.value}`}
            hidden={activeTab !== step.value}
            className="space-y-5 py-5"
          >
            {step.group === 'sensitive' && !creating ? (
              <p className="text-xs text-muted-foreground">
                身份证号码必填；保存会更新全部敏感信息，其他字段留空会清除对应内容。
              </p>
            ) : null}
            <StudentFieldSections
              group={step.group}
              form
              {...(step.sections.length > 0 ? { sectionIDs: step.sections } : {})}
            >
              {(field) => {
                if (field.key === 'birth_date') {
                  return (
                    <div className="space-y-2">
                      <p className="text-sm font-medium">{field.label}</p>
                      <p className="text-sm">{fieldLabel(field, birthDate)}</p>
                      <p className="text-xs text-muted-foreground">由身份证号码生成</p>
                    </div>
                  );
                }
                const sensitiveField = field.group === 'sensitive';
                const input = (
                  <StudentFieldInput
                    field={field}
                    value={
                      sensitiveField
                        ? (privateValues[field.key] ?? '')
                        : values[field.key as keyof StudentInput]
                    }
                    workspaceID={workspaceID}
                    canQuery={sensitiveField ? canSensitive : canQuery}
                    invalid={invalidField?.field === field.key}
                    onChange={(value) => {
                      if (invalidField?.field === field.key) setInvalidField(null);
                      if (sensitiveField) {
                        setPrivateValues((current) => ({ ...current, [field.key]: value }));
                      } else {
                        setValues((current) => ({ ...current, [field.key]: value }));
                      }
                    }}
                  />
                );
                return sensitiveField ? (
                  <div
                    className={
                      field.maxLength > 500 ||
                      field.key === 'identity_number' ||
                      field.key === 'home_address'
                        ? 'sm:col-span-2'
                        : ''
                    }
                  >
                    {input}
                  </div>
                ) : (
                  input
                );
              }}
            </StudentFieldSections>
            {showOffCampus && step.sections.includes('accommodation') ? (
              <section className="space-y-5 py-6">
                <div className="space-y-2">
                  <h2 className="text-sm font-medium">校外住宿信息</h2>
                  {values.accommodation_type !== 'off_campus' ? (
                    <p className="text-xs text-muted-foreground">
                      切换住宿类型前，请清空校外住宿明细。
                    </p>
                  ) : null}
                </div>
                <StudentOffCampusFields
                  values={offCampus}
                  onChange={(key, value) => {
                    setOffCampus((current) => ({ ...current, [key]: value }));
                  }}
                />
              </section>
            ) : null}
          </section>
        ))}
      </fieldset>
    </Drawer>
  );
}

export function DeleteStudent({
  workspaceID,
  student,
  onClose,
}: {
  workspaceID: string;
  student: Student;
  onClose: () => void;
}) {
  const client = useQueryClient();
  const mutation = useApiMutation({
    mutationFn: async () => {
      await deleteStudent(workspaceID, student);
      await Promise.all([
        client.invalidateQueries({ queryKey: studentKey(workspaceID) }),
        client.invalidateQueries({ queryKey: graduationKey(workspaceID) }),
        client.invalidateQueries({ queryKey: ['management', 'audit'] }),
      ]);
    },
    onSuccess: onClose,
  });
  return (
    <Drawer
      title="删除学生"
      truncateDescription
      description={formatIdentity(student.name, student.student_no)}
      busy={mutation.isPending}
      onClose={onClose}
      footerDismiss={
        <Button variant="cancel" disabled={mutation.isPending} onClick={onClose}>
          取消
        </Button>
      }
      footer={
        <Button
          loading={mutation.isPending}
          onClick={() => {
            mutation.mutate();
          }}
        >
          <Trash2 aria-hidden="true" />
          确认删除
        </Button>
      }
    >
      <div className="space-y-4 p-6 text-sm">
        <p>
          将永久删除该学生档案、隐私信息、毕业去向、校外住宿、获奖及党建记录，并删除相关会话、时间线和附件记录；历史审计保留。存在申报或审批节点引用时不可删除。
        </p>
        {mutation.error ? <ErrorMessage>{mutation.error.message}</ErrorMessage> : null}
      </div>
    </Drawer>
  );
}
