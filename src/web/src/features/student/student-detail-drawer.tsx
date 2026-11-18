import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Eye, EyeOff, Pencil, RefreshCw } from 'lucide-react';
import { useRef, useState } from 'react';

import { ErrorMessage } from '@/components/error-message';
import { Button } from '@/components/ui/button';
import { Drawer } from '@/components/ui/drawer';
import {
  readStudentSensitive,
  type SensitiveData,
  studentDetailOptions,
  studentKey,
} from '@/features/student/api';
import { sensitiveAction } from '@/features/student/fields';
import { StudentDetail } from '@/features/student/student-detail';
import { StudentEditor } from '@/features/student/student-editor';
import { ApiError } from '@/lib/api';
import { useApiAction } from '@/lib/api-action';
import { formatIdentity } from '@/lib/format';
import { useLoadingDelay } from '@/lib/loading';

function scrollToSection(section: HTMLElement | null) {
  if (typeof section?.scrollIntoView === 'function') {
    section.scrollIntoView({ block: 'start' });
  }
}

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
  const reveal = useApiAction();
  const loading = useLoadingDelay(valid && canDetail && detail.isPending);
  const client = useQueryClient();
  const [editing, setEditing] = useState(false);
  const [sensitiveOpen, setSensitiveOpen] = useState(false);
  const sensitiveRef = useRef<HTMLElement>(null);
  const denied = detail.error instanceof ApiError && [401, 403, 404].includes(detail.error.status);
  const student = denied ? undefined : detail.data;
  const editingStudent = editing && Boolean(student) && (canBasic || canSensitive);
  const detailKey = student
    ? `${workspaceID}:${String(student.id)}:${String(student.revision)}`
    : null;
  const sensitiveKey = canSensitive ? detailKey : null;
  const [sensitive, setSensitive] = useState<{ key: string | null; data: SensitiveData | null }>({
    key: sensitiveKey,
    data: null,
  });
  if (sensitive.key !== sensitiveKey) {
    setSensitive({ key: sensitiveKey, data: null });
  }
  return (
    <Drawer
      title="学生详情"
      description={student ? formatIdentity(student.name, student.student_no) : '查看学生档案'}
      busy={refresh.pending || editingStudent}
      onClose={onClose}
      footer={
        <>
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
          {student && canSensitive ? (
            <Button
              variant="outline"
              loading={reveal.pending}
              onClick={() => {
                if (sensitive.data) {
                  setSensitive({ key: sensitiveKey, data: null });
                  setSensitiveOpen(false);
                  return;
                }
                setSensitiveOpen(true);
                requestAnimationFrame(() => {
                  scrollToSection(sensitiveRef.current);
                });
                void reveal
                  .run('reveal', () => readStudentSensitive(workspaceID, student.id))
                  .then((result) => {
                    if (!result.ok) return;
                    // 学生、版本或权限变化后，丢弃此前发出的请求结果。
                    setSensitive((current) =>
                      current === sensitive ? { key: sensitiveKey, data: result.data } : current,
                    );
                  });
              }}
            >
              {sensitive.data ? <EyeOff aria-hidden="true" /> : <Eye aria-hidden="true" />}
              {sensitive.data ? '隐藏原始值' : '显示原始值'}
            </Button>
          ) : null}
          {student && (canBasic || canSensitive) ? (
            <Button
              onClick={() => {
                setEditing(true);
              }}
            >
              <Pencil aria-hidden="true" />
              编辑
            </Button>
          ) : null}
        </>
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
        <StudentDetail
          student={student}
          sensitive={sensitive.key === sensitiveKey ? sensitive.data : null}
          sensitiveOpen={sensitiveOpen}
          onSensitiveOpenChange={setSensitiveOpen}
          sensitiveRef={sensitiveRef}
        />
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
