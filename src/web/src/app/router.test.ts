import type { RouteObject } from 'react-router';
import { expect, it } from 'vitest';

import { router } from '@/app/router';
import {
  emailLoginPath,
  studentLoginPath,
  studentProfilePath,
  studentServicePath,
} from '@/features/user/navigation';

import { pagePaths } from '../../scripts/cdn-urls';

function registeredPaths(routes: RouteObject[], parent = ''): string[] {
  return routes.flatMap((route) => {
    if (!route.path || route.path === '*') {
      return registeredPaths(route.children ?? [], parent);
    }
    const path = route.path.startsWith('/') ? route.path : `${parent}/${route.path}`;
    return [path, ...registeredPaths(route.children ?? [], path === '/' ? '' : path)];
  });
}

it('所有页面使用固定路径并纳入 CDN 刷新清单', () => {
  const paths = registeredPaths(router.routes);
  expect(paths.filter((path) => /[:*?]/.test(path))).toEqual([]);
  expect(new Set(pagePaths)).toEqual(new Set(paths));
  expect(paths).toEqual(
    expect.arrayContaining([
      emailLoginPath,
      studentLoginPath,
      studentServicePath,
      studentProfilePath,
      '/stu/roster',
      '/stu/projects',
      '/stu/monthly-work',
      '/stu/graduation',
      '/stu/dashboard',
      '/cpc/monthly-work',
      '/cpc/dashboard',
      '/cpc/data-management',
    ]),
  );
  expect(paths).not.toContain('/users');
  expect(paths.some((path) => path === '/workspaces' || path.startsWith('/workspaces/'))).toBe(
    false,
  );
});
