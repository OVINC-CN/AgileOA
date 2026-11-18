import { z } from 'zod';

export const changePasswordPath = '/change-password';
export const emailLoginPath = '/login/email';
export const studentLoginPath = '/login/student';
export const studentServicePath = '/student-service';
export const studentProfilePath = `${studentServicePath}/profile`;
export const studentAwardsPath = `${studentServicePath}/projects`;

export function studentAwardsURL(workspaceID: string) {
  return `${studentAwardsPath}?${new URLSearchParams({ workspace: workspaceID }).toString()}`;
}

export function studentServiceURL(workspaceID: string) {
  return `${studentProfilePath}?${new URLSearchParams({ workspace: workspaceID }).toString()}`;
}

export function loginDestination(
  user: {
    identity_type: string;
    workspace_id: string;
    password_change_required: boolean;
  },
  from: string,
) {
  if (user.identity_type === 'student') return studentServiceURL(user.workspace_id);
  return user.password_change_required ? authURL(changePasswordPath, from) : from;
}

const loginState = z.object({ from: z.string(), expired: z.boolean().optional() });

function redirectPath(from: string) {
  // 只允许回到站内业务页面，避免再次进入认证流程。
  if (!/^\/(?!\/)/.test(from) || /[\\\t\n\r]/.test(from)) return '/';
  const { pathname } = new URL(from, window.location.origin);
  let decoded: string;
  try {
    decoded = decodeURIComponent(pathname);
  } catch {
    return '/';
  }
  if (!/^\/(?!\/)/.test(decoded) || /[\\\t\n\r]/.test(decoded)) return '/';
  const normalized = decoded.replace(/\/+$/, '').toLowerCase();
  return normalized === '/login' ||
    normalized.startsWith('/login/') ||
    normalized === changePasswordPath
    ? '/'
    : from;
}

export function readLoginState(value: unknown, search = '') {
  // URL 中保留回跳信息，兼容已有页面历史中的路由状态。
  const state = loginState.safeParse(value);
  const params = new URLSearchParams(search);
  const from = params.get('redirect') ?? (state.success ? state.data.from : '/');
  return {
    from: redirectPath(from),
    expired: params.get('expired') === '1' || (state.success && state.data.expired === true),
  };
}

export function authURL(
  path: '/login' | typeof changePasswordPath | typeof studentLoginPath,
  from: string,
  expired = false,
) {
  const params = new URLSearchParams({ redirect: redirectPath(from) });
  if (expired) params.set('expired', '1');
  return `${path}?${params.toString()}`;
}
