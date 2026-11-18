const initialPasswordLength = 16;
const passwordAlphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_';

// generateInitialPassword 使用浏览器安全随机源生成 16 位初始密码。
export function generateInitialPassword(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(initialPasswordLength));
  // 字符集包含 64 个字符，每个字符等概率映射，不使用可预测的随机数。
  return Array.from(bytes, (value) => passwordAlphabet.charAt(value & 63)).join('');
}
