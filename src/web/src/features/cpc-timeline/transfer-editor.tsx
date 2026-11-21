import { useQuery } from '@tanstack/react-query';
import {
  ArrowRight,
  GripVertical,
  Link2,
  LockKeyhole,
  Plus,
  Trash2,
  Unlink,
  X,
} from 'lucide-react';
import { useId, useMemo, useState } from 'react';

import { ErrorMessage } from '@/components/error-message';
import { Button } from '@/components/ui/button';
import {
  DragMappingBoard,
  MappingLocateButton,
  MappingSourceButton,
  MappingTargetButton,
} from '@/components/ui/drag-mapping-board';
import { Label } from '@/components/ui/label';
import { SearchSelect } from '@/components/ui/search-select';
import { Tooltip } from '@/components/ui/tooltip';
import { timelineTransferTargetsOptions } from '@/features/cpc-timeline/api';
import { BaselineNodeIcon } from '@/features/cpc-timeline/baseline-node-badge';
import {
  fieldTypes,
  type TimelineNode,
  type TimelineTransfer,
  type TransferNodeMapping,
} from '@/features/cpc-timeline/model';
import { TimelineNodeGroups } from '@/features/cpc-timeline/node-groups';
import {
  connectTransferMapping,
  transferItemID,
  transferMappingItems,
  transferMappingLinks,
  transferMappingProblem,
} from '@/features/cpc-timeline/transfer-mapping';
import { cn } from '@/lib/utils';

const emptyNodes: TimelineNode[] = [];

export function TemplateTransferEditor({
  workspaceID,
  templateID,
  templateName,
  nodes,
  transfers,
  onChange,
}: {
  workspaceID: string;
  templateID: number | undefined;
  templateName: string;
  nodes: TimelineNode[];
  transfers: TimelineTransfer[];
  onChange: (value: TimelineTransfer[]) => void;
}) {
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [syncScroll, setSyncScroll] = useState(true);
  const activeIndex = Math.max(0, Math.min(selectedIndex, transfers.length - 1));
  const active = transfers[activeIndex] ?? { target_template_id: 0, node_mappings: [] };
  return (
    <div className="flex min-w-0 flex-col gap-3 lg:h-full lg:min-h-0">
      <div className="flex shrink-0 flex-wrap items-center justify-between gap-3">
        <p className="text-xs leading-5 text-muted-foreground">
          节点可多对多映射，附件随节点转移；字段仅支持同类型的手动字段。
        </p>
        {transfers.some((item) => item.target_template_id > 0) ? (
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={transfers.some((item) => item.target_template_id === 0)}
            onClick={() => {
              setSelectedIndex(transfers.length);
              onChange([...transfers, { target_template_id: 0, node_mappings: [] }]);
            }}
          >
            <Plus aria-hidden="true" />
            添加转移目标
          </Button>
        ) : null}
      </div>
      {transfers.length > 1 ? (
        <div
          role="group"
          aria-label="切换转移目标"
          className="flex shrink-0 gap-2 overflow-x-auto pb-1"
        >
          {transfers.map((_, index) => (
            <Button
              key={index}
              type="button"
              variant="ghost"
              size="sm"
              aria-pressed={index === activeIndex}
              className={cn(index === activeIndex && 'bg-accent text-accent-foreground')}
              onClick={() => {
                setSelectedIndex(index);
              }}
            >
              转移目标 {String(index + 1)}
            </Button>
          ))}
        </div>
      ) : null}
      <TransferTargetEditor
        key={`${String(activeIndex)}:${String(active.target_template_id)}`}
        workspaceID={workspaceID}
        templateID={templateID}
        templateName={templateName}
        nodes={nodes}
        transfer={active}
        syncScroll={syncScroll}
        onSyncScrollChange={setSyncScroll}
        excludedIDs={transfers
          .filter((_, index) => index !== activeIndex)
          .map((item) => item.target_template_id)}
        onChange={(value) => {
          onChange(
            transfers.length
              ? transfers.map((item, index) => (index === activeIndex ? value : item))
              : [value],
          );
        }}
        onRemove={
          transfers.length
            ? () => {
                onChange(transfers.filter((_, index) => index !== activeIndex));
                setSelectedIndex(Math.max(0, activeIndex - 1));
              }
            : undefined
        }
      />
    </div>
  );
}

