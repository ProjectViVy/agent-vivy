/**
 * Google Antigravity subscription provider. This is intentionally separate
 * from Gemini CLI: it uses Antigravity OAuth scopes, project discovery, and
 * the daily-cloudcode-pa v1internal request envelope.
 */
import { LlmAdapter } from '@deepseek-ai/dsh-llm';
import type { GenerateOptions, LlmModelInfo, LlmProviderInfo, LlmResolvedModelInfo, StreamChunk } from '@deepseek-ai/dsh-llm';
import type { AttachmentStore } from '@deepseek-ai/dsh-attachment';
import type { FlowSpec } from '../auth/oauth-flow.js';
import type { AntigravitySession } from '../auth/store.js';
import type { AntigravityRequest } from '../translate/antigravity.js';
import { TokenManager } from './common.js';
import type { CatalogPersistence, DiscoveredModel, FetchFn, ModelEntry, ProviderUsage } from './common.js';
export declare const ANTIGRAVITY_AUTHORIZE_URL = "https://accounts.google.com/o/oauth2/v2/auth";
export declare const ANTIGRAVITY_TOKEN_URL = "https://oauth2.googleapis.com/token";
export declare const ANTIGRAVITY_USERINFO_URL = "https://www.googleapis.com/oauth2/v2/userinfo";
export declare const ANTIGRAVITY_DEFAULT_BASE_URL = "https://daily-cloudcode-pa.googleapis.com";
export declare const ANTIGRAVITY_PROD_BASE_URL = "https://cloudcode-pa.googleapis.com";
export declare const ANTIGRAVITY_DEFAULT_USER_AGENT = "antigravity/1.104.0 dsh-plugin-subscriptions";
export declare const ANTIGRAVITY_PREEMPT_MS: number;
/** Antigravity, not Gemini CLI, OAuth scopes from the local reference clients. */
export declare const ANTIGRAVITY_SCOPES: readonly ["openid", "https://www.googleapis.com/auth/cloud-platform", "https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile", "https://www.googleapis.com/auth/cclog", "https://www.googleapis.com/auth/experimentsandconfigs"];
/** OAuth client configuration. Values must come from config/environment. */
export interface AntigravityOAuthConfig {
    clientId: string;
    clientSecret?: string;
}
/** Runtime endpoint configuration. */
export interface AntigravityRuntimeConfig {
    baseURL?: string;
    userAgent?: string;
    /** Activate an eligible account when loadCodeAssist has no project yet. */
    onboard?: boolean;
}
/** Resolve and validate a user-supplied OAuth config without embedded credentials. */
export declare function resolveAntigravityOAuthConfig(config?: Partial<AntigravityOAuthConfig>): AntigravityOAuthConfig;
/** Normalize the configured API origin and reject paths/credentials. */
export declare function antigravityBaseURL(value?: string): string;
/** Google authorization-code + PKCE flow for Antigravity. */
export declare function antigravityFlow(oauth: AntigravityOAuthConfig): FlowSpec;
interface AntigravityAccountInfo {
    projectId: string;
    account?: string;
    plan?: string;
}
/** Shared Antigravity API headers. */
export declare function antigravityHeaders(accessToken: string, userAgent?: string): Record<string, string>;
/** Read (and, when enabled, initialize) the Antigravity project/account. */
export declare function discoverAntigravityAccount(accessToken: string, runtime?: AntigravityRuntimeConfig, fetchFn?: FetchFn): Promise<AntigravityAccountInfo>;
/** Exchange a Google OAuth authorization code and discover the Antigravity project. */
export declare function exchangeAntigravityCode(code: string, verifier: string, redirectUri: string, oauth: AntigravityOAuthConfig, runtime?: AntigravityRuntimeConfig, fetchFn?: FetchFn): Promise<AntigravitySession>;
/** Refresh a stored Antigravity Google token, preserving project/account metadata. */
export declare function refreshAntigravity(session: AntigravitySession, oauth: AntigravityOAuthConfig, fetchFn?: FetchFn): Promise<AntigravitySession>;
/** Refresh failures that require a fresh Google consent grant. */
export declare function isAntigravityPermanentRefreshError(error: unknown): boolean;
/** Fetch the authenticated account's live Antigravity model catalog. */
export declare function fetchAntigravityModels(session: AntigravitySession, runtime?: AntigravityRuntimeConfig, fetchFn?: FetchFn): Promise<DiscoveredModel[]>;
/** Fetch plan and per-model quota windows when the upstream exposes them. */
export declare function fetchAntigravityUsage(session: AntigravitySession, runtime?: AntigravityRuntimeConfig, fetchFn?: FetchFn, signal?: AbortSignal): Promise<ProviderUsage>;
/** URL for either v1internal generation transport. */
export declare function antigravityGenerateURL(baseURL: string | undefined, stream: boolean): string;
/** Forward one already-built payload to generateContent or streamGenerateContent. */
export declare function requestAntigravityContent(session: AntigravitySession, payload: AntigravityRequest, stream: boolean, runtime?: AntigravityRuntimeConfig, fetchFn?: FetchFn, signal?: AbortSignal): Promise<Response>;
export interface AntigravityAdapterOptions {
    models: readonly ModelEntry[];
    streamIdleTimeoutMs: number;
    tokens: TokenManager<AntigravitySession>;
    discovery: boolean;
    runtime?: AntigravityRuntimeConfig;
    onWarn?: (message: string) => void;
    fetchFn?: FetchFn;
    resolveAttachments?: () => AttachmentStore | undefined;
    catalogStore?: CatalogPersistence;
}
/** DSH provider adapter for the `antigravity` route. */
export declare class AntigravityAdapter extends LlmAdapter {
    private readonly options;
    private readonly catalog;
    constructor(options: AntigravityAdapterOptions);
    providerInfo(provider: string): LlmProviderInfo;
    private staticModels;
    private fetchCatalog;
    listModels(provider: string): Promise<readonly LlmModelInfo[]>;
    private discovered;
    resolveModel(provider: string, model: string): Promise<LlmResolvedModelInfo>;
    stream(options: GenerateOptions): AsyncIterable<StreamChunk>;
    /** Non-stream forwarding seam used by tests and future DSH complete calls. */
    generate(options: GenerateOptions): Promise<StreamChunk[]>;
}
export {};
