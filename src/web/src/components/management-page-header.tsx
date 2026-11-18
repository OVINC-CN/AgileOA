import type { ReactNode } from 'react';
import { Link } from 'react-router';

export function ManagementPageHeader({
  id,
  title,
  children,
}: {
  id: string;
  title: string;
  children?: ReactNode;
}) {
  return (
    <header className="flex min-h-8 flex-wrap items-center justify-between gap-4">
      <h1 id={id} className="sr-only">
        {title}
      </h1>
      <nav
        aria-label="面包屑"
        className="flex h-8 items-center text-xs leading-4 text-muted-foreground"
      >
        <ol className="flex items-center gap-2">
          <li>
            <Link to="/system" className="transition-colors hover:text-foreground">
              系统管理
            </Link>
          </li>
          <li aria-hidden="true" className="text-muted-foreground">
            /
          </li>
          <li aria-current="page">{title}</li>
        </ol>
      </nav>
      {children ? <div className="ml-auto flex items-center gap-2">{children}</div> : null}
    </header>
  );
}
