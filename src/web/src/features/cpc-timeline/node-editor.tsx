import { Plus, Trash2 } from 'lucide-react';
import type { ReactNode } from 'react';

import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { SearchSelect } from '@/components/ui/search-select';
import { Textarea } from '@/components/ui/textarea';
import {
  fieldTypes,
  newID,
  studentFieldOptions,
  type TimelineField,
  type TimelineNode,
} from '@/features/cpc-timeline/model';
import { RuleEditor } from '@/features/cpc-timeline/rule-editor';
import { getConfig } from '@/lib/config';

export function NodeEditor({
  node,
  nodes,
  onChange,
  attachments,
}: {
  node: TimelineNode;
  nodes: TimelineNode[];
  onChange: (node: TimelineNode) => void;
  attachments?: ReactNode;
}) {
  const changeField = (id: string, patch: Partial<TimelineField>) => {
    onChange({
      ...node,
      fields: node.fields.map((field) => (field.id === id ? { ...field, ...patch } : field)),
    });
  };
  return (
    <div className="min-w-0 space-y-5">
      <Label className="block space-y-2">
        <span>节点名称</span>
        <Input
          aria-label="节点名称"
          required
          maxLength={100}
          value={node.name}
          onChange={(event) => {
            onChange({ ...node, name: event.target.value });
          }}
        />
      </Label>
      <div className="space-y-2">
        <Label htmlFor={`group-${node.id}`}>节点分组</Label>
        <SearchSelect
          id={`group-${node.id}`}
          label="节点分组"
          className="w-full"
          allowCreate
          maxLength={100}
          value={node.group_name}
          options={[
            { value: '', label: '不分组' },
            ...[...new Set(nodes.map((item) => item.group_name.trim()).filter(Boolean))].map(
              (name) => ({ value: name, label: name }),
            ),
          ]}
          onValueChange={(value) => {
            onChange({ ...node, group_name: value });
          }}
        />
        <p className="text-xs text-muted-foreground">选择已有分组或输入新名称。</p>
      </div>
      <Label className="block space-y-2">
        <span>节点说明</span>
        <Textarea
          aria-label="节点说明"
          maxLength={1000}
          value={node.description}
          onChange={(event) => {
            onChange({ ...node, description: event.target.value });
          }}
        />
      </Label>
      <Label className="flex items-center gap-2 text-sm">
        <Checkbox
          checked={node.is_baseline}
          onChange={(event) => {
            onChange({ ...node, is_baseline: event.target.checked });
          }}
        />
        基准节点
      </Label>
      {attachments}
      <section className="space-y-3" aria-label="节点字段">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <h3 className="text-sm font-semibold">字段配置</h3>
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={node.fields.length >= getConfig().timelineMaxFieldsPerNode}
            onClick={() => {
              onChange({
                ...node,
                fields: [
                  ...node.fields,
                  {
                    id: newID('field'),
                    name: '',
                    type: 'text',
                    required: false,
                  },
                ],
              });
            }}
          >
            <Plus aria-hidden="true" />
            添加字段
          </Button>
        </div>
        {node.fields.map((field, index) => {
          const isTime = field.id === node.time_field_id;
          return (
            <fieldset
              key={field.id}
              className="space-y-3 rounded-lg border border-border bg-background/70 p-3"
            >
              <div className="flex items-center justify-between gap-2">
                <span className="text-xs font-medium">
                  {isTime
                    ? field.student_field
                      ? '发生时间 · 日期 · 自动引用'
                      : '发生时间 · 日期 · 必填'
                    : `字段 ${String(index + 1)}`}
                </span>
                {!isTime ? (
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    aria-label={`删除字段 ${String(index + 1)}`}
                    onClick={() => {
                      onChange({
                        ...node,
                        fields: node.fields.filter((item) => item.id !== field.id),
                      });
                    }}
                  >
                    <Trash2 aria-hidden="true" />
                  </Button>
                ) : null}
              </div>
              <Input
                aria-label={isTime ? '发生时间字段名称' : `字段 ${String(index + 1)} 名称`}
                placeholder="字段名称"
                required
                maxLength={100}
                value={field.name}
                onChange={(event) => {
                  changeField(field.id, { name: event.target.value });
                }}
              />
              <div className="space-y-2">
                <Label htmlFor={`source-${field.id}`}>数据来源</Label>
                <SearchSelect
                  id={`source-${field.id}`}
                  label={isTime ? '发生时间数据来源' : `字段 ${String(index + 1)} 数据来源`}
                  className="w-full"
                  value={field.student_field ?? ''}
                  options={[{ value: '', label: '手动填写' }, ...studentFieldOptions]}
                  onValueChange={(value) => {
                    const source = studentFieldOptions.find((item) => item.value === value);
                    changeField(field.id, {
                      student_field: source?.value,
                      ...(source ? { type: 'date' } : {}),
                    });
                  }}
                />
              </div>
              {field.student_field ? (
                <p className="text-xs text-muted-foreground">
                  内容来自学生档案，请在学生信息中修改。
                </p>
              ) : null}
              {!isTime && !field.student_field ? (
                <div className="flex flex-wrap items-center gap-3">
                  <SearchSelect
                    label={`字段 ${String(index + 1)} 类型`}
                    className="min-w-32 flex-1"
                    options={[...fieldTypes]}
                    value={field.type}
                    onValueChange={(value) => {
                      const type = fieldTypes.find((item) => item.value === value)?.value;
                      if (type) changeField(field.id, { type });
                    }}
                  />
                  <Label className="flex items-center gap-2 text-xs">
                    <Checkbox
                      checked={field.required}
                      onChange={(event) => {
                        changeField(field.id, { required: event.target.checked });
                      }}
                    />
                    必填
                  </Label>
                </div>
              ) : isTime ? (
                <p className="text-xs text-muted-foreground">
                  名称可自定义，所有时间规则均使用此字段
                </p>
              ) : null}
            </fieldset>
          );
        })}
      </section>
      <RuleEditor
        node={node}
        nodes={nodes}
        onChange={(rules) => {
          onChange({ ...node, rules });
        }}
      />
    </div>
  );
}
