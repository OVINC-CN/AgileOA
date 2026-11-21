import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import {
  Download,
  Eye,
  Pencil,
  Plus,
  RefreshCw,
  Search,
  Shuffle,
  Trash2,
  Upload,
} from 'lucide-react';
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
import { personKey } from '@/features/cpc-person/api';
import type { AdmissionRuleNode } from '@/features/party-branch/admission-rule';
import { AdmissionRulePreview } from '@/features/party-branch/admission-rule-preview';
import {
  type AdmissionRuleInfo,
  deletePartyBranch,
  exportPartyBranches,
  type PartyBranch,
  partyBranchAdmissionRuleOptions,
  partyBranchKey,
  partyBranchListOptions,
  reorderPartyBranchPeople,
} from '@/features/party-branch/api';
import { BranchEditor } from '@/features/party-branch/branch-editor';
import { BranchTransfer } from '@/features/party-branch/branch-transfer';
import type { WorkspaceContext } from '@/features/workspace/context';
import { ApiError } from '@/lib/api';
import { useApiAction } from '@/lib/api-action';
import { emptyValueLabel, formatIdentity, formatValue } from '@/lib/format';
import { showMessage } from '@/lib/messages';
import { useTablePagination } from '@/lib/pagination';
import { recordTimeColumns } from '@/lib/record-time-columns';
import { saveSpreadsheetFile } from '@/lib/spreadsheet';

export function BranchPage(props: WorkspaceContext) {
  return <BranchWorkspace key={props.workspaceID} {...props} />;
}

