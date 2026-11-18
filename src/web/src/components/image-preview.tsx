import { RotateCcw, RotateCw, ZoomIn, ZoomOut } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';

import { Button } from '@/components/ui/button';
import { LoadingIcon } from '@/components/ui/loading-icon';

const defaultZoom = 100;
const minimumZoom = 25;
const maximumZoom = 400;
const zoomStep = 25;
const rotationStep = 90;

/* eslint-disable jsx-a11y/no-noninteractive-tabindex -- 图片滚动区域提供键盘焦点以查看放大后的内容。 */
export function ImagePreview({
  source,
  filename,
  onError,
}: {
  source: string;
  filename: string;
  onError: () => void;
}) {
  const viewport = useRef<HTMLDivElement>(null);
  const [viewportSize, setViewportSize] = useState({ width: 0, height: 0 });
  const [imageSize, setImageSize] = useState({ width: 0, height: 0 });
  const [zoom, setZoom] = useState(defaultZoom);
  const [rotation, setRotation] = useState(0);

  useEffect(() => {
    const element = viewport.current;
    if (!element) return;

    // 测量外框，避免滚动条出现后反复改变图片的适配比例。
    const measure = () => {
      const { width, height } = element.getBoundingClientRect();
      setViewportSize((current) =>
        current.width === width && current.height === height ? current : { width, height },
      );
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    return () => {
      observer.disconnect();
    };
  }, []);

  // 按旋转后的方向适配视口，放大倍数以适配后的尺寸为基准。
  const quarterTurn = rotation % 180 !== 0;
  const rotatedWidth = quarterTurn ? imageSize.height : imageSize.width;
  const rotatedHeight = quarterTurn ? imageSize.width : imageSize.height;
  const ready =
    rotatedWidth > 0 && rotatedHeight > 0 && viewportSize.width > 0 && viewportSize.height > 0;
  const fit = ready
    ? Math.min(1, viewportSize.width / rotatedWidth, viewportSize.height / rotatedHeight)
    : 0;
  const scale = (fit * zoom) / defaultZoom;

  return (
    <div className="flex min-h-0 w-full flex-col" aria-busy={!ready}>
      <div className="flex shrink-0 flex-wrap items-center gap-2 border-b border-border-subtle bg-popover px-4 py-2">
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={!ready || zoom <= minimumZoom}
          onClick={() => {
            setZoom((value) => Math.max(minimumZoom, value - zoomStep));
          }}
        >
          <ZoomOut aria-hidden="true" />
          缩小
        </Button>
        <span
          className="w-12 text-center text-xs text-muted-foreground tabular-nums"
          aria-live="polite"
        >
          {zoom}%
        </span>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={!ready || zoom >= maximumZoom}
          onClick={() => {
            setZoom((value) => Math.min(maximumZoom, value + zoomStep));
          }}
        >
          <ZoomIn aria-hidden="true" />
          放大
        </Button>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={!ready}
          aria-label="顺时针旋转 90 度"
          onClick={() => {
            setRotation((value) => (value + rotationStep) % 360);
          }}
        >
          <RotateCw aria-hidden="true" />
          旋转
        </Button>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={!ready || (zoom === defaultZoom && rotation === 0)}
          onClick={() => {
            setZoom(defaultZoom);
            setRotation(0);
          }}
        >
          <RotateCcw aria-hidden="true" />
          重置
        </Button>
      </div>
      <div className="relative min-h-0 flex-1 p-4">
        <div
          ref={viewport}
          role="region"
          aria-label="图片预览内容"
          tabIndex={0}
          className="h-full w-full overflow-auto overscroll-contain rounded-lg outline-none focus-visible:ring-2 focus-visible:ring-ring/30 focus-visible:ring-inset"
        >
          <div className="flex min-h-full w-max min-w-full items-center justify-center">
            <div
              className="relative shrink-0"
              style={{ width: rotatedWidth * scale, height: rotatedHeight * scale }}
            >
              <img
                src={source}
                alt={filename}
                draggable={false}
                className="absolute top-1/2 left-1/2 max-w-none select-none"
                style={{
                  width: imageSize.width * scale,
                  height: imageSize.height * scale,
                  transform: `translate(-50%, -50%) rotate(${String(rotation)}deg)`,
                }}
                onLoad={(event) => {
                  const { naturalWidth: width, naturalHeight: height } = event.currentTarget;
                  if (width <= 0 || height <= 0) {
                    onError();
                    return;
                  }
                  setImageSize({ width, height });
                }}
                onError={onError}
              />
            </div>
          </div>
        </div>
        {!ready ? (
          <div className="pointer-events-none absolute inset-0 flex items-center justify-center gap-3 text-sm text-muted-foreground">
            <LoadingIcon className="size-6" />
            正在加载预览
          </div>
        ) : null}
      </div>
    </div>
  );
}
/* eslint-enable jsx-a11y/no-noninteractive-tabindex */
