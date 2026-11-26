import type { AppConfig } from '@/lib/config';

export async function initializeRum({
  aegis: options,
}: Pick<AppConfig, 'aegis'>): Promise<((uin: string) => void) | undefined> {
  try {
    // 可选监控只使用应用 ID 判断是否启用，其余配置由 SDK 处理。
    if (!options || typeof options !== 'object' || Array.isArray(options)) return;
    const id = 'id' in options && typeof options.id === 'string' ? options.id.trim() : '';
    if (!id) return;

    const { default: Aegis } = await import('aegis-web-sdk');
    const configuredUin = 'uin' in options ? options.uin : undefined;
    let currentUin =
      typeof configuredUin === 'string' || typeof configuredUin === 'number' ? configuredUin : '';

    const aegis = new Aegis(options);

    return (uin) => {
      if (currentUin === uin) return;
      try {
        aegis.setConfig({ uin });
        currentUin = uin;
      } catch {
        // 监控更新失败时继续处理登录状态。
      }
    };
  } catch {
    // 监控加载失败时继续提供页面服务。
    return undefined;
  }
}
