import { RefreshCw } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { useApiAction } from '@/lib/api-action';

export function RefreshButton({
  onRefresh,
  label = '刷新',
}: {
  onRefresh: () => Promise<unknown>;
  label?: string;
}) {
  const action = useApiAction();
  return (
    <Button
      variant="outline"
      size="sm"
      loading={action.pending}
      aria-label={label}
      onClick={() => {
        void action.run('refresh', onRefresh);
      }}
    >
      <RefreshCw aria-hidden="true" />
      刷新
    </Button>
  );
}
