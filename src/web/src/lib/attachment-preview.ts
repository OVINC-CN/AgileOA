import type { FileViewerOptions } from '@file-viewer/core';

import { getConfig } from '@/lib/config';
import { attachmentPreviewExtension, type AttachmentPreviewKind } from '@/lib/file-preview';

export interface AttachmentPreviewPosition {
  index: number;
  total: number;
  names?: string[];
  zoom?: number;
  canFitPage?: boolean;
}

export interface AttachmentPreviewViewer {
  load: (file: File, contentType: string) => Promise<void>;
  goTo: (index: number) => Promise<void>;
  zoomIn: () => Promise<void>;
  zoomOut: () => Promise<void>;
  fitWidth: () => Promise<void>;
  fitPage: () => Promise<void>;
  rotate: () => Promise<void>;
  destroy: () => void;
}

export const emptyPosition: AttachmentPreviewPosition = { index: 0, total: 0 };
export const minimumImageZoom = 0.25;
export const maximumImageZoom = 4;
const pageSelectors = {
  docx: '.docx-page-frame, .docx-canvas-sheet, .msdoc-page, .docx-flow-frame',
  pptx: '.flyfish-pptx-slide-slot, .ppt-binary-page',
};
const renderStateSelectors = {
  pdf: {
    ready: '.pdf-shell:has(.pdf-state[hidden]) .pdfViewer .page',
    error: '.pdf-state--error',
  },
  xlsx: {
    ready: '.excel-wrapper:has(> .loading.hidden) .sheet-tab',
    error: '.excel-wrapper > .error:not(.hidden)',
  },
};
const hiddenViewerControls = `
  .excel-wrapper > .toolbar,
  .image-toolbar,
  .image-lightbox,
  .pptx-slideshow-button { display: none !important; }
  .image-stage img { cursor: default; }
`;
const imageZoomStep = 0.25;
const imageFitPadding = 24;
const documentFitPadding = 12;
const slideFitPadding = 24;

const rendererImports = {
  docx: () => import('@file-viewer/renderer-word'),
  xlsx: () => import('@file-viewer/renderer-spreadsheet'),
  pptx: () => import('@file-viewer/renderer-presentation'),
  pdf: () => import('@file-viewer/renderer-pdf'),
  image: () => import('@file-viewer/renderer-image'),
};

function waitForPreviewContent(
  surface: ShadowRoot,
  selectors: (typeof renderStateSelectors)[keyof typeof renderStateSelectors],
  signal: AbortSignal,
) {
  // PDF 和 Excel 会先挂载预览，再异步生成页面或工作表。
  return new Promise<void>((resolve, reject) => {
    const observer = new MutationObserver(check);
    function finish(error?: unknown) {
      observer.disconnect();
      signal.removeEventListener('abort', abort);
      if (error) reject(error instanceof Error ? error : new Error('文件预览已取消'));
      else resolve();
    }
    function abort() {
      finish(signal.reason);
    }
    function check() {
      if (surface.querySelector(selectors.error)) finish(new Error('无法读取文件内容'));
      else if (surface.querySelector(selectors.ready)) finish();
    }
    if (signal.aborted) {
      abort();
      return;
    }
    signal.addEventListener('abort', abort, { once: true });
    observer.observe(surface, {
      childList: true,
      subtree: true,
      attributes: true,
      attributeFilter: ['class', 'hidden'],
    });
    check();
  });
}

