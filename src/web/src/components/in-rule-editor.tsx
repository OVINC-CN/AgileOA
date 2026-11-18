import { FolderPlus, Plus, Trash2 } from 'lucide-react';
import { useId } from 'react';

import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { SearchSelect } from '@/components/ui/search-select';
import { TagInput } from '@/components/ui/tag-input';

export type InRuleNode<Field extends string = string> =
  | { type: 'group'; logic: 'and' | 'or'; children: InRuleNode<Field>[] }
  | { type: 'condition'; field: Field; operator: 'in'; values: string[] };

export interface InRuleField<Field extends string = string> {
  value: Field;
  label: string;
  max_length: number;
}

export interface InRuleLimits {
  maxDepth: number;
  maxNodes: number;
  maxValues: number;
}

function inRuleSize(node: InRuleNode | null): { nodes: number; depth: number } {
  if (node === null) return { nodes: 0, depth: 0 };
  if (node.type === 'condition') return { nodes: 1, depth: 1 };
  const children = node.children.map(inRuleSize);
  return {
    nodes: 1 + children.reduce((total, child) => total + child.nodes, 0),
    depth: 1 + Math.max(0, ...children.map((child) => child.depth)),
  };
}

export function InRuleEditor<Field extends string>({
  rule,
  fields,
  limits,
  readOnly = false,
  onChange,
}: {
  rule: InRuleNode<Field> | null;
  fields: readonly InRuleField<Field>[];
  limits: InRuleLimits;
  readOnly?: boolean;
  onChange: (rule: InRuleNode<Field> | null) => void;
}) {
  const count = inRuleSize(rule).nodes;
  const newCondition = (): InRuleNode<Field> => {
    const field = fields[0];
    if (!field) throw new Error('未配置条件字段');
    return { type: 'condition', field: field.value, operator: 'in', values: [] };
  };
  const newGroup = (): InRuleNode<Field> => ({
    type: 'group',
    logic: 'and',
    children: [newCondition()],
  });
  return rule ? (
    <InRuleNodeEditor
      node={rule}
      fields={fields}
      limits={limits}
      readOnly={readOnly}
      depth={1}
      count={count}
      newCondition={newCondition}
      newGroup={newGroup}
      onChange={onChange}
    />
  ) : (
    <div className="space-y-3 rounded-lg border border-dashed p-4">
      <p className="text-sm text-muted-foreground">未配置条件</p>
      {!readOnly ? (
        <div className="flex flex-wrap gap-2">
          <Button
            type="button"
            variant="outline"
            onClick={() => {
              onChange(newCondition());
            }}
          >
            <Plus aria-hidden="true" />
            添加条件
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={limits.maxDepth < 2 || limits.maxNodes < 2}
            onClick={() => {
              onChange(newGroup());
            }}
          >
            <FolderPlus aria-hidden="true" />
            添加条件组
          </Button>
        </div>
      ) : null}
    </div>
  );
}

function InRuleNodeEditor<Field extends string>({
  node,
  fields,
  limits,
  readOnly,
  depth,
  count,
  newCondition,
  newGroup,
  onChange,
}: {
  node: InRuleNode<Field>;
  fields: readonly InRuleField<Field>[];
  limits: InRuleLimits;
  readOnly: boolean;
  depth: number;
  count: number;
  newCondition: () => InRuleNode<Field>;
  newGroup: () => InRuleNode<Field>;
  onChange: (rule: InRuleNode<Field> | null) => void;
}) {
  const id = useId();
  const label = node.type === 'group' ? '条件组' : '条件';
  const canAdd = !readOnly && depth < limits.maxDepth && count < limits.maxNodes;
  return (
    <section
      aria-label={label}
      className="min-w-0 space-y-3 rounded-lg border border-border bg-background/55 p-3"
    >
      <div className="flex flex-wrap items-center gap-2">
        {node.type === 'group' ? (
          <>
            <span className="text-sm font-medium">条件组</span>
            <SearchSelect
              label="组合逻辑"
              className="min-w-0 flex-1"
              options={[
                { value: 'and', label: '全部满足（AND）' },
                { value: 'or', label: '任一满足（OR）' },
              ]}
              value={node.logic}
              disabled={readOnly}
              onValueChange={(logic) => {
                if (logic === 'and' || logic === 'or') onChange({ ...node, logic });
              }}
            />
          </>
        ) : (
          <>
            <SearchSelect
              label="条件字段"
              className="min-w-0 flex-1"
              options={[...fields]}
              value={node.field}
              disabled={readOnly}
              onValueChange={(value) => {
                const field = fields.find((item) => item.value === value);
                if (field) onChange({ ...node, field: field.value });
              }}
            />
            <span className="rounded-md bg-secondary px-2 py-1 text-xs">in</span>
          </>
        )}
        {!readOnly ? (
          <Button
            type="button"
            size="icon-sm"
            variant="ghost"
            aria-label={`删除${label}`}
            onClick={() => {
              onChange(null);
            }}
          >
            <Trash2 aria-hidden="true" />
          </Button>
        ) : null}
      </div>
      {node.type === 'group' ? (
        <>
          <div className="space-y-3 border-l-2 border-primary/20 pl-3">
            {node.children.map((child, index) => (
              <InRuleNodeEditor
                key={index}
                node={child}
                fields={fields}
                limits={limits}
                readOnly={readOnly}
                depth={depth + 1}
                count={count}
                newCondition={newCondition}
                newGroup={newGroup}
                onChange={(next) => {
                  onChange({
                    ...node,
                    children:
                      next === null
                        ? node.children.filter((_, position) => position !== index)
                        : node.children.map((item, position) => (position === index ? next : item)),
                  });
                }}
              />
            ))}
            {!node.children.length ? (
              <p className="text-xs text-muted-foreground">添加条件或子组后可保存</p>
            ) : null}
          </div>
          {!readOnly ? (
            <div className="flex flex-wrap gap-2">
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={!canAdd}
                onClick={() => {
                  onChange({ ...node, children: [...node.children, newCondition()] });
                }}
              >
                <Plus aria-hidden="true" />
                添加条件
              </Button>
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={!canAdd || depth + 2 > limits.maxDepth || count + 2 > limits.maxNodes}
                onClick={() => {
                  onChange({ ...node, children: [...node.children, newGroup()] });
                }}
              >
                <FolderPlus aria-hidden="true" />
                添加子组
              </Button>
            </div>
          ) : null}
        </>
      ) : (
        <div className="space-y-2">
          <Label htmlFor={`${id}-values`}>候选值</Label>
          <TagInput
            id={`${id}-values`}
            label={`${fields.find(({ value }) => value === node.field)?.label ?? ''}候选值`}
            readOnly={readOnly}
            maxLength={fields.find(({ value }) => value === node.field)?.max_length ?? 100}
            maxValues={limits.maxValues}
            values={node.values}
            onValuesChange={(values) => {
              onChange({ ...node, values });
            }}
          />
        </div>
      )}
    </section>
  );
}
