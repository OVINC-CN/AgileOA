import { createBrowserRouter, Navigate, type RouteObject } from 'react-router';

import { AppLayout } from '@/components/app-layout';
import { SystemLayout } from '@/components/system-layout';
import { WatermarkLayout } from '@/components/watermark-layout';
import { AuthGuard } from '@/features/user/auth-guard';
import {
  changePasswordPath,
  emailLoginPath,
  studentLoginPath,
  studentServicePath,
} from '@/features/user/navigation';
import { cpcRootPath, studentRootPath } from '@/features/workspace/navigation';
import { ChangePasswordPage } from '@/pages/change-password-page';
import { EmailLoginPage } from '@/pages/email-login-page';
import { ErrorPage } from '@/pages/error-page';
import { LoginPage, StudentLoginPage } from '@/pages/login-page';
import { NotFoundPage } from '@/pages/not-found-page';
import { OverviewPage } from '@/pages/overview-page';

const routes: RouteObject[] = [
  { path: '/login', Component: LoginPage, ErrorBoundary: ErrorPage },
  { path: studentLoginPath, Component: StudentLoginPage, ErrorBoundary: ErrorPage },
  { path: emailLoginPath, Component: EmailLoginPage, ErrorBoundary: ErrorPage },
  {
    element: <AuthGuard passwordChange />,
    ErrorBoundary: ErrorPage,
    children: [{ path: changePasswordPath, Component: ChangePasswordPage }],
  },
  {
    path: '/',
    element: <AuthGuard allowGuest />,
    ErrorBoundary: ErrorPage,
    children: [{ Component: AppLayout, children: [{ index: true, Component: OverviewPage }] }],
  },
  {
    Component: AuthGuard,
    ErrorBoundary: ErrorPage,
    children: [
      {
        Component: AppLayout,
        children: [
          {
            path: studentServicePath,
            lazy: async () => ({
              Component: (await import('@/pages/student-service-page')).StudentServicePage,
            }),
            children: [
              {
                path: 'projects',
                lazy: async () => ({
                  Component: (await import('@/pages/student-awards-page')).StudentAwardsPage,
                }),
              },
              {
                path: 'profile',
                lazy: async () => ({
                  Component: (await import('@/pages/student-profile-page')).StudentProfilePage,
                }),
              },
            ],
          },
          {
            path: studentRootPath,
            lazy: async () => ({
              Component: (await import('@/pages/workspace-page')).WorkspacePage,
            }),
            children: [
              {
                index: true,
                lazy: async () => ({
                  Component: (await import('@/pages/workspace-index-page')).WorkspaceIndexPage,
                }),
              },
              {
                path: 'roster',
                lazy: async () => ({
                  Component: (await import('@/pages/workspace-students-page'))
                    .WorkspaceStudentsPage,
                }),
              },
              {
                path: 'projects',
                lazy: async () => ({
                  Component: (await import('@/pages/workspace-projects-page'))
                    .WorkspaceProjectsPage,
                }),
              },
              {
                path: 'monthly-work',
                lazy: async () => ({
                  Component: (await import('@/pages/workspace-monthly-work-page'))
                    .WorkspaceMonthlyWorkPage,
                }),
              },
              {
                path: 'graduation',
                lazy: async () => ({
                  Component: (await import('@/pages/workspace-graduation-page'))
                    .WorkspaceGraduationPage,
                }),
              },
              {
                path: 'dashboard',
                lazy: async () => ({
                  Component: (await import('@/pages/workspace-student-dashboard-page'))
                    .WorkspaceStudentDashboardPage,
                }),
              },
            ],
          },
          {
            path: cpcRootPath,
            lazy: async () => ({
              Component: (await import('@/pages/workspace-page')).WorkspacePage,
            }),
            children: [
              {
                index: true,
                lazy: async () => ({
                  Component: (await import('@/pages/workspace-index-page')).WorkspaceIndexPage,
                }),
              },
              {
                path: 'roster',
                lazy: async () => ({
                  Component: (await import('@/pages/workspace-cpc-people-page'))
                    .WorkspaceCPCPeoplePage,
                }),
              },
              {
                path: 'dashboard',
                lazy: async () => ({
                  Component: (await import('@/pages/workspace-cpc-dashboard-page'))
                    .WorkspaceCPCDashboardPage,
                }),
              },
              {
                path: 'data-management',
                lazy: async () => ({
                  Component: (await import('@/pages/workspace-cpc-data-page')).WorkspaceCPCDataPage,
                }),
              },
              {
                path: 'monthly-work',
                lazy: async () => ({
                  Component: (await import('@/pages/workspace-monthly-work-page'))
                    .WorkspaceMonthlyWorkPage,
                }),
              },
            ],
          },
          {
            path: 'system',
            Component: SystemLayout,
            children: [
              { index: true, element: <Navigate to="users" replace /> },
              {
                path: 'users',
                lazy: async () => ({ Component: (await import('@/pages/users-page')).UsersPage }),
              },
              {
                path: 'workspaces',
                lazy: async () => ({
                  Component: (await import('@/pages/workspaces-page')).WorkspacesPage,
                }),
              },
              {
                path: 'permissions',
                lazy: async () => ({
                  Component: (await import('@/pages/permissions-page')).PermissionsPage,
                }),
              },
              {
                path: 'audit',
                lazy: async () => ({ Component: (await import('@/pages/audit-page')).AuditPage }),
              },
            ],
          },
          { path: '*', Component: NotFoundPage },
        ],
      },
    ],
  },
];

export const router = createBrowserRouter([{ Component: WatermarkLayout, children: routes }]);
