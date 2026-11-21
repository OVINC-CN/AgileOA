import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { Copy, Eye, Pencil, Plus, RefreshCw, Search, Trash2, Upload } from 'lucide-react';
import { useState } from 'react';

import { ErrorMessage } from '@/components/error-message';
import { FilterField, FilterPanel } from '@/components/filter-panel';
import { ManagementTable } from '@/components/management-table';
import { SearchResetButton } from '@/components/search-reset-button';
import { TableActionButton } from '@/components/table-action-button';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Drawer } from '@/components/ui/drawer';
import { Input } from '@/components/ui/input';
import {
  cloneTimeline,
  deleteTimeline,
  timelineDetailOptions,
  type TimelineInfo,
  timelineKey,
  timelineListOptions,
  type TimelineSummary,
} from '@/features/cpc-timeline/api';
import { TemplateEditor } from '@/features/cpc-timeline/template-editor';
import { TemplateImport } from '@/features/cpc-timeline/template-import';
import { TemplatePreview } from '@/features/cpc-timeline/template-preview';
import type { WorkspaceContext } from '@/features/workspace/context';
import { ApiError } from '@/lib/api';
import { useApiAction } from '@/lib/api-action';
import { formatValue } from '@/lib/format';
import { useTablePagination } from '@/lib/pagination';
import { recordTimeColumns } from '@/lib/record-time-columns';

