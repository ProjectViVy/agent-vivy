import { node } from "./dom";

interface BaseDialogOptions {
  title: string;
  description?: string;
  confirmLabel: string;
  cancelLabel: string;
  danger?: boolean;
}

export interface ConfirmDialogOptions extends BaseDialogOptions {
  onConfirm?: () => Promise<void>;
}

export interface PromptDialogOptions extends BaseDialogOptions {
  label: string;
  initialValue: string;
  onConfirm?: (value: string) => Promise<void>;
}

export class DialogHost {
  private readonly dialog: HTMLDialogElement;
  private readonly title: HTMLHeadingElement;
  private readonly description: HTMLParagraphElement;
  private readonly form: HTMLFormElement;
  private readonly body: HTMLDivElement;
  private readonly error: HTMLParagraphElement;
  private readonly cancelButton: HTMLButtonElement;
  private readonly confirmButton: HTMLButtonElement;
  private busy = false;
  private finish: ((value: boolean | string | null) => void) | null = null;

  constructor(root: HTMLElement) {
    this.dialog = node("dialog", "dialog-shell");
    const header = node("div", "dialog-header");
    this.title = node("h2");
    header.appendChild(this.title);
    this.dialog.appendChild(header);
    this.form = node("form", "dialog-form");
    this.form.method = "dialog";
    this.body = node("div", "dialog-body");
    this.description = node("p", "dialog-description");
    this.body.appendChild(this.description);
    this.form.appendChild(this.body);
    this.error = node("p", "form-error");
    this.error.hidden = true;
    this.form.appendChild(this.error);
    const footer = node("div", "dialog-footer");
    this.cancelButton = node("button", "button button-secondary");
    this.cancelButton.type = "button";
    this.confirmButton = node("button", "button button-primary");
    this.confirmButton.type = "submit";
    footer.append(this.cancelButton, this.confirmButton);
    this.form.appendChild(footer);
    this.dialog.appendChild(this.form);
    root.appendChild(this.dialog);

    this.cancelButton.addEventListener("click", () => this.close(false));
    this.dialog.addEventListener("cancel", (event) => {
      if (this.busy) {
        event.preventDefault();
        return;
      }
      this.close(false);
    });
  }

  async confirm(options: ConfirmDialogOptions): Promise<boolean> {
    return await new Promise<boolean>((resolve) => {
      this.prepare(options, (value) => resolve(value === true), undefined);
    });
  }

  async prompt(options: PromptDialogOptions): Promise<string | null> {
    return await new Promise<string | null>((resolve) => {
      const input = node("input", "text-input");
      input.type = "text";
      input.name = "value";
      input.required = true;
      input.value = options.initialValue;
      input.placeholder = options.label;
      const label = node("label", "field-label");
      label.textContent = options.label;
      label.appendChild(input);
      this.prepare(options, (value) => resolve(typeof value === "string" ? value : null), input);
      this.body.appendChild(label);
      window.setTimeout(() => input.select(), 0);
    });
  }

  private prepare(
    options: BaseDialogOptions & { onConfirm?: (value: string) => Promise<void> },
    resolve: (value: boolean | string | null) => void,
    input: HTMLInputElement | undefined,
  ): void {
    this.finish = resolve;
    this.busy = false;
    this.title.textContent = options.title;
    this.description.textContent = options.description ?? "";
    this.description.hidden = !options.description;
    this.cancelButton.textContent = options.cancelLabel;
    this.confirmButton.textContent = options.confirmLabel;
    this.confirmButton.className = `button ${options.danger ? "button-danger" : "button-primary"}`;
    this.error.hidden = true;
    this.error.textContent = "";
    for (const child of [...this.body.children]) {
      if (child !== this.description) child.remove();
    }
    const submit = async (event: SubmitEvent) => {
      event.preventDefault();
      if (this.busy) return;
      const value = input?.value.trim();
      if (input && !value) {
        input.focus();
        return;
      }
        this.setBusy(true, options.confirmLabel);
        try {
          if (options.onConfirm) await options.onConfirm(value ?? "");
          this.setBusy(false, options.confirmLabel);
          this.close(input ? value ?? null : true);
      } catch (error) {
        this.setError(error instanceof Error ? error.message : String(error));
        this.setBusy(false, options.confirmLabel);
      }
    };
    this.form.onsubmit = (event) => void submit(event);
    this.dialog.showModal();
  }

  private setBusy(busy: boolean, label: string): void {
    this.busy = busy;
    this.confirmButton.disabled = busy;
    this.cancelButton.disabled = busy;
    this.confirmButton.textContent = busy ? "…" : label;
  }

  private setError(message: string): void {
    this.error.hidden = false;
    this.error.textContent = message;
  }

  private close(value: boolean | string | null): void {
    if (this.busy) return;
    this.dialog.close();
    const resolve = this.finish;
    this.finish = null;
    resolve?.(value);
  }
}
