import { X } from 'lucide-react';
import { useRef, useState } from 'react';

import { Button } from '@/components/ui/button';
import { Drawer, type DrawerHandle } from '@/components/ui/drawer';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { rejectTimelineTransfer } from '@/features/cpc-person/timeline-transfer-api';
import { useApiAction } from '@/lib/api-action';

const reasonMaxLength = 1000;

export function TimelineTransferRejectDrawer({
  workspaceID,
  personID,
  personLabel,
  revision,
  invalidate,
  onClose,
}: {
  workspaceID: string;
  personID: number;
  personLabel: string | undefined;
  revision: number;
  invalidate: () => Promise<void>;
  onClose: () => void;
}) {
  const [reason, setReason] = useState('');
  const drawer = useRef<DrawerHandle>(null);
  const reject = useApiAction();

  return (
    <Drawer
      ref={drawer}
      title="驳回时间线转移"
      description={personLabel}
      truncateDescription
      busy={reject.pending}
      onClose={onClose}
      form={{
        noValidate: true,
        onSubmit: (event) => {
          event.preventDefault();
          if (reject.pending) return;
          void reject.run('reject', async () => {
            // 去除首尾空白后校验，避免只填写空格时提交驳回。
            const value = reason.trim();
            if (!value) throw new Error('请填写审批意见');
            if (Array.from(value).length > reasonMaxLength) {
              throw new Error('审批意见不能超过 1000 个字符');
            }
            await rejectTimelineTransfer(workspaceID, personID, revision, value);
            await invalidate();
            drawer.current?.close();
          });
        },
      }}
      dismissLabel="取消"
      footer={
        <Button type="submit" loading={reject.pending}>
          <X aria-hidden="true" />
          确认驳回
        </Button>
      }
    >
      <div className="space-y-2 p-6">
        <Label htmlFor="timeline-transfer-reject-reason">审批意见 *</Label>
        <Textarea
          id="timeline-transfer-reject-reason"
          value={reason}
          required
          maxLength={reasonMaxLength}
          disabled={reject.pending}
          placeholder="请填写审批意见"
          onChange={(event) => {
            setReason(event.target.value);
          }}
        />
        <p className="text-xs text-muted-foreground">审批意见将随驳回结果通知发送给提交人。</p>
      </div>
    </Drawer>
  );
}
