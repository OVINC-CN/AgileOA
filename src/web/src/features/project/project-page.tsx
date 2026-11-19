import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { Eye, Pencil, Plus, RefreshCw, Search, Trash2, Upload } from 'lucide-react';
import { useState } from 'react';

import { ErrorMessage } from '@/components/error-message';
import { ExportMenu } from '@/components/export-menu';
import { FilterField, FilterPanel } from '@/components/filter-panel';
import { ManagementTable } from '@/components/management-table';
import { SearchResetButton } from '@/components/search-reset-button';
import { TableActionButton } from '@/components/table-action-button';
import { TableTimestamp } from '@/components/table-timestamp';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Drawer } from '@/components/ui/drawer';
import { Input } from '@/components/ui/input';
import { SearchSelect } from '@/components/ui/search-select';
import {
  deleteRecord,
  emptyFilters,
  exportRecords,
  type Participation,
  participationDetailOptions,
  participationListOptions,
  type ParticipationSummary,
  type Project,
  projectExports,
  projectFilterOptions,
  type ProjectFilters,
  projectKey,
  projectListFilters,
  projectListOptions,
  statisticsOptions,
} from '@/features/project/api';
import { ApplicationList } from '@/features/project/application-list';
import { ParticipationDetail } from '@/features/project/participation-detail';
import { ParticipationEditor } from '@/features/project/participation-editor';
import { ProjectChart } from '@/features/project/project-chart';
import { ProjectDetail } from '@/features/project/project-detail';
import { ProjectEditor } from '@/features/project/project-editor';
import { ProjectNameYearFilters, ProjectStudentFilter } from '@/features/project/project-filters';
import { ProjectSelfApplyBadge } from '@/features/project/project-status';
import { ProjectTransfer } from '@/features/project/project-transfer';
import type { WorkspaceContext } from '@/features/workspace/context';
import { projectTabs } from '@/features/workspace/navigation';
import { ApiError } from '@/lib/api';
import { useApiAction } from '@/lib/api-action';
import { useFilterForm } from '@/lib/filter-form';
import { formatIdentity, formatScore, formatValue } from '@/lib/format';
import { useNavigationTabs } from '@/lib/navigation-tabs';
import { useTablePagination } from '@/lib/pagination';
import { recordTimeColumns } from '@/lib/record-time-columns';
import { saveSpreadsheetFile } from '@/lib/spreadsheet';

export function ProjectPage(props: WorkspaceContext) {
  return <ProjectWorkspace key={props.workspaceID} {...props} />;
}

