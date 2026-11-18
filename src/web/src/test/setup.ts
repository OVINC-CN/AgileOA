import '@testing-library/jest-dom/vitest';

import { cleanup, render } from '@testing-library/react';
import { createElement } from 'react';
import { afterEach, beforeEach } from 'vitest';

import { MessageViewport } from '@/components/message-viewport';
import { clearMessages } from '@/lib/messages';

beforeEach(() => {
  render(createElement(MessageViewport));
});
afterEach(() => {
  cleanup();
  clearMessages();
});

// 测试环境使用与部署配置相同的字段，不依赖构建期环境变量。
window.AGILE_WORKSPACE_CONFIG = {
  displayTimeZone: 'Asia/Shanghai',
  auditDiffTimeoutMs: 2000,
  appName: '敏捷办公',
  contactEmail: 'privacy@example.test',
  copyrightOwner: 'OVINC TECH',
  copyrightYear: 2026,
  passwordLoginEnabled: true,
  apiBaseURL: '',
  apiTimeoutMs: 15000,
  messageDurationMs: 5000,
  pageUpdateCheckIntervalMs: 60000,
  pageUpdateCheckTimeoutMs: 15000,
  queryStaleTimeMs: 30000,
  queryRetryCount: 1,
  notificationPollIntervalMs: 60000,
  studentMaxFileBytes: 5242880,
  fileMaxUploadBytes: 5242880,
  filePreviewTimeoutMs: 60000,
  fileAllowedContentTypes: ['application/pdf', 'image/*'],
  fileContentTypeGroups: {
    PDF: ['application/pdf'],
    图片: ['image/*'],
  },
  projectMaxFileBytes: 5242880,
  projectMaxDetails: 500,
  monthlyWorkMaxLinks: 20,
  monthlyWorkMaxFileBytes: 5242880,
  partyBranchMaxFileBytes: 5242880,
  cpcPersonMaxFileBytes: 5242880,
  partyBranchRuleMaxDepth: 8,
  partyBranchRuleMaxNodes: 100,
  partyBranchRuleMaxValues: 100,
  timelineMaxNodes: 100,
  timelineMaxFieldsPerNode: 20,
  timelineMaxRulesPerNode: 20,
  timelineMaxDefinitionBytes: 131072,
  timelineMaxOffset: 10000,
  tablePageSize: 5,
  tablePageSizeOptions: [5, 10, 20, 50, 100],
};
