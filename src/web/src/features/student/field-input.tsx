import { useQuery } from '@tanstack/react-query';
import { useId, useState } from 'react';

import { ErrorMessage } from '@/components/error-message';
import { FilterField } from '@/components/filter-panel';
import { DatePicker } from '@/components/ui/date-picker';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { SearchSelect } from '@/components/ui/search-select';
import { Textarea } from '@/components/ui/textarea';
import { studentOptions } from '@/features/student/api';
import type { StudentField } from '@/features/student/fields';
import { studentFormPresets } from '@/features/student/form-presets';
import { useInitialLoading } from '@/lib/api-action';
import { emptyValueLabel } from '@/lib/format';
import { mergeTextFieldOptions } from '@/lib/text-field-options';

export function StudentFieldInput({
  field,
  value,
  onChange,
  workspaceID,
  canQuery,
  disabled = false,
  required = field.required ?? false,
  invalid = false,
}: {
  field: StudentField;
  value: string;
  onChange: (value: string) => void;
  workspaceID: string;
  canQuery: boolean;
  disabled?: boolean;
  required?: boolean;
  invalid?: boolean;
}) {
  const id = useId();
  const [open, setOpen] = useState(false);
  const accommodation = field.key === 'accommodation_type';
  const presets = studentFormPresets(field.key);
  const query = useQuery({
    ...studentOptions(workspaceID, field.key),
    enabled: open && canQuery && field.enumerable && !accommodation,
  });
  const loading = useInitialLoading(open && query.isFetching);
  const options = mergeTextFieldOptions(
    presets,
    [...field.options, ...(query.data ?? [])],
    field.options.length === 0 ? value : '',
  );
  return (
    <div className="space-y-2" data-student-field={field.key}>
      <Label htmlFor={id}>
        {field.label}
        {required ? ' *' : ''}
      </Label>
      {field.type === 'month' ? (
        <DatePicker
          id={id}
          label={field.label}
          value={value}
          onValueChange={onChange}
          mode="month"
          disabled={disabled}
          required={required}
          invalid={invalid}
        />
      ) : field.enumerable || presets.length > 0 ? (
        <SearchSelect
          id={id}
          label={field.label}
          className="w-full"
          options={
            required
              ? options.filter(
                  (option) => option.value !== '' && (!accommodation || option.value !== 'unknown'),
                )
              : accommodation
                ? field.options
                : [{ value: '', label: emptyValueLabel }, ...options]
          }
          value={accommodation && !required ? value || 'unknown' : value}
          required={required}
          invalid={invalid}
          disabled={disabled}
          loading={loading}
          allowCreate={field.options.length === 0}
          maxLength={field.maxLength}
          onValueChange={onChange}
          onOpenChange={setOpen}
        />
      ) : field.maxLength > 500 ? (
        <Textarea
          id={id}
          value={value}
          maxLength={field.maxLength}
          disabled={disabled}
          required={required}
          aria-invalid={invalid || undefined}
          autoComplete="off"
          data-1p-ignore="true"
          onChange={(event) => {
            onChange(event.target.value);
          }}
        />
      ) : (
        <Input
          id={id}
          value={value}
          maxLength={field.maxLength}
          disabled={disabled}
          required={required}
          aria-invalid={invalid || undefined}
          autoComplete="off"
          data-1p-ignore="true"
          onChange={(event) => {
            onChange(event.target.value);
          }}
        />
      )}
      {query.error && open ? <ErrorMessage>{query.error.message}</ErrorMessage> : null}
    </div>
  );
}

export function StudentFilterSelect({
  field,
  value,
  onChange,
  workspaceID,
  canQuery,
  defaultOption,
}: {
  field: StudentField;
  value: string[];
  onChange: (value: string[]) => void;
  workspaceID: string;
  canQuery: boolean;
  defaultOption?: { value: string; label: string } | undefined;
}) {
  const [open, setOpen] = useState(false);
  const query = useQuery({ ...studentOptions(workspaceID, field.key), enabled: open && canQuery });
  const loading = useInitialLoading(open && query.isFetching);
  const options = [...(query.data ?? [])];
  if (defaultOption && !options.some((option) => option.value === defaultOption.value)) {
    options.unshift(defaultOption);
  }
  return (
    <FilterField label={field.label}>
      <SearchSelect
        multiple
        label={field.label}
        className="w-full"
        options={options}
        value={value}
        onValueChange={onChange}
        loading={loading}
        disabled={!canQuery}
        onOpenChange={setOpen}
      />
      {query.error ? <ErrorMessage>{query.error.message}</ErrorMessage> : null}
    </FilterField>
  );
}
