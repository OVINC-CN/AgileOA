import { useLayoutEffect, useRef } from 'react';
import type { BaseTickContentProps } from 'recharts';

import { formatNumber } from '@/lib/format';

const countFormatter = new Intl.NumberFormat('zh-CN', {
  notation: 'compact',
  maximumFractionDigits: 1,
});

export function ChartCountTick({
  x,
  y,
  width = 0,
  payload,
  className,
}: Partial<BaseTickContentProps>) {
  const text = useRef<SVGTextElement>(null);
  const value: unknown = payload?.value;
  const label = typeof value === 'number' ? countFormatter.format(value) : '';

  // 扣除坐标轴默认的刻度长度和间距，保持现有轴宽及数字右侧位置。
  const availableWidth = Math.max(0, Number(width) - 8);
  useLayoutEffect(() => {
    const element = text.current;
    if (!element || availableWidth <= 0) return;

    // 先恢复自然宽度，仅将仍然超长的紧凑数字压缩到可用区域。
    element.removeAttribute('textLength');
    if (element.getComputedTextLength() > availableWidth) {
      element.setAttribute('textLength', String(availableWidth));
    }
  }, [availableWidth, label]);

  return (
    <text
      ref={text}
      x={x}
      y={y}
      dy="0.355em"
      textAnchor="end"
      lengthAdjust="spacingAndGlyphs"
      fontSize="var(--font-size-caption)"
      fill="var(--muted-foreground)"
      className={className}
      aria-label={typeof value === 'number' ? formatNumber(value) : undefined}
    >
      {label}
    </text>
  );
}