export function TimelinePage({
  workspaceID,
  actions,
  refresh: refreshWorkspace,
}: WorkspaceContext) {
  const can = (action: string) => actions.includes(`cpc.timeline_template.${action}`);
  const [draft, setDraft] = useState('');
  const [keyword, setKeyword] = useState('');
  const [editor, setEditor] = useState<TimelineInfo | null | undefined>();
  const [detail, setDetail] = useState<TimelineInfo | null>(null);
  const [deleting, setDeleting] = useState<TimelineSummary | null>(null);
  const [importing, setImporting] = useState(false);
  const client = useQueryClient();
  const pagination = useTablePagination();
  const refresh = useApiAction();
  const search = useApiAction();
  const load = useApiAction();
  const remove = useApiAction();
  const clone = useApiAction();
  const query = useQuery({
    ...timelineListOptions(workspaceID, keyword, pagination.page, pagination.pageSize),
    enabled: can('list'),
    placeholderData: keepPreviousData,
  });
  const denied = query.error instanceof ApiError && [401, 403, 404].includes(query.error.status);
  const data = denied ? undefined : query.data;
  const fetchPage = (page: number, size: number, nextKeyword = keyword) =>
    client.query({
      ...timelineListOptions(workspaceID, nextKeyword, page, size),
      staleTime: 0,
    });
  const applySearch = async (value: string) => {
    const next = value.trim();
    setKeyword(next);
    pagination.setPage(1);
    await fetchPage(1, pagination.pageSize, next);
  };
  const open = (record: TimelineSummary, edit: boolean) => {
    void load.run(`${edit ? 'edit' : 'detail'}:${String(record.id)}`, async () => {
      const result = await client.query({
        ...timelineDetailOptions(workspaceID, record.id),
        staleTime: 0,
      });
      if (edit) setEditor(result);
      else setDetail(result);
    });
  };
  const columns: ColumnDef<TimelineSummary>[] = [
    { accessorKey: 'name', header: '模板名称' },
    {
      accessorKey: 'description',
      header: '说明',
      cell: ({ row }) => (
        <p className="max-w-80 truncate" title={row.original.description}>
          {formatValue(row.original.description)}
        </p>
      ),
    },
    { accessorKey: 'node_count', header: '节点数' },
    ...recordTimeColumns<TimelineSummary>((record) => record),
    {
      id: 'actions',
      header: '操作',
      cell: ({ row }) => (
        <div className="flex items-center gap-1">
          {can('detail') ? (
            <TableActionButton
              disabled={load.pending}
              loading={load.isPending(`detail:${String(row.original.id)}`)}
              onClick={() => {
                open(row.original, false);
              }}
            >
              <Eye aria-hidden="true" />
              详情
            </TableActionButton>
          ) : null}
          {can('update') && can('detail') ? (
            <TableActionButton
              disabled={load.pending}
              loading={load.isPending(`edit:${String(row.original.id)}`)}
              onClick={() => {
                open(row.original, true);
              }}
            >
              <Pencil aria-hidden="true" />
              编辑
            </TableActionButton>
          ) : null}
          {can('create') ? (
            <TableActionButton
              disabled={clone.pending}
              loading={clone.isPending(String(row.original.id))}
              onClick={() => {
                void clone.run(String(row.original.id), async () => {
                  await cloneTimeline(workspaceID, row.original);
                  pagination.setPage(1);
                  await client.invalidateQueries({ queryKey: timelineKey(workspaceID) });
                  await fetchPage(1, pagination.pageSize);
                });
              }}
            >
              <Copy aria-hidden="true" />
              克隆
            </TableActionButton>
          ) : null}
          {can('delete') ? (
            <TableActionButton
              onClick={() => {
                setDeleting(row.original);
              }}
            >
              <Trash2 aria-hidden="true" />
              删除
            </TableActionButton>
          ) : null}
        </div>
      ),
    },
  ];
  const refreshButton = (
    <Button
      type="button"
      size="sm"
      variant="ghost"
      loading={refresh.pending || query.isPending}
      onClick={() => {
        void refresh.run('refresh', async () => {
          await refreshWorkspace();
          await fetchPage(pagination.page, pagination.pageSize);
        });
      }}
    >
      <RefreshCw aria-hidden="true" />
      刷新
    </Button>
  );
  return (
    <section className="space-y-5" aria-labelledby="timeline-template-title">
      <h1 id="timeline-template-title" className="sr-only">
        时间线模板
      </h1>
      <header className="flex min-h-8 flex-wrap items-center justify-between gap-3">
        <nav aria-label="面包屑" className="text-xs text-muted-foreground">
          <ol className="flex flex-wrap items-center gap-2">
            <li>党建工作</li>
            <li aria-hidden="true">/</li>
            <li>数据管理</li>
            <li aria-hidden="true">/</li>
            <li aria-current="page">时间线模板</li>
          </ol>
        </nav>
        <div className="ml-auto flex flex-wrap gap-2">
          {can('import') ? (
            <Button
              variant="outline"
              onClick={() => {
                setImporting(true);
              }}
            >
              <Upload aria-hidden="true" />
              导入
            </Button>
          ) : null}
          {can('create') ? (
            <Button
              onClick={() => {
                setEditor(null);
              }}
            >
              <Plus aria-hidden="true" />
              新增模板
            </Button>
          ) : null}
        </div>
      </header>
      {query.error ? <ErrorMessage>{query.error.message}</ErrorMessage> : null}
      <div
        role="region"
        id="cpc-data-panel-timeline-templates"
        aria-label="时间线模板"
        className="space-y-4"
      >
        <FilterPanel
          onSubmit={(event) => {
            event.preventDefault();
            void search.run('search', () => applySearch(draft));
          }}
          actions={
            <>
              <SearchResetButton
                disabled={search.pending}
                loading={search.isPending('reset')}
                onClick={() => {
                  void search.run('reset', async () => {
                    setDraft('');
                    await applySearch('');
                  });
                }}
              />
              <Button type="submit" disabled={search.pending} loading={search.isPending('search')}>
                <Search aria-hidden="true" />
                搜索
              </Button>
            </>
          }
        >
          <FilterField label="模板名称或说明" htmlFor="template-page-keyword">
            <Input
              id="template-page-keyword"
              aria-label="模板名称或说明"
              placeholder="模板名称或说明"
              maxLength={100}
              className="w-full"
              value={draft}
              onChange={(event) => {
                setDraft(event.target.value);
              }}
            />
          </FilterField>
        </FilterPanel>
        <Card className="gap-0 overflow-hidden py-0">
          <ManagementTable
            refreshAction={refreshButton}
            tableID="cpc-timeline-templates"
            columns={columns}
            data={data?.items ?? []}
            total={data?.total ?? 0}
            page={pagination.page}
            pageSize={pagination.pageSize}
            loading={query.isFetching}
            emptyMessage="暂无符合条件的时间线模板"
            onPage={async (page) => {
              pagination.setPage(page);
              await fetchPage(page, pagination.pageSize);
            }}
            onPageSize={async (size) => {
              pagination.setPageSize(size);
              await fetchPage(1, size);
            }}
          />
        </Card>
      </div>
      {editor !== undefined && can(editor ? 'update' : 'create') ? (
        <TemplateEditor
          workspaceID={workspaceID}
          original={editor}
          actions={actions}
          onClose={() => {
            setEditor(undefined);
          }}
        />
      ) : null}
      {importing && can('import') ? (
        <TemplateImport
          workspaceID={workspaceID}
          onClose={() => {
            setImporting(false);
          }}
        />
      ) : null}
      {detail && can('detail') ? (
        <Drawer
          title="时间线模板详情"
          description={detail.name}
          onClose={() => {
            setDetail(null);
          }}
        >
          <div className="space-y-5 p-6">
            {detail.description ? (
              <p className="text-sm wrap-anywhere whitespace-pre-wrap text-muted-foreground">
                {detail.description}
              </p>
            ) : null}
            <TemplatePreview
              nodes={detail.definition.nodes}
              workspaceID={workspaceID}
              template={detail}
              actions={actions}
            />
          </div>
        </Drawer>
      ) : null}
      {deleting && can('delete') ? (
        <Drawer
          title="删除时间线模板"
          description={`确认删除“${deleting.name}”？`}
          busy={remove.pending}
          onClose={() => {
            setDeleting(null);
          }}
          footer={
            <>
              <Button
                variant="secondary"
                disabled={remove.pending}
                onClick={() => {
                  setDeleting(null);
                }}
              >
                取消
              </Button>
              <Button
                variant="destructive"
                loading={remove.pending}
                onClick={() => {
                  void remove.run('delete', async () => {
                    await deleteTimeline(workspaceID, deleting);
                    const page = Math.max(
                      1,
                      Math.min(
                        pagination.page,
                        Math.ceil(((data?.total ?? 1) - 1) / pagination.pageSize),
                      ),
                    );
                    pagination.setPage(page);
                    await client.invalidateQueries({ queryKey: timelineKey(workspaceID) });
                    await fetchPage(page, pagination.pageSize);
                    setDeleting(null);
                  });
                }}
              >
                <Trash2 aria-hidden="true" />
                确认删除
              </Button>
            </>
          }
        >
          <p className="p-6 text-sm text-muted-foreground">
            将永久删除该模板、全部节点、节点附件记录，以及以此模板为来源或目标的转移配置；历史审计保留。已绑定人员的模板不可删除。
          </p>
        </Drawer>
      ) : null}
    </section>
  );
}
