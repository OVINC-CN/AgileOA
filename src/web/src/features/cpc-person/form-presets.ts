import type { TextFieldOption } from '@/lib/text-field-options';

export const personStatusPresets: readonly TextFieldOption[] = [
  { value: 'staffed', label: '在编' },
  { value: 'transferred_out', label: '转出' },
];

export const partyPositionPresets: readonly TextFieldOption[] = [
  { value: 'party_branch_secretary', label: '党支部书记' },
  { value: 'organization_member', label: '组织委员' },
  { value: 'discipline_member', label: '纪检委员' },
  { value: 'publicity_member', label: '宣传委员' },
];
