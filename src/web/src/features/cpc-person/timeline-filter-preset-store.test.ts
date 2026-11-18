import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  serializeTimelineFilters,
  type TimelineFilterDraft,
} from '@/features/cpc-person/timeline-filter-model';
import {
  prepareTimelinePreset,
  readTimelinePresets,
  restoreTimelinePreset,
  timelineFilterSignature,
  timelinePresetStorageKey,
  writeTimelinePresets,
} from '@/features/cpc-person/timeline-filter-preset-store';

const score: TimelineFilterDraft = {
  id: 'score',
  field: { node_id: 7, field_id: 'score', label: '虚构节点 / 分数', type: 'number' },
  operator: 'eq',
  value: '0',
};
const filters: TimelineFilterDraft[] = [
  score,
  {
    id: 'confirmed',
    field: { node_id: 7, field_id: 'confirmed', label: '虚构节点 / 已确认', type: 'boolean' },
    operator: 'eq',
    value: 'false',
  },
];
const presetID = '2a72d10a-b2c0-49f6-81dc-2b54f297aa00';

afterEach(() => {
  vi.restoreAllMocks();
  localStorage.clear();
});

describe('时间线常用筛选方案存储', () => {
  it('保存后可重新读取，账号与工作空间分别隔离，数值零和否保持原义', () => {
    const key = timelinePresetStorageKey(1, 'workspace-one');
    const preset = { id: presetID, ...prepareTimelinePreset('  日常核对  ', true, filters) };
    writeTimelinePresets(key, [preset]);

    expect(readTimelinePresets(key)).toEqual({ presets: [preset], error: null });
    expect(readTimelinePresets(timelinePresetStorageKey(2, 'workspace-one')).presets).toEqual([]);
    expect(readTimelinePresets(timelinePresetStorageKey(1, 'workspace-two')).presets).toEqual([]);
    const restored = restoreTimelinePreset(
      preset,
      filters.flatMap(({ field }) => (field ? [field] : [])),
    );
    expect(serializeTimelineFilters(restored)).toEqual([
      { node_id: 7, field_id: 'score', operator: 'eq', value: '0' },
      { node_id: 7, field_id: 'confirmed', operator: 'eq', value: 'false' },
    ]);
    expect(restored[0]?.id).not.toBe(filters[0]?.id);
  });

  it('不保存筛选值时保留全部字段和方式，空值判断仍然生效', () => {
    const original = [...filters, { ...score, id: 'empty', operator: 'empty', value: '' }];
    const preset = { id: presetID, ...prepareTimelinePreset('待填写模板', false, original) };
    const restored = restoreTimelinePreset(
      preset,
      filters.flatMap(({ field }) => (field ? [field] : [])),
    );

    expect(restored).toHaveLength(3);
    expect(restored.every(({ value }) => value === '')).toBe(true);
    expect(serializeTimelineFilters(restored)).toEqual([
      { node_id: 7, field_id: 'score', operator: 'empty' },
    ]);
  });

  it('名称变化按稳定标识恢复，删除和类型变化保留条件并阻断提交', () => {
    const preset = { id: presetID, ...prepareTimelinePreset('日常核对', true, filters) };
    const restored = restoreTimelinePreset(preset, [
      { node_id: 7, field_id: 'score', label: '改名后的分数', type: 'number' },
      { node_id: 7, field_id: 'confirmed', label: '改名后的文本', type: 'text' },
    ]);

    expect(restored[0]?.field?.label).toBe('改名后的分数');
    expect(restored[1]?.invalid).toBe(true);
    expect(timelineFilterSignature(restored)).not.toBe(timelineFilterSignature(preset.conditions));
    expect(() => serializeTimelineFilters(restored)).toThrow('字段已失效');
    const missing = restoreTimelinePreset(preset, []);
    expect(missing).toHaveLength(filters.length);
    expect(missing.every(({ invalid }) => invalid)).toBe(true);
    expect(() => prepareTimelinePreset('失效条件', false, missing)).toThrow('重新选择');
  });

  it('拒绝未选择字段、错误的筛选值和不支持的运算符', () => {
    expect(() => prepareTimelinePreset('', true, filters)).toThrow('方案名称');
    expect(() => prepareTimelinePreset('空方案', true, [])).toThrow('添加时间线筛选条件');
    expect(() => prepareTimelinePreset('无效字段', false, [{ ...score, field: null }])).toThrow(
      '时间线字段',
    );
    expect(() =>
      prepareTimelinePreset('缺少值', true, [{ ...score, operator: 'eq', value: '' }]),
    ).toThrow('保存筛选值');
    expect(() =>
      prepareTimelinePreset('无效比较', false, [{ ...score, operator: 'contains', value: '' }]),
    ).toThrow('筛选方式');
  });

  it('损坏数据和浏览器存储异常提供明确错误，写入失败保留原数据', () => {
    const key = timelinePresetStorageKey(1, 'workspace-one');
    localStorage.setItem(key, '{broken');
    expect(readTimelinePresets(key).error).toContain('数据无效');
    const preset = { id: presetID, ...prepareTimelinePreset('日常核对', true, filters) };
    writeTimelinePresets(key, [preset]);
    const saved = localStorage.getItem(key);
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new DOMException('Quota exceeded', 'QuotaExceededError');
    });

    expect(() => {
      writeTimelinePresets(key, [preset]);
    }).toThrow('浏览器设置');
    expect(localStorage.getItem(key)).toBe(saved);
    expect(saved).not.toBeNull();
    expect(() => {
      writeTimelinePresets(key, []);
    }).not.toThrow();
    expect(localStorage.getItem(key)).toBeNull();
  });
});
