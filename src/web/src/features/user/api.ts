import { hashKey, type QueryClient, queryOptions, useQuery } from '@tanstack/react-query';
import { z } from 'zod';

import { clearWatermarkPreference } from '@/features/user/watermark';
import { ApiError, apiGet, apiPost } from '@/lib/api';

export const userEmailSchema = z
  .string()
  .trim()
  .max(254, '邮箱最多输入 254 个字符')
  .refine((value) => value === '' || z.email().safeParse(value).success, '请输入有效的邮箱地址');

export const userSchema = z
  .object({
    identity_type: z.enum(['user', 'student']).default('user'),
    workspace_id: z.string().default(''),
    workspace_name: z.string().default(''),
    id: z.number().int().positive(),
    username: z.string().min(1),
    display_name: z.string().min(1),
    email: userEmailSchema,
    is_admin: z.boolean(),
    password_login_enabled: z.boolean(),
    password_change_required: z.boolean(),
  })
  .superRefine((user, context) => {
    if (
      user.identity_type === 'student' &&
      (!/^(?=.*[a-z])(?=.*[0-9])[a-z0-9]{16}$/.test(user.workspace_id) ||
        !user.workspace_name ||
        user.is_admin ||
        user.password_login_enabled ||
        user.password_change_required)
    ) {
      context.addIssue({ code: 'custom', message: '学生登录信息无效' });
    }
  });

export type CurrentUser = z.infer<typeof userSchema>;

const emailLoginSchema = z.discriminatedUnion('username_required', [
  z.object({ username_required: z.literal(true), user: z.null() }),
  z.object({ username_required: z.literal(false), user: userSchema }),
]);

const emailLoginChallengeSchema = z.object({
  challenge_id: z.string().regex(/^[0-9a-f]{64}$/),
  expires_in: z.number().int().nonnegative(),
  retry_after: z.number().int().nonnegative(),
});

export type EmailLoginChallenge = z.infer<typeof emailLoginChallengeSchema>;

const loginCaptchaSchema = z.object({
  app_id: z.string().regex(/^[1-9][0-9]*$/),
  aid_encrypted: z.string().min(1),
  aid_encrypted_type: z.literal('gcm'),
});

export type LoginCaptcha = z.infer<typeof loginCaptchaSchema>;
export interface CaptchaProof {
  captcha_ticket: string;
  captcha_randstr: string;
}

export const currentUserKey = ['user', 'current'] as const;

export const currentUserOptions = queryOptions({
  queryKey: currentUserKey,
  queryFn: async ({ signal }) => {
    try {
      return await apiGet('/v1/user-info', userSchema, signal);
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) {
        return null;
      }
      throw error;
    }
  },
  staleTime: 0,
  retry: false,
  refetchOnWindowFocus: true,
});

export function subscribeCurrentUserUin(client: QueryClient, onChange: (uin: string) => void) {
  // 用查询确认登录身份，退出和会话失效时清空身份。
  const currentUserHash = hashKey(currentUserKey);
  return client.getQueryCache().subscribe((event) => {
    if (event.query.queryHash !== currentUserHash) return;
    if (event.type === 'removed') {
      onChange('');
      return;
    }
    if (event.type !== 'updated' || event.action.type !== 'success') return;

    const user = client.getQueryData(currentUserOptions.queryKey);
    if (user === null || (user && !event.action.manual)) {
      onChange(user ? `${user.identity_type}-${String(user.id)}` : '');
    }
  });
}

export function useCurrentUser() {
  return useQuery(currentUserOptions);
}

export async function passwordDigest(username: string, password: string) {
  // 只把用户名加盐后的摘要交给网络层，原始密码不进入请求体。
  if (!globalThis.isSecureContext) {
    throw new Error('当前连接不安全，请使用 HTTPS 地址登录');
  }
  const bytes = new TextEncoder().encode(`${username}:${password}`);
  const digest = await crypto.subtle.digest('SHA-256', bytes);
  return Array.from(new Uint8Array(digest), (value) => value.toString(16).padStart(2, '0')).join(
    '',
  );
}

export function issueLoginCaptcha() {
  return apiPost('/v1/login-captcha', loginCaptchaSchema.nullable());
}

export async function login(values: { username: string; password: string }, proof?: CaptchaProof) {
  const username = values.username.trim();
  const user = await apiPost('/v1/login', userSchema, {
    username,
    password_sha256: await passwordDigest(username, values.password),
    ...proof,
  });
  clearWatermarkPreference();
  return user;
}

export function sendLoginEmail(values: { username: string; email: string }, proof?: CaptchaProof) {
  return apiPost('/v1/login-email', emailLoginChallengeSchema, {
    username: values.username.trim(),
    email: values.email.trim(),
    ...proof,
  });
}

export function sendStudentLoginEmail(
  values: { workspace_name: string; student_no: string; email: string },
  proof?: CaptchaProof,
) {
  return apiPost('/v1/student-login-email', emailLoginChallengeSchema, {
    workspace_name: values.workspace_name.trim(),
    student_no: values.student_no.trim(),
    email: values.email.trim(),
    ...proof,
  });
}

export async function loginByEmail(challengeID: string, code: string, username?: string) {
  const result = await apiPost('/v1/login-email-verify', emailLoginSchema, {
    challenge_id: challengeID,
    code,
    ...(username === undefined ? {} : { username: username.trim() }),
  });
  if (!result.username_required) clearWatermarkPreference();
  return result;
}

export async function changePassword(
  username: string,
  currentPassword: string,
  newPassword: string,
) {
  const [current, next] = await Promise.all([
    passwordDigest(username, currentPassword),
    passwordDigest(username, newPassword),
  ]);
  const user = await apiPost('/v1/change-password', userSchema, {
    current_password_sha256: current,
    new_password_sha256: next,
  });
  clearWatermarkPreference();
  return user;
}

export async function logout() {
  const result = await apiPost('/v1/logout', z.null());
  clearWatermarkPreference();
  return result;
}

export async function createUser(values: {
  username: string;
  display_name: string;
  email: string;
  password_login_enabled: boolean;
  password: string;
}) {
  const username = values.username.trim();
  return apiPost('/v1/system/users', userSchema, {
    username,
    display_name: values.display_name.trim(),
    email: values.email.trim(),
    password_login_enabled: values.password_login_enabled,
    ...(values.password_login_enabled
      ? { password_sha256: await passwordDigest(username, values.password) }
      : {}),
  });
}
