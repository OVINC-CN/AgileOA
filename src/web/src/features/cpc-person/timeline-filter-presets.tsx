import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Save, Settings2 } from 'lucide-react';
import { useEffect, useState } from 'react';

import { ErrorMessage } from '@/components/error-message';
import { FilterField } from '@/components/filter-panel';
import { Button } from '@/components/ui/button';
import { SearchSelect } from '@/components/ui/search-select';
import { personTimelineFilterFieldOptions } from '@/features/cpc-person/api';
import type { TimelineFilterDraft } from '@/features/cpc-person/timeline-filter-model';
import {
  TimelinePresetDeleteDrawer,
  type TimelinePresetEditor,
  TimelinePresetEditorDrawer,
  TimelinePresetManagerDrawer,
} from '@/features/cpc-person/timeline-filter-preset-drawers';
import {
  prepareTimelinePreset,
  readTimelinePresets,
  restoreTimelinePreset,
  type TimelineFilterPreset,
  timelineFilterSignature,
  timelinePresetChangeEvent,
  timelinePresetStorageKey,
  writeTimelinePresets,
} from '@/features/cpc-person/timeline-filter-preset-store';
import { currentUserOptions } from '@/features/user/api';
import { useApiAction } from '@/lib/api-action';
import { showMessage } from '@/lib/messages';

interface Props {
  workspaceID: string;
  value: TimelineFilterDraft[];
  onChange: (value: TimelineFilterDraft[]) => void;
}

export function TimelineFilterPresets(props: Props) {
  // 订阅登录态缓存，复用鉴权入口已读取的身份，不额外请求用户接口。
  const { data: user } = useQuery({ ...currentUserOptions, enabled: false });
  if (!user) return null;
  return (
    <ScopedTimelineFilterPresets
      key={`${String(user.id)}:${props.workspaceID}`}
      {...props}
      userID={user.id}
    />
  );
}

function ScopedTimelineFilterPresets({
  userID,
  workspaceID,
  value,
  onChange,
}: Props & { userID: number }) {
  const storageKey = timelinePresetStorageKey(userID, workspaceID);
  const [snapshot, setSnapshot] = useState(() => readTimelinePresets(storageKey));
  const [selectedID, setSelectedID] = useState('');
  const [managing, setManaging] = useState(false);
  const [editor, setEditor] = useState<TimelinePresetEditor | null>(null);
  const [deleting, setDeleting] = useState<TimelineFilterPreset | null>(null);
  const restore = useApiAction();
  const client = useQueryClient();
  const selected = snapshot.presets.find((preset) => preset.id === selectedID);
  const selectionMatches =
    selected && timelineFilterSignature(selected.conditions) === timelineFilterSignature(value);

  useEffect(() => {
    // 同一页面和其他标签页更新方案时同步刷新，清空浏览器存储时同样清理显示。
    const onStorage = (event: StorageEvent) => {
      if (event.key === null || event.key === storageKey) {
        setSnapshot(readTimelinePresets(storageKey));
      }
    };
    const onChange = (event: Event) => {
      if (event instanceof CustomEvent && event.detail === storageKey) {
        setSnapshot(readTimelinePresets(storageKey));
      }
    };
    window.addEventListener('storage', onStorage);
    window.addEventListener(timelinePresetChangeEvent, onChange);
    return () => {
      window.removeEventListener('storage', onStorage);
      window.removeEventListener(timelinePresetChangeEvent, onChange);
    };
  }, [storageKey]);

  const save = (name: string, includeValues: boolean) => {
    if (!editor) return;
    // 写入前重新读取，避免覆盖其他标签页刚保存的方案。
    const current = readTimelinePresets(storageKey).presets;
    const existing =
      editor.mode === 'create' ? undefined : current.find((item) => item.id === editor.preset.id);
    if (editor.mode !== 'create' && !existing) throw new Error('方案已不存在，请重新选择');
    const prepared =
      editor.mode === 'rename' && existing
        ? prepareTimelinePreset(name, existing.includeValues, existing.conditions)
        : prepareTimelinePreset(name, includeValues, value);
    if (current.some((item) => item.id !== existing?.id && item.name === prepared.name)) {
      throw new Error('已存在同名方案，请更换名称或使用覆盖保存');
    }
    const preset = { ...prepared, id: existing?.id ?? crypto.randomUUID() };
    const next = existing
      ? current.map((item) => (item.id === existing.id ? preset : item))
      : [...current, preset];
    writeTimelinePresets(storageKey, next);
    setSelectedID(preset.id);
  };

  return (
    <>
      <ErrorMessage>{snapshot.error}</ErrorMessage>
      <div className="grid items-end gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <FilterField label="常用方案">
          <SearchSelect
            label="常用筛选方案"
            className="w-full"
            options={snapshot.presets.map(({ id, name }) => ({ value: id, label: name }))}
            value={selectionMatches ? selectedID : ''}
            loading={restore.pending}
            disabled={snapshot.presets.length === 0}
            onValueChange={(id) => {
              const preset = snapshot.presets.find((item) => item.id === id);
              if (!preset) return;
              void restore
                .run(id, async () => {
                  const fields = await client.query({
                    ...personTimelineFilterFieldOptions(workspaceID, ''),
                    staleTime: 0,
                  });
                  return restoreTimelinePreset(preset, fields);
                })
                .then((result) => {
                  if (!result.ok) return;
                  onChange(result.data);
                  setSelectedID(preset.id);
                  if (result.data.some((condition) => condition.invalid)) {
                    showMessage('方案中有字段已失效或类型发生变化，请重新选择标记的字段后搜索');
                  }
                });
            }}
          />
        </FilterField>
        <div className="flex flex-wrap gap-2 sm:justify-end lg:col-span-3">
          <Button
            type="button"
            variant="outline"
            disabled={value.length === 0 || restore.pending}
            onClick={() => {
              setEditor({ mode: 'create', preset: null });
            }}
          >
            <Save aria-hidden="true" />
            保存当前
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={snapshot.presets.length === 0 || restore.pending}
            onClick={() => {
              setManaging(true);
            }}
          >
            <Settings2 aria-hidden="true" />
            管理方案
          </Button>
        </div>
      </div>
      {managing ? (
        <TimelinePresetManagerDrawer
          presets={snapshot.presets}
          canOverwrite={value.length > 0}
          onEdit={setEditor}
          onDelete={setDeleting}
          onClose={() => {
            setManaging(false);
          }}
        />
      ) : null}
      {editor ? (
        <TimelinePresetEditorDrawer
          key={`${editor.mode}:${editor.preset?.id ?? ''}`}
          editor={editor}
          onSave={save}
          onClose={() => {
            setEditor(null);
          }}
        />
      ) : null}
      {deleting ? (
        <TimelinePresetDeleteDrawer
          preset={deleting}
          onDelete={() => {
            const current = readTimelinePresets(storageKey).presets;
            writeTimelinePresets(
              storageKey,
              current.filter((preset) => preset.id !== deleting.id),
            );
          }}
          onClose={() => {
            setDeleting(null);
          }}
        />
      ) : null}
    </>
  );
}
