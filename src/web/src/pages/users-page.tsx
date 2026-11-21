import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, Plus, Search, Trash2, Unplug, X } from 'lucide-react';
import { useState } from 'react';

import { ErrorMessage } from '@/components/error-message';
import { FilterField, FilterPanel } from '@/components/filter-panel';
import { UserCharts } from '@/components/management-charts';
import { ManagementPageHeader } from '@/components/management-page-header';
import { RefreshButton } from '@/components/refresh-button';
import { SearchResetButton } from '@/components/search-reset-button';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Drawer } from '@/components/ui/drawer';
import { Input } from '@/components/ui/input';
import { SearchSelect } from '@/components/ui/search-select';
import {
  deleteUser,
  revokeSessions,
  type UserFilters,
  userListOptions,
  type UserRow,
  userStatisticsOptions,
} from '@/features/management/api';
import { UserAPITokenDrawer } from '@/features/management/user-api-token';
import { UserEditor } from '@/features/management/user-editor';
import { UserTable } from '@/features/management/user-table';
import { useCurrentUser } from '@/features/user/api';
import { useApiAction, useApiMutation, useInitialLoading } from '@/lib/api-action';
import { useFilterForm } from '@/lib/filter-form';
import { formatIdentity } from '@/lib/format';
import { useTablePagination } from '@/lib/pagination';

const emptyRows: UserRow[] = [];
const defaultFilters: UserFilters = { keyword: '', status: [] };

function UserAction({
  kind,
  user,
  onClose,
  onSwitch,
  onDone,
}: {
  kind: 'delete' | 'revoke';
  user: UserRow;
  onClose: () => void;
  onSwitch: () => void;
  onDone: (message: string) => void;
}) {
  const client = useQueryClient();
  const mutation = useApiMutation({
    mutationFn: async () => {
      await (kind === 'delete' ? deleteUser(user.id) : revokeSessions(user.id));
      await Promise.all([
        client.invalidateQueries({ queryKey: ['management', 'users'] }),
        client.invalidateQueries({ queryKey: ['management', 'audit'] }),
      ]);
    },
    onSuccess: () => {
      onDone(kind === 'delete' ? `已删除用户 ${user.username}` : `已撤销 ${user.username} 的会话`);
    },
  });
  const blocked = kind === 'delete' && (user.session_count > 0 || user.has_api_token);
  return (
    <Drawer
      title={kind === 'delete' ? '删除用户' : '撤销用户会话'}
      busy={mutation.isPending}
      truncateDescription
      description={formatIdentity(user.display_name, user.username)}
      onClose={() => {
        if (!mutation.isPending) {
          onClose();
        }
      }}
      footer={
        <>
          <Button variant="cancel" disabled={mutation.isPending} onClick={onClose}>
            取消
          </Button>
          {blocked ? (
            <Button onClick={onSwitch}>撤销会话</Button>
          ) : (
            <Button
              loading={mutation.isPending}
              onClick={() => {
                mutation.mutate();
              }}
            >
              {kind === 'delete' ? <Trash2 aria-hidden="true" /> : <Unplug aria-hidden="true" />}
              {kind === 'delete' ? '确认删除' : '确认撤销'}
            </Button>
          )}
        </>
      }
    >
      <div className="space-y-4 p-6">
        <div className="flex size-11 items-center justify-center rounded-xl bg-primary/10 text-primary">
          {kind === 'delete' ? <Trash2 aria-hidden="true" /> : <Unplug aria-hidden="true" />}
        </div>
        <p className="text-sm leading-6">
          {kind === 'delete'
            ? '将永久删除该账号及其权限、负责的月度工作、担任书记的党支部和相应关联记录；历史操作记录保留。存在会话、API Token 或评奖评优审批引用时不可删除。'
            : '将撤销该用户在所有设备上的会话、API Token 及待使用的邮件登录验证码，该用户需要重新登录。'}
        </p>
        {blocked ? (
          <p className="rounded-xl border border-warning-border bg-warning/70 p-3 text-sm text-warning-foreground">
            该用户仍有会话或 API Token，请先撤销全部会话和 API Token，再删除账号。
          </p>
        ) : null}
        {mutation.error ? <ErrorMessage>{mutation.error.message}</ErrorMessage> : null}
      </div>
    </Drawer>
  );
}

