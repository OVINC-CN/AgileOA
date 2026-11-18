import { UsersRound } from 'lucide-react';

import { MetricCard } from '@/components/metric-card';
import type { PersonStatistics } from '@/features/cpc-person/api';
import { formatNumber } from '@/lib/format';

export function PersonChart({ data }: { data: PersonStatistics | undefined }) {
  const count = (value: string) => data?.stages.find((stage) => stage.value === value)?.count;
  const probationary = count('probationary_member');
  const full = count('full_member');
  return (
    <div aria-label="党建人员统计" className="grid grid-cols-2 gap-4 md:grid-cols-4">
      <MetricCard
        title="党员人数"
        value={data ? (probationary ?? 0) + (full ?? 0) : undefined}
        note={
          data
            ? `预备党员 ${formatNumber(probationary ?? 0)} 人，正式党员 ${formatNumber(full ?? 0)} 人`
            : '预备党员与正式党员合计'
        }
        icon={UsersRound}
        valueSize="large"
      />
      <MetricCard
        title="发展对象"
        value={count('development_candidate')}
        note="当前筛选结果"
        icon={UsersRound}
        valueSize="large"
      />
      <MetricCard
        title="入党积极分子"
        value={count('activist')}
        note="当前筛选结果"
        icon={UsersRound}
        valueSize="large"
      />
      <MetricCard
        title="入党申请人"
        value={count('applicant')}
        note="当前筛选结果"
        icon={UsersRound}
        valueSize="large"
      />
    </div>
  );
}
