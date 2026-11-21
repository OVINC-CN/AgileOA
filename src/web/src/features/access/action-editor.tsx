import { useQuery, useQueryClient } from '@tanstack/react-query';
import { ChevronDown, RotateCw, Save } from 'lucide-react';
import { Collapsible } from 'radix-ui';
import { useId, useState } from 'react';

import { ErrorMessage } from '@/components/error-message';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Drawer } from '@/components/ui/drawer';
import { type ActionGroup, getActionGroups } from '@/features/access/action-groups';
import {
  type AccessUser,
  type Action,
  actionCatalogOptions,
  setUserActions,
  type UserActions,
  userActionsOptions,
} from '@/features/access/api';
import { useApiAction, useApiMutation } from '@/lib/api-action';
import { formatIdentity } from '@/lib/format';
import { cn } from '@/lib/utils';

function ActionGroupSection({
  group,
  selected,
  busy,
  onToggleAll,
  onToggle,
  depth = 0,
  parentLabel = '',
}: {
  group: ActionGroup;
  selected: string[];
  busy: boolean;
  onToggleAll: (actions: Action[]) => void;
  onToggle: (value: string, checked: boolean) => void;
  depth?: number;
  parentLabel?: string;
}) {
  const id = useId();
  const Heading = depth === 0 ? 'h2' : depth === 1 ? 'h3' : 'h4';
  const label = parentLabel ? `${parentLabel} · ${group.label}` : group.label;
  const count = group.actions.filter(({ value }) => selected.includes(value)).length;
  const allSelected = count === group.actions.length;
  const selectionAction = allSelected ? '取消全选' : '全选';

  return (
    <Collapsible.Root asChild defaultOpen={depth !== 1}>
      <section
        role="group"
        aria-labelledby={id}
        className={cn(
          'min-w-0',
          depth === 0
            ? 'rounded-xl border border-border bg-background/50'
            : 'border-t border-border-subtle',
        )}
      >
        <div className="flex items-center gap-2 px-3 py-2">
          <Heading className="min-w-0 flex-1 text-sm font-medium">
            <Collapsible.Trigger
              type="button"
              aria-labelledby={id}
              aria-describedby={`${id}-count`}
              className="group flex w-full items-center gap-2 rounded-md py-1 text-left hover:text-primary focus-visible:outline-2 focus-visible:outline-ring"
            >
              <ChevronDown
                aria-hidden="true"
                className="size-4 shrink-0 -rotate-90 transition-transform duration-200 group-data-[state=open]:rotate-0 motion-reduce:transition-none"
              />
              <span id={id}>{group.label}</span>
              <span
                id={`${id}-count`}
                className="ml-auto shrink-0 text-xs font-normal text-muted-foreground tabular-nums"
                title={`已选 ${String(count)} 项，共 ${String(group.actions.length)} 项`}
              >
                {count}/{group.actions.length}
              </span>
            </Collapsible.Trigger>
          </Heading>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            aria-label={`${selectionAction}${label}权限`}
            disabled={busy || group.actions.length === 0}
            onClick={() => {
              onToggleAll(group.actions);
            }}
          >
            {selectionAction}
          </Button>
        </div>
        <Collapsible.Content className="@container px-3 pb-3">
          {group.children ? (
            group.children.map((child) => (
              <ActionGroupSection
                key={child.value}
                group={child}
                selected={selected}
                busy={busy}
                onToggleAll={onToggleAll}
                onToggle={onToggle}
                depth={depth + 1}
                parentLabel={label}
              />
            ))
          ) : (
            <fieldset disabled={busy} className="grid min-w-0 gap-1 @sm:grid-cols-2">
              <legend className="sr-only">{label}</legend>
              {group.actions.map((action) => (
                <label
                  key={action.value}
                  htmlFor={`permission-${action.value}`}
                  aria-label={action.label}
                  className="flex min-w-0 items-start gap-2 rounded-lg p-2 hover:bg-muted/60"
                >
                  <Checkbox
                    id={`permission-${action.value}`}
                    className="mt-1"
                    checked={selected.includes(action.value)}
                    aria-describedby={`permission-description-${action.value}`}
                    onChange={(event) => {
                      onToggle(action.value, event.target.checked);
                    }}
                  />
                  <span className="min-w-0 space-y-1">
                    <span className="block text-sm font-medium">{action.label}</span>
                    <span
                      id={`permission-description-${action.value}`}
                      className="block text-xs text-muted-foreground"
                    >
                      {action.description}
                    </span>
                  </span>
                </label>
              ))}
            </fieldset>
          )}
        </Collapsible.Content>
      </section>
    </Collapsible.Root>
  );
}

