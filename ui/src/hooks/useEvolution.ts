/**
 * 进化页面 Hook —— Skill 权威、待审请求与 AutoDream 运行的本地演示数据编排
 */

import { useState, useCallback, useEffect } from 'react';
import type {
  SkillDto,
  SkillDocument,
  SkillRequest,
  SkillHistoryEntry,
  SkillHistoryDocument,
  CreateSkillRequestPayload,
  AutoDreamRunRecord,
  AutoDreamRunEvent,
} from '@/lib/types';
import {
  listSkills,
  getSkillDocument,
  updateSkillDocument,
  setSkillEnabled,
  deleteSkill,
  getSkillHistory,
  getSkillHistoryDocument,
  getSkillRequests,
  createSkillRequest,
  acceptSkillRequest,
  rejectSkillRequest,
  listAutoDreamRuns,
  getAutoDreamRunEvents,
} from '@/lib/demo-api';
import { t } from '@/i18n';

export function useEvolution() {
  const [skills, setSkills] = useState<SkillDto[]>([]);
  const [requests, setRequests] = useState<SkillRequest[]>([]);
  const [runs, setRuns] = useState<AutoDreamRunRecord[]>([]);
  const [selectedSkill, setSelectedSkill] = useState<SkillDocument | null>(null);
  const [history, setHistory] = useState<SkillHistoryEntry[]>([]);
  const [historyPreview, setHistoryPreview] = useState<SkillHistoryDocument | null>(null);
  const [selectedRequestId, setSelectedRequestId] = useState<string | null>(null);
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);
  const [runEvents, setRunEvents] = useState<AutoDreamRunEvent[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busyKeys, setBusyKeys] = useState<string[]>([]);

  const beginBusy = (key: string) => setBusyKeys((prev) => (prev.includes(key) ? prev : [...prev, key]));
  const endBusy = (key: string) => setBusyKeys((prev) => prev.filter((k) => k !== key));
  const isBusy = (key: string) => busyKeys.includes(key);

  const loadAll = useCallback(async () => {
    try {
      setIsLoading(true);
      setError(null);
      const [skillList, requestList, runList] = await Promise.all([listSkills(), getSkillRequests(), listAutoDreamRuns()]);
      setSkills(skillList);
      setRequests(requestList);
      setRuns(runList);
    } catch (err) {
      setError(err instanceof Error ? err.message : t('evolution.errors.loadFailed'));
    } finally {
      setIsLoading(false);
    }
  }, []);

  const loadSkillsAndRequests = useCallback(async () => {
    try {
      const [skillList, requestList] = await Promise.all([listSkills(), getSkillRequests()]);
      setSkills(skillList);
      setRequests(requestList);
    } catch (err) {
      setError(err instanceof Error ? err.message : t('evolution.errors.loadFailed'));
    }
  }, []);

  const selectSkill = useCallback(async (slug: string) => {
    try {
      setError(null);
      setHistoryPreview(null);
      const [doc, historyList] = await Promise.all([getSkillDocument(slug), getSkillHistory(slug)]);
      setSelectedSkill(doc);
      setHistory(historyList);
    } catch (err) {
      setError(err instanceof Error ? err.message : t('evolution.errors.loadFailed'));
    }
  }, []);

  const clearSelectedSkill = useCallback(() => {
    setSelectedSkill(null);
    setHistory([]);
    setHistoryPreview(null);
  }, []);

  const saveSkill = useCallback(async (slug: string, markdown: string, baseHash: string) => {
    const key = `save:${slug}`;
    beginBusy(key);
    try {
      setError(null);
      const outcome = await updateSkillDocument(slug, markdown, baseHash);
      setSelectedSkill(outcome.document);
      setHistory(await getSkillHistory(slug));
      await loadSkillsAndRequests();
      return true;
    } catch (err) {
      setError(err instanceof Error ? err.message : t('evolution.errors.saveFailed'));
      return false;
    } finally {
      endBusy(key);
    }
  }, [loadSkillsAndRequests]);

  const toggleSkillEnabled = useCallback(async (slug: string, enabled: boolean) => {
    const key = `toggle:${slug}`;
    beginBusy(key);
    try {
      setError(null);
      await setSkillEnabled(slug, enabled);
      await loadSkillsAndRequests();
      setSelectedSkill((prev) => (prev && prev.slug === slug ? { ...prev, enabled } : prev));
    } catch (err) {
      setError(err instanceof Error ? err.message : t('evolution.errors.saveFailed'));
    } finally {
      endBusy(key);
    }
  }, [loadSkillsAndRequests]);

  const removeSkill = useCallback(async (slug: string) => {
    const key = `delete:${slug}`;
    beginBusy(key);
    try {
      setError(null);
      await deleteSkill(slug);
      if (selectedSkill?.slug === slug) {
        setSelectedSkill(null);
        setHistory([]);
        setHistoryPreview(null);
      }
      await loadSkillsAndRequests();
    } catch (err) {
      setError(err instanceof Error ? err.message : t('evolution.errors.saveFailed'));
    } finally {
      endBusy(key);
    }
  }, [loadSkillsAndRequests, selectedSkill]);

  const previewHistory = useCallback(async (slug: string, revision: number) => {
    try {
      setError(null);
      const doc = await getSkillHistoryDocument(slug, revision);
      setHistoryPreview(doc);
    } catch (err) {
      setError(err instanceof Error ? err.message : t('evolution.errors.loadFailed'));
    }
  }, []);

  const createRequest = useCallback(async (payload: CreateSkillRequestPayload) => {
    const key = 'create-request';
    beginBusy(key);
    try {
      setError(null);
      const created = await createSkillRequest(payload);
      await loadSkillsAndRequests();
      return created;
    } catch (err) {
      setError(err instanceof Error ? err.message : t('evolution.errors.createRequestFailed'));
      return null;
    } finally {
      endBusy(key);
    }
  }, [loadSkillsAndRequests]);

  const acceptRequest = useCallback(async (id: string) => {
    const key = `accept:${id}`;
    beginBusy(key);
    try {
      setError(null);
      await acceptSkillRequest(id);
      await loadSkillsAndRequests();
      if (selectedSkill) {
        const doc = await getSkillDocument(selectedSkill.slug);
        setSelectedSkill(doc);
      }
      return true;
    } catch (err) {
      setError(err instanceof Error ? err.message : t('evolution.errors.acceptFailed'));
      // 拒绝（含 stale）也会改变请求状态，刷新列表让用户看到最新状态
      await loadSkillsAndRequests();
      return false;
    } finally {
      endBusy(key);
    }
  }, [loadSkillsAndRequests, selectedSkill]);

  const rejectRequest = useCallback(async (id: string) => {
    const key = `reject:${id}`;
    beginBusy(key);
    try {
      setError(null);
      await rejectSkillRequest(id);
      await loadSkillsAndRequests();
      return true;
    } catch (err) {
      setError(err instanceof Error ? err.message : t('evolution.errors.rejectFailed'));
      return false;
    } finally {
      endBusy(key);
    }
  }, [loadSkillsAndRequests]);

  const selectRun = useCallback(async (runId: string) => {
    try {
      setError(null);
      setSelectedRunId(runId);
      setRunEvents(await getAutoDreamRunEvents(runId));
    } catch (err) {
      setError(err instanceof Error ? err.message : t('evolution.errors.loadFailed'));
    }
  }, []);

  useEffect(() => {
    loadAll();
  }, [loadAll]);

  const evolutionSkills = skills.filter((skill) => skill.evolution_managed === true);
  const pendingCount = requests.filter((request) => request.status === 'pending').length;

  return {
    skills,
    evolutionSkills,
    requests,
    pendingCount,
    runs,
    selectedSkill,
    history,
    historyPreview,
    selectedRequestId,
    selectedRunId,
    runEvents,
    isLoading,
    error,
    isBusy,
    refresh: loadAll,
    selectSkill,
    clearSelectedSkill,
    saveSkill,
    toggleSkillEnabled,
    removeSkill,
    previewHistory,
    createRequest,
    acceptRequest,
    rejectRequest,
    selectRequest: setSelectedRequestId,
    clearSelectedRequest: () => setSelectedRequestId(null),
    selectRun,
    clearSelectedRun: () => {
      setSelectedRunId(null);
      setRunEvents([]);
    },
  };
}
