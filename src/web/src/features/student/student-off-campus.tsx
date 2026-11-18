import { type ChangeEvent, useId } from 'react';

import { DetailValue } from '@/components/detail-value';
import { DatePicker } from '@/components/ui/date-picker';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import type { OffCampusValues } from '@/features/student/api';

const fields = [
  { key: 'start_date', label: '校外住宿开始日期', date: true },
  { key: 'end_date', label: '校外住宿结束日期', date: true },
  { key: 'category', label: '校外住宿类别', maxLength: 64 },
  { key: 'rental_method', label: '租住方式', maxLength: 64 },
  { key: 'landlord_name', label: '房东姓名', maxLength: 50 },
  { key: 'landlord_phone', label: '房东联系电话', maxLength: 32 },
  { key: 'roommate_name', label: '合租人姓名', maxLength: 50 },
  { key: 'roommate_unit', label: '合租人单位', maxLength: 100 },
  { key: 'roommate_phone', label: '合租人联系电话', maxLength: 32 },
  { key: 'address', label: '校外居住地详细地址', maxLength: 255 },
  { key: 'reason', label: '校外住宿原因', maxLength: 1000 },
] as const satisfies readonly {
  key: keyof OffCampusValues;
  label: string;
  maxLength?: number;
  date?: true;
}[];

export function StudentOffCampusDetails({ data }: { data: OffCampusValues }) {
  return (
    <dl className="grid grid-cols-1 gap-5 sm:grid-cols-2">
      {fields.map(({ key, label }) => (
        <div
          key={key}
          className={key === 'reason' ? 'min-w-0 space-y-1.5 sm:col-span-2' : 'min-w-0 space-y-1.5'}
        >
          <dt className="text-xs text-muted-foreground">{label}</dt>
          <DetailValue label={label} value={data[key]} copyable={data[key] !== ''} />
        </div>
      ))}
    </dl>
  );
}

export function StudentOffCampusFields({
  values,
  onChange,
}: {
  values: OffCampusValues;
  onChange: (key: keyof OffCampusValues, value: string) => void;
}) {
  const id = useId();
  return (
    <div className="grid grid-cols-1 gap-5 sm:grid-cols-2">
      {fields.map((field) => {
        const { key, label } = field;
        const inputID = `${id}-${key}`;
        const multiline = key === 'reason' || key === 'address';
        const props = {
          id: inputID,
          value: values[key],
          autoComplete: 'off',
          'data-1p-ignore': 'true',
          onChange: (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
            onChange(key, event.target.value);
          },
        };
        return (
          <div key={key} className={multiline ? 'space-y-2 sm:col-span-2' : 'space-y-2'}>
            <Label htmlFor={inputID}>{label}</Label>
            {multiline ? (
              <Textarea {...props} maxLength={field.maxLength} />
            ) : 'date' in field ? (
              <DatePicker
                id={inputID}
                label={label}
                value={values[key]}
                min="1000-01-01"
                max="9999-12-31"
                onValueChange={(value) => {
                  onChange(key, value);
                }}
              />
            ) : (
              <Input {...props} maxLength={field.maxLength} />
            )}
          </div>
        );
      })}
    </div>
  );
}
