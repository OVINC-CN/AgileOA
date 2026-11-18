import { Copy } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Tooltip } from '@/components/ui/tooltip';
import { getConfig } from '@/lib/config';
import { showMessage } from '@/lib/messages';

export function UserMCPGuide() {
  // 接入地址沿用 API 运行配置，同源部署时补全当前站点来源。
  const config = getConfig();
  const baseURL = config.apiBaseURL.replace(/\/$/, '') || window.location.origin;
  const endpoint = `${baseURL}/v1/mcp`;
  const example = JSON.stringify(
    {
      url: endpoint,
      headers: {
        Authorization: 'Bearer <API Token>',
        'X-CSRF-Protection': '1',
      },
    },
    null,
    2,
  );
  const copy = async () => {
    // 复制当前部署的接入地址，实际凭据继续由上方 Token 区域管理。
    try {
      await navigator.clipboard.writeText(endpoint);
      showMessage('MCP 服务地址已复制');
    } catch {
      showMessage('复制失败，请选中内容后手动复制');
    }
  };

  return (
    <section aria-labelledby="user-mcp-guide-title" className="min-w-0 space-y-4 border-t pt-5">
      <div className="space-y-1">
        <h2 id="user-mcp-guide-title" className="text-sm font-semibold">
          MCP 接入
        </h2>
        <p className="text-sm leading-6 text-muted-foreground">
          在支持远程 MCP 和自定义请求头的客户端中添加服务，按以下步骤填写。
        </p>
      </div>
      <div className="space-y-3">
        <h3 className="text-sm font-medium">1. 添加远程服务</h3>
        <dl className="grid gap-3 rounded-xl bg-muted/80 p-4 text-sm sm:grid-cols-2">
          <div className="min-w-0 space-y-1">
            <dt className="text-xs text-muted-foreground">服务名称</dt>
            <dd className="font-mono">agile_oa</dd>
          </div>
          <div className="space-y-1">
            <dt className="text-xs text-muted-foreground">传输方式</dt>
            <dd>Streamable HTTP</dd>
          </div>
        </dl>
        <div className="space-y-2">
          <Label htmlFor="user-mcp-endpoint">服务地址 / URL</Label>
          <div className="flex items-center gap-2">
            <Input
              id="user-mcp-endpoint"
              value={endpoint}
              readOnly
              spellCheck={false}
              className="flex-1 font-mono text-xs"
            />
            <Tooltip content="复制地址">
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label="复制地址"
                onClick={() => {
                  void copy();
                }}
              >
                <Copy aria-hidden="true" />
              </Button>
            </Tooltip>
          </div>
        </div>
      </div>
      <div className="space-y-3">
        <h3 className="text-sm font-medium">2. 配置认证和请求头</h3>
        <dl className="divide-y divide-border rounded-xl bg-muted/80 px-4 text-sm">
          <div className="grid grid-cols-2 gap-4 py-3">
            <dt className="text-muted-foreground">认证方式</dt>
            <dd className="min-w-0 space-y-1">
              <p>Bearer Token</p>
              <p className="text-xs leading-5 text-muted-foreground">
                Token 输入框填写上方 API Token。
              </p>
            </dd>
          </div>
          <div className="grid grid-cols-2 gap-4 py-3">
            <dt className="font-mono text-xs wrap-anywhere text-muted-foreground">Authorization</dt>
            <dd className="min-w-0 space-y-1">
              <p className="font-mono text-xs wrap-anywhere">Bearer &lt;API Token&gt;</p>
              <p className="text-xs leading-5 text-muted-foreground">
                通过自定义请求头认证时填写，Bearer 后保留一个空格。
              </p>
            </dd>
          </div>
          <div className="grid grid-cols-2 gap-4 py-3">
            <dt className="font-mono text-xs wrap-anywhere text-muted-foreground">
              X-CSRF-Protection
            </dt>
            <dd className="min-w-0 space-y-1">
              <p className="font-mono text-xs">1</p>
              <p className="text-xs leading-5 text-muted-foreground">
                使用以上任一认证方式均需添加此请求头。
              </p>
            </dd>
          </div>
        </dl>
        <div className="space-y-2">
          <h4 className="text-sm font-medium">连接参数示例</h4>
          <pre className="rounded-xl bg-muted/80 p-4 text-xs leading-6 wrap-anywhere whitespace-pre-wrap">
            <code>{example}</code>
          </pre>
          <p className="text-xs leading-6 text-muted-foreground">
            将 &lt;API Token&gt; 替换为上方复制的 Token，连接参数填入客户端对应的远程 MCP 配置。
          </p>
        </div>
      </div>
    </section>
  );
}
