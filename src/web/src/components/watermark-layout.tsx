import { useQuery } from '@tanstack/react-query';
import { useLayoutEffect } from 'react';
import { Outlet } from 'react-router';

import { currentUserOptions } from '@/features/user/api';
import { clearWatermarkPreference, useWatermarkHidden } from '@/features/user/watermark';
import { workspaceContextOptions } from '@/features/workspace/api';
import { useActiveWorkspaceID } from '@/features/workspace/navigation';
import { formatIdentity } from '@/lib/format';

const tileWidth = 224;
const tileHeight = 128;
const tileScale = 2;
const lineHeight = 17;

function createTile(lines: string[], font: string, color: string) {
  const canvas = document.createElement('canvas');
  canvas.width = tileWidth * tileScale;
  canvas.height = tileHeight * tileScale;
  const context = canvas.getContext('2d');
  if (!context) return;

  // 过长的文字收窄在水印单元内，避免相邻水印重叠。
  context.scale(tileScale, tileScale);
  context.translate(tileWidth / 2, tileHeight / 2);
  context.rotate((-22 * Math.PI) / 180);
  context.fillStyle = color;
  context.font = font;
  context.textAlign = 'center';
  context.textBaseline = 'middle';
  lines.forEach((line, index) => {
    context.fillText(line, 0, (index - (lines.length - 1) / 2) * lineHeight, tileWidth - 32);
  });

  return canvas.toDataURL();
}

export function WatermarkLayout() {
  const workspaceID = useActiveWorkspaceID();
  // 只订阅现有查询结果，不为水印发起请求，也不复用其他空间的数据。
  const { data: user } = useQuery({ ...currentUserOptions, enabled: false });
  const workspace = useQuery({
    ...workspaceContextOptions(workspaceID),
    enabled: false,
  });
  const workspaceName = workspace.isSuccess ? workspace.data.workspace.name : undefined;
  const username = user?.username;
  const displayName = user?.display_name;
  const hidden = useWatermarkHidden(user);

  useLayoutEffect(() => {
    // 初次读取账号前保留会话存储，退出或撤销管理员身份后清除旧偏好。
    if (user === null || user?.is_admin === false) clearWatermarkPreference();
  }, [user]);

  useLayoutEffect(() => {
    if (!username || !displayName || hidden) return;
    const lines = [formatIdentity(displayName, username), '保密信息 请勿外传'];
    if (workspaceName) lines.unshift(workspaceName);
    const body = document.body;
    const bodyStyle = getComputedStyle(body);
    const font = `500 12px ${bodyStyle.fontFamily}`;
    const tile = createTile(lines, font, bodyStyle.getPropertyValue('--watermark-color').trim());
    if (!tile) return;

    // 使用浏览器顶层覆盖页面内容，旧浏览器沿用固定定位和层级。
    const layer = document.createElement('div');
    const layerClass = 'watermark-layer';
    layer.setAttribute('aria-hidden', 'true');
    layer.style.cssText = `
      all: initial !important;
      display: block !important;
      position: fixed !important;
      inset: 0 !important;
      z-index: 2147483647 !important;
      pointer-events: none !important;
      user-select: none !important;
      background-image: url("${tile}") !important;
      background-position: center !important;
      background-repeat: repeat !important;
      background-size: ${String(tileWidth)}px ${String(tileHeight)}px !important;
    `;
    let style = layer.style.cssText;

    // 恢复被删除、移动或改动样式的水印，不监听业务页面的子树。
    const restore = () => {
      if (layer.className !== layerClass) layer.className = layerClass;
      if (layer.style.cssText !== style) layer.style.cssText = style;
      if (layer.parentNode !== body) body.append(layer);
      if (typeof layer.showPopover === 'function') {
        if (layer.popover !== 'manual') layer.popover = 'manual';
        if (!layer.matches(':popover-open')) layer.showPopover();
      }
    };
    restore();

    // 系统外观变化时重绘画布，保持深浅背景上的水印可见。
    const colorScheme = window.matchMedia('(prefers-color-scheme: dark)');
    const updateTile = () => {
      const color = getComputedStyle(body).getPropertyValue('--watermark-color').trim();
      const updatedTile = createTile(lines, font, color);
      if (!updatedTile) return;
      layer.style.setProperty('background-image', `url("${updatedTile}")`, 'important');
      style = layer.style.cssText;
      restore();
    };
    colorScheme.addEventListener('change', updateTile);
    layer.addEventListener('toggle', restore);
    const observer = new MutationObserver(restore);
    observer.observe(body, { childList: true });
    observer.observe(layer, { attributes: true, attributeFilter: ['class', 'popover', 'style'] });

    return () => {
      observer.disconnect();
      colorScheme.removeEventListener('change', updateTile);
      layer.removeEventListener('toggle', restore);
      layer.remove();
    };
  }, [workspaceName, username, displayName, hidden]);

  return <Outlet />;
}
