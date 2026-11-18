import type { MappingItem, MappingLink } from '@/components/ui/drag-mapping-board';
import type {
  TimelineField,
  TimelineNode,
  TransferNodeMapping,
} from '@/features/cpc-timeline/model';

export type TransferMappingItem = MappingItem & {
  node: TimelineNode;
} & ({ kind: 'node' } | { kind: 'field'; field: TimelineField });

export const transferItemID = (nodeID: string, fieldID?: string) =>
  JSON.stringify(fieldID ? [nodeID, fieldID] : [nodeID]);

export function transferMappingItems(nodes: TimelineNode[]): TransferMappingItem[] {
  return nodes.flatMap<TransferMappingItem>((node, index) => [
    {
      id: transferItemID(node.id),
      label: `${String(index + 1)} · ${node.name}`,
      node,
      kind: 'node',
      disabled: false,
    },
    ...node.fields.map((field): TransferMappingItem => ({
      id: transferItemID(node.id, field.id),
      label: `${String(index + 1)} · ${node.name} / ${field.name}`,
      node,
      field,
      kind: 'field',
      disabled: !!field.student_field,
    })),
  ]);
}

export function transferMappingLinks(mappings: TransferNodeMapping[]): MappingLink[] {
  return mappings.flatMap((mapping) => {
    const sourceID = transferItemID(mapping.source_node_id);
    const targetID = transferItemID(mapping.target_node_id);
    return [
      { id: JSON.stringify([sourceID, targetID]), sourceID, targetID },
      ...mapping.fields.map((field) => {
        const sourceID = transferItemID(mapping.source_node_id, field.source_field_id);
        const targetID = transferItemID(mapping.target_node_id, field.target_field_id);
        return { id: JSON.stringify([sourceID, targetID]), sourceID, targetID };
      }),
    ];
  });
}

export function transferMappingProblem(
  source: TransferMappingItem | undefined,
  target: TransferMappingItem | undefined,
  mappings: TransferNodeMapping[],
): string | null {
  // 节点与字段分别映射，引用字段只展示，不允许成为映射来源或目标。
  if (!source || !target) return '节点或字段已失效，请重新选择';
  if (source.kind !== target.kind) return '节点请映射到节点，字段请映射到字段';
  if (source.kind === 'node' || target.kind === 'node') return null;
  if (source.disabled || target.disabled) return '引用字段由学生资料自动读取，不参与字段映射';
  if (source.field.type !== target.field.type) return '只能映射到相同类型的手动字段';
  const occupied = mappings.find(
    (mapping) =>
      mapping.target_node_id === target.node.id &&
      mapping.fields.some(
        (field) =>
          field.target_field_id === target.field.id &&
          (mapping.source_node_id !== source.node.id || field.source_field_id !== source.field.id),
      ),
  );
  return occupied ? '该目标字段已有来源，请先移除原字段映射' : null;
}

export function connectTransferMapping(
  mappings: TransferNodeMapping[],
  source: TransferMappingItem,
  target: TransferMappingItem,
): TransferNodeMapping[] {
  const problem = transferMappingProblem(source, target, mappings);
  if (problem) throw new Error(problem);
  const pair = mappings.find(
    (mapping) =>
      mapping.source_node_id === source.node.id && mapping.target_node_id === target.node.id,
  );
  if (source.kind === 'node' || target.kind === 'node') {
    return pair
      ? mappings
      : [
          ...mappings,
          {
            source_node_id: source.node.id,
            target_node_id: target.node.id,
            fields: [],
          },
        ];
  }
  if (
    pair?.fields.some(
      (field) =>
        field.source_field_id === source.field.id && field.target_field_id === target.field.id,
    )
  )
    return mappings;
  const field = { source_field_id: source.field.id, target_field_id: target.field.id };
  // 拖动字段时自动建立所属节点关系，保留其他来源和目标已有的映射。
  return pair
    ? mappings.map((mapping) =>
        mapping === pair ? { ...mapping, fields: [...mapping.fields, field] } : mapping,
      )
    : [
        ...mappings,
        { source_node_id: source.node.id, target_node_id: target.node.id, fields: [field] },
      ];
}
