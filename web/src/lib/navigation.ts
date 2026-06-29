/**
 * Tiny navigator singleton so non-React code (the WS event handler, which lives
 * above the router) can do SPA navigation — e.g. click-to-jump toasts. A bridge
 * component inside the router tree registers react-router's `navigate` here.
 */
type NavigateFn = (to: string) => void;

let navigator: NavigateFn | null = null;

export function setAppNavigator(fn: NavigateFn | null) {
  navigator = fn;
}

/** SPA-navigate from anywhere. No-op until the bridge has mounted. */
export function navigateTo(to: string) {
  navigator?.(to);
}
