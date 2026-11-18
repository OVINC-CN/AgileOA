import { useQuery } from '@tanstack/react-query';
import { useId, useState } from 'react';

import { DetailSection } from '@/components/detail-section';
import { DetailValue } from '@/components/detail-value';
import { Button } from '@/components/ui/button';
import { Drawer } from '@/components/ui/drawer';
import { DrawerTabs } from '@/components/ui/drawer-tabs';
import { auditDetailOptions, type AuditRow } from '@/features/management/api';
import { formatAuditJSON } from '@/features/management/audit-json';
import { AuditJSONDiff } from '@/features/management/audit-json-diff';
import {
  auditIdentity,
  auditObjectLabel,
  auditWorkspaceLabel,
} from '@/features/management/audit-labels';
import { AuditUserSnapshot } from '@/features/management/audit-user-snapshot';
import { formatDate, formatIdentity, formatValue } from '@/lib/format';
import { useLoadingDelay } from '@/lib/loading';

const tabs = [
  { value: 'event', label: '事件信息' },
  { value: 'changes', label: 'JSON 差异' },
  { value: 'before', label: '变更前' },
  { value: 'after', label: '变更后' },
];

function EventFields({ fields }: { fields: [string, string | null][] }) {
  return (
    <dl className="grid gap-4 sm:grid-cols-2">
      {fields.map(([label, value]) => (
        <div key={label} className="min-w-0">
          <dt className="mb-1.5 text-xs text-muted-foreground">{label}</dt>
          <DetailValue label={label} value={formatValue(value)} copyable={false} />
        </div>
      ))}
    </dl>
  );
}

export function AuditDetail({
  row,
  workspaceLabel,
  onClose,
}: {
  row: AuditRow;
  workspaceLabel: string;
  onClose: () => void;
}) {
  const id = useId();
  const [view, setView] = useState<string | null>(null);
  const query = useQuery({ ...auditDetailOptions(row), refetchOnMount: false });
  const loading = useLoadingDelay(query.isPending);
  const data = query.data;
  const active =
    view ??
    (data && (data.instance_data !== null || data.instance_origin_data !== null)
      ? 'changes'
      : 'event');
  const actor = auditIdentity(row.snapshot_user_info);
  return (
    <Drawer
      title="审计详情"
      description={`${row.event_content ?? row.action_id} · ${formatDate(new Date(row.start_time).toISOString())}`}
      onClose={onClose}
      footerNavigation={
        data ? (
          <DrawerTabs id={id} label="审计详情分区" value={active} items={tabs} onChange={setView} />
        ) : null
      }
      footer={
        <Button variant="outline" onClick={onClose}>
          关闭
        </Button>
      }
    >
      {loading ? (
        <div aria-busy="true" className="min-h-64 p-5" />
      ) : query.error ? (
        <div className="p-5">
          <Button
            loading={query.isFetching}
            onClick={() => {
              void query.refetch();
            }}
          >
            重试
          </Button>
        </div>
      ) : data ? (
        <div
          role="tabpanel"
          id={`${id}-panel-${active}`}
          aria-labelledby={`${id}-tab-${active}`}
          className="p-5"
        >
          {active === 'event' ? (
            <div className="divide-y divide-border">
              <DetailSection title="操作与结果" heading="h3">
                <EventFields
                  fields={[
                    ['操作', data.event_content ?? data.action_id],
                    ['操作代码', data.action_id],
                    ['操作结果', data.result_code === 0 ? '成功' : String(data.result_code)],
                    ['结果说明', data.result_content],
                    ['敏感信息记录', data.has_sensitive_snapshot ? '有' : '无'],
                  ]}
                />
              </DetailSection>
              <DetailSection title="操作用户" heading="h3">
                <EventFields
                  fields={[
                    ['用户', formatIdentity(actor.name, actor.username)],
                    ['用户 ID', data.user_id],
                    ['身份类型', data.user_identify_type === 2 ? '学生' : '组织'],
                    ['用户租户', auditWorkspaceLabel(data.user_identify_tenant_id)],
                  ]}
                />
              </DetailSection>
              <DetailSection title="操作对象" heading="h3">
                <EventFields
                  fields={[
                    ['工作空间', workspaceLabel],
                    ['对象类型', auditObjectLabel(data.resource_type_id)],
                    ['对象 ID', data.instance_id],
                    ['对象名称', data.instance_name],
                  ]}
                />
              </DetailSection>
              <DetailSection title="请求信息" heading="h3">
                <EventFields
                  fields={[
                    ['请求 ID', data.request_id],
                    ['操作来源', data.access_type === 1 ? 'Web' : 'MCP'],
                    ['请求方法', data.extend_data.method],
                    ['请求路径', data.extend_data.path],
                    ['客户端 IP', data.access_source_ip],
                    ['客户端信息', data.access_user_agent],
                  ]}
                />
              </DetailSection>
              <DetailSection title="时间与编号" heading="h3">
                <EventFields
                  fields={[
                    ['ID', data.id],
                    ['事件 ID', data.event_id],
                    ['UTC 分区日期', data.event_date],
                    ['开始时间', formatDate(new Date(data.start_time).toISOString())],
                    ['结束时间', formatDate(new Date(data.end_time).toISOString())],
                  ]}
                />
              </DetailSection>
              <DetailSection title="操作时的用户信息" heading="h3">
                <AuditUserSnapshot snapshot={data.snapshot_user_info} />
              </DetailSection>
            </div>
          ) : active === 'changes' ? (
            <AuditJSONDiff before={data.instance_origin_data} after={data.instance_data} />
          ) : (
            <pre
              aria-label={active === 'before' ? '变更前 JSON' : '变更后 JSON'}
              className="audit-json-code"
            >
              {formatAuditJSON(
                active === 'before' ? data.instance_origin_data : data.instance_data,
              )}
            </pre>
          )}
        </div>
      ) : null}
    </Drawer>
  );
}
