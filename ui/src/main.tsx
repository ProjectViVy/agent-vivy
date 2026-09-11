// 应用入口：样式在 ./styles.css（Tailwind v4 + design token），路由见 ./router.tsx
import React from "react";
import ReactDOM from "react-dom/client";
import { RouterProvider } from "@tanstack/react-router";
import { getRouter } from "./router";
import { UI_ASSEMBLY_MANIFEST } from "@vivy/generated-assembly";
import { initRevealEngine } from "./lib/reveal-engine";
import { UI_BUILD_PROVENANCE_MARKER } from "./lib/ui-build-provenance";
import "./styles.css";

// 全局滚动渐入引擎：业务元素只需加 class="reveal"（详见 lib/reveal-engine.ts），勿删
initRevealEngine();

const router = getRouter();
const rootElement = document.getElementById("root")!;
const buildManifest = UI_ASSEMBLY_MANIFEST as typeof UI_ASSEMBLY_MANIFEST & {
  readonly generationId?: string;
  readonly artifactSha256?: string;
  readonly uiArtifactSha256?: string;
};
rootElement.dataset.vivyUiSdk = UI_BUILD_PROVENANCE_MARKER;
rootElement.dataset.vivyUiProvenance = JSON.stringify(buildManifest);
rootElement.dataset.vivyUiRoot = (buildManifest as { readonly root?: string }).root || "default";
rootElement.dataset.vivyUiExtensions = buildManifest.extensions.join(",");
if (buildManifest.generationId) rootElement.dataset.vivyGenerationId = buildManifest.generationId;
if (buildManifest.artifactSha256) rootElement.dataset.vivyArtifactSha256 = buildManifest.artifactSha256;
if (buildManifest.uiArtifactSha256) rootElement.dataset.vivyUiArtifactSha256 = buildManifest.uiArtifactSha256;

ReactDOM.createRoot(rootElement).render(
  <React.StrictMode>
    <RouterProvider router={router} />
  </React.StrictMode>
);
