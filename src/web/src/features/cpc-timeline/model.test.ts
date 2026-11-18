import { describe, expect, it } from 'vitest';

import {
  moveNode,
  newNode,
  parseTimelineInput,
  removeNode,
  type TimelineInput,
  zeroOffset,
} from '@/features/cpc-timeline/model';
import { getWorkspaceNavigation } from '@/features/workspace/navigation';

function fixture() {
  const reference = newNode();
  const node = newNode();
  node.rules = [
    {
      id: 'rule_first',
      reference_node_id: reference.id,
      min_offset: { ...zeroOffset(), years: 1, days: -7 },
      max_offset: { ...zeroOffset(), years: 1 },
    },
    {
      id: 'rule_second',
      reference_node_id: reference.id,
      min_offset: null,
      max_offset: { ...zeroOffset(), workdays: 5 },
    },
  ];
  return {
    name: '虚构模板',
    description: '',
    definition: { nodes: [node, reference] },
  } satisfies TimelineInput;
}

function firstParts(input: TimelineInput) {
  const node = input.definition.nodes[0];
  const field = node?.fields[0];
  const rule = node?.rules[0];
  if (!node || !field || !rule) throw new Error('缺少测试定义');
  return { node, field, rule };
}

describe('时间线模板定义', () => {
  it('分组可省略，保存时清理空白并校验名称长度及控制字符', () => {
    const input = fixture();
    const legacy = {
      ...input,
      definition: {
        nodes: input.definition.nodes.map(({ group_name, ...node }) => {
          expect(group_name).toBe('');
          return node;
        }),
      },
    };
    expect(parseTimelineInput(legacy).definition.nodes.map((node) => node.group_name)).toEqual([
      '',
      '',
    ]);
    const { node } = firstParts(input);
    node.group_name = '  申请阶段  ';
    expect(parseTimelineInput(input).definition.nodes[0]?.group_name).toBe('申请阶段');
    for (const name of ['中'.repeat(101), '申请\n阶段', '申请\u0000阶段']) {
      node.group_name = name;
      expect(() => parseTimelineInput(input)).toThrow();
    }
  });

  it('兼容手动填写，只允许日期字段引用出生日期和成年日期', () => {
    for (const source of ['', 'birth_date', 'adulthood_date'] as const) {
      const input = fixture();
      firstParts(input).field.student_field = source;
      expect(parseTimelineInput(input).definition.nodes[0]?.fields[0]?.student_field).toBe(source);
    }
    const input = fixture();
    const { field } = firstParts(input);
    field.student_field = 'birth_date';
    field.type = 'text';
    expect(() => parseTimelineInput(input)).toThrow();
    expect(() =>
      parseTimelineInput({
        ...input,
        definition: {
          nodes: input.definition.nodes.map((node) => ({
            ...node,
            fields: node.fields.map((item) => ({
              ...item,
              type: 'date',
              student_field: 'identity_number',
            })),
          })),
        },
      }),
    ).toThrow();
  });

  it('旧导入格式省略基准标记时默认为否，显式配置及取消配置均保留', () => {
    const input = fixture();
    const legacy = {
      ...input,
      definition: {
        nodes: input.definition.nodes.map(({ is_baseline, ...node }) => {
          expect(is_baseline).toBe(false);
          return node;
        }),
      },
    };
    expect(parseTimelineInput(legacy).definition.nodes.every((node) => !node.is_baseline)).toBe(
      true,
    );
    const first = input.definition.nodes[0];
    if (!first) throw new Error('缺少测试节点');
    first.is_baseline = true;
    expect(parseTimelineInput(input).definition.nodes[0]?.is_baseline).toBe(true);
    first.is_baseline = false;
    expect(parseTimelineInput(input).definition.nodes[0]?.is_baseline).toBe(false);
    expect(() =>
      parseTimelineInput({
        ...input,
        definition: { nodes: [{ ...first, is_baseline: 'true' }] },
      }),
    ).toThrow();
  });

  it('排序和重命名保留字段标识及跨节点引用，支持未来节点和多条复合偏移规则', () => {
    const input = fixture();
    const [node, reference] = input.definition.nodes;
    if (!node || !reference) throw new Error('缺少测试节点');
    const next = moveNode(input.definition.nodes, node.id, reference.id);
    node.fields[0] = {
      id: node.time_field_id,
      name: '自定义发生日期',
      type: 'date',
      required: true,
    };
    const result = parseTimelineInput({ ...input, definition: { nodes: next } });
    expect(result.definition.nodes.map((item) => item.id)).toEqual([reference.id, node.id]);
    expect(result.definition.nodes[1]?.rules).toEqual(node.rules);
    expect(result.definition.nodes[1]?.time_field_id).toBe(node.time_field_id);
    expect(input.definition.nodes.map((item) => item.id)).toEqual([node.id, reference.id]);
  });

  it('删除引用目标前要求显式处理规则，拖拽不会改写引用', () => {
    const { nodes } = fixture().definition;
    const reference = nodes[1];
    if (!reference) throw new Error('缺少测试节点');
    expect(() => removeNode(nodes, reference.id)).toThrow('请先处理引用');
    expect(nodes).toHaveLength(2);
  });

  it.each([
    ({ node }: ReturnType<typeof firstParts>) => {
      node.time_field_id = 'missing';
    },
    ({ field }: ReturnType<typeof firstParts>) => {
      field.required = false;
    },
    ({ node, rule }: ReturnType<typeof firstParts>) => {
      rule.reference_node_id = node.id;
    },
    ({ rule }: ReturnType<typeof firstParts>) => {
      rule.reference_node_id = 'missing';
    },
    ({ rule }: ReturnType<typeof firstParts>) => {
      rule.min_offset = null;
      rule.max_offset = null;
    },
  ])('拒绝缺失发生时间字段、非必填发生时间、自身和悬空引用及空范围', (mutate) => {
    const input = fixture();
    mutate(firstParts(input));
    expect(() => parseTimelineInput(input)).toThrow();
  });

  it('拒绝强制规则模式和人员字段值', () => {
    const input = fixture();
    const rules = input.definition.nodes[0]?.rules;
    const nodes = input.definition.nodes;
    expect(() =>
      parseTimelineInput({
        ...input,
        definition: {
          nodes: [{ ...nodes[0], rules: [{ ...rules?.[0], mode: 'blocking' }] }, nodes[1]],
        },
      }),
    ).toThrow();
    expect(() => parseTimelineInput({ ...input, occurred_at: '2026-01-01' })).toThrow();
  });

  it('只有模板列表权限也能显示数据管理入口，非列表权限不能显示', () => {
    expect(
      getWorkspaceNavigation(['cpc.timeline_template.list'], 'cpc').map((item) => item.path),
    ).toEqual(['data-management']);
    expect(getWorkspaceNavigation(['cpc.timeline_template.create'], 'cpc')).toEqual([]);
  });
});
