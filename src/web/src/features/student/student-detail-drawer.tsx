import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Pencil, RefreshCw } from 'lucide-react';
import { useId, useLayoutEffect, useRef, useState } from 'react';

import { ErrorMessage } from '@/components/error-message';
import { Button } from '@/components/ui/button';
import { Drawer } from '@/components/ui/drawer';
import { DrawerTabs } from '@/components/ui/drawer-tabs';
import { studentDetailOptions, studentKey } from '@/features/student/api';
import { sensitiveAction, studentRecordTabs } from '@/features/student/fields';
import { StudentDetail, StudentSensitiveDetail } from '@/features/student/student-detail';
import { StudentEditor } from '@/features/student/student-editor';
import { ApiError } from '@/lib/api';
import { useApiAction } from '@/lib/api-action';
import { formatIdentity } from '@/lib/format';
import { useLoadingDelay } from '@/lib/loading';

export function StudentDetailDrawer({
  workspaceID,
  studentID,
  actions,
  onRefresh,
  onClose,
}: {
  workspaceID: string;
  studentID: string;
  actions: string[];
  onRefresh: () => Promise<unknown>;
  onClose: () => void;
}) {
  const id = Number(studentID);
  const valid = /^\d+$/.test(studentID) && Number.isSafeInteger(id) && id > 0;
  const canDetail = actions.includes('student.detail');
  const canBasic = actions.includes('student.update');
  const canSensitive = actions.includes(sensitiveAction);
  const detail = useQuery({
    ...studentDetailOptions(workspaceID, id),
    enabled: valid && canDetail,
  });
  const refresh = useApiAction();
  const loading = useLoadingDelay(valid && canDetail && detail.isPending);
  const client = useQueryClient();
  const [editing, setEditing] = useState(false);
  const [tab, setTab] = useState('basic');
  const tabsID = useId();
  const panels = useRef<HTMLDivElement>(null);
  const tabs = studentRecordTabs.filter((item) => item.group === 'basic' || canSensitive);
  const activeTab = !canSensitive && tab === 'sensitive' ? 'basic' : tab;
  useLayoutEffect(() => {
    const content = panels.current?.parentElement;
    if (content) content.scrollTop = 0;
  }, [activeTab]);
  const denied = detail.error instanceof ApiError && [401, 403, 404].includes(detail.error.status);
  const student = valid && canDetail && !denied ? detail.data : undefined;
  const editingStudent = editing && Boolean(student) && (canBasic || canSensitive);
  return (
    <Drawer
      title="学生详情"
      truncateDescription={!!student}
      description={student ? formatIdentity(student.name, student.student_no) : '查看学生档案'}
      busy={refresh.pending || editingStudent}
      onClose={onClose}
      footerNavigation={
        student && !loading ? (
          <DrawerTabs
            id={tabsID}
            label="学生信息分区"
            value={activeTab}
            items={tabs}
            onChange={setTab}
          />
        ) : undefined
      }
      footerRefresh={
        <Button
          variant="outline"
          loading={refresh.pending || (valid && canDetail && detail.isPending)}
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
      }
      footer={
        student && (canBasic || canSensitive) ? (
          <Button
            onClick={() => {
              setEditing(true);
            }}
          >
            <Pencil aria-hidden="true" />
            编辑
          </Button>
        ) : null
      }
    >
      {!valid ? (
        <ErrorMessage>学生地址无效。</ErrorMessage>
      ) : !canDetail ? (
        <ErrorMessage>没有查看学生详情的权限</ErrorMessage>
      ) : null}
      {detail.error || refresh.error ? (
        <ErrorMessage>{(detail.error ?? refresh.error)?.message}</ErrorMessage>
      ) : null}
      {loading ? (
        <p role="status" className="px-6 py-12 text-center text-sm text-muted-foreground">
          正在读取学生详情
        </p>
      ) : student ? (
        <div ref={panels} className="px-5">
          {tabs.map((item) => (
            <section
              key={item.value}
              role="tabpanel"
              id={`${tabsID}-panel-${item.value}`}
              aria-labelledby={`${tabsID}-tab-${item.value}`}
              hidden={activeTab !== item.value}
            >
              {item.group === 'basic' ? (
                <StudentDetail student={student} tab={item} />
              ) : canSensitive && activeTab === item.value ? (
                <StudentSensitiveDetail workspaceID={workspaceID} student={student} tab={item} />
              ) : null}
            </section>
          ))}
        </div>
      ) : null}
      {editingStudent && student ? (
        <StudentEditor
          workspaceID={workspaceID}
          student={student}
          canQuery={actions.includes('student.list')}
          canBasic={canBasic}
          canDetail={canDetail}
          canSensitive={canSensitive}
          onClose={() => {
            setEditing(false);
          }}
        />
      ) : null}
    </Drawer>
  );
}
