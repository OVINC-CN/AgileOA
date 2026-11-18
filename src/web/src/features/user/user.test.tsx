import { webcrypto } from 'node:crypto';

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, RouterProvider } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { z } from 'zod';

import { AppLayout } from '@/components/app-layout';
import { SystemLayout } from '@/components/system-layout';
import type { AuditRow } from '@/features/management/api';
import type { StudentProfile } from '@/features/student-service/api';
import {
  createUser,
  currentUserKey,
  login,
  type LoginCaptcha,
  passwordDigest,
  userSchema,
} from '@/features/user/api';
import { AuthGuard } from '@/features/user/auth-guard';
import {
  changePasswordPath,
  studentLoginPath,
  studentServiceURL,
} from '@/features/user/navigation';
import { getConfig } from '@/lib/config';
import { AuditPage } from '@/pages/audit-page';
import { ChangePasswordPage } from '@/pages/change-password-page';
import { EmailLoginPage } from '@/pages/email-login-page';
import { LoginPage, StudentLoginPage } from '@/pages/login-page';
import { OverviewPage } from '@/pages/overview-page';
import { StudentProfilePage } from '@/pages/student-profile-page';
import { StudentServicePage } from '@/pages/student-service-page';
import { UsersPage } from '@/pages/users-page';

// 交互测试聚焦真实页面状态，图表在浏览器中验证尺寸和真实接口数据。
vi.mock('@/components/management-charts', () => ({
  UserCharts: () => <div aria-label="用户统计图表" />,
  AuditCharts: () => <div aria-label="审计统计图表" />,
}));
const trace = '4bf92f3577b34da6a3ce929d0e0e4736';
const account = {
  identity_type: 'user',
  workspace_id: '',
  workspace_name: '',
  id: 1,
  username: 'fixture.admin',
  display_name: '虚构管理员',
  email: 'fixture.admin@example.com',
  is_admin: true,
  password_login_enabled: true,
  password_change_required: false,
};
const studentAccount = {
  ...account,
  identity_type: 'student',
  workspace_id: 'w7m2k9p4s6n8c3x5',
  workspace_name: '学生事务空间',
  username: '00000001',
  display_name: '虚构学生',
  email: 'student@example.com',
  is_admin: false,
  password_login_enabled: false,
};
const studentProfile = {
  id: studentAccount.id,
  student_no: studentAccount.username,
  name: studentAccount.display_name,
  grade: '2026级',
  status: '在籍',
  education_level: '博士',
  major: '虚构专业',
  class_name: '2026级一班',
  class_position: '',
  admission_type: '非定向',
  arrival_month: '2026-09',
} satisfies StudentProfile;
const password = 'Fictional-password-2026';
const second = {
  id: 2,
  username: 'fixture.user',
  display_name: '虚构用户',
  email: 'fixture.user@example.com',
  is_admin: false,
  password_login_enabled: true,
  password_change_required: true,
  is_enabled: true,
  created_at: '2026-11-03T01:00:00Z',
  updated_at: '2026-11-03T01:00:00Z',
  last_login_at: null,
  session_count: 0,
  has_api_token: false,
};
const record: AuditRow = {
  id: '1',
  event_id: '12f8f7ba823d4a8eac12b5db99db71b4',
  event_date: '2026-11-03',
  workspace_id: 'system',
  event_content: '添加用户',
  action_id: 'user.create',
  request_id: '22f8f7ba823d4a8eac12b5db99db71b4',
  user_id: String(account.id),
  user_identify_type: 1,
  user_identify_tenant_id: 'system',
  start_time: Date.parse('2026-11-03T01:00:00Z'),
  end_time: Date.parse('2026-11-03T01:00:00.012Z'),
  access_type: 1,
  access_source_ip: '192.0.2.1',
  access_user_agent: 'Fixture',
  resource_type_id: 'user',
  instance_id: '2',
  instance_name: null,
  result_code: 0,
  result_content: '',
  snapshot_user_info: {
    id: account.id,
    username: account.username,
    display_name: account.display_name,
    is_admin: account.is_admin,
  },
};
const formBody = z.object({
  username: z.string().optional(),
  display_name: z.string(),
  email: z.string(),
  is_enabled: z.boolean().optional(),
  password_login_enabled: z.boolean(),
  password_sha256: z.string().optional(),
});
const runtimeConfig = getConfig();

