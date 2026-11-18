-- 用于已有数据库的 COS 直传结构调整，只执行一次。
-- 全新空库使用 0001_init.sql；本文件不包含历史数据回填。
-- 原有附件保留为通用二进制声明类型，新申请按 STS 类型精确登记。

ALTER TABLE `cpc_person_timeline_attachments`
  RENAME COLUMN `s3_key` TO `object_key`,
  ADD COLUMN `content_type` varchar(255) NOT NULL DEFAULT 'application/octet-stream' COMMENT 'STS 约束的声明媒体类型' AFTER `size`,
  ADD COLUMN `upload_actor` varchar(32) NOT NULL DEFAULT '' COMMENT '申请时的可信身份快照，用于确认上传' AFTER `content_type`;
ALTER TABLE `cpc_person_timeline_attachments`
  ALTER COLUMN `content_type` DROP DEFAULT,
  ALTER COLUMN `upload_actor` DROP DEFAULT;

ALTER TABLE `cpc_timeline_node_attachments`
  RENAME COLUMN `s3_key` TO `object_key`,
  ADD COLUMN `content_type` varchar(255) NOT NULL DEFAULT 'application/octet-stream' COMMENT 'STS 约束的声明媒体类型' AFTER `size`,
  ADD COLUMN `upload_actor` varchar(32) NOT NULL DEFAULT '' COMMENT '申请时的可信身份快照，用于确认上传' AFTER `content_type`;
ALTER TABLE `cpc_timeline_node_attachments`
  ALTER COLUMN `content_type` DROP DEFAULT,
  ALTER COLUMN `upload_actor` DROP DEFAULT;

ALTER TABLE `monthly_work_attachments`
  RENAME COLUMN `s3_key` TO `object_key`,
  ADD COLUMN `content_type` varchar(255) NOT NULL DEFAULT 'application/octet-stream' COMMENT 'STS 约束的声明媒体类型' AFTER `size`,
  ADD COLUMN `upload_actor` varchar(32) NOT NULL DEFAULT '' COMMENT '申请时的可信身份快照，用于确认上传' AFTER `content_type`;
ALTER TABLE `monthly_work_attachments`
  ALTER COLUMN `content_type` DROP DEFAULT,
  ALTER COLUMN `upload_actor` DROP DEFAULT;

ALTER TABLE `project_attachments`
  RENAME COLUMN `s3_key` TO `object_key`,
  ADD COLUMN `content_type` varchar(255) NOT NULL DEFAULT 'application/octet-stream' COMMENT 'STS 约束的声明媒体类型' AFTER `size`,
  ADD COLUMN `upload_actor` varchar(32) NOT NULL DEFAULT '' COMMENT '申请时的可信身份快照，用于确认上传' AFTER `content_type`;
ALTER TABLE `project_attachments`
  ALTER COLUMN `content_type` DROP DEFAULT,
  ALTER COLUMN `upload_actor` DROP DEFAULT;

ALTER TABLE `project_application_detail_attachments`
  RENAME COLUMN `s3_key` TO `object_key`,
  ADD COLUMN `content_type` varchar(255) NOT NULL DEFAULT 'application/octet-stream' COMMENT 'STS 约束的声明媒体类型' AFTER `size`,
  ADD COLUMN `upload_actor` varchar(32) NOT NULL DEFAULT '' COMMENT '申请时的可信身份快照，用于确认上传' AFTER `content_type`;
ALTER TABLE `project_application_detail_attachments`
  ALTER COLUMN `content_type` DROP DEFAULT,
  ALTER COLUMN `upload_actor` DROP DEFAULT;

CREATE TABLE `student_avatar_uploads`
(
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '头像上传编号',
  `workspace_id` char(16) NOT NULL COMMENT '工作空间编号',
  `student_id` bigint unsigned NOT NULL COMMENT '所属学生编号',
  `student_revision` bigint unsigned NOT NULL COMMENT '申请时的学生并发版本',
  `object_hash` char(64) NOT NULL COMMENT '对象键的 SHA-256',
  `object_key` varchar(1024) NOT NULL COMMENT '私有 COS 正式对象键',
  `filename` varchar(255) NOT NULL COMMENT '原始文件名',
  `size` bigint NOT NULL COMMENT '准确文件字节数',
  `content_type` varchar(255) NOT NULL COMMENT 'STS 约束的声明媒体类型',
  `upload_actor` varchar(32) NOT NULL COMMENT '申请时的可信身份快照，用于确认上传',
  `state` varchar(16) NOT NULL COMMENT 'pending、ready、deleting',
  `revision` bigint unsigned NOT NULL DEFAULT '1' COMMENT '上传记录并发版本',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC，由数据库维护',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC，由数据库维护',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_student_avatar_uploads_tenant` (`workspace_id`, `id`),
  KEY `idx_student_avatar_uploads_student` (`workspace_id`, `student_id`, `state`, `id`),
  CONSTRAINT `fk_student_avatar_uploads_student` FOREIGN KEY (`workspace_id`, `student_id`) REFERENCES `students` (`workspace_id`, `id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `ck_student_avatar_uploads_state` CHECK (`state` IN ('pending', 'ready', 'deleting'))
) ENGINE = InnoDB COMMENT ='头像直传记录，失败记录由用户显式删除';

