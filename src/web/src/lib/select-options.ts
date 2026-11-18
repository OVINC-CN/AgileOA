import { pinyin } from 'pinyin-pro';

const initialOrder = ['numeric', ...'ABCDEFGHIJKLMNOPQRSTUVWXYZ'.split(''), '#'];
const collator = new Intl.Collator('en', { sensitivity: 'base', numeric: true });

// 数字、拼音与英文共用排序键，其他开头统一放在末尾。
export function sortSelectOptions<T extends { label: string; pinned?: boolean }>(
  options: readonly T[],
) {
  const indexed = options.map((option, index) => {
    const key = pinyin(option.label.normalize('NFKC').trim(), {
      toneType: 'none',
      type: 'array',
      v: true,
    })
      .join('')
      .normalize('NFKD')
      .replace(/\p{M}/gu, '')
      .toLowerCase();
    const initial = option.pinned
      ? 'pinned'
      : /^[0-9]/.test(key)
        ? 'numeric'
        : /^[a-z]/.test(key)
          ? key.charAt(0).toUpperCase()
          : '#';
    return { option, index, key, initial };
  });

  // 相同名称及忽略大小写后相等的选项维持原有顺序，不修改传入数组。
  return indexed
    .sort((left, right) => {
      // 内置入口保持传入顺序置顶，不参与普通选项的排序与字母索引。
      if (left.option.pinned || right.option.pinned) {
        return (
          Number(Boolean(right.option.pinned)) - Number(Boolean(left.option.pinned)) ||
          left.index - right.index
        );
      }
      return (
        initialOrder.indexOf(left.initial) - initialOrder.indexOf(right.initial) ||
        collator.compare(left.key, right.key) ||
        left.index - right.index
      );
    })
    .map(({ option, initial }) => ({ ...option, initial }));
}
