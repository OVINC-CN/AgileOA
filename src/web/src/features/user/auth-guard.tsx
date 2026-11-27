import { useQueryClient } from '@tanstack/react-query';
import { RefreshCw } from 'lucide-react';
import { useEffect, useState } from 'react';
import { Navigate, Outlet, useLocation } from 'react-router';

import { ErrorMessage } from '@/components/error-message';
import { Button } from '@/components/ui/button';
import { currentUserKey, useCurrentUser } from '@/features/user/api';
import {
  authURL,
  changePasswordPath,
  readLoginState,
  studentAwardsPath,
  studentLoginPath,
  studentProfilePath,
  studentServicePath,
  studentServiceURL,
} from '@/features/user/navigation';
import { clearWatermarkPreference } from '@/features/user/watermark';
import { useApiAction } from '@/lib/api-action';
import { useLoadingDelay } from '@/lib/loading';

export function AuthGuard({
  allowGuest = false,
  passwordChange = false,
}: {
  allowGuest?: boolean;
  passwordChange?: boolean;
}) {
  const client = useQueryClient();
  const currentUser = useCurrentUser();
  const location = useLocation();
  const from = passwordChange
    ? readLoginState(location.state, location.search).from
    : location.pathname + location.search + location.hash;
  const [expired, setExpired] = useState(false);
  const retry = useApiAction();
  const loading = useLoadingDelay(currentUser.isPending);

  useEffect(() => {
    // 认证撤销后清理所有业务数据，避免其他账号复用缓存。
    const expire = () => {
      clearWatermarkPreference();
      if (!allowGuest) setExpired(true);
      void client.cancelQueries().then(() => {
        client.removeQueries({
          predicate: (query) => query.queryKey[0] !== 'user' || query.queryKey[1] !== 'current',
        });
        client.setQueryData(currentUserKey, null);
      });
    };
    window.addEventListener('auth:expired', expire);
    return () => {
      window.removeEventListener('auth:expired', expire);
    };
  }, [allowGuest, client]);

  useEffect(() => {
    if (currentUser.data === null) {
      client.removeQueries({
        predicate: (query) => query.queryKey[0] !== 'user' || query.queryKey[1] !== 'current',
      });
    }
  }, [client, currentUser.data]);

  if (currentUser.data?.password_change_required && !passwordChange && !expired) {
    return <Navigate to={authURL(changePasswordPath, from)} replace />;
  }
  if (currentUser.data?.identity_type === 'student' && !expired) {
    const target = studentServiceURL(currentUser.data.workspace_id);
    const workspaces = new URLSearchParams(location.search).getAll('workspace');
    if (
      ![studentProfilePath, studentAwardsPath].includes(location.pathname) ||
      workspaces.length !== 1 ||
      workspaces[0] !== currentUser.data.workspace_id
    ) {
      return <Navigate to={target} replace />;
    }
  }
  if (allowGuest) return <Outlet />;
  const inStudentService =
    location.pathname === studentServicePath ||
    location.pathname.startsWith(`${studentServicePath}/`);
  const loginPath = inStudentService ? studentLoginPath : '/login';
  if (expired) {
    return <Navigate to={authURL(loginPath, from, true)} replace />;
  }
  if (retry.pending || currentUser.isError) {
    return (
      <div className="flex min-h-svh flex-col items-center justify-center gap-4 p-4">
        <ErrorMessage>{currentUser.error?.message}</ErrorMessage>
        <Button
          loading={retry.pending}
          onClick={() => {
            void retry.run('retry', () => currentUser.refetch());
          }}
        >
          <RefreshCw aria-hidden="true" />
          重新加载
        </Button>
      </div>
    );
  }
  if (loading) {
    return <main aria-busy="true" className="min-h-svh" />;
  }
  if (!currentUser.data) {
    return <Navigate to={authURL(loginPath, from)} replace />;
  }
  if (inStudentService && currentUser.data.identity_type !== 'student') {
    return <Navigate to="/" replace />;
  }
  if (passwordChange && !currentUser.data.password_change_required) {
    return <Navigate to={from} replace />;
  }
  return <Outlet />;
}
