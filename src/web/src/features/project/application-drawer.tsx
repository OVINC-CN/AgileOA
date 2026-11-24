import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, Pencil, Save, Send, Trash2, Undo2 } from 'lucide-react';
import { useId, useState } from 'react';

import { ErrorMessage } from '@/components/error-message';
import { TableTimestamp } from '@/components/table-timestamp';
import { AutoTextarea } from '@/components/ui/auto-textarea';
import { Button } from '@/components/ui/button';
import { Drawer } from '@/components/ui/drawer';
import { DrawerSteps } from '@/components/ui/drawer-steps';
import { DrawerTabs } from '@/components/ui/drawer-tabs';
import { FileUploadButton } from '@/components/ui/file-upload-button';
import { Label } from '@/components/ui/label';
import { SearchSelect } from '@/components/ui/search-select';
import { type Project, projectKey } from '@/features/project/api';
import {
  type Application,
  type ApplicationAttachment,
  type ApplicationContent,
  applicationDetailOptions,
  type ApplicationOperation,
  type ApplicationScope,
  changeApplication,
  createApplication,
  deleteApplicationAttachment,
  studentProjectOptions,
  uploadApplicationAttachment,
} from '@/features/project/application-api';
import {
  ApplicationContentEditor,
  ApplicationMetadata,
  type ApplicationValues,
  type EditableApplicationDetail,
} from '@/features/project/application-content';
import { ApplicationContentVersions } from '@/features/project/application-content-versions';
import { ApplicationFlow, ApplicationHistory } from '@/features/project/application-timeline';
import { validateUniqueDetailNames } from '@/features/project/detail-validation';
import { useApiAction } from '@/lib/api-action';
import { uploadHint, type UploadUpdate, validateUploadFile } from '@/lib/file-upload';
import { formatValue } from '@/lib/format';

const emptyValues: ApplicationValues = { statement: '', rank: null, score: null };
const applicationTabs = [
  { value: 'main', label: '主要信息' },
  { value: 'flow', label: '审批流程' },
  { value: 'history', label: '流转记录' },
] as const;
const valuesOf = (record: Application): ApplicationValues => ({
  statement: record.statement,
  rank: record.rank,
  score: record.score,
});
const detailsOf = (record: Application): EditableApplicationDetail[] =>
  record.details.map((item) => ({
    key: item.key,
    name: item.name,
    description: item.description,
    score: item.score,
    attachments: item.attachments,
  }));
const contentOf = (
  values: ApplicationValues,
  items: EditableApplicationDetail[],
  calculationEnabled = false,
): ApplicationContent => ({
  ...values,
  ...(calculationEnabled ? { rank: null, score: null } : {}),
  details: items.map(({ key, name, description, score }) => ({ key, name, description, score })),
});
const storedContent = (record: Application) =>
  contentOf(valuesOf(record), detailsOf(record), record.score_calculation_enabled);

