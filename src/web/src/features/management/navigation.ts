import { ClipboardList, LayoutDashboard, ShieldCheck, UsersRound } from 'lucide-react';

export const systemNavigation = [
  { id: 'system-users', to: '/system/users', label: '用户管理', icon: UsersRound },
  {
    id: 'system-workspaces',
    to: '/system/workspaces',
    label: '工作空间',
    icon: LayoutDashboard,
  },
  { id: 'system-permissions', to: '/system/permissions', label: '权限管理', icon: ShieldCheck },
  { id: 'system-audit', to: '/system/audit', label: '审计日志', icon: ClipboardList },
];