afterEach(() => {
  vi.unstubAllGlobals();
  window.AGILE_WORKSPACE_CONFIG = runtimeConfig;
});
function respond(data: unknown, status = 200, message = '') {
  return new Response(JSON.stringify({ trace, error_msg: message, data }), { status });
}
function setup(
  entry = '/system/users',
  initialUser: typeof account | null = account,
  loginUser = account,
  challenge: LoginCaptcha | null = null,
  emailUsernameRequired = false,
  emailTiming: { expires_in?: number; retry_after?: number } = {},
) {
  vi.stubGlobal('isSecureContext', true);
  vi.stubGlobal('crypto', webcrypto);
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe = vi.fn();
      unobserve = vi.fn();
      disconnect = vi.fn();
    },
  );
  let currentUser = initialUser;
  let emailSequence = 0;
  let emailChallengeID = '';
  let apiToken = '';
  let apiTokenVersion = 0;
  let users = [
    {
      ...account,
      is_enabled: true,
      created_at: '2026-11-03T00:00:00Z',
      updated_at: '2026-11-03T00:00:00Z',
      last_login_at: '2026-11-03T01:00:00Z',
      session_count: 1,
      has_api_token: false,
    },
    second,
  ];
  const fetchMock = vi
    .fn<(url: string, options: RequestInit) => Promise<Response>>()
    .mockImplementation((url, options) => {
      const address = new URL(url, 'https://fixture.example');
      const path = address.pathname;
      if (path === '/v1/home/calendar') {
        return Promise.resolve(
          respond({ date: address.searchParams.get('date'), holidays: [], next: null }),
        );
      }
      if (
        path === '/v1/notifications/unread' ||
        path ===
          `/v1/workspaces/${studentAccount.workspace_id}/student-service/notifications/unread`
      ) {
        return Promise.resolve(respond({ unread_count: 0 }));
      }
      if (path === '/v1/user-info') {
        return Promise.resolve(currentUser ? respond(currentUser) : respond(null, 401, '请先登录'));
      }
      if (path === '/v1/login-captcha') {
        return Promise.resolve(respond(challenge));
      }
      if (path === '/v1/login') {
        currentUser = loginUser;
        return Promise.resolve(respond(currentUser));
      }
      if (path === '/v1/login-email-verify' && typeof options.body === 'string') {
        const body = z
          .object({
            challenge_id: z.string(),
            code: z.string(),
            username: z.string().optional(),
          })
          .parse(JSON.parse(options.body));
        if (body.challenge_id !== emailChallengeID || body.code !== '012345') {
          return Promise.resolve(respond(null, 401, '验证码错误、已过期或已失效，请重新获取'));
        }
        if (emailUsernameRequired && body.username === undefined) {
          return Promise.resolve(respond({ username_required: true, user: null }));
        }
        if (body.username !== undefined && body.username !== loginUser.username) {
          return Promise.resolve(respond(null, 401, '用户名不匹配，请重新输入'));
        }
        currentUser = loginUser;
        return Promise.resolve(respond({ username_required: false, user: currentUser }));
      }
      if (path === '/v1/login-email' || path === '/v1/student-login-email') {
        emailChallengeID = (++emailSequence).toString(16).padStart(64, '0');
        return Promise.resolve(
          respond({
            challenge_id: emailChallengeID,
            expires_in: 600,
            retry_after: 0,
            ...emailTiming,
          }),
        );
      }
      if (path === `/v1/workspaces/${studentAccount.workspace_id}/student-service`) {
        return Promise.resolve(respond(studentProfile));
      }
      if (path === '/v1/change-password' && options.method === 'POST') {
        currentUser = account;
        return Promise.resolve(respond(currentUser));
      }
      if (path === '/v1/logout') {
        currentUser = null;
        return Promise.resolve(respond(null));
      }
      if (path === '/v1/system/users/statistics') {
        return Promise.resolve(
          respond({
            total: users.length,
            enabled: users.filter((item) => item.is_enabled).length,
            disabled: users.filter((item) => !item.is_enabled).length,
            admins: 1,
            logins: 2,
            days: [{ date: '2026-11-03', created: users.length, logins: 2 }],
          }),
        );
      }
      if (path === '/v1/system/users' && options.method === 'GET') {
        const keyword = address.searchParams.get('keyword') ?? '';
        const status = address.searchParams.get('status') ?? '';
        const filtered = users.filter(
          (item) =>
            (item.username.includes(keyword) ||
              item.display_name.includes(keyword) ||
              item.email.includes(keyword)) &&
            (!status || item.is_enabled === (status === 'enabled')),
        );
        return Promise.resolve(
          respond({ items: filtered, total: filtered.length, page: 1, page_size: 5 }),
        );
      }
      if (/^\/v1\/system\/users\/\d+\/api-token$/.test(path)) {
        const id = Number(path.split('/').at(-2));
        if (options.method === 'PUT') {
          apiToken = `fixture-api-token-${String(++apiTokenVersion)}`;
        } else if (options.method === 'DELETE') {
          apiToken = '';
        }
        users = users.map((item) =>
          item.id === id ? { ...item, has_api_token: Boolean(apiToken) } : item,
        );
        return Promise.resolve(
          respond(
            options.method === 'DELETE'
              ? null
              : {
                  token: apiToken,
                  created_at: apiToken ? '2026-11-03T01:00:00Z' : null,
                  updated_at: apiToken ? '2026-11-03T01:00:00Z' : null,
                },
          ),
        );
      }
      if (
        path === '/v1/system/users' &&
        options.method === 'POST' &&
        typeof options.body === 'string'
      ) {
        const body = formBody.parse(JSON.parse(options.body));
        const user = {
          ...second,
          id: 3,
          username: body.username ?? '',
          display_name: body.display_name,
          email: body.email,
          password_login_enabled: body.password_login_enabled,
          password_change_required: body.password_login_enabled,
        };
        users = [...users, user];
        return Promise.resolve(respond(user));
      }
      if (
        path.startsWith('/v1/system/users/') &&
        options.method === 'PUT' &&
        typeof options.body === 'string'
      ) {
        const body = formBody.parse(JSON.parse(options.body));
        const id = Number(path.split('/').at(-1));
        users = users.map((item) =>
          item.id === id
            ? {
                ...item,
                display_name: body.display_name,
                email: body.email,
                is_enabled: body.is_enabled ?? item.is_enabled,
                password_login_enabled: body.password_login_enabled,
                password_change_required:
                  body.password_login_enabled &&
                  (Boolean(body.password_sha256) || item.password_change_required),
              }
            : item,
        );
        return Promise.resolve(respond(users.find((item) => item.id === id)));
      }
      if (path.startsWith('/v1/system/users/') && options.method === 'DELETE') {
        const id = Number(path.split('/').at(-1));
        users = users.filter((item) => item.id !== id);
        return Promise.resolve(respond(null));
      }
      if (path === '/v1/system/audit/logs') {
        const items = address.searchParams.get('result_code') === '403' ? [] : [record];
        return Promise.resolve(
          respond({
            items,
            total: items.length,
            page: 1,
            page_size: 5,
            actions: [{ value: record.action_id, label: record.event_content }],
            workspaces: [{ value: 'system', label: '系统' }],
            user_tenants: [{ value: 'system', label: '系统' }],
            result_codes: [
              { value: '0', label: '成功' },
              { value: '403', label: '403 无权限' },
            ],
            start_time: Date.parse('2026-10-21T00:00:00Z'),
            end_time: Date.parse('2026-11-04T00:00:00Z'),
          }),
        );
      }
      if (path === '/v1/system/audit/statistics') {
        return Promise.resolve(
          respond({
            total: 1,
            success: 1,
            failure: 0,
            denied: 0,
            days: [{ date: '2026-11-03', success: 1, failure: 0 }],
          }),
        );
      }
      if (path.startsWith('/v1/system/audit/logs/')) {
        return Promise.resolve(
          respond({
            ...record,
            instance_origin_data: null,
            instance_data: { display_name: '虚构用户' },
            has_sensitive_snapshot: false,
            extend_data: { method: 'POST', path: '/v1/system/users' },
          }),
        );
      }
      return Promise.resolve(respond(null, 404, '资源不存在'));
    });
  vi.stubGlobal('fetch', fetchMock);
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: 60_000 },
      mutations: { retry: false },
    },
  });
  const router = createMemoryRouter(
    [
      { path: '/login', Component: LoginPage },
      { path: studentLoginPath, Component: StudentLoginPage },
      { path: '/login/email', Component: EmailLoginPage },
      {
        element: <AuthGuard passwordChange />,
        children: [{ path: changePasswordPath, Component: ChangePasswordPage }],
      },
      {
        path: '/',
        element: <AuthGuard allowGuest />,
        children: [{ Component: AppLayout, children: [{ index: true, Component: OverviewPage }] }],
      },
      {
        Component: AuthGuard,
        children: [
          {
            Component: AppLayout,
            children: [
              {
                path: '/student-service',
                Component: StudentServicePage,
                children: [{ path: 'profile', Component: StudentProfilePage }],
              },
              { path: '/stu/roster', element: <h1>学生信息页面</h1> },
              {
                path: '/system',
                Component: SystemLayout,
                children: [
                  { path: 'users', Component: UsersPage },
                  { path: 'audit', Component: AuditPage },
                ],
              },
            ],
          },
        ],
      },
    ],
    { initialEntries: [entry] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  const user = userEvent.setup();
  const requestLoginEmail = async () => {
    // 在原登录页完成发送和验证码输入，不把校验标识或验证码放入 URL。
    if (router.state.location.pathname === studentLoginPath) {
      await user.type(screen.getByLabelText('空间名称'), studentAccount.workspace_name);
      await user.type(screen.getByLabelText('学号'), studentAccount.username);
    } else {
      await user.type(screen.getByLabelText('用户名'), loginUser.username);
    }
    await user.type(screen.getByLabelText('邮箱'), loginUser.email);
    await user.click(
      screen.getByRole('checkbox', { name: '我已阅读并同意《用户协议》和《隐私协议》' }),
    );
    const submit = screen.getByRole('button', { name: '发送验证码' });
    await waitFor(() => expect(submit).toBeEnabled());
    await user.click(submit);
    await screen.findByLabelText('第 1 位验证码');
  };
  const signInByEmail = async () => {
    await requestLoginEmail();
    await user.type(screen.getByLabelText('第 1 位验证码'), '012345');
    if (emailUsernameRequired) {
      await user.type(await screen.findByLabelText('用户名'), loginUser.username);
    }
  };
  return { client, router, fetchMock, user, requestLoginEmail, signInByEmail };
}

describe('登录与系统管理', () => {
  it('登录身份按钮组位于表单上方，学生表单只提交空间、学号和邮箱', async () => {
    const { user, fetchMock, router } = setup('/login', null);
    const group = screen.getByRole('group', { name: '登录身份' });
    expect(group).toHaveAttribute('data-slot', 'button-group');
    expect(
      group.compareDocumentPosition(screen.getByLabelText('用户名')) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(within(group).getByRole('button', { name: '组织账号' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    await user.click(within(group).getByRole('button', { name: '学生账号' }));
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(studentLoginPath);
    });
    expect(screen.queryByLabelText('用户名')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('密码')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '学生账号' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    await user.type(screen.getByLabelText('空间名称'), studentAccount.workspace_name);
    await user.type(screen.getByLabelText('学号'), studentAccount.username);
    await user.type(screen.getByLabelText('邮箱'), studentAccount.email);
    const submit = screen.getByRole('button', { name: '发送验证码' });
    expect(submit).toBeDisabled();
    await user.click(
      screen.getByRole('checkbox', { name: '我已阅读并同意《用户协议》和《隐私协议》' }),
    );
    await user.click(submit);
    expect(await screen.findByRole('status')).toHaveTextContent('登录验证码已发送至');
    const call = fetchMock.mock.calls.find(([url]) => url.endsWith('/v1/student-login-email'));
    expect(call).toBeDefined();
    const body = call?.[1].body;
    if (typeof body !== 'string') throw new Error('缺少学生登录请求体');
    expect(JSON.parse(body)).toEqual({
      workspace_name: studentAccount.workspace_name,
      student_no: '00000001',
      email: studentAccount.email,
    });
    expect(screen.queryByRole('button', { name: '切换到密码登录' })).not.toBeInTheDocument();
  });

  it('学生邮件登录绑定本人空间，仅显示学生服务并阻止进入管理页面', async () => {
    const { router, fetchMock, signInByEmail } = setup(studentLoginPath, null, studentAccount);
    await signInByEmail();
    expect(await screen.findByText('虚构学生', { selector: 'dd' })).toBeVisible();
    expect(router.state.location.pathname + router.state.location.search).toBe(
      studentServiceURL(studentAccount.workspace_id),
    );
    const navigation = screen.getByRole('navigation', { name: '主导航' });
    expect(within(navigation).getAllByRole('link')).toHaveLength(1);
    expect(within(navigation).getByRole('link', { name: '学生服务' })).toBeVisible();
    expect(screen.queryByText('学生工作')).not.toBeInTheDocument();
    expect(screen.queryByText('党建工作')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /切换工作空间/ })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '通知' })).toBeVisible();
    expect(
      fetchMock.mock.calls.some(([url]) =>
        url.includes(
          `/workspaces/${studentAccount.workspace_id}/student-service/notifications/unread`,
        ),
      ),
    ).toBe(true);
    expect(fetchMock.mock.calls.some(([url]) => url.endsWith('/v1/notifications/unread'))).toBe(
      false,
    );
    await act(() => router.navigate('/stu/roster?workspace=a1b2c3d4e5f6g7h8'));
    await waitFor(() => {
      expect(router.state.location.pathname + router.state.location.search).toBe(
        studentServiceURL(studentAccount.workspace_id),
      );
    });
    expect(screen.queryByText('学生信息页面')).not.toBeInTheDocument();
  });

  it('学生会话失效后返回学生登录页，保留重新登录标识', async () => {
    const { router } = setup(studentServiceURL(studentAccount.workspace_id), studentAccount);
    await screen.findByText('虚构学生', { selector: 'dd' });
    await act(async () => {
      window.dispatchEvent(new Event('auth:expired'));
      await Promise.resolve();
    });
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(studentLoginPath);
    });
    expect(new URLSearchParams(router.state.location.search).get('expired')).toBe('1');
    expect(screen.getByRole('button', { name: '学生账号' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
  });

  it('学生身份契约拒绝缺失绑定空间和管理权限', () => {
    expect(userSchema.safeParse(studentAccount).success).toBe(true);
    for (const invalid of [
      { ...studentAccount, workspace_id: '' },
      { ...studentAccount, workspace_name: '' },
      { ...studentAccount, is_admin: true },
      { ...studentAccount, password_login_enabled: true },
      { ...studentAccount, password_change_required: true },
    ]) {
      expect(userSchema.safeParse(invalid).success).toBe(false);
    }
  });

  it('个人信息仅展示学籍字段，不显示分组标题或折叠入口', async () => {
    const { user, router } = setup(
      '/student-service?workspace=' + studentAccount.workspace_id,
      studentAccount,
    );
    await screen.findByText('虚构学生', { selector: 'dd' });
    const sidebar = screen.getByRole('navigation', { name: '学生服务导航' });
    const [first] = within(sidebar).getAllByRole('link');
    expect(first).toHaveAccessibleName('个人信息');
    expect(first).toHaveAttribute('aria-current', 'page');
    const breadcrumb = screen.getByRole('navigation', { name: '面包屑' });
    expect(breadcrumb).toHaveTextContent('学生服务/个人信息');
    expect(screen.queryByText('学籍信息')).not.toBeInTheDocument();
    const profile = screen.getByRole('region', { name: '个人信息' });
    expect(within(profile).queryByRole('button', { expanded: true })).not.toBeInTheDocument();
    expect(within(profile).queryByRole('button', { expanded: false })).not.toBeInTheDocument();
    expect(within(profile).getAllByRole('term')).toHaveLength(10);
    for (const label of [
      '学号',
      '姓名',
      '年级',
      '状态',
      '学历',
      '专业',
      '班级',
      '班内任职',
      '录取类型',
      '来校时间',
    ]) {
      expect(screen.getByText(label, { selector: 'dt' })).toBeInTheDocument();
    }
    expect(screen.getByText(studentProfile.student_no, { selector: 'dd' })).toBeVisible();
    expect(within(profile).getByText(studentProfile.status)).toBeVisible();
    expect(screen.getByText('虚构专业', { selector: 'dd' })).toBeVisible();
    for (const heading of [
      '基本信息',
      '来校前经历',
      '导师信息',
      '联系方式',
      '住宿信息',
      '家庭联系',
      '毕业去向',
      '记录信息',
    ]) {
      expect(screen.queryByRole('heading', { name: heading })).not.toBeInTheDocument();
    }
    expect(screen.queryByText('创建时间')).not.toBeInTheDocument();
    expect(screen.queryByText('更新时间')).not.toBeInTheDocument();
    expect(screen.queryByText('出生日期')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('虚构学生暂无头像')).not.toBeInTheDocument();
    expect(screen.queryByText('敏感信息')).not.toBeInTheDocument();
    expect(screen.queryByText('身份证号码')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '显示原始值' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '编辑' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '折叠侧栏' }));
    expect(screen.getByRole('button', { name: '展开侧栏' })).toHaveAttribute(
      'aria-expanded',
      'false',
    );
    expect(within(sidebar).getByRole('link', { name: '个人信息' })).toBeVisible();
    await user.click(screen.getByRole('button', { name: '展开侧栏' }));
    expect(screen.getByRole('button', { name: '折叠侧栏' })).toHaveAttribute(
      'aria-expanded',
      'true',
    );
    expect(screen.getByText('虚构专业', { selector: 'dd' })).toBeVisible();
    await act(() => router.navigate(studentServiceURL(studentAccount.workspace_id) + '#details'));
    expect(router.state.location.hash).toBe('#details');
  });

  it('关闭密码登录时发送并输入验证码，重发清空旧验证码并保留回跳目标', async () => {
    window.AGILE_WORKSPACE_CONFIG = { ...runtimeConfig, passwordLoginEnabled: false };
    const destination = '/stu/roster?workspace=fixture#details';
    const params = new URLSearchParams({ redirect: destination });
    const { user, fetchMock, router, requestLoginEmail } = setup(`/login?${params}`, null);
    expect(screen.queryByLabelText('密码')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '切换到密码登录' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '发送验证码' })).toBeDisabled();
    await requestLoginEmail();
    expect(await screen.findByRole('status')).toHaveTextContent('登录验证码已发送至');
    expect(router.state.location.pathname).toBe('/login');
    const call = fetchMock.mock.calls.find(([url]) => url === '/v1/login-email');
    const body = call?.[1].body;
    if (typeof body !== 'string') throw new Error('缺少登录邮件请求体');
    expect(JSON.parse(body)).toEqual({ username: account.username, email: account.email });
    expect(new URLSearchParams(router.state.location.search).get('redirect')).toBe(destination);
    expect(screen.queryByLabelText('用户名')).not.toBeInTheDocument();
    const code = screen.getByLabelText('第 1 位验证码');
    expect(code).toHaveAttribute('inputmode', 'numeric');
    expect(code).toHaveAttribute('autocomplete', 'one-time-code');
    expect(code).toHaveAttribute('type', 'text');
    await user.type(code, '123');
    expect(screen.queryByRole('button', { name: '登录' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '重新发送验证码' }));
    await waitFor(() => expect(screen.getByLabelText('第 1 位验证码')).toHaveValue(''));
    expect(fetchMock.mock.calls.filter(([url]) => url === '/v1/login-email')).toHaveLength(2);
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
    expect(fetchMock.mock.calls.some(([url]) => url === '/v1/login-email-verify')).toBe(false);
    await user.click(screen.getByRole('button', { name: '修改登录信息' }));
    expect(screen.getByLabelText('用户名')).toHaveValue(account.username);
    expect(screen.getByLabelText('邮箱')).toHaveValue(account.email);
  });

  it('开启密码登录后可切换登录方式，验证协议并在登录成功后回跳', async () => {
    const destination = '/system/users?filter=all#list';
    const { user, fetchMock, router, client } = setup(
      `/login?${new URLSearchParams({ redirect: destination })}`,
      null,
    );
    await user.type(screen.getByLabelText('用户名'), account.username);
    await user.click(screen.getByRole('button', { name: '切换到密码登录' }));
    expect(screen.queryByLabelText('邮箱')).not.toBeInTheDocument();
    await user.type(screen.getByLabelText('密码'), password);
    expect(screen.getByRole('button', { name: '登录' })).toBeDisabled();
    await user.click(screen.getByRole('button', { name: '切换到邮件登录' }));
    expect(screen.getByLabelText('用户名')).toHaveValue(account.username);
    await user.click(screen.getByRole('button', { name: '切换到密码登录' }));
    expect(screen.getByLabelText('密码')).toHaveValue('');
    await user.type(screen.getByLabelText('密码'), password);
    await user.click(
      screen.getByRole('checkbox', { name: '我已阅读并同意《用户协议》和《隐私协议》' }),
    );
    client.setQueryData(['private', 'previous-account'], ['旧数据']);
    await user.click(screen.getByRole('button', { name: '登录' }));
    await screen.findByRole('row', { name: /fixture.user/ });
    expect(router.state.location).toMatchObject({
      pathname: '/system/users',
      search: '?filter=all',
      hash: '#list',
    });
    expect(client.getQueryData(['private', 'previous-account'])).toBeUndefined();
    const request = fetchMock.mock.calls.find(([url]) => url === '/v1/login');
    expect(request?.[1].body).toBe(
      JSON.stringify({
        username: account.username,
        password_sha256: await passwordDigest(account.username, password),
      }),
    );
    expect(fetchMock.mock.calls.some(([url]) => url === '/v1/login-email')).toBe(false);
  });

  it('填满六格自动登录，仅提交一次并清理旧缓存和回跳原页面', async () => {
    const destination = '/stu/roster?workspace=fixture#details';
    const { router, client, fetchMock, user, requestLoginEmail } = setup(
      `/login?${new URLSearchParams({ redirect: destination })}`,
      null,
    );
    client.setQueryData(['private', 'previous-account'], ['旧数据']);
    await requestLoginEmail();
    expect(client.getQueryData(['private', 'previous-account'])).toEqual(['旧数据']);
    expect(fetchMock.mock.calls.some(([url]) => url === '/v1/login-email-verify')).toBe(false);
    await user.type(screen.getByLabelText('第 1 位验证码'), '012345');
    expect(await screen.findByRole('heading', { name: '学生信息页面' })).toBeVisible();
    expect(router.state.location).toMatchObject({
      pathname: '/stu/roster',
      search: '?workspace=fixture',
      hash: '#details',
    });
    const requests = fetchMock.mock.calls.filter(([url]) => url === '/v1/login-email-verify');
    expect(requests).toHaveLength(1);
    const body = requests[0]?.[1].body;
    if (typeof body !== 'string') throw new Error('缺少邮件登录请求体');
    expect(JSON.parse(body)).toEqual({ challenge_id: '1'.padStart(64, '0'), code: '012345' });
    expect(client.getQueryData(['private', 'previous-account'])).toBeUndefined();
    expect(client.getQueryData(currentUserKey)).toEqual(account);
    expect(router.state.location.search).not.toContain('code=');
  });

  it('邮件验证码登录仍然要求初次改密，保留原页面目标', async () => {
    const destination = '/stu/roster?workspace=fixture#details';
    const { router, signInByEmail } = setup(
      `/login?${new URLSearchParams({ redirect: destination })}`,
      null,
      { ...account, password_change_required: true },
    );
    await signInByEmail();
    expect(await screen.findByRole('heading', { name: '修改密码' })).toBeVisible();
    expect(new URLSearchParams(router.state.location.search).get('redirect')).toBe(destination);
  });

  it('验证码页不重复展示协议、登录按钮、密码切换或身份说明', async () => {
    const { requestLoginEmail, user } = setup('/login', null);
    expect(screen.queryByText('教职工 / 党支部书记登录')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '切换到密码登录' })).toBeVisible();
    await requestLoginEmail();
    const group = screen.getByRole('group', { name: '邮箱验证码' });
    expect(within(group).getAllByRole('textbox')).toHaveLength(6);
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '《用户协议》' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '《隐私协议》' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '登录' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '切换到密码登录' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '修改登录信息' }));
    expect(screen.getByRole('checkbox')).toBeChecked();
    expect(screen.getByRole('button', { name: '切换到密码登录' })).toBeVisible();
  });

  it('首次核验用户名不自动回填，手动填写匹配后自动继续登录', async () => {
    const destination = '/stu/roster?workspace=fixture#details';
    const { router, client, fetchMock, user, requestLoginEmail } = setup(
      `/login?${new URLSearchParams({ redirect: destination })}`,
      null,
      account,
      null,
      true,
    );
    client.setQueryData(['private', 'previous-account'], ['旧数据']);
    await requestLoginEmail();
    await user.type(screen.getByLabelText('第 1 位验证码'), '012345');
    const username = await screen.findByLabelText('用户名');
    expect(username).toHaveValue('');
    expect(username).toHaveFocus();
    expect(screen.queryByRole('button', { name: '登录' })).not.toBeInTheDocument();
    await user.type(username, 'wrong.fixture');
    await user.keyboard('{Enter}');
    await screen.findByText('用户名不匹配，请重新输入');
    await waitFor(() => expect(username).not.toHaveAttribute('readonly'));
    expect(router.state.location.pathname).toBe('/login');
    const fields = within(
      screen.getByRole('group', { name: '邮箱验证码' }),
    ).getAllByRole<HTMLInputElement>('textbox');
    expect(fields.every((field) => field.value === '')).toBe(true);
    expect(fields[0]).toHaveFocus();
    expect(username).toHaveValue('');
    expect(client.getQueryData(['private', 'previous-account'])).toEqual(['旧数据']);
    await user.type(screen.getByLabelText('第 1 位验证码'), '012345');
    await user.type(username, account.username);
    expect(await screen.findByRole('heading', { name: '学生信息页面' })).toBeVisible();
    const requests = fetchMock.mock.calls
      .filter(([url]) => url === '/v1/login-email-verify')
      .map(([, options]) => {
        if (typeof options.body !== 'string') throw new Error('缺少邮件登录请求体');
        return JSON.parse(options.body) as unknown;
      });
    const proof = { challenge_id: '1'.padStart(64, '0'), code: '012345' };
    expect(requests).toEqual([
      proof,
      { ...proof, username: 'wrong.fixture' },
      { ...proof, username: account.username },
    ]);
    expect(client.getQueryData(['private', 'previous-account'])).toBeUndefined();
  });

  it('验证码错误后清空并聚焦首格，重新填写后自动重试', async () => {
    const { user, fetchMock, requestLoginEmail } = setup('/login', null);
    await requestLoginEmail();
    await user.type(screen.getByLabelText('第 1 位验证码'), '123456');
    await screen.findByText('验证码错误、已过期或已失效，请重新获取');
    await waitFor(() => expect(screen.getByLabelText('第 1 位验证码')).toHaveValue(''));
    const fields = within(screen.getByRole('group', { name: '邮箱验证码' })).getAllByRole(
      'textbox',
    );
    fields.forEach((field) => expect(field).toHaveValue(''));
    expect(fields[0]).toHaveFocus();
    expect(fetchMock.mock.calls.filter(([url]) => url === '/v1/login-email-verify')).toHaveLength(
      1,
    );
    await user.type(screen.getByLabelText('第 1 位验证码'), '012345');
    expect(await screen.findByRole('group', { name: '当前日期' })).toBeVisible();
    expect(fetchMock.mock.calls.filter(([url]) => url === '/v1/login-email-verify')).toHaveLength(
      2,
    );
  });

  it('重发倒计时取服务端间隔，页面重新可见时更新并在重发后清空旧验证码', async () => {
    const { user, requestLoginEmail, fetchMock } = setup('/login', null, account, null, false, {
      retry_after: 60,
    });
    await requestLoginEmail();
    expect(screen.getByRole('button', { name: /重新发送验证码/ })).toBeDisabled();
    await user.type(screen.getByLabelText('第 1 位验证码'), '012');
    const clock = vi.spyOn(Date, 'now').mockReturnValue(Date.now() + 61000);
    try {
      act(() => {
        document.dispatchEvent(new Event('visibilitychange'));
      });
      await user.click(screen.getByRole('button', { name: '重新发送验证码' }));
      await waitFor(() => expect(screen.getByLabelText('第 1 位验证码')).toHaveValue(''));
      expect(fetchMock.mock.calls.filter(([url]) => url === '/v1/login-email')).toHaveLength(2);
      expect(screen.getByRole('button', { name: /重新发送验证码/ })).toBeDisabled();
    } finally {
      clock.mockRestore();
    }
  });

  it('验证码过期时禁止登录并允许重新获取', async () => {
    const { user, fetchMock, requestLoginEmail } = setup('/login', null, account, null, false, {
      expires_in: 0,
    });
    await requestLoginEmail();
    await user.type(screen.getByLabelText('第 1 位验证码'), '012345');
    expect(screen.getByText('验证码已过期，请重新获取。')).toBeVisible();
    expect(screen.getByLabelText('第 1 位验证码')).toHaveAttribute('readonly');
    expect(screen.queryByRole('button', { name: '登录' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '重新发送验证码' })).toBeEnabled();
    expect(fetchMock.mock.calls.some(([url]) => url === '/v1/login-email-verify')).toBe(false);
  });

  it.each([
    '/login/email',
    '/login/email?code=',
    '/login/email?code=invalid',
    `/login/email?code=${'a'.repeat(64)}`,
  ])('旧邮件入口 %s 返回验证码登录页并清除凭据，不自动登录', async (entry) => {
    const { router, fetchMock } = setup(entry, null);
    await screen.findByRole('button', { name: '发送验证码' });
    expect(router.state.location.pathname).toBe('/login');
    expect(new URLSearchParams(router.state.location.search).has('code')).toBe(false);
    expect(fetchMock.mock.calls.some(([url]) => url === '/v1/login-email-verify')).toBe(false);
  });

  it('内嵌验证码可取消和重试，验证通过才发送登录邮件和票据', async () => {
    const challenge = {
      app_id: '199999164',
      aid_encrypted: 'encrypted-fixture',
      aid_encrypted_type: 'gcm',
    } satisfies LoginCaptcha;
    type CaptchaArguments = ConstructorParameters<NonNullable<Window['TencentCaptcha']>>;
    const initialize = vi.fn<(...args: CaptchaArguments) => void>();
    const destroy = vi.fn();
    vi.stubGlobal(
      'TencentCaptcha',
      class {
        constructor(...args: CaptchaArguments) {
          initialize(...args);
        }
        show = vi.fn();
        destroy = destroy;
      },
    );
    const { user, fetchMock, router } = setup('/login', null, account, challenge);
    const requestsTo = (path: string) =>
      fetchMock.mock.calls.filter(
        ([url]) => new URL(url, 'https://fixture.example').pathname === path,
      );

    const submit = screen.getByRole('button', { name: '发送验证码' });
    const usernameField = screen.getByLabelText('用户名');
    expect(submit).toBeDisabled();
    await user.type(usernameField, account.username);
    expect(submit).toBeDisabled();
    await user.type(screen.getByLabelText('邮箱'), account.email);
    const agreements = screen.getByRole('checkbox', {
      name: '我已阅读并同意《用户协议》和《隐私协议》',
    });
    expect(agreements).not.toBeChecked();
    expect(submit).toBeDisabled();
    const form = submit.closest('form');
    if (!form) throw new Error('缺少登录表单');
    fireEvent.submit(form);
    expect(await screen.findByText('请先阅读并同意用户协议和隐私协议')).toBeVisible();
    expect(requestsTo('/v1/login-captcha')).toHaveLength(0);
    expect(requestsTo('/v1/login-email')).toHaveLength(0);
    await user.click(screen.getByRole('button', { name: '《用户协议》' }));
    const userAgreement = await screen.findByRole('dialog');
    expect(within(userAgreement).getByRole('heading', { name: '用户协议' })).toBeVisible();
    expect(within(userAgreement).getByText(/欢迎使用敏捷办公/)).toBeVisible();
    expect(within(userAgreement).getAllByText(/privacy@example\.test/u)).toHaveLength(2);
    await user.click(within(userAgreement).getByRole('button', { name: '关闭' }));
    expect(agreements).not.toBeChecked();
    expect(usernameField).toHaveValue(account.username);
    expect(screen.getByLabelText('邮箱')).toHaveValue(account.email);
    await user.click(screen.getByRole('button', { name: '《隐私协议》' }));
    const privacyAgreement = await screen.findByRole('dialog');
    expect(within(privacyAgreement).getByRole('heading', { name: '隐私协议' })).toBeVisible();
    expect(within(privacyAgreement).getByText(/敏捷办公重视您的隐私/)).toBeVisible();
    expect(within(privacyAgreement).getAllByText(/privacy@example\.test/u)).toHaveLength(2);
    await user.keyboard('{Escape}');
    expect(agreements).not.toBeChecked();
    expect(submit).toBeDisabled();
    await user.click(agreements);
    await waitFor(() => expect(submit).toBeEnabled());
    await user.click(agreements);
    expect(submit).toBeDisabled();
    await user.click(agreements);
    await user.clear(usernameField);
    expect(submit).toBeDisabled();
    await user.type(usernameField, account.username);
    await user.click(submit);
    await waitFor(() => {
      expect(initialize).toHaveBeenCalledTimes(1);
    });
    expect(initialize.mock.lastCall?.[3]).toMatchObject({
      type: 'embed',
      aidEncrypted: challenge.aid_encrypted,
      aidEncryptedType: 'gcm',
    });
    expect(screen.getByText('安全验证')).toBeVisible();
    expect(screen.queryByRole('button', { name: '发送验证码' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '取消验证' }));
    expect(destroy).toHaveBeenCalledTimes(1);
    expect(requestsTo('/v1/login-email')).toHaveLength(0);

    const retry = await screen.findByRole('button', { name: '发送验证码' });
    await waitFor(() => expect(retry).toBeEnabled());
    await user.click(retry);
    await waitFor(() => {
      expect(initialize).toHaveBeenCalledTimes(2);
    });
    expect(requestsTo('/v1/login-captcha')).toHaveLength(2);
    expect(requestsTo('/v1/login-email')).toHaveLength(0);
    act(() =>
      initialize.mock.lastCall?.[2]({ ret: 0, errorCode: 1006, ticket: 'trerror_fixture' }),
    );
    expect(await screen.findByText('验证码服务暂时不可用（错误码 1006），请重试')).toBeVisible();
    expect(requestsTo('/v1/login-email')).toHaveLength(0);

    const retryAfterError = await screen.findByRole('button', { name: '发送验证码' });
    await waitFor(() => expect(retryAfterError).toBeEnabled());
    await user.click(retryAfterError);
    await waitFor(() => {
      expect(initialize).toHaveBeenCalledTimes(3);
    });
    act(() =>
      initialize.mock.lastCall?.[2]({ ret: 0, ticket: 'ticket-fixture', randstr: '@fixture' }),
    );
    await waitFor(() => {
      expect(requestsTo('/v1/login-email')).toHaveLength(1);
    });
    const body = requestsTo('/v1/login-email')[0]?.[1].body;
    expect(body).toContain('"captcha_ticket":"ticket-fixture"');
    expect(body).toContain('"captcha_randstr":"@fixture"');
    expect(body).not.toContain(password);
    expect(await screen.findByRole('status')).toHaveTextContent('登录验证码已发送至');
    expect(router.state.location.pathname).toBe('/login');
    expect(requestsTo('/v1/login')).toHaveLength(0);
  });

  it('登录和添加用户只发送用户名加盐后的 SHA-256 摘要', async () => {
    vi.stubGlobal('isSecureContext', true);
    vi.stubGlobal('crypto', webcrypto);
    vi.stubGlobal(
      'ResizeObserver',
      class {
        observe = vi.fn();
        unobserve = vi.fn();
        disconnect = vi.fn();
      },
    );
    const fetchMock = vi.fn().mockResolvedValue(respond(account));
    vi.stubGlobal('fetch', fetchMock);
    const expected = await passwordDigest(account.username, password);
    expect(expected).toBe('b157afc1eeae5c340d93228a2d2d8a5d1b31250496946b72205fb9d71224bff4');
    await login({ username: account.username, password });
    fetchMock.mockResolvedValue(respond(account));
    await createUser({ ...account, password });
    for (const call of fetchMock.mock.calls) {
      const options = call[1] as RequestInit;
      expect(options.body).toContain(expected);
      expect(options.body).not.toContain(password);
      expect(options.credentials).toBe('include');
    }
  });
  it('禁止密码登录的新用户不处理或发送密码', async () => {
    vi.stubGlobal('isSecureContext', false);
    const fetchMock = vi
      .fn()
      .mockResolvedValue(respond({ ...account, password_login_enabled: false }));
    vi.stubGlobal('fetch', fetchMock);
    await createUser({ ...account, password_login_enabled: false, password });
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const options = fetchMock.mock.calls[0]?.[1] as RequestInit;
    if (typeof options.body !== 'string') throw new Error('缺少创建用户的请求体');
    expect(JSON.parse(options.body)).toEqual({
      username: account.username,
      display_name: account.display_name,
      email: account.email,
      password_login_enabled: false,
    });
  });
  it('登录后回跳系统管理，顶栏只显示名称，下拉显示用户名并退出清理缓存', async () => {
    const destination = '/system/audit?result=success#records';
    const { user, client, router, signInByEmail } = setup(destination, null);
    expect(await screen.findByRole('region', { name: '邮件登录' })).toBeVisible();
    expect(new URLSearchParams(router.state.location.search).get('redirect')).toBe(destination);
    // 模拟重新打开登录地址，不依赖原有页面历史中的路由状态。
    await act(async () => {
      await router.navigate(`/login${router.state.location.search}`, {
        replace: true,
        state: null,
      });
    });
    await signInByEmail();
    expect(await screen.findByRole('heading', { name: '审计日志', level: 1 })).toBeVisible();
    expect(router.state.location).toMatchObject({
      pathname: '/system/audit',
      search: '?result=success',
      hash: '#records',
    });
    const trigger = screen.getByRole('button', { name: '当前用户 虚构管理员' });
    expect(trigger).not.toHaveTextContent(account.username);
    client.setQueryData(['private', 'records'], ['待清理']);
    await user.click(trigger);
    const menu = await screen.findByRole('menu');
    expect(within(menu).getByText(account.username)).toBeVisible();
    await user.click(within(menu).getByRole('menuitem', { name: '退出登录' }));
    expect(await screen.findByRole('region', { name: '邮件登录' })).toBeVisible();
    expect(client.getQueryData(['private', 'records'])).toBeUndefined();
    expect(client.getQueryData(currentUserKey)).toBeNull();
  });
  it.each([false, true])(
    '登录页按 URL 回跳工作空间并保留查询参数和锚点，已有会话=%s',
    async (signedIn) => {
      const search = '?workspace=w7m2k9p4s6n8c3x5&student_id=12&keyword=%E5%AD%A6%E7%94%9F';
      const destination = `/stu/roster${search}#details`;
      const params = new URLSearchParams({ redirect: destination });
      const { router, signInByEmail } = setup(
        `/login?${params.toString()}`,
        signedIn ? account : null,
      );
      if (!signedIn) {
        await screen.findByRole('region', { name: '邮件登录' });
        await signInByEmail();
      }
      expect(await screen.findByRole('heading', { name: '学生信息页面' })).toBeVisible();
      expect(router.state.location).toMatchObject({
        pathname: '/stu/roster',
        search,
        hash: '#details',
      });
    },
  );
  it('系统管理包含真实用户列表，没有组件示例入口；新增使用弹窗并刷新列表', async () => {
    const { user, fetchMock } = setup();
    expect(await screen.findByText(second.username)).toBeVisible();
    expect(screen.queryByRole('link', { name: '组件示例' })).not.toBeInTheDocument();
    expect(screen.queryByRole('navigation', { name: '主导航' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '当前用户 虚构管理员' }));
    expect(await screen.findByRole('menuitem', { name: '系统管理' })).toBeVisible();
    await user.keyboard('{Escape}');
    await user.click(screen.getByRole('button', { name: '添加用户' }));
    const dialog = await screen.findByRole('dialog');
    const passwordEnabled = within(dialog).getByRole('checkbox', { name: '允许密码登录' });
    expect(passwordEnabled).toBeChecked();
    expect(passwordEnabled).toBeEnabled();
    await user.type(within(dialog).getByLabelText('用户名'), 'new.fixture');
    await user.type(within(dialog).getByLabelText('显示名称'), '新增用户');
    await user.type(within(dialog).getByLabelText('邮箱（可选）'), 'new.fixture@example.com');
    expect(within(dialog).queryByLabelText('初始密码')).not.toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: '添加用户' }));
    expect(await screen.findByRole('heading', { name: '用户已创建' })).toBeVisible();
    const credentials = screen.getByRole('dialog');
    const initialPassword = within(credentials).getByLabelText<HTMLInputElement>('初始密码').value;
    expect(initialPassword).toHaveLength(16);
    expect(within(credentials).getByText(/用户首次登录时须修改密码/)).toBeVisible();
    await user.click(within(credentials).getByRole('button', { name: '关闭抽屉' }));
    expect(await screen.findByText('已添加用户 new.fixture')).toBeVisible();
    expect(await screen.findByText('new.fixture')).toBeVisible();
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    const call = fetchMock.mock.calls.find(
      ([url, options]) => url === '/v1/system/users' && options.method === 'POST',
    );
    const body = call?.[1].body;
    if (typeof body !== 'string') {
      throw new Error('缺少创建用户的请求体');
    }
    expect(body).not.toContain(password);
    expect(body).not.toContain(initialPassword);
    expect(JSON.parse(body)).toEqual({
      username: 'new.fixture',
      display_name: '新增用户',
      email: 'new.fixture@example.com',
      password_login_enabled: true,
      password_sha256: await passwordDigest('new.fixture', initialPassword),
    });
  });
  it('新增可关闭密码登录，编辑重新启用时必须设置临时密码', async () => {
    const { user, fetchMock } = setup();
    await user.click(await screen.findByRole('button', { name: '添加用户' }));
    const dialog = screen.getByRole('dialog');
    await user.type(within(dialog).getByLabelText('用户名'), 'mail.fixture');
    await user.type(within(dialog).getByLabelText('显示名称'), '邮件用户');
    await user.type(within(dialog).getByLabelText('邮箱（可选）'), 'mail@example.com');
    const createCheckbox = within(dialog).getByRole('checkbox', { name: '允许密码登录' });
    expect(createCheckbox).toBeChecked();
    expect(createCheckbox).toBeEnabled();
    await user.click(createCheckbox);
    expect(createCheckbox).not.toBeChecked();
    await user.click(within(dialog).getByRole('button', { name: '添加用户' }));
    await screen.findByRole('heading', { name: '用户已创建' });
    const credentials = screen.getByRole('dialog');
    expect(within(credentials).queryByLabelText('初始密码')).not.toBeInTheDocument();
    expect(within(credentials).getByText(/无需修改密码/)).toBeVisible();
    const copy = vi.spyOn(navigator.clipboard, 'writeText');
    await user.click(within(credentials).getByRole('button', { name: '复制登录信息' }));
    expect(copy).toHaveBeenCalledWith('用户名：mail.fixture\n登录方式：邮件登录');
    const creation = fetchMock.mock.calls.find(
      ([url, options]) => url === '/v1/system/users' && options.method === 'POST',
    );
    const creationBody = creation?.[1].body;
    if (typeof creationBody !== 'string') throw new Error('缺少创建用户的请求体');
    expect(JSON.parse(creationBody)).toEqual({
      username: 'mail.fixture',
      display_name: '邮件用户',
      email: 'mail@example.com',
      password_login_enabled: false,
    });
    await user.click(within(credentials).getByRole('button', { name: '关闭抽屉' }));
    const row = await screen.findByRole('row', { name: /mail.fixture/ });
    expect(within(row).getByText('禁止')).toBeVisible();
    await user.click(within(row).getByRole('button', { name: '编辑' }));
    const editor = screen.getByRole('dialog');
    const editCheckbox = within(editor).getByRole('checkbox', { name: '允许密码登录' });
    expect(editCheckbox).not.toBeChecked();
    expect(editCheckbox).toBeEnabled();
    expect(within(editor).queryByLabelText(/临时密码/)).not.toBeInTheDocument();
    await user.click(editCheckbox);
    expect(editCheckbox).toBeChecked();
    await user.click(within(editor).getByRole('button', { name: '保存更改' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('启用密码登录时请设置临时密码');
    expect(fetchMock.mock.calls.some(([, options]) => options.method === 'PUT')).toBe(false);
    await user.type(within(editor).getByLabelText('临时密码'), password);
    await user.click(within(editor).getByRole('button', { name: '保存更改' }));
    await screen.findByText('已更新用户 mail.fixture');
    const update = fetchMock.mock.calls.find(([, options]) => options.method === 'PUT');
    const updateBody = update?.[1].body;
    if (typeof updateBody !== 'string') throw new Error('缺少更新用户的请求体');
    expect(JSON.parse(updateBody)).toEqual({
      display_name: '邮件用户',
      email: 'mail@example.com',
      is_enabled: true,
      password_login_enabled: true,
      password_sha256: await passwordDigest('mail.fixture', password),
    });
  });
  it('关闭全局开关后新用户仅邮件登录，编辑保留账号已有的密码设置', async () => {
    window.AGILE_WORKSPACE_CONFIG = { ...runtimeConfig, passwordLoginEnabled: false };
    const { user, fetchMock } = setup();
    await user.click(await screen.findByRole('button', { name: '添加用户' }));
    const creator = screen.getByRole('dialog');
    expect(within(creator).queryByLabelText('允许密码登录')).not.toBeInTheDocument();
    await user.type(within(creator).getByLabelText('用户名'), 'mail.fixture');
    await user.type(within(creator).getByLabelText('显示名称'), '邮件用户');
    await user.click(within(creator).getByRole('button', { name: '添加用户' }));
    await screen.findByRole('heading', { name: '用户已创建' });
    const credentials = screen.getByRole('dialog');
    expect(within(credentials).queryByLabelText('初始密码')).not.toBeInTheDocument();
    const creation = fetchMock.mock.calls.find(
      ([url, options]) => url === '/v1/system/users' && options.method === 'POST',
    );
    expect(creation?.[1].body).toBe(
      JSON.stringify({
        username: 'mail.fixture',
        display_name: '邮件用户',
        email: '',
        password_login_enabled: false,
      }),
    );
    await user.click(within(credentials).getByRole('button', { name: '关闭抽屉' }));
    const row = await screen.findByRole('row', { name: /fixture.user/ });
    await user.click(within(row).getByRole('button', { name: '编辑' }));
    const editor = screen.getByRole('dialog');
    expect(within(editor).queryByLabelText('允许密码登录')).not.toBeInTheDocument();
    expect(within(editor).queryByLabelText(/临时密码/)).not.toBeInTheDocument();
    await user.click(within(editor).getByRole('button', { name: '保存更改' }));
    await screen.findByText('已更新用户 fixture.user');
    const update = fetchMock.mock.calls.find(([, options]) => options.method === 'PUT');
    expect(update?.[1].body).toBe(
      JSON.stringify({
        display_name: second.display_name,
        email: second.email,
        is_enabled: true,
        password_login_enabled: true,
      }),
    );
  });
  it.each([
    { entry: '/system/users', path: '/v1/system/users', label: '搜索用户' },
    { entry: '/system/audit', path: '/v1/system/audit/logs', label: '用户 ID' },
  ])(
    '$label 每次提交都请求接口，包括空条件、重复条件和缓存条件',
    async ({ entry, path, label }) => {
      const { user, fetchMock } = setup(entry);
      const requests = (target: string) =>
        fetchMock.mock.calls.filter(
          ([url, options]) =>
            new URL(url, 'https://fixture.example').pathname === target && options.method === 'GET',
        );
      const input = await screen.findByRole('textbox', { name: label });
      for (const keyword of entry === '/system/audit'
        ? ['', '123', '123', '']
        : ['', 'fixture.user', 'fixture.user', '']) {
        await waitFor(() => {
          expect(screen.getByRole('button', { name: '搜索' })).toBeEnabled();
        });
        await user.clear(input);
        if (keyword) {
          await user.type(input, ` ${keyword} `);
        }
        const before = requests(path).length;
        const beforeStats = requests('/v1/system/audit/statistics').length;
        await user.click(screen.getByRole('button', { name: '搜索' }));
        await waitFor(() => {
          expect(requests(path)).toHaveLength(before + 1);
          if (entry === '/system/audit') {
            expect(requests('/v1/system/audit/statistics')).toHaveLength(beforeStats + 1);
          }
          expect(screen.getByRole('button', { name: '搜索' })).toBeEnabled();
        });
        const address = new URL(requests(path).at(-1)?.[0] ?? '', 'https://fixture.example');
        expect(address.searchParams.get(entry === '/system/audit' ? 'user_id' : 'keyword')).toBe(
          keyword || null,
        );
        expect(address.searchParams.get('page')).toBe('1');
      }
    },
  );
  it('编辑保留不可变用户名，保存信息并通过确认弹窗删除用户', async () => {
    const { user, fetchMock } = setup();
    const row = await screen.findByRole('row', { name: /fixture.user/ });
    await user.click(within(row).getByRole('button', { name: '编辑' }));
    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByLabelText('用户名')).toHaveAttribute('readonly');
    const passwordEnabled = within(dialog).getByRole('checkbox', { name: '允许密码登录' });
    expect(passwordEnabled).toBeChecked();
    expect(passwordEnabled).toBeEnabled();
    expect(within(dialog).getByLabelText(/临时密码/)).toHaveValue('');
    expect(within(dialog).getByLabelText('邮箱（可选）')).toHaveValue(second.email);
    await user.clear(within(dialog).getByLabelText('显示名称'));
    await user.type(within(dialog).getByLabelText('显示名称'), '更新后的名称');
    await user.clear(within(dialog).getByLabelText('邮箱（可选）'));
    await user.click(within(dialog).getByRole('button', { name: '保存更改' }));
    expect(await screen.findByText('更新后的名称')).toBeVisible();
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
    const update = fetchMock.mock.calls.find(([, options]) => options.method === 'PUT');
    expect(update?.[1].body).toBe(
      JSON.stringify({
        display_name: '更新后的名称',
        email: '',
        is_enabled: true,
        password_login_enabled: true,
      }),
    );
    expect(screen.queryByText(second.email)).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '删除用户 fixture.user' }));
    const confirm = await screen.findByRole('dialog');
    expect(within(confirm).getByText(/永久删除/)).toBeVisible();
    await user.click(within(confirm).getByRole('button', { name: '确认删除' }));
    expect(await screen.findByText('已删除用户 fixture.user')).toBeVisible();
    expect(screen.queryByText('fixture.user')).not.toBeInTheDocument();
  });
  it('审计筛选和详情使用实际接口，展示快照差异与请求信息', async () => {
    const { user, fetchMock } = setup('/system/audit');
    await user.click(await screen.findByRole('button', { name: '查看添加用户详情' }));
    const detail = await screen.findByRole('dialog');
    expect(await within(detail).findByRole('tabpanel', { name: 'JSON 差异' })).toHaveTextContent(
      '虚构用户',
    );
    await user.click(within(detail).getByRole('tab', { name: '事件信息' }));
    expect(within(detail).getByText(record.request_id)).toBeVisible();
    await user.click(within(detail).getByRole('button', { name: '关闭抽屉' }));
    await user.click(screen.getByRole('button', { name: '更多筛选' }));
    await user.click(screen.getByRole('combobox', { name: '操作结果' }));
    await user.click(screen.getByRole('option', { name: '403 无权限' }));
    await user.keyboard('{Escape}');
    await user.click(screen.getByRole('button', { name: '搜索' }));
    expect(await screen.findByText('没有符合条件的审计记录')).toBeVisible();
    expect(
      fetchMock.mock.calls.some(
        ([url]) => url.includes('/v1/system/audit/logs?') && url.includes('result_code=403'),
      ),
    ).toBe(true);
  });
  it('API Token 支持关闭后再次查看、确认重置和撤销，凭据不进入查询缓存', async () => {
    const { user, client } = setup();
    const open = async () => {
      await user.click(await screen.findByRole('button', { name: '配置 fixture.admin 的 MCP' }));
      return screen.findByRole('dialog');
    };
    let drawer = await open();
    expect(await within(drawer).findByText('尚未配置 API Token')).toBeVisible();
    await user.click(within(drawer).getByRole('button', { name: '生成 Token' }));
    await waitFor(() => {
      expect(within(drawer).getByLabelText('API Token')).toHaveValue('fixture-api-token-1');
    });
    expect(
      client
        .getQueryCache()
        .getAll()
        .some((query) => JSON.stringify(query.state.data ?? null).includes('fixture-api-token-1')),
    ).toBe(false);
    await user.click(within(drawer).getByRole('button', { name: '关闭' }));
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
    drawer = await open();
    const input = await within(drawer).findByLabelText('API Token');
    expect(input).toHaveValue('fixture-api-token-1');
    expect(input).toHaveAttribute('type', 'password');
    await user.click(within(drawer).getByRole('button', { name: '重置 Token' }));
    let confirmation = screen.getAllByRole('dialog').at(-1);
    if (!confirmation) throw new Error('重置确认抽屉未打开');
    await user.click(within(confirmation).getByRole('button', { name: '确认重置' }));
    await waitFor(() => {
      expect(input).toHaveValue('fixture-api-token-2');
    });
    await user.click(within(drawer).getByRole('button', { name: '撤销 Token' }));
    confirmation = screen.getAllByRole('dialog').at(-1);
    if (!confirmation) throw new Error('撤销确认抽屉未打开');
    await user.click(within(confirmation).getByRole('button', { name: '确认撤销' }));
    expect(await within(drawer).findByText('尚未配置 API Token')).toBeVisible();
  });
  it('未登录时可直接查看首页日期与时间，点击登录后才进入登录页', async () => {
    const { router, fetchMock, user } = setup('/', null);
    expect(await screen.findByRole('group', { name: '当前日期' })).toBeVisible();
    const loginLink = await screen.findByRole('link', { name: '登录' });
    expect(router.state.location.pathname).toBe('/');
    expect(fetchMock.mock.calls.some(([url]) => url.startsWith('/v1/workspaces'))).toBe(false);
    await user.click(loginLink);
    expect(await screen.findByRole('region', { name: '邮件登录' })).toBeVisible();
  });
  it('普通用户访问系统管理时回到首页，认证失效后清理缓存并留在首页', async () => {
    const { client, router, fetchMock } = setup('/system/users', { ...account, is_admin: false });
    expect(await screen.findByRole('group', { name: '当前日期' })).toBeVisible();
    expect(screen.queryByRole('link', { name: '系统管理' })).not.toBeInTheDocument();
    client.setQueryData(['private', 'records'], ['待清理']);
    fetchMock.mockImplementation(() => Promise.resolve(respond(null, 401, '请先登录')));
    window.dispatchEvent(new Event('auth:expired'));
    await waitFor(() => {
      expect(client.getQueryData(['private', 'records'])).toBeUndefined();
    });
    expect(await screen.findByRole('link', { name: '登录' })).toBeVisible();
    expect(screen.getByRole('group', { name: '当前日期' })).toBeVisible();
    expect(router.state.location.pathname).toBe('/');
  });
  it.each([false, true])('待改密会话须完成修改才回跳，已有会话=%s', async (alreadySignedIn) => {
    const restricted = { ...account, password_change_required: true };
    const destination = '/system/users?filter=all#list';
    const { fetchMock, router, user, signInByEmail } = setup(
      destination,
      alreadySignedIn ? restricted : null,
      restricted,
    );
    if (!alreadySignedIn) {
      await screen.findByRole('region', { name: '邮件登录' });
      await signInByEmail();
    }
    expect(await screen.findByRole('heading', { name: '修改密码' })).toBeVisible();
    expect(new URLSearchParams(router.state.location.search).get('redirect')).toBe(destination);
    // 改密页重新打开后仍须返回原地址。
    await act(async () => {
      await router.navigate(`${changePasswordPath}${router.state.location.search}`, {
        replace: true,
        state: null,
      });
    });
    expect(fetchMock.mock.calls.some(([url]) => url.startsWith('/v1/system/'))).toBe(false);
    const newPassword = 'New-fictional-password-2026';
    await user.type(screen.getByLabelText('当前密码'), password);
    await user.type(screen.getByLabelText('新密码', { exact: true }), newPassword);
    await user.type(screen.getByLabelText('确认新密码'), newPassword);
    await user.click(screen.getByRole('button', { name: '保存新密码' }));
    expect(await screen.findByRole('row', { name: /fixture.user/ })).toBeVisible();
    expect(router.state.location).toMatchObject({
      pathname: '/system/users',
      search: '?filter=all',
      hash: '#list',
    });
    const request = fetchMock.mock.calls.find(([url]) => url === '/v1/change-password');
    expect(request?.[1].body).toBe(
      JSON.stringify({
        current_password_sha256: await passwordDigest(account.username, password),
        new_password_sha256: await passwordDigest(account.username, newPassword),
      }),
    );
  });
  it('业务认证失效时即使当前用户接口仍有效也停留登录页，重新登录后才回跳', async () => {
    const destination = '/system/users?filter=all#list';
    const { client, router, signInByEmail } = setup(destination);
    await screen.findByRole('row', { name: /fixture.user/ });
    window.dispatchEvent(new Event('auth:expired'));
    expect(await screen.findByRole('region', { name: '邮件登录' })).toBeVisible();
    await waitFor(() => {
      expect(client.getQueryData(currentUserKey)).toEqual(account);
    });
    expect(router.state.location.pathname).toBe('/login');
    expect(new URLSearchParams(router.state.location.search).get('redirect')).toBe(destination);
    expect(new URLSearchParams(router.state.location.search).get('expired')).toBe('1');
    await act(async () => {
      await router.navigate(`/login${router.state.location.search}`, {
        replace: true,
        state: null,
      });
    });
    expect(screen.getByRole('region', { name: '邮件登录' })).toBeVisible();
    await signInByEmail();
    await waitFor(() => {
      expect(router.state.location).toMatchObject({
        pathname: '/system/users',
        search: '?filter=all',
        hash: '#list',
      });
    });
  });
});
