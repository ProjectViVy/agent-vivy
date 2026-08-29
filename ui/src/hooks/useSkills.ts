import { useState, useCallback, useEffect } from 'react';
import { getSkill, listSkills, type SkillSummary, type SkillView } from '@/lib/api';
import { t } from '@/i18n';

export function useSkills() {
  const [skills, setSkills] = useState<SkillSummary[]>([]);
  const [selected, setSelected] = useState<SkillView | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadSkills = useCallback(async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await listSkills();
      setSkills(data.skills);
      setSelected((current) => {
        if (!current) return current;
        return data.skills.some((skill) => skill.name === current.name) ? current : null;
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : t('skills.errors.loadSkillsFailed'));
    } finally {
      setIsLoading(false);
    }
  }, []);

  const loadSkillDocument = useCallback(async (name: string, path?: string) => {
    try {
      setError(null);
      const doc = await getSkill(name, path);
      setSelected(doc);
    } catch (err) {
      setError(err instanceof Error ? err.message : t('skills.errors.loadDocumentFailed'));
    }
  }, []);

  useEffect(() => {
    void loadSkills();
  }, [loadSkills]);

  return {
    skills,
    selected,
    isLoading,
    error,
    loadSkills,
    loadSkillDocument,
    clearSelected: () => setSelected(null),
  };
}
