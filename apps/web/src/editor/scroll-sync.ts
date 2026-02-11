import type { DirectionAnchor } from "./types";

// 数值钳制，避免 scrollTop 越界。
export function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

// 获取元素可滚动高度（scrollHeight - clientHeight）。
export function getMaxScrollable(element: HTMLElement): number {
  return Math.max(0, element.scrollHeight - element.clientHeight);
}

// 构建单向映射锚点表：同一 sourceY 取最大 targetY，并保证 target 单调不减。
export function buildDirectionAnchors(
  points: DirectionAnchor[],
  sourceMaxScrollable: number,
  targetMaxScrollable: number
): DirectionAnchor[] {
  const sanitizedPoints = points
    .filter(
      (point) =>
        Number.isFinite(point.sourceY) &&
        Number.isFinite(point.targetY) &&
        point.sourceY >= 0 &&
        point.targetY >= 0
    )
    .map((point) => ({
      sourceY: clamp(point.sourceY, 0, sourceMaxScrollable),
      targetY: clamp(point.targetY, 0, targetMaxScrollable)
    }));

  // 头尾边界锚点保证全区间可映射。
  sanitizedPoints.push({ sourceY: 0, targetY: 0 });
  sanitizedPoints.push({ sourceY: sourceMaxScrollable, targetY: targetMaxScrollable });
  sanitizedPoints.sort((left, right) => left.sourceY - right.sourceY || left.targetY - right.targetY);

  const groupedAnchors: DirectionAnchor[] = [];
  let index = 0;
  while (index < sanitizedPoints.length) {
    const sourceY = sanitizedPoints[index].sourceY;
    let targetY = sanitizedPoints[index].targetY;
    index += 1;
    while (index < sanitizedPoints.length && sanitizedPoints[index].sourceY === sourceY) {
      targetY = Math.max(targetY, sanitizedPoints[index].targetY);
      index += 1;
    }
    groupedAnchors.push({ sourceY, targetY });
  }

  // target 轴做前缀最大化，确保单调，防止插值反向。
  for (let anchorIndex = 1; anchorIndex < groupedAnchors.length; anchorIndex += 1) {
    if (groupedAnchors[anchorIndex].targetY < groupedAnchors[anchorIndex - 1].targetY) {
      groupedAnchors[anchorIndex].targetY = groupedAnchors[anchorIndex - 1].targetY;
    }
  }

  // 固定顶部边界：source=0 必须严格映射到 target=0，避免首屏出现错位。
  const topBoundaryAnchor = groupedAnchors.find((anchor) => anchor.sourceY === 0);
  if (topBoundaryAnchor) {
    topBoundaryAnchor.targetY = 0;
  } else {
    groupedAnchors.unshift({ sourceY: 0, targetY: 0 });
  }

  // 固定底部边界：source=max 必须映射到 target=max，避免尾部无法对齐到底部。
  const bottomBoundaryAnchor = groupedAnchors.find(
    (anchor) => anchor.sourceY === sourceMaxScrollable
  );
  if (bottomBoundaryAnchor) {
    bottomBoundaryAnchor.targetY = targetMaxScrollable;
  } else {
    groupedAnchors.push({
      sourceY: sourceMaxScrollable,
      targetY: targetMaxScrollable
    });
  }

  // 重新按 source 排序并再做一次单调修正，保证插值阶段始终可用。
  groupedAnchors.sort((left, right) => left.sourceY - right.sourceY || left.targetY - right.targetY);
  for (let anchorIndex = 1; anchorIndex < groupedAnchors.length; anchorIndex += 1) {
    if (groupedAnchors[anchorIndex].targetY < groupedAnchors[anchorIndex - 1].targetY) {
      groupedAnchors[anchorIndex].targetY = groupedAnchors[anchorIndex - 1].targetY;
    }
  }

  return groupedAnchors;
}
