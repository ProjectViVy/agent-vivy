/**
 * Multi-account token plumbing: one {@link AccountTokenManager} per provider
 * owns a lazily-built {@link TokenManager} per account, so refresh coalescing
 * (`inflight`) and permanent-failure removal stay scoped to ONE account —
 * a revoked account deletes itself without touching its siblings.
 *
 * {@link AccountAwareAdapter} is the internal interface the pool uses to
 * stream through a specific account. A catalog model listed by several
 * accounts failovers; one listed by a single account is pinned to it.
 */
import { LlmAdapter, LlmError } from '@deepseek-ai/dsh-llm';
import { TokenManager, withTimeout } from './common.js';
import { deleteAccountSession, getAccountSession, listAccounts, saveAccountSession, } from '../auth/store.js';
export { DISCOVERY_TIMEOUT_MS } from './common.js';
/** Catalog sort hint when the provider advertised one (Codex `priority`). */
function catalogPriority(model) {
    const ranked = model;
    return typeof ranked.priority === 'number' ? ranked.priority : Number.MAX_SAFE_INTEGER;
}
/**
 * Merge per-account catalogs, keeping the first occurrence of each model id.
 * Rows that carry a numeric `priority` (Codex discovery) are then ordered by
 * it so a model only the second account lists — e.g. `gpt-5.6-sol` — still
 * sits with its generation instead of being appended after the default
 * account's older ids.
 */
export async function unionAccountCatalogs(accounts, listOne, options) {
    const timeoutMs = options?.timeoutMs;
    const caller = options?.signal;
    const catalogs = await Promise.all(accounts.map(async (account) => {
        try {
            if (timeoutMs === undefined)
                return await listOne(account, caller);
            const models = await withTimeout(timeoutSignal => listOne(account, caller === undefined ? timeoutSignal : AbortSignal.any([timeoutSignal, caller])), timeoutMs);
            return models ?? [];
        }
        catch (error) {
            // One expired or failing account must not hide models the others list.
            if (caller?.aborted === true)
                throw error;
            return [];
        }
    }));
    const seen = new Set();
    const models = [];
    for (const catalog of catalogs) {
        for (const model of catalog) {
            if (seen.has(model.id))
                continue;
            seen.add(model.id);
            models.push(model);
        }
    }
    models.sort((left, right) => catalogPriority(left) - catalogPriority(right));
    return models;
}
export class AccountTokenManager {
    options;
    managers = new Map();
    io;
    constructor(options) {
        this.options = options;
        const provider = options.provider;
        this.io = options.io ?? {
            list: () => listAccounts(provider),
            get: account => getAccountSession(provider, account),
            save: (account, session) => saveAccountSession(provider, account, session),
            remove: account => deleteAccountSession(provider, account),
        };
    }
    /** The provider's accounts, default first (straight from the store). */
    list() {
        return this.io.list();
    }
    /** The default account's key, or undefined when logged out. */
    async defaultAccount() {
        return (await this.list())[0]?.key;
    }
    /**
     * Resolve a usable session for one account (default when omitted),
     * refreshing proactively or on demand.
     * @param account - the account key; the default account when undefined.
     * @param forceRefresh - refresh regardless of expiry (used after a 401).
     * @returns the persisted session to send.
     * @throws LlmError MISSING_CREDENTIAL when the account is not logged in.
     */
    async session(account, forceRefresh = false) {
        const key = account ?? await this.defaultAccount();
        if (key === undefined)
            throw this.missingCredential();
        return this.tokensFor(key).session(forceRefresh);
    }
    /** Read an account's stored session without any refresh side effect. */
    peek(account) {
        return this.io.get(account);
    }
    /** Whether a session is stored for the account (cheap; never refreshes). */
    async hasSession(account) {
        return (await this.peek(account)) !== undefined;
    }
    /** The TokenManager bound to one account (created lazily, then cached). */
    tokensFor(account) {
        let manager = this.managers.get(account);
        if (manager === undefined) {
            const io = this.io;
            manager = new TokenManager({
                displayName: this.options.displayName,
                ...this.options.makeOptions(account),
                load: () => io.get(account),
                save: session => io.save(account, session),
                remove: () => io.remove(account),
                onRemoved: () => { this.options.onAccountRemoved?.(account); },
            });
            this.managers.set(account, manager);
        }
        return manager;
    }
    /** The logged-out error, mirroring TokenManager's own message. */
    missingCredential() {
        return new LlmError(`dsh-plugin-subscriptions: not logged in to ${this.options.displayName}; `
            + 'log in via Settings → Subscriptions in the dsh web app', 'MISSING_CREDENTIAL');
    }
}
