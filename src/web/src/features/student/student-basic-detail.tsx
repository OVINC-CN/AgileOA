import { DetailSection } from '@/components/detail-section';
import { DetailValue } from '@/components/detail-value';
import { emptyOffCampus, type Student, type StudentDetails } from '@/features/student/api';
import { fieldLabel } from '@/features/student/fields';
import { StudentAvatar } from '@/features/student/student-avatar';
import { StudentFieldSections } from '@/features/student/student-field-sections';
import { StudentOffCampusDetails } from '@/features/student/student-off-campus';

export function StudentBasicDetail({ student }: { student: StudentDetails }) {
  return (
    <DetailSection title="基本信息">
      <StudentAvatar name={student.name} src={student.avatar_url} />
      <StudentFieldSections
        group="basic"
        accommodationDetails={
          student.accommodation_type === 'off_campus' ? (
            <StudentOffCampusDetails data={student.off_campus_details ?? emptyOffCampus} />
          ) : null
        }
      >
        {(field) => {
          const value = String(student[field.key as keyof Student]);
          return (
            <div className="min-w-0 space-y-1.5">
              <dt className="text-xs text-muted-foreground">{field.label}</dt>
              <DetailValue
                label={field.label}
                value={fieldLabel(field, value)}
                copyable={value !== ''}
              />
            </div>
          );
        }}
      </StudentFieldSections>
    </DetailSection>
  );
}