export async function createAttachmentPreviewViewer(
  kind: AttachmentPreviewKind,
  host: HTMLDivElement,
  onPosition: (position: AttachmentPreviewPosition) => void,
): Promise<AttachmentPreviewViewer> {
  const [{ createViewer }, { default: renderer }] = await Promise.all([
    import('@file-viewer/core'),
    rendererImports[kind](),
  ]);
  const timeout = getConfig().filePreviewTimeoutMs;
  const controller = new AbortController();
  // 工作表自动适配时只在列宽超出视口时缩小，窄表保持原始比例。
  const fitMode = kind === 'image' ? 'scale-down' : kind === 'xlsx' ? 'auto' : 'contain';
  const options: FileViewerOptions = {
    // Flyfish 为这些渲染器创建 div，注册接口的类型以 HTMLElement 表示。
    renderers: renderer as NonNullable<FileViewerOptions['renderers']>,
    rendererMode: 'replace',
    autoRenderers: false,
    styleIsolation: 'shadow',
    toolbar: false,
    zoomGestures: false,
    locale: 'zh-CN',
    fit: {
      mode: fitMode,
      resize: 'until-interaction',
      padding: kind === 'image' ? imageFitPadding : documentFitPadding,
    },
    pdf: { toolbar: false, navigation: false, streaming: false },
    docx: {
      visualPagination: true,
      awaitLayout: true,
      workerTimeout: timeout,
      externalLinkPolicy: 'block',
      externalResourcePolicy: 'block',
      renderAltChunks: false,
    },
    presentation: { pptCache: false },
  };
  let surface: ShadowRoot | null = null;
  let frame = 0;
  let imageZoom = 1;
  let fitScale = 1;
  let rotation = 0;
  let slideFit: 'contain' | 'width' | null = null;
  let legacyWordFlow = false;
  let dimensions: ResizeObserver | undefined;
  const instance = createViewer(host, { options, signal: controller.signal, onEvent: schedule });
  const sheets = () => Array.from(surface?.querySelectorAll<HTMLButtonElement>('.sheet-tab') ?? []);
  const pages = () =>
    kind === 'docx' || kind === 'pptx'
      ? Array.from(surface?.querySelectorAll<HTMLElement>(pageSelectors[kind]) ?? [])
      : [];

  // 分页文档和工作表使用渲染器生成的页和标签，交互入口由公共控件提供。
  function schedule() {
    if (frame || controller.signal.aborted) return;
    frame = requestAnimationFrame(() => {
      frame = 0;
      const state = instance.getViewState();
      if (kind === 'pdf' || kind === 'image') {
        onPosition({
          index: (state?.page ?? 1) - 1,
          total: state?.pageCount ?? 0,
          zoom: imageZoom,
        });
        return;
      }
      if (kind === 'xlsx') {
        const tabs = sheets();
        onPosition({
          index: Math.max(
            0,
            tabs.findIndex((tab) => tab.getAttribute('aria-pressed') === 'true'),
          ),
          total: tabs.length,
          names: tabs.map((tab) => tab.textContent.trim()),
        });
        return;
      }
      const renderedPages = pages();
      const top = host.getBoundingClientRect().top + 48;
      const index = renderedPages.findIndex((page) => page.getBoundingClientRect().bottom > top);
      onPosition({
        index: Math.max(0, index),
        total: renderedPages.length,
        canFitPage: !legacyWordFlow,
      });
    });
  }

  async function fitImage() {
    const result = await instance.fitToView({
      mode: 'scale-down',
      resize: 'initial',
      padding: imageFitPadding,
    });
    if (result.scale !== undefined) fitScale = result.scale;
    await applyImageZoom();
  }

  async function applyImageZoom() {
    const scale = fitScale * imageZoom;
    const state = await instance.applyViewState({ scale });
    const imageFrame = surface?.querySelector<HTMLElement>('.image-frame');
    // 图片很大时，渲染器的绝对缩放下限可能高于预览适配尺寸的四分之一。
    if (imageFrame && state?.scale) {
      imageFrame.style.zoom = scale < state.scale ? String(scale / state.scale) : '';
    }
    schedule();
  }

  async function zoomImage(step: number) {
    imageZoom = Math.min(maximumImageZoom, Math.max(minimumImageZoom, imageZoom + step));
    await applyImageZoom();
  }

  async function fitDocument(mode: 'contain' | 'width') {
    const slide = surface?.querySelector<HTMLElement>('.flyfish-pptx-slide-slot .slide');
    if (slide?.offsetWidth && slide.offsetHeight) {
      // PPTX 的通用适配以幻灯片列表为内容，整页缩放以单张幻灯片尺寸计算。
      slideFit = mode;
      const widthScale = (host.clientWidth - slideFitPadding * 2) / slide.offsetWidth;
      const heightScale = (host.clientHeight - slideFitPadding * 2) / slide.offsetHeight;
      const scale = mode === 'contain' ? Math.min(widthScale, heightScale) : widthScale;
      if (scale > 0) await instance.applyViewState({ scale });
      return;
    }
    await instance.fitToView({
      mode,
      padding: documentFitPadding,
      ...(legacyWordFlow ? { maxScale: 1 } : {}),
    });
  }

  function blockEmbeddedInteraction(event: Event) {
    const embedded = event.composedPath().some((target) => {
      return (
        target instanceof Element &&
        target.matches('a, audio, video, [data-pptx-media-id], .image-stage img')
      );
    });
    const slideshow =
      kind === 'pptx' &&
      event instanceof KeyboardEvent &&
      (event.key === 'F5' || event.key.toLowerCase() === 'p');
    if (embedded || slideshow) {
      event.preventDefault();
      event.stopPropagation();
    }
  }

  for (const type of ['click', 'auxclick', 'keydown']) {
    host.addEventListener(type, blockEmbeddedInteraction, {
      capture: true,
      signal: controller.signal,
    });
  }

  async function loadContent(file: File, contentType: string, signal: AbortSignal) {
    const buffer = await file.arrayBuffer();
    signal.throwIfAborted();
    const session = await instance.load(
      { buffer, filename: file.name, type: attachmentPreviewExtension(contentType) },
      { signal },
    );
    signal.throwIfAborted();
    const renderHost = host.firstElementChild;
    surface = renderHost?.shadowRoot ?? null;
    if (!session || !surface || !(renderHost instanceof HTMLElement)) {
      throw new Error('无法打开文件预览');
    }
    const style = host.ownerDocument.createElement('style');
    style.textContent = hiddenViewerControls;
    surface.append(style);

    if (kind === 'pptx') {
      // 演示文稿的渲染根默认随内容增高，整页缩放使用预览区的高度。
      renderHost.style.height = '100%';
      const target = surface.querySelector<HTMLElement>('.file-render');
      if (target) target.style.height = '100%';
    }
    if (kind === 'pdf' || kind === 'xlsx') {
      await waitForPreviewContent(surface, renderStateSelectors[kind], signal);
    }
    signal.throwIfAborted();
    legacyWordFlow = Boolean(surface.querySelector('.msdoc-page:not([data-msdoc-page-width])'));
    if (kind === 'xlsx') {
      await instance.fitToView();
    } else if (kind !== 'image') {
      await fitDocument(legacyWordFlow ? 'width' : 'contain');
    }
    surface.addEventListener('scroll', schedule, { capture: true, signal: controller.signal });
    host.addEventListener('scroll', schedule, { signal: controller.signal });
    if (kind === 'image') {
      const image = surface.querySelector<HTMLImageElement>('.image-stage img');
      if (image) {
        image.alt = file.name;
        image.removeAttribute('role');
        image.removeAttribute('aria-haspopup');
        image.removeAttribute('tabindex');
      }
      await fitImage();
    }
    if (kind === 'image' || slideFit) {
      dimensions = new ResizeObserver(() => {
        if (kind === 'image') void fitImage().catch(() => undefined);
        else if (slideFit) void fitDocument(slideFit).catch(() => undefined);
      });
      dimensions.observe(host);
    }
    schedule();
  }

  return {
    async load(file, contentType) {
      // 从已鉴权的 File 读取，预览器不获取签名地址。
      const loadController = new AbortController();
      const signal = AbortSignal.any([controller.signal, loadController.signal]);
      let timer: ReturnType<typeof setTimeout> | undefined;
      const deadline = new Promise<never>((_resolve, reject) => {
        timer = setTimeout(() => {
          const error = new DOMException('预览超时', 'TimeoutError');
          loadController.abort(error);
          reject(error);
        }, timeout);
      });
      try {
        await Promise.race([loadContent(file, contentType, signal), deadline]);
      } finally {
        clearTimeout(timer);
      }
    },
    async goTo(index) {
      if (kind === 'xlsx') {
        sheets()[index]?.click();
      } else if (kind === 'pdf') {
        await instance.applyViewState({ page: index + 1 });
      } else {
        pages()[index]?.scrollIntoView({ block: 'start', inline: 'nearest' });
      }
      schedule();
    },
    async zoomIn() {
      slideFit = null;
      if (kind === 'image') await zoomImage(imageZoomStep);
      else await instance.zoomIn();
    },
    async zoomOut() {
      slideFit = null;
      if (kind === 'image') await zoomImage(-imageZoomStep);
      else await instance.zoomOut();
    },
    async fitWidth() {
      await fitDocument('width');
    },
    async fitPage() {
      if (kind === 'image') {
        imageZoom = 1;
        rotation = 0;
        await instance.applyViewState({ rotation });
        await fitImage();
        return;
      }
      await fitDocument('contain');
    },
    async rotate() {
      rotation = (rotation + 90) % 360;
      await instance.applyViewState({ rotation });
      if (kind === 'image') await fitImage();
      schedule();
    },
    destroy() {
      controller.abort();
      cancelAnimationFrame(frame);
      dimensions?.disconnect();
      void instance.destroy();
    },
  };
}

export function attachmentPreviewErrorMessage(error: unknown): string {
  if (error instanceof DOMException && error.name === 'TimeoutError') {
    return '文件预览超时，请重试或下载原文件查看';
  }
  return '文件预览失败，请重试或下载原文件查看';
}
