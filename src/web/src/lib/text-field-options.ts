export interface TextFieldOption {
  value: string;
  label: string;
}

// 预置使用英文标识，选择时沿用自由文本接口保存中文名称。
export function mergeTextFieldOptions(
  presets: readonly TextFieldOption[],
  options: readonly TextFieldOption[],
  currentValue: string,
) {
  // 预置优先展示，同值的后端选项保留后端显示名称。
  const merged = new Map<string, TextFieldOption>(
    presets.map(({ label }) => [label, { value: label, label }]),
  );
  for (const option of options) merged.set(option.value, option);

  // 编辑时保留尚未出现在选项接口中的历史值和自填值。
  if (currentValue && !merged.has(currentValue)) {
    merged.set(currentValue, { value: currentValue, label: currentValue });
  }
  return [...merged.values()];
}
