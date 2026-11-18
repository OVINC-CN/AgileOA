import { useMutation, type UseMutationOptions } from '@tanstack/react-query';
import { useEffect, useRef, useState } from 'react';

import { showMessage } from '@/lib/messages';

type Result<T> = { ok: true; data: T } | { ok: false };

export function useApiAction() {
  const [active, setActive] = useState<string | null>(null);
  const [error, setError] = useState<Error | null>(null);
  const locked = useRef(false);
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);

  const run = async <T>(key: string, operation: () => Promise<T>): Promise<Result<T>> => {
    // 同一组操作在第一次点击时立即上锁，避免状态尚未渲染时重复提交。
    if (locked.current) return { ok: false };
    locked.current = true;
    setActive(key);
    setError(null);
    try {
      const data = await operation();
      return mounted.current ? { ok: true, data } : { ok: false };
    } catch (cause) {
      if (mounted.current) {
        const failure = cause instanceof Error ? cause : new Error('请求失败，请重试');
        setError(failure);
        showMessage(failure.message);
      }
      return { ok: false };
    } finally {
      locked.current = false;
      if (mounted.current) setActive(null);
    }
  };
  return { run, pending: active !== null, isPending: (key: string) => active === key, error };
}

type MutationOptions<TData, TVariables> = Omit<
  UseMutationOptions<TData, Error, TVariables>,
  'mutationFn'
> & {
  mutationFn: NonNullable<UseMutationOptions<TData, Error, TVariables>['mutationFn']>;
};

export function useApiMutation<TData, TVariables = void>({
  mutationFn,
  onError,
  ...options
}: MutationOptions<TData, TVariables>) {
  return useMutation({
    ...options,
    mutationFn,
    onError: async (error, variables, result, context) => {
      showMessage(error.message);
      await onError?.(error, variables, result, context);
    },
  });
}

// 首次读取沿用请求状态，指示器退出延迟由公共组件统一处理，避免多层叠加。
export function useInitialLoading(active: boolean) {
  return active;
}
