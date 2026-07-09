import "@testing-library/jest-dom/vitest";

// jsdom has no ResizeObserver; Radix UI primitives (e.g. Select) touch it on
// mount to measure trigger size. Stub it out rather than pull in a browser.
if (typeof globalThis.ResizeObserver === "undefined") {
  globalThis.ResizeObserver = class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
}
