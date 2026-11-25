import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import {
  ChevronDown,
  Download,
  Eye,
  GitBranch,
  Pencil,
  Plus,
  RefreshCw,
  Trash2,
  Upload,
} from 'lucide-react';
import { DropdownMenu } from 'radix-ui';
import { useState } from 'react';
import { useSearchParams } from 'react-router';

import { ErrorMessage } from '@/components/error-message';
import { ManagementTable } from '@/components/management-table';
import { TableActionButton } from '@/components/table-action-button';
import { TableActionsMenu } from '@/components/table-actions-menu';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Drawer } from '@/components/ui/drawer';
import { LoadingIcon } from '@/components/ui/loading-icon';
import {
  defaultFilters,
  deletePerson,
  exportPeople,
  type Person,
  personDetailOptions,
  personFilterParams,
  type PersonFilters,
  personKey,
  personListOptions,
  personStatisticsOptions,
  savePersonFile,
  stageLabel,
  timelineStatusLabel,
} from '@/features/cpc-person/api';
import { PersonChart } from '@/features/cpc-person/person-chart';
import { PersonDetail } from '@/features/cpc-person/person-detail';
import { PersonEditor } from '@/features/cpc-person/person-editor';
import { PersonFilterPanel } from '@/features/cpc-person/person-filters';
import { PersonTimelineDrawer } from '@/features/cpc-person/person-timeline';
import { PersonTransfer } from '@/features/cpc-person/person-transfer';
import { PersonTimelineView } from '@/features/cpc-person/timeline-view';
import { usePersonTimelineView } from '@/features/cpc-person/timeline-view-state';
import type { WorkspaceContext } from '@/features/workspace/context';
import { cpcPeopleTabs as views } from '@/features/workspace/navigation';
import { ApiError } from '@/lib/api';
import { useApiAction } from '@/lib/api-action';
import { useFilterForm } from '@/lib/filter-form';
import { formatIdentity } from '@/lib/format';
import { useNavigationTabs } from '@/lib/navigation-tabs';
import { useTablePagination } from '@/lib/pagination';
import { recordTimeColumns } from '@/lib/record-time-columns';

export function PersonPage(props: WorkspaceContext) {
  return <PersonWorkspace key={props.workspaceID} {...props} />;
}

