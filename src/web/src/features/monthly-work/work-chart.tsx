import { ClipboardCheck, UsersRound } from 'lucide-react';

import { MetricCard } from '@/components/metric-card';
import type { WorkRecord } from '@/features/monthly-work/api';

export function WorkChart({
  records,
  userID,
}: {
  records: WorkRecord[] | undefined;
  userID: number | undefined;
}) {
  // 分别统计当前筛选结果中本人负责和参与的工作，兼任时分别计入。
  const owned =
    userID === undefined
      ? undefined
      : records?.filter((record) => record.owner_id === userID).length;
  const participating =
    userID === undefined
      ? undefined
      : records?.filter((record) => record.participant_ids.includes(userID)).length;

  return (
    <div aria-label="月度工作统计" className="grid grid-cols-2 gap-4">
      <MetricCard
        title="我负责的"
        value={owned}
        note="当前筛选结果"
        icon={ClipboardCheck}
        valueSize="large"
      />
      <MetricCard
        title="我参与的"
        value={participating}
        note="当前筛选结果"
        icon={UsersRound}
        valueSize="large"
      />
    </div>
  );
}
