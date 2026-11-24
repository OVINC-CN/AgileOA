import { useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowDown, ArrowUp, Plus, RefreshCw, Save, Trash2 } from 'lucide-react';
import { useRef, useState } from 'react';

import { ErrorMessage } from '@/components/error-message';
import { SearchResetButton } from '@/components/search-reset-button';
import { Button } from '@/components/ui/button';
import { Drawer, type DrawerHandle } from '@/components/ui/drawer';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { SearchSelect } from '@/components/ui/search-select';
import { Textarea } from '@/components/ui/textarea';
import {
  detailNameOptions,
  maxParticipationRank,
  type Participation,
  type ParticipationInput,
  projectKey,
  projectOptions,
  saveParticipation,
  type ScoreItem,
} from '@/features/project/api';
import { validateUniqueDetailNames } from '@/features/project/detail-validation';
import { useApiAction, useApiMutation } from '@/lib/api-action';
import { getConfig } from '@/lib/config';
import { formatIdentity } from '@/lib/format';

export function ParticipationEditor({
  workspaceID,
  record,
  actions,
  onClose,
}: {
  workspaceID: string;
  record: Participation | null;
  actions: string[];
  onClose: () => void;
}) {
  const [original] = useState(record);
  const studentLabel = record ? formatIdentity(record.student_name, record.student_no) : '';
  const [values, setValues] = useState<ParticipationInput>(
    record ?? { project_id: 0, student_id: 0, rank: null, score: null, details: [] },
  );
  const [items, setItems] = useState(() =>
    (record?.details ?? []).map((value) => ({ key: crypto.randomUUID(), ...value })),
  );
  const client = useQueryClient();
  const drawer = useRef<DrawerHandle>(null);
  const mutation = useApiMutation({
    mutationFn: async () => {
      if (!values.project_id || !values.student_id) throw new Error('请选择项目和学生');
      if (items.some((item) => !item.name.trim())) throw new Error('请选择已配置的子项名称');
      validateUniqueDetailNames(items);
      const result = await saveParticipation(workspaceID, original, {
        ...values,
        details: items.map(({ name, description, score }) => ({ name, description, score })),
      });
      await client.invalidateQueries({ queryKey: projectKey(workspaceID) });
      return result;
    },
    onSuccess: () => {
      drawer.current?.close();
    },
  });
  const changeItem = (key: string, patch: Partial<ScoreItem>) => {
    setItems((current) => current.map((item) => (item.key === key ? { ...item, ...patch } : item)));
  };
  const move = (index: number, delta: number) => {
    setItems((current) => {
      const next = [...current];
      const item = next.splice(index, 1)[0];
      if (item) next.splice(index + delta, 0, item);
      return next;
    });
  };
  return (
    <Drawer
      ref={drawer}
      title={record ? '编辑参与评分' : '绑定学生项目'}
      truncateDescription={!!record}
      description={
        record
          ? `${record.project_name} · ${String(record.year)} · ${studentLabel}`
          : '排名、总分与子项分数分别填写，均可留空'
      }
      busy={mutation.isPending}
      onClose={onClose}
      dismissLabel="取消"
      footer={
        <Button type="submit" loading={mutation.isPending}>
          <Save aria-hidden="true" />
          保存
        </Button>
      }
      form={{
        onSubmit: (event) => {
          event.preventDefault();
          mutation.mutate();
        },
      }}
    >
      <fieldset disabled={mutation.isPending} className="space-y-5 p-6">
        <div className="grid gap-4 sm:grid-cols-2">
          <RecordSelector
            workspaceID={workspaceID}
            students={false}
            canQuery={actions.includes('project.list')}
            value={values.project_id}
            fallback={
              values.project_id === record?.project_id
                ? `${String(record.year)} · ${record.project_name}`
                : ''
            }
            onChange={(id) => {
              if (id !== values.project_id) {
                setItems((current) => current.map((item) => ({ ...item, name: '' })));
              }
              setValues((current) => ({ ...current, project_id: id }));
            }}
          />
          <RecordSelector
            workspaceID={workspaceID}
            students
            canQuery={actions.includes('project.participation.list')}
            value={values.student_id}
            fallback={values.student_id === record?.student_id ? studentLabel : ''}
            onChange={(id) => {
              setValues((current) => ({ ...current, student_id: id }));
            }}
          />
        </div>
        <div className="grid gap-4 sm:grid-cols-2">
          <div className="space-y-2">
            <Label htmlFor="participation-rank">排名（选填）</Label>
            <Input
              id="participation-rank"
              type="number"
              min={1}
              max={maxParticipationRank}
              step={1}
              value={values.rank ?? ''}
              onChange={(event) => {
                setValues((current) => ({
                  ...current,
                  rank: event.target.value === '' ? null : Number(event.target.value),
                }));
              }}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="participation-score">总分（选填）</Label>
            <Input
              id="participation-score"
              type="number"
              step="any"
              value={values.score ?? ''}
              onChange={(event) => {
                setValues((current) => ({
                  ...current,
                  score: event.target.value === '' ? null : Number(event.target.value),
                }));
              }}
            />
            <p className="text-xs text-muted-foreground">
              总分独立保存，不自动累加子项；留空与 0 分分别记录。
            </p>
          </div>
        </div>
        <div className="space-y-5 border-t border-border pt-5">
          <div className="flex items-center justify-between">
            <h2 className="text-sm font-medium">评分子项（{String(items.length)}）</h2>
            <Button
              type="button"
              variant="outline"
              disabled={
                !values.project_id ||
                !actions.includes('project.participation.list') ||
                items.length >= getConfig().projectMaxDetails
              }
              onClick={() => {
                setItems((current) => [
                  ...current,
                  { key: crypto.randomUUID(), name: '', description: '', score: null },
                ]);
              }}
            >
              <Plus aria-hidden="true" />
              添加子项
            </Button>
          </div>
          {items.map((item, index) => (
            <fieldset key={item.key} className="min-w-0 space-y-3 border-t border-border pt-5">
              <legend className="sr-only">子项 {String(index + 1)}</legend>
              <h3 className="text-xs text-muted-foreground">子项 {String(index + 1)}</h3>
              <div className="grid gap-3 sm:grid-cols-[1fr_9rem]">
                <div className="space-y-2">
                  <Label htmlFor={`item-name-${item.key}`}>子项名称</Label>
                  <DetailNameInput
                    key={values.project_id}
                    id={`item-name-${item.key}`}
                    value={item.name}
                    usedNames={items
                      .filter((other) => other.key !== item.key)
                      .map((other) => other.name)}
                    workspaceID={workspaceID}
                    projectID={values.project_id}
                    canQuery={actions.includes('project.participation.list')}
                    onChange={(name) => {
                      changeItem(item.key, { name });
                    }}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor={`item-score-${item.key}`}>分数（选填）</Label>
                  <Input
                    id={`item-score-${item.key}`}
                    type="number"
                    step="any"
                    value={item.score ?? ''}
                    onChange={(event) => {
                      changeItem(item.key, {
                        score: event.target.value === '' ? null : Number(event.target.value),
                      });
                    }}
                  />
                </div>
              </div>
              <div className="space-y-2">
                <Label htmlFor={`item-description-${item.key}`}>子项说明</Label>
                <Textarea
                  id={`item-description-${item.key}`}
                  rows={4}
                  maxLength={10000}
                  value={item.description}
                  onChange={(event) => {
                    changeItem(item.key, { description: event.target.value });
                  }}
                />
              </div>
              <div className="flex justify-end gap-1">
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  aria-label={`上移子项 ${String(index + 1)}`}
                  disabled={index === 0}
                  onClick={() => {
                    move(index, -1);
                  }}
                >
                  <ArrowUp aria-hidden="true" />
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  aria-label={`下移子项 ${String(index + 1)}`}
                  disabled={index === items.length - 1}
                  onClick={() => {
                    move(index, 1);
                  }}
                >
                  <ArrowDown aria-hidden="true" />
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => {
                    setItems((current) => current.filter((entry) => entry.key !== item.key));
                  }}
                >
                  <Trash2 aria-hidden="true" />
                  移除
                </Button>
              </div>
            </fieldset>
          ))}
          {!items.length ? <p className="text-sm text-muted-foreground">暂无评分子项</p> : null}
        </div>
      </fieldset>
    </Drawer>
  );
}

