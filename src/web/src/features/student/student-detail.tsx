import { useQuery } from '@tanstack/react-query';

import { DetailValue } from '@/components/detail-value';
import { ErrorMessage } from '@/components/error-message';
import {
  emptyOffCampus,
  type SensitiveData,
  type Student,
  type StudentDetails,
  studentSensitiveOptions,
} from '@/features/student/api';
import { fieldLabel, type StudentRecordTab } from '@/features/student/fields';
import { StudentAvatar } from '@/features/student/student-avatar';
import { StudentFieldSections } from '@/features/student/student-field-sections';
import { StudentOffCampusDetails } from '@/features/student/student-off-campus';

export function StudentDetail({
  student,
  tab,
  sensitive,
}: {
  student: StudentDetails;
  tab: StudentRecordTab;
  sensitive?: SensitiveData;
}) {
  const privateValues = Object.fromEntries(
    sensitive?.values.map((item) => [item.field, item.value]) ?? [],
  );
  return (
    <div className="divide-y divide-border">
      {tab.value === 'basic' ? (
        <div className="py-4">
          <StudentAvatar name={student.name} src={student.avatar_url} />
        </div>
      ) : null}
      <StudentFieldSections
        group={tab.group}
        {...(tab.sections.length ? { sectionIDs: tab.sections } : {})}
        detailTitleValue={(field) => fieldLabel(field, String(student[field.key as keyof Student]))}
        accommodationDetails={
          tab.value === 'contact' && student.accommodation_type === 'off_campus' ? (
            <StudentOffCampusDetails data={student.off_campus_details ?? emptyOffCampus} />
          ) : null
        }
      >
        {(field) => {
          const isSensitive = field.group === 'sensitive';
          const value = isSensitive
            ? (privateValues[field.key] ?? '')
            : String(student[field.key as keyof Student]);
          return (
            <div
              className={
                isSensitive && (field.maxLength > 500 || field.key === 'home_address')
                  ? 'min-w-0 space-y-1.5 sm:col-span-2'
                  : 'min-w-0 space-y-1.5'
              }
            >
              <dt className="text-xs text-muted-foreground">
                {field.labelInSection ?? field.label}
              </dt>
              <DetailValue
                label={field.label}
                value={fieldLabel(field, value)}
                copyable={value !== ''}
              />
            </div>
          );
        }}
      </StudentFieldSections>
    </div>
  );
}

export function StudentSensitiveDetail({
  workspaceID,
  student,
  tab,
}: {
  workspaceID: string;
  student: StudentDetails;
  tab: StudentRecordTab;
}) {
  const sensitive = useQuery(studentSensitiveOptions(workspaceID, student.id, student.revision));
  if (sensitive.isError) return <ErrorMessage>{sensitive.error.message}</ErrorMessage>;
  if (sensitive.isPending) {
    return (
      <p role="status" className="py-12 text-center text-sm text-muted-foreground">
        正在读取敏感信息
      </p>
    );
  }
  return <StudentDetail student={student} tab={tab} sensitive={sensitive.data} />;
}
