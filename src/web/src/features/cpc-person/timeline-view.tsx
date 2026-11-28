import { useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import {
  CheckCheck,
  CircleDashed,
  Eye,
  GitBranch,
  Pencil,
  SkipForward,
  UsersRound,
} from 'lucide-react';
import { type ReactNode, useState } from 'react';

import { ErrorMessage } from '@/components/error-message';
import { FilterField } from '@/components/filter-panel';
import { ManagementTable } from '@/components/management-table';
import { MetricCard } from '@/components/metric-card';
import { TableActionButton } from '@/components/table-action-button';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { SearchSelect } from '@/components/ui/search-select';
import { Tooltip } from '@/components/ui/tooltip';
import type { Person } from '@/features/cpc-person/api';
import { PersonTimelineDrawer } from '@/features/cpc-person/person-timeline';
import { personTimelineOptions } from '@/features/cpc-person/timeline-api';
import {
  PersonTimelineNodeDrawer,
  type PersonTimelineNodeSelection,
} from '@/features/cpc-person/timeline-node-drawer';
import { timelineValueLabel } from '@/features/cpc-person/timeline-values';
import type { TimelineViewPerson } from '@/features/cpc-person/timeline-view-api';
import type { usePersonTimelineView } from '@/features/cpc-person/timeline-view-state';
import { BaselineNodeIcon } from '@/features/cpc-timeline/baseline-node-badge';
import { TimelineNodeGroups } from '@/features/cpc-timeline/node-groups';
import { useApiAction } from '@/lib/api-action';
import { recordTimeColumns } from '@/lib/record-time-columns';
import { cn } from '@/lib/utils';

export function PersonTimelineView({
  view,
  filters,
  refreshAction,
  workspaceID,
  actions,
}: {
  view: ReturnType<typeof usePersonTimelineView>;
  filters: ReactNode;
  refreshAction?: ReactNode;
  workspaceID: string;
  actions: string[];
}) {
  const { node, data, stats, pagination } = view;
  const client = useQueryClient();
  const read = useApiAction();
  const edit = useApiAction();
  const canDetail = actions.includes('cpc.person.detail');
  const canUpdate = actions.includes('cpc.person.update');
  const canReview = actions.includes('cpc.person.timeline_transfer.review');
  const [timelinePerson, setTimelinePerson] = useState<Person | null>(null);
  const [selection, setSelection] = useState<PersonTimelineNodeSelection | null>(null);
  const openNode = async (person: Person, editing: boolean) => {
    if (!node) return;

    // 详情与编辑分别读取对应权限的最新节点，不能使用名单值替代节点版本和附件。
    const timeline = await client.query({
      ...personTimelineOptions(workspaceID, person.id, editing),
      staleTime: 0,
    });
    const current = timeline.nodes.find((item) => item.id === node.id);
    if (timeline.template_id !== view.templateID || !current) {
      throw new Error('人员时间线或模板已被修改，请刷新后重试');
    }
    if (editing && timeline.status === 'transferring') {
      throw new Error('人员时间线正在转移中，请先取消转移后再操作');
    }
    if (editing && !timeline.can_update) throw new Error('暂无编辑此节点的权限');
    setSelection({
      person,
      node: current,
      templateRevision: timeline.template_revision,
      editing,
    });
  };
  const openDetail = (person: Person, key: string) => {
    void read.run(key, () => openNode(person, false));
  };
  const columns: ColumnDef<TimelineViewPerson>[] = [
    { id: 'person_name', accessorKey: 'person.name', header: '姓名' },
    { accessorKey: 'person.student_no', header: '学号' },
    {
      accessorKey: 'status_label',
      header: '节点状态',
      cell: ({ row }) => (
        <span
          className={
            row.original.status === 'completed'
              ? 'text-success-foreground'
              : row.original.status === 'pending'
                ? 'text-warning-foreground'
                : 'text-muted-foreground'
          }
        >
          {row.original.status_label}
        </span>
      ),
    },
    ...(node?.fields.map((field): ColumnDef<TimelineViewPerson> => ({
      id: `field-${field.id}`,
      header: field.name,
      cell: ({ row }) => timelineValueLabel(row.original.values[field.id]),
    })) ?? []),
    ...recordTimeColumns<TimelineViewPerson>((record) => record.person),
    {
      id: 'actions',
      header: '操作',
      cell: ({ row }) => {
        const person = row.original.person;
        const canEdit = canUpdate && person.can_update && person.timeline_status !== 'transferring';
        return (
          <div className="flex items-center gap-2">
            {canDetail ? (
              <TableActionButton
                loading={read.isPending(String(person.id))}
                onClick={() => {
                  openDetail(person, String(person.id));
                }}
              >
                <Eye aria-hidden="true" />
                详情
              </TableActionButton>
            ) : null}
            {canEdit ? (
              <TableActionButton
                loading={edit.isPending(String(person.id))}
                onClick={() => {
                  void edit.run(String(person.id), () => openNode(person, true));
                }}
              >
                <Pencil aria-hidden="true" />
                编辑
              </TableActionButton>
            ) : null}
            {canDetail ||
            (canUpdate && person.can_update) ||
            (canReview && person.can_review_timeline_transfer) ? (
              <TableActionButton
                onClick={() => {
                  setTimelinePerson(person);
                }}
              >
                <GitBranch aria-hidden="true" />
                时间线
              </TableActionButton>
            ) : null}
          </div>
        );
      },
    },
  ];

  return (
    <div className="space-y-5">
      {[view.templates, view.template, view.query, view.statistics].map((query, index) =>
        query.error ? <ErrorMessage key={index}>{query.error.message}</ErrorMessage> : null,
      )}
      <div className="grid items-start gap-5 lg:grid-cols-[minmax(220px,280px)_minmax(0,1fr)] lg:grid-rows-[max-content_max-content_minmax(0,1fr)]">
        <div className="min-w-0 space-y-5 lg:col-start-2">
          {node ? (
            <div className="space-y-1">
              <h2 className="text-base font-medium wrap-anywhere">{node.name}</h2>
              {node.description ? (
                <p className="text-xs leading-5 wrap-anywhere whitespace-pre-wrap text-muted-foreground">
                  {node.description}
                </p>
              ) : null}
            </div>
          ) : null}
          {view.canStatistics ? (
            <div aria-label="时间线节点统计" className="grid grid-cols-2 gap-4 md:grid-cols-4">
              {[
                { title: '总人数', value: stats?.total, icon: UsersRound },
                { title: '已完成', value: stats?.completed, icon: CheckCheck },
                { title: '待处理', value: stats?.pending, icon: CircleDashed },
                { title: '已跳过', value: stats?.skipped, icon: SkipForward },
              ].map((metric) => (
                <MetricCard
                  key={metric.title}
                  {...metric}
                  note="当前节点筛选结果"
                  valueSize="large"
                />
              ))}
            </div>
          ) : null}
        </div>
        <div className="min-w-0 lg:col-start-2">{filters}</div>
        <Card
          aria-label="时间线与节点选择"
          className="min-w-0 gap-4 p-4 sm:p-5 lg:col-start-1 lg:row-span-3 lg:row-start-1"
        >
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-1">
            <FilterField label="时间线" htmlFor="timeline-view-template">
              <SearchSelect
                id="timeline-view-template"
                label="时间线"
                className="w-full"
                options={view.options}
                value={String(view.templateID || '')}
                loading={view.templates.isFetching}
                onSearchChange={view.searchTemplates}
                onValueChange={view.selectTemplate}
              />
            </FilterField>
            <div className="min-w-0 lg:hidden">
              <FilterField label="节点" htmlFor="timeline-view-node">
                <SearchSelect
                  id="timeline-view-node"
                  label="节点"
                  className="w-full"
                  options={view.nodes.map((item, index) => ({
                    value: String(item.numeric_id),
                    label: `${String(index + 1)}. ${item.name}`,
                  }))}
                  value={String(node?.numeric_id ?? '')}
                  placeholder={view.nodes.length ? '选择节点' : '暂无可用节点'}
                  disabled={!view.nodes.length}
                  loading={
                    view.template.isFetching || view.query.isFetching || view.statistics.isFetching
                  }
                  onValueChange={(value) => {
                    if (value && value !== String(node?.numeric_id)) {
                      view.selectNode(Number(value));
                    }
                  }}
                />
              </FilterField>
            </div>
          </div>
          <div className="hidden max-h-[min(65vh,640px)] overflow-y-auto overscroll-contain pr-1 lg:block">
            <TimelineNodeGroups
              nodes={view.nodes}
              label="时间线视图节点"
              groupStartClassName="not-first:pt-5"
              renderNode={(item, index, isLastInGroup) => (
                <li
                  key={item.id}
                  className={cn(
                    'relative ml-3 border-l pl-6',
                    isLastInGroup ? 'border-transparent' : 'border-border pb-4',
                  )}
                >
                  <span
                    aria-hidden="true"
                    className={`absolute -left-3 flex size-6 items-center justify-center rounded-full text-xs ${item.id === node?.id ? 'bg-primary text-primary-foreground' : 'bg-secondary text-foreground-secondary'}`}
                  >
                    {String(index + 1)}
                  </span>
                  <Tooltip content={item.name} onlyOverflow>
                    <Button
                      variant={item.id === node?.id ? 'secondary' : 'ghost'}
                      className="h-auto w-full min-w-0 justify-start px-3 py-2 text-left"
                      aria-label={item.name}
                      aria-pressed={item.id === node?.id}
                      loading={
                        item.id === node?.id &&
                        (view.query.isFetching || view.statistics.isFetching)
                      }
                      onClick={() => {
                        view.selectNode(item.numeric_id);
                      }}
                    >
                      <span data-tooltip-overflow className="min-w-0 truncate">
                        {item.name}
                      </span>
                      {item.is_baseline ? <BaselineNodeIcon /> : null}
                    </Button>
                  </Tooltip>
                </li>
              )}
            />
            {!view.loading && !view.nodes.length ? (
              <p className="text-sm text-muted-foreground">暂无可查看的时间线</p>
            ) : null}
          </div>
        </Card>
        <Card className="min-w-0 gap-0 overflow-hidden py-0 lg:col-start-2">
          <ManagementTable
            refreshAction={refreshAction}
            tableID={`cpc-person-timeline:${workspaceID}:${String(view.templateID)}:${node?.id ?? 'none'}`}
            pinnedColumn="person_name"
            primaryColumn={{
              id: 'person_name',
              getAction: ({ person }) => {
                if (!canDetail) return undefined;
                const key = `name:${String(person.id)}`;
                return {
                  loading: read.isPending(key),
                  onClick: () => {
                    openDetail(person, key);
                  },
                };
              },
            }}
            columns={columns}
            data={data?.items ?? []}
            total={data?.total ?? 0}
            page={pagination.page}
            pageSize={pagination.pageSize}
            loading={view.loading || view.query.isFetching}
            emptyMessage={node ? '暂无符合条件的人员' : '请选择时间线及节点'}
            onPage={(page) => {
              pagination.setPage(page);
              return view.fetchPage(page, pagination.pageSize);
            }}
            onPageSize={(size) => {
              pagination.setPageSize(size);
              return view.fetchPage(1, size);
            }}
          />
        </Card>
      </div>
      {data &&
      timelinePerson &&
      (canDetail ||
        (canUpdate && timelinePerson.can_update) ||
        (canReview && timelinePerson.can_review_timeline_transfer)) ? (
        <PersonTimelineDrawer
          key={timelinePerson.id}
          workspaceID={workspaceID}
          person={timelinePerson}
          canUpdate={canUpdate && timelinePerson.can_update}
          canReview={canReview && timelinePerson.can_review_timeline_transfer === true}
          onClose={() => {
            setTimelinePerson(null);
          }}
        />
      ) : null}
      {data &&
      selection &&
      (selection.editing ? canUpdate && selection.person.can_update : canDetail) ? (
        <PersonTimelineNodeDrawer
          key={`${String(selection.person.id)}:${selection.node.id}:${String(selection.editing)}`}
          workspaceID={workspaceID}
          selection={selection}
          onClose={() => {
            setSelection(null);
          }}
        />
      ) : null}
    </div>
  );
}
