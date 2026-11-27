import { Copy } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { emptyValueLabel, formatValue } from '@/lib/format';
import { showMessage } from '@/lib/messages';

export function DetailValue({
  label,
  value,
  copyable,
}: {
  label: string;
  value: string;
  copyable: boolean;
}) {
  const formattedValue = formatValue(value);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(value);
      showMessage('已复制', undefined, { tone: 'success' });
    } catch {
      showMessage('复制失败，请重试');
    }
  };
  return (
    <dd className="group min-w-0 text-sm wrap-anywhere whitespace-pre-wrap">
      {formattedValue}
      {copyable && formattedValue !== emptyValueLabel ? (
        <Button
          type="button"
          size="icon-xs"
          variant="ghost"
          aria-label={`复制${label}`}
          title={`复制${label}`}
          className="ml-1 align-middle text-muted-foreground opacity-0 group-hover:opacity-100 focus-visible:opacity-100 [@media(hover:none)]:opacity-100"
          onClick={() => {
            void copy();
          }}
        >
          <Copy aria-hidden="true" />
        </Button>
      ) : null}
    </dd>
  );
}
