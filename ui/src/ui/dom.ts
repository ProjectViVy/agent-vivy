export function node<K extends keyof HTMLElementTagNameMap>(tag: K, className?: string): HTMLElementTagNameMap[K] {
  const element = document.createElement(tag);
  if (className) element.className = className;
  return element;
}

export function text(value: string): Text {
  return document.createTextNode(value);
}

export function button(label: string, className = "button button-secondary"): HTMLButtonElement {
  const element = node("button", className);
  element.type = "button";
  element.textContent = label;
  return element;
}

export function formatDate(value: number, locale: string, options: Intl.DateTimeFormatOptions = {}): string {
  return new Intl.DateTimeFormat(locale, {
    dateStyle: "medium",
    timeStyle: "short",
    ...options,
  }).format(new Date(value));
}

export function formatTime(value: number, locale: string): string {
  return new Intl.DateTimeFormat(locale, { timeStyle: "short" }).format(new Date(value));
}

export function setBusy(element: HTMLButtonElement, busy: boolean, busyLabel: string, normalLabel: string): void {
  element.disabled = busy;
  element.classList.toggle("is-busy", busy);
  element.textContent = busy ? busyLabel : normalLabel;
}

export function clear(element: Element): void {
  while (element.firstChild) element.firstChild.remove();
}
