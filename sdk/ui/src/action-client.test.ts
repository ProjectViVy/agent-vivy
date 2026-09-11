import { describe, expect, it } from "vitest";

import {
  MODULE_ACTION_METHOD,
  ModuleActionClientError,
  createModuleActionClient,
  type ModuleActionRPC,
} from "./action-client";

function rpcStub(result: unknown = { ok: true }): ModuleActionRPC & { calls: Array<{ method: string; params: unknown }> } {
  const calls: Array<{ method: string; params: unknown }> = [];
  return {
    calls,
    call: async <T>(method: string, params?: unknown) => {
      calls.push({ method, params });
      return result as T;
    },
  };
}

describe("ModuleActionClient", () => {
  it("serializes one bounded module action request through the fixed RPC method", async () => {
    const rpc = rpcStub({ ok: true });
    const client = createModuleActionClient(rpc);

    await expect(client.invoke("example/module", "example.action.read", { value: "ok" }))
      .resolves.toEqual({ ok: true });
    expect(rpc.calls).toEqual([{
      method: MODULE_ACTION_METHOD,
      params: {
        module_id: "example/module",
        action_id: "example.action.read",
        input: { value: "ok" },
      },
    }]);
  });

  it("accepts a typed request object without making an authority decision", async () => {
    const rpc = rpcStub({ accepted: true });
    const client = createModuleActionClient(rpc);
    const input = { value: "ok", approval: "browser data" };

    await expect(client.invoke({ moduleId: "example/module", actionId: "example.action.read", input }))
      .resolves.toEqual({ accepted: true });
    expect(rpc.calls[0]?.params).toEqual({
      module_id: "example/module",
      action_id: "example.action.read",
      input,
    });
  });

  it("rejects oversized or deeply nested input before calling the transport", async () => {
    const rpc = rpcStub();
    const client = createModuleActionClient(rpc);
    const oversized = { value: "x".repeat(1 << 20) };
    const deep = Array.from({ length: 40 }, () => []).reduce<unknown>((value) => [value], { value: "ok" });

    await expect(client.invoke("example/module", "example.action.read", oversized))
      .rejects.toMatchObject({ code: "ACTION_INPUT_BOUNDED" });
    await expect(client.invoke("example/module", "example.action.read", deep))
      .rejects.toMatchObject({ code: "ACTION_INPUT_BOUNDED" });
    expect(rpc.calls).toHaveLength(0);
  });

  it("rejects non-serializable input without adding a browser authority field", async () => {
    const rpc = rpcStub();
    const client = createModuleActionClient(rpc);
    const cyclic: { self?: unknown } = {};
    cyclic.self = cyclic;

    await expect(client.invoke("example/module", "example.action.read", cyclic))
      .rejects.toMatchObject({ code: "ACTION_INPUT_BOUNDED" });
    expect(rpc.calls).toHaveLength(0);
  });

  it("returns a typed transport error without rewriting it into local authority", async () => {
    const error = new ModuleActionClientError(-32004, "action not found");
    const rpc: ModuleActionRPC = {
      call: async <T>() => { throw error as T; },
    };
    const client = createModuleActionClient(rpc);

    await expect(client.invoke("example/module", "example.action.missing", { value: "ok" }))
      .rejects.toBe(error);
  });
});
