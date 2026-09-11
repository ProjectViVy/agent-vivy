import { describe, expect, it } from "vitest";
import type {
  FaceClientAPI,
  FaceClientRouter,
  FaceClientStore,
  FaceNavigationOptions,
  FaceStoreState,
} from "@vivy/ui-sdk";

type CurrentFaceAPI = typeof import("./api");
type CurrentFaceStore = typeof import("./store").useVivyStore;
type CurrentFaceRouter = ReturnType<typeof import("../router").getRouter>;

type FunctionKeys<T> = {
  [Key in keyof T]-?: T[Key] extends (...args: never[]) => unknown ? Key : never
}[keyof T];

type CurrentFaceOperationKeys = FunctionKeys<CurrentFaceAPI>;
type MissingFaceAPIOperations = Exclude<CurrentFaceOperationKeys, keyof FaceClientAPI>;
type ExtraFaceAPIOperations = Exclude<keyof FaceClientAPI, CurrentFaceOperationKeys>;
type FaceAPIHasCompleteSurface =
  [MissingFaceAPIOperations, ExtraFaceAPIOperations] extends [never, never] ? true : false;
const faceAPIHasCompleteSurface: true = true as FaceAPIHasCompleteSurface;

type CurrentRuntimeState = ReturnType<CurrentFaceStore["getState"]>;
type MissingFaceStoreStateKeys = Exclude<keyof CurrentRuntimeState, keyof FaceStoreState>;
type ExtraFaceStoreStateKeys = Exclude<keyof FaceStoreState, keyof CurrentRuntimeState>;
type FaceStoreHasCompleteSurface =
  [MissingFaceStoreStateKeys, ExtraFaceStoreStateKeys] extends [never, never] ? true : false;
const faceStoreHasCompleteSurface: true = true as FaceStoreHasCompleteSurface;

type FaceAPICompatible = CurrentFaceAPI extends FaceClientAPI ? true : false;
type FaceStoreCompatible = CurrentFaceStore extends FaceClientStore ? true : false;
type CurrentRouterNavigateOptions = Parameters<CurrentFaceRouter["navigate"]>[0];
type FaceRouterCompatible =
  CurrentFaceRouter["invalidate"] extends FaceClientRouter["invalidate"]
    ? ReturnType<CurrentFaceRouter["navigate"]> extends Promise<void> ? true : false
    : false;

const faceAPICompatible: true = true as FaceAPICompatible;
const faceStoreCompatible: true = true as FaceStoreCompatible;
const faceRouterCompatible: true = true as FaceRouterCompatible;

type CurrentApprovalResponse = Awaited<ReturnType<CurrentFaceAPI["respondApproval"]>>;
type CurrentQuestionResponse = Awaited<ReturnType<CurrentFaceAPI["respondQuestion"]>>;
type SDKApprovalResponse = Awaited<ReturnType<FaceClientAPI["respondApproval"]>>;
type SDKQuestionResponse = Awaited<ReturnType<FaceClientAPI["respondQuestion"]>>;
type ApprovalResponseHasPayload = CurrentApprovalResponse extends {
  approval_id: string;
  decision: "approved" | "denied";
} ? true : false;
type QuestionResponseHasPayload = CurrentQuestionResponse extends {
  question_id: string;
  answer: string;
} ? true : false;
type SDKApprovalResponseHasPayload = SDKApprovalResponse extends {
  approval_id: string;
  decision: "approved" | "denied";
} ? true : false;
type SDKQuestionResponseHasPayload = SDKQuestionResponse extends {
  question_id: string;
  answer: string;
} ? true : false;
const approvalResponseHasPayload: true = true as ApprovalResponseHasPayload;
const questionResponseHasPayload: true = true as QuestionResponseHasPayload;
const sdkApprovalResponseHasPayload: true = true as SDKApprovalResponseHasPayload;
const sdkQuestionResponseHasPayload: true = true as SDKQuestionResponseHasPayload;

type MissingCurrentNavigationKeys = Exclude<
  keyof FaceNavigationOptions,
  keyof CurrentRouterNavigateOptions
>;
type ExtraCurrentNavigationKeys = Exclude<
  keyof CurrentRouterNavigateOptions,
  keyof FaceNavigationOptions
>;
type FaceNavigationHasExactKeys =
  [MissingCurrentNavigationKeys, ExtraCurrentNavigationKeys] extends [never, never] ? true : false;
const faceNavigationHasExactKeys: true = true as FaceNavigationHasExactKeys;

const navigationOptions: FaceNavigationOptions = {
  to: "/settings",
  from: "/",
  params: { sessionId: "fixture" },
  search: { tab: "model" },
  hash: "models",
  replace: true,
};
void navigationOptions;

describe("complete Face client SDK compatibility", () => {
  it("keeps every current client seam represented by the public contract", () => {
    expect(faceAPIHasCompleteSurface).toBe(true);
    expect(faceStoreHasCompleteSurface).toBe(true);
    expect(faceAPICompatible).toBe(true);
    expect(faceStoreCompatible).toBe(true);
    expect(faceRouterCompatible).toBe(true);
    expect(approvalResponseHasPayload).toBe(true);
    expect(questionResponseHasPayload).toBe(true);
    expect(sdkApprovalResponseHasPayload).toBe(true);
    expect(sdkQuestionResponseHasPayload).toBe(true);
    expect(faceNavigationHasExactKeys).toBe(true);
  });
});
