import { z } from 'zod';

import {
  isEmptyTimelineOperator,
  serializeTimelineFilters,
  type TimelineFilterDraft,
  type TimelineFilterField,
  timelineFilterFieldSchema,
  timelineFilterOperators,
} from '@/features/cpc-person/timeline-filter-model';

export const timelinePresetNameLength = 50;
export const timelinePresetChangeEvent = 'cpc-person:timeline-presets-changed';
const presetNameSchema = z
  .string()
  .trim()
  .min(1, '请输入方案名称')
  .max(timelinePresetNameLength, `方案名称最多 ${String(timelinePresetNameLength)} 个字符`)
  .refine((value) => !/\p{Cc}/u.test(value), '方案名称不能包含控制字符');
const presetSchema = z.object({
  id: z.uuid(),
  name: presetNameSchema,
  includeValues: z.boolean(),
  conditions: z
    .array(
      z.object({
        field: timelineFilterFieldSchema,
        operator: z.string(),
        value: z.string().max(1000),
      }),
    )
    .min(1)
    .refine((conditions) =>
      conditions.every(({ field, operator }) =>
        timelineFilterOperators(field.type).some((item) => item.value === operator),
      ),
    ),
});
const presetsSchema = z
  .array(presetSchema)
  .refine(
    (presets) =>
      new Set(presets.map(({ id }) => id)).size === presets.length &&
      new Set(presets.map(({ name }) => name)).size === presets.length,
  );

export type TimelineFilterPreset = z.infer<typeof presetSchema>;
export interface TimelinePresetSnapshot {
  presets: TimelineFilterPreset[];
  error: string | null;
}

export const timelinePresetStorageKey = (userID: number, workspaceID: string) =>
  `cpc-person:timeline-filter-presets:${String(userID)}:${workspaceID}`;

export function readTimelinePresets(storageKey: string): TimelinePresetSnapshot {
  // 读取失败时保留页面筛选能力，用户主动保存后才替换损坏的数据。
  let stored: string | null;
  try {
    stored = localStorage.getItem(storageKey);
  } catch {
    return { presets: [], error: '无法读取常用筛选方案，请检查浏览器设置' };
  }
  if (stored === null) return { presets: [], error: null };
  try {
    const value: unknown = JSON.parse(stored);
    const presets = presetsSchema.parse(value);
    presets.forEach((preset) => {
      prepareTimelinePreset(preset.name, preset.includeValues, preset.conditions);
    });
    return { presets, error: null };
  } catch {
    return { presets: [], error: '本地常用筛选方案数据无效，请重新保存方案' };
  }
}

export function writeTimelinePresets(storageKey: string, presets: TimelineFilterPreset[]) {
  // 先校验完整结果，存储成功后再由调用方更新界面。
  if (!presetsSchema.safeParse(presets).success) throw new Error('常用筛选方案格式无效');
  try {
    if (presets.length === 0) localStorage.removeItem(storageKey);
    else localStorage.setItem(storageKey, JSON.stringify(presets));
  } catch {
    throw new Error('保存常用筛选方案失败，请检查浏览器设置');
  }
  window.dispatchEvent(new CustomEvent(timelinePresetChangeEvent, { detail: storageKey }));
}

export function prepareTimelinePreset(
  name: string,
  includeValues: boolean,
  filters: Pick<TimelineFilterDraft, 'field' | 'operator' | 'value' | 'invalid'>[],
): Omit<TimelineFilterPreset, 'id'> {
  // 字段未选或已失效时拒绝保存，避免方案缺少用户配置的条件。
  const parsedName = presetNameSchema.safeParse(name);
  if (!parsedName.success) throw new Error(parsedName.error.issues[0]?.message ?? '方案名称无效');
  if (filters.length === 0) throw new Error('请先添加时间线筛选条件');
  const conditions = filters.map(({ field, operator, value, invalid }, index) => {
    if (!field || invalid) throw new Error(`请重新选择第 ${String(index + 1)} 个时间线字段`);
    if (!timelineFilterOperators(field.type).some((item) => item.value === operator)) {
      throw new Error('请选择有效的时间线筛选方式');
    }
    if (includeValues && !isEmptyTimelineOperator(operator) && !value.trim()) {
      throw new Error('请填写全部筛选值，或取消“保存筛选值”');
    }
    return {
      field: { ...field },
      operator,
      value: includeValues && !isEmptyTimelineOperator(operator) ? value.trim() : '',
    };
  });
  serializeTimelineFilters(conditions.map((condition) => ({ ...condition, id: '' })));
  return { name: parsedName.data, includeValues, conditions };
}

export function restoreTimelinePreset(
  preset: TimelineFilterPreset,
  fields: TimelineFilterField[],
): TimelineFilterDraft[] {
  // 使用稳定标识匹配当前目录；失效条件保留并标记，禁止静默放宽查询范围。
  const directory = new Map(
    fields.map((field) => [`${String(field.node_id)}:${field.field_id}`, field]),
  );
  return preset.conditions.map(({ field, operator, value }) => {
    const current = directory.get(`${String(field.node_id)}:${field.field_id}`);
    const invalid = current?.type !== field.type;
    return {
      id: crypto.randomUUID(),
      field: { ...(!invalid ? current : field) },
      operator,
      value: preset.includeValues ? value : '',
      ...(invalid ? { invalid: true } : {}),
    };
  });
}

export function timelineFilterSignature(
  filters: Pick<TimelineFilterDraft, 'field' | 'operator' | 'value' | 'invalid'>[],
) {
  return JSON.stringify(
    filters.map(({ field, operator, value, invalid }) => [
      field?.node_id,
      field?.field_id,
      field?.type,
      operator,
      value,
      Boolean(invalid),
    ]),
  );
}
