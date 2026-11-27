import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Save } from 'lucide-react';
import { useRef, useState } from 'react';

import { ErrorMessage } from '@/components/error-message';
import { Button } from '@/components/ui/button';
import { DatePicker } from '@/components/ui/date-picker';
import { Drawer, type DrawerHandle } from '@/components/ui/drawer';
import { FileUploadButton } from '@/components/ui/file-upload-button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { SearchSelect } from '@/components/ui/search-select';
import { Textarea } from '@/components/ui/textarea';
import {
  deleteWorkAttachment,
  saveWork,
  uploadWorkAttachment,
  type WorkInput,
  workKey,
  workOwnerOptions,
  type WorkRecord,
} from '@/features/monthly-work/api';
import { WorkAttachmentList } from '@/features/monthly-work/attachments';
import { workAction, type WorkCategory } from '@/features/monthly-work/category';
import { LinksEditor } from '@/features/monthly-work/links-editor';
import type { CurrentUser } from '@/features/user/api';
import { ApiError } from '@/lib/api';
import { useApiAction, useApiMutation } from '@/lib/api-action';
import { currentCalendarDate } from '@/lib/calendar';
import { uploadHint } from '@/lib/file-upload';
import { formatIdentity } from '@/lib/format';

export function WorkEditor({
  workspaceID,
  category = 'student',
  record,
  actions,
  currentUser,
  onClose,
}: {
  workspaceID: string;
  category?: WorkCategory;
  record: WorkRecord | null;
  actions: string[];
  currentUser?: Pick<CurrentUser, 'id' | 'display_name' | 'username'> | null | undefined;
  onClose: () => void;
}) {
  const [original] = useState(record);
  const [values, setValues] = useState<WorkInput>(
    record ?? {
      date: currentCalendarDate(),
      work_item: '',
      owner_id: currentUser?.id ?? 0,
      participant_ids: [],
      description: '',
      links: [],
    },
  );
  const can = (operation: string) => actions.includes(workAction(category, operation));
  const isOwner = original !== null && original.owner_id === currentUser?.id;
  const canSave = can(original ? 'update' : 'create');
  const canReadFiles = can('attachment.read');
  const canUpload = isOwner && can('attachment.upload');
  const canDeleteFile = isOwner && can('attachment.delete');
  const showCreateHint = !original && can('attachment.upload');
  const client = useQueryClient();
  const owners = useQuery({
    ...workOwnerOptions(workspaceID, category),
    enabled: canSave && can('list'),
  });
  const denied = owners.error instanceof ApiError && [401, 403, 404].includes(owners.error.status);
  const availableOptions = (denied ? [] : (owners.data ?? [])).map((item) => ({
    value: String(item.value),
    label: item.label,
  }));
  const ownerOptions = [...availableOptions];
  const initialOwner = original
    ? {
        value: String(original.owner_id),
        label: formatIdentity(original.owner_name, original.owner_username),
      }
    : currentUser && {
        value: String(currentUser.id),
        label: formatIdentity(currentUser.display_name, currentUser.username),
      };
  if (initialOwner && !ownerOptions.some((item) => item.value === initialOwner.value)) {
    ownerOptions.push(initialOwner);
  }
  const participantOptions = [...availableOptions];
  for (const person of original?.participants ?? []) {
    if (!participantOptions.some((option) => option.value === String(person.id))) {
      participantOptions.push({
        value: String(person.id),
        label: formatIdentity(person.name, person.username),
      });
    }
  }
  const drawer = useRef<DrawerHandle>(null);
  const upload = useApiAction();
  const remove = useApiAction();
  const invalidate = () => client.invalidateQueries({ queryKey: workKey(workspaceID, category) });
  const mutation = useApiMutation({
    mutationFn: async () => {
      await saveWork(workspaceID, original, values, category);
      await invalidate();
    },
    onSuccess: () => {
      drawer.current?.close();
    },
  });
  const busy = mutation.isPending || upload.pending || remove.pending;
  return (
    <Drawer
      ref={drawer}
      title={original ? '编辑月度工作' : '新增月度工作'}
      description={canSave ? '日期、工作项目及负责人必填' : undefined}
      busy={busy}
      onClose={onClose}
      form={{
        onSubmit: (event) => {
          event.preventDefault();
          if (canSave && !busy) mutation.mutate();
        },
      }}
      dismissLabel={canSave ? '取消' : '关闭'}
      footer={
        canSave ? (
          <Button
            type="submit"
            loading={mutation.isPending}
            disabled={upload.pending || remove.pending}
          >
            <Save aria-hidden="true" />
            保存
          </Button>
        ) : null
      }
    >
      <div className="space-y-4 p-6">
        <fieldset disabled={busy || !canSave} className="space-y-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="work-date">日期</Label>
              <DatePicker
                id="work-date"
                label="日期"
                required
                min="1900-01-01"
                max="9999-12-31"
                value={values.date}
                onValueChange={(date) => {
                  setValues((current) => ({ ...current, date }));
                }}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="work-owner">负责人</Label>
              <SearchSelect
                id="work-owner"
                label="负责人"
                className="w-full"
                options={ownerOptions}
                value={values.owner_id ? String(values.owner_id) : ''}
                loading={owners.isFetching}
                onValueChange={(value) => {
                  setValues((current) => ({ ...current, owner_id: Number(value) }));
                }}
              />
              {owners.error ? <ErrorMessage>{owners.error.message}</ErrorMessage> : null}
            </div>
            <div className="space-y-2 sm:col-span-2">
              <Label htmlFor="work-participants">参与人</Label>
              <SearchSelect
                id="work-participants"
                label="参与人"
                multiple
                allLabel="未选择参与人"
                className="w-full"
                options={participantOptions}
                value={values.participant_ids.map(String)}
                loading={owners.isFetching}
                onValueChange={(value) => {
                  setValues((current) => ({ ...current, participant_ids: value.map(Number) }));
                }}
              />
            </div>
            <div className="space-y-2 sm:col-span-2">
              <Label htmlFor="work-item">工作项目</Label>
              <Input
                id="work-item"
                required
                maxLength={100}
                value={values.work_item}
                onChange={(event) => {
                  setValues((current) => ({ ...current, work_item: event.target.value }));
                }}
              />
            </div>
          </div>
          <div className="space-y-2">
            <Label htmlFor="work-description">详细说明</Label>
            <Textarea
              id="work-description"
              rows={6}
              maxLength={10000}
              value={values.description}
              onChange={(event) => {
                setValues((current) => ({ ...current, description: event.target.value }));
              }}
            />
          </div>
        </fieldset>
        {(original ? canReadFiles || canUpload : showCreateHint) ? (
          <section className="space-y-3 border-t border-border pt-5" aria-label="附件">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h2 className="text-base leading-5 font-medium">附件</h2>
              {original && canUpload ? (
                <FileUploadButton
                  loading={upload.pending}
                  disabled={mutation.isPending || remove.pending}
                  onFileSelect={(file, updateUpload) => {
                    if (busy) return;
                    void upload.run('upload', async () => {
                      try {
                        await uploadWorkAttachment(
                          workspaceID,
                          original.id,
                          file,
                          category,
                          updateUpload,
                        );
                      } finally {
                        await invalidate();
                      }
                      updateUpload('complete');
                    });
                  }}
                />
              ) : null}
            </div>
            {canUpload ? <p className="text-xs text-muted-foreground">{uploadHint()}</p> : null}
            {showCreateHint ? (
              <p className="text-xs text-muted-foreground">创建后由负责人上传附件</p>
            ) : null}
            {original && canReadFiles ? (
              <WorkAttachmentList
                workspaceID={workspaceID}
                category={category}
                workID={original.id}
                busy={busy}
                onDelete={
                  canDeleteFile
                    ? async (file) => {
                        if (busy) return false;
                        const result = await remove.run('delete', async () => {
                          try {
                            await deleteWorkAttachment(workspaceID, file, category);
                          } finally {
                            await invalidate();
                          }
                        });
                        return result.ok;
                      }
                    : undefined
                }
              />
            ) : null}
          </section>
        ) : null}
        <fieldset disabled={busy || !canSave}>
          <LinksEditor
            value={values.links}
            onChange={(links) => {
              setValues((current) => ({ ...current, links }));
            }}
          />
        </fieldset>
      </div>
    </Drawer>
  );
}
