import { useQueryClient } from '@tanstack/react-query';
import { Copy, Eye, EyeOff, KeyRound, Trash2 } from 'lucide-react';
import { useEffect, useState } from 'react';

import { Button } from '@/components/ui/button';
import { Drawer } from '@/components/ui/drawer';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Tooltip } from '@/components/ui/tooltip';
import {
  deleteUserAPIToken,
  getUserAPIToken,
  resetUserAPIToken,
  type UserAPIToken,
  type UserRow,
} from '@/features/management/api';
import { UserMCPGuide } from '@/features/management/user-mcp-guide';
import { useApiAction } from '@/lib/api-action';
import { formatIdentity } from '@/lib/format';
import { showMessage } from '@/lib/messages';

export function UserAPITokenDrawer({ user, onClose }: { user: UserRow; onClose: () => void }) {
  const client = useQueryClient();
  const [data, setData] = useState<UserAPIToken | null>(null);
  const [loading, setLoading] = useState(true);
  const [visible, setVisible] = useState(false);
  const [confirmation, setConfirmation] = useState<'reset' | 'delete' | null>(null);
  const action = useApiAction();
  const configured = Boolean(data?.token);

  useEffect(() => {
    // 凭据仅保留在当前抽屉内，关闭时取消请求，不进入查询缓存或浏览器存储。
    const controller = new AbortController();
    void getUserAPIToken(user.id, controller.signal)
      .then((result) => {
        if (!controller.signal.aborted) setData(result);
      })
      .catch(() => {
        // 请求层统一展示错误消息。
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => {
      controller.abort();
    };
  }, [user.id]);

  const save = async (kind: 'reset' | 'delete') => {
    const result = await action.run(kind, async () => {
      const next =
        kind === 'reset'
          ? await resetUserAPIToken(user.id)
          : await deleteUserAPIToken(user.id).then(() => ({
              token: '',
              created_at: null,
              updated_at: null,
            }));
      // 相关列表和审计刷新结束后才退出当前按钮的加载状态。
      await Promise.all([
        client.invalidateQueries({ queryKey: ['management', 'users'] }),
        client.invalidateQueries({ queryKey: ['management', 'audit'] }),
      ]);
      return next;
    });
    if (result.ok) {
      setData(result.data);
      setVisible(kind === 'reset');
      setConfirmation(null);
      showMessage(kind === 'reset' ? 'API Token 已生成，可随时再次查看' : 'API Token 已撤销');
    }
  };
  const copy = async () => {
    if (!data?.token) return;
    try {
      await navigator.clipboard.writeText(data.token);
      showMessage('API Token 已复制');
    } catch {
      showMessage('复制失败，请显示 API Token 后手动复制');
    }
  };

  return (
    <Drawer
      title="MCP 配置"
      description={formatIdentity(user.display_name, user.username)}
      busy={action.pending}
      onClose={onClose}
      footer={
        <>
          <Button variant="secondary" disabled={action.pending} onClick={onClose}>
            关闭
          </Button>
          {configured ? (
            <Button
              variant="outline"
              loading={action.isPending('delete')}
              disabled={action.pending}
              onClick={() => {
                setConfirmation('delete');
              }}
            >
              <Trash2 aria-hidden="true" />
              撤销 Token
            </Button>
          ) : null}
          {data ? (
            <Button
              loading={action.isPending('reset')}
              disabled={action.pending || !user.is_enabled || user.password_change_required}
              onClick={() => {
                if (configured) setConfirmation('reset');
                else void save('reset');
              }}
            >
              <KeyRound aria-hidden="true" />
              {configured ? '重置 Token' : '生成 Token'}
            </Button>
          ) : null}
        </>
      }
    >
      <div className="space-y-5 p-6">
        <p className="text-sm leading-6 text-muted-foreground">
          每个账号最多配置一个 API Token，用于连接 MCP。
        </p>
        {loading ? (
          <p role="status" className="text-sm text-muted-foreground">
            正在读取 API Token…
          </p>
        ) : data ? (
          configured ? (
            <div className="space-y-2">
              <Label htmlFor="user-api-token">API Token</Label>
              <div className="flex items-center gap-2">
                <Input
                  id="user-api-token"
                  type={visible ? 'text' : 'password'}
                  value={data.token}
                  readOnly
                  autoComplete="off"
                  spellCheck={false}
                  className="flex-1 font-mono text-xs"
                />
                <Tooltip content={visible ? '隐藏 Token' : '显示 Token'}>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    aria-label={visible ? '隐藏 Token' : '显示 Token'}
                    onClick={() => {
                      setVisible(!visible);
                    }}
                  >
                    {visible ? <EyeOff aria-hidden="true" /> : <Eye aria-hidden="true" />}
                  </Button>
                </Tooltip>
                <Tooltip content="复制 Token">
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    aria-label="复制 Token"
                    onClick={() => {
                      void copy();
                    }}
                  >
                    <Copy aria-hidden="true" />
                  </Button>
                </Tooltip>
              </div>
            </div>
          ) : (
            <p className="text-sm">尚未配置 API Token</p>
          )
        ) : null}
        {!user.is_enabled || user.password_change_required ? (
          <p className="text-sm text-muted-foreground">
            启用账号并完成密码修改后可生成 API Token。
          </p>
        ) : null}
        <UserMCPGuide />
      </div>
      {confirmation ? (
        <Drawer
          title={confirmation === 'reset' ? '重置 API Token' : '撤销 API Token'}
          description={formatIdentity(user.display_name, user.username)}
          busy={action.pending}
          onClose={() => {
            setConfirmation(null);
          }}
          footer={
            <>
              <Button
                variant="secondary"
                disabled={action.pending}
                onClick={() => {
                  setConfirmation(null);
                }}
              >
                取消
              </Button>
              <Button
                loading={action.isPending(confirmation)}
                disabled={action.pending}
                onClick={() => {
                  void save(confirmation);
                }}
              >
                {confirmation === 'reset' ? (
                  <KeyRound aria-hidden="true" />
                ) : (
                  <Trash2 aria-hidden="true" />
                )}
                {confirmation === 'reset' ? '确认重置' : '确认撤销'}
              </Button>
            </>
          }
        >
          <p className="p-6 text-sm leading-6">
            {confirmation === 'reset'
              ? '重置后旧 Token 将立即失效，使用该 Token 的调用需要更新凭据。'
              : '撤销后该 Token 将立即失效，需要时可重新生成。'}
          </p>
        </Drawer>
      ) : null}
    </Drawer>
  );
}
