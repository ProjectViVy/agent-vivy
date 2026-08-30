import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { useTranslation } from '@/i18n';

/**
 * 通道编辑表单：只编辑 kernel envelope 的真实旋钮。
 * - enabled：启停开关（写入 settings overlay，重启进程后生效）。
 * - allow_from：入站发送者白名单；编辑面是原始文本（每行一个），保存时
 *   才解析；留空 = 拒绝启动（fail-closed）。
 * - token_env：只读展示环境变量名 + 是否已设置；密钥值永不读取、
 *   永不显示、永不保存（D-010）。变量名不是密钥，完整明文展示。
 * 平台凭据 schema（channel-schema.ts）是 C6/C7 的元数据，不在本表单
 * 渲染——后端 envelope 不接收那些字段。
 */

type ChannelEditorFormProps = {
  platform: string;
  enabled: boolean;
  /** 允许的发送者，原始文本（每行一个）。 */
  allowFromText: string;
  /** 环境变量名（envelope 文档真值）；空 = 未声明。 */
  tokenEnv?: string;
  /** 环境变量当前是否非空（inspect 表面）。 */
  tokenEnvSet?: boolean;
  onEnabledChange?: (enabled: boolean) => void;
  onAllowFromTextChange?: (text: string) => void;
};

export function ChannelEditorForm({
  platform,
  enabled,
  allowFromText,
  tokenEnv,
  tokenEnvSet,
  onEnabledChange,
  onAllowFromTextChange,
}: ChannelEditorFormProps) {
  const { t } = useTranslation();
  void platform;

  return (
    <div className="space-y-4">
      {onEnabledChange ? (
        <div className="flex items-center justify-between gap-3 rounded-lg border p-3">
          <span className="text-sm font-medium">{t('channels.enabled')}</span>
          <Switch
            checked={enabled}
            onCheckedChange={onEnabledChange}
            aria-label={enabled ? t('channels.enabled') : t('channels.disabled')}
          />
        </div>
      ) : null}

      {onAllowFromTextChange ? (
        <div className="space-y-2">
          <Label>{t('channels.allowFromLabel')}</Label>
          <Textarea
            rows={3}
            value={allowFromText}
            placeholder={t('channels.allowFromPlaceholder')}
            onChange={(event) => onAllowFromTextChange(event.target.value)}
          />
          <p className="text-xs text-muted-foreground">{t('channels.allowFromHint')}</p>
        </div>
      ) : null}

      <div className="space-y-2">
        <Label>{t('channels.tokenEnvLabel')}</Label>
        <div className="flex items-center gap-2">
          <Input
            readOnly
            value={tokenEnv ?? ''}
            placeholder={t('channels.tokenEnvUnset')}
            className="font-mono text-xs"
            aria-label={t('channels.tokenEnvLabel')}
          />
          <Badge variant={tokenEnv ? (tokenEnvSet ? 'default' : 'secondary') : 'secondary'}>
            {tokenEnv
              ? tokenEnvSet
                ? t('channels.tokenEnvSet')
                : t('channels.tokenEnvMissing')
              : t('channels.tokenEnvUnset')}
          </Badge>
        </div>
        <p className="text-xs text-muted-foreground">{t('channels.tokenEnvNote')}</p>
      </div>
    </div>
  );
}
