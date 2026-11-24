import { Paperclip, Plus, Trash2, Upload } from 'lucide-react';

import { AttachmentFilename } from '@/components/attachment-filename';
import { DetailValue } from '@/components/detail-value';
import { TableTimestamp } from '@/components/table-timestamp';
import { AutoTextarea } from '@/components/ui/auto-textarea';
import { Button } from '@/components/ui/button';
import { DeleteConfirmation } from '@/components/ui/delete-confirmation';
import { FileUploadButton } from '@/components/ui/file-upload-button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { SearchSelect } from '@/components/ui/search-select';
import { Tooltip } from '@/components/ui/tooltip';
import { maxParticipationRank } from '@/features/project/api';
import {
  type Application,
  type ApplicationAttachment,
  applicationAttachmentDownload,
  type ApplicationContent,
  type ApplicationScope,
} from '@/features/project/application-api';
import { ApplicationStatusBadge } from '@/features/project/application-status';
import { getConfig } from '@/lib/config';
import { uploadHint, uploadStateLabels, type UploadUpdate } from '@/lib/file-upload';
import { formatIdentity, formatScore, formatValue } from '@/lib/format';

export type ApplicationValues = Pick<ApplicationContent, 'statement' | 'rank' | 'score'>;
export type EditableApplicationDetail = ApplicationContent['details'][number] & {
  attachments: ApplicationAttachment[];
};

interface FileProps {
  workspaceID: string;
  scope: ApplicationScope;
  applicationID: number;
  busy: boolean;
  uploading: boolean;
  onUpload?: (file: File, updateUpload: UploadUpdate) => void;
  onPrepareUpload?: (() => void) | undefined;
  onDelete?: (file: ApplicationAttachment) => Promise<boolean>;
}

export function ApplicationMetadata({ record }: { record: Application }) {
  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="text-xs text-muted-foreground">申报编号 #{String(record.id)}</span>
        <ApplicationStatusBadge status={record.status} />
      </div>
      <dl className="grid grid-cols-1 gap-x-6 gap-y-4 text-sm sm:grid-cols-2">
        <div className="min-w-0 space-y-1 sm:col-span-2">
          <dt className="text-xs text-muted-foreground">项目说明</dt>
          <DetailValue label="项目说明" value={record.project_description} copyable={false} />
        </div>
        <div className="min-w-0 space-y-1">
          <dt className="text-xs text-muted-foreground">申报人</dt>
          <dd className="wrap-anywhere">
            {formatIdentity(record.student_name, record.student_no)}
          </dd>
        </div>
        <div className="min-w-0 space-y-1">
          <dt className="text-xs text-muted-foreground">班级</dt>
          <dd className="wrap-anywhere">{formatValue(record.class_name)}</dd>
        </div>
        <div className="min-w-0 space-y-1 sm:col-span-2">
          <dt className="text-xs text-muted-foreground">当前处理人</dt>
          <dd className="wrap-anywhere">{formatValue(record.current_reviewer)}</dd>
        </div>
        <div className="space-y-1">
          <dt className="text-xs text-muted-foreground">创建时间</dt>
          <dd>
            <TableTimestamp value={record.created_at} />
          </dd>
        </div>
        <div className="space-y-1">
          <dt className="text-xs text-muted-foreground">更新时间</dt>
          <dd>
            <TableTimestamp value={record.updated_at} />
          </dd>
        </div>
      </dl>
    </div>
  );
}

export function ApplicationFiles({
  files,
  workspaceID,
  scope,
  applicationID,
  busy,
  uploading,
  onUpload,
  onPrepareUpload,
  onDelete,
}: FileProps & { files: ApplicationAttachment[] }) {
  return (
    <div className="space-y-2">
      {files.map((file) => (
        <div key={file.id} className="flex min-h-8 min-w-0 items-center gap-2 text-xs">
          <Paperclip className="size-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
          <AttachmentFilename
            filename={file.filename}
            className="flex-1"
            downloadURL={
              file.state === 'ready'
                ? applicationAttachmentDownload(workspaceID, scope, applicationID, file.id)
                : undefined
            }
          />
          {file.state !== 'ready' ? (
            <span className="shrink-0 text-warning-foreground">
              {uploadStateLabels[file.state]}
            </span>
          ) : null}
          {onDelete ? (
            <DeleteConfirmation
              target={file.filename}
              onConfirm={() => onDelete(file)}
              disabled={busy}
            />
          ) : null}
        </div>
      ))}
      {onUpload ? (
        <div className="flex flex-wrap items-center gap-2">
          {onPrepareUpload ? (
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={busy}
              onClick={onPrepareUpload}
            >
              <Upload aria-hidden="true" />
              保存内容并上传
            </Button>
          ) : (
            <FileUploadButton
              label="上传附件"
              loading={uploading}
              disabled={busy}
              onFileSelect={onUpload}
            />
          )}
          <span className="text-xs text-muted-foreground">{uploadHint()}</span>
        </div>
      ) : null}
    </div>
  );
}

