/*
 * tests/ffi/smoke.c — ABI v1 smoke driver for the sealed DIVA shared library.
 *
 * Links the cgo-generated header (vivy.h emitted by -buildmode=c-shared) and
 * the hand-maintained ABI constants header (vivy_abi.h), then drives:
 *   bad init → init → initialize → session → turn → approval → cancel →
 *   poll → shutdown → free, plus duplicate-init / stale-handle / oversize
 *   input error envelopes.
 *
 * Usage: ./smoke <config.yaml>
 * Env: DEEPSEEK_API_KEY, VIVY_API_BASE (scripted provider), VIVY_PROVIDER.
 */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#include "vivy-shared.h"
#include "vivy_abi.h"

static int failures = 0;

#define CHECK(cond, msg) do { \
    if (!(cond)) { fprintf(stderr, "FAIL: %s\n", msg); failures++; } \
    else { fprintf(stdout, "ok: %s\n", msg); } \
} while (0)

/* Take an envelope return, copy it, then free — mirrors the Rust bridge. */
static char *take(char *p) {
    if (p == NULL) return NULL;
    char *copy = strdup(p);
    VivyFree(p);
    return copy;
}

/* Helpers never free env — the caller owns it. */
static int env_ok(char *env, const char *ctx) {
    if (env == NULL) { fprintf(stderr, "FAIL: %s returned NULL\n", ctx); failures++; return 0; }
    if (strstr(env, "\"ok\":true") == NULL) {
        fprintf(stderr, "FAIL: %s: %s\n", ctx, env);
        failures++;
        return 0;
    }
    return 1;
}

static int env_kind(char *env, const char *kind, const char *ctx) {
    if (env == NULL) { fprintf(stderr, "FAIL: %s returned NULL\n", ctx); failures++; return 0; }
    char needle[128];
    snprintf(needle, sizeof needle, "\"kind\":\"%s\"", kind);
    if (strstr(env, needle) == NULL) {
        fprintf(stderr, "FAIL: %s expected kind %s: %s\n", ctx, kind, env);
        failures++;
        return 0;
    }
    fprintf(stdout, "ok: %s -> %s\n", ctx, kind);
    return 1;
}

/* Extract "key":<number> from a JSON envelope value. */
static unsigned long long env_u64(char *env, const char *key) {
    char needle[64];
    snprintf(needle, sizeof needle, "\"%s\":", key);
    char *p = strstr(env, needle);
    if (!p) return 0;
    return strtoull(p + strlen(needle), NULL, 10);
}

/* Extract "key":"value" (no escapes) from a JSON envelope value. */
static int env_str(char *env, const char *key, char *out, size_t n) {
    char needle[64];
    snprintf(needle, sizeof needle, "\"%s\":\"", key);
    char *p = strstr(env, needle);
    if (!p) return 0;
    p += strlen(needle);
    char *e = strchr(p, '"');
    if (!e || (size_t)(e - p) >= n) return 0;
    memcpy(out, p, (size_t)(e - p));
    out[e - p] = 0;
    return 1;
}

/* One VivyCall that must succeed; returns the envelope (caller frees). */
static char *call_ok(unsigned long long handle, const char *method, const char *params) {
    char buf[8192];
    snprintf(buf, sizeof buf, "{\"method\":\"%s\",\"params\":%s}", method, params);
    char *env = take(VivyCall(handle, buf));
    char ctx[128];
    snprintf(ctx, sizeof ctx, "VivyCall %s", method);
    if (!env_ok(env, ctx)) { free(env); return NULL; }
    return env;
}

/* Wait for a run to reach a terminal/needed state by polling notifications. */
static int poll_for(unsigned long long handle, const char *needle, int tries) {
    for (int i = 0; i < tries; i++) {
        char *env = take(VivyPollEvents(handle, 0));
        if (env && strstr(env, "\"ok\":true")) {
            if (strstr(env, needle)) { free(env); return 1; }
        }
        free(env);
        usleep(50 * 1000);
    }
    return 0;
}

