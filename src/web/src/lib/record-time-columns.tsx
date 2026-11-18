import type { ColumnDef } from '@tanstack/react-table';

import { TableTimestamp } from '@/components/table-timestamp';

interface RecordTimes {
  created_at: string;
  updated_at: string;
}

// 业务记录统一在操作列前展示创建时间和更新时间，字段标识保持稳定。
export function recordTimeColumns<T>(timestamps: (record: T) => RecordTimes): ColumnDef<T>[] {
  return [
    {
      id: 'created_at',
      header: '创建时间',
      cell: ({ row }) => <TableTimestamp value={timestamps(row.original).created_at} />,
    },
    {
      id: 'updated_at',
      header: '更新时间',
      cell: ({ row }) => <TableTimestamp value={timestamps(row.original).updated_at} />,
    },
  ];
}
