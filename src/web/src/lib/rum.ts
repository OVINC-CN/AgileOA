import type { AppConfig } from '@/lib/config';

const staticAssetDirectory = /^\/(?:assets|extra-assets)\//u;
const runtimeConfigPath = /^\/config(?:\.[a-f0-9]+)?\.js$/u;
const workspaceIdSegment = /\/workspaces\/[a-z0-9]{16}(?=\/|$)/gu;
const recordIdSegment = /\/\d+(?=\/|$)/gu;

interface SpeedLog {
  url: string;
  urlQuery?: string;
}

interface ErrorLog {
  level: string;
  from?: string;
}

interface ReportRequest {
  url: string;
}

export async function initializeRum(
  { aegis: options, apiBaseURL }: Pick<AppConfig, 'aegis' | 'apiBaseURL'>,
  pageURL: () => string,
): Promise<void> {
  // 配置留空时不加载 SDK。
  if (!options?.id) return;

  try {
    const { default: Aegis } = await import('aegis-web-sdk');
    const siteOrigin = window.location.origin;
    const apiURL = new URL(apiBaseURL || siteOrigin);
    const apiPrefix = `${apiURL.pathname.replace(/\/$/u, '')}/v1`;
    const isBusinessAPI = ({ origin, pathname }: URL) =>
      origin === apiURL.origin && pathname.startsWith(`${apiPrefix}/`);
    const { ERROR, PROMISE_ERROR, AJAX_ERROR, RET_ERROR, SCRIPT_ERROR, CSS_ERROR } = Aegis.logType;
    const errorTypes = new Map<string, string>([
      [ERROR, 'JavaScript 执行异常'],
      [PROMISE_ERROR, 'Promise 未处理异常'],
      [AJAX_ERROR, '接口请求异常'],
      [RET_ERROR, '接口返回异常'],
      [SCRIPT_ERROR, '脚本加载异常'],
      [CSS_ERROR, '样式加载异常'],
    ]);

    // 接口结果使用 HTTP 状态码。地址仅保留本站资源和业务接口的归一化路径。
    const aegis = new Aegis({
      ...options,
      env: import.meta.env.DEV ? Aegis.environment.development : Aegis.environment.production,
      urlHandler: pageURL,
      api: {
        ...options.api,
        resourceTypeHandler(address: string) {
          return URL.canParse(address, siteOrigin) && isBusinessAPI(new URL(address, siteOrigin))
            ? 'fetch'
            : 'static';
        },
        retCodeHandler(_body: unknown, _url: string, response?: { status: number }) {
          const status = response?.status ?? 0;
          return { code: String(status), isErr: status === 0 || status >= 400 };
        },
      },
      beforeReportSpeed(log: SpeedLog) {
        if (!URL.canParse(log.url, siteOrigin)) return false;
        const url = new URL(log.url, siteOrigin);
        const { origin, pathname } = url;
        if (isBusinessAPI(url)) {
          log.url = pathname
            .replace(workspaceIdSegment, '/workspaces/:workspace_id')
            .replace(recordIdSegment, '/:id');
        } else if (
          origin === siteOrigin &&
          (staticAssetDirectory.test(pathname) || runtimeConfigPath.test(pathname))
        ) {
          log.url = pathname;
        } else {
          return false;
        }
        delete log.urlQuery;
        return true;
      },
      beforeRequest({ logType, logs }: { logType: string; logs: ErrorLog }) {
        // 错误正文可能含有请求地址、凭据或业务资料，只保留固定类型与页面维度。
        if (logType === 'sdkError') return false;
        if (logType !== 'log') return true;
        const msg = errorTypes.get(logs.level);
        if (!msg) return false;
        return { logType, logs: { level: logs.level, msg, from: logs.from } };
      },
      onBeforeRequest(request: ReportRequest) {
        const url = new URL(request.url);
        url.searchParams.delete('originFrom');
        url.searchParams.delete('referer');
        url.searchParams.delete('uin');
        request.url = url.href;
        return request;
      },
    });
    // 清除 SDK 自动提取的信息，批量测速正文也使用这些公共字段。
    aegis.extendBean('referer', '');
    aegis.extendBean('uin', '');
  } catch {
    // 监控加载失败时继续提供页面服务。
  }
}
