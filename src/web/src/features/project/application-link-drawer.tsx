import { useQuery } from '@tanstack/react-query';

import { ErrorMessage } from '@/components/error-message';
import { Button } from '@/components/ui/button';
import { Drawer } from '@/components/ui/drawer';
import {
  applicationDetailOptions,
  type ApplicationScope,
} from '@/features/project/application-api';
import { ApplicationDrawer } from '@/features/project/application-drawer';

export function ApplicationLinkDrawer({
  workspaceID,
  scope,
  applicationID,
  onClose,
}: {
  workspaceID: string;
  scope: ApplicationScope;
  applicationID: string;
  onClose: () => void;
}) {
  const id = Number(applicationID);
  const valid = /^[1-9]\d*$/u.test(applicationID) && Number.isSafeInteger(id);
  const query = useQuery({
    ...applicationDetailOptions(workspaceID, scope, id),
    enabled: valid,
  });
  if (valid && query.data && !query.error) {
    return (
      <ApplicationDrawer
        workspaceID={workspaceID}
        scope={scope}
        record={query.data}
        onClose={onClose}
      />
    );
  }
  return (
    <Drawer
      title="申报详情"
      description="查看申报内容及审批流程"
      onClose={onClose}
      footer={
        <Button variant="cancel" onClick={onClose}>
          关闭
        </Button>
      }
    >
      {!valid || query.error ? (
        <ErrorMessage>{valid ? query.error?.message : '申报地址无效'}</ErrorMessage>
      ) : (
        <p role="status" className="px-6 py-12 text-center text-sm text-muted-foreground">
          正在读取申报详情
        </p>
      )}
    </Drawer>
  );
}
