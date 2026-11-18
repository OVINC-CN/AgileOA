import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router';

import { DetailValue } from '@/components/detail-value';
import { ErrorMessage } from '@/components/error-message';
import { RefreshButton } from '@/components/refresh-button';
import { Badge } from '@/components/ui/badge';
import { Card } from '@/components/ui/card';
import { fieldLabel, studentFields, studentSections } from '@/features/student/fields';
import { type StudentProfile, studentProfileOptions } from '@/features/student-service/api';
import { useCurrentUser } from '@/features/user/api';
import { studentServiceURL } from '@/features/user/navigation';
import { ApiError } from '@/lib/api';
import { formatValue } from '@/lib/format';
import { cn } from '@/lib/utils';

const profileDetailFields = studentSections.basic
  .filter((section) => section.id === 'academic')
  .flatMap((section) => section.fields)
  .flatMap((key) => studentFields.filter((field) => field.key === key))
  .filter((field) => !['name', 'student_no', 'status'].includes(field.key));

export function StudentProfilePage() {
  const { data: user } = useCurrentUser();
  const profile = useQuery(studentProfileOptions(user?.workspace_id ?? '', user?.id ?? 0));
  const denied =
    profile.error instanceof ApiError && [401, 403, 404].includes(profile.error.status);
  const student = denied ? undefined : profile.data;
  return (
    <section aria-labelledby="student-profile-heading" className="space-y-4">
      <header className="flex min-h-8 flex-wrap items-center justify-between gap-4">
        <h1 id="student-profile-heading" className="sr-only">
          个人信息
        </h1>
        <nav aria-label="面包屑" className="flex h-8 items-center text-xs text-muted-foreground">
          <ol className="flex items-center gap-2">
            <li>
              <Link
                to={studentServiceURL(user?.workspace_id ?? '')}
                className="transition-colors hover:text-foreground"
              >
                学生服务
              </Link>
            </li>
            <li aria-hidden="true" className="text-muted-foreground">
              /
            </li>
            <li aria-current="page">个人信息</li>
          </ol>
        </nav>
        <RefreshButton
          onRefresh={async () => {
            const result = await profile.refetch();
            if (result.error) throw result.error;
          }}
        />
      </header>
      {profile.error ? <ErrorMessage>{profile.error.message}</ErrorMessage> : null}
      {profile.isPending ? <Card aria-busy="true" className="h-40" /> : null}
      {student ? (
        <Card className="gap-0 overflow-hidden py-0">
          <div className="flex flex-wrap items-start justify-between gap-3 border-b border-primary/10 bg-accent/50 px-5 py-4 sm:px-6">
            <dl className="min-w-0 flex-1 space-y-1">
              <div className="[&_dd]:font-medium">
                <dt className="sr-only">姓名</dt>
                <DetailValue
                  label="姓名"
                  value={student.name}
                  copyable={student.name.trim() !== ''}
                />
              </div>
              <div className="flex min-w-0 items-baseline gap-2 [&_dd]:min-w-0 [&_dd]:text-xs [&_dd]:text-muted-foreground [&_dd]:tabular-nums">
                <dt className="shrink-0 text-xs text-muted-foreground">学号</dt>
                <DetailValue
                  label="学号"
                  value={student.student_no}
                  copyable={student.student_no.trim() !== ''}
                />
              </div>
            </dl>
            <dl className="shrink-0 pt-1">
              <div>
                <dt className="sr-only">状态</dt>
                <dd>
                  <Badge
                    variant="secondary"
                    className="border-primary/10 bg-primary/10 px-2.5 py-1 text-accent-foreground"
                  >
                    {formatValue(student.status)}
                  </Badge>
                </dd>
              </div>
            </dl>
          </div>
          <dl className="grid gap-x-8 gap-y-4 px-5 py-4 sm:grid-cols-2 sm:px-6 xl:grid-cols-4">
            {profileDetailFields.map((field) => {
              const value = String(student[field.key as keyof StudentProfile]);
              return (
                <div
                  key={field.key}
                  className={cn('min-w-0 space-y-1', field.key === 'major' && 'sm:col-span-2')}
                >
                  <dt className="text-xs text-muted-foreground">{field.label}</dt>
                  <DetailValue
                    label={field.label}
                    value={fieldLabel(field, value)}
                    copyable={value !== ''}
                  />
                </div>
              );
            })}
          </dl>
        </Card>
      ) : null}
    </section>
  );
}
