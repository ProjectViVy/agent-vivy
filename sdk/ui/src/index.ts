/** Public package/version identity used by deterministic frontend builds. */
export const UI_SDK_PACKAGE_NAME = "@vivy/ui-sdk" as const;
export const UI_SDK_VERSION = "1.0.0" as const;
export const UI_SDK_PROVENANCE = Object.freeze({
  packageName: UI_SDK_PACKAGE_NAME,
  version: UI_SDK_VERSION,
});

export * from "./module";
export * from "./action-client";