function PersonWorkspace({ workspaceID, actions, refresh: refreshWorkspace }: WorkspaceContext) {
  const [params, setParams] = useSearchParams();
  const timelineID = params.get('person_id');
  const personID = Number(timelineID);
  const validTimelineID =
    timelineID !== null && /^[1-9]\d*$/u.test(timelineID) && Number.isSafeInteger(personID);
  const can = (operation: string) => actions.includes(`cpc.person.${operation}`);
  const canList = can('list');
  const { selected } = useNavigationTabs(canList ? views : []);
  const activeView = selected?.value ?? 'people';
  const isTimeline = activeView === 'timeline';
  const [filters, setFilters] = useState<PersonFilters>(defaultFilters);
  const [editing, setEditing] = useState<Person | null | undefined>();
  const [detail, setDetail] = useState<Person | null>(null);
  const [transfer, setTransfer] = useState(false);
  const peoplePagination = useTablePagination();
  const client = useQueryClient();
  const refresh = useApiAction();
  const read = useApiAction();
  const download = useApiAction();
  const timelineView = usePersonTimelineView({
    workspaceID,
    filters,
    active: isTimeline,
    canList,
    canStatistics: can('statistics'),
  });
  const pagination = isTimeline ? timelineView.pagination : peoplePagination;
  const query = useQuery({
    ...personListOptions(workspaceID, filters, peoplePagination.page, peoplePagination.pageSize),
    enabled: canList && !isTimeline,
    placeholderData: keepPreviousData,
  });
  const denied = query.error instanceof ApiError && [401, 403, 404].includes(query.error.status);
  const peopleData = denied || !canList ? undefined : query.data;
  const data = isTimeline ? timelineView.data : peopleData;
  const timeline = isTimeline
    ? timelineView.data?.items.find(({ person }) => person.id === personID)?.person
    : peopleData?.items.find((person) => person.id === personID);
  const canOpenTimeline = can('detail') || can('update') || can('timeline_transfer.review');
  const reviewMode = params.get('timeline_mode') === 'review' && can('timeline_transfer.review');
  const statistics = useQuery({
    ...personStatisticsOptions(workspaceID, filters),
    enabled: can('statistics') && !isTimeline,
    placeholderData: keepPreviousData,
  });
  const statsDenied =
    statistics.error instanceof ApiError && [401, 403, 404].includes(statistics.error.status);
  const fetchPage = (page: number, size: number, values = filters) =>
    isTimeline
      ? timelineView.fetchPage(page, size, values)
      : client.query({ ...personListOptions(workspaceID, values, page, size), staleTime: 0 });
  const apply = async (values: PersonFilters) => {
    personFilterParams(values, isTimeline);
    peoplePagination.setPage(1);
    timelineView.pagination.setPage(1);
    setFilters(values);
    await Promise.all([
      fetchPage(1, pagination.pageSize, values),
      ...(can('statistics')
        ? [
            isTimeline
              ? timelineView.fetchStatistics(values)
              : client.query({ ...personStatisticsOptions(workspaceID, values), staleTime: 0 }),
          ]
        : []),
    ]);
  };
  const { draft, setDraft, changeFilters, applyChanges, hasUnappliedChanges } = useFilterForm(
    defaultFilters,
    apply,
    filters,
  );
  const openDetail = (id: number, key: string) => {
    void read.run(key, async () => {
      const person = await client.query({ ...personDetailOptions(workspaceID, id), staleTime: 0 });
      setDetail(person);
    });
  };
  const renderActions = (person: Person) => (
    <div className="flex items-center gap-2">
      {can('detail') ||
      (can('update') && person.can_update) ||
      (can('timeline_transfer.review') && person.can_review_timeline_transfer) ? (
        <TableActionButton
          onClick={() => {
            // 抽屉开关同步提交，避免页面过渡快照遮挡右侧面板。
            setParams(
              (current) => {
                current.set('person_id', String(person.id));
                current.delete('timeline_mode');
                return current;
              },
              { flushSync: true, preventScrollReset: true },
            );
          }}
        >
          <GitBranch aria-hidden="true" />
          {!person.timeline_template_id && person.can_update ? '绑定时间线' : '时间线'}
        </TableActionButton>
      ) : null}
      {can('detail') ? (
        <TableActionButton
          loading={read.isPending(String(person.id))}
          onClick={() => {
            openDetail(person.id, String(person.id));
          }}
        >
          <Eye aria-hidden="true" />
          详情
        </TableActionButton>
      ) : null}
      {can('update') && person.can_update && person.timeline_status !== 'transferring' ? (
        <TableActionButton
          onClick={() => {
            setEditing(person);
          }}
        >
          <Pencil aria-hidden="true" />
          编辑
        </TableActionButton>
      ) : null}
      {can('delete') && person.can_delete && person.timeline_status !== 'transferring' ? (
        <TableActionsMenu
          label={formatIdentity(person.name, person.student_no)}
          actions={[
            {
              label: '删除',
              icon: Trash2,
              destructive: true,
              confirmDelete: {
                target: formatIdentity(person.name, person.student_no),
                onConfirm: async () => {
                  await deletePerson(workspaceID, person);
                  const page = Math.max(
                    1,
                    Math.min(
                      pagination.page,
                      Math.ceil(((data?.total ?? 1) - 1) / pagination.pageSize),
                    ),
                  );
                  pagination.setPage(page);
                  await client.invalidateQueries({ queryKey: personKey(workspaceID) });
                  await fetchPage(page, pagination.pageSize);
                },
              },
            },
          ]}
        />
      ) : null}
    </div>
  );
  const columns: ColumnDef<Person>[] = [
    { accessorKey: 'name', header: '姓名' },
    { accessorKey: 'student_no', header: '学号' },
    { accessorKey: 'grade', header: '年级' },
    { accessorKey: 'status', header: '状态' },
    {
      accessorKey: 'party_branch_short_name',
      header: '所属党支部',
    },
    {
      id: 'party_branch_secretaries',
      accessorFn: (person) => person.party_branch_secretaries.join('、'),
      header: '支部书记',
    },
    {
      accessorKey: 'party_position',
      header: '党内职务',
    },
    { accessorKey: 'stage', header: '发展阶段', cell: ({ row }) => stageLabel(row.original.stage) },
    { accessorKey: 'major', header: '专业' },
    { accessorKey: 'advisor', header: '导师' },
    {
      accessorKey: 'timeline_template_name',
      header: '时间线模板',
      cell: ({ row }) => row.original.timeline_template_name || '未绑定',
    },
    {
      accessorKey: 'timeline_status',
      header: '时间线状态',
      cell: ({ row }) =>
        row.original.timeline_template_id ? (
          <span
            className={
              row.original.timeline_status === 'abnormal'
                ? 'font-semibold text-danger-foreground'
                : row.original.timeline_status === 'transferring'
                  ? 'text-warning-foreground'
                  : 'text-success-foreground'
            }
          >
            {timelineStatusLabel(row.original.timeline_status)}
          </span>
        ) : (
          '未绑定'
        ),
    },
    {
      id: 'timeline_progress',
      header: '节点进度',
      cell: ({ row }) =>
        row.original.timeline_template_id ? (
          <span
            className={
              row.original.timeline_pending > 0
                ? 'text-warning-foreground'
                : 'text-success-foreground'
            }
          >
            {String(row.original.timeline_completed + row.original.timeline_skipped)}/
            {String(
              row.original.timeline_pending +
                row.original.timeline_completed +
                row.original.timeline_skipped,
            )}
            {row.original.timeline_skipped > 0
              ? `（跳过 ${String(row.original.timeline_skipped)}）`
              : ''}
          </span>
        ) : (
          '未绑定'
        ),
    },
    ...recordTimeColumns<Person>((record) => record),
    { id: 'actions', header: '操作' },
  ];
  const refreshButton = (
    <Button
      type="button"
      size="sm"
      variant="ghost"
      loading={
        refresh.pending ||
        (isTimeline
          ? timelineView.loading
          : (canList && query.isPending) || (can('statistics') && statistics.isPending))
      }
      onClick={() => {
        void refresh.run('refresh', async () => {
          await refreshWorkspace();
          if (canList || can('statistics')) {
            await client.invalidateQueries({ queryKey: personKey(workspaceID) });
            if (canList) await fetchPage(pagination.page, pagination.pageSize);
          }
        });
      }}
    >
      <RefreshCw aria-hidden="true" />
      刷新
    </Button>
  );
  const filterPanel = (
    <PersonFilterPanel
      workspaceID={workspaceID}
      applied={filters}
      draft={draft}
      hasUnappliedChanges={hasUnappliedChanges}
      setDraft={setDraft}
      changeFilters={changeFilters}
      applyChanges={applyChanges}
      onApply={apply}
      nodeView={isTimeline}
    />
  );
  return (
    <section className="space-y-5" aria-labelledby="cpc-people-title">
      <h1 id="cpc-people-title" className="sr-only">
        人员名单
      </h1>
      <header className="flex min-h-8 flex-wrap items-center justify-between gap-3">
        <nav aria-label="面包屑" className="text-xs text-muted-foreground">
          <ol className="flex flex-wrap items-center gap-2">
            <li>党建工作</li>
            <li aria-hidden="true">/</li>
            <li aria-current={selected ? undefined : 'page'}>人员名单</li>
            {selected ? (
              <>
                <li aria-hidden="true">/</li>
                <li aria-current="page">{selected.label}</li>
              </>
            ) : null}
          </ol>
        </nav>
        <div className="ml-auto flex flex-wrap gap-2">
          {data?.can_import && can('import') ? (
            <Button
              variant="outline"
              onClick={() => {
                setTransfer(true);
              }}
            >
              <Upload aria-hidden="true" />
              导入
            </Button>
          ) : null}
          {data?.can_export && can('export') ? (
            <span className="inline-flex items-center gap-2">
              <DropdownMenu.Root modal={false}>
                <DropdownMenu.Trigger asChild>
                  <Button variant="outline" loading={download.pending} className="group">
                    <Download aria-hidden="true" />
                    导出
                    <ChevronDown
                      aria-hidden="true"
                      className="size-3 transition-transform duration-200 group-data-[state=open]:rotate-180 motion-reduce:transition-none"
                    />
                  </Button>
                </DropdownMenu.Trigger>
                <DropdownMenu.Portal>
                  <DropdownMenu.Content
                    align="end"
                    sideOffset={8}
                    className="glass-panel menu-motion z-50 min-w-40 rounded-lg border bg-popover p-1"
                  >
                    {(
                      [
                        { value: 'basic', label: '基础数据' },
                        { value: 'full', label: '完整数据' },
                      ] as const
                    ).map((format) => (
                      <DropdownMenu.Item
                        key={format.value}
                        className="flex cursor-pointer items-center gap-2 rounded-md px-3 py-2 text-sm outline-none data-disabled:pointer-events-none data-disabled:opacity-50 data-highlighted:bg-accent"
                        disabled={download.pending}
                        onSelect={() => {
                          void download.run(format.value, async () => {
                            savePersonFile(
                              await exportPeople(
                                workspaceID,
                                filters,
                                format.value,
                                isTimeline ? timelineView.templateID : undefined,
                                isTimeline ? timelineView.node?.numeric_id : undefined,
                              ),
                            );
                          });
                        }}
                      >
                        <LoadingIcon
                          loading={download.isPending(format.value)}
                          fallback={<Download aria-hidden="true" className="size-4" />}
                        />
                        {format.label}
                      </DropdownMenu.Item>
                    ))}
                  </DropdownMenu.Content>
                </DropdownMenu.Portal>
              </DropdownMenu.Root>
            </span>
          ) : null}
          {data?.can_create && can('create') ? (
            <Button
              onClick={() => {
                setEditing(null);
              }}
            >
              <Plus aria-hidden="true" />
              新增人员
            </Button>
          ) : null}
          {!canList ? refreshButton : null}
        </div>
      </header>
      <div
        role={canList ? 'region' : undefined}
        id={`cpc-people-panel-${activeView}`}
        aria-label={selected?.label}
        className="space-y-5"
      >
        {isTimeline && canList ? (
          <PersonTimelineView
            refreshAction={refreshButton}
            view={timelineView}
            filters={filterPanel}
            workspaceID={workspaceID}
            actions={actions}
          />
        ) : (
          <>
            {query.error ? <ErrorMessage>{query.error.message}</ErrorMessage> : null}
            {statistics.error ? <ErrorMessage>{statistics.error.message}</ErrorMessage> : null}
            {can('statistics') ? (
              <PersonChart data={statsDenied ? undefined : statistics.data} />
            ) : null}
            {canList ? (
              <>
                {filterPanel}
                <Card className="gap-0 overflow-hidden py-0">
                  <ManagementTable
                    refreshAction={refreshButton}
                    title="人员列表"
                    tableID="cpc-people"
                    rowID={(person) => String(person.id)}
                    pinnedColumn="name"
                    primaryColumn={{
                      id: 'name',
                      getAction: ({ id }) => {
                        if (!can('detail')) return undefined;
                        const key = `name:${String(id)}`;
                        return {
                          loading: read.isPending(key),
                          onClick: () => {
                            openDetail(id, key);
                          },
                        };
                      },
                    }}
                    rowActions={renderActions}
                    columns={columns}
                    data={peopleData?.items ?? []}
                    total={peopleData?.total ?? 0}
                    page={pagination.page}
                    pageSize={pagination.pageSize}
                    loading={query.isFetching}
                    emptyMessage="暂无符合条件的人员"
                    onPage={async (page) => {
                      pagination.setPage(page);
                      return fetchPage(page, pagination.pageSize);
                    }}
                    onPageSize={async (size) => {
                      pagination.setPageSize(size);
                      return fetchPage(1, size);
                    }}
                  />
                </Card>
              </>
            ) : (
              <Card className="p-6 text-sm text-muted-foreground">暂无查看人员名单的权限</Card>
            )}
          </>
        )}
      </div>
      {data && editing !== undefined && can(editing ? 'update' : 'create') ? (
        <PersonEditor
          workspaceID={workspaceID}
          person={editing}
          onClose={() => {
            setEditing(undefined);
          }}
        />
      ) : null}
      {data && transfer && can('import') ? (
        <PersonTransfer
          workspaceID={workspaceID}
          onClose={() => {
            setTransfer(false);
          }}
        />
      ) : null}
      {timelineID !== null && (!validTimelineID || !canOpenTimeline) ? (
        <ErrorMessage>
          {!validTimelineID ? '人员时间线地址无效' : '没有查看人员时间线的权限'}
        </ErrorMessage>
      ) : null}
      {validTimelineID && canOpenTimeline ? (
        <PersonTimelineDrawer
          key={`${String(personID)}:${String(reviewMode)}`}
          workspaceID={workspaceID}
          person={timeline ?? { id: personID, name: '', student_no: '' }}
          canUpdate={!reviewMode && can('update') && (timeline?.can_update ?? true)}
          canReview={
            can('timeline_transfer.review') && (timeline?.can_review_timeline_transfer ?? true)
          }
          onClose={() => {
            setParams(
              (current) => {
                current.delete('person_id');
                current.delete('timeline_mode');
                return current;
              },
              { replace: true, flushSync: true, preventScrollReset: true },
            );
          }}
        />
      ) : null}
      {data && detail && can('detail') ? (
        <Drawer
          title="党建人员详情"
          truncateDescription
          description={formatIdentity(detail.name, detail.student_no)}
          onClose={() => {
            setDetail(null);
          }}
        >
          <PersonDetail person={detail} />
        </Drawer>
      ) : null}
    </section>
  );
}
