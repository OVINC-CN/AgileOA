import { useQueryClient } from '@tanstack/react-query';
import { Save } from 'lucide-react';
import { useState } from 'react';

import { Button } from '@/components/ui/button';
import { Drawer } from '@/components/ui/drawer';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { SearchSelect } from '@/components/ui/search-select';
import {
  type Graduation,
  type GraduationInput,
  graduationKey,
  type GraduationOption,
  pendingOption,
  saveGraduation,
} from '@/features/graduation/api';
import { studentKey } from '@/features/student/api';
import { useApiMutation } from '@/lib/api-action';
import { formatIdentity } from '@/lib/format';

export function GraduationEditor({
  workspaceID,
  record,
  types,
  onClose,
}: {
  workspaceID: string;
  record: Graduation;
  types: GraduationOption[];
  onClose: () => void;
}) {
  const [values, setValues] = useState<GraduationInput>(() => ({
    graduation_year: record.graduation_year,
    type: record.type || pendingOption.value,
    type_label: record.type_label,
    destination: record.destination,
  }));
  const client = useQueryClient();
  const mutation = useApiMutation({
    mutationFn: async () => {
      const result = await saveGraduation(workspaceID, record, values);
      await Promise.all([
        client.invalidateQueries({ queryKey: graduationKey(workspaceID) }),
        client.invalidateQueries({ queryKey: studentKey(workspaceID) }),
      ]);
      return result;
    },
    onSuccess: onClose,
  });
  const storedOptions = [
    pendingOption,
    ...types.filter((item) => item.value !== pendingOption.value),
  ];
  if (record.type && !storedOptions.some((item) => item.value === record.type)) {
    storedOptions.push({ value: record.type, label: record.type_label });
  }
  const options = [...storedOptions];
  const selected = values.type || values.type_label;
  if (selected && !options.some((item) => item.value === selected)) {
    options.push({ value: selected, label: values.type_label });
  }
  return (
    <Drawer
      title={record.type ? '编辑毕业去向' : '绑定毕业去向'}
      truncateDescription
      description={formatIdentity(record.student_name, record.student_no)}
      busy={mutation.isPending}
      onClose={onClose}
      form={{
        onSubmit: (event) => {
          event.preventDefault();
          mutation.mutate();
        },
      }}
      footer={
        <>
          <Button type="button" variant="outline" disabled={mutation.isPending} onClick={onClose}>
            取消
          </Button>
          <Button type="submit" loading={mutation.isPending}>
            <Save aria-hidden="true" />
            保存
          </Button>
        </>
      }
    >
      <fieldset disabled={mutation.isPending} className="space-y-5 p-6">
        <div className="space-y-2">
          <Label htmlFor="graduation-year">毕业年份</Label>
          <Input
            id="graduation-year"
            type="number"
            min={1900}
            max={9999}
            autoComplete="off"
            data-1p-ignore="true"
            value={values.graduation_year}
            onChange={(event) => {
              setValues((current) => ({ ...current, graduation_year: event.target.value }));
            }}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="graduation-type">类型</Label>
          <SearchSelect
            id="graduation-type"
            label="类型"
            className="w-full"
            options={options}
            value={selected}
            allowCreate
            maxLength={100}
            onValueChange={(value) => {
              const item = storedOptions.find((option) => option.value === value);
              setValues((current) => ({
                ...current,
                type: item?.value ?? '',
                type_label: item?.value === '' ? '' : (item?.label ?? value),
              }));
            }}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="graduation-destination">毕业去向</Label>
          <Input
            id="graduation-destination"
            maxLength={255}
            autoComplete="off"
            data-1p-ignore="true"
            value={values.destination}
            onChange={(event) => {
              setValues((current) => ({ ...current, destination: event.target.value }));
            }}
          />
        </div>
      </fieldset>
    </Drawer>
  );
}