function TransferTargetEditor({
  workspaceID,
  templateID,
  templateName,
  nodes,
  transfer,
  syncScroll,
  onSyncScrollChange,
  excludedIDs,
  onChange,
  onRemove,
}: {
  workspaceID: string;
  templateID: number | undefined;
  templateName: string;
  nodes: TimelineNode[];
  transfer: TimelineTransfer;
  syncScroll: boolean;
  onSyncScrollChange: (value: boolean) => void;
  excludedIDs: number[];
  onChange: (value: TimelineTransfer) => void;
  onRemove: (() => void) | undefined;
}) {
  const [keyword, setKeyword] = useState('');
  const targetID = useId();
  const purpose = templateID ? 'update' : 'create';
  const choices = useQuery(timelineTransferTargetsOptions(workspaceID, purpose, keyword));
  const selected = useQuery({
    ...timelineTransferTargetsOptions(workspaceID, purpose, '', transfer.target_template_id),
    enabled: transfer.target_template_id > 0,
  });
  const targets = [...(selected.data ?? []), ...(choices.data ?? [])].filter(
    (target, index, all) =>
      target.id !== templateID &&
      !excludedIDs.includes(target.id) &&
      all.findIndex((item) => item.id === target.id) === index,
  );
  const target = selected.error
    ? undefined
    : targets.find((item) => item.id === transfer.target_template_id);
  const targetNodes = target?.definition.nodes ?? emptyNodes;
  const sourceItems = useMemo(() => transferMappingItems(nodes), [nodes]);
  const targetItems = useMemo(() => transferMappingItems(targetNodes), [targetNodes]);
  const sources = useMemo(() => new Map(sourceItems.map((item) => [item.id, item])), [sourceItems]);
  const destinations = useMemo(
    () => new Map(targetItems.map((item) => [item.id, item])),
    [targetItems],
  );
  const links = useMemo(
    () => transferMappingLinks(transfer.node_mappings),
    [transfer.node_mappings],
  );
  const scrollLinks = useMemo(
    () =>
      links.filter(
        (link) =>
          sources.get(link.sourceID)?.kind === 'node' &&
          destinations.get(link.targetID)?.kind === 'node',
      ),
    [destinations, links, sources],
  );
  const setMappings = (value: TransferNodeMapping[]) => {
    onChange({ ...transfer, node_mappings: value });
  };
  const missing = transfer.node_mappings.flatMap((mapping, index) => {
    if (
      !sources.has(transferItemID(mapping.source_node_id)) ||
      !destinations.has(transferItemID(mapping.target_node_id))
    ) {
      return [{ index, fieldIndex: -1, label: '节点映射已失效' }];
    }
    return mapping.fields.flatMap((field, fieldIndex) => {
      const source = sources.get(transferItemID(mapping.source_node_id, field.source_field_id));
      const target = destinations.get(
        transferItemID(mapping.target_node_id, field.target_field_id),
      );
      const problem = transferMappingProblem(source, target, transfer.node_mappings);
      return problem
        ? [
            {
              index,
              fieldIndex,
              label: `${source?.label ?? '来源字段已失效'} → ${target?.label ?? '目标字段已失效'}`,
            },
          ]
        : [];
    });
  });
  return (
    <section
      aria-label="时间线拖拽映射"
      className="flex min-w-0 flex-col gap-3 lg:min-h-0 lg:flex-1"
    >
      <ErrorMessage>{choices.error?.message ?? selected.error?.message}</ErrorMessage>
      {target || onRemove ? (
        <div className="flex shrink-0 flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
          {target ? (
            <p>
              已配置 {String(transfer.node_mappings.length)} 组节点映射 ·{' '}
              {String(
                transfer.node_mappings.reduce((count, mapping) => count + mapping.fields.length, 0),
              )}{' '}
              组字段映射
            </p>
          ) : null}
          <div className="ml-auto flex max-w-full flex-wrap items-center justify-end gap-2">
            {target ? (
              <Button
                type="button"
                variant="ghost"
                size="sm"
                aria-pressed={syncScroll}
                aria-label={syncScroll ? '关闭联动滚动' : '开启联动滚动'}
                className={cn(syncScroll && 'bg-accent text-accent-foreground')}
                onClick={() => {
                  onSyncScrollChange(!syncScroll);
                }}
              >
                {syncScroll ? <Link2 aria-hidden="true" /> : <Unlink aria-hidden="true" />}
                联动滚动
              </Button>
            ) : null}
            {onRemove ? (
              <Button type="button" variant="ghost" size="sm" onClick={onRemove}>
                <Trash2 aria-hidden="true" />
                移除当前目标
              </Button>
            ) : null}
          </div>
        </div>
      ) : null}
      {target && missing.length ? (
        <div className="max-h-28 shrink-0 space-y-1 overflow-y-auto rounded-lg border border-warning-border bg-warning px-3 py-2">
          <p className="text-xs font-medium text-warning-foreground">
            以下映射已失效，请移除后重新配置
          </p>
          {missing.map((item) => (
            <div
              key={`${String(item.index)}:${String(item.fieldIndex)}`}
              className="flex items-center gap-2 text-xs"
            >
              <span className="min-w-0 flex-1 truncate">{item.label}</span>
              <Button
                type="button"
                variant="ghost"
                size="icon-xs"
                aria-label={`移除失效映射 ${String(item.index + 1)}-${String(item.fieldIndex + 1)}`}
                onClick={() => {
                  setMappings(
                    item.fieldIndex < 0
                      ? transfer.node_mappings.filter((_, index) => index !== item.index)
                      : transfer.node_mappings.map((mapping, index) =>
                          index === item.index
                            ? {
                                ...mapping,
                                fields: mapping.fields.filter(
                                  (_, fieldIndex) => fieldIndex !== item.fieldIndex,
                                ),
                              }
                            : mapping,
                        ),
                  );
                }}
              >
                <X aria-hidden="true" />
              </Button>
            </div>
          ))}
        </div>
      ) : null}
      <DragMappingBoard
        sourceItems={sourceItems}
        targetItems={targetItems}
        links={links}
        scrollLinks={scrollLinks}
        syncScroll={syncScroll}
        validate={(from, to) =>
          transferMappingProblem(sources.get(from), destinations.get(to), transfer.node_mappings)
        }
        onConnect={(from, to) => {
          const source = sources.get(from);
          const target = destinations.get(to);
          if (source && target)
            setMappings(connectTransferMapping(transfer.node_mappings, source, target));
        }}
        sourceTitle={
          <div className="space-y-2">
            <p className="text-xs leading-5 text-muted-foreground">来源模板</p>
            <h3 className="flex h-9 min-w-0 items-center rounded-lg bg-background/70 px-3 text-sm font-medium">
              <Tooltip content={templateName.trim() || '当前模板'} onlyOverflow>
                <span className="min-w-0 flex-1 truncate">{templateName.trim() || '当前模板'}</span>
              </Tooltip>
            </h3>
            <p className="text-xs leading-4 text-muted-foreground">
              {String(nodes.length)} 个节点 · {String(sourceItems.length - nodes.length)} 个字段
            </p>
          </div>
        }
        targetTitle={
          <div className="space-y-2">
            <Label
              htmlFor={targetID}
              className="text-xs leading-5 font-normal text-muted-foreground"
            >
              目标模板
            </Label>
            <SearchSelect
              id={targetID}
              label="转移目标模板"
              placeholder="选择目标模板"
              className="w-full"
              value={transfer.target_template_id ? String(transfer.target_template_id) : ''}
              options={targets.map((item) => ({ value: String(item.id), label: item.name }))}
              loading={choices.isPending || (transfer.target_template_id > 0 && selected.isPending)}
              onSearchChange={setKeyword}
              onValueChange={(value) => {
                onChange({ target_template_id: Number(value), node_mappings: [] });
              }}
            />
            <p className="min-h-4 text-xs leading-4 text-muted-foreground">
              {target
                ? `${String(targetNodes.length)} 个节点 · ${String(targetItems.length - targetNodes.length)} 个字段`
                : choices.isPending || (selected.isPending && transfer.target_template_id > 0)
                  ? '正在加载目标模板'
                  : null}
            </p>
          </div>
        }
        source={
          <MappingTimeline
            nodes={nodes}
            oppositeNodes={targetNodes}
            side="source"
            mappings={transfer.node_mappings}
            enabled={!!target}
            onChange={setMappings}
          />
        }
        target={
          target ? (
            <MappingTimeline
              nodes={targetNodes}
              oppositeNodes={nodes}
              side="target"
              mappings={transfer.node_mappings}
              enabled
              onChange={setMappings}
            />
          ) : (
            <div className="flex min-h-40 flex-col items-center justify-center gap-3 text-center">
              <ArrowRight aria-hidden="true" className="size-6 text-muted-foreground" />
              <p className="text-sm text-muted-foreground">选择目标模板后开始映射</p>
            </div>
          )
        }
      />
    </section>
  );
}

