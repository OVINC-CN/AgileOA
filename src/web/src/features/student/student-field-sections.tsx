import { Fragment, type ReactNode, useId } from 'react';

import { DetailSection } from '@/components/detail-section';
import { type StudentField, studentFields, studentSections } from '@/features/student/fields';
import { cn } from '@/lib/utils';

export function StudentFieldSections({
  group,
  form = false,
  accommodationDetails,
  detailTitleValue,
  sectionIDs,
  children,
}: {
  group: StudentField['group'];
  form?: boolean;
  accommodationDetails?: ReactNode;
  detailTitleValue?: (field: StudentField) => string;
  sectionIDs?: readonly string[];
  children: (field: StudentField) => ReactNode;
}) {
  const id = useId();
  const sections = sectionIDs
    ? sectionIDs.flatMap((id) => studentSections[group].filter((section) => section.id === id))
    : studentSections[group];
  return (
    <div className="divide-y divide-border">
      {sections.map((section) => {
        const fields = section.fields.flatMap((key) => {
          const field = studentFields.find((item) => item.key === key);
          return field ? [field] : [];
        });
        const titleField =
          !form && detailTitleValue
            ? fields.find((field) => field.key === section.detailTitleField)
            : undefined;
        const content = fields
          .filter((field) => field !== titleField)
          .map((field) => <Fragment key={field.key}>{children(field)}</Fragment>);
        if (!form) {
          const title =
            titleField && detailTitleValue
              ? `${section.label} (${detailTitleValue(titleField)})`
              : section.label;
          return (
            <DetailSection key={section.id} title={title} heading="h3">
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
            <h3 id={`${id}-${section.id}`} className="text-sm font-medium">
              {section.label}
            </h3>
            <div
              className={cn('grid gap-5', section.formGridColumns ?? 'grid-cols-1 sm:grid-cols-2')}
            >
              {content}
            </div>
          </section>
        );
      })}
    </div>
  );
}
