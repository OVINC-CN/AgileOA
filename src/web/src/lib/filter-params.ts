// 多选条件使用重复参数，数组中的空字符串保留其业务含义。
export function filterParams(values: Record<string, string | number | string[]>) {
  const parameters = new URLSearchParams();
  for (const [key, value] of Object.entries(values)) {
    if (Array.isArray(value)) {
      for (const item of value) parameters.append(key, item);
    } else if (value !== '') {
      parameters.set(key, String(value));
    }
  }
  return parameters;
}
