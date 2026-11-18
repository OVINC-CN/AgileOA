import { readdir } from 'node:fs/promises';
import { relative, resolve, sep } from 'node:path';
import { argv, stderr, stdout } from 'node:process';

// 新增或调整前端路由时同步更新固定路径，由路由覆盖测试检查。
export const pagePaths = [
  '/',
  '/login',
  // 兼容旧邮件入口，仅返回验证码登录页，固定路径仍纳入刷新。
  '/login/email',
  '/login/student',
  '/student-service',
  '/student-service/profile',
  '/student-service/projects',
  '/change-password',
  '/system',
  '/system/users',
  '/system/workspaces',
  '/system/permissions',
  '/system/audit',
  '/stu',
  '/stu/roster',
  '/stu/projects',
  '/stu/monthly-work',
  '/stu/graduation',
  '/stu/dashboard',
  '/cpc',
  '/cpc/roster',
  '/cpc/dashboard',
  '/cpc/data-management',
  '/cpc/monthly-work',
] as const;

async function listAssetPaths(dist: string) {
  // 使用实际部署产物，包含 HTML、独立配置和带哈希的静态资源。
  const entries = await readdir(dist, { recursive: true, withFileTypes: true });
  const paths = entries
    .filter((entry) => entry.isFile())
    .map((entry) => relative(dist, resolve(entry.parentPath, entry.name)));
  if (!paths.includes('index.html')) {
    throw new Error('CDN_WEB_DIST 中没有 index.html，请指定前端部署产物目录');
  }
  return paths.map((path) => `/${path.split(sep).map(encodeURIComponent).join('/')}`);
}

async function main() {
  const [, , originValue, dist] = argv;
  if (!originValue || !dist) {
    throw new Error('请设置 CDN_ORIGIN 和 CDN_WEB_DIST');
  }
  const origin = URL.parse(originValue);
  if (
    !origin ||
    !['http:', 'https:'].includes(origin.protocol) ||
    origin.href !== `${origin.origin}/`
  ) {
    throw new Error('CDN_ORIGIN 必须是前端站点的 HTTP 或 HTTPS 源地址');
  }

  // 资源扫描完成后再输出，避免失败时留下部分清单。
  const assets = await listAssetPaths(resolve(dist));
  const paths = [...pagePaths, ...assets];
  const urls = new Set(paths.map((path) => new URL(path, origin).href));
  stdout.write(`${[...urls].sort().join('\n')}\n`);
}

if (import.meta.main) {
  try {
    await main();
  } catch (error) {
    stderr.write(`${error instanceof Error ? error.message : '生成 CDN 刷新清单失败'}\n`);
    process.exitCode = 1;
  }
}
