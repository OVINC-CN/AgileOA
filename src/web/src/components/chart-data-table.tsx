import { ChevronDown, Download } from 'lucide-react';
import { Collapsible } from 'radix-ui';

import { Button } from '@/components/ui/button';
import { useApiAction } from '@/lib/api-action';
import { formatValue } from '@/lib/format';
import { downloadXLSX } from '@/lib/spreadsheet';
import type { TableColumn } from '@/lib/table-columns';

export function ChartDataTable({
  caption,
  columns,
  rows,
}: {
  caption: string;
  columns: TableColumn[];
  rows: { id: string; cells: (string | number)[] }[];
}) {
  const download = useApiAction();
  return (
    <Collapsible.Root className="border-t border-border pt-4 text-xs text-muted-foreground">
      <Collapsible.Trigger asChild>
        <Button type="button" variant="ghost" size="xs" className="group">
          <ChevronDown
            aria-hidden="true"
            className="-rotate-90 transition-transform group-data-[state=open]:rotate-0 motion-reduce:transition-none"
          />
          查看人数数据
        </Button>
      </Collapsible.Trigger>
      <Collapsible.Content>
        <div className="mt-3 flex flex-wrap justify-end gap-2 border-t border-border pt-2">
          <Button
            type="button"
            variant="outline"
            size="xs"
            loading={download.pending}
            disabled={rows.length === 0}
            onClick={() => {
              void download.run('xlsx', () =>
                downloadXLSX(`${caption}.xlsx`, [
                  columns.map((column) => column.label),
                  ...rows.map((row) => row.cells),
                ]),
              );
            }}
          >
            <Download aria-hidden="true" />
            下载 XLSX
          </Button>
        </div>
        <div className="mt-2 overflow-x-auto">
          <table className="w-full text-left whitespace-nowrap">
            <caption className="sr-only">{caption}</caption>
            <thead>
              <tr>
                {columns.map((column) => (
                  <th key={column.id} scope="col" className="px-2 py-0.5 leading-4">
                    {column.label}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map(({ id, cells }) => (
                <tr key={id}>
                  {columns.map((column, index) => (
                    <td key={column.id} className="px-2 py-0.5 leading-4">
                      {formatValue(cells[index])}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Collapsible.Content>
    </Collapsible.Root>
  );
}
