import { useCallback, useState } from 'react';

import { useApiAction } from '@/lib/api-action';
import { showMessage } from '@/lib/messages';

export interface DeleteConfirmationAction {
  target: string;
  onConfirm: (() => void | Promise<void>) | (() => Promise<boolean>);
  blocked?: string | undefined;
}

export function useDeleteConfirmation() {
  const [target, setTarget] = useState<string | null>(null);
  const action = useApiAction();
  const cancel = useCallback(() => {
    if (!action.pending) setTarget(null);
  }, [action.pending]);

  const select = (next: DeleteConfirmationAction, onSuccess?: () => void) => {
    if (action.pending) return;
    if (next.blocked !== undefined) {
      cancel();
      showMessage(next.blocked);
      return;
    }
    if (target !== next.target) {
      setTarget(next.target);
      return;
    }

    void action.run('delete', async () => {
      if ((await next.onConfirm()) !== false) {
        setTarget(null);
        onSuccess?.();
      }
    });
  };

  return { target, pending: action.pending, select, cancel };
}
