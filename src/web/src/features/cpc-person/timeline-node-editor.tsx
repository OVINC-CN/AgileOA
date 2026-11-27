import { RotateCcw, Save, SkipForward } from 'lucide-react';
import { useRef, useState } from 'react';

import { Button } from '@/components/ui/button';
import { DatePicker } from '@/components/ui/date-picker';
import { DeleteConfirmation } from '@/components/ui/delete-confirmation';
import { Drawer, type DrawerHandle } from '@/components/ui/drawer';
import { FileUploadButton } from '@/components/ui/file-upload-button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { SearchSelect } from '@/components/ui/search-select';
import { Textarea } from '@/components/ui/textarea';
import {
  deletePersonTimelineAttachment,
  type PersonTimelineNode,
  resetPersonTimelineNode,
  savePersonTimelineNode,
  skipPersonTimelineNode,
  type TimelineValues,
  uploadPersonTimelineAttachment,
} from '@/features/cpc-person/timeline-api';
import { PersonTimelineFiles } from '@/features/cpc-person/timeline-files';
import { PersonTimelineRules } from '@/features/cpc-person/timeline-rules';
import {
  parsePersonTimelineValues,
  timelineValueLabel,
} from '@/features/cpc-person/timeline-values';
import { studentFieldOptions, type TimelineNode } from '@/features/cpc-timeline/model';
import { useApiAction } from '@/lib/api-action';
import { getConfig } from '@/lib/config';

