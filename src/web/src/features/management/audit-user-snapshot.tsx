import { isLosslessNumber } from 'lossless-json';

import { DetailValue } from '@/components/detail-value';
import { formatValue } from '@/lib/format';

const fieldLabels: Record<string, string> = {
  id: '账号 ID',
  student_id: '学生 ID',
  identity_type: '身份类型',
  workspace_id: '所属工作空间',
  username: '用户名',
  display_name: '显示名称',
  name: '姓名',
  student_no: '学号',
  email: '邮箱',
  is_admin: '系统管理员',
};
const groups = [
  { title: '身份标识', fields: ['id', 'student_id', 'identity_type', 'workspace_id'] },
  { title: '账号资料', fields: ['display_name', 'name', 'username', 'student_no', 'email'] },
  { title: '权限状态', fields: ['is_admin'] },
];
const knownFields = new Set(groups.flatMap((group) => group.fields));

function structured(value: unknown): value is Record<string, unknown> | unknown[] {
  return value !== null && typeof value === 'object' && !isLosslessNumber(value);
}

function SnapshotFields({ entries }: { entries: [string, unknown][] }) {
  return (
    <dl className="grid gap-4 sm:grid-cols-2">
      {entries.map(([key, value]) => {
        const nested = structured(value);
        const children = nested
          ? Array.isArray(value)
            ? value.map(
                (item: unknown, index) => [`第 ${String(index + 1)} 项`, item] as [string, unknown],
              )
            : Object.entries(value)
          : [];
        const label = fieldLabels[key] ?? key;
        return (
          <div key={key} className={nested ? 'min-w-0 sm:col-span-2' : 'min-w-0'}>
            <dt className="mb-1.5 text-xs text-muted-foreground">{label}</dt>
            {nested && children.length > 0 ? (
              <dd className="border-l border-border pl-4">
                <SnapshotFields entries={children} />
              </dd>
            ) : (
              <DetailValue
                label={label}
                value={nested ? (Array.isArray(value) ? '[]' : '{}') : snapshotValue(value)}
                copyable={false}
              />
            )}
          </div>
        );
      })}
    </dl>
  );
}

function snapshotValue(value: unknown) {
  if (isLosslessNumber(value)) return value.toString();
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
    return formatValue(value);
  }
  return formatValue(null);
}

export function AuditUserSnapshot({ snapshot }: { snapshot: Record<string, unknown> }) {
  const extra = Object.entries(snapshot).filter(([key]) => !knownFields.has(key));
  const sections = groups.map((group) => ({
    title: group.title,
    entries: group.fields
      .filter((key) => Object.hasOwn(snapshot, key))
      .map((key) => [key, snapshot[key]] as [string, unknown]),
  }));
  if (extra.length > 0) sections.push({ title: '其他资料', entries: extra });
  return (
    <div className="space-y-5">
      {sections
        .filter((group) => group.entries.length > 0)
        .map((group) => (
          <section key={group.title} aria-label={group.title} className="space-y-3">
            <h4 className="text-sm font-medium">{group.title}</h4>
            <SnapshotFields entries={group.entries} />
          </section>
        ))}
      {Object.keys(snapshot).length === 0 ? <p>{formatValue(null)}</p> : null}
    </div>
  );
}
