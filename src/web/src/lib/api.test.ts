import { describe, expect, it, vi } from 'vitest';
import { z } from 'zod';

import { ApiError, apiGet, apiPost } from '@/lib/api';
import { getConfig } from '@/lib/config';

const trace = '4bf92f3577b34da6a3ce929d0e0e4736';

function mockResponse(body: unknown, status = 200) {
  const response = new Response(JSON.stringify(body), { status });
  const mock = vi.fn().mockResolvedValue(response);
  vi.stubGlobal('fetch', mock);
  return mock;
}

describe('接口响应协议', () => {
  it('报告导入请求的实际上传进度并沿用会话和响应协议', async () => {
    const request = new XMLHttpRequest();
    const open = vi.spyOn(request, 'open').mockImplementation(() => undefined);
    const setHeader = vi.spyOn(request, 'setRequestHeader').mockImplementation(() => undefined);
    const send = vi.spyOn(request, 'send').mockImplementation(() => undefined);
    vi.stubGlobal(
      'XMLHttpRequest',
      vi.fn(function () {
        return request;
      }),
    );
    const onProgress = vi.fn();
    const result = apiPost(
      '/fixture/import',
      z.object({ created: z.number() }),
      { content: 'YQ==' },
      onProgress,
    );

    expect(open).toHaveBeenCalledWith('POST', '/fixture/import');
    expect(request.withCredentials).toBe(true);
    expect(request.timeout).toBe(getConfig().apiTimeoutMs);
    expect(setHeader).toHaveBeenCalledWith('x-csrf-protection', '1');
    expect(send).toHaveBeenCalledWith('{"content":"YQ=="}');
    request.upload.dispatchEvent(new ProgressEvent('progress', { loaded: 5, total: 20 }));
    request.upload.dispatchEvent(new ProgressEvent('load', { loaded: 20, total: 20 }));
    expect(onProgress.mock.calls).toEqual([[0], [25], [100]]);

    vi.spyOn(request, 'status', 'get').mockReturnValue(200);
    vi.spyOn(request, 'responseText', 'get').mockReturnValue(
      JSON.stringify({ trace, error_msg: '', data: { created: 1 } }),
    );
    request.dispatchEvent(new ProgressEvent('load'));
    await expect(result).resolves.toEqual({ created: 1 });
  });

  it('公开节假日接口携带日期参数时，失败不会撤销已有会话', async () => {
    const expired = vi.fn();
    window.addEventListener('auth:expired', expired);
    try {
      mockResponse({ trace, error_msg: '请求被拒绝', data: null }, 401);
      await expect(apiGet('/v1/home/calendar?date=2027-01-01', z.null())).rejects.toMatchObject({
        status: 401,
      });
      expect(expired).not.toHaveBeenCalled();
    } finally {
      window.removeEventListener('auth:expired', expired);
    }
  });

  it.each(['/v1/login-email', '/v1/login-email-verify'])(
    '公开认证接口 %s 的失败不会撤销已有会话',
    async (path) => {
      const expired = vi.fn();
      window.addEventListener('auth:expired', expired);
      try {
        mockResponse({ trace, error_msg: '登录信息无效', data: null }, 401);
        await expect(apiPost(path, z.null(), {})).rejects.toMatchObject({ status: 401 });
        expect(expired).not.toHaveBeenCalled();
      } finally {
        window.removeEventListener('auth:expired', expired);
      }
    },
  );
  it('拒绝不符合业务模型的数据', async () => {
    mockResponse({ trace, error_msg: '', data: { count: '无效' } });
    await expect(apiGet('/example', z.object({ count: z.number() }))).rejects.toMatchObject({
      message: '服务响应异常，请重试',
      trace,
    });
  });

  it('保留简体中文错误和真实的 Trace ID', async () => {
    mockResponse({ trace, error_msg: '暂时不可用', data: null }, 503);
    await expect(apiGet('/ready', z.null())).rejects.toMatchObject({
      name: 'ApiError',
      status: 503,
      message: '暂时不可用',
      trace,
    });
  });

  it('将非 JSON 代理错误转换为可显示的错误', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(new Response('<html>代理异常</html>', { status: 502 })),
    );
    await expect(apiGet('/ready', z.null())).rejects.toBeInstanceOf(ApiError);
  });

  it('拒绝空追踪', async () => {
    mockResponse({ trace: '0'.repeat(32), error_msg: '', data: null });
    await expect(apiGet('/ready', z.null())).rejects.toMatchObject({
      message: '服务响应异常，请重试',
    });
  });

  it('修改运行配置即可使用新的后端地址，无需重新构建', async () => {
    const original = window.AGILE_WORKSPACE_CONFIG;
    try {
      window.AGILE_WORKSPACE_CONFIG = { ...getConfig(), apiBaseURL: 'https://api.example.com' };
      const fetchMock = mockResponse({ trace, error_msg: '', data: null });
      await expect(apiGet('/ready', z.null())).resolves.toBeNull();
      expect(fetchMock).toHaveBeenCalledWith('https://api.example.com/ready', expect.any(Object));
    } finally {
      window.AGILE_WORKSPACE_CONFIG = original;
    }
  });
});
