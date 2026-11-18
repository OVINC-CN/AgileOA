import type { Ref } from 'react';

import { DetailSection } from '@/components/detail-section';
import { DetailValue } from '@/components/detail-value';
import type { SensitiveData, StudentDetails } from '@/features/student/api';
import { fieldLabel } from '@/features/student/fields';
import { StudentBasicDetail } from '@/features/student/student-basic-detail';
import { StudentFieldSections } from '@/features/student/student-field-sections';

export function StudentDetail({
  student,
  sensitive,
  sensitiveOpen,
  onSensitiveOpenChange,
  sensitiveRef,
}: {
  student: StudentDetails;
  sensitive: SensitiveData | null;
  sensitiveOpen: boolean;
  onSensitiveOpenChange: (open: boolean) => void;
  sensitiveRef: Ref<HTMLElement>;
}) {
  const values = Object.fromEntries(
    sensitive?.values.map((item) => [item.field, item.value]) ?? [],
  );
  return (
    <div className="divide-y divide-border px-6">
      <StudentBasicDetail student={student} />
      <DetailSection
        title="敏感信息"
        open={sensitiveOpen}
        onOpenChange={onSensitiveOpenChange}
        sectionRef={sensitiveRef}
      >
        <StudentFieldSections group="sensitive">
          {(field) => (
            <div
              className={
                field.maxLength > 500 || field.key === 'home_address'
                  ? 'min-w-0 space-y-1.5 sm:col-span-2'
                  : 'min-w-0 space-y-1.5'
              }
            >
              <dt className="text-xs text-muted-foreground">{field.label}</dt>
              <DetailValue
                label={field.label}
                value={sensitive ? fieldLabel(field, values[field.key] ?? '') : '***'}
                copyable={Boolean(sensitive && values[field.key])}
              />
            </div>
          )}
        </StudentFieldSections>
      </DetailSection>
    </div>
  );
}
