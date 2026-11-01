// Generated from vivy-diva-front Rust DTOs. Do not edit.
export const PROTOCOL_VERSION = 1 as const;
export type Request = { v: number, id: string, method: Method, };
export type Method = "snapshot" | "shutdown";
export type Frame = { v: number, instance: string, seq: number, id: string | null, } & ({ "type": "ready", "data": Snapshot } | { "type": "reply", "data": Reply } | { "type": "error", "data": Fault });
export type Payload = { "type": "ready", "data": Snapshot } | { "type": "reply", "data": Reply } | { "type": "error", "data": Fault };
export type Reply = Snapshot | { shutting_down: boolean, };
export type Snapshot = { session: string | null, assistant: string | null, core_version: string, protocol_version: number, watermark: number, plugins: Array<PluginInfo>, agents: Array<AgentInfo>, model: Readiness, python: Readiness, memory: Readiness, };
export type PluginInfo = { name: string, package: string, compiled: boolean, };
export type AgentInfo = { id: string, working: boolean, };
export type Readiness = { compiled: boolean, configured: boolean, ready: boolean, state: string, issue: string | null, };
export type ErrorCode = "invalid_frame" | "invalid_request" | "incompatible_version" | "unsupported_operation" | "conflict" | "not_ready" | "stale_identity" | "storage_failure" | "indeterminate";
export type Fault = { code: ErrorCode, message: string, };
