import { describe, expect, it } from "vitest";
import {
  UI_BUILD_MANIFEST,
  UI_BUILD_PROVENANCE_MARKER,
} from "./ui-build-provenance";
import type {
  FaceClientAPI,
  FaceClientRouter,
  FaceClientStore,
  FaceNavigationOptions,
} from "@vivy/ui-sdk";

type FaceAPICompatible = typeof import("./api") extends FaceClientAPI ? true : false;
type FaceStoreCompatible = typeof import("./store").useVivyStore extends FaceClientStore ? true : false;
type CurrentFaceRouter = ReturnType<typeof import("../router").getRouter>;
type CurrentRouterNavigateOptions = Parameters<CurrentFaceRouter["navigate"]>[0];
type FaceRouterCompatible =
  CurrentFaceRouter["invalidate"] extends FaceClientRouter["invalidate"]
    ? ReturnType<CurrentFaceRouter["navigate"]> extends Promise<void>
      ? Exclude<keyof FaceNavigationOptions, keyof CurrentRouterNavigateOptions> extends never ? true : false
      : false
    : false;
const faceAPIContract: true = true as FaceAPICompatible;
const faceStoreContract: true = true as FaceStoreCompatible;
const faceRouterContract: true = true as FaceRouterCompatible;
void faceAPIContract;
void faceStoreContract;
void faceRouterContract;

describe("UI build provenance", () => {
  it("publishes the pinned SDK identity through the consumed build manifest", () => {
    expect(UI_BUILD_MANIFEST.uiSdk).toEqual({
      packageName: "@vivy/ui-sdk",
      version: "1.0.0",
    });
    expect(UI_BUILD_PROVENANCE_MARKER).toBe("@vivy/ui-sdk@1.0.0");
  });
});
