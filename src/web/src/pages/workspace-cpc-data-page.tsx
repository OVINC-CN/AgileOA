import { Card } from '@/components/ui/card';
import { TimelinePage } from '@/features/cpc-timeline/template-page';
import { BranchPage } from '@/features/party-branch/branch-page';
import { useWorkspaceContext } from '@/features/workspace/context';
import { cpcDataTabs } from '@/features/workspace/navigation';
import { useNavigationTabs } from '@/lib/navigation-tabs';

export function WorkspaceCPCDataPage() {
  const context = useWorkspaceContext();
  const tabs = cpcDataTabs.filter(({ listAction }) => context.actions.includes(listAction));
  const { selected } = useNavigationTabs(tabs);
  if (!selected) {
    return <Card className="p-6 text-sm text-muted-foreground">暂无可查看的数据管理页面</Card>;
  }
  return (
    <div>
      {selected.value === 'timeline-templates' ? (
        <TimelinePage key={`${context.workspaceID}:timeline`} {...context} />
      ) : (
        <BranchPage {...context} />
      )}
    </div>
  );
}
