import { describe, expect, it, vi } from 'vitest';

import { getConfig } from '@/lib/config';
import { initializeRum } from '@/lib/rum';

const createAegis = vi.hoisted(() =>
  vi.fn(function (_settings: unknown) {
    return { extendBean: vi.fn() };
  }),
);

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
