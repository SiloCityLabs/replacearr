import { describe, expect, it } from 'vitest';
import { mapGamepadToAction } from './useGamepad';

function buttons(pressedIndexes: number[], length = 16): Array<{ pressed: boolean }> {
  return Array.from({ length }, (_, i) => ({ pressed: pressedIndexes.includes(i) }));
}

describe('mapGamepadToAction', () => {
  it('maps D-pad buttons to directions', () => {
    expect(mapGamepadToAction(buttons([12]), [0, 0])).toBe('up');
    expect(mapGamepadToAction(buttons([13]), [0, 0])).toBe('down');
    expect(mapGamepadToAction(buttons([14]), [0, 0])).toBe('left');
    expect(mapGamepadToAction(buttons([15]), [0, 0])).toBe('right');
  });

  it('maps A/B to confirm/back', () => {
    expect(mapGamepadToAction(buttons([0]), [0, 0])).toBe('confirm');
    expect(mapGamepadToAction(buttons([1]), [0, 0])).toBe('back');
  });

  it('maps left stick axes beyond deadzone', () => {
    expect(mapGamepadToAction(buttons([]), [0, -0.9])).toBe('up');
    expect(mapGamepadToAction(buttons([]), [0, 0.9])).toBe('down');
    expect(mapGamepadToAction(buttons([]), [-0.9, 0])).toBe('left');
    expect(mapGamepadToAction(buttons([]), [0.9, 0])).toBe('right');
  });

  it('ignores stick movement inside deadzone', () => {
    expect(mapGamepadToAction(buttons([]), [0.2, -0.2])).toBeNull();
  });

  it('prefers D-pad over axes when both active', () => {
    expect(mapGamepadToAction(buttons([12]), [0.9, 0.9])).toBe('up');
  });

  it('accepts boolean button arrays', () => {
    const bools = Array.from({ length: 16 }, (_, i) => i === 0);
    expect(mapGamepadToAction(bools, [0, 0])).toBe('confirm');
  });
});
