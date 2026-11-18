import { describe, expect, it } from 'vitest';

import { readLoginState } from '@/features/user/navigation';

describe('认证回跳地址', () => {
  it('优先读取 URL 中的完整目标地址，兼容已有路由状态', () => {
    const from = '/stu/roster?workspace=w7m2k9p4s6n8c3x5&student_id=12#details';
    const params = new URLSearchParams({ redirect: from });
    expect(readLoginState({ from: '/system/users' }, `?${params.toString()}`)).toEqual({
      from,
      expired: false,
    });
    expect(readLoginState({ from, expired: true })).toEqual({ from, expired: true });
    expect(readLoginState(null)).toEqual({ from: '/', expired: false });
  });

  it.each([
    '',
    'https://example.com',
    '//example.com',
    '/\\example.com',
    '/\texample.com',
    '/%2fexample.com',
    '/broken%zz',
    '/login?redirect=%2Fsystem%2Fusers',
    '/LoGiN/',
    '/%6cogin',
    '/system/../login',
    '/login/email',
    '/login/student',
    `/login/email?code=${'a'.repeat(64)}`,
    '/LoGiN/EmAiL?code=invalid',
    '/%6cogin/email?code=invalid',
    '/change-password?redirect=%2Fsystem%2Fusers',
    '/change-password/',
    '/change%2dpassword',
  ])('拒绝非法或认证页面目标 %j，同时保留重新登录要求', (from) => {
    const params = new URLSearchParams({ redirect: from, expired: '1' });
    expect(readLoginState(null, `?${params.toString()}`)).toEqual({ from: '/', expired: true });
  });
});