export function ApplicationContentView({
  record,
  files,
}: {
  record: Application;
  files: Omit<FileProps, 'applicationID' | 'uploading'>;
}) {
  return (
    <div className="space-y-5 border-t border-border pt-5 text-sm">
      <dl className="space-y-4">
        <div className="space-y-1">
          <dt className="text-xs text-muted-foreground">申报说明</dt>
          <DetailValue
            label="申报说明"
            value={record.statement}
            copyable={record.statement.trim() !== ''}
          />
        </div>
        <div className="grid grid-cols-2 gap-6">
          <div className="space-y-1">
            <dt className="text-xs text-muted-foreground">排名</dt>
            <dd className="text-sm tabular-nums">{formatValue(record.rank)}</dd>
          </div>
          <div className="space-y-1">
            <dt className="text-xs text-muted-foreground">总分</dt>
            <dd className="text-sm tabular-nums">{formatScore(record.score)}</dd>
          </div>
        </div>
      </dl>
      <section className="space-y-3 border-t border-border pt-4" aria-label="评分子项详情">
        <h3 className="text-sm font-medium">
          评分子项{' '}
          <span className="text-xs font-normal text-muted-foreground">
            （{String(record.details.length)}）
          </span>
        </h3>
        <ol className="divide-y divide-border">
          {record.details.map((item, index) => (
            <li key={item.key} className="space-y-3 py-4 first:pt-0 last:pb-0">
              <div className="flex items-start justify-between gap-4">
                <div className="min-w-0 space-y-1">
                  <p className="text-xs text-muted-foreground">子项 {String(index + 1)}</p>
                  <h4 className="text-sm font-medium wrap-anywhere">{item.name}</h4>
                </div>
                <dl className="shrink-0 space-y-1 text-right">
                  <dt className="text-xs text-muted-foreground">分数</dt>
                  <dd className="text-sm tabular-nums">{formatValue(item.score)}</dd>
                </dl>
              </div>
              <p className="text-sm leading-6 wrap-anywhere whitespace-pre-wrap">
                {formatValue(item.description)}
              </p>
              <ApplicationFiles
                {...files}
                applicationID={record.id}
                files={item.attachments}
                uploading={false}
              />
            </li>
          ))}
        </ol>
        {record.details.length === 0 ? (
          <p className="text-sm text-muted-foreground">暂无评分子项</p>
        ) : null}
      </section>
    </div>
  );
}