function RecordSelector({
  workspaceID,
  students,
  canQuery,
  value,
  fallback,
  onChange,
}: {
  workspaceID: string;
  students: boolean;
  canQuery: boolean;
  value: number;
  fallback: string;
  onChange: (id: number) => void;
}) {
  const client = useQueryClient();
  const [keyword, setKeyword] = useState('');
  const [search, setSearch] = useState('');
  const [selected, setSelected] = useState<{ value: string; label: string } | undefined>();
  const reload = useApiAction();
  const query = useQuery({ ...projectOptions(workspaceID, students, search), enabled: canQuery });
  const applySearch = async (next: string) => {
    await client.query({ ...projectOptions(workspaceID, students, next), staleTime: 0 });
    setSearch(next);
  };
  const name = students ? '学生' : '项目';
  const options = (query.data ?? []).map((item) => ({
    value: String(item.value),
    label: item.label,
  }));
  const selectedValue = value ? String(value) : '';
  if (selectedValue && !options.some((item) => item.value === selectedValue)) {
    options.push({
      value: selectedValue,
      label:
        (selected?.value === selectedValue ? selected.label : fallback) ||
        (students ? '未知学生' : `${name} #${selectedValue}`),
    });
  }
  return (
    <div className="space-y-2">
      <Label htmlFor={`participation-${students ? 'student' : 'project'}`}>{name}</Label>
      {canQuery ? (
        <>
          <SearchSelect
            id={`participation-${students ? 'student' : 'project'}`}
            label={name}
            className="w-full"
            options={options}
            value={selectedValue}
            onValueChange={(id) => {
              setSelected(options.find((item) => item.value === id));
              onChange(Number(id));
            }}
            loading={query.isFetching}
          />
          <div className="flex flex-wrap gap-2">
            <Input
              aria-label={`检索${name}关键词`}
              placeholder={`${name}关键词`}
              className="min-w-24 flex-1"
              maxLength={100}
              value={keyword}
              onChange={(event) => {
                setKeyword(event.target.value);
              }}
            />
            <div className="flex gap-2">
              <SearchResetButton
                loading={reload.isPending('reset')}
                disabled={reload.pending}
                onClick={() => {
                  void reload.run('reset', async () => {
                    setKeyword('');
                    await applySearch('');
                  });
                }}
              />
              <Button
                type="button"
                variant="outline"
                loading={reload.isPending('search')}
                disabled={reload.pending}
                onClick={() => {
                  void reload.run('search', () => applySearch(keyword.trim()));
                }}
              >
                <RefreshCw aria-hidden="true" />
                搜索
              </Button>
            </div>
          </div>
          {query.error ? <ErrorMessage>{query.error.message}</ErrorMessage> : null}
        </>
      ) : (
        <Input
          id={`participation-${students ? 'student' : 'project'}`}
          type="number"
          min={1}
          required
          value={value || ''}
          onChange={(event) => {
            onChange(Number(event.target.value));
          }}
        />
      )}
      {!canQuery && fallback ? (
        <p className="text-xs wrap-anywhere text-muted-foreground">{fallback}</p>
      ) : null}
    </div>
  );
}

function DetailNameInput({
  id,
  value,
  usedNames,
  workspaceID,
  projectID,
  canQuery,
  onChange,
}: {
  id: string;
  value: string;
  usedNames: string[];
  workspaceID: string;
  projectID: number;
  canQuery: boolean;
  onChange: (value: string) => void;
}) {
  const [keyword, setKeyword] = useState('');
  const names = useQuery({
    ...detailNameOptions(workspaceID, projectID, keyword),
    enabled: canQuery && projectID > 0,
  });
  const options = (names.data ?? []).filter(
    (item) => item.value === value || !usedNames.includes(item.value),
  );
  if (value && !options.some((item) => item.value === value)) options.push({ value, label: value });
  return (
    <>
      <SearchSelect
        id={id}
        label="子项名称"
        options={options}
        value={value}
        onValueChange={onChange}
        onSearchChange={setKeyword}
        loading={names.isFetching}
        disabled={!projectID || !canQuery}
        maxLength={100}
        className="w-full"
      />
      {names.error ? <ErrorMessage>{names.error.message}</ErrorMessage> : null}
    </>
  );
}
