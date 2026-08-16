export interface ShellElements {
  sidebar: HTMLElement;
  sidebarScrim: HTMLButtonElement;
  newSession: HTMLButtonElement;
  reviewCenterButton: HTMLButtonElement;
  studioButton: HTMLButtonElement;
  sessionList: HTMLUListElement;
  sidebarFooter: HTMLElement;
  activityHeading: HTMLElement;
  activityList: HTMLUListElement;
  localeButton: HTMLButtonElement;
  themeButton: HTMLButtonElement;
  mobileSidebarButton: HTMLButtonElement;
  main: HTMLElement;
  sessionTitle: HTMLElement;
  sessionMeta: HTMLElement;
  runStatus: HTMLElement;
  cancelRun: HTMLButtonElement;
  inspectorToggle: HTMLButtonElement;
  chat: HTMLElement;
  composer: HTMLFormElement;
  composerInput: HTMLTextAreaElement;
  modeToggle: HTMLInputElement;
  modeLabel: HTMLElement;
  sendButton: HTMLButtonElement;
  inspector: HTMLElement;
  inspectorTitle: HTMLElement;
  inspectorClose: HTMLButtonElement;
  inspectorConnection: HTMLElement;
  inspectorTabs: HTMLElement;
  inspectorBody: HTMLElement;
  reviewCenter: HTMLElement;
  reviewCenterBody: HTMLElement;
  studio: HTMLElement;
  studioBody: HTMLElement;
  dialogRoot: HTMLElement;
}

function required<T extends Element>(root: ParentNode, selector: string): T {
  const element = root.querySelector<T>(selector);
  if (!element) throw new Error(`Vivy UI shell is missing ${selector}`);
  return element;
}

export function createShell(root: HTMLElement): ShellElements {
  root.innerHTML = `
    <div class="app-shell">
      <button class="sidebar-scrim" id="sidebar-scrim" type="button" aria-hidden="true"></button>
      <aside class="sidebar" id="sidebar">
        <header class="sidebar-header">
          <div class="brand-lockup"><span class="brand-mark">V</span><span class="brand-name" data-copy="appName">Vivy</span></div>
          <button class="icon-button mobile-only" id="mobile-sidebar-close" type="button" aria-label="Close">×</button>
        </header>
        <div class="sidebar-actions">
          <button class="button button-primary button-wide" id="new-session" type="button"></button>
          <button class="button button-secondary button-wide" id="review-center-button" type="button"></button>
          <button class="button button-secondary button-wide" id="studio-button" type="button"></button>
        </div>
        <div class="section-heading"><span data-copy="sessions">Sessions</span></div>
        <ul class="session-list" id="session-list"></ul>
        <div class="section-heading section-heading-activity" id="activity-heading" hidden><span data-copy="activity">Live activity</span></div>
        <ul class="activity-list" id="activity-list" hidden></ul>
        <footer class="sidebar-footer" id="sidebar-footer">
          <button class="preference-button" id="locale-button" type="button"></button>
          <button class="preference-button" id="theme-button" type="button"></button>
        </footer>
      </aside>
      <main class="main-column" id="main">
        <header class="main-header">
          <button class="icon-button mobile-only" id="mobile-sidebar-open" type="button" aria-label="Open sessions">☰</button>
          <div class="session-heading">
            <h1 id="session-title">Vivy</h1>
            <p id="session-meta"></p>
          </div>
          <div class="header-actions">
            <span class="run-status" id="run-status"></span>
            <button class="button button-danger button-small" id="cancel-run" type="button" hidden></button>
            <button class="button button-secondary button-small" id="inspector-toggle" type="button"></button>
          </div>
        </header>
        <section class="chat-region" id="chat" aria-live="polite"></section>
        <form class="composer" id="composer">
          <div class="composer-input-wrap">
            <textarea id="composer-input" rows="3"></textarea>
            <div class="composer-tools">
              <label class="mode-toggle"><input id="plan-mode" type="checkbox" /><span id="mode-label"></span></label>
              <span class="composer-error" id="composer-error" hidden></span>
            </div>
          </div>
          <button class="button button-primary composer-send" id="composer-send" type="submit"></button>
        </form>
        <section class="review-center" id="review-center" hidden aria-labelledby="review-center-title">
          <div id="review-center-body"></div>
        </section>
        <section class="review-center" id="studio" hidden aria-labelledby="studio-title">
          <div id="studio-body"></div>
        </section>
      </main>
      <aside class="inspector" id="inspector" aria-label="Run inspector">
        <header class="inspector-header">
          <div><p class="eyebrow" data-copy="run">Run</p><h2 id="inspector-title"></h2></div>
          <button class="icon-button" id="inspector-close" type="button" aria-label="Close">×</button>
        </header>
        <div class="inspector-connection" id="inspector-connection"></div>
        <nav class="inspector-tabs" id="inspector-tabs"></nav>
        <div class="inspector-body" id="inspector-body"></div>
      </aside>
    </div>`;

  const dialogRoot = required<HTMLElement>(document, "#dialog-root");
  return {
    sidebar: required<HTMLElement>(root, "#sidebar"),
    sidebarScrim: required<HTMLButtonElement>(root, "#sidebar-scrim"),
    newSession: required<HTMLButtonElement>(root, "#new-session"),
    reviewCenterButton: required<HTMLButtonElement>(root, "#review-center-button"),
    studioButton: required<HTMLButtonElement>(root, "#studio-button"),
    sessionList: required<HTMLUListElement>(root, "#session-list"),
    sidebarFooter: required<HTMLElement>(root, "#sidebar-footer"),
    activityHeading: required<HTMLElement>(root, "#activity-heading"),
    activityList: required<HTMLUListElement>(root, "#activity-list"),
    localeButton: required<HTMLButtonElement>(root, "#locale-button"),
    themeButton: required<HTMLButtonElement>(root, "#theme-button"),
    mobileSidebarButton: required<HTMLButtonElement>(root, "#mobile-sidebar-open"),
    main: required<HTMLElement>(root, "#main"),
    sessionTitle: required<HTMLElement>(root, "#session-title"),
    sessionMeta: required<HTMLElement>(root, "#session-meta"),
    runStatus: required<HTMLElement>(root, "#run-status"),
    cancelRun: required<HTMLButtonElement>(root, "#cancel-run"),
    inspectorToggle: required<HTMLButtonElement>(root, "#inspector-toggle"),
    chat: required<HTMLElement>(root, "#chat"),
    composer: required<HTMLFormElement>(root, "#composer"),
    composerInput: required<HTMLTextAreaElement>(root, "#composer-input"),
    modeToggle: required<HTMLInputElement>(root, "#plan-mode"),
    modeLabel: required<HTMLElement>(root, "#mode-label"),
    sendButton: required<HTMLButtonElement>(root, "#composer-send"),
    inspector: required<HTMLElement>(root, "#inspector"),
    inspectorTitle: required<HTMLElement>(root, "#inspector-title"),
    inspectorClose: required<HTMLButtonElement>(root, "#inspector-close"),
    inspectorConnection: required<HTMLElement>(root, "#inspector-connection"),
    inspectorTabs: required<HTMLElement>(root, "#inspector-tabs"),
    inspectorBody: required<HTMLElement>(root, "#inspector-body"),
    reviewCenter: required<HTMLElement>(root, "#review-center"),
    reviewCenterBody: required<HTMLElement>(root, "#review-center-body"),
    studio: required<HTMLElement>(root, "#studio"),
    studioBody: required<HTMLElement>(root, "#studio-body"),
    dialogRoot,
  };
}
