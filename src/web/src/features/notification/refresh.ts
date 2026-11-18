import type { QueryClient, QueryKey } from '@tanstack/react-query';

import { isNotificationURL } from '@/features/notification/navigation';
import { currentUserOptions } from '@/features/user/api';
import { workspaceContextOptions, workspaceIDSchema } from '@/features/workspace/api';
import { workspaceQueryParam } from '@/features/workspace/navigation';

export async function refreshNotificationTarget(client: QueryClient, value: string) {
  // 只刷新固定业务页面对应的工作空间，链接中的其他参数不参与缓存范围推导。
  if (!value || !isNotificationURL(value)) return;
  const target = new URL(value, window.location.origin);
  const workspace = workspaceIDSchema.safeParse(target.searchParams.get(workspaceQueryParam));
  if (!workspace.success) return;
  let queryKey: QueryKey;
  let studentService = false;
  switch (target.pathname) {
    case '/stu/monthly-work':
    case '/cpc/monthly-work': {
      const { workKey } = await import('@/features/monthly-work/api');
      queryKey = workKey(workspace.data, target.pathname.startsWith('/cpc/') ? 'cpc' : 'student');
      break;
    }
    case '/cpc/roster': {
      const { personKey } = await import('@/features/cpc-person/api');
      queryKey = personKey(workspace.data);
      break;
    }
    case '/stu/projects':
    case '/student-service/projects': {
      const { projectKey } = await import('@/features/project/api');
      queryKey = projectKey(workspace.data);
      studentService = target.pathname === '/student-service/projects';
      break;
    }
    default:
      return;
  }

  // 同页跳转也重新检查空间权限，取消旧请求后再刷新，避免旧响应覆盖最新结果。
  if (studentService) {
    const user = await client.query({ ...currentUserOptions, staleTime: 0 });
    if (user?.identity_type !== 'student' || user.workspace_id !== workspace.data) {
      throw new Error('学生登录已失效，请重新登录');
    }
  } else {
    await client.query({ ...workspaceContextOptions(workspace.data), staleTime: 0 });
  }
  await client.cancelQueries({ queryKey });
  // 未打开的详情重新加载时不展示旧快照，当前正在展示的数据保留到刷新结束。
  client.removeQueries({ queryKey, type: 'inactive' });
  await client.invalidateQueries({ queryKey, refetchType: 'active' }, { throwOnError: true });
}
