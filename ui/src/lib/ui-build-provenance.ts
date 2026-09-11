import {
  UI_SDK_PACKAGE_NAME,
  UI_SDK_PROVENANCE,
  UI_SDK_VERSION,
} from "@vivy/ui-sdk";

export interface UIBuildManifest {
  readonly uiSdk: {
    readonly packageName: string;
    readonly version: string;
  };
}

/** Replaced by Vite's sealed build value; tests use the SDK fallback. */
declare const __VIVY_UI_BUILD_PROVENANCE__: UIBuildManifest;
declare const __VIVY_UI_SDK_PACKAGE_NAME__: string;
declare const __VIVY_UI_SDK_VERSION__: string;
declare const __VIVY_UI_SDK_PROVENANCE_MARKER__: string;

function readInjectedBuildManifest(): UIBuildManifest | undefined {
  try {
    const manifest = __VIVY_UI_BUILD_PROVENANCE__;
    return {
      ...manifest,
      uiSdk: {
        ...manifest.uiSdk,
        packageName: __VIVY_UI_SDK_PACKAGE_NAME__,
        version: __VIVY_UI_SDK_VERSION__,
      },
    };
  } catch {
    return undefined;
  }
}

const injectedBuildManifest = readInjectedBuildManifest();

function readInjectedProvenanceMarker(): string | undefined {
  try {
    return __VIVY_UI_SDK_PROVENANCE_MARKER__;
  } catch {
    return undefined;
  }
}

/** The consumed manifest entry carried by the running Web Face artifact. */
export const UI_BUILD_MANIFEST: UIBuildManifest = Object.freeze({
  uiSdk: Object.freeze({
    ...(injectedBuildManifest?.uiSdk ?? UI_SDK_PROVENANCE),
  }),
});

/** Human-readable marker retained in the artifact for diagnostics and Inspect. */
export const UI_BUILD_PROVENANCE_MARKER = readInjectedProvenanceMarker()
  ?? `${UI_BUILD_MANIFEST.uiSdk.packageName}@${UI_BUILD_MANIFEST.uiSdk.version}`;

// Keep the public identity imports in this module's type surface so package
// upgrades cannot silently change the manifest's expected shape.
export type UIBuildSDKPackageName = typeof UI_SDK_PACKAGE_NAME;
export type UIBuildSDKVersion = typeof UI_SDK_VERSION;
