/**
 * Thin wrapper over sonner so call sites stay consistent. Callers pass
 * already-translated strings (run them through t() first). The <Toaster/> is
 * mounted once in AppShell and themed to the design tokens.
 */
import { toast as sonner, type ExternalToast } from 'sonner';

export const toast = {
  success: (message: string, opts?: ExternalToast) => sonner.success(message, opts),
  error: (message: string, opts?: ExternalToast) => sonner.error(message, opts),
  info: (message: string, opts?: ExternalToast) => sonner.message(message, opts),
  warning: (message: string, opts?: ExternalToast) => sonner.warning(message, opts),
  /** Generic; use for neutral notices. */
  show: (message: string, opts?: ExternalToast) => sonner(message, opts),
  /** Promise lifecycle toast (loading → success/error). */
  promise: sonner.promise,
  dismiss: sonner.dismiss,
};
