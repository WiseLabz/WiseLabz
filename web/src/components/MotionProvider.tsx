/**
 * Bridges the in-app motion setting to framer/motion. When the user picks
 * Reduced or Off, framer springs collapse to instant; CSS handles the rest via
 * [data-motion] (set by the settings store). Motion stays ON by default.
 */
import { MotionConfig } from 'motion/react';
import type { ReactNode } from 'react';
import { useSettings, framerReducedMotion } from '../store/settings';

export function MotionProvider({ children }: { children: ReactNode }) {
  const motion = useSettings((s) => s.motion);
  return (
    <MotionConfig reducedMotion={framerReducedMotion(motion)}>
      {children}
    </MotionConfig>
  );
}
