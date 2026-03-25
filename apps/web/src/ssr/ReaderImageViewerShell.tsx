import { ChevronLeft, ChevronRight, Minus, Plus, RotateCw, X } from "lucide-react";

function OriginalSizeIcon() {
  return (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" aria-hidden="true">
      <path d="M3 6V3h3" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M10 3h3v3" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M13 10v3h-3" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M6 13H3v-3" strokeLinecap="round" strokeLinejoin="round" />
      <circle cx="8" cy="8" r="1.2" fill="currentColor" stroke="none" />
    </svg>
  );
}

export function ReaderImageViewerShell() {
  return (
    <div
      className="reader-image-viewer"
      data-reader-hook="image-viewer"
      aria-hidden="true"
      hidden
    >
      <button
        type="button"
        className="reader-image-viewer__backdrop"
        data-reader-hook="image-viewer-backdrop"
        aria-label="关闭图片浏览器"
      />
      <div className="reader-image-viewer__chrome">
        <button
          type="button"
          className="reader-image-viewer__close"
          data-reader-hook="image-viewer-close"
          aria-label="关闭图片浏览器"
        >
          <X size={18} />
        </button>
        <div className="reader-image-viewer__stage" data-reader-hook="image-viewer-stage">
          <div className="reader-image-viewer__content" data-reader-hook="image-viewer-content" />
        </div>
        <div className="reader-image-viewer__toolbar" data-reader-hook="image-viewer-toolbar">
          <button
            type="button"
            className="reader-image-viewer__tool-button"
            data-reader-hook="image-viewer-prev"
            aria-label="查看上一张图片"
          >
            <ChevronLeft size={18} />
          </button>
          <span className="reader-image-viewer__tool-value" data-reader-hook="image-viewer-index">
            0/0
          </span>
          <button
            type="button"
            className="reader-image-viewer__tool-button"
            data-reader-hook="image-viewer-next"
            aria-label="查看下一张图片"
          >
            <ChevronRight size={18} />
          </button>
          <span className="reader-image-viewer__tool-divider" aria-hidden="true" />
          <button
            type="button"
            className="reader-image-viewer__tool-button"
            data-reader-hook="image-viewer-zoom-out"
            aria-label="缩小图片"
          >
            <Minus size={18} />
          </button>
          <span className="reader-image-viewer__tool-value" data-reader-hook="image-viewer-scale">
            100%
          </span>
          <button
            type="button"
            className="reader-image-viewer__tool-button"
            data-reader-hook="image-viewer-zoom-in"
            aria-label="放大图片"
          >
            <Plus size={18} />
          </button>
          <span className="reader-image-viewer__tool-divider" aria-hidden="true" />
          <button
            type="button"
            className="reader-image-viewer__tool-button"
            data-reader-hook="image-viewer-original"
            aria-label="恢复原始尺寸"
          >
            <OriginalSizeIcon />
          </button>
          <span className="reader-image-viewer__tool-divider" aria-hidden="true" />
          <button
            type="button"
            className="reader-image-viewer__tool-button"
            data-reader-hook="image-viewer-rotate"
            aria-label="顺时针旋转图片"
          >
            <RotateCw size={18} />
          </button>
        </div>
      </div>
    </div>
  );
}
