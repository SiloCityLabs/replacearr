import { useCallback, useState } from 'react';
import type { KeyboardEvent as ReactKeyboardEvent } from 'react';

export type RovingOrientation = 'vertical' | 'horizontal' | 'grid';

interface UseRovingFocusOptions {
  itemCount: number;
  orientation?: RovingOrientation;
  columns?: number;
  loop?: boolean;
  onActivate?: (index: number) => void;
}

export function useRovingFocus({
  itemCount,
  orientation = 'vertical',
  columns = 1,
  loop = true,
  onActivate,
}: UseRovingFocusOptions) {
  const [activeIndex, setActiveIndexState] = useState(0);

  const setActiveIndex = useCallback(
    (index: number) => {
      if (itemCount <= 0) {
        setActiveIndexState(0);
        return;
      }
      setActiveIndexState(Math.max(0, Math.min(itemCount - 1, index)));
    },
    [itemCount]
  );

  const move = useCallback(
    (delta: number) => {
      if (itemCount <= 0) return 0;
      setActiveIndexState((current) => {
        let next = current + delta;
        if (loop) {
          next = ((next % itemCount) + itemCount) % itemCount;
        } else {
          next = Math.max(0, Math.min(itemCount - 1, next));
        }
        return next;
      });
      // Approximate next for immediate focus targeting
      let next = activeIndex + delta;
      if (loop) {
        next = ((next % itemCount) + itemCount) % itemCount;
      } else {
        next = Math.max(0, Math.min(itemCount - 1, next));
      }
      return next;
    },
    [activeIndex, itemCount, loop]
  );

  const getTabIndex = useCallback(
    (index: number) => (index === activeIndex ? 0 : -1),
    [activeIndex]
  );

  const onKeyDown = useCallback(
    (event: ReactKeyboardEvent, index: number) => {
      if (itemCount <= 0) return;
      setActiveIndexState(index);

      const cols = Math.max(1, columns);
      let handled = false;
      let next = index;

      const compute = (delta: number) => {
        let n = index + delta;
        if (loop) {
          n = ((n % itemCount) + itemCount) % itemCount;
        } else {
          n = Math.max(0, Math.min(itemCount - 1, n));
        }
        return n;
      };

      switch (event.key) {
        case 'ArrowDown':
          if (orientation === 'horizontal') break;
          next = compute(orientation === 'grid' ? cols : 1);
          handled = true;
          break;
        case 'ArrowUp':
          if (orientation === 'horizontal') break;
          next = compute(orientation === 'grid' ? -cols : -1);
          handled = true;
          break;
        case 'ArrowRight':
          if (orientation === 'vertical') break;
          next = compute(1);
          handled = true;
          break;
        case 'ArrowLeft':
          if (orientation === 'vertical') break;
          next = compute(-1);
          handled = true;
          break;
        case 'Home':
          next = 0;
          handled = true;
          break;
        case 'End':
          next = itemCount - 1;
          handled = true;
          break;
        case 'Enter':
        case ' ':
          onActivate?.(index);
          handled = true;
          break;
        default:
          break;
      }

      if (handled) {
        event.preventDefault();
        event.stopPropagation();
        if (event.key !== 'Enter' && event.key !== ' ') {
          setActiveIndexState(next);
          const parent = event.currentTarget.parentElement;
          const el = parent?.querySelectorAll<HTMLElement>('[data-roving-item="true"]')?.[next];
          el?.focus();
        }
      }
    },
    [columns, itemCount, loop, onActivate, orientation]
  );

  const itemProps = useCallback(
    (index: number) => ({
      'data-roving-item': 'true' as const,
      tabIndex: getTabIndex(index),
      onFocus: () => setActiveIndex(index),
      onKeyDown: (event: ReactKeyboardEvent) => onKeyDown(event, index),
    }),
    [getTabIndex, onKeyDown, setActiveIndex]
  );

  return { activeIndex, setActiveIndex, getTabIndex, itemProps, move };
}
