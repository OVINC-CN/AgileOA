import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import {
  ChevronDown,
  Download,
  Eye,
  FileDown,
  Pencil,
  Plus,
  RefreshCw,
  Search,
  Trash2,
  Upload,
} from 'lucide-react';
import { DropdownMenu } from 'radix-ui';
import { useState } from 'react';
import { Link, useSearchParams } from 'react-router';

import { ErrorMessage } from '@/components/error-message';
import { FilterPanel } from '@/components/filter-panel';
import { ManagementTable } from '@/components/management-table';
import { SearchResetButton } from '@/components/search-reset-button';
import { TableActionButton } from '@/components/table-action-button';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { SearchSelect } from '@/components/ui/search-select';
import {
  exportStudentReport,
  saveStudentFile,
  type Student,
  type StudentFilters,
  studentKey,
  studentListOptions,
  studentStatisticsOptions,
} from '@/features/student/api';
import { StudentFilterSelect } from '@/features/student/field-input';
import { politicalStatusLabel, sensitiveAction, studentFields } from '@/features/student/fields';
import { StudentChart } from '@/features/student/student-chart';
import { StudentDetailDrawer } from '@/features/student/student-detail-drawer';
import { DeleteStudent, StudentEditor } from '@/features/student/student-editor';
import { StudentTransfer } from '@/features/student/student-transfer';
import { ApiError } from '@/lib/api';
import { useApiAction, useInitialLoading } from '@/lib/api-action';
import { useFilterForm } from '@/lib/filter-form';
import { formatValue } from '@/lib/format';
import { useTablePagination } from '@/lib/pagination';
import { recordTimeColumns } from '@/lib/record-time-columns';

const empty: Student[] = [];
const studentDetailParam = 'student_id';
const enrolledStatus = { value: '在籍', label: '在籍' };
const defaultFilters: StudentFilters = { keyword: '', status: [enrolledStatus.value] };
const exportItemClass =
  'flex cursor-pointer items-center gap-2 rounded-md px-3 py-2 text-sm outline-none data-highlighted:bg-accent';
