import { diffWordsWithSpace } from 'diff';
import { useMemo } from 'react';

import type { Application, ApplicationVersion } from '@/features/project/application-api';
import { getConfig } from '@/lib/config';
import { formatScore, formatValue } from '@/lib/format';
import { cn } from '@/lib/utils';

type Change = 'added' | 'removed';
type Detail = Application['details'][number];
type PreviousDetail = ApplicationVersion['details'][number];

function decimalScore(value: number) {
  const [coefficient = '0', exponent = '0'] = String(value).split('e');
  const [whole = '0', fraction = ''] = coefficient.split('.');
  return { coefficient: BigInt(`${whole}${fraction}`), scale: fraction.length - Number(exponent) };
}

function scoreDifference(before: number, after: number) {
  // 对十进制显示值做整数减法，避免 20 - 18.2 显示浮点尾差，保留子项原精度。
  const oldScore = decimalScore(before);
  const newScore = decimalScore(after);
  const scale = Math.max(oldScore.scale, newScore.scale, 0);
  const difference =
    newScore.coefficient * 10n ** BigInt(scale - newScore.scale) -
    oldScore.coefficient * 10n ** BigInt(scale - oldScore.scale);
  if (difference === 0n) return null;
  const digits = (difference < 0n ? -difference : difference).toString().padStart(scale + 1, '0');
  const value =
    scale > 0
      ? `${digits.slice(0, -scale)}.${digits.slice(-scale)}`.replace(/\.?0+$/u, '')
      : digits;
  return `${difference > 0n ? '+' : '-'}${value}`;
}

function ScoreDiff({
  before,
  after,
  change,
  total = false,
}: {
  before: number | null;
  after: number | null;
  change?: Change | undefined;
  total?: boolean;
}) {
  const format = total ? formatScore : formatValue;
  const value = format(change === 'removed' ? before : after);
  const difference = before !== null && after !== null ? scoreDifference(before, after) : null;
  const note =
    change || before === after
      ? null
      : (difference ?? (before === null ? '原未填' : `原 ${format(before)}`));
  return (
    <span className="inline-flex items-baseline gap-1 whitespace-nowrap">
      {change === 'added' ? (
        <ins>{value}</ins>
      ) : change === 'removed' ? (
        <del>{value}</del>
      ) : (
        <span>{value}</span>
      )}
      {note ? (
        <span
          className={
            after === null || difference?.startsWith('-')
              ? 'text-danger-foreground'
              : 'text-success-foreground'
          }
        >
          ({note})
        </span>
      ) : null}
    </span>
  );
}

function RevisionValue({ value, change }: { value: string; change?: Change | undefined }) {
  const text = change === 'added' ? <ins>{value}</ins> : change ? <del>{value}</del> : value;
  return <div className="content-revision">{text}</div>;
}

function TextRevision({
  before,
  after,
  change,
}: {
  before: string;
  after: string;
  change?: Change | undefined;
}) {
  const changes = useMemo(() => {
    if (change || before === after) return null;
    // 以 Word 修订形式在原文内标记增删，超时仍保留完整前后内容。
    return (
      diffWordsWithSpace(before, after, { timeout: getConfig().auditDiffTimeoutMs }) ?? [
        { value: before, added: false, removed: true },
        { value: after, added: true, removed: false },
      ]
    );
  }, [before, after, change]);
  if (change) return <RevisionValue value={change === 'added' ? after : before} change={change} />;
  if (!changes) return <p className="leading-5 wrap-anywhere whitespace-pre-wrap">{after}</p>;
  return (
    <div className="content-revision">
      {changes.map((part, index) =>
        part.added ? (
          <ins key={index}>{part.value}</ins>
        ) : part.removed ? (
          <del key={index}>{part.value}</del>
        ) : (
          <span key={index}>{part.value}</span>
        ),
      )}
    </div>
  );
}

function FileDiff({
  before,
  after,
  change,
}: {
  before: { filename: string }[];
  after: { filename: string }[];
  change?: Change | undefined;
}) {
  // 文件只比较名称和同名数量，不因状态、大小、编号或排序变化产生差异。
  const remaining = before.map((file) => file.filename);
  const entries = after.map((file) => {
    const index = remaining.indexOf(file.filename);
    if (index >= 0) remaining.splice(index, 1);
    return { name: file.filename, change: index < 0 ? ('added' as const) : undefined };
  });
  return (
    <div aria-label="附件名称差异" className="space-y-1 text-xs">
      {change
        ? (change === 'added' ? after : before).map((file, index) => (
            <RevisionValue key={index} value={file.filename} change={change} />
          ))
        : [...remaining.map((name) => ({ name, change: 'removed' as const })), ...entries].map(
            (file, index) => <RevisionValue key={index} value={file.name} change={file.change} />,
          )}
    </div>
  );
}

