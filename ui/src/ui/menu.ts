import { node } from "./dom";

export interface MenuItem {
  label: string;
  danger?: boolean;
  disabled?: boolean;
  onSelect: () => void;
}

export class FloatingMenu {
  private current: HTMLElement | null = null;
  private removeDocumentListener: (() => void) | null = null;
  private removeViewportListeners: (() => void) | null = null;

  open(anchor: HTMLElement, items: MenuItem[]): void {
    this.close();
    const root = document.getElementById("overlay-root");
    if (!root) return;
    const menu = node("div", "floating-menu");
    menu.setAttribute("role", "menu");
    for (const item of items) {
      const entry = node("button", `menu-item${item.danger ? " is-danger" : ""}`);
      entry.type = "button";
      entry.textContent = item.label;
      entry.disabled = item.disabled ?? false;
      entry.setAttribute("role", "menuitem");
      entry.addEventListener("click", () => {
        if (entry.disabled) return;
        this.close();
        item.onSelect();
      });
      menu.appendChild(entry);
    }
    root.appendChild(menu);
    this.current = menu;
    this.position(anchor, menu);
    const onDocumentPointer = (event: PointerEvent) => {
      const target = event.target as Node;
      if (!menu.contains(target) && !anchor.contains(target)) this.close();
    };
    document.addEventListener("pointerdown", onDocumentPointer, true);
    this.removeDocumentListener = () => document.removeEventListener("pointerdown", onDocumentPointer, true);
    const reposition = () => this.position(anchor, menu);
    window.addEventListener("resize", reposition);
    window.addEventListener("scroll", reposition, true);
    this.removeViewportListeners = () => {
      window.removeEventListener("resize", reposition);
      window.removeEventListener("scroll", reposition, true);
    };
  }

  close(): void {
    this.removeDocumentListener?.();
    this.removeViewportListeners?.();
    this.removeDocumentListener = null;
    this.removeViewportListeners = null;
    this.current?.remove();
    this.current = null;
  }

  private position(anchor: HTMLElement, menu: HTMLElement): void {
    const rect = anchor.getBoundingClientRect();
    const menuRect = menu.getBoundingClientRect();
    const margin = 10;
    const top = rect.bottom + menuRect.height + margin <= window.innerHeight
      ? rect.bottom + 6
      : Math.max(margin, rect.top - menuRect.height - 6);
    const left = Math.min(
      Math.max(margin, rect.right - menuRect.width),
      Math.max(margin, window.innerWidth - menuRect.width - margin),
    );
    menu.style.top = `${top}px`;
    menu.style.left = `${left}px`;
  }
}
