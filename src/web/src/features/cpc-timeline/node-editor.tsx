import { Plus, Trash2 } from 'lucide-react';
import { type ReactNode, useLayoutEffect, useRef, useState } from 'react';

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
  autoFocusName = false,
}: {
  node: TimelineNode;
  nodes: TimelineNode[];
  onChange: (node: TimelineNode) => void;
  attachments?: ReactNode;
  autoFocusName?: boolean;
}) {
  const [newFieldID, setNewFieldID] = useState('');
  const nameInput = useRef<HTMLInputElement>(null);
  const newFieldInput = useRef<HTMLInputElement>(null);
  useLayoutEffect(() => {
    // 新增节点后直接编辑名称，已有节点切换保留导航焦点。
    if (!autoFocusName) return;
    nameInput.current?.focus();
    nameInput.current?.select();
  }, [autoFocusName, node.id]);
  useLayoutEffect(() => {
    // 仅在添加字段时定位到新字段，普通编辑不移动焦点。
    if (newFieldID) newFieldInput.current?.focus();
  }, [newFieldID]);
  const changeField = (id: string, patch: Partial<TimelineField>) => {
    onChange({
      ...node,
      fields: node.fields.map((field) => (field.id === id ? { ...field, ...patch } : field)),
    });
  };
  return (
    <div className="min-w-0 space-y-6">
      <section aria-label="节点信息" className="space-y-4">
        <div className="grid gap-4 sm:grid-cols-2">
          <div className="space-y-2">
            <Label htmlFor={`name-${node.id}`} required className="leading-5">
              节点名称
            </Label>
            <Input
              id={`name-${node.id}`}
              ref={nameInput}
              aria-label="节点名称"
              placeholder="例如：提交申请"
              required
              maxLength={100}
              value={node.name}
              onChange={(event) => {
                onChange({ ...node, name: event.target.value });
              }}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor={`group-${node.id}`} className="leading-5">
              节点分组
            </Label>
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
          </div>
        </div>
        <div className="space-y-2">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <Label htmlFor={`description-${node.id}`} className="leading-5">
              节点说明
            </Label>
            <Label className="inline-flex items-center gap-2 leading-5 font-normal">
              <Checkbox
                checked={node.is_baseline}
                onChange={(event) => {
                  onChange({ ...node, is_baseline: event.target.checked });
                }}
              />
              基准节点
            </Label>
          </div>
          <Textarea
            id={`description-${node.id}`}
            aria-label="节点说明"
            placeholder="说明该节点的填写要求或注意事项"
            className="min-h-20"
            rows={2}
            maxLength={1000}
            value={node.description}
            onChange={(event) => {
              onChange({ ...node, description: event.target.value });
            }}
          />
        </div>
      </section>
      <section className="space-y-4" aria-label="节点字段">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <h3 className="text-sm font-medium">字段配置</h3>
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={node.fields.length >= getConfig().timelineMaxFieldsPerNode}
            onClick={() => {
              const id = newID('field');
              setNewFieldID(id);
              onChange({
                ...node,
                fields: [
                  ...node.fields,
                  {
                    id,
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
        <div className={node.fields.length > 1 ? 'grid gap-4 xl:grid-cols-2' : 'grid gap-4'}>
          {node.fields.map((field, index) => {
            const isTime = field.id === node.time_field_id;
            return (
              <fieldset
                key={field.id}
                className="min-w-0 space-y-4 rounded-xl border border-border-subtle bg-muted/30 p-4"
              >
                <div className="flex min-h-8 items-center justify-between gap-2">
                  <Label
                    htmlFor={`field-name-${field.id}`}
                    required={field.required && !field.student_field}
                    className="text-xs font-medium"
                  >
                    {isTime ? '发生时间' : `字段 ${String(index + 1)}`}
                  </Label>
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
                <div className="grid gap-4 sm:grid-cols-2">
                  <div className="space-y-2">
                    <Label htmlFor={`field-name-${field.id}`} required className="leading-5">
                      字段名称
                    </Label>
                    <Input
                      id={`field-name-${field.id}`}
                      ref={field.id === newFieldID ? newFieldInput : undefined}
                      aria-label={isTime ? '发生时间字段名称' : `字段 ${String(index + 1)} 名称`}
                      placeholder="填写字段名称"
                      required
                      maxLength={100}
                      value={field.name}
                      onChange={(event) => {
                        changeField(field.id, { name: event.target.value });
                      }}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor={`source-${field.id}`} className="leading-5">
                      数据来源
                    </Label>
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
                </div>
                {field.student_field ? (
                  <p className="text-xs text-muted-foreground">
                    内容来自学生档案，请在学生信息中修改。
                  </p>
                ) : null}
                {!isTime && !field.student_field ? (
                  <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto]">
                    <div className="space-y-2">
                      <Label htmlFor={`type-${field.id}`} required className="leading-5">
                        字段类型
                      </Label>
                      <SearchSelect
                        id={`type-${field.id}`}
                        label={`字段 ${String(index + 1)} 类型`}
                        required
                        className="w-full"
                        options={[...fieldTypes]}
                        value={field.type}
                        onValueChange={(value) => {
                          const type = fieldTypes.find((item) => item.value === value)?.value;
                          if (type) changeField(field.id, { type });
                        }}
                      />
                    </div>
                    <Label className="flex h-9 items-center gap-2 self-end font-normal">
                      <Checkbox
                        checked={field.required}
                        onChange={(event) => {
                          changeField(field.id, { required: event.target.checked });
                        }}
                      />
                      必填
                    </Label>
                  </div>
                ) : null}
              </fieldset>
            );
          })}
        </div>
      </section>
      <RuleEditor
        node={node}
        nodes={nodes}
        onChange={(rules) => {
          onChange({ ...node, rules });
        }}
      />
      {attachments}
    </div>
  );
}
