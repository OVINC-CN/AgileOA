import { BookOpen, House, Landmark, UsersRound } from 'lucide-react';

import { MetricCard } from '@/components/metric-card';
import type { StudentStatistics } from '@/features/student/api';

export function StudentChart({ data }: { data: StudentStatistics | undefined }) {
  return (
    <div aria-label="学生统计" className="grid grid-cols-2 gap-4 md:grid-cols-4">
      <MetricCard
        title="学生数量"
        value={data?.total}
        note="当前筛选结果"
        icon={UsersRound}
        valueSize="large"
      />
      <MetricCard
        title="班级数量"
        value={data?.class_count}
        note="不含未填写班级"
        icon={BookOpen}
        valueSize="large"
      />
      <MetricCard
        title="有宗教信仰"
        value={data?.religious_count}
        note="宗教信仰为是的学生"
        icon={Landmark}
        valueSize="large"
      />
      <MetricCard
        title="校外住宿"
        value={data?.off_campus_count}
        note="住宿类型为校外的学生"
        icon={House}
        valueSize="large"
      />
    </div>
  );
}
