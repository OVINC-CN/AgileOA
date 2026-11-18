import { Award, UserRound } from 'lucide-react';
import type { Location } from 'react-router';

import {
  studentAwardsPath,
  studentAwardsURL,
  studentProfilePath,
  studentServiceURL,
} from '@/features/user/navigation';
import { getTabMenuItems } from '@/features/workspace/navigation';

export const studentAwardsTabs = [
  { value: 'mine', label: '我的申报' },
  { value: 'reviews', label: '我的审核' },
] as const;

export function getStudentServiceNavigation(
  workspaceID: string,
  location: Pick<Location, 'pathname' | 'search' | 'hash'>,
) {
  return [
    {
      id: 'student-profile',
      to: studentServiceURL(workspaceID),
      label: '个人信息',
      icon: UserRound,
      isActive: location.pathname === studentProfilePath,
    },
    {
      id: 'student-awards',
      to: studentAwardsURL(workspaceID),
      label: '评奖评优',
      icon: Award,
      isActive: location.pathname === studentAwardsPath,
      children: getTabMenuItems(
        'student-awards',
        studentAwardsPath,
        workspaceID,
        studentAwardsTabs,
        location,
      ),
    },
  ];
}
