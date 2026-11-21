import { Plus, Trash2 } from 'lucide-react';
import { useId } from 'react';

import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { SearchSelect } from '@/components/ui/search-select';
import {
  newID,
  offsetUnits,
  type TimelineNode,
  type TimelineRule,
  type TimeOffset,
  zeroOffset,
} from '@/features/cpc-timeline/model';
import {
  describeTimelineRule,
  timelineRuleRangeLabel,
} from '@/features/cpc-timeline/rule-description';
import { getConfig } from '@/lib/config';

function OffsetEditor({
  label,
  value,
  onChange,
}: {
  label: string;
  value: TimeOffset | null;
  onChange: (value: TimeOffset | null) => void;
}) {
  const limit = getConfig().timelineMaxOffset;
  const id = useId();
  return (
    <fieldset className="min-w-0 space-y-3 rounded-lg bg-muted/40 p-3">
      <legend className="text-xs font-medium">{label}（含边界）</legend>
      <Label className="flex items-center gap-2 leading-5 font-normal">
        <Checkbox
          checked={value === null}
          onChange={(event) => {
            onChange(event.target.checked ? null : zeroOffset());
          }}
        />
        不限
      </Label>
      {value ? (
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
          {offsetUnits.map(({ value: unit, label: unitLabel }) => (
            <div key={unit} className="space-y-2">
              <Label className="text-xs leading-4 font-normal" htmlFor={`${id}-${unit}`} required>
                {unitLabel}
              </Label>
              <Input
                id={`${id}-${unit}`}
                aria-label={`${label}偏移${unitLabel}`}
                type="number"
                step={1}
                min={-limit}
                max={limit}
                required
                value={Number.isNaN(value[unit]) ? '' : value[unit]}
                onChange={(event) => {
                  onChange({ ...value, [unit]: event.target.valueAsNumber });
                }}
              />
            </div>
          ))}
        </div>
      ) : null}
    </fieldset>
  );
}

export function RuleEditor({
  node,
  nodes,
  onChange,
}: {
  node: TimelineNode;
  nodes: TimelineNode[];
  onChange: (rules: TimelineRule[]) => void;
}) {
  const references = nodes.filter((item) => item.id !== node.id);
  const update = (id: string, patch: Partial<TimelineRule>) => {
    onChange(node.rules.map((rule) => (rule.id === id ? { ...rule, ...patch } : rule)));
  };
  return (
    <section className="min-w-0 space-y-4" aria-label="节点提示规则">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="text-sm font-medium">时间校验规则</h3>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={!references.length || node.rules.length >= getConfig().timelineMaxRulesPerNode}
          onClick={() => {
            const reference = references[0];
            if (!reference) return;
            onChange([
              ...node.rules,
              {
                id: newID('rule'),
                reference_node_id: reference.id,
                min_offset: zeroOffset(),
                max_offset: null,
              },
            ]);
          }}
        >
          <Plus aria-hidden="true" />
          添加规则
        </Button>
      </div>
      <p className="text-xs leading-5 text-muted-foreground">
        {references.length
          ? '超出时间范围仅提示，不影响人员数据保存。'
          : '添加其他节点后可配置时间提示规则。'}
      </p>
      {node.rules.length > 0 ? (
        <p className="text-xs leading-5 text-muted-foreground">
          负数表示提前，正数表示延后，0
          表示当天。依次按年、月、天、工作日偏移，月末取目标月最后一天；工作日按周一至周五计算，不含节假日和调休修正。
        </p>
      ) : null}
      {node.rules.map((rule, index) => (
        <fieldset
          key={rule.id}
          className="min-w-0 space-y-4 rounded-xl border border-border-subtle p-4"
        >
          <div className="flex items-center justify-between gap-2">
            <span className="text-xs font-medium">规则 {String(index + 1)} · 仅提示</span>
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              aria-label={`删除规则 ${String(index + 1)}`}
              onClick={() => {
                onChange(node.rules.filter((item) => item.id !== rule.id));
              }}
            >
              <Trash2 aria-hidden="true" />
            </Button>
          </div>
          <div className="space-y-2">
            <Label htmlFor={`reference-${rule.id}`} required className="leading-5">
              参考节点
            </Label>
            <SearchSelect
              id={`reference-${rule.id}`}
              label={`规则 ${String(index + 1)} 参考节点`}
              required
              className="w-full"
              value={rule.reference_node_id}
              options={references.map((item) => ({
                value: item.id,
                label: item.name || '未命名节点',
              }))}
              onValueChange={(value) => {
                update(rule.id, { reference_node_id: value });
              }}
            />
          </div>
          <p className="text-xs text-muted-foreground">范围以参考节点的发生时间为基准</p>
          <div className="grid gap-4 md:grid-cols-2">
            <OffsetEditor
              label="最早时间"
              value={rule.min_offset}
              onChange={(value) => {
                update(rule.id, { min_offset: value });
              }}
            />
            <OffsetEditor
              label="最晚时间"
              value={rule.max_offset}
              onChange={(value) => {
                update(rule.id, { max_offset: value });
              }}
            />
          </div>
          <div className="min-w-0 space-y-2">
            <p className="text-xs font-medium">提示文案预览</p>
            <output
              aria-label={`规则 ${String(index + 1)} 提示文案预览`}
              className="block rounded-lg bg-warning/60 p-3 text-sm leading-5 wrap-anywhere text-warning-foreground"
            >
              <span className="mr-2 text-xs font-medium">{timelineRuleRangeLabel(rule)}</span>
              {describeTimelineRule(node, rule, nodes)}
            </output>
          </div>
        </fieldset>
      ))}
    </section>
  );
}
