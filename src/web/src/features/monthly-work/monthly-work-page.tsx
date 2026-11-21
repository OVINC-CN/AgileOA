import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  CalendarDays,
  ExternalLink,
  Inbox,
  Paperclip,
  Pencil,
  Plus,
  RefreshCw,
  Search,
  Trash2,
  Upload,
} from 'lucide-react';
import { useEffect, useRef, useState } from 'react';

import { AttachmentFilename } from '@/components/attachment-filename';
import { ErrorMessage } from '@/components/error-message';
import { FilterField, FilterPanel } from '@/components/filter-panel';
import { SearchResetButton } from '@/components/search-reset-button';
import { TableActionButton } from '@/components/table-action-button';
import { TableActionsMenu } from '@/components/table-actions-menu';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Drawer } from '@/components/ui/drawer';
import { Input } from '@/components/ui/input';
import { SearchSelect } from '@/components/ui/search-select';
import {
  deleteWork,
  workAttachmentDownload,
  workFilterOptions,
  type WorkFilters,
  workKey,
  workListOptions,
  type WorkRecord,
} from '@/features/monthly-work/api';
import { WorkAttachments } from '@/features/monthly-work/attachments';
import { workAction, type WorkCategory } from '@/features/monthly-work/category';
import type { WorkTarget } from '@/features/monthly-work/navigation';
import { groupWorkRecords, monthLabel } from '@/features/monthly-work/timeline';
import { WorkChart } from '@/features/monthly-work/work-chart';
import { WorkEditor } from '@/features/monthly-work/work-editor';
import { WorkTransfer } from '@/features/monthly-work/work-transfer';
import { currentUserOptions } from '@/features/user/api';
import type { WorkspaceContext } from '@/features/workspace/context';
import { ApiError } from '@/lib/api';
import { useApiAction } from '@/lib/api-action';
import { currentCalendarDate } from '@/lib/calendar';
import { useFilterForm } from '@/lib/filter-form';
import { formatIdentity, formatValue } from '@/lib/format';
import { useLoadingDelay } from '@/lib/loading';
import { cn } from '@/lib/utils';

function defaultWorkFilters(): WorkFilters {
  return { year: [currentCalendarDate().slice(0, 4)], month: [], keyword: '' };
}

type MonthlyWorkProps = WorkspaceContext & {
  category?: WorkCategory;
  target?: WorkTarget | undefined;
};

export function MonthlyWorkPage(props: MonthlyWorkProps) {
  return (
    <MonthlyWorkWorkspace
      key={[
        props.workspaceID,
        props.category ?? 'student',
        props.target?.id,
        props.target?.year,
        props.target?.month,
      ].join(':')}
      {...props}
    />
  );
}

