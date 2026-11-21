import { RotateCcw } from 'lucide-react';
import type { ComponentProps } from 'react';

import { Button } from '@/components/ui/button';

export function SearchResetButton(
  props: Pick<ComponentProps<typeof Button>, 'onClick' | 'loading' | 'disabled'>,
) {
  return (
    <Button type="button" variant="outline" {...props}>
      <RotateCcw aria-hidden="true" />
      重置
    </Button>
  );
}