int main(int argc, char **argv) {
    if (argc < 2) { fprintf(stderr, "usage: %s <config.yaml>\n", argv[0]); return 2; }

    /* ABI version check: header constant is what Rust asserts against. */
    if (VIVY_ABI_VERSION != 1) { fprintf(stderr, "FAIL: VIVY_ABI_VERSION %d\n", VIVY_ABI_VERSION); return 2; }

    /* incompatible_abi */
    char *env = take(VivyInit("{\"config_path\":\"x\",\"abi_version\":999}"));
    env_kind(env, "incompatible_abi", "init wrong abi_version");
    free(env);

    /* invalid_input: missing config_path */
    env = take(VivyInit("{\"abi_version\":1}"));
    env_kind(env, "invalid_input", "init missing config_path");
    free(env);

    /* invalid_input: oversize frame (4 MiB + slack) */
    {
        size_t n = (4 << 20) + 1024;
        char *big = malloc(n);
        memset(big, ' ', n - 64);
        memcpy(big, "{\"config_path\":\"", 16);
        memcpy(big + n - 64, "\",\"abi_version\":1}", 18);
        big[n - 64 + 18] = 0;
        env = take(VivyInit(big));
        free(big);
        env_kind(env, "invalid_input", "init oversize frame");
        free(env);
    }

    /* init */
    char init[1024];
    snprintf(init, sizeof init,
             "{\"config_path\":\"%s\",\"abi_version\":%d,\"without_ears\":true}", argv[1], VIVY_ABI_VERSION);
    env = take(VivyInit(init));
    if (!env_ok(env, "VivyInit")) return 2;
    unsigned long long handle = env_u64(env, "handle");
    CHECK(handle != 0, "init returned handle");
    CHECK(env_u64(env, "abi_version") == VIVY_ABI_VERSION, "init returned abi_version");
    free(env);

    /* already_initialized on second init */
    env = take(VivyInit(init));
    env_kind(env, "already_initialized", "second VivyInit");
    free(env);

    /* closed/stale handle */
    env = take(VivyCall(424242, "{\"method\":\"initialize\"}"));
    env_kind(env, "closed", "VivyCall stale handle");
    free(env);

    /* initialize */
    env = call_ok(handle, "initialize", "{}");
    CHECK(env && strstr(env, "vivy.rpc.v1"), "initialize protocol_version vivy.rpc.v1");
    free(env);

    /* session/create */
    env = call_ok(handle, "session/create", "{\"title\":\"ffi-smoke\"}");
    char session_id[128] = {0};
    CHECK(env && env_str(env, "id", session_id, sizeof session_id), "session/create id");
    free(env);

    /* turn/start -> approval_required (write_note held by policy) */
    char params[1024];
    snprintf(params, sizeof params, "{\"session_id\":\"%s\",\"text\":\"approve me\"}", session_id);
    env = call_ok(handle, "turn/start", params);
    char run_id[128] = {0};
    CHECK(env && env_str(env, "run_id", run_id, sizeof run_id), "turn/start run_id");
    free(env);

    snprintf(params, sizeof params, "{\"run_id\":\"%s\"}", run_id);
    env = call_ok(handle, "run/subscribe", params);
    free(env);

    CHECK(poll_for(handle, "tool.approval_required", 400), "poll saw tool.approval_required");

    env = call_ok(handle, "approval/list", "{}");
    char approval_id[128] = {0};
    CHECK(env && env_str(env, "id", approval_id, sizeof approval_id), "approval/list id");
    free(env);

    snprintf(params, sizeof params,
             "{\"review_id\":\"%s\",\"action\":\"approve\"}", approval_id);
    env = call_ok(handle, "review/respond", params);
    free(env);

    CHECK(poll_for(handle, "run.completed", 400), "poll saw run.completed after approval");

    /* second turn, cancel while the scripted provider is still answering */
    snprintf(params, sizeof params, "{\"session_id\":\"%s\",\"text\":\"cancel me\"}", session_id);
    env = call_ok(handle, "turn/start", params);
    char run2[128] = {0};
    env_str(env, "run_id", run2, sizeof run2);
    free(env);
    char req[512];
    snprintf(req, sizeof req, "{\"method\":\"run/cancel\",\"params\":{\"run_id\":\"%s\"}}", run2);
    env = take(VivyCall(handle, req));
    CHECK(env && strstr(env, "\"ok\":true"), "run/cancel accepted on live run");
    free(env);

    /* cancelled run is not active: second cancel -> -32004 invalid_input */
    sleep(1);
    env = take(VivyCall(handle, req));
    CHECK(env && strstr(env, "\"ok\":false") && strstr(env, "-32004"),
          "run/cancel on dead run -> -32004");
    free(env);

    /* shutdown is idempotent */
    env = take(VivyShutdown(handle));
    CHECK(env && strstr(env, "\"ok\":true"), "VivyShutdown ok");
    free(env);
    env = take(VivyShutdown(handle));
    env_kind(env, "closed", "second VivyShutdown");
    free(env);

    /* post-shutdown call -> closed */
    env = take(VivyCall(handle, "{\"method\":\"initialize\"}"));
    env_kind(env, "closed", "VivyCall after shutdown");
    free(env);

    if (failures) { fprintf(stderr, "smoke: %d failures\n", failures); return 1; }
    fprintf(stdout, "smoke: all checks passed\n");
    return 0;
}
