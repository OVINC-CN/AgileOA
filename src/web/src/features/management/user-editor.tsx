import { zodResolver } from '@hookform/resolvers/zod';
import { useQueryClient } from '@tanstack/react-query';
import { Check, Copy, KeyRound, Plus, Save } from 'lucide-react';
import { Switch } from 'radix-ui';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';

import { ErrorMessage } from '@/components/error-message';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Drawer } from '@/components/ui/drawer';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { updateUser, type UserRow } from '@/features/management/api';
import { createUser, type CurrentUser, currentUserKey, userEmailSchema } from '@/features/user/api';
import { useApiMutation } from '@/lib/api-action';
import { getConfig } from '@/lib/config';
import { formatIdentity } from '@/lib/format';
import { showMessage } from '@/lib/messages';
import { generateInitialPassword } from '@/lib/password';

const schema = z
  .object({
    username: z
      .string()
      .trim()
      .regex(/^[a-z0-9][a-z0-9._-]{2,63}$/, '请输入 3 至 64 位小写英文、数字或 ._-'),
    display_name: z.string().trim().min(1, '请输入显示名称').max(100, '最多输入 100 个字符'),
    email: userEmailSchema,
    password: z.string(),
    password_login_enabled: z.boolean(),
    is_enabled: z.boolean(),
  })
  .refine(
    (values) => {
      const size = new TextEncoder().encode(values.password).length;
      return !values.password_login_enabled || values.password === '' || (size >= 12 && size <= 72);
    },
    { path: ['password'], message: '密码长度须为 12 至 72 字节' },
  );
type Values = z.infer<typeof schema>;

