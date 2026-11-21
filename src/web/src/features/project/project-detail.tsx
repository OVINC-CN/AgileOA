import { Paperclip } from 'lucide-react';
import { useId, useState } from 'react';

import { AttachmentFilename } from '@/components/attachment-filename';
import { TableTimestamp } from '@/components/table-timestamp';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Drawer } from '@/components/ui/drawer';
import { DrawerTabs } from '@/components/ui/drawer-tabs';
import { attachmentDownload, type Project } from '@/features/project/api';
import { ApplicationFlow } from '@/features/project/application-timeline';
import { ProjectEnabledBadge } from '@/features/project/project-status';
import { ScoreCalculationDetail } from '@/features/project/score-calculation-detail';
import { formatValue } from '@/lib/format';

export function ProjectDetail({
  workspaceID,
  project,
  canReadAttachments,
  onClose,
}: {
  workspaceID: string;
  project: Project;
  canReadAttachments: boolean;
  onClose: () => void;
}) {
  const [tab, setTab] = useState('main');
  const tabsID = useId();
  return (
    <Drawer
      title={project.name}
      description={`归属年份：${String(project.year)}`}
      contentClassName="flex flex-col overflow-hidden"
      onClose={onClose}
      footerNavigation={
        <DrawerTabs
          id={tabsID}
          label="项目内容分类"
          value={tab}
          items={[
            { value: 'main', label: '主要信息' },
            { value: 'application', label: '申报设置' },
            { value: 'scoring', label: '分数计算' },
          ]}
          onChange={setTab}
        />
      }
      footerDismiss={
        <Button variant="cancel" onClick={onClose}>
          关闭
        </Button>
      }
    >
      <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain p-6">
        <section
          role="tabpanel"
          id={`${tabsID}-panel-scoring`}
          aria-labelledby={`${tabsID}-tab-scoring`}
          hidden={tab !== 'scoring'}
        >
          <ScoreCalculationDetail value={project.score_calculation} />
        </section>
        <section
          role="tabpanel"
          id={`${tabsID}-panel-main`}
          aria-labelledby={`${tabsID}-tab-main`}
          hidden={tab !== 'main'}
          className="space-y-5"
        >
          <section className="space-y-3">
            <h2 className="text-sm font-medium">项目说明</h2>
            <p className="text-sm leading-7 wrap-anywhere whitespace-pre-wrap">
              {formatValue(project.description)}
            </p>
          </section>
          <section className="space-y-3 border-t border-border pt-5">
            <h2 className="text-sm font-medium">可选子项名称</h2>
            {project.detail_names.length ? (
              <ul aria-label="可选子项名称" className="flex flex-wrap gap-2">
                {project.detail_names.map((name) => (
                  <li key={name} className="max-w-full">
                    <Badge
                      variant="outline"
                      className="max-w-full rounded-md border-primary/15 bg-primary/8 px-2 py-1 text-left font-normal wrap-anywhere whitespace-normal"
                    >
                      {name}
                    </Badge>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-sm text-muted-foreground">{formatValue(null)}</p>
            )}
          </section>
          <section className="space-y-3 border-t border-border pt-5">
            <h2 className="text-sm font-medium">项目附件</h2>
            {project.attachment_ids.length ? (
              project.attachment_ids.map((id) =>
                canReadAttachments ? (
                  <span key={id} className="flex min-w-0 items-center gap-2">
                    <Paperclip className="size-4" aria-hidden="true" />
                    <AttachmentFilename
                      filename={`附件 #${String(id)}`}
                      className="flex-1"
                      downloadURL={attachmentDownload(workspaceID, id)}
                    />
                  </span>
                ) : (
                  <p key={id} className="text-sm">
                    附件 #{String(id)}
                  </p>
                ),
              )
            ) : (
              <p className="text-sm text-muted-foreground">{formatValue(null)}</p>
            )}
          </section>
          <dl className="grid grid-cols-2 gap-4 border-t border-border pt-5">
            <div className="space-y-1.5">
              <dt className="text-xs text-muted-foreground">创建时间</dt>
              <dd>
                <TableTimestamp value={project.created_at} />
              </dd>
            </div>
            <div className="space-y-1.5">
              <dt className="text-xs text-muted-foreground">更新时间</dt>
              <dd>
                <TableTimestamp value={project.updated_at} />
              </dd>
            </div>
          </dl>
        </section>
        <section
          role="tabpanel"
          id={`${tabsID}-panel-application`}
          aria-labelledby={`${tabsID}-tab-application`}
          hidden={tab !== 'application'}
          className="space-y-5"
        >
          <dl className="grid gap-4 text-sm sm:grid-cols-2">
            <div className="space-y-1">
              <dt className="text-xs text-muted-foreground">允许学生自行申报</dt>
              <dd>
                <ProjectEnabledBadge enabled={project.allow_self_apply} enabledLabel="允许" />
              </dd>
            </div>
            {project.allow_self_apply ? (
              <div className="space-y-1 sm:col-span-2">
                <dt className="text-xs text-muted-foreground">自行申报截止时间</dt>
                <dd className="space-y-1">
                  <TableTimestamp value={project.self_apply_deadline} />
                  <p className="text-xs text-muted-foreground">
                    截止时间后 1 分钟自动提交本项目全部草稿申报。
                  </p>
                </dd>
              </div>
            ) : null}
          </dl>
          {project.allow_self_apply ? (
            <section className="space-y-3 border-t border-border pt-5">
              <h2 className="text-sm font-medium">审批流程</h2>
              {project.approval_steps.length ? (
                <ApplicationFlow record={null} preview={project.approval_steps} />
              ) : (
                <p className="text-sm text-muted-foreground">{formatValue(null)}</p>
              )}
            </section>
          ) : null}
        </section>
      </div>
    </Drawer>
  );
}
