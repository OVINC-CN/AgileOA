import { formatValue } from '@/lib/format';
import { type TableColumn, useTableColumns } from '@/lib/table-columns';

export function ChartTooltipTable({
  tableID,
  caption,
  columns,
  rows,
  footer,
  fixedColumns = false,
  lastRowID,
}: {
  tableID: string;
  caption: string | number | undefined;
  columns: TableColumn[];
  rows: { id: string; label: string; color: string | undefined; value: number; cells: string[] }[];
  footer?: { label: string; cells: string[] };
  fixedColumns?: boolean;
  lastRowID?: string | undefined;
}) {
  const indexedColumns = columns.map((column, index) => ({ ...column, index }));
  const settings = useTableColumns(tableID, indexedColumns);
  const visibleColumns = fixedColumns ? indexedColumns : settings.visibleColumns;
  if (!rows.length) return null;
  const displayedRows = rows.toSorted(
    (a, b) => Number(a.id === lastRowID) - Number(b.id === lastRowID) || b.value - a.value,
  );

  return (
    <div
      role="status"
      className="glass-panel pointer-events-none max-w-[min(36rem,calc(100vw-2rem))] rounded-xl border bg-popover p-3 text-xs shadow-popover"
    >
      <table className="w-full text-left leading-4 whitespace-nowrap">
        <caption className="mb-1 text-left font-medium">{formatValue(caption)}</caption>
        <thead className="border-b border-border text-muted-foreground">
          <tr>
            {visibleColumns.map((column) => (
              <th
                key={column.id}
                scope="col"
                className={
                  column.index === 0 ? 'px-2 py-1 font-medium' : 'px-2 py-1 text-right font-medium'
                }
              >
                {column.label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="tabular-nums">
          {displayedRows.map((row) => (
            <tr key={row.id}>
              {visibleColumns.map((column) =>
                column.index === 0 ? (
                  <th key={column.id} scope="row" className="px-2 py-1 font-normal">
                    <span className="flex items-center gap-1.5">
                      <span
                        aria-hidden="true"
                        className="size-2 shrink-0 rounded-xs"
                        style={{ backgroundColor: row.color }}
                      />
                      <span className="max-w-[min(24rem,45vw)] truncate">
                        {formatValue(row.label)}
                      </span>
                    </span>
                  </th>
                ) : (
                  <td key={column.id} className="px-2 py-1 text-right">
                    {formatValue(row.cells[column.index - 1])}
                  </td>
                ),
              )}
            </tr>
          ))}
        </tbody>
        {footer ? (
          <tfoot className="border-t border-border font-medium tabular-nums">
            <tr>
              {visibleColumns.map((column) =>
                column.index === 0 ? (
                  <th key={column.id} scope="row" className="px-2 py-1">
                    {footer.label}
                  </th>
                ) : (
                  <td key={column.id} className="px-2 py-1 text-right">
                    {formatValue(footer.cells[column.index - 1])}
                  </td>
                ),
              )}
            </tr>
          </tfoot>
        ) : null}
      </table>
    </div>
  );
}
