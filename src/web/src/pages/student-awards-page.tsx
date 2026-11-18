import { ApplicationList } from '@/features/project/application-list';
import { useStudentServiceContext } from '@/features/student-service/context';
import { studentAwardsTabs as tabs } from '@/features/student-service/navigation';
import { useNavigationTabs } from '@/lib/navigation-tabs';

export function StudentAwardsPage() {
  const { workspaceID } = useStudentServiceContext();
  const { selected } = useNavigationTabs(tabs);
  const tab = selected?.value ?? 'mine';
  const scope = tab === 'mine' ? 'mine' : 'student-reviews';
  return (
    <section aria-labelledby="student-awards-title">
      <h1 id="student-awards-title" className="sr-only">
        评奖评优
      </h1>
      <ApplicationList
        key={`${workspaceID}:${scope}`}
        workspaceID={workspaceID}
        scope={scope}
        panelID={`student-awards-panel-${tab}`}
        panelLabel={selected?.label ?? tabs[0].label}
        breadcrumb={
          <nav aria-label="面包屑" className="text-xs text-muted-foreground">
            <ol className="flex flex-wrap items-center gap-2">
              <li>学生服务</li>
              <li aria-hidden="true">/</li>
              <li>评奖评优</li>
              <li aria-hidden="true">/</li>
              <li aria-current="page">{selected?.label}</li>
            </ol>
          </nav>
        }
      />
    </section>
  );
}
