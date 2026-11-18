import { type SetStateAction, useEffect, useRef, useState } from 'react';

import { showMessage } from '@/lib/messages';

// 筛选表单保留文本草稿，选项变化时立即提交当前条件。
export function useFilterForm<T extends object>(
  initialValues: T | (() => T),
  onApply: (values: T) => Promise<unknown>,
) {
  const [draft, updateDraft] = useState(initialValues);
  const current = useRef(draft);
  const sequence = useRef(0);

  useEffect(
    () => () => {
      sequence.current += 1;
    },
    [],
  );

  const setDraft = (values: SetStateAction<T>) => {
    // 同步保存草稿，连续多选时合并最新条件。
    const next = typeof values === 'function' ? values(current.current) : values;
    current.current = next;
    sequence.current += 1;
    updateDraft(next);
  };
  const changeFilters = (changes: Partial<T>) => {
    // 相同选项不重复请求，清空多选同样视为条件变化。
    const changed = Object.entries(changes).some(([key, value]) => {
      const previous = current.current[key as keyof T];
      return Array.isArray(previous) && Array.isArray(value)
        ? previous.length !== value.length || previous.some((item, index) => item !== value[index])
        : previous !== value;
    });
    if (!changed) return;

    // 选项请求独立执行，避免搜索按钮的提交锁丢弃后续变化。
    const next = { ...current.current, ...changes };
    setDraft(next);
    const request = sequence.current;
    void onApply(next).catch((cause: unknown) => {
      if (request !== sequence.current) return;
      showMessage(cause instanceof Error ? cause.message : '查询失败，请重试');
    });
  };

  return { draft, setDraft, changeFilters };
}
