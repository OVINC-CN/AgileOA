import { zodResolver } from '@hookform/resolvers/zod';
import { useQueryClient } from '@tanstack/react-query';
import { GraduationCap, KeyRound, LogIn, Mail, UserRound } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { Navigate, useLocation, useNavigate } from 'react-router';
import { z } from 'zod';

import { ErrorMessage } from '@/components/error-message';
import { Button } from '@/components/ui/button';
import { ButtonGroup } from '@/components/ui/button-group';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { agreementRequiredMessage } from '@/features/user/agreement-content';
import {
  type CaptchaProof,
  type CurrentUser,
  currentUserKey,
  issueLoginCaptcha,
  login,
  loginByEmail,
  sendLoginEmail,
  sendStudentLoginEmail,
  useCurrentUser,
  userEmailSchema,
} from '@/features/user/api';
import { solveLoginCaptcha } from '@/features/user/captcha';
import {
  EmailLoginVerification,
  type ReceivedEmailChallenge,
} from '@/features/user/email-login-verification';
import { LoginAgreements } from '@/features/user/login-agreements';
import {
  authURL,
  loginDestination,
  readLoginState,
  studentLoginPath,
} from '@/features/user/navigation';
import { useApiAction, useInitialLoading } from '@/lib/api-action';
import { getConfig } from '@/lib/config';
import { showMessage } from '@/lib/messages';

const passwordSchema = z
  .string()
  .min(1, '请输入密码')
  .refine((value) => new TextEncoder().encode(value).length <= 72, '密码不能超过 72 字节');
const emailSchema = z.string().trim().min(1, '请输入邮箱').pipe(userEmailSchema);
const schema = z
  .object({
    mode: z.enum(['password', 'email', 'student']),
    username: z.string().trim(),
    workspace_name: z.string().trim(),
    student_no: z.string().trim(),
    password: z.string(),
    email: z.string(),
    agreementsAccepted: z.boolean().refine((value) => value, agreementRequiredMessage),
  })
  .superRefine((values, context) => {
    if (values.mode === 'student') {
      for (const [field, label, max] of [
        ['workspace_name', '空间名称', 100],
        ['student_no', '学号', 32],
      ] as const) {
        const result = z
          .string()
          .min(1, `请输入${label}`)
          .max(max, `${label}最多输入 ${String(max)} 个字符`)
          .safeParse(values[field]);
        if (!result.success) {
          for (const issue of result.error.issues) {
            context.addIssue({ code: 'custom', path: [field], message: issue.message });
          }
        }
      }
    } else if (!/^[a-z0-9][a-z0-9._-]{2,63}$/.test(values.username)) {
      context.addIssue({
        code: 'custom',
        path: ['username'],
        message: '请输入 3 至 64 位小写英文、数字或 ._-',
      });
    }
    const field = values.mode !== 'password' ? 'email' : 'password';
    const result = (field === 'email' ? emailSchema : passwordSchema).safeParse(values[field]);
    if (!result.success) {
      for (const issue of result.error.issues) {
        context.addIssue({ code: 'custom', path: [field], message: issue.message });
      }
    }
  });
type Values = z.infer<typeof schema>;