export function ApplicationDrawer({
  workspaceID,
  scope,
  record: initialRecord,
  onClose,
}: {
  workspaceID: string;
  scope: ApplicationScope;
  record: Application | null;
  onClose: () => void;
}) {
  const [savedRecord, setRecord] = useState(initialRecord);
  const [syncedRecord, setSyncedRecord] = useState(initialRecord);
  const [editing, setEditing] = useState(initialRecord === null);
  const [values, setValues] = useState<ApplicationValues>(
    initialRecord ? valuesOf(initialRecord) : emptyValues,
  );
  const [items, setItems] = useState<EditableApplicationDetail[]>(
    initialRecord ? detailsOf(initialRecord) : [],
  );
  const [tab, setTab] = useState('main');
  const [keyword, setKeyword] = useState('');
  const [project, setProject] = useState<Project | null>(null);
  const [decision, setDecision] = useState<'approve' | 'reject' | null>(null);
  const [comment, setComment] = useState('');
  const [deleting, setDeleting] = useState<ApplicationAttachment | null>(null);
  const [uploadTarget, setUploadTarget] = useState<{ key: string; completed: boolean } | null>(
    null,
  );
  // 项目配置刷新立即生效，编辑草稿及原并发版本保留到保存时复核。
  const record =
    savedRecord &&
    initialRecord?.id === savedRecord.id &&
    initialRecord.revision >= savedRecord.revision
      ? {
          ...savedRecord,
          project_name: initialRecord.project_name,
          project_description: initialRecord.project_description,
          year: initialRecord.year,
          self_apply_deadline: initialRecord.self_apply_deadline,
          allow_self_apply: initialRecord.allow_self_apply,
          score_calculation_enabled: initialRecord.score_calculation_enabled,
          detail_names: initialRecord.detail_names,
          can_edit: initialRecord.can_edit,
          can_approve: initialRecord.can_approve,
          can_reject: initialRecord.can_reject,
        }
      : savedRecord;
  const tabsID = useId();
  const client = useQueryClient();
  const action = useApiAction();
  const files = useApiAction();
  const busy = action.pending || files.pending;
  const calculationEnabled =
    record?.score_calculation_enabled ?? project?.score_calculation?.enabled ?? false;
  const needsSave =
    !record ||
    JSON.stringify(contentOf(values, items, calculationEnabled)) !==
      JSON.stringify(storedContent(record));
  const projects = useQuery({
    ...studentProjectOptions(workspaceID, keyword),
    enabled: record === null,
  });
  // 通知刷新同步最新详情，正在编辑或提交时保留当前表单与关闭保护。
  if (initialRecord !== syncedRecord && !editing && !busy) {
    setSyncedRecord(initialRecord);
    if (initialRecord && initialRecord.revision >= (record?.revision ?? 0)) {
      setRecord(initialRecord);
      setValues(valuesOf(initialRecord));
      setItems(detailsOf(initialRecord));
    }
  }
  const options = (projects.data?.items ?? []).map((item) => ({
    value: String(item.id),
    label: `${String(item.year)} · ${item.name}`,
  }));
  if (project && !options.some((item) => item.value === String(project.id)))
    options.push({ value: String(project.id), label: `${String(project.year)} · ${project.name}` });

  const accept = async (next: Application, resetForm: boolean) => {
    setRecord(next);
    if (resetForm) {
      setValues(valuesOf(next));
      setItems(detailsOf(next));
    } else {
      setValues((current) => ({ ...current, score: next.score, rank: next.rank }));
      const attachments = new Map(next.details.map((detail) => [detail.key, detail.attachments]));
      setItems((current) =>
        current.map((item) => ({
          ...item,
          attachments: attachments.get(item.key) ?? item.attachments,
        })),
      );
    }
    await client.invalidateQueries({ queryKey: projectKey(workspaceID) });
  };
  const formContent = () => {
    if (items.some((item) => item.name.trim() === '')) {
      setTab('main');
      throw new Error('请选择子项名称');
    }
    validateUniqueDetailNames(items);
    return contentOf(values, items, calculationEnabled);
  };
  const save = () => {
    if (record && !record.can_edit) return;
    void action.run('save', async () => {
      const content = formContent();
      let next: Application;
      if (record) next = await changeApplication(workspaceID, scope, record, 'update', content, '');
      else {
        if (!project) throw new Error('请选择申报项目');
        next = await createApplication(workspaceID, project.id, content, true);
      }
      await accept(next, true);
      setEditing(false);
    });
  };
  const process = (operation: ApplicationOperation) => {
    void action.run(operation, async () => {
      if (!record) return;
      if (operation === 'reject' && !comment.trim()) throw new Error('请填写驳回意见');

      // 重新申报提交当前表单，校验或请求失败时保留编辑内容。
      const content = operation === 'submit' && editing ? formContent() : storedContent(record);
      const next = await changeApplication(
        workspaceID,
        scope,
        record,
        operation,
        content,
        operation === 'reject' || operation === 'approve' ? comment : '',
      );
      await accept(next, true);
      setEditing(false);
      setDecision(null);
      setComment('');
    });
  };
  const cancelEdit = () => {
    if (!record) {
      onClose();
      return;
    }
    setValues(valuesOf(record));
    setItems(detailsOf(record));
    setEditing(false);
  };
  const upload = (key: string, file: File, updateUpload: UploadUpdate) => {
    if (record && !record.can_edit) return;
    void files.run(`upload:${key}`, async () => {
      validateUploadFile(file);
      const content = formContent();
      if (!items.find((item) => item.key === key)?.name.trim())
        throw new Error('请选择子项名称后上传附件');
      let working = record;
      let contentSaved = false;
      try {
        if (working && needsSave) {
          working = await changeApplication(workspaceID, scope, working, 'update', content, '');
          contentSaved = true;
        } else if (!working) {
          if (!project) throw new Error('请选择申报项目');
          working = await createApplication(workspaceID, project.id, content, true);
          contentSaved = true;
        }
        setRecord(working);
        await accept(
          await uploadApplicationAttachment(workspaceID, scope, working, key, file, updateUpload),
          false,
        );
        updateUpload('complete');
        setUploadTarget((current) => (current?.key === key ? { key, completed: true } : current));
      } catch (cause) {
        if (working) {
          try {
            const current = await client.query({
              ...applicationDetailOptions(workspaceID, scope, working.id),
              staleTime: 0,
            });
            await accept(current, false);
          } catch {
            // 保留原始上传错误，后续刷新可重新读取待完成附件。
          }
        }
        setUploadTarget(null);
        if (contentSaved) {
          const message = cause instanceof Error ? cause.message : '上传失败，请重试';
          throw new Error(`申报内容已保存，附件上传未完成。${message}`);
        }
        throw cause;
      }
    });
  };
  const removeFile = () => {
    if (!record || !record.can_edit || !deleting) return;
    const file = deleting;
    void files.run(`delete:${String(file.id)}`, async () => {
      try {
        await accept(await deleteApplicationAttachment(workspaceID, scope, record, file), false);
        setDeleting(null);
      } catch (cause) {
        try {
          const current = await client.query({
            ...applicationDetailOptions(workspaceID, scope, record.id),
            staleTime: 0,
          });
          await accept(current, false);
          const latest = current.details
            .flatMap((detail) => detail.attachments)
            .find((item) => item.id === file.id);
          if (latest) setDeleting(latest);
        } catch {
          // 后续刷新可按已持久化的删除状态继续重试。
        }
        throw cause;
      }
    });
  };
  const fileProps = {
    workspaceID,
    scope,
    applicationID: record?.id ?? 0,
    busy,
    deleting: (id: number) => files.isPending(`delete:${String(id)}`),
  };
  const canProcess = record && (record.can_edit || record.can_approve || record.can_reject);
  const reapplying = scope === 'mine' && record?.status === 'rejected';
  const showEditSteps = editing && initialRecord !== null;
  const deadline = record ? record.self_apply_deadline : project?.self_apply_deadline;
  return (
    <>
      <Drawer
        title={editing ? (record ? '编辑申报' : '新建申报') : '申报详情'}
        description={
          record
            ? `${record.project_name} · ${String(record.year)}`
            : '填写申报内容，保存后可提交审批'
        }
        onClose={onClose}
        busy={busy}
        contentClassName="p-6 text-sm"
        headerNavigation={
          showEditSteps ? (
            <DrawerSteps
              id={tabsID}
              label="申报内容分类"
              value={tab}
              items={applicationTabs}
              onChange={setTab}
              disabled={busy}
            />
          ) : undefined
        }
        footerNavigation={
          !showEditSteps ? (
            <DrawerTabs
              id={tabsID}
              label="申报内容分类"
              value={tab}
              items={applicationTabs}
              onChange={setTab}
              disabled={editing && busy}
            />
          ) : undefined
        }
        footerDismiss={
          editing ? (
            <Button type="button" variant="cancel" disabled={busy} onClick={cancelEdit}>
              取消
            </Button>
          ) : undefined
        }
        footer={
          editing ? (
            <>
              <Button
                type="button"
                variant={reapplying ? 'outline' : 'default'}
                loading={action.isPending('save')}
                disabled={busy || (record !== null && !record.can_edit)}
                onClick={save}
              >
                <Save aria-hidden="true" />
                保存
              </Button>
              {reapplying ? (
                <Button
                  type="button"
                  disabled={busy || !record.can_approve}
                  loading={action.isPending('submit')}
                  onClick={() => {
                    process('submit');
                  }}
                >
                  <Send aria-hidden="true" />
                  提交
                </Button>
              ) : null}
            </>
          ) : reapplying ? (
            <Button
              type="button"
              disabled={busy || !record.can_edit}
              onClick={() => {
                setEditing(true);
                setTab('main');
              }}
            >
              <Pencil aria-hidden="true" />
              重新申报
            </Button>
          ) : (
            <>
              {record?.can_edit ? (
                <Button
                  type="button"
                  variant="outline"
                  disabled={busy}
                  onClick={() => {
                    setEditing(true);
                    setTab('main');
                  }}
                >
                  <Pencil aria-hidden="true" />
                  编辑
                </Button>
              ) : null}
              {scope !== 'mine' && record?.can_reject ? (
                <Button
                  type="button"
                  variant="outline"
                  disabled={busy}
                  onClick={() => {
                    setComment('');
                    setDecision('reject');
                  }}
                >
                  <Undo2 aria-hidden="true" />
                  驳回
                </Button>
              ) : null}
              {canProcess ? (
                <Button
                  type="button"
                  disabled={busy || !record.can_approve}
                  loading={action.isPending(scope === 'mine' ? 'submit' : 'approve')}
                  onClick={() => {
                    if (scope === 'mine') process('submit');
                    else {
                      setComment('');
                      setDecision('approve');
                    }
                  }}
                >
                  {scope === 'mine' ? <Send aria-hidden="true" /> : <Check aria-hidden="true" />}
                  {scope === 'mine' ? '提交' : '通过'}
                </Button>
              ) : null}
            </>
          )
        }
      >
        <div
          role="tabpanel"
          id={`${tabsID}-panel-main`}
          aria-labelledby={`${tabsID}-tab-main`}
          hidden={tab !== 'main'}
          className="space-y-5"
        >
          {record ? (
            <ApplicationMetadata record={record} />
          ) : (
            <div className="space-y-2">
              <Label
                htmlFor="application-project"
                className="text-xs font-normal text-muted-foreground"
              >
                申报项目
              </Label>
              <SearchSelect
                id="application-project"
                label="申报项目"
                className="w-full"
                value={project ? String(project.id) : ''}
                options={options}
                loading={projects.isFetching}
                disabled={busy}
                onSearchChange={setKeyword}
                onValueChange={(id) => {
                  setProject(projects.data?.items.find((item) => item.id === Number(id)) ?? null);
                  setItems([]);
                }}
              />
              {project ? (
                <p className="text-xs leading-5 wrap-anywhere whitespace-pre-wrap text-muted-foreground">
                  {formatValue(project.description)}
                </p>
              ) : null}
              {projects.data && projects.data.total > projects.data.items.length ? (
                <p className="text-xs text-muted-foreground">可搜索项目名称以查找更多奖项</p>
              ) : null}
              {projects.error ? <ErrorMessage>{projects.error.message}</ErrorMessage> : null}
            </div>
          )}
          {scope === 'mine' && (!record || record.status === 'draft') && deadline ? (
            <div
              role="note"
              aria-label="自动提交说明"
              className="space-y-2 rounded-xl border border-primary/20 bg-primary/5 p-4 text-xs leading-5"
            >
              <p>
                自行申报截止时间：
                <TableTimestamp value={deadline} />
              </p>
              <p className="text-muted-foreground">
                已保存的草稿将在截止时间后 1
                分钟自动提交并进入审批。请在截止前完善并保存材料、完成附件上传，也可提前手动提交。未保存的修改不会自动提交。
              </p>
            </div>
          ) : null}
          {editing ? (
            <ApplicationContentEditor
              values={values}
              items={items}
              names={record?.detail_names ?? project?.detail_names ?? []}
              busy={busy || (record !== null && !record.can_edit)}
              calculationEnabled={calculationEnabled}
              onValues={(patch) => {
                setValues((current) => ({ ...current, ...patch }));
              }}
              onItems={(next) => {
                if (
                  calculationEnabled &&
                  (next.length !== items.length ||
                    next.some((item) => {
                      const before = items.find((current) => current.key === item.key);
                      return before?.name !== item.name || before.score !== item.score;
                    }))
                )
                  setValues((current) => ({ ...current, score: null }));
                setItems(next);
              }}
              filesFor={(key) => ({
                ...fileProps,
                uploading: files.isPending(`upload:${key}`),
                onUpload: (file, updateUpload) => {
                  upload(key, file, updateUpload);
                },
                onPrepareUpload: needsSave
                  ? () => {
                      setUploadTarget({ key, completed: false });
                    }
                  : undefined,
                onDelete: setDeleting,
              })}
            />
          ) : record ? (
            <ApplicationContentVersions
              key={`${workspaceID}:${scope}:${String(record.id)}`}
              record={record}
              files={fileProps}
              active={tab === 'main'}
            />
          ) : null}
        </div>
        <section
          role="tabpanel"
          id={`${tabsID}-panel-flow`}
          aria-labelledby={`${tabsID}-tab-flow`}
          hidden={tab !== 'flow'}
        >
          <ApplicationFlow record={record} preview={project?.approval_steps ?? []} />
        </section>
        <section
          role="tabpanel"
          id={`${tabsID}-panel-history`}
          aria-labelledby={`${tabsID}-tab-history`}
          hidden={tab !== 'history'}
        >
          <ApplicationHistory record={record} />
        </section>
      </Drawer>
      {uploadTarget ? (
        <Drawer
          title="保存内容并上传附件"
          description={items.find((item) => item.key === uploadTarget.key)?.name ?? '申报子项附件'}
          onClose={() => {
            setUploadTarget(null);
          }}
          busy={files.pending}
          footerDismiss={
            <Button
              variant="cancel"
              disabled={files.pending}
              onClick={() => {
                setUploadTarget(null);
              }}
            >
              {uploadTarget.completed ? '关闭' : '取消'}
            </Button>
          }
          footer={
            <FileUploadButton
              label="选择文件并保存上传"
              loading={files.isPending(`upload:${uploadTarget.key}`)}
              disabled={busy || uploadTarget.completed}
              onFileSelect={(file, updateUpload) => {
                upload(uploadTarget.key, file, updateUpload);
              }}
            />
          }
        >
          <div className="space-y-3 p-5 text-sm leading-5">
            <p>选择文件后，将先保存当前申报说明、排名、分数及全部子项，再上传附件。</p>
            <p>保存成功后，取消编辑或上传失败均不会撤销本次保存。</p>
            {record?.status === 'pending' ? <p>修改申报内容或附件会清除本级已有签署。</p> : null}
            <p className="text-xs text-muted-foreground">{uploadHint()}</p>
          </div>
        </Drawer>
      ) : null}
      {decision ? (
        <Drawer
          title={decision === 'reject' ? '驳回申报' : '通过申报'}
          description={
            decision === 'reject'
              ? '直接退回申报人，修改并重新提交后从第一级重新审批'
              : '确认通过当前审批节点，可填写审批说明'
          }
          onClose={() => {
            setDecision(null);
          }}
          busy={action.pending}
          footerDismiss={
            <Button
              variant="cancel"
              disabled={action.pending}
              onClick={() => {
                setDecision(null);
              }}
            >
              取消
            </Button>
          }
          footer={
            <Button
              loading={action.isPending(decision)}
              onClick={() => {
                process(decision);
              }}
            >
              {decision === 'reject' ? <Undo2 aria-hidden="true" /> : <Check aria-hidden="true" />}
              {decision === 'reject' ? '驳回' : '通过'}
            </Button>
          }
        >
          <div className="space-y-2 p-6">
            <Label htmlFor="application-decision-comment" className="text-xs text-muted-foreground">
              {decision === 'reject' ? '驳回意见' : '审批说明（选填）'}
            </Label>
            <AutoTextarea
              id="application-decision-comment"
              maxLength={10000}
              value={comment}
              onChange={(event) => {
                setComment(event.target.value);
              }}
              disabled={action.pending}
            />
          </div>
        </Drawer>
      ) : null}
      {deleting ? (
        <Drawer
          title="删除附件"
          description={deleting.filename}
          onClose={() => {
            setDeleting(null);
          }}
          busy={files.pending}
          footerDismiss={
            <Button
              variant="cancel"
              disabled={files.pending}
              onClick={() => {
                setDeleting(null);
              }}
            >
              取消
            </Button>
          }
          footer={
            <Button loading={files.isPending(`delete:${String(deleting.id)}`)} onClick={removeFile}>
              <Trash2 aria-hidden="true" />
              删除
            </Button>
          }
        >
          <p className="p-6 text-sm">将永久删除此子项附件及文件，历史审计保留。</p>
        </Drawer>
      ) : null}
    </>
  );
}