function MappingTimeline({
  nodes,
  oppositeNodes,
  side,
  mappings,
  enabled,
  onChange,
}: {
  nodes: TimelineNode[];
  oppositeNodes: TimelineNode[];
  side: 'source' | 'target';
  mappings: TransferNodeMapping[];
  enabled: boolean;
  onChange: (value: TransferNodeMapping[]) => void;
}) {
  const Entry = side === 'source' ? MappingSourceButton : MappingTargetButton;
  return (
    <TimelineNodeGroups
      nodes={nodes}
      label={side === 'source' ? '当前时间线节点和字段' : '目标时间线节点和字段'}
      renderNode={(node, index) => {
        const related = mappings.filter(
          (mapping) =>
            (side === 'source' ? mapping.source_node_id : mapping.target_node_id) === node.id,
        );
        return (
          <li key={node.id} data-mapping-node={transferItemID(node.id)} className="pb-3 last:pb-0">
            <div className="space-y-2 rounded-xl border border-border-subtle bg-background/75 p-2">
              <h4>
                <Entry
                  itemID={transferItemID(node.id)}
                  disabled={!enabled}
                  aria-label={`${side === 'source' ? '拖动来源节点' : '映射到目标节点'} ${node.name.trim() || '未命名节点'}`}
                  className="min-h-9 py-1 font-normal text-foreground"
                >
                  <span className="flex size-6 shrink-0 items-center justify-center rounded-md bg-muted text-xs text-foreground-secondary tabular-nums">
                    {String(index + 1)}
                  </span>
                  {side === 'source' ? (
                    <GripVertical aria-hidden="true" className="size-4 text-muted-foreground" />
                  ) : (
                    <ArrowRight aria-hidden="true" className="size-4 text-accent-foreground" />
                  )}
                  <Tooltip content={node.name.trim() || '未命名节点'} onlyOverflow>
                    <span className="min-w-0 flex-1 truncate font-medium">
                      {node.name.trim() || '未命名节点'}
                    </span>
                  </Tooltip>
                  {node.is_baseline ? <BaselineNodeIcon /> : null}
                  {related.length ? (
                    <span className="shrink-0 text-xs text-accent-foreground">
                      {String(related.length)} 组映射
                    </span>
                  ) : null}
                </Entry>
              </h4>
              {related.map((mapping) => {
                const opposite = oppositeNodes.find(
                  (item) =>
                    item.id ===
                    (side === 'source' ? mapping.target_node_id : mapping.source_node_id),
                );
                return (
                  <MappingChip
                    key={side === 'source' ? mapping.target_node_id : mapping.source_node_id}
                    source={side === 'source' ? node.name : (opposite?.name ?? '来源节点已失效')}
                    target={side === 'source' ? (opposite?.name ?? '目标节点已失效') : node.name}
                    sourceID={transferItemID(mapping.source_node_id)}
                    targetID={transferItemID(mapping.target_node_id)}
                    side={side}
                    kind="节点"
                    onRemove={
                      side === 'target'
                        ? () => {
                            onChange(mappings.filter((item) => item !== mapping));
                          }
                        : undefined
                    }
                  />
                );
              })}
              {node.description ? (
                <p className="px-2 text-xs leading-5 wrap-anywhere whitespace-pre-wrap text-muted-foreground">
                  {node.description}
                </p>
              ) : null}
              <ul
                className="space-y-1 border-t border-border-subtle pt-2"
                aria-label={`${node.name}的字段`}
              >
                {node.fields.map((field) => {
                  const fieldMappings = related.flatMap((mapping) =>
                    mapping.fields
                      .filter(
                        (item) =>
                          (side === 'source' ? item.source_field_id : item.target_field_id) ===
                          field.id,
                      )
                      .map((item) => ({ mapping, field: item })),
                  );
                  return (
                    <li key={field.id} className="space-y-1">
                      <Entry
                        itemID={transferItemID(node.id, field.id)}
                        disabled={!enabled || !!field.student_field}
                        aria-label={`${side === 'source' ? '拖动来源字段' : '映射到目标字段'} ${node.name.trim() || '未命名节点'} / ${field.name.trim() || '未命名字段'}`}
                        className="min-h-8 bg-muted/40 py-1 font-normal text-foreground"
                      >
                        {field.student_field ? (
                          <LockKeyhole
                            aria-hidden="true"
                            className="size-3.5 text-muted-foreground"
                          />
                        ) : side === 'source' ? (
                          <GripVertical
                            aria-hidden="true"
                            className="size-3.5 text-muted-foreground"
                          />
                        ) : (
                          <ArrowRight
                            aria-hidden="true"
                            className="size-3.5 text-accent-foreground"
                          />
                        )}
                        <Tooltip content={field.name.trim() || '未命名字段'} onlyOverflow>
                          <span className="min-w-0 flex-1 truncate text-sm font-normal">
                            {field.name.trim() || '未命名字段'}
                            {field.required && !field.student_field ? (
                              <span aria-hidden="true" className="ml-1 text-danger-foreground">
                                *
                              </span>
                            ) : null}
                          </span>
                        </Tooltip>
                        <span className="shrink-0 text-xs font-normal text-muted-foreground">
                          {field.student_field
                            ? '引用 · 只读'
                            : fieldTypes.find((item) => item.value === field.type)?.label}
                        </span>
                        {side === 'source' && fieldMappings.length ? (
                          <span className="flex shrink-0 items-center gap-1 text-xs text-accent-foreground">
                            <ArrowRight aria-hidden="true" className="size-3" />
                            {String(fieldMappings.length)}
                          </span>
                        ) : null}
                      </Entry>
                      {fieldMappings.map(({ mapping, field: value }) => {
                        const opposite = oppositeNodes.find(
                          (item) =>
                            item.id ===
                            (side === 'source' ? mapping.target_node_id : mapping.source_node_id),
                        );
                        const oppositeField = opposite?.fields.find(
                          (item) =>
                            item.id ===
                            (side === 'source' ? value.target_field_id : value.source_field_id),
                        );
                        const oppositeLabel = `${opposite?.name ?? '节点已失效'} / ${oppositeField?.name ?? '字段已失效'}`;
                        return (
                          <MappingChip
                            key={JSON.stringify([
                              mapping.source_node_id,
                              value.source_field_id,
                              mapping.target_node_id,
                              value.target_field_id,
                            ])}
                            source={
                              side === 'source' ? `${node.name} / ${field.name}` : oppositeLabel
                            }
                            target={side === 'source' ? oppositeLabel : field.name}
                            sourceID={transferItemID(mapping.source_node_id, value.source_field_id)}
                            targetID={transferItemID(mapping.target_node_id, value.target_field_id)}
                            side={side}
                            kind="字段"
                            onRemove={
                              side === 'target'
                                ? () => {
                                    onChange(
                                      mappings.map((item) =>
                                        item === mapping
                                          ? {
                                              ...item,
                                              fields: item.fields.filter((part) => part !== value),
                                            }
                                          : item,
                                      ),
                                    );
                                  }
                                : undefined
                            }
                          />
                        );
                      })}
                    </li>
                  );
                })}
              </ul>
            </div>
          </li>
        );
      }}
    />
  );
}

