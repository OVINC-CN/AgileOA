import { DetailSection } from '@/components/detail-section';
import { DetailValue } from '@/components/detail-value';
import { type Person, stageLabel } from '@/features/cpc-person/api';

const sections = [
  {
    title: '党建信息',
    fields: [
      ['stage', '发展阶段'],
      ['status', '状态'],
      ['party_branch_short_name', '所属党支部'],
      ['party_branch_secretaries', '支部书记'],
      ['party_position', '党内职务'],
    ],
  },
  {
    title: '学籍信息',
    fields: [
      ['student_no', '学号'],
      ['name', '姓名'],
      ['grade', '年级'],
      ['student_status', '学生状态'],
      ['education_level', '学历'],
      ['class_name', '班级'],
      ['major', '专业'],
      ['advisor', '导师'],
      ['arrival_month', '来校时间'],
    ],
  },
  {
    title: '个人与联系方式',
    fields: [
      ['gender', '性别'],
      ['ethnicity', '民族'],
      ['phone', '联系电话'],
    ],
  },
  {
    title: '入学前信息',
    fields: [
      ['previous_school_or_employer', '毕（肄）业学校及单位'],
      ['previous_graduation_month', '毕（肄）业时间'],
    ],
  },
] as const satisfies readonly {
  title: string;
  fields: readonly (readonly [keyof Person, string])[];
}[];

export function PersonDetail({ person }: { person: Person }) {
  const values = {
    ...person,
    stage: stageLabel(person.stage),
    party_branch_secretaries: person.party_branch_secretaries.join('、'),
  };
  return (
    <div className="divide-y divide-border px-6">
      {sections.map((section) => (
        <DetailSection key={section.title} title={section.title}>
          <dl className="grid grid-cols-1 gap-5 sm:grid-cols-2">
            {section.fields.map(([key, label]) => {
              const value = values[key];
              return (
                <div
                  key={key}
                  className={
                    key === 'previous_school_or_employer'
                      ? 'min-w-0 space-y-1.5 sm:col-span-2'
                      : 'min-w-0 space-y-1.5'
                  }
                >
                  <dt className="text-xs text-muted-foreground">{label}</dt>
                  <DetailValue label={label} value={value} copyable={value.trim() !== ''} />
                </div>
              );
            })}
          </dl>
        </DetailSection>
      ))}
    </div>
  );
}
