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

// jsdom implements neither the Pointer Capture API nor scrollIntoView, both
// of which Radix UI's Select touches while opening/positioning its portal
// content — without these stubs, driving a Select open in a test throws.
if (typeof Element.prototype.hasPointerCapture === "undefined") {
  Element.prototype.hasPointerCapture = () => false;
}
if (typeof Element.prototype.setPointerCapture === "undefined") {
  Element.prototype.setPointerCapture = () => {};
}
if (typeof Element.prototype.releasePointerCapture === "undefined") {
  Element.prototype.releasePointerCapture = () => {};
}
if (typeof Element.prototype.scrollIntoView === "undefined") {
  Element.prototype.scrollIntoView = () => {};
}
