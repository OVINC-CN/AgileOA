import { getConfig } from '@/lib/config';
import type { OfficePreviewKind } from '@/lib/file-preview';

export interface OfficePreviewPosition {
  index: number;
  total: number;
  names?: string[];
}

export interface OfficePreviewViewer {
  load: (source: string) => Promise<void>;
  goTo: (index: number) => Promise<void>;
  zoomIn: () => void | Promise<void>;
  zoomOut: () => void | Promise<void>;
  fitWidth: () => void | Promise<void>;
  destroy: () => void;
}

export async function createOfficePreviewViewer(
  kind: OfficePreviewKind,
  canvas: HTMLCanvasElement,
  onPosition: (position: OfficePreviewPosition) => void,
  onError: (error: unknown) => void,
): Promise<OfficePreviewViewer> {
  // 只按当前格式加载解析器；字体沿用本机，禁用文档内链接和媒体的自动交互。
  const options = {
    cjkFallback: 'sc' as const,
    enableHyperlinks: false,
    workerTimeoutMs: getConfig().filePreviewTimeoutMs,
    onError,
  };
  switch (kind) {
    case 'docx': {
      const [{ DocxViewer }, { legacyDoc }] = await Promise.all([
        import('@silurus/ooxml/docx'),
        import('@silurus/ooxml-legacy-office/doc'),
      ]);
      const viewer = new DocxViewer(canvas, {
        ...options,
        legacyDoc,
        onPageChange: (index, total) => {
          onPosition({ index, total });
        },
      });
      return {
        load: (source) => viewer.load(source),
        goTo: (index) => viewer.goToPage(index),
        zoomIn: () => viewer.zoomIn(),
        zoomOut: () => viewer.zoomOut(),
        fitWidth: () => viewer.fitWidth(),
        destroy: () => {
          viewer.destroy();
        },
      };
    }
    case 'xlsx': {
      const [{ XlsxSheetViewer }, { legacyXls }] = await Promise.all([
        import('@silurus/ooxml/xlsx'),
        import('@silurus/ooxml-legacy-office/xls'),
      ]);
      const viewer = new XlsxSheetViewer(canvas, {
        ...options,
        legacyXls,
        onReady: (names) => {
          onPosition({ index: viewer.sheetIndex, total: names.length, names });
        },
        onSheetChange: (index, total) => {
          onPosition({ index, total, names: viewer.sheetNames });
        },
      });
      return {
        load: (source) => viewer.load(source),
        goTo: (index) => viewer.goToSheet(index),
        zoomIn: () => {
          viewer.zoomIn();
        },
        zoomOut: () => {
          viewer.zoomOut();
        },
        fitWidth: () => {
          viewer.fitWidth();
        },
        destroy: () => {
          viewer.destroy();
        },
      };
    }
    case 'pptx': {
      const [{ PptxViewer }, { legacyPpt }] = await Promise.all([
        import('@silurus/ooxml/pptx'),
        import('@silurus/ooxml-legacy-office/ppt'),
      ]);
      const viewer = new PptxViewer(canvas, {
        ...options,
        legacyPpt,
        enableMediaPlayback: false,
        onSlideChange: (index, total) => {
          onPosition({ index, total });
        },
      });
      return {
        load: (source) => viewer.load(source),
        goTo: (index) => viewer.goToSlide(index),
        zoomIn: () => viewer.zoomIn(),
        zoomOut: () => viewer.zoomOut(),
        fitWidth: () => viewer.fitWidth(),
        destroy: () => {
          viewer.destroy();
        },
      };
    }
  }
}

export function officePreviewErrorMessage(error: unknown): string {
  // 只将稳定的错误代码映射为中文，不向用户展示解析器的原始诊断内容。
  const code = error && typeof error === 'object' && 'code' in error ? error.code : undefined;
  switch (code) {
    case 'encrypted':
    case 'invalid-password':
    case 'unsupported-encryption':
      return '文件已加密，请下载原文件后打开';
    case 'unsupported-content':
      return '文件包含暂不支持的内容，请下载原文件查看';
    case 'ooxml-resource-limit':
    case 'ooxml-decoded-image-limit':
      return '文件内容过大，无法预览，请下载原文件查看';
    default:
      return '文件预览失败，请重试或下载原文件查看';
  }
}
