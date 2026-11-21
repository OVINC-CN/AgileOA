import { useQueryClient } from '@tanstack/react-query';
import { Plus, Save } from 'lucide-react';
import { useState } from 'react';
import { z } from 'zod';

import { ErrorMessage } from '@/components/error-message';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Drawer } from '@/components/ui/drawer';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { createWorkspace, updateWorkspace, type Workspace } from '@/features/workspace/api';
import { useApiMutation } from '@/lib/api-action';
import { showMessage } from '@/lib/messages';

const schema = z.object({
  name: z.string().trim().min(1, '请输入工作空间名称').max(100, '名称最多 100 个字符'),
  description: z.string().trim().max(500, '说明最多 500 个字符'),
  is_enabled: z.boolean(),
});

export function WorkspaceEditor({
  workspace,
  onClose,
  onSaved,
}: {
  workspace: Workspace | null;
  onClose: () => void;
  onSaved: (message: string) => void;
}) {
  const client = useQueryClient();
  const [values, setValues] = useState({
    name: workspace?.name ?? '',
    description: workspace?.description ?? '',
    is_enabled: workspace?.is_enabled ?? true,
  });
  const mutation = useApiMutation({
    mutationFn: async (input: z.infer<typeof schema>) => {
      const result = await (workspace
        ? updateWorkspace(workspace, input)
        : createWorkspace({ name: input.name, description: input.description }));
      await Promise.all([
        client.invalidateQueries({ queryKey: ['management', 'workspaces'] }),
        client.invalidateQueries({ queryKey: ['management', 'audit'] }),
        client.invalidateQueries({ queryKey: ['workspace'] }),
      ]);
      return result;
    },
    onSuccess: (result) => {
      onSaved(`已${workspace ? '更新' : '创建'}工作空间 ${result.name}`);
    },
  });
  return (
    <Drawer
      title={workspace ? '编辑工作空间' : '创建工作空间'}
      busy={mutation.isPending}
      description={workspace ? workspace.name : '创建后在权限管理中为用户勾选操作权限。'}
      onClose={() => {
        if (!mutation.isPending) onClose();
      }}
      footer={
        <>
          <Button type="button" variant="cancel" disabled={mutation.isPending} onClick={onClose}>
            取消
          </Button>
          <Button type="submit" loading={mutation.isPending}>
            {workspace ? <Save aria-hidden="true" /> : <Plus aria-hidden="true" />}
            {workspace ? '保存' : '创建工作空间'}
          </Button>
        </>
      }
      form={{
        onSubmit: (event) => {
          event.preventDefault();
          const parsed = schema.safeParse(values);
          if (!parsed.success) {
            showMessage(parsed.error.issues[0]?.message ?? '请检查表单');
            return;
          }
          mutation.mutate(parsed.data);
        },
      }}
    >
      <div className="space-y-5 p-6">
        <div className="space-y-2">
          <Label htmlFor="workspace-name">名称</Label>
          <Input
            id="workspace-name"
            value={values.name}
            maxLength={100}
            required
            disabled={mutation.isPending}
            onChange={(event) => {
              setValues({ ...values, name: event.target.value });
            }}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="workspace-description">说明</Label>
          <Textarea
            id="workspace-description"
            rows={3}
            maxLength={500}
            value={values.description}
            disabled={mutation.isPending}
            onChange={(event) => {
              setValues({ ...values, description: event.target.value });
            }}
          />
        </div>
        {workspace ? (
          <label className="flex items-center gap-2 text-sm">
            <Checkbox
              checked={values.is_enabled}
              disabled={mutation.isPending}
              onChange={(event) => {
                setValues({ ...values, is_enabled: event.target.checked });
              }}
            />
            启用工作空间
          </label>
        ) : null}
        {mutation.error ? <ErrorMessage>{mutation.error.message}</ErrorMessage> : null}
      </div>
    </Drawer>
  );
}
