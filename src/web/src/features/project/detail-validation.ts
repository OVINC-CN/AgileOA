export function validateUniqueDetailNames(items: { name: string }[]) {
  const seen = new Set<string>();
  for (const item of items) {
    const name = item.name.trim();
    if (!name) continue;
    if (seen.has(name)) throw new Error('同一条记录中的子项名称不能重复');
    seen.add(name);
  }
}
