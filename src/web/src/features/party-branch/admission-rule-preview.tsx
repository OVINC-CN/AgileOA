import { InRuleTree } from '@/components/in-rule-tree';
import { type AdmissionRuleNode, ruleFieldLabels } from '@/features/party-branch/admission-rule';

export function AdmissionRulePreview({ rule }: { rule: AdmissionRuleNode | null }) {
  return (
    <section aria-label="规则预览" className="space-y-2 text-sm">
      <h2 className="text-sm font-normal text-muted-foreground">规则预览</h2>
      {rule ? (
        <InRuleTree rule={rule} labels={ruleFieldLabels} label="准入规则树" />
      ) : (
        <p>未配置准入规则</p>
      )}
    </section>
  );
}
