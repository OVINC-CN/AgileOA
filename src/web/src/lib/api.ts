import { z } from 'zod';

import { getConfig } from '@/lib/config';
import { showMessage } from '@/lib/messages';

const envelopeSchema = z.object({
  trace: z.string().regex(/^(?!0{32}$)[0-9a-f]{32}$/),
  error_msg: z.string(),
  data: z.unknown(),
});

export class ApiError extends Error {
  readonly status: number;
  readonly trace: string | undefined;

  constructor(message: string, status: number, trace?: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.trace = trace;
  }
}

// 请求数据进入组件和查询缓存前，统一校验服务端响应。
export async function apiGet<T>(
  path: string,
  schema: z.ZodType<T>,
  signal?: AbortSignal,
  options?: Pick<RequestInit, 'redirect'> & { parseJSON?: (text: string) => unknown },
): Promise<T> {
  const { parseJSON, ...requestOptions } = options ?? {};
  return apiRequest(path, schema, { method: 'GET', ...requestOptions }, signal, parseJSON);
}

export async function apiPost<T>(path: string, schema: z.ZodType<T>, body?: unknown): Promise<T> {
  return apiWrite('POST', path, schema, body);
}

export function apiPut<T>(path: string, schema: z.ZodType<T>, body: unknown): Promise<T> {
  return apiWrite('PUT', path, schema, body);
}

export function apiDelete<T>(path: string, schema: z.ZodType<T>): Promise<T> {
  return apiWrite('DELETE', path, schema);
}

function apiWrite<T>(
  method: string,
  path: string,
  schema: z.ZodType<T>,
  body?: unknown,
): Promise<T> {
  return apiRequest(path, schema, {
    method,
    headers: { 'Content-Type': 'application/json', 'X-CSRF-Protection': '1' },
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
  });
}

async function apiRequest<T>(
  path: string,
  schema: z.ZodType<T>,
  options: RequestInit,
  signal?: AbortSignal,
  parseJSON?: (text: string) => unknown,
): Promise<T> {
  try {
    return await performRequest(path, schema, options, signal, parseJSON);
  } catch (cause) {
    if (signal?.aborted) throw cause;
    const error =
      cause instanceof ApiError
        ? cause
        : new ApiError(
            cause instanceof DOMException && cause.name === 'TimeoutError'
              ? '请求超时，请重试'
              : '网络请求失败，请重试',
            0,
          );
    // 未登录时探测当前账号属于正常流程，不展示异常通知。
    if (!(path === '/v1/user-info' && error.status === 401)) showMessage(error.message);
    throw error;
  }
}

async function performRequest<T>(
  path: string,
  schema: z.ZodType<T>,
  options: RequestInit,
  signal?: AbortSignal,
  parseJSON?: (text: string) => unknown,
): Promise<T> {
  // 读取请求配置，合并调用方取消信号和超时控制。
  const config = getConfig();
  const baseURL = config.apiBaseURL.replace(/\/$/, '');
  const timeout = AbortSignal.timeout(config.apiTimeoutMs);
  const headers = new Headers(options.headers);
  headers.set('Accept', 'application/json');
  const response = await fetch(`${baseURL}${path}`, {
    ...options,
    headers,
    credentials: 'include',
    signal: signal ? AbortSignal.any([signal, timeout]) : timeout,
  });

  // 任一受保护接口失去认证后立即通知页面清理用户缓存。
  if (
    response.status === 401 &&
    ![
      '/v1/login',
      '/v1/login-email',
      '/v1/student-login-email',
      '/v1/login-email-verify',
      '/v1/user-info',
      '/v1/home/calendar',
    ].includes(path.split('?')[0] ?? path)
  ) {
    window.dispatchEvent(new Event('auth:expired'));
  }

  // 所有接口仅接受 trace、error_msg、data 的统一协议。
  let body: unknown;
  try {
    body = parseJSON ? parseJSON(await response.text()) : await response.json();
  } catch {
    throw new ApiError('服务响应异常，请重试', response.status);
  }
  const parsed = envelopeSchema.safeParse(body);
  if (!parsed.success) {
    throw new ApiError('服务响应异常，请重试', response.status);
  }
  const envelope = parsed.data;
  if (!response.ok || envelope.error_msg !== '') {
    throw new ApiError(envelope.error_msg || '请求失败', response.status, envelope.trace);
  }

  // 校验具体业务数据，并保留服务端 Trace ID 用于定位错误。
  const data = schema.safeParse(envelope.data);
  if (!data.success) {
    throw new ApiError('服务响应异常，请重试', response.status, envelope.trace);
  }
  return data.data;
}