export function PersonTimelineNodeEditor({
  workspaceID,
  personID,
  personLabel,
  original,
  current,
  nodes,
  templateRevision,
  invalidate,
  onClose,
}: {
  workspaceID: string;
  personID: number;
  personLabel?: string | undefined;
  original: PersonTimelineNode;
  current: PersonTimelineNode;
  nodes: TimelineNode[];
  templateRevision: number;
  invalidate: () => Promise<void>;
  onClose: () => void;
}) {
  const [values, setValues] = useState<TimelineValues>(original.values);
  const [confirmingReset, setConfirmingReset] = useState(false);
  const drawer = useRef<DrawerHandle>(null);
  const resetDrawer = useRef<DrawerHandle>(null);
  const save = useApiAction();
  const reset = useApiAction();
  const skip = useApiAction();
  const upload = useApiAction();
  const remove = useApiAction();
  const busy = save.pending || skip.pending || reset.pending || upload.pending || remove.pending;
  const setValue = (id: string, value: TimelineValues[string]) => {
    setValues((previous) => ({ ...previous, [id]: value }));
  };
  return (
    <Drawer
      ref={drawer}
      title="编辑时间线节点"
      truncateDescription={!!personLabel}
      description={personLabel ? `${personLabel} · ${original.name}` : original.name}
      busy={busy || confirmingReset}
      onClose={onClose}
      form={{
        noValidate: true,
        onSubmit: (event) => {
          event.preventDefault();
          if (busy) return;
          void save.run('save', async () => {
            const parsed = parsePersonTimelineValues(original, values);
            await savePersonTimelineNode(
              workspaceID,
              personID,
              original.id,
              parsed,
              original.revision,
              templateRevision,
            );
            await invalidate();
            drawer.current?.close();
          });
        },
      }}
      dismissLabel="取消"
      footer={
        <>
          <Button
            type="button"
            variant="outline"
            loading={skip.pending}
            disabled={
              save.pending ||
              reset.pending ||
              upload.pending ||
              remove.pending ||
              current.status === 'skipped'
            }
            onClick={() => {
              if (busy) return;
              void skip.run('skip', async () => {
                await skipPersonTimelineNode(
                  workspaceID,
                  personID,
                  original.id,
                  original.revision,
                  templateRevision,
                );
                await invalidate();
                drawer.current?.close();
              });
            }}
          >
            <SkipForward aria-hidden="true" />
            跳过
          </Button>
          <Button
            type="button"
            variant="outline"
            loading={reset.pending}
            disabled={save.pending || skip.pending || upload.pending || remove.pending}
            onClick={() => {
              if (busy) return;
              setConfirmingReset(true);
            }}
          >
            <RotateCcw aria-hidden="true" />
            重置
          </Button>
          <Button
            type="submit"
            loading={save.pending}
            disabled={reset.pending || skip.pending || upload.pending || remove.pending}
          >
            <Save aria-hidden="true" />
            提交
          </Button>
        </>
      }
    >
      <fieldset disabled={busy} className="space-y-5 p-6">
        {original.description ? (
          <p className="text-sm wrap-anywhere whitespace-pre-wrap text-muted-foreground">
            {original.description}
          </p>
        ) : null}
        {original.fields.map((field) => {
          const id = `person-node-${field.id}`;
          const value = values[field.id];
          return (
            <div key={field.id} className="space-y-2">
              <Label htmlFor={id}>
                {field.name}
                {field.required && !field.student_field ? ' *' : ''}
              </Label>
              {field.student_field ? (
                <>
                  <Input
                    id={id}
                    disabled
                    readOnly
                    value={timelineValueLabel(current.values[field.id])}
                  />
                  <p className="text-xs text-muted-foreground">
                    引用学生
                    {studentFieldOptions.find((item) => item.value === field.student_field)?.label}
                    ，随学生信息更新。
                  </p>
                </>
              ) : field.type === 'boolean' ? (
                <SearchSelect
                  id={id}
                  label={field.name}
                  className="w-full"
                  value={typeof value === 'boolean' ? String(value) : ''}
                  options={[
                    { value: 'true', label: '是' },
                    { value: 'false', label: '否' },
                  ]}
                  onValueChange={(next) => {
                    setValue(field.id, next === '' ? null : next === 'true');
                  }}
                />
              ) : field.type === 'textarea' ? (
                <Textarea
                  id={id}
                  value={typeof value === 'string' ? value : ''}
                  maxLength={1000}
                  required={field.required}
                  onChange={(event) => {
                    setValue(field.id, event.target.value);
                  }}
                />
              ) : field.type === 'date' ? (
                <DatePicker
                  id={id}
                  label={field.name}
                  min="0001-01-01"
                  max="9999-12-31"
                  value={typeof value === 'string' ? value : ''}
                  required={field.required}
                  disabled={busy}
                  onValueChange={(next) => {
                    setValue(field.id, next);
                  }}
                />
              ) : (
                <Input
                  id={id}
                  type={field.type === 'number' ? 'number' : 'text'}
                  step={field.type === 'number' ? 'any' : undefined}
                  value={typeof value === 'string' || typeof value === 'number' ? value : ''}
                  maxLength={1000}
                  required={field.required}
                  onChange={(event) => {
                    setValue(field.id, event.target.value);
                  }}
                />
              )}
            </div>
          );
        })}
        <PersonTimelineRules node={current} nodes={nodes} />
        <section className="space-y-3" aria-label="附件管理">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h3 className="text-sm font-medium">附件管理</h3>
            <FileUploadButton
              loading={upload.pending}
              disabled={save.pending || skip.pending || reset.pending || remove.pending}
              onFileSelect={(file, updateUpload) => {
                if (busy) return;
                void upload.run('upload', async () => {
                  if (file.size < 1 || file.size > getConfig().fileMaxUploadBytes)
                    throw new Error('文件为空或超过大小限制');
                  try {
                    await uploadPersonTimelineAttachment(
                      workspaceID,
                      personID,
                      original.id,
                      file,
                      updateUpload,
                    );
                  } finally {
                    await invalidate();
                  }
                  updateUpload('complete');
                });
              }}
            />
          </div>
          <PersonTimelineFiles
            workspaceID={workspaceID}
            personID={personID}
            node={current}
            editing
            renderAction={(attachment) => (
              <DeleteConfirmation
                target={attachment.filename}
                onConfirm={async () => {
                  const result = await remove.run('delete', async () => {
                    try {
                      await deletePersonTimelineAttachment(workspaceID, attachment);
                    } finally {
                      await invalidate();
                    }
                  });
                  return result.ok;
                }}
                disabled={busy}
              />
            )}
          />
        </section>
      </fieldset>
      {confirmingReset ? (
        <Drawer
          ref={resetDrawer}
          title="重置节点"
          description={original.name}
          busy={reset.pending}
          onClose={() => {
            setConfirmingReset(false);
          }}
          dismissLabel="取消"
          footer={
            <Button
              type="button"
              variant="destructive"
              loading={reset.pending}
              onClick={() => {
                if (busy) return;
                void reset.run('reset', async () => {
                  try {
                    await resetPersonTimelineNode(
                      workspaceID,
                      personID,
                      original.id,
                      original.revision,
                      templateRevision,
                    );
                  } finally {
                    await invalidate();
                  }
                  resetDrawer.current?.close();
                  drawer.current?.close();
                });
              }}
            >
              <RotateCcw aria-hidden="true" />
              确认重置
            </Button>
          }
        >
          <p className="p-6 text-sm">
            重置后将清空此节点的全部填写内容和人员附件，并恢复为待处理，无法恢复。
            {original.fields.some((field) => field.student_field)
              ? '引用学生信息的字段仍按当前学生资料显示。'
              : null}
          </p>
        </Drawer>
      ) : null}
    </Drawer>
  );
}