function MonthlyWorkWorkspace({
  workspaceID,
  category = 'student',
  actions,
  refresh: refreshWorkspace,
  target,
}: MonthlyWorkProps) {
  const can = (operation: string) => actions.includes(workAction(category, operation));
  const canList = can('list');
  const canReadFiles = can('attachment.read');
  // 复用鉴权入口已读取的登录态，不额外请求用户接口。
  const { data: user } = useQuery({ ...currentUserOptions, enabled: false });
  const userID = user?.id;
  const [filters, setFilters] = useState<WorkFilters>(() =>
    target ? { year: [target.year], month: [target.month], keyword: '' } : defaultWorkFilters(),
  );
  const targetRecord = useRef<HTMLLIElement>(null);
  const [editing, setEditing] = useState<WorkRecord | null | undefined>();
  const [deleting, setDeleting] = useState<WorkRecord | null>(null);
  const [attachmentWorkID, setAttachmentWorkID] = useState<number | null>(null);
  const [transfer, setTransfer] = useState(false);
  const client = useQueryClient();
  const refresh = useApiAction();
  const search = useApiAction();
  const remove = useApiAction();
  const query = useQuery({
    ...workListOptions(workspaceID, filters, category),
    enabled: canList,
    placeholderData: keepPreviousData,
  });
  const denied = query.error instanceof ApiError && [401, 403, 404].includes(query.error.status);
  const data = denied || !canList ? undefined : query.data;
  const findWork = (id: number | null | undefined) =>
    data?.items.find((record) => record.id === id);
  const attachments = findWork(attachmentWorkID);
  const isOwner = (record: WorkRecord | undefined) =>
    record !== undefined && record.owner_id === userID;
  const canOpenAttachments = (record: WorkRecord) =>
    canReadFiles || (isOwner(record) && can('attachment.upload'));
  const canManageFiles = (record: WorkRecord) =>
    isOwner(record) && (can('attachment.upload') || can('attachment.delete'));
  const canChangeWork = (record: WorkRecord | undefined, operation: 'update' | 'delete') =>
    isOwner(record) && can(operation);
  const targetVisible = data?.items.some((record) => record.id === target?.id) ?? false;
  useEffect(() => {
    // 通知按目标年月查询，记录可见时滚动到对应卡片。
    if (targetVisible && typeof targetRecord.current?.scrollIntoView === 'function') {
      targetRecord.current.scrollIntoView({ block: 'center' });
    }
  }, [targetVisible]);
  const loading = useLoadingDelay(query.isFetching);
  const filterOptions = useQuery({ ...workFilterOptions(workspaceID, category), enabled: canList });
  const filterDenied =
    filterOptions.error instanceof ApiError && [401, 403, 404].includes(filterOptions.error.status);
  const fetchRecords = (nextFilters = filters) =>
    client.query({ ...workListOptions(workspaceID, nextFilters, category), staleTime: 0 });
  const applyFilters = async (values: WorkFilters) => {
    const next = { ...values, keyword: values.keyword.trim() };
    setFilters(next);
    await fetchRecords(next);
  };
  const { draft, setDraft, changeFilters, hasUnappliedChanges } = useFilterForm(
    filters,
    applyFilters,
    filters,
  );
  const yearOptions = [
    ...new Map(
      [
        {
          value: currentCalendarDate().slice(0, 4),
          label: `${currentCalendarDate().slice(0, 4)} 年`,
        },
        ...draft.year.map((year) => ({ value: year, label: `${year} 年` })),
        ...(filterDenied ? [] : (filterOptions.data?.years ?? [])),
      ].map((option) => [option.value, option]),
    ).values(),
  ].sort((first, second) => second.value.localeCompare(first.value));
  const refreshButton = (
    <Button
      type="button"
      size="sm"
      variant="ghost"
      loading={refresh.pending || (canList && (query.isPending || filterOptions.isPending))}
      onClick={() => {
        void refresh.run('refresh', async () => {
          await refreshWorkspace();
          if (canList)
            await Promise.all([
              fetchRecords(),
              client.query({ ...workFilterOptions(workspaceID, category), staleTime: 0 }),
            ]);
        });
      }}
    >
      <RefreshCw aria-hidden="true" />
      刷新
    </Button>
  );
  return (
    <section className="space-y-5" aria-labelledby="monthly-work-title">
      <header className="flex min-h-8 flex-wrap items-center justify-between gap-3">
        <h1 id="monthly-work-title" className="sr-only">
          月度工作
        </h1>
        <nav aria-label="面包屑" className="text-xs text-muted-foreground">
          <ol className="flex items-center gap-2">
            <li>{category === 'cpc' ? '党建工作' : '学生工作'}</li>
            <li aria-hidden="true">/</li>
            <li aria-current="page">月度工作</li>
          </ol>
        </nav>
        <div className="ml-auto flex flex-wrap gap-2">
          {can('import') ? (
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
          {can('create') ? (
            <Button
              onClick={() => {
                setEditing(null);
              }}
            >
              <Plus aria-hidden="true" />
              新增工作
            </Button>
          ) : null}
          {!canList ? refreshButton : null}
        </div>
      </header>
      {canList ? <WorkChart records={data?.items} userID={userID} /> : null}
      {canList ? (
        <FilterPanel
          refreshAction={refreshButton}
          hasUnappliedChanges={hasUnappliedChanges}
          onSubmit={(event) => {
            event.preventDefault();
            void search.run('search', () => applyFilters(draft));
          }}
          actions={
            <>
              <SearchResetButton
                loading={search.isPending('reset')}
                disabled={search.pending}
                onClick={() => {
                  void search.run('reset', async () => {
                    const defaults = defaultWorkFilters();
                    setDraft(defaults);
                    await applyFilters(defaults);
                  });
                }}
              />
              <Button type="submit" loading={search.isPending('search')} disabled={search.pending}>
                <Search aria-hidden="true" />
                搜索
              </Button>
            </>
          }
        >
          <FilterField label="年份" htmlFor="work-year">
            <SearchSelect
              multiple
              id="work-year"
              label="年份"
              className="w-full"
              options={yearOptions}
              value={draft.year}
              loading={filterOptions.isFetching}
              onValueChange={(year) => {
                changeFilters({ year });
              }}
            />
          </FilterField>
          <FilterField label="月份" htmlFor="work-month">
            <SearchSelect
              multiple
              id="work-month"
              label="月份"
              className="w-full"
              options={[
                ...Array.from({ length: 12 }, (_, index) => ({
                  value: String(index + 1),
                  label: `${String(index + 1)} 月`,
                })),
              ]}
              value={draft.month}
              onValueChange={(month) => {
                changeFilters({ month });
              }}
            />
          </FilterField>
          <FilterField label="关键词" htmlFor="work-keyword">
            <Input
              id="work-keyword"
              placeholder="工作项目、负责人、参与人或详细说明"
              maxLength={100}
              value={draft.keyword}
              onChange={(event) => {
                setDraft((current) => ({ ...current, keyword: event.target.value }));
              }}
            />
          </FilterField>
        </FilterPanel>
      ) : null}
      {canList && filterOptions.error ? (
        <ErrorMessage>{filterOptions.error.message}</ErrorMessage>
      ) : null}
      {canList && query.error ? <ErrorMessage>{query.error.message}</ErrorMessage> : null}
      {canList ? (
        <section aria-label="月度工作时间轴" aria-busy={loading}>
          {groupWorkRecords(data?.items ?? []).map(({ month, items }) => (
            <section
              key={month}
              aria-labelledby={`work-month-${month}`}
              className="relative min-w-0 pb-6 before:hidden last:pb-0 lg:ml-2 lg:pb-7 lg:pl-6 lg:before:absolute lg:before:top-2.5 lg:before:-bottom-2.5 lg:before:left-0 lg:before:block lg:before:w-px lg:before:-translate-x-1/2 lg:before:bg-border lg:last:before:hidden"
            >
              <h2
                id={`work-month-${month}`}
                className="mb-3 flex items-center gap-2 text-base leading-5 font-medium tabular-nums lg:mb-4"
              >
                <span
                  aria-hidden="true"
                  className="absolute top-1 -left-1.5 hidden size-3 rounded-full border-2 border-muted-foreground bg-background ring-4 ring-background lg:block"
                />
                <CalendarDays className="size-4 text-muted-foreground" aria-hidden="true" />
                {monthLabel(month)}
              </h2>
              <ol className="min-w-0 space-y-4">
                {items.map((record) => (
                  <li
                    key={record.id}
                    ref={record.id === target?.id ? targetRecord : undefined}
                    className="min-w-0"
                  >
                    <Card
                      role="article"
                      aria-labelledby={`work-item-${String(record.id)}`}
                      className={cn(
                        'grid min-w-0 gap-3 p-4 text-sm leading-5 sm:p-5 lg:grid-cols-[minmax(0,1fr)_auto]',
                        record.id === target?.id && 'border-primary/50',
                      )}
                    >
                      <div className="min-w-0 space-y-1 lg:col-start-1 lg:row-start-1">
                        <h3
                          id={`work-item-${String(record.id)}`}
                          className="text-sm leading-5 font-medium wrap-anywhere"
                        >
                          {record.work_item}
                        </h3>
                        <time
                          dateTime={record.date}
                          className="block text-xs text-muted-foreground tabular-nums"
                        >
                          {record.date}
                        </time>
                      </div>
                      <dl className="grid min-w-0 gap-2 text-sm leading-5 lg:col-span-2">
                        <div className="grid min-w-0 grid-cols-[auto_minmax(0,1fr)] items-baseline gap-x-2">
                          <dt className="text-muted-foreground">负责人：</dt>
                          <dd className="min-w-0 wrap-anywhere text-foreground-secondary">
                            {formatIdentity(record.owner_name, record.owner_username)}
                          </dd>
                        </div>
                        <div className="grid min-w-0 grid-cols-[auto_minmax(0,1fr)] items-baseline gap-x-2">
                          <dt className="text-muted-foreground">参与人：</dt>
                          <dd className="min-w-0 text-foreground-secondary">
                            {record.participants.length ? (
                              <ul className="space-y-1 lg:flex lg:flex-wrap lg:space-y-0 lg:gap-x-3 lg:gap-y-1">
                                {record.participants.map((person) => (
                                  <li key={person.id} className="min-w-0 wrap-anywhere">
                                    {formatIdentity(person.name, person.username)}
                                  </li>
                                ))}
                              </ul>
                            ) : (
                              formatValue(null)
                            )}
                          </dd>
                        </div>
                      </dl>
                      {record.description ? (
                        <p className="min-w-0 text-sm leading-5 wrap-anywhere whitespace-pre-wrap text-foreground-secondary lg:col-span-2">
                          {record.description}
                        </p>
                      ) : null}
                      {record.links.length || (canReadFiles && record.attachments.length) ? (
                        <ul
                          aria-label="链接和附件"
                          className="grid min-w-0 gap-2 border-t border-border pt-3 text-sm leading-5 lg:col-span-2 lg:flex lg:flex-wrap lg:items-start lg:gap-x-5"
                        >
                          {record.links.map((link, index) => (
                            <li key={`link-${String(index)}`} className="min-w-0 lg:max-w-full">
                              <a
                                href={link.url}
                                target="_blank"
                                rel="noopener noreferrer"
                                title={link.url}
                                className="flex min-h-8 w-full max-w-full min-w-0 items-center gap-2 rounded-sm text-foreground-secondary underline-offset-4 hover:text-foreground hover:underline focus-visible:outline-2 focus-visible:outline-ring lg:inline-flex lg:min-h-0 lg:w-auto"
                              >
                                <ExternalLink className="size-4 shrink-0" aria-hidden="true" />
                                <span className="min-w-0 truncate">{link.name || link.url}</span>
                                <span className="sr-only">（新窗口打开）</span>
                              </a>
                            </li>
                          ))}
                          {canReadFiles
                            ? record.attachments.map((file) => (
                                <li
                                  key={`file-${String(file.id)}`}
                                  className="min-w-0 lg:max-w-full"
                                >
                                  <div className="flex min-w-0 items-center gap-2 text-foreground-secondary">
                                    <Paperclip className="size-4 shrink-0" aria-hidden="true" />
                                    <AttachmentFilename
                                      filename={file.filename}
                                      className="min-h-8 max-w-full flex-1 lg:min-h-0"
                                      downloadURL={workAttachmentDownload(
                                        workspaceID,
                                        file.id,
                                        category,
                                      )}
                                    />
                                  </div>
                                </li>
                              ))
                            : null}
                        </ul>
                      ) : null}
                      {canOpenAttachments(record) ||
                      canChangeWork(record, 'update') ||
                      canChangeWork(record, 'delete') ? (
                        <div className="flex min-w-0 flex-wrap items-center gap-1 border-t border-border pt-3 lg:col-start-2 lg:row-start-1 lg:justify-end lg:self-start lg:border-0 lg:pt-0">
                          {canOpenAttachments(record) ? (
                            <TableActionButton
                              onClick={() => {
                                setAttachmentWorkID(record.id);
                              }}
                            >
                              <Paperclip aria-hidden="true" />
                              {canManageFiles(record) ? '管理附件' : '查看附件'}
                            </TableActionButton>
                          ) : null}
                          {canChangeWork(record, 'update') ? (
                            <TableActionButton
                              onClick={() => {
                                setEditing(record);
                              }}
                            >
                              <Pencil aria-hidden="true" />
                              编辑
                            </TableActionButton>
                          ) : null}
                          {canChangeWork(record, 'delete') ? (
                            <TableActionsMenu
                              label={record.work_item}
                              loading={remove.isPending(String(record.id))}
                              disabled={remove.pending}
                              actions={[
                                {
                                  label: '删除',
                                  icon: Trash2,
                                  destructive: true,
                                  onSelect: () => {
                                    setDeleting(record);
                                  },
                                },
                              ]}
                            />
                          ) : null}
                        </div>
                      ) : null}
                    </Card>
                  </li>
                ))}
              </ol>
            </section>
          ))}
          {!query.isPending && !data?.items.length ? (
            <Card className="items-center gap-3 p-12 text-sm text-muted-foreground">
              <Inbox className="size-7 opacity-50" aria-hidden="true" />
              暂无符合条件的月度工作
            </Card>
          ) : null}
        </section>
      ) : (
        <Card className="p-6 text-sm text-muted-foreground">没有查看月度工作的权限</Card>
      )}
      {editing !== undefined &&
      (editing ? canChangeWork(findWork(editing.id), 'update') : can('create')) ? (
        <WorkEditor
          workspaceID={workspaceID}
          category={category}
          record={editing}
          actions={actions}
          currentUser={user}
          onClose={() => {
            setEditing(undefined);
          }}
        />
      ) : null}
      {attachments && canOpenAttachments(attachments) ? (
        <WorkAttachments
          workspaceID={workspaceID}
          category={category}
          record={attachments}
          actions={actions}
          isOwner={isOwner(attachments)}
          onClose={() => {
            setAttachmentWorkID(null);
          }}
        />
      ) : null}
      {transfer ? (
        <WorkTransfer
          workspaceID={workspaceID}
          category={category}
          onClose={() => {
            setTransfer(false);
          }}
        />
      ) : null}
      {deleting && canChangeWork(findWork(deleting.id), 'delete') ? (
        <Drawer
          title="删除月度工作"
          description={`${deleting.date} · ${deleting.work_item}`}
          busy={remove.pending}
          onClose={() => {
            setDeleting(null);
          }}
          footer={
            <>
              <Button
                variant="outline"
                disabled={remove.pending}
                onClick={() => {
                  setDeleting(null);
                }}
              >
                取消
              </Button>
              <Button
                loading={remove.pending}
                onClick={() => {
                  void (async () => {
                    const result = await remove.run(String(deleting.id), async () => {
                      await deleteWork(workspaceID, deleting, category);
                      await client.invalidateQueries({ queryKey: workKey(workspaceID, category) });
                    });
                    if (result.ok) setDeleting(null);
                  })();
                }}
              >
                <Trash2 aria-hidden="true" />
                确认删除
              </Button>
            </>
          }
        >
          <p className="p-6 text-sm leading-6">
            将永久删除该工作记录、参与人关联及所属附件记录，历史审计保留。
          </p>
        </Drawer>
      ) : null}
    </section>
  );
}
