import { QueryClient } from '@tanstack/react-query';
import { describe, expect, it, vi } from 'vitest';

import {
  type CurrentUser,
  currentUserKey,
  currentUserOptions,
  subscribeCurrentUserUin,
} from '@/features/user/api';
import { getConfig } from '@/lib/config';
import { initializeRum } from '@/lib/rum';

const { createAegis, setConfig } = vi.hoisted(() => {
  const setConfig = vi.fn();
  const createAegis = vi.fn<
    (settings: unknown) => { extendBean: () => void; setConfig: typeof setConfig }
  >(function () {
    return { extendBean: vi.fn(), setConfig };
  });
  return { createAegis, setConfig };
});

vi.mock('aegis-web-sdk', () => ({
  default: Object.assign(createAegis, {
    environment: { development: 'development', production: 'production' },
    logType: {},
  }),
}));

describe('前端性能监控配置', () => {
  it('保留完整的 SDK 选项与自定义回调', async () => {
    const options = {
      id: ' example-id ',
      aid: true,
      env: 'test',
      reportApiSpeed: { urlHandler: vi.fn() },
      api: { retCodeHandler: vi.fn() },
      beforeRequest: vi.fn(),
    };
    vi.stubGlobal('AGILE_WORKSPACE_CONFIG', { ...getConfig(), aegis: options });

    const config = getConfig();
    await initializeRum(config, () => '/');

    expect(config.aegis).toBe(options);
    expect(createAegis.mock.calls[0]?.[0]).toMatchObject({ ...options, id: 'example-id' });
  });

  it('只在用户标识变化时更新 SDK', async () => {
    const setUin = await initializeRum({ ...getConfig(), aegis: { id: 'example-id' } }, () => '/');
    expect(setUin).toBeDefined();

    setUin?.('user-6666');
    setUin?.('user-6666');
    setUin?.('');

    expect(setConfig).toHaveBeenCalledTimes(2);
    expect(setConfig).toHaveBeenNthCalledWith(1, { uin: 'user-6666' });
    expect(setConfig).toHaveBeenNthCalledWith(2, { uin: '' });
  });

  it('按查询确认的账号类型更新标识并在会话结束时清空', async () => {
    const user: CurrentUser = {
      identity_type: 'user',
      id: 6666,
      workspace_id: '',
      workspace_name: '',
      username: 'example-user',
      display_name: '示例账号',
      email: '',
      is_admin: false,
      password_login_enabled: false,
      password_change_required: false,
    };
    const student: CurrentUser = {
      ...user,
      identity_type: 'student',
      workspace_id: 'w7m2k9p4s6n8c3x5',
      workspace_name: '示例工作空间',
    };
    const response = (data: CurrentUser | null, status = 200) =>
      Response.json(
        { trace: '4bf92f3577b34da6a3ce929d0e0e4736', error_msg: data ? '' : '未登录', data },
        { status },
      );
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockResolvedValueOnce(response(user))
        .mockResolvedValueOnce(response(student))
        .mockResolvedValueOnce(response(null, 401))
        .mockResolvedValueOnce(response(user)),
    );
    const client = new QueryClient();
    const onChange = vi.fn();
    const unsubscribe = subscribeCurrentUserUin(client, onChange);

    try {
      client.setQueryData(currentUserKey, user);
      expect(onChange).not.toHaveBeenCalled();
      await client.query(currentUserOptions);
      await client.query(currentUserOptions);
      await client.query(currentUserOptions);
      await client.query(currentUserOptions);
      client.clear();
      expect(onChange.mock.calls).toEqual([
        ['user-6666'],
        ['student-6666'],
        [''],
        ['user-6666'],
        [''],
      ]);
    } finally {
      unsubscribe();
      client.clear();
    }
  });

  it.each([
    { name: '省略对象', aegis: undefined },
    { name: '空对象', aegis: {} },
    { name: '空值', aegis: null },
    { name: '数组', aegis: [] },
    { name: '字符串', aegis: '无效配置' },
    { name: '空白 ID', aegis: { id: ' ' } },
  ])('$name 时保持应用配置可用', async ({ aegis }) => {
    vi.stubGlobal('AGILE_WORKSPACE_CONFIG', { ...getConfig(), aegis });

    await initializeRum(getConfig(), () => '/');

    expect(createAegis).not.toHaveBeenCalled();
  });
});
