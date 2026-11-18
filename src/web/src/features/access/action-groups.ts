import type { Action } from '@/features/access/api';
import {
  cpcDashboardNavigation,
  cpcDataTabs,
  getWorkspaceNavigation,
  projectTabs,
  studentDashboardNavigation,
  workspaceSections,
} from '@/features/workspace/navigation';

export interface ActionGroup {
  value: string;
  label: string;
  actions: Action[];
  children?: ActionGroup[];
}

export function getActionGroups(actions: Action[]): ActionGroup[] {
  // 按目录归类，空间通用权限和未匹配导航的类别保持独立展示。
  const groups = new Map<string, ActionGroup>();
  for (const action of actions) {
    const group = groups.get(action.group.value);
    if (group) {
      group.actions.push(action);
    } else {
      groups.set(action.group.value, { ...action.group, actions: [action] });
    }
  }

  // 使用完整目录匹配导航顺序，不根据被编辑用户的已选权限隐藏类别。
  const available = actions.map(({ value }) => value);
  // 权限目录按资源归类，即使目录片段缺少列表操作也不将子页散落到外层。
  for (const tab of cpcDataTabs) {
    if (groups.has(tab.group)) available.push(tab.listAction);
  }
  for (const dashboard of [studentDashboardNavigation, cpcDashboardNavigation]) {
    if (groups.has(dashboard.group)) available.push(...dashboard.listActions);
  }
  const sections = workspaceSections.map((section) => {
    const children = getWorkspaceNavigation(available, section.value).flatMap((item) => {
      // 数据管理按各子页的列表权限归类，保持导航、子页及授权层级一致。
      if (item.path === 'data-management') {
        const tabs = cpcDataTabs.flatMap((tab) => {
          const group = groups.get(tab.group);
          if (!group) return [];
          groups.delete(group.value);
          return [{ ...group, label: tab.label }];
        });
        return [
          {
            value: 'cpc_data',
            label: item.label,
            actions: tabs.flatMap((tab) => tab.actions),
            children: tabs,
          },
        ];
      }
      const action = actions.find(({ value }) => item.listActions.includes(value));
      const dashboard =
        section.value === 'student' ? studentDashboardNavigation : cpcDashboardNavigation;
      const group =
        item.path === dashboard.path
          ? groups.get(dashboard.group)
          : action
            ? groups.get(action.group.value)
            : undefined;
      if (!group) return [];
      groups.delete(group.value);

      if (item.path !== 'projects') return [{ ...group, label: item.label }];

      // 评奖评优沿用页面的子页顺序，项目附件归入奖项管理。
      const tabs = projectTabs.map((tab) => ({
        value: tab.value,
        label: tab.label,
        actions: group.actions.filter((entry) => {
          const owner = projectTabs
            .toSorted((left, right) => right.listAction.length - left.listAction.length)
            .find(({ listAction }) =>
              entry.value.startsWith(listAction.slice(0, listAction.lastIndexOf('.') + 1)),
            );
          return (owner?.value ?? 'projects') === tab.value;
        }),
      }));
      return [
        {
          ...group,
          label: item.label,
          children: tabs.filter((tab) => tab.actions.length > 0),
        },
      ];
    });
    return {
      value: section.value,
      label: section.label,
      actions: children.flatMap((group) => group.actions),
      children,
    };
  });

  return [...groups.values(), ...sections.filter((section) => section.actions.length > 0)];
}
