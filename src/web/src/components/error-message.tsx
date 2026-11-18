import { type ReactNode, useEffect } from 'react';

import { showMessage } from '@/lib/messages';

export function ErrorMessage({ children, id }: { children: ReactNode; id?: string }) {
  const message = Array.isArray(children)
    ? children.filter((value) => typeof value === 'string').join('')
    : typeof children === 'string'
      ? children
      : '';
  useEffect(() => {
    if (message) showMessage(message, id);
  }, [message, id]);
  return null;
}
