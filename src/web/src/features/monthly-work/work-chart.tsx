import { useQuery } from '@tanstack/react-query';
import { ClipboardCheck, UsersRound } from 'lucide-react';

import { MetricCard } from '@/components/metric-card';
import type { WorkRecord } from '@/features/monthly-work/api';
import { currentUserOptions } from '@/features/user/api';

export function WorkChart({ records }: { records: WorkRecord[] | undefined }) {
  // 复用鉴权入口已读取的登录态，不额外请求用户接口。
  const { data: user } = useQuery({ ...currentUserOptions, enabled: false });

  // 分别统计当前筛选结果中本人负责和参与的工作，兼任时分别计入。
  const owned = user ? records?.filter((record) => record.owner_id === user.id).length : undefined;
  const participating = user
    ? records?.filter((record) => record.participant_ids.includes(user.id)).length
    : undefined;

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