export function UserEditor({
  user,
  currentID,
  onClose,
  onSaved,
}: {
  user: UserRow | null;
  currentID: number;
  onClose: () => void;
  onSaved: (message: string) => void;
}) {
  const { passwordLoginEnabled: passwordLoginAvailable } = getConfig();
  const client = useQueryClient();
  const [initialPassword] = useState(() =>
    user || !passwordLoginAvailable ? '' : generateInitialPassword(),
  );
  const [created, setCreated] = useState<CurrentUser | null>(null);
  const [copyState, setCopyState] = useState<'idle' | 'copied' | 'failed'>('idle');
  const {
    register,
    handleSubmit,
    setValue,
    watch,
    formState: { errors },
  } = useForm<Values>({
    resolver: zodResolver(
      schema.refine(
        (values) =>
          !values.password_login_enabled ||
          !user ||
          user.password_login_enabled ||
          values.password !== '',
        { path: ['password'], message: '启用密码登录时请设置临时密码' },
      ),
    ),
    defaultValues: {
      username: user?.username ?? '',
      display_name: user?.display_name ?? '',
      email: user?.email ?? '',
      password: initialPassword,
      password_login_enabled: user?.password_login_enabled ?? passwordLoginAvailable,
      is_enabled: user?.is_enabled ?? true,
    },
  });
  const enabled = watch('is_enabled');
  const passwordLoginEnabled = watch('password_login_enabled');
  const mutation = useApiMutation({
    mutationFn: async (values: Values) => {
      const result = await (user ? updateUser(user, values) : createUser(values));
      await Promise.all([
        client.invalidateQueries({ queryKey: ['management', 'users'] }),
        client.invalidateQueries({ queryKey: ['management', 'audit'] }),
      ]);
      if (user?.id === currentID) {
        await client.invalidateQueries({ queryKey: currentUserKey });
      }
      return result;
    },
    onSuccess: (result) => {
      if (user) {
        onSaved(`已更新用户 ${result.username}`);
      } else {
        setCreated(result);
      }
    },
  });
  const submit = (values: Values) => {
    mutation.mutate(values);
  };
  if (created) {
    const finish = () => {
      onSaved(`已添加用户 ${created.username}`);
    };
    const copy = async () => {
      try {
        await navigator.clipboard.writeText(
          created.password_login_enabled
            ? `用户名：${created.username}\n初始密码：${initialPassword}`
            : `用户名：${created.username}\n登录方式：邮件登录`,
        );
        setCopyState('copied');
      } catch {
        setCopyState('failed');
      }
    };
    return (
      <Drawer
        title="用户已创建"
        description={
          created.password_login_enabled
            ? '初始密码仅在本次创建后展示。用户首次登录时须修改密码。'
            : '已关闭密码登录，用户可通过邮件登录，无需修改密码。'
        }
        onClose={finish}
        footer={
          <Button
            onClick={() => {
              void copy();
            }}
          >
            {copyState === 'copied' ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}
            {copyState === 'copied' ? '已复制' : '复制登录信息'}
          </Button>
        }
      >
        <div className="space-y-5 p-6">
          <div className="flex items-center gap-3 text-sm">
            <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-warning text-warning-foreground">
              <KeyRound className="size-4" aria-hidden="true" />
            </span>
            <span className="min-w-0 flex-1 wrap-anywhere">{created.display_name}</span>
          </div>
          <div className="space-y-2">
            <Label htmlFor="created-username">用户名</Label>
            <Input id="created-username" value={created.username} readOnly autoComplete="off" />
          </div>
          {created.password_login_enabled ? (
            <div className="space-y-2">
              <Label htmlFor="created-password">初始密码</Label>
              <Input
                id="created-password"
                value={initialPassword}
                readOnly
                autoComplete="off"
                className="tracking-wider"
              />
            </div>
          ) : null}
          {copyState === 'copied' ? (
            <p role="status" className="text-xs text-muted-foreground">
              登录信息已复制
            </p>
          ) : null}
          {copyState === 'failed' ? (
            <ErrorMessage>无法访问剪贴板，请选中上方登录信息手动复制保存。</ErrorMessage>
          ) : null}
        </div>
      </Drawer>
    );
  }
  return (
    <Drawer
      title={user ? '编辑用户' : '添加用户'}
      truncateDescription={!!user}
      busy={mutation.isPending}
      description={
        user
          ? formatIdentity(user.display_name, user.username)
          : passwordLoginEnabled
            ? '填写用户名和显示名称，自动生成 16 位初始密码。'
            : '填写账号信息，用户通过邮件登录，无需设置或修改密码。'
      }
      onClose={() => {
        if (!mutation.isPending) {
          onClose();
        }
      }}
      footer={
        <>
          <Button type="button" variant="outline" disabled={mutation.isPending} onClick={onClose}>
            取消
          </Button>
          <Button type="submit" loading={mutation.isPending}>
            {user ? <Save aria-hidden="true" /> : <Plus aria-hidden="true" />}
            {user ? '保存更改' : '添加用户'}
          </Button>
        </>
      }
      form={{
        noValidate: true,
        onSubmit: (event) => {
          void handleSubmit(submit, (issues) => {
            Object.entries(issues).forEach(([name, issue]) => {
              if (issue.message) showMessage(issue.message, `editor-${name}-error`);
            });
          })(event);
        },
      }}
    >
      <fieldset disabled={mutation.isPending} className="space-y-5 p-6">
        <div className="grid gap-5 sm:grid-cols-2">
          <div className="space-y-2">
            <Label htmlFor="editor-username">用户名</Label>
            <Input
              id="editor-username"
              autoComplete="off"
              autoCapitalize="none"
              spellCheck={false}
              readOnly={Boolean(user)}
              required
              {...register('username')}
              className={user ? 'bg-muted text-muted-foreground' : undefined}
              aria-invalid={Boolean(errors.username)}
              aria-describedby={errors.username ? 'editor-username-error' : undefined}
            />
            {errors.username ? (
              <ErrorMessage id="editor-username-error">{errors.username.message}</ErrorMessage>
            ) : null}
          </div>
          <div className="space-y-2">
            <Label htmlFor="editor-name">显示名称</Label>
            <Input
              id="editor-name"
              autoComplete="off"
              required
              maxLength={100}
              {...register('display_name')}
              aria-invalid={Boolean(errors.display_name)}
              aria-describedby={errors.display_name ? 'editor-name-error' : undefined}
            />
            {errors.display_name ? (
              <ErrorMessage id="editor-name-error">{errors.display_name.message}</ErrorMessage>
            ) : null}
          </div>
        </div>
        <div className="space-y-2">
          <Label htmlFor="editor-email">邮箱（可选）</Label>
          <Input
            id="editor-email"
            type="email"
            autoComplete="off"
            autoCapitalize="none"
            spellCheck={false}
            maxLength={254}
            {...register('email')}
            aria-invalid={Boolean(errors.email)}
            aria-describedby={errors.email ? 'editor-email-error' : undefined}
          />
          {errors.email ? (
            <ErrorMessage id="editor-email-error">{errors.email.message}</ErrorMessage>
          ) : null}
        </div>
        {passwordLoginAvailable ? (
          <div className="flex items-start gap-3">
            <Checkbox
              id="editor-password-login"
              checked={passwordLoginEnabled}
              aria-describedby="editor-password-login-help"
              onChange={(event) => {
                setValue('password_login_enabled', event.target.checked, { shouldValidate: true });
                setValue('password', event.target.checked && !user ? initialPassword : '', {
                  shouldValidate: true,
                });
              }}
            />
            <div className="space-y-1">
              <Label htmlFor="editor-password-login">允许密码登录</Label>
              <p id="editor-password-login-help" className="text-xs text-muted-foreground">
                关闭后仅可通过邮件登录，无需修改密码。
              </p>
            </div>
          </div>
        ) : null}
        {user && passwordLoginAvailable && passwordLoginEnabled ? (
          <div className="space-y-2">
            <Label htmlFor="editor-password">
              {user.password_login_enabled ? '临时密码（可选）' : '临时密码'}
            </Label>
            <Input
              id="editor-password"
              type="password"
              autoComplete="new-password"
              {...register('password')}
              aria-invalid={Boolean(errors.password)}
              aria-describedby={errors.password ? 'editor-password-error' : 'editor-password-help'}
            />
            {errors.password ? (
              <ErrorMessage id="editor-password-error">{errors.password.message}</ErrorMessage>
            ) : (
              <p id="editor-password-help" className="text-xs text-muted-foreground">
                {user.password_login_enabled
                  ? '留空保留原密码。重置后，用户须重新登录并更换临时密码。'
                  : '启用密码登录须设置临时密码，用户下次登录时须修改。'}
              </p>
            )}
          </div>
        ) : null}
        {user ? (
          <div className="flex items-center justify-between gap-4 border-t border-border pt-5">
            <div className="space-y-1">
              <Label htmlFor="editor-enabled">启用账号</Label>
              <p className="text-xs text-muted-foreground">停用后该用户将无法登录</p>
            </div>
            <Switch.Root
              id="editor-enabled"
              checked={enabled}
              disabled={user.id === currentID}
              onCheckedChange={(value) => {
                setValue('is_enabled', value);
              }}
              className="h-6 w-10 rounded-full bg-border p-0.5 transition-colors disabled:opacity-60 data-[state=checked]:bg-primary"
            >
              <Switch.Thumb className="block size-5 rounded-full bg-primary-foreground shadow-sm transition-transform data-[state=checked]:translate-x-4" />
            </Switch.Root>
          </div>
        ) : null}
        {mutation.error ? <ErrorMessage>{mutation.error.message}</ErrorMessage> : null}
      </fieldset>
    </Drawer>
  );
}
