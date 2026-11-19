import { Drawer } from '@/components/ui/drawer';
import type { Participation } from '@/features/project/api';
import { formatIdentity, formatScore, formatValue } from '@/lib/format';

export function ParticipationDetail({
  record,
  onClose,
}: {
  record: Participation;
  onClose: () => void;
}) {
  return (
    <Drawer
      title="参与评分详情"
      truncateDescription
      description={formatIdentity(record.student_name, record.student_no)}
      onClose={onClose}
    >
      <div className="space-y-6 p-6">
        <dl className="grid grid-cols-2 gap-x-6 gap-y-5">
          <div className="col-span-2 space-y-1.5">
            <dt className="text-xs text-muted-foreground">项目</dt>
            <dd className="text-base font-medium wrap-anywhere">{record.project_name}</dd>
          </div>
          <div className="space-y-1.5">
            <dt className="text-xs text-muted-foreground">归属年份</dt>
            <dd className="text-sm">{record.year}</dd>
          </div>
          <div className="space-y-1.5">
            <dt className="text-xs text-muted-foreground">学号</dt>
            <dd className="text-sm wrap-anywhere">{record.student_no}</dd>
          </div>
          <div className="space-y-1.5">
            <dt className="text-xs text-muted-foreground">姓名</dt>
            <dd className="text-sm wrap-anywhere">{record.student_name}</dd>
          </div>
          <div className="space-y-1.5">
            <dt className="text-xs text-muted-foreground">学历</dt>
            <dd className="text-sm wrap-anywhere">{formatValue(record.education_level)}</dd>
          </div>
          <div className="space-y-1.5">
            <dt className="text-xs text-muted-foreground">专业</dt>
            <dd className="text-sm wrap-anywhere">{formatValue(record.major)}</dd>
          </div>
          <div className="space-y-1.5">
            <dt className="text-xs text-muted-foreground">年级</dt>
            <dd className="text-sm wrap-anywhere">{formatValue(record.grade)}</dd>
          </div>
          <div className="space-y-1.5">
            <dt className="text-xs text-muted-foreground">排名</dt>
            <dd className="text-sm tabular-nums">{formatValue(record.rank)}</dd>
          </div>
          <div className="space-y-1.5">
            <dt className="text-xs text-muted-foreground">总分</dt>
            <dd className="text-lg font-semibold wrap-anywhere tabular-nums">
              {formatScore(record.score)}
            </dd>
          </div>
        </dl>
        <section
          aria-labelledby="participation-details-title"
          className="space-y-4 border-t border-border pt-5"
        >
          <h2 id="participation-details-title" className="text-sm font-medium">
            评分子项（{String(record.details.length)}）
          </h2>
          {record.details.length ? (
            <ol className="divide-y divide-border">
              {record.details.map((item, index) => (
                <li
                  key={`${String(index)}:${item.name}`}
                  className="space-y-4 py-5 first:pt-0 last:pb-0"
                >
                  <div className="flex flex-wrap items-start justify-between gap-4">
                    <div className="min-w-0 flex-1 space-y-1.5">
                      <p className="text-xs text-muted-foreground">子项 {String(index + 1)}</p>
                      <h3 className="text-sm font-medium wrap-anywhere">{item.name}</h3>
                    </div>
                    <dl className="space-y-1.5 text-right">
                      <dt className="text-xs text-muted-foreground">分数</dt>
                      <dd className="text-sm font-medium tabular-nums">
                        {formatValue(item.score)}
                      </dd>
                    </dl>
                  </div>
                  <div className="space-y-1.5">
                    <p className="text-xs text-muted-foreground">子项说明</p>
                    <p className="text-sm leading-7 wrap-anywhere whitespace-pre-wrap">
                      {formatValue(item.description)}
                    </p>
                  </div>
                </li>
              ))}
            </ol>
          ) : (
            <p className="text-sm text-muted-foreground">暂无评分子项</p>
          )}
        </section>
      </div>
    </Drawer>
  );
}
