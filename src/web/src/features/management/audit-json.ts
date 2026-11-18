import { isLosslessNumber, isSafeNumber, LosslessNumber, parse, stringify } from 'lossless-json';

const maximumSafeInteger = BigInt(Number.MAX_SAFE_INTEGER);
const minimumSafeInteger = BigInt(Number.MIN_SAFE_INTEGER);

// 审计快照中的长整数直接保留原始数值文本，不先转换成浮点数。
export function parseAuditJSON(text: string): unknown {
  return parse(text, null, {
    parseNumber: (value: string) => {
      if (/^-?\d+$/u.test(value)) {
        const integer = BigInt(value);
        return integer >= minimumSafeInteger && integer <= maximumSafeInteger
          ? Number(value)
          : new LosslessNumber(value);
      }
      return isSafeNumber(value) ? Number(value) : new LosslessNumber(value);
    },
  });
}

function ordered(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(ordered);
  if (isLosslessNumber(value)) return value;
  if (value !== null && typeof value === 'object') {
    return Object.fromEntries(
      Object.entries(value)
        .sort(([left], [right]) => (left < right ? -1 : left > right ? 1 : 0))
        .map(([key, item]) => [key, ordered(item)]),
    );
  }
  return value;
}

// 只规范展示副本的对象键和缩进，数组顺序与所有数值类型保持。
export function formatAuditJSON(value: unknown): string {
  return stringify(ordered(value), undefined, 2) ?? 'null';
}
