import { Fragment, type ReactNode, useId } from 'react';

import { DetailSection } from '@/components/detail-section';
import { type StudentField, studentFields, studentSections } from '@/features/student/fields';

export function StudentFieldSections({
  group,
  form = false,
  accommodationDetails,
  sectionIDs,
  children,
}: {
  group: StudentField['group'];
  form?: boolean;
  accommodationDetails?: ReactNode;
  sectionIDs?: readonly string[];
  children: (field: StudentField) => ReactNode;
}) {
  const id = useId();
  // 编辑时将住宿信息放在基本信息末尾，紧邻后续的校外住宿信息。
  const sections = studentSections[group].filter(
    (section) => !sectionIDs || sectionIDs.includes(section.id),
  );
  if (form) {
    sections.sort(
      (first, second) =>
        Number(first.id === 'accommodation') - Number(second.id === 'accommodation'),
    );
  }
  return (
    <div className="divide-y divide-border">
      {sections.map((section) => {
        const fields = section.fields.flatMap((key) => {
          const field = studentFields.find((item) => item.key === key);
          return field ? [field] : [];
        });
        const content = fields.map((field) => (
          <Fragment key={field.key}>{children(field)}</Fragment>
        ));
        if (!form) {
          return (
            <DetailSection key={section.id} title={section.label} heading="h3">
              <dl className="grid grid-cols-1 gap-5 sm:grid-cols-2">{content}</dl>
              {section.id === 'accommodation' ? accommodationDetails : null}
            </DetailSection>
          );
        }
        return (
          <section
            key={section.id}
            aria-labelledby={`${id}-${section.id}`}
            className="space-y-4 py-5 first:pt-0 last:pb-0"
          >
            <h3 id={`${id}-${section.id}`} className="text-sm font-semibold">
              {section.label}
            </h3>
            <div className="grid grid-cols-1 gap-5 sm:grid-cols-2">{content}</div>
          </section>
        );
      })}
    </div>
  );
}