function MappingChip({
  source,
  target,
  sourceID,
  targetID,
  side,
  kind,
  onRemove,
}: {
  source: string;
  target: string;
  sourceID: string;
  targetID: string;
  side: 'source' | 'target';
  kind: '节点' | '字段';
  onRemove: (() => void) | undefined;
}) {
  const counterpart = side === 'source' ? target : source;
  return (
    <div className="flex min-w-0 items-center gap-1 rounded-lg bg-accent/70 px-1 py-0.5 text-xs text-foreground-secondary">
      {side === 'source' ? (
        <ArrowRight aria-hidden="true" className="ml-1 size-3.5 shrink-0 text-primary" />
      ) : null}
      <Tooltip content={counterpart} onlyOverflow>
        <MappingLocateButton
          side={side === 'source' ? 'target' : 'source'}
          itemID={side === 'source' ? targetID : sourceID}
          sourceID={sourceID}
          aria-label={`定位${side === 'source' ? '目标' : '来源'}${kind} ${counterpart}`}
        >
          <span data-tooltip-overflow className="min-w-0 flex-1 truncate">
            {counterpart}
          </span>
        </MappingLocateButton>
      </Tooltip>
      {side === 'target' ? (
        <>
          <ArrowRight aria-hidden="true" className="size-3.5 shrink-0 text-primary" />
          <Tooltip content={target} onlyOverflow>
            <span className="max-w-[35%] min-w-0 truncate">{target}</span>
          </Tooltip>
        </>
      ) : null}
      {onRemove ? (
        <Tooltip content={kind === '节点' ? '移除节点映射及其字段映射' : '移除字段映射'}>
          <Button
            type="button"
            variant="ghost"
            size="icon-xs"
            aria-label={`移除${kind}映射 ${source} → ${target}`}
            onClick={onRemove}
          >
            <X aria-hidden="true" />
          </Button>
        </Tooltip>
      ) : null}
    </div>
  );
}