export function UsersPage() {
  const { data: current } = useCurrentUser();
  const client = useQueryClient();
  const [filters, setFilters] = useState(defaultFilters);
  const { page, pageSize, setPage, setPageSize } = useTablePagination();
  const [editor, setEditor] = useState<{ user: UserRow | null } | null>(null);
  const [action, setAction] = useState<{ kind: 'delete' | 'revoke'; user: UserRow } | null>(null);
  const [apiTokenUser, setAPITokenUser] = useState<UserRow | null>(null);
  const [notice, setNotice] = useState('');
  const searchAction = useApiAction();
  const list = useQuery({
    ...userListOptions(filters, page, pageSize),
    enabled: current?.is_admin === true,
    placeholderData: keepPreviousData,
  });
  const stats = useQuery({ ...userStatisticsOptions, enabled: current?.is_admin === true });
  const initialLoading = useInitialLoading(
    (list.isPending && !list.isFetched) || (stats.isPending && !stats.isFetched),
  );
  const applyFilters = async (values: UserFilters) => {
    const next = { ...values, keyword: values.keyword.trim() };
    // 提交筛选时重新请求，相同条件和已缓存条件也获取最新数据。
    setPage(1);
    setFilters(next);
    await client.query({ ...userListOptions(next, 1, pageSize), staleTime: 0 });
  };
  const { draft, setDraft, changeFilters, hasUnappliedChanges } = useFilterForm(
    defaultFilters,
    applyFilters,
    filters,
  );
  const done = (message: string) => {
    setEditor(null);
    setAction(null);
    setNotice(message);
    if (list.data?.items.length === 1 && page > 1 && message.startsWith('已删除')) {
      setPage(page - 1);
    }
  };
  if (!current?.is_admin) {
    return <ErrorMessage>仅系统管理员可以访问用户管理。</ErrorMessage>;
  }
  return (
    <section className="space-y-6" aria-labelledby="users-title">
      <ManagementPageHeader id="users-title" title="用户管理">
        <Button
          onClick={() => {
            setEditor({ user: null });
          }}
        >
          <Plus aria-hidden="true" />
          添加用户
        </Button>
      </ManagementPageHeader>
      <UserCharts data={stats.data} loading={stats.isPending} />
      {stats.error ? <ErrorMessage>{stats.error.message}</ErrorMessage> : null}
      {notice ? (
        <div
          role="status"
          className="flex items-center gap-2 rounded-xl border border-warning-border/70 bg-warning/70 px-4 py-3 text-sm text-warning-foreground"
        >
          <Check className="size-4 shrink-0" aria-hidden="true" />
          <span className="min-w-0 flex-1 wrap-anywhere">{notice}</span>
          <Button
            variant="cancel"
            size="icon-xs"
            className="ml-auto"
            aria-label="关闭提示"
            onClick={() => {
              setNotice('');
            }}
          >
            <X aria-hidden="true" />
          </Button>
        </div>
      ) : null}
      <FilterPanel
        hasUnappliedChanges={hasUnappliedChanges}
        onSubmit={(event) => {
          event.preventDefault();
          void searchAction.run('search', () => applyFilters(draft));
        }}
        actions={
          <>
            <SearchResetButton
              loading={searchAction.isPending('reset')}
              disabled={initialLoading || searchAction.pending}
              onClick={() => {
                void searchAction.run('reset', async () => {
                  setDraft(defaultFilters);
                  await applyFilters(defaultFilters);
                });
              }}
            />
            <Button
              type="submit"
              loading={
                searchAction.isPending('search') || (!searchAction.pending && initialLoading)
              }
              disabled={searchAction.pending}
            >
              <Search aria-hidden="true" />
              搜索
            </Button>
          </>
        }
      >
        <FilterField label="用户信息" htmlFor="users-page-keyword">
          <Input
            id="users-page-keyword"
            aria-label="搜索用户"
            placeholder="搜索用户名、显示名称或邮箱"
            className="w-full"
            value={draft.keyword}
            onChange={(event) => {
              setDraft({ ...draft, keyword: event.target.value });
            }}
          />
        </FilterField>
        <FilterField label="账号状态">
          <SearchSelect
            multiple
            allLabel="全部状态"
            label="账号状态"
            className="w-full"
            options={[
              { value: 'enabled', label: '启用' },
              { value: 'disabled', label: '停用' },
            ]}
            value={draft.status}
            onValueChange={(status) => {
              changeFilters({ status });
            }}
          />
        </FilterField>
      </FilterPanel>
      <Card className="gap-0 overflow-hidden border-border py-0">
        {list.error ? <ErrorMessage>{list.error.message}</ErrorMessage> : null}
        <UserTable
          refreshAction={
            <RefreshButton
              label="刷新用户数据"
              onRefresh={() => client.invalidateQueries({ queryKey: ['management', 'users'] })}
            />
          }
          data={list.data?.items ?? emptyRows}
          total={list.data?.total ?? 0}
          page={page}
          pageSize={pageSize}
          loading={list.isPending}
          currentID={current.id}
          onPage={async (next) => {
            setPage(next);
            await client.query({
              ...userListOptions(filters, next, pageSize),
              staleTime: 0,
            });
          }}
          onPageSize={async (size) => {
            setPageSize(size);
            await client.query({ ...userListOptions(filters, 1, size), staleTime: 0 });
          }}
          onEdit={(user) => {
            setEditor({ user });
          }}
          onDelete={(user) => {
            setAction({ kind: 'delete', user });
          }}
          onRevoke={(user) => {
            setAction({ kind: 'revoke', user });
          }}
          onAPIToken={setAPITokenUser}
        />
      </Card>
      {editor ? (
        <UserEditor
          user={editor.user}
          currentID={current.id}
          onClose={() => {
            setEditor(null);
          }}
          onSaved={done}
        />
      ) : null}
      {apiTokenUser ? (
        <UserAPITokenDrawer
          key={apiTokenUser.id}
          user={apiTokenUser}
          onClose={() => {
            setAPITokenUser(null);
          }}
        />
      ) : null}
      {action ? (
        <UserAction
          key={action.kind + String(action.user.id)}
          kind={action.kind}
          user={action.user}
          onClose={() => {
            setAction(null);
          }}
          onSwitch={() => {
            setAction({ ...action, kind: 'revoke' });
          }}
          onDone={done}
        />
      ) : null}
    </section>
  );
}
