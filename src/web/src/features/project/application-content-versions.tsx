import { useQuery } from '@tanstack/react-query';
import { History } from 'lucide-react';
import { type ComponentProps, useState } from 'react';

import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Label } from '@/components/ui/label';
import { SearchSelect } from '@/components/ui/search-select';
import {
  type ApplicationVersion,
  applicationVersionsOptions,
} from '@/features/project/application-api';
import { ApplicationContentView } from '@/features/project/application-content';
import { ApplicationContentDiff } from '@/features/project/application-content-diff';
import { formatDate, formatValue } from '@/lib/format';

export function ApplicationContentVersions({
  record,
  files,
  active,
}: ComponentProps<typeof ApplicationContentView> & { active: boolean }) {
  const [comparing, setComparing] = useState(record.status === 'pending');
  const [selectedRevision, setSelectedRevision] = useState('');
  const query = useQuery({
    ...applicationVersionsOptions(files.workspaceID, files.scope, record),
    enabled: comparing && active,
  });
  // 仅读取最近历史版及截止边界版本，版本或截止配置变化后使用独立缓存重新读取。
  const byRevision = new Map<number, ApplicationVersion>();
  for (const version of query.data?.items ?? []) {
    if (version.revision < record.revision && !byRevision.has(version.revision)) {
      byRevision.set(version.revision, version);
    }
  }
  const versions = [...byRevision.values()].sort((a, b) => b.revision - a.revision);
  const deadline = record.self_apply_deadline;
  const firstBeforeDeadline = deadline
    ? versions.find((item) => new Date(item.updated_at).getTime() < new Date(deadline).getTime())
    : undefined;
  const defaultVersion = deadline ? (firstBeforeDeadline ?? versions.at(-1)) : versions[0];
  const previous =
    versions.find((item) => String(item.revision) === selectedRevision) ?? defaultVersion;
  return (
    <>
      <div className="space-y-3 border-t border-border pt-4">
        <Label className="flex w-fit cursor-pointer items-center gap-2 font-normal">
          <Checkbox
            checked={comparing}
            onChange={(event) => {
              if (event.target.checked) setSelectedRevision('');
              setComparing(event.target.checked);
            }}
          />
          对比历史版本
        </Label>
        {comparing ? (
          <div className="space-y-3">
            {versions.length > 0 ? (
              <div className="space-y-2">
                <Label
                  htmlFor="application-history-version"
                  className="text-xs text-muted-foreground"
                >
                  历史版本
                </Label>
                <SearchSelect
                  id="application-history-version"
                  label="历史版本"
                  className="w-full"
                  value={String(previous?.revision ?? '')}
                  defaultValue={String(defaultVersion?.revision ?? '')}
                  options={versions.map((item) => ({
                    value: String(item.revision),
                    label: `版本 ${String(item.revision)} · ${formatDate(item.updated_at)} · ${formatValue(item.actor_name)}`,
                    pinned: true,
                  }))}
                  {...(deadline
                    ? {
                        optionSeparator: {
                          beforeValue: firstBeforeDeadline
                            ? String(firstBeforeDeadline.revision)
                            : null,
                          label: `申报截止：${formatDate(deadline)}`,
                        },
                      }
                    : {})}
                  loading={query.isFetching}
                  disabled={query.isError}
                  required
                  onValueChange={setSelectedRevision}
                />
              </div>
            ) : query.isPending ? (
              <p role="status" className="text-xs text-muted-foreground">
                正在读取历史版本…
              </p>
            ) : !query.error ? (
              <p className="text-xs text-muted-foreground">暂无可对比的历史版本</p>
            ) : null}
            {query.error ? (
              <Button
                variant="ghost"
                size="sm"
                loading={query.isFetching}
                onClick={() => {
                  void query.refetch();
                }}
              >
                <History aria-hidden="true" />
                重试读取历史版本
              </Button>
            ) : null}
          </div>
        ) : null}
      </div>
      {comparing && previous && !query.error ? (
        <ApplicationContentDiff record={record} previous={previous} />
      ) : (
        <ApplicationContentView record={record} files={files} />
      )}
    </>
  );
}
