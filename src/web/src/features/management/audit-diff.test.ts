import { describe, expect, it } from 'vitest';

import { buildAuditDiff } from '@/features/management/audit-diff';
import { formatAuditJSON, parseAuditJSON } from '@/features/management/audit-json';

function diff(before: string, after: string, expanded = false) {
  return buildAuditDiff({ before, after, expanded, timeoutMs: 2000 });
}

describe('审计 JSON 差异', () => {
  it('保留长整数、零、布尔值和数组顺序，并忽略对象键顺序', () => {
    const before = '{"id":18446744073709551614,"nested":{"a":false,"b":0},"items":[2,1]}';
    const after = '{"items":[2,1],"nested":{"b":0,"a":false},"id":18446744073709551615}';
    const result = diff(before, after);
    expect(result.status).toBe('ready');
    expect(result.additions).toBe(1);
    expect(result.removals).toBe(1);
    expect(result.hunks.flatMap((hunk) => hunk.lines)).toContain('-  "id": 18446744073709551614,');
    expect(result.hunks.flatMap((hunk) => hunk.lines)).toContain('+  "id": 18446744073709551615,');
    expect(formatAuditJSON(parseAuditJSON(after))).toContain('18446744073709551615');
    expect(diff('{"a":1,"b":2}', '{"b":2,"a":1}').hunks).toEqual([]);
    expect(diff('[2,1]', '[1,2]').additions).toBeGreaterThan(0);
  });

  it('新增和删除保留完整内容，嵌套更新显示删除及新增行', () => {
    expect(diff('null', '{"enabled":false}')).toMatchObject({ additions: 3, removals: 0 });
    expect(diff('{"enabled":false}', 'null')).toMatchObject({ additions: 0, removals: 3 });
    const result = diff('{"values":{"count":0}}', '{"values":{"count":1}}');
    expect(result.hunks[0]?.lines).toContain('-    "count": 0');
    expect(result.hunks[0]?.lines).toContain('+    "count": 1');
  });

  it('默认保留三行上下文，展开后可见未变化内容', () => {
    const values = Array.from({ length: 30 }, (_, index) => index);
    const changed = [...values];
    changed[15] = 99;
    const before = JSON.stringify(values);
    const after = JSON.stringify(changed);
    const compact = diff(before, after);
    const expanded = diff(before, after, true);
    expect(compact.hunks[0]?.oldStart).toBeGreaterThan(1);
    expect(expanded.hunks[0]?.oldStart).toBe(1);
    expect(expanded.hunks[0]?.lines.length).toBeGreaterThan(compact.hunks[0]?.lines.length ?? 0);
  });
});