export function LoginPage({ student = false }: { student?: boolean }) {
  const config = getConfig();
  const currentUser = useCurrentUser();
  const client = useQueryClient();
  const location = useLocation();
  const navigate = useNavigate();
  const { from, expired } = readLoginState(location.state, location.search);
  const {
    control,
    register,
    handleSubmit,
    setValue,
    clearErrors,
    getValues,
    formState: { errors },
  } = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      mode: student ? 'student' : 'email',
      username: '',
      workspace_name: new URLSearchParams(location.search).get('workspace_name') ?? '',
      student_no: '',
      password: '',
      email: '',
      agreementsAccepted: false,
    },
  });
  const [mode, username, workspaceName, studentNo, password, email, agreementsAccepted] = useWatch({
    control,
    name: [
      'mode',
      'username',
      'workspace_name',
      'student_no',
      'password',
      'email',
      'agreementsAccepted',
    ] as const,
  });
  const emailMode = mode !== 'password';
  const [emailChallenge, setEmailChallenge] = useState<ReceivedEmailChallenge | null>(null);
  const initialLoading = useInitialLoading(currentUser.isPending && !currentUser.isFetched);
  const { run, pending, isPending } = useApiAction();
  const captchaContainer = useRef<HTMLDivElement>(null);
  const captchaAttempt = useRef<AbortController | null>(null);
  const [captchaOpen, setCaptchaOpen] = useState(false);
  useEffect(
    () => () => {
      captchaAttempt.current?.abort();
      captchaAttempt.current = null;
    },
    [],
  );

  async function completeLogin(user: CurrentUser, attempt: AbortController) {
    await client.cancelQueries();
    if (captchaAttempt.current !== attempt) return;
    client.clear();
    client.setQueryData(currentUserKey, user);
    await navigate(loginDestination(user, from), { replace: true });
  }

  async function verifyEmail(code: string, username?: string) {
    if (!emailChallenge) return undefined;
    if (!getValues('agreementsAccepted')) {
      showMessage(agreementRequiredMessage);
      return undefined;
    }
    const result = await run('email-login', async () => {
      const attempt = new AbortController();
      captchaAttempt.current = attempt;
      try {
        const response = await loginByEmail(emailChallenge.challenge_id, code, username);
        if (captchaAttempt.current !== attempt) return undefined;
        if (!response.username_required) await completeLogin(response.user, attempt);
        return response.username_required;
      } finally {
        if (captchaAttempt.current === attempt) captchaAttempt.current = null;
      }
    });
    return result.ok ? result.data : undefined;
  }

  async function submit(values: Values) {
    if (!values.agreementsAccepted) {
      showMessage(agreementRequiredMessage);
      return;
    }
    await run(values.mode === 'password' ? 'login' : 'send-email', async () => {
      // 每次登录都重新获取加密串，取消或校验失败后重新发起验证。
      const attempt = new AbortController();
      captchaAttempt.current = attempt;
      try {
        const challenge = await issueLoginCaptcha();
        if (attempt.signal.aborted) return;

        let proof: CaptchaProof | undefined;
        if (challenge) {
          if (!captchaContainer.current) throw new Error('验证码加载失败，请重试');
          setCaptchaOpen(true);
          const result = await solveLoginCaptcha(
            captchaContainer.current,
            challenge,
            attempt.signal,
          );
          if (!result) return;
          proof = result;
          setCaptchaOpen(false);
        }

        if (values.mode !== 'password') {
          const challenge =
            values.mode === 'student'
              ? await sendStudentLoginEmail(values, proof)
              : await sendLoginEmail(values, proof);
          if (captchaAttempt.current !== attempt) return;
          setEmailChallenge({ ...challenge, receivedAt: Date.now() });
          return;
        }

        const user = await login(values, proof);
        if (captchaAttempt.current !== attempt) return;
        await completeLogin(user, attempt);
      } finally {
        if (captchaAttempt.current === attempt) {
          captchaAttempt.current = null;
          setCaptchaOpen(false);
        }
      }
    });
  }

  // 业务接口失去认证后必须重新登录，不能被其他接口残留的会话反复自动送回。
  if (currentUser.data && !pending && !expired) {
    return <Navigate to={loginDestination(currentUser.data, from)} replace />;
  }

  return (
    <div className="flex min-h-svh flex-col bg-background">
      <main className="mx-auto flex w-full max-w-sm flex-1 flex-col justify-center px-2 py-10 sm:px-6">
        <header className="mb-8 flex justify-center">
          <img src="/agile.svg" alt={config.appName} className="h-5 w-auto" />
        </header>
        <section
          aria-label={student ? '学生登录' : emailMode ? '邮件登录' : '登录'}
          className="mx-auto w-full max-w-xs"
        >
          <div className="mb-6 space-y-3">
            <ButtonGroup aria-label="登录身份" className="flex w-full [&>button]:flex-1">
              <Button
                type="button"
                variant="ghost"
                aria-pressed={!student}
                disabled={pending}
                onClick={() => {
                  if (student) void navigate(authURL('/login', from, expired));
                }}
              >
                <UserRound aria-hidden="true" />
                组织账号
              </Button>
              <Button
                type="button"
                variant="ghost"
                aria-pressed={student}
                disabled={pending}
                onClick={() => {
                  if (!student) void navigate(authURL(studentLoginPath, from, expired));
                }}
              >
                <GraduationCap aria-hidden="true" />
                学生账号
              </Button>
            </ButtonGroup>
          </div>
          {emailChallenge ? (
            <EmailLoginVerification
              key={emailChallenge.challenge_id}
              challenge={emailChallenge}
              email={email.trim()}
              requestedUsername={username.trim()}
              pending={pending}
              verifying={isPending('email-login')}
              resending={isPending('send-email')}
              onVerify={verifyEmail}
              onResend={() => {
                void submit(getValues());
              }}
              onBack={() => {
                setEmailChallenge(null);
                clearErrors();
              }}
            />
          ) : (
            <form
              noValidate
              onSubmit={(event) => {
                void handleSubmit(submit, (issues) => {
                  Object.entries(issues).forEach(([name, issue]) => {
                    if (issue.message) showMessage(issue.message, `${name}-error`);
                  });
                })(event);
              }}
            >
              {student ? (
                <>
                  <div className="mb-5 space-y-2">
                    <Label htmlFor="workspace-name">空间名称</Label>
                    <Input
                      id="workspace-name"
                      autoComplete="organization"
                      maxLength={100}
                      required
                      readOnly={pending}
                      className="h-10"
                      {...register('workspace_name')}
                      aria-invalid={Boolean(errors.workspace_name)}
                    />
                  </div>
                  <div className="mb-5 space-y-2">
                    <Label htmlFor="student-no">学号</Label>
                    <Input
                      id="student-no"
                      autoComplete="username"
                      maxLength={32}
                      required
                      readOnly={pending}
                      className="h-10"
                      {...register('student_no')}
                      aria-invalid={Boolean(errors.student_no)}
                    />
                  </div>
                </>
              ) : (
                <div className="mb-5 space-y-2">
                  <Label htmlFor="username">用户名</Label>
                  <Input
                    id="username"
                    autoComplete="username"
                    autoCapitalize="none"
                    spellCheck={false}
                    required
                    maxLength={64}
                    className="h-10"
                    readOnly={pending}
                    {...register('username')}
                    aria-invalid={Boolean(errors.username)}
                    aria-describedby={errors.username ? 'username-error' : undefined}
                  />
                  {errors.username ? (
                    <ErrorMessage id="username-error">{errors.username.message}</ErrorMessage>
                  ) : null}
                </div>
              )}
              {emailMode ? (
                <div className="space-y-2">
                  <Label htmlFor="email">邮箱</Label>
                  <Input
                    id="email"
                    type="email"
                    autoComplete="email"
                    autoCapitalize="none"
                    spellCheck={false}
                    required
                    maxLength={254}
                    className="h-10"
                    readOnly={pending}
                    {...register('email')}
                    aria-invalid={Boolean(errors.email)}
                  />
                </div>
              ) : (
                <div className="space-y-2">
                  <Label htmlFor="password">密码</Label>
                  <Input
                    id="password"
                    type="password"
                    autoComplete="current-password"
                    required
                    className="h-10"
                    {...register('password')}
                    readOnly={pending}
                    aria-invalid={Boolean(errors.password)}
                    aria-describedby={errors.password ? 'password-error' : undefined}
                  />
                  {errors.password ? (
                    <ErrorMessage id="password-error">{errors.password.message}</ErrorMessage>
                  ) : null}
                </div>
              )}
              <div className="mt-5">
                <LoginAgreements
                  checked={agreementsAccepted}
                  onCheckedChange={(checked) => {
                    setValue('agreementsAccepted', checked);
                  }}
                  disabled={pending}
                />
              </div>
              {!captchaOpen ? (
                <Button
                  type="submit"
                  className="mt-3 h-10 w-full"
                  disabled={
                    !agreementsAccepted ||
                    (student ? workspaceName === '' || studentNo === '' : username.trim() === '') ||
                    (emailMode ? email.trim() === '' : password === '')
                  }
                  loading={initialLoading || isPending('login') || isPending('send-email')}
                >
                  {emailMode ? <Mail aria-hidden="true" /> : <LogIn aria-hidden="true" />}
                  {emailMode ? '发送验证码' : '登录'}
                </Button>
              ) : null}
            </form>
          )}
          <div className={captchaOpen ? 'mt-5 space-y-2' : undefined}>
            {captchaOpen ? (
              <div className="flex items-center justify-between gap-3">
                <span className="text-sm font-medium">安全验证</span>
                <Button
                  type="button"
                  variant="cancel"
                  size="xs"
                  className="text-muted-foreground"
                  onClick={() => captchaAttempt.current?.abort()}
                >
                  取消验证
                </Button>
              </div>
            ) : null}
            {/* 勾选式验证码含边框至少需要 302px，容器使用表单的可用宽度。 */}
            <div ref={captchaContainer} className="w-full overflow-x-auto" />
          </div>
          {!emailChallenge && !student && config.passwordLoginEnabled ? (
            <Button
              type="button"
              variant="ghost"
              className="mt-3 h-9 w-full text-muted-foreground hover:bg-primary/5 hover:text-accent-foreground"
              disabled={pending}
              onClick={() => {
                setValue('mode', emailMode ? 'password' : 'email');
                setValue('password', '');
                clearErrors();
                setEmailChallenge(null);
              }}
            >
              {emailMode ? <KeyRound aria-hidden="true" /> : <Mail aria-hidden="true" />}
              {emailMode ? '切换到密码登录' : '切换到邮件登录'}
            </Button>
          ) : null}
        </section>
      </main>
      <footer className="px-4 py-4 text-center text-xs text-muted-foreground">
        © {config.copyrightYear} {config.copyrightOwner}®
      </footer>
    </div>
  );
}

export function StudentLoginPage() {
  return <LoginPage student />;
}
