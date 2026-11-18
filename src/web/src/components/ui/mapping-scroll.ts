import { type RefObject, useCallback, useLayoutEffect, useMemo, useRef } from 'react';

type MappingSide = 'source' | 'target';

export interface MappingScrollLink {
  sourceID: string;
  targetID: string;
}

type MappingAnchors = Record<MappingSide, ReadonlyMap<string, HTMLElement>>;

const scrollPositionTolerance = 1;

// 按可见节点映射联动两侧滚动，避免程序滚动事件相互回传。
export function useMappingScroll({
  enabled,
  links,
  anchors,
}: {
  enabled: boolean;
  links: MappingScrollLink[];
  anchors: RefObject<MappingAnchors>;
}) {
  const source = useRef<HTMLDivElement>(null);
  const target = useRef<HTMLDivElement>(null);
  const expected = useRef<Record<MappingSide, number | null>>({ source: null, target: null });
  const previous = useRef<MappingScrollLink | null>(null);
  const mappings = useMemo(() => {
    const source = new Map<string, string[]>();
    const target = new Map<string, string[]>();
    for (const link of links) {
      const destinations = source.get(link.sourceID) ?? [];
      const origins = target.get(link.targetID) ?? [];
      destinations.push(link.targetID);
      origins.push(link.sourceID);
      source.set(link.sourceID, destinations);
      target.set(link.targetID, origins);
    }
    return { source, target };
  }, [links]);

  useLayoutEffect(() => {
    expected.current = { source: null, target: null };
    previous.current = null;
  }, [enabled, links]);

  const ignore = useCallback((side: MappingSide, panel: HTMLElement | null) => {
    if (panel) expected.current[side] = panel.scrollTop;
  }, []);

  const synchronize = useCallback(
    (side: MappingSide, panel: HTMLDivElement) => {
      // 忽略联动和定位产生的回传事件，实际滚动位置变化后允许另一侧接管。
      const position = expected.current[side];
      if (position !== null && Math.abs(panel.scrollTop - position) < scrollPositionTolerance)
        return;
      expected.current[side] = null;
      if (!enabled) return;
      const oppositeSide = side === 'source' ? 'target' : 'source';
      const opposite = oppositeSide === 'source' ? source.current : target.current;
      if (!opposite || panel.clientHeight <= 0 || opposite.clientHeight <= 0) return;
      const viewport = panel.getBoundingClientRect();
      const oppositeViewport = opposite.getBoundingClientRect();
      const center = viewport.top + panel.clientHeight / 2;
      const oppositeCenter = oppositeViewport.top + opposite.clientHeight / 2;
      const nodeBounds = (nodeSide: MappingSide, id: string) =>
        anchors.current[nodeSide]
          .get(id)
          ?.closest<HTMLElement>('[data-mapping-node]')
          ?.getBoundingClientRect();

      // 只跟随当前可见且已映射的节点，优先选择靠近视野中心的节点。
      const previousID =
        side === 'source' ? previous.current?.sourceID : previous.current?.targetID;
      let current:
        { id: string; bounds: DOMRect; destinations: string[]; distance: number } | undefined;
      for (const [id, destinations] of mappings[side]) {
        const bounds = nodeBounds(side, id);
        if (
          !bounds ||
          bounds.height <= 0 ||
          bounds.bottom <= viewport.top ||
          bounds.top >= viewport.bottom
        )
          continue;
        const distance = Math.max(bounds.top - center, center - bounds.bottom, 0);
        if (
          !current ||
          distance < current.distance ||
          (distance === current.distance && id === previousID)
        ) {
          current = { id, bounds, destinations, distance };
        }
      }
      if (!current) return;

      // 多个对应节点优先沿用当前配对，首次联动选择另一侧最靠近视野中心的节点。
      const previousDestination =
        side === 'source' ? previous.current?.targetID : previous.current?.sourceID;
      const preferred = current.id === previousID ? previousDestination : undefined;
      let destination: { id: string; bounds: DOMRect; distance: number } | undefined;
      for (const id of current.destinations) {
        const bounds = nodeBounds(oppositeSide, id);
        if (!bounds || bounds.height <= 0) continue;
        const distance =
          id === preferred ? -1 : Math.abs(bounds.top + bounds.height / 2 - oppositeCenter);
        if (!destination || distance < destination.distance) {
          destination = { id, bounds, distance };
        }
      }
      if (!destination) return;
      previous.current =
        side === 'source'
          ? { sourceID: current.id, targetID: destination.id }
          : { sourceID: destination.id, targetID: current.id };

      // 按节点内部的相对进度对齐，适应两侧字段数量和卡片高度不同的情况。
      const progress = Math.max(
        0,
        Math.min(1, (center - current.bounds.top) / current.bounds.height),
      );
      const top = Math.max(
        0,
        Math.min(
          opposite.scrollHeight - opposite.clientHeight,
          opposite.scrollTop +
            destination.bounds.top -
            oppositeViewport.top +
            progress * destination.bounds.height -
            opposite.clientHeight / 2,
        ),
      );
      if (Math.abs(opposite.scrollTop - top) < scrollPositionTolerance) return;
      expected.current[oppositeSide] = top;
      opposite.scrollTo({ top, behavior: 'instant' });
      expected.current[oppositeSide] = opposite.scrollTop;
    },
    [anchors, enabled, mappings],
  );

  return { source, target, synchronize, ignore };
}
