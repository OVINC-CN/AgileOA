import { isRouteErrorResponse, Link, useRouteError } from 'react-router';

import { ErrorMessage } from '@/components/error-message';
import { Button } from '@/components/ui/button';

export function ErrorPage() {
  const error: unknown = useRouteError();
  const message =
    isRouteErrorResponse(error) && error.status === 404 ? '页面不存在' : '页面暂时无法加载';

  return (
    <main className="flex min-h-svh flex-col items-center justify-center gap-4 px-4 text-center">
      <ErrorMessage>{message}</ErrorMessage>

      <Button asChild>
        <Link to="/" reloadDocument>
          返回工作空间
        </Link>
      </Button>
    </main>
  );
}
