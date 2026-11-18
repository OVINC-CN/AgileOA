import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import {
  BookOpen,
  BriefcaseBusiness,
  Download,
  Link2,
  Pencil,
  RefreshCw,
  Search,
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
import { ButtonGroup } from '@/components/ui/button-group';
import { Card } from '@/components/ui/card';
import { Drawer } from '@/components/ui/drawer';
import { Input } from '@/components/ui/input';
import { SearchSelect } from '@/components/ui/search-select';
import {
  clearGraduation,
  type Dimension,
  emptyFilters,
  exportGraduations,
  type Graduation,
  graduationFilterOptions,
  type GraduationFilters,
  graduationKey,
  graduationListOptions,
  graduationStatisticsOptions,
  pendingOption,
  unsetGraduationYearOption,
} from '@/features/graduation/api';
import { GraduationChart } from '@/features/graduation/graduation-chart';
import { GraduationEditor } from '@/features/graduation/graduation-editor';
import { GraduationTransfer } from '@/features/graduation/graduation-transfer';
import { studentKey } from '@/features/student/api';
import type { WorkspaceContext } from '@/features/workspace/context';
import { ApiError } from '@/lib/api';
import { useApiAction } from '@/lib/api-action';
import { useFilterForm } from '@/lib/filter-form';
import { formatIdentity, formatValue } from '@/lib/format';
import { useTablePagination } from '@/lib/pagination';
import { recordTimeColumns } from '@/lib/record-time-columns';
import { saveSpreadsheetFile } from '@/lib/spreadsheet';

const statisticDimensions = [
  { value: 'major', label: '专业', icon: BookOpen },
  { value: 'type', label: '去向类型', icon: BriefcaseBusiness },
] as const;

export function GraduationPage(props: WorkspaceContext) {
  return <GraduationWorkspace key={props.workspaceID} {...props} />;
}

function GraduationWorkspace({
  workspaceID,
  actions,
  refresh: refreshWorkspace,
}: WorkspaceContext) {
  const canList = actions.includes('graduation.list');
  const canStatistics = actions.includes('graduation.statistics');
  const [filters, setFilters] = useState<GraduationFilters>(emptyFilters);
  const [dimension, setDimension] = useState<Dimension>('major');
  const [editing, setEditing] = useState<Graduation | null>(null);
  const [clearing, setClearing] = useState<Graduation | null>(null);
  const [transfer, setTransfer] = useState(false);
  const pagination = useTablePagination();
  const client = useQueryClient();
  const refresh = useApiAction();
  const search = useApiAction();
  const changeDimension = useApiAction();
  const remove = useApiAction();
  const download = useApiAction();
  const list = useQuery({
    ...graduationListOptions(workspaceID, filters, pagination.page, pagination.pageSize),
    enabled: canList,
    placeholderData: keepPreviousData,
  });
  const statistics = useQuery({
    ...graduationStatisticsOptions(workspaceID, filters, dimension),
    enabled: canStatistics,
    placeholderData: keepPreviousData,
  });
  const options = useQuery({
    ...graduationFilterOptions(workspaceID, !canList),
    enabled: canList || canStatistics,
  });
  const denied = (error: Error | null) =>
    error instanceof ApiError && [401, 403, 404].includes(error.status);
  const data = denied(list.error) ? undefined : list.data;
  const enums = denied(options.error) ? undefined : options.data;
  const fetchPage = (page: number, size: number, selected = filters) =>
    client.query({ ...graduationListOptions(workspaceID, selected, page, size), staleTime: 0 });
  const fetchResults = async (selected: GraduationFilters, page: number) => {
    const results = await Promise.allSettled([
      ...(canList ? [fetchPage(page, pagination.pageSize, selected)] : []),
      ...(canStatistics
        ? [
            client.query({
              ...graduationStatisticsOptions(workspaceID, selected, dimension),
              staleTime: 0,
            }),
          ]
        : []),
    ]);
    const failure = results.find((result) => result.status === 'rejected');
    if (failure)
      throw failure.reason instanceof Error
        ? failure.reason
        : new Error('查询毕业去向失败，请重试');
  };
  const applyFilters = async (values: GraduationFilters) => {
    const next = { ...values, keyword: values.keyword.trim() };
    setFilters(next);
    pagination.setPage(1);
    await fetchResults(next, 1);
  };
  const { draft, setDraft, changeFilters } = useFilterForm(emptyFilters, applyFilters);
  const columns: ColumnDef<Graduation>[] = [
    { accessorKey: 'student_no', header: '学号' },
    { accessorKey: 'student_name', header: '姓名' },
    { accessorKey: 'education_level', header: '学历' },
    { accessorKey: 'major', header: '专业' },
    { accessorKey: 'grade', header: '年级' },
    { accessorKey: 'graduation_year', header: '毕业年份' },
    {
      accessorKey: 'type_label',
      header: '类型',
      cell: ({ row }) => row.original.type_label || pendingOption.label,
    },
    {
      accessorKey: 'destination',
      header: '毕业去向',
      cell: ({ row }) => (
        <span className="block max-w-80 truncate" title={row.original.destination}>
          {formatValue(row.original.destination)}
        </span>
      ),
    },
    ...recordTimeColumns<Graduation>((record) => record),
    {
      id: 'actions',
      header: '操作',
      cell: ({ row }) => (
        <div className="flex items-center gap-1">
          {actions.includes('graduation.update') ? (
            <TableActionButton
              onClick={() => {
                setEditing(row.original);
              }}
            >
              {row.original.type ? <Pencil aria-hidden="true" /> : <Link2 aria-hidden="true" />}
              {row.original.type ? '编辑' : '绑定'}
            </TableActionButton>
          ) : null}
          {actions.includes('graduation.delete') &&
          (row.original.type || row.original.destination) ? (
            <TableActionButton
              onClick={() => {
                setClearing(row.original);
              }}
            >
              <Trash2 aria-hidden="true" />
              清除
            </TableActionButton>
          ) : null}
        </div>
      ),
    },
  ];
  return (
    <section className="space-y-5" aria-labelledby="graduation-page-title">
      <header className="flex min-h-8 flex-wrap items-center justify-between gap-3">
        <h1 id="graduation-page-title" className="sr-only">
          毕业去向
        </h1>
        <nav aria-label="面包屑" className="text-xs text-muted-foreground">
          <ol className="flex items-center gap-2">
            <li>学生工作</li>
            <li aria-hidden="true">/</li>
            <li aria-current="page">毕业去向</li>
          </ol>
        </nav>
        <div className="ml-auto flex flex-wrap items-center gap-2 max-sm:w-full">
          {canStatistics ? (
            <ButtonGroup
              aria-label="统计维度"
              className="max-w-full rounded-lg bg-muted/60 p-px max-sm:w-full [&>button]:flex-1 [&>button]:rounded-md"
            >
              {statisticDimensions.map(({ value, label, icon: Icon }) => (
                <Button
                  key={value}
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="aria-pressed:bg-background aria-pressed:text-accent-foreground aria-pressed:shadow-panel"
                  aria-pressed={dimension === value}
                  loading={changeDimension.isPending(value)}
                  disabled={
                    changeDimension.pending ||
                    search.pending ||
                    refresh.pending ||
                    statistics.isPending
                  }
                  onClick={() => {
                    if (value === dimension) return;
                    void changeDimension.run(value, async () => {
                      await client.query({
                        ...graduationStatisticsOptions(workspaceID, filters, value),
                        staleTime: 0,
                      });
                      setDimension(value);
                    });
                  }}
                >
                  <Icon aria-hidden="true" />
                  {label}
                </Button>
              ))}
            </ButtonGroup>
          ) : null}
          {actions.includes('graduation.import') ? (
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
          {actions.includes('graduation.export') ? (
            <span className="inline-flex items-center gap-2">
              <Button
                variant="outline"
                loading={download.pending}
                disabled={denied(list.error)}
                onClick={() => {
                  void (async () => {
                    const result = await download.run('export', () =>
                      exportGraduations(workspaceID, filters),
                    );
                    if (result.ok) saveSpreadsheetFile(result.data);
                  })();
                }}
              >
                <Download aria-hidden="true" />
                导出
              </Button>
            </span>
          ) : null}
          <Button
            variant="outline"
            loading={
              refresh.pending ||
              (canList && list.isPending) ||
              (canStatistics && statistics.isPending) ||
              ((canList || canStatistics) && options.isPending)
            }
            disabled={search.pending || changeDimension.pending}
            onClick={() => {
              void refresh.run('refresh', async () => {
                await refreshWorkspace();
                await Promise.all([
                  fetchResults(filters, pagination.page),
                  ...(canList || canStatistics
                    ? [
                        client.query({
                          ...graduationFilterOptions(workspaceID, !canList),
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
      {canStatistics && !denied(statistics.error) ? (
        <GraduationChart key={dimension} data={statistics.data ?? []} dimension={dimension} />
      ) : null}
      {canList || canStatistics ? (
        <FilterPanel
          onSubmit={(event) => {
            event.preventDefault();
            void search.run('search', () => applyFilters(draft));
          }}
          actions={
            <>
              <SearchResetButton
                loading={search.isPending('reset')}
                disabled={search.pending || changeDimension.pending || refresh.pending}
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
                disabled={search.pending || changeDimension.pending || refresh.pending}
              >
                <Search aria-hidden="true" />
                搜索
              </Button>
            </>
          }
        >
          <FilterField label="学历">
            <SearchSelect
              multiple
              label="学历"
              className="w-full"
              options={enums?.education_levels ?? []}
              value={draft.education_level}
              loading={options.isFetching}
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
              loading={options.isFetching}
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
              loading={options.isFetching}
              onValueChange={(value) => {
                changeFilters({ grade: value });
              }}
            />
          </FilterField>
          <FilterField label="学号或姓名" htmlFor="graduation-page-keyword">
            <Input
              id="graduation-page-keyword"
              aria-label="学号或姓名"
              placeholder="学号或姓名"
              className="w-full"
              maxLength={100}
              autoComplete="off"
              data-1p-ignore="true"
              value={draft.keyword}
              onChange={(event) => {
                setDraft((current) => ({ ...current, keyword: event.target.value }));
              }}
            />
          </FilterField>
          <FilterField label="毕业年份">
            <SearchSelect
              multiple
              label="毕业年份"
              className="w-full"
              options={[unsetGraduationYearOption, ...(enums?.graduation_years ?? [])]}
              value={draft.graduation_year}
              loading={options.isFetching}
              onValueChange={(value) => {
                changeFilters({ graduation_year: value });
              }}
            />
          </FilterField>
          <FilterField label="类型">
            <SearchSelect
              multiple
              label="类型"
              className="w-full"
              options={[pendingOption, ...(enums?.types ?? [])]}
              value={draft.type}
              loading={options.isFetching}
              onValueChange={(value) => {
                changeFilters({ type: value });
              }}
            />
          </FilterField>
        </FilterPanel>
      ) : null}
      {list.error ? <ErrorMessage>{list.error.message}</ErrorMessage> : null}
      {statistics.error ? <ErrorMessage>{statistics.error.message}</ErrorMessage> : null}
      {options.error ? <ErrorMessage>{options.error.message}</ErrorMessage> : null}
      {canList ? (
        <Card className="gap-0 overflow-hidden py-0">
          <ManagementTable
            tableID="graduations"
            columns={columns}
            data={data?.items ?? []}
            total={data?.total ?? 0}
            page={pagination.page}
            pageSize={pagination.pageSize}
            loading={list.isFetching}
            emptyMessage="暂无符合条件的学生"
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
      ) : !canStatistics ? (
        <Card className="p-6 text-sm text-muted-foreground">没有查看毕业去向的权限</Card>
      ) : null}
      {editing ? (
        <GraduationEditor
          workspaceID={workspaceID}
          record={editing}
          types={enums?.types ?? []}
          onClose={() => {
            setEditing(null);
          }}
        />
      ) : null}
      {clearing ? (
        <Drawer
          title="清除毕业去向"
          description={formatIdentity(clearing.student_name, clearing.student_no)}
          busy={remove.pending}
          onClose={() => {
            setClearing(null);
          }}
          footer={
            <>
              <Button
                variant="secondary"
                disabled={remove.pending}
                onClick={() => {
                  setClearing(null);
                }}
              >
                取消
              </Button>
              <Button
                variant="destructive"
                loading={remove.pending}
                onClick={() => {
                  void remove.run('clear', async () => {
                    await clearGraduation(workspaceID, clearing);
                    await Promise.all([
                      client.invalidateQueries({ queryKey: graduationKey(workspaceID) }),
                      client.invalidateQueries({ queryKey: studentKey(workspaceID) }),
                    ]);
                    setClearing(null);
                  });
                }}
              >
                <Trash2 aria-hidden="true" />
                清除
              </Button>
            </>
          }
        >
          <p className="p-6 text-sm">清除该学生的类型和毕业去向后，该学生继续计入待就业。</p>
        </Drawer>
      ) : null}
      {transfer ? (
        <GraduationTransfer
          workspaceID={workspaceID}
          onClose={() => {
            setTransfer(false);
          }}
        />
      ) : null}
    </section>
  );
}