const common = [
  'education_level',
  'grade',
  'class_name',
  'class_position',
  'accommodation_type',
  'campus',
  'building',
  'room',
  'ethnicity',
  'advisor',
  'has_religion',
];
export function StudentPage({
  workspaceID,
  actions,
  onRefresh,
}: {
  workspaceID: string;
  workspaceName: string;
  actions: string[];
  onRefresh: () => Promise<unknown>;
}) {
  const client = useQueryClient();
  const pagination = useTablePagination();
  const [params, setParams] = useSearchParams();
  const studentID = params.get(studentDetailParam);
  const [filters, setFilters] = useState(defaultFilters);
  const [editor, setEditor] = useState<Student | null | undefined>(undefined);
  const [deleting, setDeleting] = useState<Student | null>(null);
  const [transfer, setTransfer] = useState<'import' | 'export' | null>(null);
  const refresh = useApiAction();
  const search = useApiAction();
  const reportExport = useApiAction();
  const can = (action: string) => actions.includes(action);
  const list = useQuery({
    ...studentListOptions(workspaceID, filters, pagination.page, pagination.pageSize),
    enabled: can('student.list'),
    placeholderData: keepPreviousData,
  });
  const statistics = useQuery({
    ...studentStatisticsOptions(workspaceID, filters),
    enabled: can('student.statistics'),
    placeholderData: keepPreviousData,
  });
  const initial = useInitialLoading(
    (can('student.list') && list.isPending) || (can('student.statistics') && statistics.isPending),
  );
  const statsDenied =
    statistics.error instanceof ApiError && [401, 403, 404].includes(statistics.error.status);
  const denied = list.error instanceof ApiError && [401, 403, 404].includes(list.error.status);
  const data = denied ? undefined : list.data;
  const applyFilters = async (values: StudentFilters) => {
    setFilters(values);
    pagination.setPage(1);
    const results = await Promise.allSettled([
      ...(can('student.list')
        ? [
            client.query({
              ...studentListOptions(workspaceID, values, 1, pagination.pageSize),
              staleTime: 0,
            }),
          ]
        : []),
      ...(can('student.statistics')
        ? [client.query({ ...studentStatisticsOptions(workspaceID, values), staleTime: 0 })]
        : []),
    ]);
    const failure = results.find((result) => result.status === 'rejected');
    if (failure)
      throw failure.reason instanceof Error ? failure.reason : new Error('查询学生失败，请重试');
  };
  const { draft, setDraft, changeFilters } = useFilterForm(defaultFilters, applyFilters);
  const columns: ColumnDef<Student>[] = [
    { accessorKey: 'student_no', header: '学号' },
    { accessorKey: 'name', header: '姓名' },
    { accessorKey: 'grade', header: '年级' },
    { accessorKey: 'status', header: '状态' },
    { accessorKey: 'advisor', header: '导师' },
    { accessorKey: 'major', header: '专业' },
    { accessorKey: 'education_level', header: '学历' },
    { accessorKey: 'political_status', header: politicalStatusLabel },
    {
      accessorKey: 'campus',
      header: '住宿园区',
      cell: ({ row: entry }) =>
        formatValue(
          entry.original.campus ||
            (entry.original.accommodation_type === 'off_campus' ? '校外住宿' : ''),
        ),
    },
    { accessorKey: 'building', header: '宿舍楼' },
    { accessorKey: 'room', header: '宿舍号' },
    { accessorKey: 'phone', header: '电话' },
    ...recordTimeColumns<Student>((record) => record),
    {
      id: 'operations',
      header: '操作',
      cell: ({ row: entry }) => {
        const student = entry.original;
        const detailParams = new URLSearchParams(params);
        detailParams.set(studentDetailParam, String(student.id));
        return (
          <div className="flex items-center gap-1">
            {can('student.detail') ? (
              <TableActionButton asChild>
                <Link to={{ search: detailParams.toString() }}>
                  <Eye aria-hidden="true" />
                  详情
                </Link>
              </TableActionButton>
            ) : null}
            {can('student.update') || can(sensitiveAction) ? (
              <TableActionButton
                onClick={() => {
                  setEditor(student);
                }}
              >
                <Pencil aria-hidden="true" />
                编辑
              </TableActionButton>
            ) : null}
            {can('student.delete') ? (
              <TableActionButton
                onClick={() => {
                  setDeleting(student);
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
  const selectors = studentFields.filter((field) => field.enumerable);
  const selectedValues = (key: string) => {
    const values = draft[key];
    return Array.isArray(values) ? values : [];
  };
  return (
    <section className="space-y-5" aria-labelledby="student-page-title">
      <header className="flex min-h-8 flex-wrap items-center justify-between gap-3">
        <h1 id="student-page-title" className="sr-only">
          学生信息
        </h1>
        <nav aria-label="面包屑" className="text-xs text-muted-foreground">
          <ol className="flex items-center gap-2">
            <li>学生工作</li>
            <li aria-hidden="true">/</li>
            <li aria-current="page">学生信息</li>
          </ol>
        </nav>
        <div className="ml-auto flex flex-wrap items-center gap-2">
          {can('student.import') && can(sensitiveAction) ? (
            <Button
              variant="outline"
              onClick={() => {
                setTransfer('import');
              }}
            >
              <Upload aria-hidden="true" />
              导入
            </Button>
          ) : null}
          {can('student.export') ? (
            <span className="inline-flex items-center gap-2">
              <DropdownMenu.Root modal={false}>
                <DropdownMenu.Trigger asChild>
                  <Button variant="outline" loading={reportExport.pending} className="group">
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
                    <DropdownMenu.Item
                      className={exportItemClass}
                      onSelect={() => {
                        setTransfer('export');
                      }}
                    >
                      <Download className="size-4" aria-hidden="true" />
                      导出所有数据
                    </DropdownMenu.Item>
                    {can(sensitiveAction) ? (
                      <DropdownMenu.Item
                        className={exportItemClass}
                        onSelect={() => {
                          void reportExport.run('religion', async () => {
                            saveStudentFile(
                              await exportStudentReport(workspaceID, filters, 'religion'),
                            );
                          });
                        }}
                      >
                        <FileDown className="size-4" aria-hidden="true" />
                        导出 XJ 明细表
                      </DropdownMenu.Item>
                    ) : null}
                    {can(sensitiveAction) ? (
                      <DropdownMenu.Item
                        className={exportItemClass}
                        onSelect={() => {
                          void reportExport.run('off-campus', async () => {
                            saveStudentFile(
                              await exportStudentReport(workspaceID, filters, 'off_campus'),
                            );
                          });
                        }}
                      >
                        <FileDown className="size-4" aria-hidden="true" />
                        导出校外住宿明细表
                      </DropdownMenu.Item>
                    ) : null}
                  </DropdownMenu.Content>
                </DropdownMenu.Portal>
              </DropdownMenu.Root>
            </span>
          ) : null}
          {can('student.create') && can(sensitiveAction) ? (
            <Button
              onClick={() => {
                setEditor(null);
              }}
            >
              <Plus aria-hidden="true" />
              添加学生
            </Button>
          ) : null}
          <Button
            variant="outline"
            loading={refresh.pending || initial}
            onClick={() => {
              void refresh.run('refresh', async () => {
                await onRefresh();
                await client.invalidateQueries({ queryKey: studentKey(workspaceID) });
              });
            }}
          >
            <RefreshCw aria-hidden="true" />
            刷新
          </Button>
        </div>
      </header>
      {can('student.statistics') ? (
        <StudentChart data={statsDenied ? undefined : statistics.data} />
      ) : null}
      {can('student.list') || can('student.statistics') || can('student.export') ? (
        <FilterPanel
          onSubmit={(event) => {
            event.preventDefault();
            void search.run('search', () => applyFilters(draft));
          }}
          more={
            <>
              <div className="space-y-1.5">
                <span className="text-xs text-muted-foreground">班内任职状态</span>
                <SearchSelect
                  multiple
                  allLabel="全部任职状态"
                  label="班内任职状态"
                  className="w-full"
                  value={selectedValues('position_status')}
                  onValueChange={(value) => {
                    changeFilters({ position_status: value });
                  }}
                  options={[
                    { value: 'assigned', label: '有任职' },
                    { value: 'unassigned', label: '无任职或未填写' },
                  ]}
                />
              </div>
              {selectors
                .filter((field) => !common.includes(field.key))
                .map((field) => (
                  <StudentFilterSelect
                    key={field.key}
                    field={field}
                    workspaceID={workspaceID}
                    canQuery={can(field.group === 'basic' ? 'student.list' : sensitiveAction)}
                    defaultOption={field.key === 'status' ? enrolledStatus : undefined}
                    value={selectedValues(field.key)}
                    onChange={(value) => {
                      changeFilters({ [field.key]: value });
                    }}
                  />
                ))}
            </>
          }
          actions={
            <>
              <SearchResetButton
                loading={search.isPending('reset')}
                disabled={search.pending}
                onClick={() => {
                  void search.run('reset', async () => {
                    setDraft(defaultFilters);
                    await applyFilters(defaultFilters);
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
          <label htmlFor="student-keyword" className="space-y-1.5">
            <span className="text-xs text-muted-foreground">学号或姓名</span>
            <Input
              id="student-keyword"
              name="student-filter-keyword"
              type="search"
              autoComplete="off"
              data-1p-ignore="true"
              aria-label="学号或姓名"
              placeholder="搜索"
              maxLength={100}
              value={draft.keyword}
              onChange={(event) => {
                setDraft((current) => ({ ...current, keyword: event.target.value }));
              }}
            />
          </label>
          {common.map((key) => {
            const field = selectors.find((item) => item.key === key);
            return field ? (
              <StudentFilterSelect
                key={key}
                field={field}
                workspaceID={workspaceID}
                canQuery={can(field.group === 'basic' ? 'student.list' : sensitiveAction)}
                value={selectedValues(key)}
                onChange={(value) => {
                  changeFilters({ [key]: value });
                }}
              />
            ) : null;
          })}
        </FilterPanel>
      ) : null}
      {refresh.error || search.error || statistics.error ? (
        <ErrorMessage>{(refresh.error ?? search.error ?? statistics.error)?.message}</ErrorMessage>
      ) : null}
      {can('student.list') ? (
        <Card className="gap-0 overflow-hidden border-border py-0">
          {list.error ? <ErrorMessage>{list.error.message}</ErrorMessage> : null}
          <ManagementTable
            tableID="students"
            columns={columns}
            data={data?.items ?? empty}
            total={data?.total ?? 0}
            page={pagination.page}
            pageSize={pagination.pageSize}
            loading={list.isPending || search.pending || refresh.pending}
            emptyMessage="暂无符合条件的学生"
            onPage={async (page) => {
              pagination.setPage(page);
              return client.query({
                ...studentListOptions(workspaceID, filters, page, pagination.pageSize),
                staleTime: 0,
              });
            }}
            onPageSize={async (size) => {
              pagination.setPageSize(size);
              return client.query({
                ...studentListOptions(workspaceID, filters, 1, size),
                staleTime: 0,
              });
            }}
          />
        </Card>
      ) : (
        <ErrorMessage>没有查看学生明细表的权限</ErrorMessage>
      )}
      {studentID !== null ? (
        <StudentDetailDrawer
          key={`${workspaceID}:${studentID}`}
          workspaceID={workspaceID}
          studentID={studentID}
          actions={actions}
          onRefresh={onRefresh}
          onClose={() => {
            const next = new URLSearchParams(params);
            next.delete(studentDetailParam);
            setParams(next, { replace: true });
          }}
        />
      ) : null}
      {editor !== undefined ? (
        <StudentEditor
          workspaceID={workspaceID}
          student={editor}
          canQuery={can('student.list')}
          canBasic={can(editor ? 'student.update' : 'student.create')}
          canDetail={can('student.detail')}
          canSensitive={can(sensitiveAction)}
          onClose={() => {
            setEditor(undefined);
          }}
        />
      ) : null}
      {deleting ? (
        <DeleteStudent
          workspaceID={workspaceID}
          student={deleting}
          onClose={() => {
            setDeleting(null);
          }}
        />
      ) : null}
      {transfer ? (
        <StudentTransfer
          workspaceID={workspaceID}
          mode={transfer}
          filters={filters}
          actions={actions}
          onClose={() => {
            setTransfer(null);
          }}
        />
      ) : null}
    </section>
  );
}
