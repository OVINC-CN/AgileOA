import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { Eye, Pencil, Plus, RefreshCw, Search, Trash2 } from 'lucide-react';
import { type ReactNode, useState } from 'react';
import { useSearchParams } from 'react-router';

import { ErrorMessage } from '@/components/error-message';
import { ExportMenu } from '@/components/export-menu';
import { FilterField, FilterPanel } from '@/components/filter-panel';
import { ManagementTable } from '@/components/management-table';
import { SearchResetButton } from '@/components/search-reset-button';
import { TableActionButton } from '@/components/table-action-button';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Drawer } from '@/components/ui/drawer';
import { SearchSelect } from '@/components/ui/search-select';
import { projectKey } from '@/features/project/api';
import {
  applicationDetailOptions,
  type ApplicationFilters,
  applicationListOptions,
  applicationProjectFilterOptions,
  type ApplicationScope,
  applicationStatusOptions,
  type ApplicationSummary,
  defaultApplicationFilters,
  deleteApplication,
  exportApplicationRecords,
} from '@/features/project/application-api';
import { ApplicationDrawer } from '@/features/project/application-drawer';
import { ApplicationLinkDrawer } from '@/features/project/application-link-drawer';
import { ApplicationStatusBadge } from '@/features/project/application-status';
import { ProjectNameYearFilters, ProjectStudentFilter } from '@/features/project/project-filters';
import { ApiError } from '@/lib/api';
import { useApiAction } from '@/lib/api-action';
import { useFilterForm } from '@/lib/filter-form';
import { formatIdentity } from '@/lib/format';
import { useTablePagination } from '@/lib/pagination';
import { recordTimeColumns } from '@/lib/record-time-columns';
import { saveSpreadsheetFile } from '@/lib/spreadsheet';

