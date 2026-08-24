/**
 * 技能管理 Hook
 */

import { useState, useCallback, useEffect } from 'react';
import type { SkillDto, SkillDocument, SkillRequest, CreateSkillRequestPayload } from '@/lib/types';
import { listSkills, getSkillDocument, createSkillRequest, getSkillRequests } from '@/lib/demo-api';

export function useSkills() {
  const [skills, setSkills] = useState<SkillDto[]>([]);
  const [selectedSkill, setSelectedSkill] = useState<SkillDocument | null>(null);
  const [requests, setRequests] = useState<SkillRequest[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // 加载技能列表
  const loadSkills = useCallback(async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await listSkills();
      setSkills(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载技能失败');
    } finally {
      setIsLoading(false);
    }
  }, []);

  // 加载技能文档
  const loadSkillDocument = useCallback(async (slug: string) => {
    try {
      setError(null);
      const doc = await getSkillDocument(slug);
      setSelectedSkill(doc);
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载技能文档失败');
    }
  }, []);

  // 创建技能请求
  const handleCreateRequest = useCallback(async (payload: CreateSkillRequestPayload) => {
    try {
      setError(null);
      const request = await createSkillRequest(payload);
      setRequests((prev) => [...prev, request]);
      return request;
    } catch (err) {
      setError(err instanceof Error ? err.message : '创建请求失败');
      throw err;
    }
  }, []);

  // 加载技能请求列表
  const loadRequests = useCallback(async () => {
    try {
      setError(null);
      const data = await getSkillRequests();
      setRequests(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载请求失败');
    }
  }, []);

  // 初始加载
  useEffect(() => {
    loadSkills();
    loadRequests();
  }, [loadSkills, loadRequests]);

  return {
    skills,
    selectedSkill,
    requests,
    isLoading,
    error,
    loadSkills,
    loadSkillDocument,
    clearSelectedSkill: () => setSelectedSkill(null),
    createRequest: handleCreateRequest,
    loadRequests,
  };
}
