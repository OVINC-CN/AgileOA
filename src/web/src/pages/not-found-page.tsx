import { Link } from 'react-router';

import { ErrorMessage } from '@/components/error-message';
import { Button } from '@/components/ui/button';

export function NotFoundPage() {
  return (
    <div className="space-y-4 py-16 text-center">
      <ErrorMessage>页面不存在</ErrorMessage>
      <Button asChild>
        <Link to="/">返回工作空间</Link>
      </Button>
    </div>
  );
}
