import { Navigate, useLocation } from 'react-router';

import {
  authURL,
  readLoginState,
  studentLoginPath,
  studentServicePath,
} from '@/features/user/navigation';

export function EmailLoginPage() {
  const location = useLocation();
  const { from, expired } = readLoginState(null, location.search);
  // 兼容旧邮件入口，立即移除 URL 中的旧凭据，统一返回验证码登录页。
  return (
    <Navigate
      to={authURL(from.startsWith(studentServicePath) ? studentLoginPath : '/login', from, expired)}
      replace
    />
  );
}
