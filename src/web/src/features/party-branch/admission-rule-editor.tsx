import { FolderPlus, Plus, Trash2 } from 'lucide-react';

import { InRuleEditor } from '@/components/in-rule-editor';
import { Button } from '@/components/ui/button';
import {
  type AdmissionRuleNode,
  newRuleCondition,
  newRuleGroup,
} from '@/features/party-branch/admission-rule';
import { AdmissionRulePreview } from '@/features/party-branch/admission-rule-preview';
import type { AdmissionRuleInfo } from '@/features/party-branch/api';
import { getConfig } from '@/lib/config';

export function AdmissionRuleFields({
  rule,
  catalog,
  onChange,
  disabled = false,
}: {
  rule: AdmissionRuleNode | null;
  catalog: AdmissionRuleInfo;
  onChange: (rule: AdmissionRuleNode | null) => void;
  disabled?: boolean;
}) {
  const config = getConfig();
  return (
    <section aria-label="准入规则" className="min-w-0 space-y-4 border-t border-border pt-5">
      <h2 className="text-sm font-medium">准入规则</h2>
      <p className="text-xs leading-5 text-muted-foreground">
        条件组可选择全部满足（AND）或任一满足（OR），支持嵌套。条件按完整文本匹配候选值。
      </p>
      {rule ? (
        <InRuleEditor
          rule={rule}
          fields={catalog.fields}
          limits={{
            maxDepth: config.partyBranchRuleMaxDepth,
            maxNodes: config.partyBranchRuleMaxNodes,
            maxValues: config.partyBranchRuleMaxValues,
          }}
          readOnly={disabled}
          onChange={onChange}
        />
      ) : (
        <div className="space-y-3 rounded-lg border border-dashed p-4">
          <p className="text-sm text-muted-foreground">未配置准入规则</p>
          {!disabled ? (
            <div className="flex flex-wrap gap-2">
              <Button
                type="button"
                variant="outline"
                onClick={() => {
                  onChange(newRuleCondition());
                }}
              >
                <Plus aria-hidden="true" />
                添加条件
              </Button>
              <Button
                type="button"
                variant="outline"
                disabled={config.partyBranchRuleMaxDepth < 2 || config.partyBranchRuleMaxNodes < 2}
                onClick={() => {
                  onChange(newRuleGroup());
                }}
              >
                <FolderPlus aria-hidden="true" />
                添加条件组
              </Button>
            </div>
          ) : null}
        </div>
      )}
      <AdmissionRulePreview rule={rule} />
      {!disabled && rule ? (
        <Button
          type="button"
          variant="ghost"
          onClick={() => {
            onChange(null);
          }}
        >
          <Trash2 aria-hidden="true" />
          清空规则
        </Button>
      ) : null}
    </section>
  );
}
