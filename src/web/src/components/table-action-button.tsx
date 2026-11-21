import type { ComponentProps } from 'react';

import { Button } from '@/components/ui/button';

export function TableActionButton(props: Omit<ComponentProps<typeof Button>, 'variant' | 'size'>) {
  return <Button variant="table-action" size="sm" {...props} />;
}