export function ApplicationContentEditor({
  values,
  items,
  names,
  busy,
  onValues,
  onItems,
  filesFor,
  calculationEnabled = false,
}: {
  values: ApplicationValues;
  items: EditableApplicationDetail[];
  names: string[];
  busy: boolean;
  onValues: (patch: Partial<ApplicationValues>) => void;
  onItems: (items: EditableApplicationDetail[]) => void;
  filesFor: (key: string) => FileProps;
  calculationEnabled?: boolean;
}) {
  const changeItem = (key: string, patch: Partial<EditableApplicationDetail>) => {
    onItems(items.map((item) => (item.key === key ? { ...item, ...patch } : item)));
  };
  return (
    <fieldset disabled={busy} className="space-y-5 text-sm">
      <div className="space-y-2">
        <Label
          htmlFor="application-statement"
          className="text-xs font-normal text-muted-foreground"
        >
          申报说明（选填）
        </Label>
        <AutoTextarea
          id="application-statement"
          aria-label="申报说明"
          placeholder="申报说明"
          maxLength={10000}
          value={values.statement}
          onChange={(event) => {
            onValues({ statement: event.target.value });
          }}
        />
      </div>
      {calculationEnabled ? (
        <div className="space-y-1">
          <p className="text-xs text-muted-foreground">总分</p>
          <p className="tabular-nums">{formatScore(values.score)}</p>
          <p className="text-xs text-muted-foreground">保存、提交时按项目公式计算。</p>
        </div>
      ) : (
        <div className="grid grid-cols-2 gap-4">
          <div className="space-y-2">
            <Label htmlFor="application-rank" className="text-xs font-normal text-muted-foreground">
              排名（选填）
            </Label>
            <Input
              id="application-rank"
              className="text-sm"
              type="number"
              min={1}
              max={maxParticipationRank}
              step={1}
              value={values.rank ?? ''}
              onChange={(event) => {
                onValues({ rank: event.target.value === '' ? null : Number(event.target.value) });
              }}
            />
          </div>
          <div className="space-y-2">
            <Label
              htmlFor="application-score"
              className="text-xs font-normal text-muted-foreground"
            >
              总分（选填）
            </Label>
            <Input
              id="application-score"
              className="text-sm"
              type="number"
              step="any"
              value={values.score ?? ''}
              onChange={(event) => {
                onValues({ score: event.target.value === '' ? null : Number(event.target.value) });
              }}
            />
          </div>
        </div>
      )}
      <section className="space-y-3 border-t border-border pt-4" aria-label="申报评分子项">
        <div className="flex items-center justify-between gap-3">
          <h3 className="text-sm font-medium">
            评分子项{' '}
            <span className="text-xs font-normal text-muted-foreground">
              （{String(items.length)}）
            </span>
          </h3>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            disabled={names.length === 0 || items.length >= getConfig().projectMaxDetails}
            onClick={() => {
              onItems([
                ...items,
                {
                  key: crypto.randomUUID().replaceAll('-', ''),
                  name: '',
                  description: '',
                  score: null,
                  attachments: [],
                },
              ]);
            }}
          >
            <Plus aria-hidden="true" />
            添加子项
          </Button>
        </div>
        <div className="divide-y divide-border">
          {items.map((item, index) => (
            <div key={item.key} className="space-y-3 py-4 first:pt-0 last:pb-0">
              <div className="flex items-center justify-between gap-3">
                <span className="text-xs text-muted-foreground">子项 {String(index + 1)}</span>
                <Tooltip
                  content={item.attachments.length ? '请先删除附件，再移除子项' : '移除子项'}
                >
                  <span>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon-sm"
                      aria-label={`移除子项 ${String(index + 1)}`}
                      disabled={item.attachments.length > 0}
                      onClick={() => {
                        onItems(items.filter((entry) => entry.key !== item.key));
                      }}
                    >
                      <Trash2 aria-hidden="true" />
                    </Button>
                  </span>
                </Tooltip>
              </div>
              <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_8rem]">
                <div className="space-y-2">
                  <Label
                    htmlFor={`application-name-${item.key}`}
                    className="text-xs font-normal text-muted-foreground"
                  >
                    子项名称
                  </Label>
                  <SearchSelect
                    id={`application-name-${item.key}`}
                    label="子项名称"
                    className="w-full"
                    value={item.name}
                    options={names
                      .filter(
                        (name) =>
                          name === item.name ||
                          !items.some((other) => other.key !== item.key && other.name === name),
                      )
                      .map((name) => ({ value: name, label: name }))}
                    onValueChange={(name) => {
                      changeItem(item.key, { name });
                    }}
                  />
                </div>
                <div className="space-y-2">
                  <Label
                    htmlFor={`application-score-${item.key}`}
                    className="text-xs font-normal text-muted-foreground"
                  >
                    分数（选填）
                  </Label>
                  <Input
                    id={`application-score-${item.key}`}
                    className="text-sm"
                    type="number"
                    step="any"
                    value={item.score ?? ''}
                    onChange={(event) => {
                      changeItem(item.key, {
                        score: event.target.value === '' ? null : Number(event.target.value),
                      });
                    }}
                  />
                </div>
              </div>
              <div className="space-y-2">
                <Label
                  htmlFor={`application-description-${item.key}`}
                  className="text-xs font-normal text-muted-foreground"
                >
                  子项说明
                </Label>
                <AutoTextarea
                  id={`application-description-${item.key}`}
                  value={item.description}
                  maxLength={10000}
                  onChange={(event) => {
                    changeItem(item.key, { description: event.target.value });
                  }}
                />
              </div>
              <ApplicationFiles {...filesFor(item.key)} files={item.attachments} />
            </div>
          ))}
        </div>
        {items.length === 0 ? <p className="text-sm text-muted-foreground">暂无评分子项</p> : null}
      </section>
    </fieldset>
  );
}
