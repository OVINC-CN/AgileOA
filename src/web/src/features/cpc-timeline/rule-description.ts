import {
  offsetUnits,
  type TimelineNode,
  type TimelineRule,
  type TimeOffset,
} from '@/features/cpc-timeline/model';

function describeOffset(offset: TimeOffset) {
  // 按实际校验顺序合并同方向偏移，混合正负值时保留先后关系。
  const groups: { positive: boolean; parts: string[] }[] = [];
  for (const { value, label } of offsetUnits) {
    const amount = offset[value];
    if (amount === 0) continue;
    const positive = amount > 0;
    const unit = value === 'months' ? '个月' : value === 'workdays' ? '个工作日' : label;
    const part = `${String(Math.abs(amount))}${unit}`;
    const previous = groups.at(-1);
    if (previous?.positive === positive) {
      previous.parts.push(part);
    } else {
      groups.push({ positive, parts: [part] });
    }
  }
  return (
    groups
      .map(({ positive, parts }, index) =>
        index === 0
          ? `${parts.join('')}${positive ? '后' : '前'}`
          : `再${positive ? '延后' : '提前'}${parts.join('')}`,
      )
      .join('') || '当天'
  );
}

export function timelineRuleRangeLabel(rule: TimelineRule) {
  if (rule.min_offset && !rule.max_offset) return '最早时间';
  if (rule.max_offset && !rule.min_offset) return '最晚时间';
  return '时间范围';
}

export function describeTimelineRule(
  node: TimelineNode,
  rule: TimelineRule,
  nodes: TimelineNode[],
  dates?: { min_date: string; max_date: string },
) {
  // 未完成的编辑状态只展示待补齐提示，不将缺失范围解释为有效规则。
  const nodeName = node.name.trim() || '未命名节点';
  const reference = nodes.find((item) => item.id === rule.reference_node_id);
  if (!reference) return `${nodeName}的参考节点待选择。`;
  const min = rule.min_offset;
  const max = rule.max_offset;
  if (!min && !max) return `${nodeName}的时间范围待设置。`;
  if (
    [min, max].some(
      (offset) => offset && Object.values(offset).some((value) => !Number.isFinite(value)),
    )
  ) {
    return `${nodeName}的时间偏移待填写完整。`;
  }

  // 人员参考日期已填写时直接使用后端计算的边界，避免重复实现日期偏移。
  if (dates?.min_date || dates?.max_date) {
    const range =
      dates.min_date && dates.max_date && dates.min_date !== dates.max_date
        ? `${dates.min_date}至${dates.max_date}`
        : dates.min_date || dates.max_date;
    return `${nodeName}应该在${range}。`;
  }

  // 根据边界生成肯定句，单侧范围由旁边的最早或最晚时间标签区分。
  const referenceName = reference.name.trim() || '未命名节点';
  let range = '';
  if (min && max) {
    range = offsetUnits.every(({ value }) => min[value] === max[value])
      ? describeOffset(min)
      : `${describeOffset(min)}至${describeOffset(max)}`;
  } else if (min) {
    range = describeOffset(min);
  } else if (max) {
    range = describeOffset(max);
  }
  return `${nodeName}应该在${referenceName}的${range}。`;
}
