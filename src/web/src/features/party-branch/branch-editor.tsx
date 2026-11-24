import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Save } from 'lucide-react';
import { useRef, useState } from 'react';

import { ErrorMessage } from '@/components/error-message';
import { Button } from '@/components/ui/button';
import { Drawer, type DrawerHandle } from '@/components/ui/drawer';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { SearchSelect } from '@/components/ui/search-select';
import { personKey } from '@/features/cpc-person/api';
import type { AdmissionRuleNode } from '@/features/party-branch/admission-rule';
import { AdmissionRuleFields } from '@/features/party-branch/admission-rule-editor';
import {
  type AdmissionRuleInfo,
  type PartyBranch,
  type PartyBranchInput,
  partyBranchKey,
  partyBranchSecretaryOptions,
  priorityLimits,
  savePartyBranch,
} from '@/features/party-branch/api';
import { ApiError } from '@/lib/api';
import { useApiMutation } from '@/lib/api-action';
import { formatIdentity } from '@/lib/format';

const fields = [
  { name: 'short_name', label: '党支部简称', limit: 100 },
  { name: 'full_name', label: '党支部全称', limit: 200 },
] as const;

export function BranchEditor({
  workspaceID,
  branch,
  admissionRule,
  onClose,
}: {
  workspaceID: string;
  branch: PartyBranch | null;
  admissionRule?: AdmissionRuleInfo | null;
  onClose: () => void;
}) {
  const [original] = useState(branch);
  const [rule, setRule] = useState<AdmissionRuleNode | null>(admissionRule?.rule ?? null);
  const [values, setValues] = useState<PartyBranchInput>(
    branch ?? { full_name: '', short_name: '', secretary_id: null, priority: 0 },
  );
  const client = useQueryClient();
  const secretaries = useQuery(partyBranchSecretaryOptions(workspaceID));
  const denied =
    secretaries.error instanceof ApiError && [401, 403, 404].includes(secretaries.error.status);
  const options = [
    { value: '', label: '未配置' },
    ...(denied ? [] : (secretaries.data ?? [])).map((item) => ({
      value: String(item.value),
      label: item.label,
    })),
  ];
  if (
    !denied &&
    original?.secretary_id &&
    !options.some((item) => item.value === String(original.secretary_id))
  ) {
    options.push({
      value: String(original.secretary_id),
      label: `${formatIdentity(original.secretary_name, original.secretary_username)}（原管理员）`,
    });
  }
  const drawer = useRef<DrawerHandle>(null);
  const mutation = useApiMutation({
    mutationFn: async () => {
      await savePartyBranch(workspaceID, original, values, admissionRule ? { rule } : undefined);
      await client.invalidateQueries({ queryKey: partyBranchKey(workspaceID) });
      await client.invalidateQueries({ queryKey: personKey(workspaceID) });
    },
    onSuccess: () => {
      drawer.current?.close();
    },
  });
  return (
    <Drawer
      ref={drawer}
      title={original ? '编辑党支部' : '新增党支部'}
      description="简称、全称必填，分别在当前空间内唯一；管理员可选"
      busy={mutation.isPending}
      onClose={onClose}
      form={{
        onSubmit: (event) => {
          event.preventDefault();
          mutation.mutate();
        },
      }}
      dismissLabel="取消"
      footer={
        <Button type="submit" loading={mutation.isPending}>
          <Save aria-hidden="true" />
          保存
        </Button>
      }
    >
      <fieldset disabled={mutation.isPending} className="space-y-5 p-6">
        {original ? (
          <p className="text-sm text-muted-foreground">编号：{String(original.id)}</p>
        ) : null}
        {fields.map(({ name, label, limit }) => (
          <div key={name} className="space-y-2">
            <Label htmlFor={`branch-${name}`}>{label}</Label>
            <Input
              id={`branch-${name}`}
              required
              maxLength={limit}
              value={values[name]}
              onChange={(event) => {
                setValues((current) => ({ ...current, [name]: event.target.value }));
              }}
            />
          </div>
        ))}
        <div className="space-y-2">
          <Label htmlFor="branch-priority">优先级</Label>
          <Input
            id="branch-priority"
            required
            type="number"
            min={priorityLimits.min}
            max={priorityLimits.max}
            step={1}
            value={Number.isNaN(values.priority) ? '' : values.priority}
            onChange={(event) => {
              setValues((current) => ({ ...current, priority: event.target.valueAsNumber }));
            }}
          />
          <p className="text-xs text-muted-foreground">
            数字越大越优先；相同时按编号从大到小匹配。
          </p>
        </div>
        <div className="space-y-2">
          <Label htmlFor="branch-secretary">管理员</Label>
          <SearchSelect
            id="branch-secretary"
            label="管理员"
            className="w-full"
            options={options}
            value={values.secretary_id === null ? '' : String(values.secretary_id)}
            loading={secretaries.isFetching}
            disabled={denied}
            onValueChange={(value) => {
              setValues((current) => ({ ...current, secretary_id: value ? Number(value) : null }));
            }}
          />
          {secretaries.error ? <ErrorMessage>{secretaries.error.message}</ErrorMessage> : null}
        </div>
        {admissionRule ? (
          <AdmissionRuleFields
            rule={rule}
            catalog={admissionRule}
            onChange={setRule}
            disabled={mutation.isPending}
          />
        ) : null}
      </fieldset>
    </Drawer>
  );
}
