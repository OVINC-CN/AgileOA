import { getConfig } from '@/lib/config';

export const emptyValueLabel = '-';

// 空字段统一显示占位符，零与布尔值保留原有语义。
export function formatValue(value: string | number | boolean | null | undefined) {
  if (value === null || value === undefined || (typeof value === 'string' && value.trim() === '')) {
    return emptyValueLabel;
  }
  return String(value);
}

// 总分固定显示两位小数，已有浮点尾差同样处理，空分仍显示占位符。
export function formatScore(value: number | null | undefined) {
  if (value === null || value === undefined) return emptyValueLabel;
  return value.toFixed(2);
}

// 用户与学生统一显示为名称(账号或学号)，缺失字段沿用公共占位规则。
export function formatIdentity(
  name: string | null | undefined,
  identifier: string | null | undefined,
) {
  const label = name?.trim();
  const code = identifier?.trim();
  if (!label) return formatValue(code);
  return code ? `${label}(${code})` : label;
}

export function formatDate(value: string | number | null | undefined, { includeYear = true } = {}) {
  if (value === null || value === undefined || (typeof value === 'string' && !value.trim())) {
    return emptyValueLabel;
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return emptyValueLabel;

  // 按配置时区展示固定格式，避免区域设置改变分隔符或午夜小时。
  const parts = new Intl.DateTimeFormat('zh-CN', {
    timeZone: getConfig().displayTimeZone,
    year: '2-digit',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hourCycle: 'h23',
  }).formatToParts(date);
  const {
    year = '',
    month = '',
    day = '',
    hour = '',
    minute = '',
  } = Object.fromEntries(parts.map((part) => [part.type, part.value]));
  const monthDayTime = `${month}/${day} ${hour}:${minute}`;
  return includeYear ? `${year}/${monthDayTime}` : monthDayTime;
}

export function formatNumber(value: number) {
  return new Intl.NumberFormat('zh-CN').format(value);
}

// 数据仪表中的零人数留空，其他人数沿用默认数字格式。
export function formatDashboardCount(value: number) {
  return value === 0 ? '' : formatNumber(value);
}
