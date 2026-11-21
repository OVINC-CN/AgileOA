import { type SetStateAction, useEffect, useRef, useState } from 'react';

import { showMessage } from '@/lib/messages';

function sameValue(left: unknown, right: unknown): boolean {
  if (left === right) return true;
  if (!left || !right || typeof left !== 'object' || typeof right !== 'object') return false;
  const leftEntries = Object.entries(left);
  const rightEntries = Object.entries(right);
  return (
    leftEntries.length === rightEntries.length &&
    leftEntries.every(
      ([key, value]) =>
        Object.hasOwn(right, key) && sameValue(value, (right as Record<string, unknown>)[key]),
    )
  );
}

// 文本草稿等待搜索；选项只更新已应用条件，连续多选保留最新选择。
export function useFilterForm<T extends object>(
  initialValues: T | (() => T),
  onApply: (values: T) => Promise<unknown>,
  appliedValues: T,
) {
  const [draft, updateDraft] = useState(initialValues);
  const current = useRef(draft);
  const sequence = useRef(0);
  const submitted = useRef(appliedValues);
  const applied = useRef(appliedValues);

  useEffect(() => {
    submitted.current = appliedValues;
    applied.current = appliedValues;
  }, [appliedValues]);

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
  const applyChanges = async (changes: Partial<T>) => {
    // 相同选项不重复请求，清空多选同样视为条件变化。
    const changed = Object.entries(changes).some(
      ([key, value]) =>
        !sameValue(current.current[key as keyof T], value) ||
        !sameValue(submitted.current[key as keyof T], value),
    );
    if (!changed) return;

    // 选项请求独立执行，避免搜索按钮的提交锁丢弃后续变化。
    const next = { ...submitted.current, ...changes };
    submitted.current = next;
    setDraft({ ...current.current, ...changes });
    const request = sequence.current;
    await onApply(next).catch((cause: unknown) => {
      if (request !== sequence.current) return;
      submitted.current = applied.current;
      showMessage(cause instanceof Error ? cause.message : '查询失败，请重试');
    });
  };
  const changeFilters = (changes: Partial<T>) => {
    void applyChanges(changes);
  };

  // 搜索会清理文字两端空白，提示按相同口径比较，选项值仍逐字匹配。
  const comparable = (values: T) =>
    Object.fromEntries(
      Object.entries(values).map(([key, value]) => [
        key,
        typeof value === 'string' ? value.trim() : value,
      ]),
    );
  return {
    draft,
    setDraft,
    changeFilters,
    applyChanges,
    hasUnappliedChanges: !sameValue(comparable(draft), comparable(appliedValues)),
  };
}