function DetailDiff({
  before,
  after,
  index,
}: {
  before?: PreviousDetail | undefined;
  after?: Detail | undefined;
  index: number;
}) {
  const item = after ?? before;
  if (!item) return null;
  const change: Change | undefined = !before ? 'added' : !after ? 'removed' : undefined;
  return (
    <li
      aria-label={`${change === 'added' ? '新增子项' : change === 'removed' ? '删除子项' : '子项'}：${item.name}`}
      className={cn(
        'space-y-3 rounded-xl border p-5',
        change === 'added' && 'border-success-foreground/30 bg-success text-success-foreground',
        change === 'removed' && 'border-danger-foreground/30 bg-danger text-danger-foreground',
        !change && 'border-border',
      )}
    >
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0 space-y-1">
          <p className="text-xs">
            {change === 'added'
              ? '新增子项'
              : change === 'removed'
                ? '删除子项'
                : `子项 ${String(index + 1)}`}
          </p>
          <h4 className="content-revision text-sm font-medium">
            {change === 'added' ? (
              <ins>{item.name}</ins>
            ) : change === 'removed' ? (
              <del>{item.name}</del>
            ) : (
              item.name
            )}
          </h4>
        </div>
        <dl className="shrink-0 space-y-1 text-right">
          <dt className="text-xs text-muted-foreground">分数</dt>
          <dd aria-label="子项分数差异" className="tabular-nums">
            <ScoreDiff
              before={before?.score ?? null}
              after={after?.score ?? null}
              change={change}
            />
          </dd>
        </dl>
      </div>
      <div aria-label="子项内容差异">
        <TextRevision
          before={formatValue(before?.description)}
          after={formatValue(after?.description)}
          change={change}
        />
      </div>
      <FileDiff
        before={before?.attachments ?? []}
        after={after?.attachments ?? []}
        change={change}
      />
    </li>
  );
}

export function ApplicationContentDiff({
  record,
  previous,
}: {
  record: Application;
  previous: ApplicationVersion;
}) {
  // 优先按稳定代码关联；旧审计缺少代码时按子项编号关联，名称不能充当标识。
  const remaining = [...previous.details];
  const details = record.details.map((after) => {
    const index = remaining.findIndex((before) =>
      before.detail_code ? before.detail_code === after.key : before.id === after.id,
    );
    const before = index >= 0 ? remaining.splice(index, 1)[0] : undefined;
    return { before, after };
  });
  return (
    <div aria-label="申报内容差异" className="space-y-5 border-t border-border pt-5 text-sm">
      <p className="text-xs text-muted-foreground">
        历史版本 {String(previous.revision)} → 当前版本 {String(record.revision)}
        <span className="ml-3 text-danger-foreground line-through">删除内容</span>
        <span className="ml-3 text-success-foreground underline">新增内容</span>
      </p>
      <dl className="space-y-4">
        <div className="space-y-1">
          <dt className="text-xs text-muted-foreground">申报说明</dt>
          <dd aria-label="申报说明差异">
            <TextRevision
              before={formatValue(previous.statement)}
              after={formatValue(record.statement)}
            />
          </dd>
        </div>
        <div className="grid grid-cols-2 gap-6">
          <div className="space-y-1">
            <dt className="text-xs text-muted-foreground">排名</dt>
            <dd aria-label="排名差异" className="tabular-nums">
              <TextRevision before={formatValue(previous.rank)} after={formatValue(record.rank)} />
            </dd>
          </div>
          <div className="space-y-1">
            <dt className="text-xs text-muted-foreground">总分</dt>
            <dd aria-label="总分差异" className="tabular-nums">
              <ScoreDiff before={previous.score} after={record.score} total />
            </dd>
          </div>
        </div>
      </dl>
      <section className="space-y-3 border-t border-border pt-4" aria-label="评分子项差异">
        <h3 className="text-sm font-medium">评分子项</h3>
        <ol className="space-y-3">
          {details.map(({ before, after }, index) => (
            <DetailDiff key={after.key} before={before} after={after} index={index} />
          ))}
          {remaining.map((before, index) => (
            <DetailDiff key={`removed:${String(before.id)}`} before={before} index={index} />
          ))}
        </ol>
        {details.length + remaining.length === 0 ? (
          <p className="text-sm text-muted-foreground">暂无评分子项</p>
        ) : null}
      </section>
    </div>
  );
}
