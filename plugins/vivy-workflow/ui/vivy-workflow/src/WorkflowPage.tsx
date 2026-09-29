/**
 * Host page for the vendored INOFY editor.
 *
 * The editor is mounted inside a Shadow DOM root so its stylesheet (a global
 * document stylesheet upstream) neither leaks into the VIVY shell nor gets
 * overridden by it. Selector scoping is done textually on mount: `:root`
 * custom properties move to `:host`, and the upstream `body` rules land on
 * the `.studio-shell` mount node. The vendored file itself stays
 * byte-identical (see studio/VENDORED.md).
 */
import { useEffect, useRef } from 'react';
import { createRoot } from 'react-dom/client';
import { usePluginHost } from '@vivy/ui-sdk';
import studioCss from './studio/styles.css?inline';
import xyflowCss from '@xyflow/react/dist/style.css?inline';
import { App } from './studio/App';
import { FaceBridge, FaceVivyTransport } from './face-bridge';

const scopedCss = `${studioCss
  .replace(/:root\b/g, ':host')
  .replace(/((?:^|[{}>,])\s*)body\b/gm, '$1.studio-shell')}\n${xyflowCss}`;

export function WorkflowPage() {
  const host = usePluginHost();
  const mountRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const mount = mountRef.current;
    if (!mount || !host) return;
    if (!host.rpc || !host.store) return;
    const shadow = mount.shadowRoot ?? mount.attachShadow({ mode: 'open' });
    const style = document.createElement('style');
    style.textContent = scopedCss;
    const inner = document.createElement('div');
    inner.className = 'studio-shell';
    inner.style.height = '100%';
    inner.style.width = '100%';
    shadow.append(style, inner);

    const transport = new FaceVivyTransport(new FaceBridge(host.rpc, host.store));
    const root = createRoot(inner);
    root.render(<App transport={transport} />);
    return () => {
      // Synchronous unmount races React's own render of this tree; defer it.
      // Only this effect's nodes may be removed — a replayed mount appends a
      // second pair into the same shadow root.
      setTimeout(() => {
        root.unmount();
        style.remove();
        inner.remove();
      }, 0);
    };
  }, [host]);

  return <div data-testid="vivy-workflow-studio" ref={mountRef} style={{ height: '100%', minHeight: 0 }} />;
}