function ProjectWorkspace({ workspaceID, actions, refresh: refreshWorkspace }: WorkspaceContext) {
  const can = (action: string) => actions.includes(action);
  const tabs = projectTabs.filter(({ listAction }) => can(listAction));
  const { selected } = useNavigationTabs(tabs);
  const tab = selected?.value;
  const [previousTab, setPreviousTab] = useState(tab);
  const isParticipation = tab === 'participations';
  const [filters, setFilters] = useState<ProjectFilters>(emptyFilters);
  const [project, setProject] = useState<Project | null | undefined>();
  const [participation, setParticipation] = useState<{
    record: Participation | null;
    readOnly: boolean;
  } | null>(null);
  const [detail, setDetail] = useState<Project | null>(null);
  const [transfer, setTransfer] = useState(false);
  const [deleting, setDeleting] = useState<{
    record: Project | ParticipationSummary;
    participation: boolean;
  } | null>(null);
  const pagination = useTablePagination();
  const client = useQueryClient();
  const refresh = useApiAction();
  const search = useApiAction();
  const remove = useApiAction();
  const loadDetail = useApiAction();
  const download = useApiAction();
  const canViewStatistics = tab === 'projects' && can('project.statistics');
  const statisticsFilters = projectListFilters(filters);
  const projectQuery = useQuery({
    ...projectListOptions(workspaceID, filters, pagination.page, pagination.pageSize),
    enabled: tab === 'projects',
    placeholderData: keepPreviousData,
  });
  const participationQuery = useQuery({
    ...participationListOptions(workspaceID, filters, pagination.page, pagination.pageSize),
    enabled: tab === 'participations',
    placeholderData: keepPreviousData,
  });
  const statistics = useQuery({
    ...statisticsOptions(workspaceID, statisticsFilters),
    enabled: canViewStatistics,
    placeholderData: keepPreviousData,
  });
  const denied = (error: Error | null) =>
    error instanceof ApiError && [401, 403, 404].includes(error.status);
  const projects = denied(projectQuery.error) ? undefined : projectQuery.data;
  const participations = denied(participationQuery.error) ? undefined : participationQuery.data;
  const activeQuery = tab === 'projects' ? projectQuery : participationQuery;
  const canList = tab !== undefined && tab !== 'applications';
  const filterOptions = useQuery({
    ...projectFilterOptions(workspaceID, isParticipation),
    enabled: canList,
    placeholderData: keepPreviousData,
  });
  const enums = denied(filterOptions.error) ? undefined : filterOptions.data;
  const columns: ColumnDef<Project>[] = [
    { accessorKey: 'name', header: '项目' },
    { accessorKey: 'year', header: '归属年份' },
    {
      accessorKey: 'allow_self_apply',
      header: '自行申报',
      cell: ({ row }) => <ProjectSelfApplyBadge allowed={row.original.allow_self_apply} />,
    },
    {
      accessorKey: 'self_apply_deadline',
      header: '自行申报截止时间',
      cell: ({ row }) => <TableTimestamp value={row.original.self_apply_deadline} />,
    },
    {
      accessorKey: 'description',
      header: '说明',
      cell: ({ row }) => (
        <p className="max-w-80 truncate" title={row.original.description}>
          {formatValue(row.original.description)}
        </p>
      ),
    },
    {
      id: 'attachments',
      header: '附件',
      cell: ({ row }) => String(row.original.attachment_ids.length),
    },
    ...recordTimeColumns<Project>((record) => record),
    {
      id: 'actions',
      header: '操作',
      cell: ({ row }) => (
        <div className="flex items-center gap-1">
          <TableActionButton
            onClick={() => {
              setDetail(row.original);
            }}
          >
            <Eye aria-hidden="true" />
            详情
          </TableActionButton>
          {can('project.update') ? (
            <TableActionButton
              onClick={() => {
                setProject(row.original);
              }}
            >
              <Pencil aria-hidden="true" />
              编辑
            </TableActionButton>
          ) : null}
          {can('project.delete') ? (
            <TableActionButton
              onClick={() => {
                setDeleting({ record: row.original, participation: false });
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
  const participationColumns: ColumnDef<ParticipationSummary>[] = [
    { accessorKey: 'project_name', header: '项目' },
    { accessorKey: 'year', header: '归属年份' },
    { accessorKey: 'student_no', header: '学号' },
    { accessorKey: 'student_name', header: '姓名' },
    { accessorKey: 'education_level', header: '学历' },
    { accessorKey: 'major', header: '专业' },
    { accessorKey: 'grade', header: '年级' },
    { accessorKey: 'rank', header: '排名' },
    { accessorKey: 'score', header: '总分', cell: ({ row }) => formatScore(row.original.score) },
    {
      id: 'details',
      header: '评分子项',
      cell: ({ row }) => `${String(row.original.detail_count)} 项`,
    },
    ...recordTimeColumns<ParticipationSummary>((record) => record),
    {
      id: 'actions',
      header: '操作',
      cell: ({ row }) => (
        <div className="flex items-center gap-1">
          <TableActionButton
            loading={loadDetail.isPending(`view:${String(row.original.id)}`)}
            onClick={() => {
              void (async () => {
                const result = await loadDetail.run(`view:${String(row.original.id)}`, () =>
                  client.query({
                    ...participationDetailOptions(workspaceID, row.original.id),
                    staleTime: 0,
                  }),
                );
                if (result.ok) setParticipation({ record: result.data, readOnly: true });
              })();
            }}
          >
            <Eye aria-hidden="true" />
            详情
          </TableActionButton>
          {can('project.participation.update') ? (
            <TableActionButton
              loading={loadDetail.isPending(`edit:${String(row.original.id)}`)}
              onClick={() => {
                void (async () => {
                  const result = await loadDetail.run(`edit:${String(row.original.id)}`, () =>
                    client.query({
                      ...participationDetailOptions(workspaceID, row.original.id),
                      staleTime: 0,
                    }),
                  );
                  if (result.ok) setParticipation({ record: result.data, readOnly: false });
                })();
              }}
            >
              <Pencil aria-hidden="true" />
              编辑
            </TableActionButton>
          ) : null}
          {can('project.participation.delete') ? (
            <TableActionButton
              onClick={() => {
                setDeleting({ record: row.original, participation: true });
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
  const fetchPage = (
    page: number,
    size: number,
    selectedFilters = filters,
    participationTab = isParticipation,
  ) =>
    participationTab
      ? client.query({
          ...participationListOptions(workspaceID, selectedFilters, page, size),
          staleTime: 0,
        })
      : client.query({
          ...projectListOptions(workspaceID, selectedFilters, page, size),
          staleTime: 0,
        });
  const fetchResults = async (selected: ProjectFilters, page: number) => {
    const results = await Promise.allSettled([
      ...(canList ? [fetchPage(page, pagination.pageSize, selected)] : []),
      ...(canViewStatistics
        ? [
            client.query({
              ...statisticsOptions(workspaceID, projectListFilters(selected)),
              staleTime: 0,
            }),
          ]
        : []),
    ]);
    const failure = results.find((result) => result.status === 'rejected');
    if (failure)
      throw failure.reason instanceof Error
        ? failure.reason
        : new Error('查询评奖评优失败，请重试');
  };
  const applyFilters = async (values: ProjectFilters) => {
    const next = {
      ...values,
      keyword: values.keyword.trim(),
      student: values.student.trim(),
      detail_keyword:
        isParticipation && values.project_name.length > 0 && values.year.length > 0
          ? values.detail_keyword.trim()
          : '',
    };
    setFilters(next);
    pagination.setPage(1);
    await fetchResults(next, 1);
  };
  const { draft, setDraft, changeFilters } = useFilterForm(emptyFilters, applyFilters);

  // 子菜单由 URL 驱动，切换视图时保留公共条件，重置分页及子项关键词。
  if (previousTab !== tab) {
    setPreviousTab(tab);
    setDraft((current) => ({ ...current, detail_keyword: '' }));
    setFilters((current) => ({ ...current, detail_keyword: '' }));
    pagination.setPage(1);
  }

  const canSearchDetails = draft.project_name.length > 0 && draft.year.length > 0;
  if (tab === 'applications')
    return (
      <section aria-labelledby="application-management-title">
        <h1 id="application-management-title" className="sr-only">
          填报管理
        </h1>
        <ApplicationList
          key={workspaceID}
          workspaceID={workspaceID}
          scope="manage"
          canExport={can('project.application.export')}
          canDelete={can('project.application.delete')}
          onRefresh={refreshWorkspace}
          panelID="project-panel-applications"
          panelLabel="填报管理"
          breadcrumb={
            <nav aria-label="面包屑" className="text-xs text-muted-foreground">
              <ol className="flex flex-wrap items-center gap-2">
                <li>学生工作</li>
                <li aria-hidden="true">/</li>
                <li>评奖评优</li>
                <li aria-hidden="true">/</li>
                <li aria-current="page">填报管理</li>
              </ol>
            </nav>
          }
        />
      </section>
    );
  return (
    <section className="space-y-5" aria-labelledby="project-page-title">
      <header className="flex min-h-8 flex-wrap items-center justify-between gap-3">
        <h1 id="project-page-title" className="sr-only">
          评奖评优
        </h1>
        <nav aria-label="面包屑" className="text-xs text-muted-foreground">
          <ol className="flex flex-wrap items-center gap-2">
            <li>学生工作</li>
            <li aria-hidden="true">/</li>
            <li aria-current={selected ? undefined : 'page'}>评奖评优</li>
            {selected ? (
              <>
                <li aria-hidden="true">/</li>
                <li aria-current="page">{selected.label}</li>
              </>
            ) : null}
          </ol>
        </nav>
        <div className="ml-auto flex flex-wrap gap-2">
          {canList && can(isParticipation ? 'project.participation.import' : 'project.import') ? (
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
          {canList ? (
            <ExportMenu
              items={projectExports.filter(
                ({ action, value }) =>
                  can(action) && (isParticipation ? value !== 'projects' : value === 'projects'),
              )}
              loading={download.pending}
              disabled={denied(activeQuery.error)}
              onSelect={(type) => {
                void (async () => {
                  const result = await download.run(type, () =>
                    exportRecords(workspaceID, type, filters),
                  );
                  if (result.ok) saveSpreadsheetFile(result.data);
                })();
              }}
            />
          ) : null}
          {tab === 'projects' && can('project.create') ? (
            <Button
              onClick={() => {
                setProject(null);
              }}
            >
              <Plus aria-hidden="true" />
              新建项目
            </Button>
          ) : null}
          {isParticipation && can('project.participation.create') ? (
            <Button
              onClick={() => {
                setParticipation({ record: null, readOnly: false });
              }}
            >
              <Plus aria-hidden="true" />
              绑定学生
            </Button>
          ) : null}
          <Button
            variant="outline"
            loading={
              refresh.pending ||
              (canList && (activeQuery.isPending || filterOptions.isPending)) ||
              (canViewStatistics && statistics.isPending)
            }
            onClick={() => {
              void refresh.run('refresh', async () => {
                await refreshWorkspace();
                await Promise.all([
                  fetchResults(filters, pagination.page),
                  ...(canList
                    ? [
                        client.query({
                          ...projectFilterOptions(workspaceID, isParticipation),
                          staleTime: 0,
                        }),
                      ]
                    : []),
                ]);
              });
            }}
          >
            <RefreshCw aria-hidden="true" />
            刷新
          </Button>
        </div>
      </header>
      {canViewStatistics && !denied(statistics.error) ? (
        <ProjectChart data={statistics.data ?? []} />
      ) : null}
      {canViewStatistics && statistics.error ? (
        <ErrorMessage>{statistics.error.message}</ErrorMessage>
      ) : null}
      {filterOptions.error ? <ErrorMessage>{filterOptions.error.message}</ErrorMessage> : null}
      <div
        role={tab ? 'region' : undefined}
        id={tab ? `project-panel-${tab}` : undefined}
        aria-label={selected?.label}
        className="space-y-4"
      >
        {canList ? (
          <FilterPanel
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
                      setDraft(emptyFilters);
                      await applyFilters(emptyFilters);
                    });
                  }}
                />
                <Button
                  type="submit"
                  loading={search.isPending('search')}
                  disabled={search.pending}
                >
                  <Search aria-hidden="true" />
                  搜索
                </Button>
              </>
            }
          >
            <ProjectNameYearFilters
              names={enums?.names ?? []}
              years={enums?.years ?? []}
              projectNames={draft.project_name}
              selectedYears={draft.year}
              loading={filterOptions.isFetching}
              onProjectNamesChange={(project_name) => {
                changeFilters({
                  project_name,
                  detail_keyword:
                    project_name.length > 0 && draft.year.length > 0 ? draft.detail_keyword : '',
                });
              }}
              onYearsChange={(year) => {
                changeFilters({
                  year,
                  detail_keyword:
                    year.length > 0 && draft.project_name.length > 0 ? draft.detail_keyword : '',
                });
              }}
            />
            {isParticipation ? (
              <ProjectStudentFilter
                id="project-page-student"
                value={draft.student}
                onChange={(student) => {
                  setDraft((current) => ({ ...current, student }));
                }}
              />
            ) : null}
            {isParticipation ? (
              <>
                <FilterField label="学历">
                  <SearchSelect
                    multiple
                    label="学历"
                    className="w-full"
                    options={enums?.education_levels ?? []}
                    value={draft.education_level}
                    loading={filterOptions.isFetching}
                    onValueChange={(value) => {
                      changeFilters({ education_level: value });
                    }}
                  />
                </FilterField>
                <FilterField label="专业">
                  <SearchSelect
                    multiple
                    label="专业"
                    className="w-full"
                    options={enums?.majors ?? []}
                    value={draft.major}
                    loading={filterOptions.isFetching}
                    onValueChange={(value) => {
                      changeFilters({ major: value });
                    }}
                  />
                </FilterField>
                <FilterField label="年级">
                  <SearchSelect
                    multiple
                    label="年级"
                    className="w-full"
                    options={enums?.grades ?? []}
                    value={draft.grade}
                    loading={filterOptions.isFetching}
                    onValueChange={(value) => {
                      changeFilters({ grade: value });
                    }}
                  />
                </FilterField>
              </>
            ) : null}
            {isParticipation ? (
              <FilterField label="子项名称或说明" htmlFor="project-page-detail-keyword">
                <Input
                  id="project-page-detail-keyword"
                  aria-label="子项名称或说明"
                  placeholder={canSearchDetails ? '子项名称或说明' : '先选择项目和归属年份'}
                  title={
                    canSearchDetails ? '包含匹配子项名称或说明' : '先选择项目和归属年份后可搜索子项'
                  }
                  disabled={!canSearchDetails}
                  maxLength={100}
                  value={draft.detail_keyword}
                  onChange={(event) => {
                    setDraft((current) => ({ ...current, detail_keyword: event.target.value }));
                  }}
                />
              </FilterField>
            ) : null}
          </FilterPanel>
        ) : null}
        {activeQuery.error ? <ErrorMessage>{activeQuery.error.message}</ErrorMessage> : null}
        {canList ? (
          <Card className="gap-0 overflow-hidden py-0">
            {isParticipation ? (
              <ManagementTable
                tableID="project-participations"
                columns={participationColumns}
                data={participations?.items ?? []}
                total={participations?.total ?? 0}
                page={pagination.page}
                pageSize={pagination.pageSize}
                loading={activeQuery.isFetching}
                emptyMessage="暂无符合条件的参与记录"
                onPage={async (page) => {
                  pagination.setPage(page);
                  return fetchPage(page, pagination.pageSize);
                }}
                onPageSize={async (size) => {
                  pagination.setPageSize(size);
                  return fetchPage(1, size);
                }}
              />
            ) : (
              <ManagementTable
                tableID="projects"
                columns={columns}
                data={projects?.items ?? []}
                total={projects?.total ?? 0}
                page={pagination.page}
                pageSize={pagination.pageSize}
                loading={activeQuery.isFetching}
                emptyMessage="暂无符合条件的项目"
                onPage={async (page) => {
                  pagination.setPage(page);
                  return fetchPage(page, pagination.pageSize);
                }}
                onPageSize={async (size) => {
                  pagination.setPageSize(size);
                  return fetchPage(1, size);
                }}
              />
            )}
          </Card>
        ) : (
          <p className="py-12 text-center text-sm text-muted-foreground">没有查看此列表的权限</p>
        )}
      </div>
      {project !== undefined ? (
        <ProjectEditor
          workspaceID={workspaceID}
          project={project}
          actions={actions}
          onClose={() => {
            setProject(undefined);
          }}
        />
      ) : null}
      {participation?.readOnly && participation.record ? (
        <ParticipationDetail
          record={participation.record}
          onClose={() => {
            setParticipation(null);
          }}
        />
      ) : participation ? (
        <ParticipationEditor
          workspaceID={workspaceID}
          record={participation.record}
          actions={actions}
          onClose={() => {
            setParticipation(null);
          }}
        />
      ) : null}
      {transfer ? (
        <ProjectTransfer
          workspaceID={workspaceID}
          participation={isParticipation}
          onClose={() => {
            setTransfer(false);
          }}
        />
      ) : null}
      {detail ? (
        <ProjectDetail
          workspaceID={workspaceID}
          project={detail}
          canReadAttachments={can('project.attachment.read')}
          onClose={() => {
            setDetail(null);
          }}
        />
      ) : null}
      {deleting ? (
        <Drawer
          title={deleting.participation ? '删除获奖记录' : '删除项目'}
          truncateDescription
          description={
            'name' in deleting.record
              ? deleting.record.name
              : `${deleting.record.project_name} · ` +
                formatIdentity(deleting.record.student_name, deleting.record.student_no)
          }
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
                      await deleteRecord(workspaceID, deleting.record, deleting.participation);
                      await client.invalidateQueries({ queryKey: projectKey(workspaceID) });
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
            {deleting.participation
              ? '将永久删除本条获奖记录及其全部评分子项，学生档案、项目及历史审计保留。'
              : '将永久删除项目、子项配置及审批流程，历史审计保留。项目仍有关联申报、获奖记录或附件时不可删除。'}
          </p>
        </Drawer>
      ) : null}
    </section>
  );
}
