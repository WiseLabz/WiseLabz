/**
 * Command-palette registry. Later feature phases contribute their own commands by
 * calling `registerCommands(factory)` at module load — the palette composes the
 * built-in commands with every registered factory at open time, passing a shared
 * context (navigation, i18n, role, query client) so commands can navigate, mutate,
 * or invalidate without each owning that plumbing.
 */
import type { NavigateFunction } from 'react-router-dom';
import type { QueryClient } from '@tanstack/react-query';
import type { TFunction } from 'i18next';

export type CommandGroup = 'navigate' | 'actions' | 'services' | 'docs';

export interface CommandCtx {
  navigate: NavigateFunction;
  t: TFunction;
  /** True for operator role — mutating commands hide themselves when false. */
  canMutate: boolean;
  queryClient: QueryClient;
}

export interface Command {
  id: string;
  label: string;
  hint?: string;
  group: CommandGroup;
  Icon: React.ComponentType<{ size?: number; className?: string }>;
  run: (ctx: CommandCtx) => void;
}

export type CommandFactory = (ctx: CommandCtx) => Command[];

const factories: CommandFactory[] = [];

/** Register a factory of extra commands. Call once at feature-module load. */
export function registerCommands(factory: CommandFactory): void {
  factories.push(factory);
}

/** Flatten every registered factory for the current context. */
export function registeredCommands(ctx: CommandCtx): Command[] {
  return factories.flatMap((f) => f(ctx));
}