export function ApplicationList({
  workspaceID,
  scope,
  breadcrumb,
  panelID,
  panelLabel,
  onRefresh,
  canExport = false,
  canDelete = false,
}: {
  workspaceID: string;
  scope: ApplicationScope;
  breadcrumb: ReactNode;
  panelID: string;
  panelLabel: string;
  onRefresh?: () => Promise<unknown>;
  canExport?: boolean;
  canDelete?: boolean;
}) {
  const [params, setParams] = useSearchParams();
  const applicationID = params.get('application_id');
  const [filters, setFilters] = useState(() => defaultApplicationFilters(scope));
  const [creating, setCreating] = useState(false);
  const [deleting, setDeleting] = useState<ApplicationSummary | null>(null);
  const pagination = useTablePagination();
  const client = useQueryClient();
  const query = useQuery({
    ...applicationListOptions(workspaceID, scope, filters, pagination.page, pagination.pageSize),
    placeholderData: keepPreviousData,
  });
  const projectFilters = useQuery(applicationProjectFilterOptions(workspaceID, scope));
  const search = useApiAction();
  const refresh = useApiAction();
  const loadDetail = useApiAction();
  const exportAction = useApiAction();
  const remove = useApiAction();
  const closeDetail = () => {
    setCreating(false);
    if (applicationID !== null) {
      setParams(
        (current) => {
          current.delete('application_id');
          return current;
        },
        { replace: true },
      );
    }
  };
  const fetchPage = (page: number, size: number, selected = filters) =>
    client.query({
      ...applicationListOptions(workspaceID, scope, selected, page, size),
      staleTime: 0,
    });
  const applyFilters = async (values: ApplicationFilters) => {
    const next = { ...values, student: values.student.trim() };
    setFilters(next);
    pagination.setPage(1);
    await fetchPage(1, pagination.pageSize, next);
  };
  const { draft, setDraft, changeFilters, hasUnappliedChanges } = useFilterForm(
    () => defaultApplicationFilters(scope),
    applyFilters,
    filters,
  );
  const columns: ColumnDef<ApplicationSummary>[] = [
    { accessorKey: 'id', header: '申报编号' },
    { accessorKey: 'project_name', header: '项目' },
    { accessorKey: 'year', header: '归属年份' },
    { accessorKey: 'student_no', header: '学号' },
    { accessorKey: 'student_name', header: '姓名' },
    { accessorKey: 'class_name', header: '班级' },
    {
      accessorKey: 'status',
      header: '审批状态',
      cell: ({ row }) => <ApplicationStatusBadge status={row.original.status} />,
    },
    { accessorKey: 'current_reviewer', header: '当前处理人' },
    ...recordTimeColumns<ApplicationSummary>((record) => record),
    {
      id: 'actions',
      header: '操作',
      cell: ({ row }) => {
        const record = row.original;
        const editable = record.can_edit || record.can_approve || record.can_reject;
        return (
          <div className="flex items-center gap-1">
            <TableActionButton
              disabled={loadDetail.pending}
              loading={loadDetail.isPending(String(record.id))}
              onClick={() => {
                void loadDetail.run(String(record.id), async () => {
                  await client.query({
                    ...applicationDetailOptions(workspaceID, scope, record.id),
                    staleTime: 0,
                  });
                  setCreating(false);
                  setParams((current) => {
                    current.set('application_id', String(record.id));
                    return current;
                  });
                });
              }}
            >
              {editable ? <Pencil aria-hidden="true" /> : <Eye aria-hidden="true" />}
              {editable ? '处理' : '详情'}
            </TableActionButton>
            {scope === 'manage' && canDelete ? (
              <TableActionButton
                disabled={remove.pending || denied}
                onClick={() => {
                  setDeleting(record);
                }}
              >
                <Trash2 aria-hidden="true" />
                删除
              </TableActionButton>
            ) : null}
          </div>
        );
      },
    },
  ];
  const denied = query.error instanceof ApiError && [401, 403, 404].includes(query.error.status);
  const data = denied ? undefined : query.data;
  const refreshButton = (
    <Button
      type="button"
      size="sm"
      variant="ghost"
      loading={refresh.pending}
      onClick={() => {
        void refresh.run('refresh', async () => {
          await Promise.all([
            fetchPage(pagination.page, pagination.pageSize),
            client.query({
              ...applicationProjectFilterOptions(workspaceID, scope),
              staleTime: 0,
            }),
            ...(onRefresh ? [onRefresh()] : []),
          ]);
        });
      }}
    >
      <RefreshCw aria-hidden="true" />
      刷新
    </Button>
  );
  return (
    <div className="space-y-5">
      <header className="flex min-h-8 flex-wrap items-center justify-between gap-3">
        {breadcrumb}
        <div className="ml-auto flex flex-wrap gap-2">
          {scope === 'manage' && canExport ? (
            <ExportMenu
              items={[{ value: 'applications', label: '导出填报明细' }]}
              loading={exportAction.pending}
              disabled={exportAction.pending || denied}
              onSelect={() => {
                void exportAction.run('export', async () => {
                  saveSpreadsheetFile(await exportApplicationRecords(workspaceID, filters));
                });
              }}
            />
          ) : null}
          {scope === 'mine' ? (
            <Button
              onClick={() => {
                closeDetail();
                setCreating(true);
              }}
            >
              <Plus aria-hidden="true" />
              新建申报
            </Button>
          ) : null}
        </div>
      </header>
      <div role="region" id={panelID} aria-label={panelLabel} className="space-y-4">
        <FilterPanel
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
                    setDraft(defaultApplicationFilters(scope));
                    await applyFilters(defaultApplicationFilters(scope));
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
          <ProjectNameYearFilters
            names={projectFilters.data?.names ?? []}
            years={projectFilters.data?.years ?? []}
            projectNames={draft.project_name}
            selectedYears={draft.year}
            loading={projectFilters.isFetching}
            onProjectNamesChange={(project_name) => {
              changeFilters({ project_name });
            }}
            onYearsChange={(year) => {
              changeFilters({ year });
            }}
          />
          <ProjectStudentFilter
            id="application-student"
            value={draft.student}
            onChange={(student) => {
              setDraft((current) => ({ ...current, student }));
            }}
          />
          <FilterField label="审批状态">
            <SearchSelect
              label="审批状态"
              className="w-full"
              multiple
              allLabel="全部状态"
              options={applicationStatusOptions(scope)}
              value={draft.status}
              onValueChange={(status) => {
                changeFilters({ status });
              }}
            />
          </FilterField>
        </FilterPanel>
        {query.error ? <ErrorMessage>{query.error.message}</ErrorMessage> : null}
        {projectFilters.error ? <ErrorMessage>{projectFilters.error.message}</ErrorMessage> : null}
        <Card className="gap-0 overflow-hidden py-0">
          <ManagementTable
            refreshAction={refreshButton}
            tableID={`project-applications-${scope}`}
            columns={columns}
            data={data?.items ?? []}
            total={data?.total ?? 0}
            page={pagination.page}
            pageSize={pagination.pageSize}
            loading={query.isFetching}
            emptyMessage="暂无符合条件的申报单"
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
      {applicationID !== null ? (
        <ApplicationLinkDrawer
          key={`${workspaceID}:${scope}:${applicationID}`}
          workspaceID={workspaceID}
          scope={scope}
          applicationID={applicationID}
          onClose={closeDetail}
        />
      ) : creating ? (
        <ApplicationDrawer
          workspaceID={workspaceID}
          scope={scope}
          record={null}
          onClose={closeDetail}
        />
      ) : null}
      {scope === 'manage' && canDelete && !denied && deleting ? (
        <Drawer
          title="删除填报"
          truncateDescription
          description={`${deleting.project_name} · ${String(deleting.year)} · ${formatIdentity(deleting.student_name, deleting.student_no)}`}
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
                loading={remove.pending}
                onClick={() => {
                  void (async () => {
                    const result = await remove.run('delete', async () => {
                      await deleteApplication(workspaceID, deleting);
                      client.removeQueries({
                        queryKey: applicationDetailOptions(workspaceID, scope, deleting.id)
                          .queryKey,
                      });
                      await client.invalidateQueries({ queryKey: projectKey(workspaceID) });
                      const lastPage = Math.max(
                        1,
                        Math.ceil(((data?.total ?? 1) - 1) / pagination.pageSize),
                      );
                      if (pagination.page > lastPage) {
                        pagination.setPage(lastPage);
                        await fetchPage(lastPage, pagination.pageSize);
                      }
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
          <p className="p-6 text-sm">
            将永久删除本条填报及其全部评分子项、审批流程、签署记录、流转历史和附件。
            学生档案、项目、获奖记录及历史审计保留。
          </p>
        </Drawer>
      ) : null}
    </div>
  );
}
