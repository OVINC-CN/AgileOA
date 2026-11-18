import type { PersonTimelineNode } from '@/features/cpc-person/timeline-api';
import { PersonTimelineFiles } from '@/features/cpc-person/timeline-files';
import { PersonTimelineRules } from '@/features/cpc-person/timeline-rules';
import { timelineValueLabel } from '@/features/cpc-person/timeline-values';
import type { TimelineNode } from '@/features/cpc-timeline/model';

export function PersonTimelineNodeDetails({
  workspaceID,
  personID,
  node,
  nodes,
  editing,
  preview = false,
}: {
  workspaceID: string;
  personID: number;
  node: PersonTimelineNode;
  nodes: TimelineNode[];
  editing: boolean;
  preview?: boolean;
}) {
  return (
    <>
      {node.description ? (
        <p className="text-xs leading-5 wrap-anywhere whitespace-pre-wrap text-muted-foreground">
          {node.description}
        </p>
      ) : null}
      <dl className="grid grid-cols-1 gap-3 text-sm sm:grid-cols-2">
        {node.fields.map((field) => (
          <div key={field.id} className="min-w-0 space-y-1">
            <dt className="text-xs wrap-anywhere text-muted-foreground">
              {field.name}
              {field.required && !field.student_field ? ' *' : ''}
            </dt>
            <dd className="wrap-anywhere whitespace-pre-wrap">
              {timelineValueLabel(node.values[field.id])}
            </dd>
          </div>
        ))}
      </dl>
      <PersonTimelineRules node={node} nodes={nodes} />
      <PersonTimelineFiles
        workspaceID={workspaceID}
        personID={personID}
        node={node}
        editing={editing}
        downloadable={!preview}
      />
    </>
  );
}
