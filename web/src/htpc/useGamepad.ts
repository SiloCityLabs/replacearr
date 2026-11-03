import { useEffect, useRef } from 'react';

export type GamepadAction = 'up' | 'down' | 'left' | 'right' | 'confirm' | 'back';

const AXIS_DEADZONE = 0.55;
const REPEAT_MS = 180;

/** Pure mapping from a gamepad snapshot to a single navigation action. */
export function mapGamepadToAction(
  buttons: ReadonlyArray<{ pressed: boolean } | boolean>,
  axes: ReadonlyArray<number>
): GamepadAction | null {
  const pressed = (index: number): boolean => {
    const b = buttons[index];
    if (typeof b === 'boolean') return b;
    return Boolean(b?.pressed);
  };

  // Standard mapping: 12–15 D-pad, 0 = A/confirm, 1 = B/back
  if (pressed(12)) return 'up';
  if (pressed(13)) return 'down';
  if (pressed(14)) return 'left';
  if (pressed(15)) return 'right';
  if (pressed(0)) return 'confirm';
  if (pressed(1)) return 'back';

  const ax = axes[0] ?? 0;
  const ay = axes[1] ?? 0;
  if (ay < -AXIS_DEADZONE) return 'up';
  if (ay > AXIS_DEADZONE) return 'down';
  if (ax < -AXIS_DEADZONE) return 'left';
  if (ax > AXIS_DEADZONE) return 'right';

  return null;
}

export function useGamepad(onAction: (action: GamepadAction) => void, enabled = true): void {
  const onActionRef = useRef(onAction);
  onActionRef.current = onAction;
  const lastActionRef = useRef<{ action: GamepadAction; at: number } | null>(null);

  useEffect(() => {
    if (!enabled || typeof window === 'undefined' || !navigator.getGamepads) {
      return;
    }

    let raf = 0;

    const tick = () => {
      const pads = navigator.getGamepads();
      for (const pad of pads) {
        if (!pad) continue;
        const action = mapGamepadToAction(pad.buttons, pad.axes);
        if (action) {
          const now = performance.now();
          const last = lastActionRef.current;
          if (!last || last.action !== action || now - last.at >= REPEAT_MS) {
            lastActionRef.current = { action, at: now };
            onActionRef.current(action);
          }
        } else {
          lastActionRef.current = null;
        }
        break;
      }
      raf = requestAnimationFrame(tick);
    };

    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [enabled]);
}