function BranchWorkspace({ workspaceID, actions, refresh: refreshWorkspace }: WorkspaceContext) {
  const can = (action: string) => actions.includes(`cpc.party_branch.${action}`);
  const canList = can('list');
  const [draft, setDraft] = useState('');
  const [keyword, setKeyword] = useState('');
  const [editing, setEditing] = useState<PartyBranch | null | undefined>();
  const [detail, setDetail] = useState<{
    branch: PartyBranch;
    rule: AdmissionRuleNode | null;
  } | null>(null);
  const [deleting, setDeleting] = useState<PartyBranch | null>(null);
  const [transfer, setTransfer] = useState(false);
  const [reordering, setReordering] = useState(false);
  const [editingRule, setEditingRule] = useState<AdmissionRuleInfo | null>(null);
  const pagination = useTablePagination();
  const client = useQueryClient();
  const refresh = useApiAction();
  const search = useApiAction();
  const remove = useApiAction();
  const reorder = useApiAction();
  const download = useApiAction();
  const loadEdit = useApiAction();
  const loadDetail = useApiAction();
  const query = useQuery({
    ...partyBranchListOptions(workspaceID, keyword, pagination.page, pagination.pageSize),
    enabled: canList,
    placeholderData: keepPreviousData,
  });
  const denied = query.error instanceof ApiError && [401, 403, 404].includes(query.error.status);
  const data = denied ? undefined : query.data;
  const fetchPage = (page: number, size: number, searchKeyword = keyword) =>
    client.query({
      ...partyBranchListOptions(workspaceID, searchKeyword, page, size),
      staleTime: 0,
    });
  const applySearch = async (value: string) => {
    const next = value.trim();
    setKeyword(next);
    pagination.setPage(1);
    await fetchPage(1, pagination.pageSize, next);
  };
  const columns: ColumnDef<PartyBranch>[] = [
    { accessorKey: 'priority', header: '优先级' },
    { accessorKey: 'short_name', header: '党支部简称' },
    {
      accessorKey: 'full_name',
      header: '党支部全称',
      cell: ({ row }) => (
        <p className="max-w-112 truncate" title={row.original.full_name}>
          {row.original.full_name}
        </p>
      ),
    },
    {
      id: 'secretary',
      header: '管理员',
      cell: ({ row }) =>
        row.original.secretary_id === null
          ? emptyValueLabel
          : formatIdentity(row.original.secretary_name, row.original.secretary_username),
    },
    ...recordTimeColumns<PartyBranch>((record) => record),
    {
      id: 'actions',
      header: '操作',
      cell: ({ row }) => (
        <div className="flex items-center gap-1">
          <TableActionButton
            loading={loadDetail.isPending(String(row.original.id))}
            disabled={loadDetail.pending}
            onClick={() => {
              void (async () => {
                const result = await loadDetail.run(String(row.original.id), () =>
                  client.query({
                    ...partyBranchAdmissionRuleOptions(workspaceID, row.original.id),
                    staleTime: 0,
                  }),
                );
                if (result.ok) setDetail({ branch: row.original, rule: result.data.rule });
              })();
            }}
          >
            <Eye aria-hidden="true" />
            详情
          </TableActionButton>
          {can('update') ? (
            <TableActionButton
              loading={loadEdit.isPending(String(row.original.id))}
              disabled={loadEdit.pending}
              onClick={() => {
                void (async () => {
                  const result = await loadEdit.run(String(row.original.id), async () => {
                    const info = await client.query({
                      ...partyBranchAdmissionRuleOptions(workspaceID, row.original.id),
                      staleTime: 0,
                    });
                    if (info.revision !== row.original.revision) {
                      await client.invalidateQueries({ queryKey: partyBranchKey(workspaceID) });
                      throw new Error('党支部已被修改，请刷新后重试');
                    }
                    return info;
                  });
                  if (result.ok) {
                    setEditingRule(result.data);
                    setEditing(row.original);
                  }
                })();
              }}
            >
              <Pencil aria-hidden="true" />
              编辑
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
      loading={refresh.pending || (canList && query.isPending)}
      onClick={() => {
        void refresh.run('refresh', async () => {
          await refreshWorkspace();
          if (canList) await fetchPage(pagination.page, pagination.pageSize);
        });
      }}
    >
      <RefreshCw aria-hidden="true" />
      刷新
    </Button>
  );
  return (
    <section className="space-y-5" aria-labelledby="cpc-data-title">
      <header className="flex min-h-8 flex-wrap items-center justify-between gap-3">
        <h1 id="cpc-data-title" className="sr-only">
          数据管理
        </h1>
        <nav aria-label="面包屑" className="text-xs text-muted-foreground">
          <ol className="flex flex-wrap items-center gap-2">
            <li>党建工作</li>
            <li aria-hidden="true">/</li>
            <li>数据管理</li>
            {canList ? (
              <>
                <li aria-hidden="true">/</li>
                <li aria-current="page">党支部</li>
              </>
            ) : null}
          </ol>
        </nav>
        <div className="ml-auto flex flex-wrap gap-2">
          {canList && can('import') ? (
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
          {canList && can('export') ? (
            <span className="inline-flex items-center gap-2">
              <Button
                variant="outline"
                loading={download.pending}
                disabled={denied}
                onClick={() => {
                  void download.run('export', async () => {
                    saveSpreadsheetFile(await exportPartyBranches(workspaceID, keyword));
                  });
                }}
              >
                <Download aria-hidden="true" />
                导出
              </Button>
            </span>
          ) : null}
          {canList && can('update') ? (
            <Button
              variant="outline"
              loading={reorder.pending}
              onClick={() => {
                setReordering(true);
              }}
            >
              <Shuffle aria-hidden="true" />
              重排人员
            </Button>
          ) : null}
          {canList && can('create') ? (
            <Button
              onClick={() => {
                setEditingRule(null);
                setEditing(null);
              }}
            >
              <Plus aria-hidden="true" />
              新增党支部
            </Button>
          ) : null}
          {!canList ? refreshButton : null}
        </div>
      </header>
      {query.error ? <ErrorMessage>{query.error.message}</ErrorMessage> : null}
      {canList ? (
        <div
          role="region"
          id="cpc-data-panel-party-branches"
          aria-label="党支部"
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
                <Button
                  type="submit"
                  disabled={search.pending}
                  loading={search.isPending('search')}
                >
                  <Search aria-hidden="true" />
                  搜索
                </Button>
              </>
            }
          >
            <FilterField label="简称、全称或管理员" htmlFor="branch-page-keyword">
              <Input
                id="branch-page-keyword"
                aria-label="简称、全称或管理员"
                placeholder="简称、全称或管理员"
                maxLength={200}
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
              tableID="cpc-party-branches"
              columns={columns}
              data={data?.items ?? []}
              total={data?.total ?? 0}
              page={pagination.page}
              pageSize={pagination.pageSize}
              loading={query.isFetching}
              emptyMessage="暂无符合条件的党支部"
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
        </div>
      ) : (
        <Card className="p-6 text-sm text-muted-foreground">暂无可查看的数据管理页面</Card>
      )}
      {canList && editing !== undefined && can(editing ? 'update' : 'create') ? (
        <BranchEditor
          workspaceID={workspaceID}
          branch={editing}
          admissionRule={editingRule}
          onClose={() => {
            setEditing(undefined);
            setEditingRule(null);
          }}
        />
      ) : null}
      {canList && transfer && can('import') ? (
        <BranchTransfer
          workspaceID={workspaceID}
          onClose={() => {
            setTransfer(false);
          }}
        />
      ) : null}
      {canList && reordering && can('update') ? (
        <Drawer
          title="确认重排人员"
          description="重新计算当前工作空间在编人员的所属党支部"
          busy={reorder.pending}
          onClose={() => {
            setReordering(false);
          }}
          footer={
            <>
              <Button
                variant="secondary"
                disabled={reorder.pending}
                onClick={() => {
                  setReordering(false);
                }}
              >
                取消
              </Button>
              <Button
                loading={reorder.pending}
                onClick={() => {
                  void reorder.run('reorder', async () => {
                    const result = await reorderPartyBranchPeople(workspaceID);
                    await client.invalidateQueries({
                      queryKey: personKey(workspaceID),
                      refetchType: 'none',
                    });
                    await client.invalidateQueries({ queryKey: ['management', 'audit'] });
                    setReordering(false);
                    showMessage(
                      result.queued
                        ? '重排任务已提交，执行结果将通过站内通知发送'
                        : '当前没有需要重排的在编人员',
                    );
                  });
                }}
              >
                <Shuffle aria-hidden="true" />
                确认重排
              </Button>
            </>
          }
        >
          <div className="space-y-3 p-6 text-sm">
            <p>将重新分配在编人员的所属党支部。</p>
            <p>
              按当前准入规则匹配，优先选择优先级较高的党支部；优先级相同时选择编号较大的党支部。
            </p>
            <p>若有人未匹配到党支部，本次重排不生效。</p>
            <p>重排完成后将发送站内通知。</p>
          </div>
        </Drawer>
      ) : null}
      {canList && detail ? (
        <Drawer
          title="党支部详情"
          description={detail.branch.short_name}
          onClose={() => {
            setDetail(null);
          }}
        >
          <div className="space-y-5 p-6">
            <dl className="space-y-5 text-sm">
              {[
                ['编号', String(detail.branch.id)],
                ['优先级', String(detail.branch.priority)],
                ['党支部简称', detail.branch.short_name],
                ['党支部全称', detail.branch.full_name],
                [
                  '管理员',
                  detail.branch.secretary_id === null
                    ? emptyValueLabel
                    : formatIdentity(
                        detail.branch.secretary_name,
                        detail.branch.secretary_username,
                      ),
                ],
              ].map(([label, value]) => (
                <div key={label} className="space-y-1">
                  <dt className="text-muted-foreground">{label}</dt>
                  <dd className="break-words">{formatValue(value)}</dd>
                </div>
              ))}
            </dl>
            <AdmissionRulePreview rule={detail.rule} />
          </div>
        </Drawer>
      ) : null}
      {canList && deleting && can('delete') ? (
        <Drawer
          title="删除党支部"
          description={`确认删除“${deleting.full_name}”？`}
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
                    await deletePartyBranch(workspaceID, deleting);
                    const page = Math.max(
                      1,
                      Math.min(
                        pagination.page,
                        Math.ceil(((data?.total ?? 1) - 1) / pagination.pageSize),
                      ),
                    );
                    pagination.setPage(page);
                    await client.invalidateQueries({ queryKey: partyBranchKey(workspaceID) });
                    await client.invalidateQueries({ queryKey: personKey(workspaceID) });
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
            将永久删除该党支部及其准入规则、党建人员、时间线和附件记录；学生基础档案、书记账号及历史审计保留。
          </p>
        </Drawer>
      ) : null}
    </section>
  );
}
