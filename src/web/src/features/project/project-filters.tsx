import { FilterField } from '@/components/filter-panel';
import { Input } from '@/components/ui/input';
import { SearchSelect } from '@/components/ui/search-select';

interface FilterOption {
  value: string;
  label: string;
}

export function ProjectNameYearFilters({
  names,
  years,
  projectNames,
  selectedYears,
  loading,
  onProjectNamesChange,
  onYearsChange,
}: {
  names: FilterOption[];
  years: FilterOption[];
  projectNames: string[];
  selectedYears: string[];
  loading: boolean;
  onProjectNamesChange: (values: string[]) => void;
  onYearsChange: (values: string[]) => void;
}) {
  return (
    <>
      <FilterField label="项目名称">
        <SearchSelect
          multiple
          allLabel="全部项目"
          label="项目名称"
          className="w-full"
          options={names}
          value={projectNames}
          loading={loading}
          onValueChange={onProjectNamesChange}
        />
      </FilterField>
      <FilterField label="归属年份">
        <SearchSelect
          multiple
          allLabel="全部年份"
          label="归属年份"
          className="w-full"
          options={years}
          value={selectedYears}
          loading={loading}
          onValueChange={onYearsChange}
        />
      </FilterField>
    </>
  );
}

export function ProjectStudentFilter({
  id,
  value,
  onChange,
}: {
  id: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <FilterField label="学号或姓名" htmlFor={id}>
      <Input
        id={id}
        aria-label="学号或姓名"
        placeholder="学号或姓名"
        maxLength={100}
        value={value}
        onChange={(event) => {
          onChange(event.target.value);
        }}
      />
    </FilterField>
  );
}
