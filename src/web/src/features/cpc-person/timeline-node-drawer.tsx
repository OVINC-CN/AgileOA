import { useQuery, useQueryClient } from '@tanstack/react-query';
import { RefreshCw } from 'lucide-react';

import { ErrorMessage } from '@/components/error-message';
import { Button } from '@/components/ui/button';
import { Drawer } from '@/components/ui/drawer';
import { type Person, personKey } from '@/features/cpc-person/api';
import { type PersonTimelineNode, personTimelineOptions } from '@/features/cpc-person/timeline-api';
import { PersonTimelineNodeDetails } from '@/features/cpc-person/timeline-node-details';
import { PersonTimelineNodeEditor } from '@/features/cpc-person/timeline-node-editor';
import { ApiError } from '@/lib/api';
import { useApiAction } from '@/lib/api-action';
import { formatIdentity } from '@/lib/format';

export interface PersonTimelineNodeSelection {
  person: Person;
  node: PersonTimelineNode;
  templateRevision: number;
  editing: boolean;
}

export function PersonTimelineNodeDrawer({
  workspaceID,
  selection,
  onClose,
}: {
  workspaceID: string;
  selection: PersonTimelineNodeSelection;
  onClose: () => void;
}) {
  const { person, node: original, templateRevision, editing } = selection;
  const client = useQueryClient();
  const refresh = useApiAction();
  const query = useQuery(personTimelineOptions(workspaceID, person.id, editing));
  const denied = query.error instanceof ApiError && [401, 403, 404].includes(query.error.status);
  const data = denied ? undefined : query.data;
  const node =
    data?.template_id === person.timeline_template_id
      ? data.nodes.find((item) => item.id === original.id)
      : undefined;
  const personLabel = formatIdentity(person.name, person.student_no);
  const invalidate = async () => {
    await Promise.all([
      client.invalidateQueries({ queryKey: personKey(workspaceID) }),
      client.invalidateQueries({ queryKey: ['management', 'audit'] }),
    ]);
  };

  if (editing && data?.can_update && data.status !== 'transferring' && node) {
    return (
      <PersonTimelineNodeEditor
        workspaceID={workspaceID}
        personID={person.id}
        personLabel={personLabel}
        original={original}
        current={node}
        nodes={data.nodes}
        templateRevision={templateRevision}
        invalidate={invalidate}
        onClose={onClose}
      />
    );
  }

  return (
    <Drawer
      title={editing ? '编辑时间线节点' : '时间线节点详情'}
      truncateDescription
      description={`${personLabel} · ${original.name}`}
      busy={refresh.pending}
      onClose={onClose}
      footer={
        <Button
          variant="outline"
          loading={refresh.pending || query.isPending}
          onClick={() => {
            void refresh.run('refresh', () =>
              client.query({
                ...personTimelineOptions(workspaceID, person.id, editing),
                staleTime: 0,
              }),
            );
          }}
        >
          <RefreshCw aria-hidden="true" />
          刷新
        </Button>
      }
    >
      <div className="space-y-4 p-6">
        <ErrorMessage>
          {query.error?.message ??
            (data && !node
              ? '人员时间线或模板已被修改，请刷新后重试'
              : editing && data?.status === 'transferring'
                ? '人员时间线正在转移中，请先取消转移后再操作'
                : editing && data && !data.can_update
                  ? '暂无编辑此节点的权限'
                  : undefined)}
        </ErrorMessage>
        {!editing && data && node ? (
          <>
            <p className="text-sm">节点状态：{node.status_label}</p>
            <PersonTimelineNodeDetails
              workspaceID={workspaceID}
              personID={person.id}
              node={node}
              nodes={data.nodes}
              editing={false}
            />
          </>
        ) : null}
      </div>
    </Drawer>
  );
}
