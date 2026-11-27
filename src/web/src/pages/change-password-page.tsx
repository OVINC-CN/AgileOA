import { zodResolver } from '@hookform/resolvers/zod';
import { useQueryClient } from '@tanstack/react-query';
import { KeyRound, LogOut } from 'lucide-react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { changePassword, currentUserKey, logout, useCurrentUser } from '@/features/user/api';
import { useApiAction } from '@/lib/api-action';
import { getConfig } from '@/lib/config';
import { showMessage } from '@/lib/messages';

const schema = z
  .object({
    current: z.string().min(1, '请输入当前密码'),
    next: z.string().refine((value) => {
      const size = new TextEncoder().encode(value).length;
      return size >= 12 && size <= 72;
    }, '密码长度须为 12 至 72 字节'),
    confirmation: z.string().min(1, '请确认新密码'),
  })
  .refine(({ current, next }) => current !== next, {
    path: ['next'],
    message: '新密码不能与当前密码相同',
  })
  .refine(({ next, confirmation }) => next === confirmation, {
    path: ['confirmation'],
    message: '两次输入的新密码不一致',
  });

const fields = [
  { name: 'current', label: '当前密码', autoComplete: 'current-password' },
  { name: 'next', label: '新密码', autoComplete: 'new-password' },
  { name: 'confirmation', label: '确认新密码', autoComplete: 'new-password' },
] as const;

export function ChangePasswordPage() {
  const config = getConfig();
  const { data: user } = useCurrentUser();
  const client = useQueryClient();
  const action = useApiAction();
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm({ resolver: zodResolver(schema) });

  if (!user) return null;

  const submit = handleSubmit(
    async ({ current, next }) => {
      await action.run('save', async () => {
        const updated = await changePassword(user.username, current, next);
        await client.cancelQueries();
        client.setQueryData(currentUserKey, updated);
      });
    },
    (issues) => {
      const issue = Object.values(issues)[0];
      if (issue?.message) showMessage(issue.message, 'change-password-error');
    },
  );

  return (
    <div className="flex min-h-svh flex-col">
      <main className="flex flex-1 items-center justify-center px-4 py-12">
        <section
          aria-labelledby="change-password-title"
          className="glass-panel w-full max-w-sm rounded-2xl border p-7 sm:p-8"
        >
          <img src="/agile.svg" alt={config.appName} className="mb-8 h-6 w-auto" />
          <h1 id="change-password-title" className="text-xl font-medium">
            修改密码
          </h1>
          <p className="mt-2 mb-6 text-sm text-muted-foreground">请更换临时密码后继续使用。</p>
          <form
            noValidate
            className="space-y-5"
            onSubmit={(event) => {
              void submit(event);
            }}
          >
            {fields.map(({ name, label, autoComplete }) => (
              <div key={name} className="space-y-2">
                <Label htmlFor={name}>{label}</Label>
                <Input
                  id={name}
                  type="password"
                  autoComplete={autoComplete}
                  required
                  disabled={action.pending}
                  aria-invalid={Boolean(errors[name])}
                  {...register(name)}
                />
              </div>
            ))}
            <Button
              type="submit"
              className="w-full"
              disabled={action.pending}
              loading={action.isPending('save')}
            >
              <KeyRound aria-hidden="true" />
              保存新密码
            </Button>
            <Button
              type="button"
              variant="ghost"
              className="w-full"
              disabled={action.pending}
              loading={action.isPending('logout')}
              onClick={() => {
                void action.run('logout', async () => {
                  await logout();
                  await client.cancelQueries();
                  client.setQueryData(currentUserKey, null);
                });
              }}
            >
              <LogOut aria-hidden="true" />
              退出登录
            </Button>
          </form>
        </section>
      </main>
      <footer className="px-4 py-4 text-center text-xs text-muted-foreground">
        © {config.copyrightYear} {config.copyrightOwner}®
      </footer>
    </div>
  );
}
