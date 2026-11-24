import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Paperclip, Save } from 'lucide-react';
import { useRef, useState } from 'react';

import { ErrorMessage } from '@/components/error-message';
import { Button } from '@/components/ui/button';
import { DatePicker } from '@/components/ui/date-picker';
import { Drawer, type DrawerHandle } from '@/components/ui/drawer';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { SearchSelect } from '@/components/ui/search-select';
import { Textarea } from '@/components/ui/textarea';
import {
  saveWork,
  type WorkInput,
  workKey,
  workOwnerOptions,
  type WorkRecord,
} from '@/features/monthly-work/api';
import { WorkAttachments } from '@/features/monthly-work/attachments';
import { workAction, type WorkCategory } from '@/features/monthly-work/category';
import { LinksEditor } from '@/features/monthly-work/links-editor';
import type { CurrentUser } from '@/features/user/api';
import { ApiError } from '@/lib/api';
import { useApiMutation } from '@/lib/api-action';
import { currentCalendarDate } from '@/lib/calendar';
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
  const [attachments, setAttachments] = useState(false);
  const isOwner = original !== null && original.owner_id === currentUser?.id;
  const canReadFiles = actions.includes(workAction(category, 'attachment.read'));
  const canUpload = isOwner && actions.includes(workAction(category, 'attachment.upload'));
  const canManageFiles =
    canUpload || (isOwner && actions.includes(workAction(category, 'attachment.delete')));
  const client = useQueryClient();
  const owners = useQuery({
    ...workOwnerOptions(workspaceID, category),
    enabled: actions.includes(workAction(category, 'list')),
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
  const mutation = useApiMutation({
    mutationFn: async () => {
      await saveWork(workspaceID, original, values, category);
      await client.invalidateQueries({ queryKey: workKey(workspaceID, category) });
    },
    onSuccess: () => {
      drawer.current?.close();
    },
  });
  return (
    <>
      <Drawer
        ref={drawer}
        title={original ? '编辑月度工作' : '新增月度工作'}
        description="日期、工作项目及负责人必填"
        busy={mutation.isPending}
        onClose={onClose}
        form={{
          onSubmit: (event) => {
            event.preventDefault();
            mutation.mutate();
          },
        }}
        dismissLabel="取消"
        footer={
          <Button type="submit" loading={mutation.isPending}>
            <Save aria-hidden="true" />
            保存
          </Button>
        }
      >
        <fieldset disabled={mutation.isPending} className="space-y-4 p-6">
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
          <section className="space-y-3 border-t border-border pt-5" aria-label="附件">
            <div className="flex items-center justify-between gap-3">
              <h2 className="text-base leading-5 font-medium">附件</h2>
              {original && (canReadFiles || canUpload) ? (
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => {
                    setAttachments(true);
                  }}
                >
                  <Paperclip aria-hidden="true" />
                  {canManageFiles ? '管理附件' : '查看附件'}
                </Button>
              ) : null}
            </div>
            {original || !actions.includes(workAction(category, 'attachment.upload')) ? null : (
              <p className="text-xs text-muted-foreground">创建后由负责人上传附件</p>
            )}
          </section>
          <LinksEditor
            value={values.links}
            onChange={(links) => {
              setValues((current) => ({ ...current, links }));
            }}
          />
        </fieldset>
      </Drawer>
      {attachments && original && (canReadFiles || canUpload) ? (
        <WorkAttachments
          workspaceID={workspaceID}
          category={category}
          record={original}
          actions={actions}
          isOwner={isOwner}
          onClose={() => {
            setAttachments(false);
          }}
        />
      ) : null}
    </>
  );
}
