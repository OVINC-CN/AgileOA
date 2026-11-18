import { useCallback, useSyncExternalStore } from 'react';

export interface TableColumn {
  id: string;
  label: string;
}

const storagePrefix = 'table-hidden-columns:';
const changeEvent = 'table-hidden-columns-change';
const memoryPreferences = new Map<string, string | null>();

function readPreference(key: string): string | null {
  // 本地存储不可写时保留当前页面内的选择。
  if (memoryPreferences.has(key)) return memoryPreferences.get(key) ?? null;
  try {
    return window.localStorage.getItem(key);
  } catch {
    return null;
  }
}

function parseHiddenColumns(value: string | null): string[] {
  if (value === null) return [];
  try {
    const parsed: unknown = JSON.parse(value);
    if (!Array.isArray(parsed)) return [];
    const ids: unknown[] = parsed;
    return ids.every((id): id is string => typeof id === 'string' && id !== '')
      ? [...new Set(ids)]
      : [];
  } catch {
    return [];
  }
}

function writePreference(key: string, hidden: string[]) {
  // 仅持久化隐藏字段的稳定标识，全部显示时删除配置。
  const value = hidden.length ? JSON.stringify(hidden) : null;
  try {
    if (value === null) window.localStorage.removeItem(key);
    else window.localStorage.setItem(key, value);
    memoryPreferences.delete(key);
  } catch {
    memoryPreferences.set(key, value);
  }
  window.dispatchEvent(new CustomEvent(changeEvent, { detail: key }));
}

export function useTableColumns<T extends TableColumn>(tableID: string, columns: readonly T[]) {
  const storageKey = `${storagePrefix}${tableID}`;
  const subscribe = useCallback(
    (notify: () => void) => {
      // 同页实例与其他标签页共享配置，按表格标识隔离。
      const onStorage = (event: StorageEvent) => {
        if (event.key !== null && event.key !== storageKey) return;
        memoryPreferences.delete(storageKey);
        notify();
      };
      const onChange = (event: Event) => {
        if (event instanceof CustomEvent && event.detail === storageKey) notify();
      };
      window.addEventListener('storage', onStorage);
      window.addEventListener(changeEvent, onChange);
      return () => {
        window.removeEventListener('storage', onStorage);
        window.removeEventListener(changeEvent, onChange);
      };
    },
    [storageKey],
  );
  const snapshot = useSyncExternalStore(
    subscribe,
    () => readPreference(storageKey),
    () => null,
  );
  const hidden = parseHiddenColumns(snapshot);
  const selected = columns.filter((column) => !hidden.includes(column.id));

  // 字段目录变更后若全部被隐藏，恢复显示，保证表格仍可使用。
  const visibleColumns = selected.length ? selected : columns;
  const isVisible = (id: string) => visibleColumns.some((column) => column.id === id);
  const setVisible = (id: string, visible: boolean) => {
    if (!columns.some((column) => column.id === id)) return;

    // 读取最新配置，避免同页多个表格实例相互覆盖。
    let current = parseHiddenColumns(readPreference(storageKey));
    if (columns.every((column) => current.includes(column.id))) {
      current = current.filter((entry) => !columns.some((column) => column.id === entry));
    }
    const count = columns.filter((column) => !current.includes(column.id)).length;
    if (!visible && !current.includes(id) && count <= 1) return;
    writePreference(
      storageKey,
      visible ? current.filter((entry) => entry !== id) : [...new Set([...current, id])],
    );
  };

  return {
    columns,
    visibleColumns,
    isVisible,
    setVisible,
    showAll: () => {
      writePreference(storageKey, []);
    },
  };
}