function ActionForm({
  workspaceID,
  workspaceName,
  user,
  catalog,
  current,
  creating,
  onSaved,
  onClose,
}: {
  workspaceID: string;
  workspaceName: string;
  user: AccessUser;
  catalog: Action[];
  current: UserActions;
  creating: boolean;
  onSaved: () => void;
  onClose: () => void;
}) {
  const client = useQueryClient();
  const [selected, setSelected] = useState(current.actions);
  const [revision] = useState(current.revision);
  const actions = catalog.filter((action) => action.scope === 'workspace');
  const groups = getActionGroups(actions);
  const allSelected = (entries: Action[]) => entries.every(({ value }) => selected.includes(value));
  const selectAll = (entries: Action[]) => {
    setSelected((previous) => [...new Set([...previous, ...entries.map(({ value }) => value)])]);
  };
  const toggleAll = (entries: Action[]) => {
    const values = new Set(entries.map(({ value }) => value));
    setSelected((previous) =>
      entries.every(({ value }) => previous.includes(value))
        ? previous.filter((value) => !values.has(value))
        : [...new Set([...previous, ...values])],
    );
  };
  const mutation = useApiMutation({
    mutationFn: async () => {
      await setUserActions(workspaceID, user.id, { actions: selected, revision });
      await Promise.all([
        client.invalidateQueries({ queryKey: ['management', 'access'] }),
        client.invalidateQueries({ queryKey: ['management', 'workspaces'] }),
        client.invalidateQueries({ queryKey: ['management', 'audit'] }),
        client.invalidateQueries({ queryKey: ['workspace'] }),
      ]);
    },
    onSuccess: onSaved,
  });
  return (
    <Drawer
      title={creating ? '添加操作权限' : '配置操作权限'}
      truncateDescription
      description={`${workspaceName} · ${formatIdentity(user.display_name, user.username)}`}
      busy={mutation.isPending}
      onClose={onClose}
      form={{
        onSubmit: (event) => {
          event.preventDefault();
          if (mutation.isPending || (creating && selected.length === 0)) return;
          mutation.mutate();
        },
      }}
      footerDismiss={
        <Button type="button" variant="cancel" disabled={mutation.isPending} onClick={onClose}>
          取消
        </Button>
      }
      footer={
        <>
          <div className="mr-auto flex flex-wrap items-center gap-3">
            <Button
              type="button"
              variant="outline"
              disabled={mutation.isPending || allSelected(actions)}
              onClick={() => {
                selectAll(actions);
              }}
            >
              全选
            </Button>
            <Button
              type="button"
              variant="outline"
              disabled={mutation.isPending || selected.length === 0}
              onClick={() => {
                setSelected([]);
              }}
            >
              取消全选
            </Button>
          </div>

          <Button
            type="submit"
            loading={mutation.isPending}
            disabled={creating && selected.length === 0}
          >
            <Save aria-hidden="true" />
            保存权限
          </Button>
        </>
      }
    >
      <div className="space-y-4 p-6">
        <p className="text-sm text-muted-foreground">
          选择该用户在当前工作空间可以执行的操作。
          {creating ? '至少选择一项权限，保存后用户将出现在权限列表中。' : null}
        </p>
        {groups.map((group) => (
          <ActionGroupSection
            key={group.value}
            group={group}
            selected={selected}
            busy={mutation.isPending}
            onToggleAll={toggleAll}
            onToggle={(value, checked) => {
              setSelected((previous) =>
                checked
                  ? [...new Set([...previous, value])]
                  : previous.filter((entry) => entry !== value),
              );
            }}
          />
        ))}
        {actions.length === 0 ? (
          <p className="text-sm text-muted-foreground">暂无可配置的操作权限。</p>
        ) : null}
      </div>
    </Drawer>
  );
}

export function ActionEditor({
  workspaceID,
  workspaceName,
  user,
  creating = false,
  onClose,
  onSaved,
}: {
  workspaceID: string;
  workspaceName: string;
  user: AccessUser;
  creating?: boolean;
  onClose: () => void;
  onSaved: () => void;
}) {
  const catalog = useQuery({ ...actionCatalogOptions, refetchOnMount: false });
  const permissions = useQuery({
    ...userActionsOptions(workspaceID, user.id),
    refetchOnMount: false,
  });
  const retry = useApiAction();
  if (catalog.data && permissions.data && !retry.pending) {
    return (
      <ActionForm
        workspaceID={workspaceID}
        workspaceName={workspaceName}
        user={user}
        catalog={catalog.data.items}
        current={permissions.data}
        creating={creating}
        onSaved={onSaved}
        onClose={onClose}
      />
    );
  }
  return (
    <Drawer
      title="配置操作权限"
      truncateDescription
      description={`${workspaceName} · ${formatIdentity(user.display_name, user.username)}`}
      onClose={onClose}
      footerRefresh={
        retry.pending || catalog.error || permissions.error ? (
          <Button
            variant="outline"
            loading={retry.pending}
            onClick={() => {
              void retry.run('retry', () =>
                Promise.all([catalog.refetch(), permissions.refetch()]),
              );
            }}
          >
            <RotateCw aria-hidden="true" />
            重新加载
          </Button>
        ) : undefined
      }
    >
      {retry.pending || catalog.error || permissions.error ? (
        <div className="space-y-3 p-6">
          <ErrorMessage>{catalog.error?.message ?? permissions.error?.message}</ErrorMessage>
        </div>
      ) : (
        <p role="status" className="p-6 text-sm text-muted-foreground">
          正在加载权限…
        </p>
      )}
    </Drawer>
  );
}
