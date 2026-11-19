-- 由数据库维护账号在全新、空的 utf8mb4 数据库人工执行；既有数据库不得重跑。
-- 已合并当前全部结构变更；运行账号仅需 DML 权限，不导入业务数据。
-- 表和列继承数据库字符集与排序规则，保留审计动态分区初始化及维护逻辑。
SET NAMES utf8mb4;
SET SESSION time_zone = '+00:00';
SET SESSION group_concat_max_len = 1048576;
SET SESSION foreign_key_checks = 1;

CREATE TABLE `users`
(
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '账号编号',
  `username` varchar(64) NOT NULL COMMENT '唯一小写登录账号',
  `display_name` varchar(100) NOT NULL COMMENT '显示名称',
  `email` varchar(254) NOT NULL DEFAULT '' COMMENT '联系邮箱',
  `email_login_verified` tinyint(1) NOT NULL DEFAULT '0' COMMENT '当前邮箱是否已通过邮件登录用户名核验',
  `password_hash` varchar(60) NOT NULL COMMENT 'bcrypt 用户名加盐口令摘要散列',
  `password_login_enabled` tinyint(1) NOT NULL DEFAULT '1' COMMENT '是否允许密码登录，禁用时不要求修改密码',
  `password_change_required` tinyint(1) NOT NULL DEFAULT '1' COMMENT '首次登录或重置后必须更换口令',
  `is_admin` tinyint(1) NOT NULL DEFAULT '0' COMMENT '是否系统管理员，仅用于系统管理',
  `is_enabled` tinyint(1) NOT NULL DEFAULT '1' COMMENT '账号是否启用',
  `session_version` bigint unsigned NOT NULL DEFAULT '1' COMMENT '会话版本，口令修改、重置或停用时递增',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC，由数据库维护',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC，由数据库维护',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_users_username` (`username`),
  KEY `idx_users_access_search` (`id`, `username`, `display_name`)
) ENGINE = InnoDB COMMENT ='全局登录账号';

-- 每个账号最多一个 API Token，认证使用摘要，独立密钥加密原值以支持再次查看。
CREATE TABLE `user_api_tokens`
(
  `user_id` bigint unsigned NOT NULL COMMENT '所属账号编号',
  `token_hash` char(64) NOT NULL COMMENT 'API Token 的 SHA-256 摘要',
  `ciphertext` varbinary(256) NOT NULL COMMENT '使用独立密钥加密的 API Token',
  `session_version` bigint unsigned NOT NULL COMMENT '生成时的账号会话版本',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC，由数据库维护',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC，由数据库维护',
  PRIMARY KEY (`user_id`),
  UNIQUE KEY `uk_user_api_tokens_hash` (`token_hash`),
  CONSTRAINT `fk_user_api_tokens_user_cascade` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE = InnoDB COMMENT ='可由管理员再次查看的账号 API Token';

CREATE TABLE `workspaces`
(
  `id` char(16) NOT NULL COMMENT '16 位英文小写与数字混合的随机工作空间编号，即租户编号',
  `name` varchar(100) NOT NULL COMMENT '唯一工作空间名称',
  `description` varchar(500) NOT NULL COMMENT '用途说明',
  `is_enabled` tinyint(1) NOT NULL DEFAULT '1' COMMENT '工作空间是否启用',
  `revision` bigint unsigned NOT NULL DEFAULT '1' COMMENT '并发修改校验版本',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC，由数据库维护',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC，由数据库维护',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_workspaces_name` (`name`)
) ENGINE = InnoDB COMMENT ='工作空间租户边界';

CREATE TABLE `access_permissions`
(
  `user_id` bigint unsigned NOT NULL COMMENT '获得授权的全局账号编号',
  `action` varchar(64) NOT NULL COMMENT '代码目录登记的英文 workspace 操作标识',
  `workspace_id` char(16) NOT NULL COMMENT '授权所属工作空间编号，即租户编号',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC，由数据库维护',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC，由数据库维护',
  PRIMARY KEY (`workspace_id`, `user_id`, `action`),
  KEY `idx_access_permissions_user` (`user_id`),
  CONSTRAINT `fk_access_permissions_user_cascade` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `fk_access_permissions_workspace_cascade` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE = InnoDB COMMENT ='用户操作工作空间的直接授权，无记录即拒绝';

CREATE TABLE `calendar_holidays`
(
  `date` date NOT NULL COMMENT '节日对应的公历日期',
  `holiday` varchar(100) NOT NULL COMMENT '公开节日名称',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC',
  PRIMARY KEY (`date`, `holiday`),
  CONSTRAINT `ck_calendar_holidays_name` CHECK ((char_length(trim(`holiday`)) > 0))
) ENGINE = InnoDB COMMENT ='首页公开节日资料，同一天可记录多个节日，按公历年份人工维护';

CREATE TABLE `cpc_party_branches`
(
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '党支部自增编号',
  `workspace_id` char(16) NOT NULL COMMENT '所属工作空间编号',
  `full_name` varchar(200) NOT NULL COMMENT '党支部全称，同一空间内唯一',
  `short_name` varchar(100) NOT NULL COMMENT '党支部简称，同一空间内唯一',
  `priority` int NOT NULL DEFAULT '0' COMMENT '优先级，数字越大越优先，同值按编号倒序',
  `secretary_id` bigint unsigned DEFAULT NULL COMMENT '党支部书记用户编号，不参与授权判断',
  `revision` bigint unsigned NOT NULL DEFAULT '1' COMMENT '并发修改版本',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC，由数据库维护',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC，由数据库维护',
  `admission_rule` json DEFAULT NULL COMMENT '人员准入规则树，条件组支持 and、or，条件仅支持 in',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_cpc_party_branches_tenant` (`workspace_id`, `id`),
  UNIQUE KEY `uk_cpc_party_branches_name` (`workspace_id`, `full_name`),
  UNIQUE KEY `uk_cpc_party_branches_short_name` (`workspace_id`, `short_name`),
  KEY `idx_cpc_party_branches_secretary` (`secretary_id`),
  KEY `idx_cpc_party_branches_priority` (`workspace_id`, `priority`, `id`),
  CONSTRAINT `fk_cpc_party_branches_secretary_cascade` FOREIGN KEY (`secretary_id`) REFERENCES `users` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `fk_cpc_party_branches_workspace_cascade` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `ck_cpc_party_branches_admission_rule` CHECK (((`admission_rule` is null) or (json_type(`admission_rule`) = _utf8mb4'OBJECT')))
) ENGINE = InnoDB COMMENT ='党建工作党支部，按工作空间隔离，硬删除';

CREATE TABLE `cpc_timeline_templates`
(
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '时间线模板编号',
  `workspace_id` char(16) NOT NULL COMMENT '所属工作空间编号',
  `name` varchar(100) NOT NULL COMMENT '空间内唯一模板名称',
  `description` varchar(1000) NOT NULL DEFAULT '' COMMENT '模板说明',
  `revision` bigint unsigned NOT NULL DEFAULT '1' COMMENT '并发修改版本',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC，由数据库维护',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC，由数据库维护',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_cpc_timeline_templates_tenant` (`workspace_id`, `id`),
  UNIQUE KEY `uk_cpc_timeline_templates_name` (`workspace_id`, `name`),
  CONSTRAINT `fk_cpc_timeline_templates_workspace_cascade` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE = InnoDB COMMENT ='党建时间线模板，按空间隔离，硬删除';

CREATE TABLE `students`
(
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '学生记录编号',
  `workspace_id` char(16) NOT NULL COMMENT '工作空间编号',
  `student_no` varchar(32) NOT NULL COMMENT '学号',
  `name` varchar(50) NOT NULL COMMENT '姓名',
  `avatar` varchar(1024) NOT NULL DEFAULT '' COMMENT '头像的私有 COS 对象键，空字符串为未设置',
  `grade` varchar(16) NOT NULL COMMENT '年级',
  `status` varchar(8) NOT NULL COMMENT '用户手动维护的学生状态',
  `education_level` varchar(32) NOT NULL COMMENT '学历',
  `major` varchar(100) NOT NULL COMMENT '专业',
  `class_name` varchar(64) NOT NULL COMMENT '班级',
  `class_position` varchar(64) NOT NULL COMMENT '班内任职',
  `admission_type` varchar(32) NOT NULL COMMENT '录取类型',
  `advisor` varchar(50) NOT NULL COMMENT '导师姓名',
  `advisor_no` varchar(32) NOT NULL DEFAULT '' COMMENT '导师工号',
  `advisor_phone` varchar(32) NOT NULL DEFAULT '' COMMENT '导师电话',
  `identity_number` varchar(18) NOT NULL COMMENT '身份证号码',
  `birth_date` varchar(10) GENERATED ALWAYS AS ((case when regexp_like(`identity_number`, _utf8mb4'^[0-9]{6}[1-9][0-9]{3}(0[1-9]|1[0-2])(0[1-9]|[12][0-9]|3[01])[0-9]{3}[0-9Xx]$', _utf8mb4'c') then (case when ( cast(substr(`identity_number`, 13, 2) as unsigned) <= dayofmonth(last_day(concat( substr(`identity_number`, 7, 4), _utf8mb4'-', substr( `identity_number`, 11, 2), _utf8mb4'-01')))) then concat( substr(`identity_number`, 7, 4), _utf8mb4'-', substr(`identity_number`, 11, 2), _utf8mb4'-', substr(`identity_number`, 13, 2)) else _utf8mb4'' end) else _utf8mb4'' end)) STORED NOT NULL COMMENT '根据18位身份证号码生成的出生日期，无法解析时为空',
  `gender` varchar(16) NOT NULL COMMENT '性别',
  `ethnicity` varchar(32) NOT NULL COMMENT '民族',
  `political_status` varchar(32) NOT NULL COMMENT '政治面貌',
  `phone` varchar(32) NOT NULL COMMENT '电话号码',
  `qq` varchar(20) NOT NULL COMMENT 'QQ号码',
  `email` varchar(254) NOT NULL COMMENT '邮箱',
  `accommodation_type` varchar(4) NOT NULL COMMENT '住宿类型；空文本为未知（未填写），in 校内，out 校外，abrd 出国联培',
  `campus` varchar(64) NOT NULL COMMENT '住宿园区',
  `building` varchar(32) NOT NULL COMMENT '宿舍楼',
  `room` varchar(32) NOT NULL COMMENT '宿舍号',
  `bed` varchar(16) NOT NULL COMMENT '床位号',
  `origin` varchar(128) NOT NULL COMMENT '户籍所在地（省市）',
  `home_address` varchar(255) NOT NULL COMMENT '家庭详细住址',
  `guardian1_name` varchar(50) NOT NULL COMMENT '家长1姓名',
  `guardian1_relationship` varchar(32) NOT NULL COMMENT '家长1关系',
  `guardian1_phone` varchar(32) NOT NULL COMMENT '家长1联系方式',
  `guardian2_name` varchar(50) NOT NULL COMMENT '家长2姓名',
  `guardian2_relationship` varchar(32) NOT NULL COMMENT '家长2关系',
  `guardian2_phone` varchar(32) NOT NULL COMMENT '家长2联系方式',
  `only_child` varchar(4) NOT NULL COMMENT '是否独生子女；空文本为未填写，yes 是，no 否',
  `single_parent` varchar(4) NOT NULL COMMENT '是否单亲家庭；空文本为未填写，yes 是，no 否',
  `orphan` varchar(4) NOT NULL COMMENT '是否孤儿；空文本为未填写，yes 是，no 否',
  `student_loan` varchar(4) NOT NULL COMMENT '是否助学贷款；空文本为未填写，yes 是，no 否',
  `registered_poverty` varchar(4) NOT NULL COMMENT '是否建档立卡生；空文本为未填写，yes 是，no 否',
  `has_religion` varchar(4) NOT NULL COMMENT '是否有宗教信仰；空文本为未填写，yes 是，no 否',
  `religion_category` varchar(32) NOT NULL COMMENT '宗教信仰类别',
  `religion_identification` varchar(16) NOT NULL COMMENT '宗教信仰标识',
  `religion_start_year` varchar(16) NOT NULL COMMENT '宗教信仰起始年份',
  `family_religion` varchar(1000) NOT NULL COMMENT '家庭成员宗教信仰情况',
  `medical_history` varchar(2000) NOT NULL COMMENT '治疗中的心理疾病或既往病史',
  `special_circumstances` varchar(2000) NOT NULL COMMENT '其他本人或家庭特殊情况',
  `graduation_year` varchar(4) NOT NULL DEFAULT '' COMMENT '独立填写的四位毕业年份，空值单独归组',
  `graduation_type` varchar(71) NOT NULL DEFAULT '' COMMENT '毕业去向英文类型代码，空值计入待就业',
  `graduation_type_label` varchar(100) NOT NULL DEFAULT '' COMMENT '毕业去向类型显示名称，从已保存学生数据去重枚举',
  `graduation_destination` varchar(255) NOT NULL DEFAULT '' COMMENT '毕业去向',
  `revision` bigint unsigned NOT NULL DEFAULT '1' COMMENT '基础、头像、隐私和毕业去向修改共用的并发版本',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC，由数据库维护',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC，由数据库维护',
  `native_place` varchar(128) NOT NULL DEFAULT '' COMMENT '籍贯',
  `previous_school_or_employer` varchar(255) NOT NULL DEFAULT '' COMMENT '毕（肄）业学校及单位',
  `arrival_month` varchar(7) NOT NULL DEFAULT '' COMMENT '来校时间，YYYY-MM，未填写时为空',
  `previous_graduation_month` varchar(7) NOT NULL DEFAULT '' COMMENT '既往毕（肄）业时间，YYYY-MM，未填写时为空',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_students_tenant_id` (`workspace_id`, `id`),
  UNIQUE KEY `uk_students_number` (`workspace_id`, `student_no`),
  KEY `idx_students_class` (`workspace_id`, `class_name`),
  KEY `idx_students_grade` (`workspace_id`, `grade`),
  KEY `idx_students_status` (`workspace_id`, `status`),
  KEY `idx_students_education` (`workspace_id`, `education_level`),
  KEY `idx_students_major` (`workspace_id`, `major`),
  KEY `idx_students_graduation_type` (`workspace_id`, `graduation_type`),
  KEY `idx_students_graduation_year` (`workspace_id`, `graduation_year`),
  KEY `idx_students_position` (`workspace_id`, `class_position`),
  KEY `idx_students_accommodation` (`workspace_id`, `accommodation_type`),
  KEY `idx_students_religion` (`workspace_id`, `has_religion`),
  CONSTRAINT `fk_students_workspace_cascade` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `ck_students_accommodation_type_v2` CHECK ((`accommodation_type` in (_utf8mb4'', _utf8mb4'in', _utf8mb4'out', _utf8mb4'abrd'))),
  CONSTRAINT `ck_students_arrival_month` CHECK (((`arrival_month` = _utf8mb4'') or (regexp_like(`arrival_month`, _utf8mb4'^[0-9]{4}-(0[1-9]|1[0-2])$') and (`arrival_month` between _utf8mb4'1900-01' and _utf8mb4'9999-12')))),
  CONSTRAINT `ck_students_graduation_type` CHECK (( ((`graduation_type` = _utf8mb4'') and (`graduation_type_label` = _utf8mb4'')) or (regexp_like(`graduation_type`, _utf8mb4'^custom_[0-9a-f]{64}$') and (`graduation_type_label` <> _utf8mb4'')))),
  CONSTRAINT `ck_students_graduation_year` CHECK (((`graduation_year` = _utf8mb4'') or (regexp_like(`graduation_year`, _utf8mb4'^[0-9]{4}$') and (`graduation_year` between _utf8mb4'1900' and _utf8mb4'9999')))),
  CONSTRAINT `ck_students_has_religion` CHECK ((`has_religion` in (_utf8mb4'', _utf8mb4'yes', _utf8mb4'no'))),
  CONSTRAINT `ck_students_only_child` CHECK ((`only_child` in (_utf8mb4'', _utf8mb4'yes', _utf8mb4'no'))),
  CONSTRAINT `ck_students_orphan` CHECK ((`orphan` in (_utf8mb4'', _utf8mb4'yes', _utf8mb4'no'))),
  CONSTRAINT `ck_students_previous_graduation_month` CHECK (((`previous_graduation_month` = _utf8mb4'') or (regexp_like(`previous_graduation_month`, _utf8mb4'^[0-9]{4}-(0[1-9]|1[0-2])$') and (`previous_graduation_month` between _utf8mb4'1900-01' and _utf8mb4'9999-12')))),
  CONSTRAINT `ck_students_registered_poverty` CHECK ((`registered_poverty` in (_utf8mb4'', _utf8mb4'yes', _utf8mb4'no'))),
  CONSTRAINT `ck_students_single_parent` CHECK ((`single_parent` in (_utf8mb4'', _utf8mb4'yes', _utf8mb4'no'))),
  CONSTRAINT `ck_students_student_loan` CHECK ((`student_loan` in (_utf8mb4'', _utf8mb4'yes', _utf8mb4'no')))
) ENGINE = InnoDB COMMENT ='工作空间学生信息宽表，业务数据明文存储，API 分别授权';

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

CREATE TABLE `cpc_people`
(
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '党建人员记录编号',
  `workspace_id` char(16) NOT NULL COMMENT '所属工作空间编号',
  `student_id` bigint unsigned NOT NULL COMMENT '基础信息来源学生编号',
  `stage` varchar(32) NOT NULL COMMENT '发展阶段英文代码',
  `status` varchar(8) NOT NULL DEFAULT '在编' COMMENT '用户自定义的党建人员状态',
  `party_branch_id` bigint unsigned NOT NULL COMMENT '准入规则匹配后编号最大的党支部，不接受手工修改',
  `party_position` varchar(64) NOT NULL DEFAULT '' COMMENT '自定义党内职务，不参与授权',
  `revision` bigint unsigned NOT NULL DEFAULT '1' COMMENT '并发修改版本',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC，由数据库维护',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC，由数据库维护',
  `timeline_template_id` bigint unsigned DEFAULT NULL COMMENT '绑定的时间线模板，存量可为空',
  `timeline_status` varchar(16) DEFAULT NULL COMMENT 'normal 正常、abnormal 异常、transferring 转移中，未绑定为空',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_cpc_people_tenant` (`workspace_id`, `id`),
  UNIQUE KEY `uk_cpc_people_student` (`workspace_id`, `student_id`),
  UNIQUE KEY `uk_cpc_people_timeline` (`workspace_id`, `id`, `timeline_template_id`),
  KEY `idx_cpc_people_branch` (`workspace_id`, `party_branch_id`),
  KEY `idx_cpc_people_stage` (`workspace_id`, `stage`),
  KEY `idx_cpc_people_timeline_status` (`workspace_id`, `timeline_status`, `id`),
  KEY `fk_cpc_people_timeline` (`workspace_id`, `timeline_template_id`),
  KEY `idx_cpc_people_status` (`workspace_id`, `status`),
  CONSTRAINT `fk_cpc_people_branch_cascade` FOREIGN KEY (`workspace_id`, `party_branch_id`) REFERENCES `cpc_party_branches` (`workspace_id`, `id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `fk_cpc_people_student_cascade` FOREIGN KEY (`workspace_id`, `student_id`) REFERENCES `students` (`workspace_id`, `id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `fk_cpc_people_timeline_cascade` FOREIGN KEY (`workspace_id`, `timeline_template_id`) REFERENCES `cpc_timeline_templates` (`workspace_id`, `id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `ck_cpc_people_stage` CHECK ((`stage` in (_utf8mb4'applicant', _utf8mb4'activist', _utf8mb4'development_candidate', _utf8mb4'probationary_member', _utf8mb4'full_member'))),
  CONSTRAINT `ck_cpc_people_timeline_status_values` CHECK ( `timeline_status` IS NULL OR `timeline_status` IN ('normal', 'abnormal', 'transferring') )
) ENGINE = InnoDB COMMENT ='党建人员当前名单，基础信息关联学生表，硬删除';

CREATE TABLE `cpc_timeline_template_nodes`
(
  `workspace_id` char(16) NOT NULL COMMENT '工作空间编号',
  `timeline_template_id` bigint unsigned NOT NULL COMMENT '所属时间线模板编号',
  `node_id` varchar(64) NOT NULL COMMENT '模板内稳定英文节点标识',
  `name` varchar(100) NOT NULL COMMENT '节点名称',
  `description` varchar(1000) NOT NULL DEFAULT '' COMMENT '节点说明',
  `group_name` varchar(100) NOT NULL DEFAULT '' COMMENT '节点分组，连续同名节点合并展示',
  `is_baseline` tinyint(1) NOT NULL DEFAULT '0' COMMENT '是否为基准节点，用于高亮及筛选',
  `time_field_id` varchar(64) NOT NULL COMMENT '必填发生时间字段标识',
  `field_definitions` json NOT NULL COMMENT '有序自定义字段，包含必填日期字段',
  `validation_rules` json NOT NULL COMMENT '仅提示的相对时间规则列表',
  `position` int unsigned NOT NULL COMMENT '从一开始的节点展示顺序',
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '固定数字节点编号，排序及重命名时保留',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC，由数据库维护',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC，由数据库维护',
  PRIMARY KEY (`workspace_id`, `timeline_template_id`, `node_id`),
  UNIQUE KEY `uk_cpc_timeline_nodes_numeric_id` (`id`),
  KEY `idx_cpc_timeline_nodes_position` (`workspace_id`, `timeline_template_id`, `position`, `node_id`),
  CONSTRAINT `fk_cpc_timeline_nodes_template_owned_cascade` FOREIGN KEY (`workspace_id`, `timeline_template_id`) REFERENCES `cpc_timeline_templates` (`workspace_id`, `id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `ck_cpc_timeline_nodes_fields` CHECK ((json_type(`field_definitions`) = _utf8mb4'ARRAY')),
  CONSTRAINT `ck_cpc_timeline_nodes_position` CHECK ((`position` > 0)),
  CONSTRAINT `ck_cpc_timeline_nodes_rules` CHECK ((json_type(`validation_rules`) = _utf8mb4'ARRAY'))
) ENGINE = InnoDB COMMENT ='党建时间线节点定义及顺序，按工作空间和模板隔离';

CREATE TABLE `cpc_person_timeline_nodes`
(
  `workspace_id` char(16) NOT NULL COMMENT '工作空间编号',
  `person_id` bigint unsigned NOT NULL COMMENT '党建人员编号',
  `timeline_template_id` bigint unsigned NOT NULL COMMENT '绑定的模板编号',
  `node_id` varchar(64) NOT NULL COMMENT '模板内稳定节点标识',
  `field_values` json NOT NULL COMMENT '人员独立填写值，键为字段标识',
  `status` varchar(16) NOT NULL DEFAULT 'pending' COMMENT 'pending 待处理、completed 已完成、skipped 已跳过',
  `revision` bigint unsigned NOT NULL DEFAULT '1' COMMENT '人员节点并发版本',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC，由数据库维护',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC，由数据库维护',
  PRIMARY KEY (`workspace_id`, `person_id`, `node_id`),
  KEY `idx_cpc_person_nodes_template` (`workspace_id`, `timeline_template_id`, `node_id`),
  KEY `fk_cpc_person_nodes_person` (`workspace_id`, `person_id`, `timeline_template_id`),
  CONSTRAINT `fk_cpc_person_nodes_person_owned_cascade` FOREIGN KEY (`workspace_id`, `person_id`, `timeline_template_id`) REFERENCES `cpc_people` (`workspace_id`, `id`, `timeline_template_id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `fk_cpc_person_nodes_template_cascade` FOREIGN KEY (`workspace_id`, `timeline_template_id`, `node_id`) REFERENCES `cpc_timeline_template_nodes` (`workspace_id`, `timeline_template_id`, `node_id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `ck_cpc_person_nodes_status` CHECK ((`status` in (_utf8mb4'pending', _utf8mb4'completed', _utf8mb4'skipped'))),
  CONSTRAINT `ck_cpc_person_nodes_values` CHECK ((json_type(`field_values`) = _utf8mb4'OBJECT'))
) ENGINE = InnoDB COMMENT ='人员独立时间线节点，模板变更在业务事务中显式同步';

CREATE TABLE `cpc_person_timeline_attachments`
(
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '人员节点附件编号',
  `workspace_id` char(16) NOT NULL COMMENT '工作空间编号',
  `person_id` bigint unsigned NOT NULL COMMENT '党建人员编号',
  `node_id` varchar(64) NOT NULL COMMENT '所属人员时间线节点标识',
  `object_hash` char(64) NOT NULL COMMENT '原始对象键的 SHA-256',
  `object_key` varchar(1024) NOT NULL COMMENT '私有对象键，包含人员、节点和随机目录',
  `filename` varchar(255) NOT NULL COMMENT '原始文件名',
  `size` bigint NOT NULL COMMENT '文件字节数',
  `content_type` varchar(255) NOT NULL COMMENT 'STS 约束的声明媒体类型',
  `upload_actor` varchar(32) NOT NULL COMMENT '申请时的可信身份快照，用于确认上传',
  `state` varchar(16) NOT NULL COMMENT 'pending、ready、deleting',
  `revision` bigint unsigned NOT NULL DEFAULT '1' COMMENT '并发版本',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC，由数据库维护',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC，由数据库维护',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_cpc_person_files_tenant` (`workspace_id`, `id`),
  KEY `idx_cpc_person_files_node` (`workspace_id`, `person_id`, `node_id`, `state`, `id`),
  KEY `idx_cpc_person_files_object` (`workspace_id`, `object_hash`),
  CONSTRAINT `fk_cpc_person_files_node_cascade` FOREIGN KEY (`workspace_id`, `person_id`, `node_id`) REFERENCES `cpc_person_timeline_nodes` (`workspace_id`, `person_id`, `node_id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `ck_cpc_person_files_state` CHECK ((`state` in (_utf8mb4'pending', _utf8mb4'ready', _utf8mb4'deleting')))
) ENGINE = InnoDB COMMENT ='人员节点私有附件，权限及支部范围沿用人员名单';

CREATE TABLE `cpc_person_timeline_transfer_reviews`
(
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '转移审核编号',
  `workspace_id` char(16) NOT NULL COMMENT '工作空间编号',
  `person_id` bigint unsigned NOT NULL COMMENT '党建人员编号',
  `requested_by_id` bigint unsigned NOT NULL COMMENT '转移申请提交人账号编号',
  `source_template_id` bigint unsigned NOT NULL COMMENT '发起时绑定的来源模板编号',
  `target_template_id` bigint unsigned NOT NULL COMMENT '申请转移到的目标模板编号',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_cpc_person_transfer_review` (`workspace_id`, `person_id`),
  KEY `fk_cpc_person_transfer_review_source` (`workspace_id`, `person_id`, `source_template_id`),
  KEY `fk_cpc_person_transfer_review_target` (`workspace_id`, `target_template_id`),
  KEY `fk_cpc_person_transfer_review_requester` (`requested_by_id`),
  CONSTRAINT `fk_cpc_person_transfer_review_requester_cascade` FOREIGN KEY (`requested_by_id`) REFERENCES `users` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `fk_cpc_person_transfer_review_source_cascade` FOREIGN KEY (`workspace_id`, `person_id`, `source_template_id`) REFERENCES `cpc_people` (`workspace_id`, `id`, `timeline_template_id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `fk_cpc_person_transfer_review_target_cascade` FOREIGN KEY (`workspace_id`, `target_template_id`) REFERENCES `cpc_timeline_templates` (`workspace_id`, `id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE = InnoDB COMMENT ='人员时间线待审核转移申请，完成或取消后硬删除，历史保留审计';

CREATE TABLE `cpc_timeline_node_attachments`
(
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '节点附件编号',
  `workspace_id` char(16) NOT NULL COMMENT '工作空间编号',
  `timeline_template_id` bigint unsigned NOT NULL COMMENT '所属时间线模板编号',
  `node_id` varchar(64) NOT NULL COMMENT '所属模板内的稳定节点标识',
  `object_hash` char(64) NOT NULL COMMENT '原始对象键的 SHA-256，保证对象键唯一',
  `object_key` varchar(1024) NOT NULL COMMENT '私有 COS 对象键，包含模板、节点和随机目录',
  `filename` varchar(255) NOT NULL COMMENT '原始文件名',
  `size` bigint NOT NULL COMMENT '文件字节数',
  `content_type` varchar(255) NOT NULL COMMENT 'STS 约束的声明媒体类型',
  `upload_actor` varchar(32) NOT NULL COMMENT '申请时的可信身份快照，用于确认上传',
  `state` varchar(16) NOT NULL COMMENT 'pending、ready、deleting',
  `revision` bigint unsigned NOT NULL DEFAULT '1' COMMENT '并发版本',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC，由数据库维护',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC，由数据库维护',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_cpc_timeline_files_object` (`object_hash`),
  UNIQUE KEY `uk_cpc_timeline_files_tenant` (`workspace_id`, `id`),
  KEY `idx_cpc_timeline_files_node` (`workspace_id`, `timeline_template_id`, `node_id`, `state`, `id`),
  CONSTRAINT `fk_cpc_timeline_files_node_cascade` FOREIGN KEY (`workspace_id`, `timeline_template_id`, `node_id`) REFERENCES `cpc_timeline_template_nodes` (`workspace_id`, `timeline_template_id`, `node_id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `ck_cpc_timeline_files_state` CHECK ((`state` in (_utf8mb4'pending', _utf8mb4'ready', _utf8mb4'deleting')))
) ENGINE = InnoDB COMMENT ='党建时间线节点私有附件，不保存签名和下载地址';

CREATE TABLE `cpc_timeline_template_transfers`
(
  `workspace_id` char(16) NOT NULL COMMENT '工作空间编号',
  `source_template_id` bigint unsigned NOT NULL COMMENT '来源时间线模板编号',
  `target_template_id` bigint unsigned NOT NULL COMMENT '目标时间线模板编号',
  `node_mappings` json NOT NULL COMMENT '支持多对多的节点及手动字段映射',
  `position` int NOT NULL COMMENT '从一开始的配置顺序',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC，由数据库维护',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC，由数据库维护',
  PRIMARY KEY (`workspace_id`, `source_template_id`, `target_template_id`),
  KEY `fk_cpc_timeline_transfer_target` (`workspace_id`, `target_template_id`),
  CONSTRAINT `fk_cpc_timeline_transfer_source_owned_cascade` FOREIGN KEY (`workspace_id`, `source_template_id`) REFERENCES `cpc_timeline_templates` (`workspace_id`, `id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `fk_cpc_timeline_transfer_target_cascade` FOREIGN KEY (`workspace_id`, `target_template_id`) REFERENCES `cpc_timeline_templates` (`workspace_id`, `id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `ck_cpc_timeline_transfer_mappings` CHECK ((json_type(`node_mappings`) = _utf8mb4'ARRAY'))
) ENGINE = InnoDB COMMENT ='时间线模板转移配置，不预置转移规则';

CREATE TABLE `monthly_work_records`
(
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '月度工作记录编号',
  `workspace_id` char(16) NOT NULL COMMENT '工作空间编号',
  `category` varchar(16) NOT NULL DEFAULT 'student' COMMENT '工作类别：student 学生工作，cpc 党建工作',
  `date` varchar(10) NOT NULL COMMENT '工作日期 YYYY-MM-DD，用于月份归组和上传目录',
  `work_item` varchar(100) NOT NULL COMMENT '工作项目',
  `owner_id` bigint unsigned NOT NULL COMMENT '负责人用户编号，限定月度工作的读写范围',
  `description` text NOT NULL COMMENT '详细说明，最多 10000 字符',
  `links` json NOT NULL COMMENT '多条外部链接，包含 name 和 url，保持录入顺序',
  `revision` bigint unsigned NOT NULL DEFAULT '1' COMMENT '并发修改版本',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC，由数据库维护',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC，由数据库维护',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_monthly_work_tenant` (`workspace_id`, `id`),
  KEY `idx_monthly_work_owner` (`owner_id`),
  KEY `idx_monthly_work_date` (`workspace_id`, `category`, `date` DESC, `id` DESC),
  CONSTRAINT `fk_monthly_work_owner_cascade` FOREIGN KEY (`owner_id`) REFERENCES `users` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `fk_monthly_work_workspace_cascade` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `ck_monthly_work_category` CHECK ((`category` in (_utf8mb4'student', _utf8mb4'cpc'))),
  CONSTRAINT `ck_monthly_work_links` CHECK ((json_type(`links`) = _utf8mb4'ARRAY'))
) ENGINE = InnoDB COMMENT ='学生工作与党建工作月度记录，业务与审计同事务，硬删除';

CREATE TABLE `monthly_work_attachments`
(
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '月度工作附件编号',
  `workspace_id` char(16) NOT NULL COMMENT '工作空间编号',
  `monthly_work_id` bigint unsigned NOT NULL COMMENT '所属月度工作记录编号',
  `object_hash` char(64) NOT NULL COMMENT '原始 COS 对象键的 SHA-256，用于保证对象键唯一',
  `object_key` varchar(512) NOT NULL COMMENT '私有 COS 原始对象键，包含工作日期和随机目录',
  `filename` varchar(255) NOT NULL COMMENT '原始文件名',
  `size` bigint NOT NULL COMMENT '文件字节数',
  `content_type` varchar(255) NOT NULL COMMENT 'STS 约束的声明媒体类型',
  `upload_actor` varchar(32) NOT NULL COMMENT '申请时的可信身份快照，用于确认上传',
  `state` varchar(16) NOT NULL COMMENT '存储流程状态 pending、ready、deleting',
  `revision` bigint unsigned NOT NULL DEFAULT '1' COMMENT '并发版本',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC，由数据库维护',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC，由数据库维护',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_monthly_work_attachments_object` (`object_hash`),
  KEY `idx_monthly_work_attachments_tenant` (`workspace_id`, `id`),
  KEY `idx_monthly_work_attachments_work` (`workspace_id`, `monthly_work_id`, `state`, `id`),
  CONSTRAINT `fk_monthly_work_attachments_work_cascade` FOREIGN KEY (`workspace_id`, `monthly_work_id`) REFERENCES `monthly_work_records` (`workspace_id`, `id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `ck_monthly_work_attachments_state` CHECK ((`state` in (_utf8mb4'pending', _utf8mb4'ready', _utf8mb4'deleting')))
) ENGINE = InnoDB COMMENT ='月度工作私有附件，不存储签名和 URL';

CREATE TABLE `monthly_work_participants`
(
  `workspace_id` char(16) NOT NULL COMMENT '所属工作空间编号',
  `monthly_work_id` bigint unsigned NOT NULL COMMENT '所属月度工作记录编号',
  `user_id` bigint unsigned NOT NULL COMMENT '参与人用户编号，限定月度工作的读取范围',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC，由数据库维护',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC，由数据库维护',
  PRIMARY KEY (`workspace_id`, `monthly_work_id`, `user_id`),
  KEY `idx_monthly_work_participants_user` (`workspace_id`, `user_id`, `monthly_work_id`),
  KEY `idx_monthly_work_participants_account` (`user_id`),
  CONSTRAINT `fk_monthly_work_participants_user_cascade` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `fk_monthly_work_participants_work_owned_cascade` FOREIGN KEY (`workspace_id`, `monthly_work_id`) REFERENCES `monthly_work_records` (`workspace_id`, `id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE = InnoDB COMMENT ='月度工作参与人明细，按工作空间隔离，硬删除';

CREATE TABLE `notifications`
(
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '通知编号',
  `user_id` bigint unsigned NOT NULL COMMENT '接收人账号编号',
  `source_id` varchar(100) NOT NULL COMMENT '后台事件的幂等标识，不对外开放',
  `title` varchar(200) NOT NULL COMMENT '通知标题',
  `content` text NOT NULL COMMENT '通知正文，纯文本',
  `url` varchar(2048) NOT NULL DEFAULT '' COMMENT '可选站内跳转地址，空值表示无链接',
  `read_at` datetime(6) DEFAULT NULL COMMENT '已读时间，UTC，空值表示未读',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC，由数据库维护',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC，由数据库维护',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_notifications_source` (`user_id`, `source_id`),
  KEY `idx_notifications_inbox` (`user_id`, `id`),
  KEY `idx_notifications_unread` (`user_id`, `read_at`, `id`),
  CONSTRAINT `fk_notifications_user_cascade` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE = InnoDB COMMENT ='个人站内通知，按接收人隔离，硬删除';

CREATE TABLE `email_callbacks`
(
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '回调编号',
  `event_key` char(64) NOT NULL COMMENT '规范化回调内容的 SHA-256 摘要，用于幂等去重',
  `event` varchar(32) NOT NULL COMMENT '腾讯云邮件事件代码',
  `email` varchar(254) NOT NULL COMMENT '小写收件邮箱',
  `link` text NOT NULL COMMENT '点击站点及路径，不保存用户信息、查询参数和片段',
  `bulk_id` varchar(255) NOT NULL COMMENT '腾讯云 SendEmail 返回的 MessageId',
  `timestamp` bigint NOT NULL COMMENT '事件 Unix 秒时间戳',
  `reason` text NOT NULL COMMENT '投递失败原因',
  `bounce_type` varchar(32) NOT NULL COMMENT '退信类型，非退信事件可为空',
  `username` varchar(64) NOT NULL COMMENT '腾讯云账号 AppID',
  `from` varchar(254) NOT NULL COMMENT '不含别名的发信地址',
  `from_domain` varchar(253) NOT NULL COMMENT '发信域名',
  `template_id` bigint unsigned NOT NULL COMMENT '邮件模板编号',
  `user_agent` text NOT NULL COMMENT '打开或点击邮件时的设备信息',
  `subject` text NOT NULL COMMENT '邮件主题，可使用 MIME 编码',
  `message_id` varchar(255) NOT NULL COMMENT 'SMTP Message-ID',
  `event_id` tinyint unsigned NOT NULL COMMENT '腾讯云邮件事件编号，1 至 8',
  `task_id` bigint unsigned NOT NULL COMMENT '批量发信任务编号',
  `sent_timestamp` bigint NOT NULL COMMENT '发信 Unix 秒时间戳，未提供时为 0',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_email_callbacks_event_key` (`event_key`),
  KEY `idx_email_callbacks_email_timestamp` (`email`, `timestamp`),
  KEY `idx_email_callbacks_bulk_id` (`bulk_id`)
) ENGINE = InnoDB COMMENT ='腾讯云邮件事件回调';

CREATE TABLE `email_blacklists`
(
  `email` varchar(254) NOT NULL COMMENT '小写收件邮箱',
  `event` varchar(32) NOT NULL COMMENT '首次触发拉黑的事件代码',
  `reason` text NOT NULL COMMENT '首次触发拉黑的失败原因',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC',
  PRIMARY KEY (`email`)
) ENGINE = InnoDB COMMENT ='禁止发信的邮箱黑名单';

CREATE TABLE `student_notifications`
(
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '学生通知编号',
  `workspace_id` char(16) NOT NULL COMMENT '所属工作空间',
  `student_id` bigint unsigned NOT NULL COMMENT '接收学生编号',
  `source_id` varchar(100) NOT NULL COMMENT '业务事件的幂等标识，不对外开放',
  `title` varchar(200) NOT NULL COMMENT '通知标题',
  `content` text NOT NULL COMMENT '通知正文，纯文本',
  `url` varchar(2048) NOT NULL DEFAULT '' COMMENT '可选站内跳转地址，空值表示无链接',
  `read_at` datetime(6) DEFAULT NULL COMMENT '已读时间，UTC，空值表示未读',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC，由数据库维护',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC，由数据库维护',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_student_notifications_source` (`workspace_id`, `student_id`, `source_id`),
  KEY `idx_student_notifications_inbox` (`workspace_id`, `student_id`, `id`),
  KEY `idx_student_notifications_unread` (`workspace_id`, `student_id`, `read_at`, `id`),
  CONSTRAINT `fk_student_notifications_student` FOREIGN KEY (`workspace_id`, `student_id`) REFERENCES `students` (`workspace_id`, `id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE = InnoDB COMMENT = '学生站内通知，按工作空间及接收学生隔离，硬删除';

CREATE TABLE `projects`
(
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '项目编号',
  `workspace_id` char(16) NOT NULL COMMENT '工作空间编号',
  `name` varchar(100) NOT NULL COMMENT '项目名称，同名项目按归属年份分别维护',
  `year` smallint unsigned NOT NULL COMMENT '项目归属年份，统计使用此年份',
  `description` text NOT NULL COMMENT '项目说明，最多 10000 字符',
  `revision` bigint unsigned NOT NULL DEFAULT '1' COMMENT '并发版本',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC，由数据库维护',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC，由数据库维护',
  `allow_self_apply` boolean NOT NULL DEFAULT FALSE COMMENT '是否允许在籍学生自行申报',
  `self_apply_deadline` datetime(6) DEFAULT NULL COMMENT '自行申报截止时间，UTC；开放申报时必填，截止后自动提交草稿',
  `score_calculation` json DEFAULT NULL COMMENT '申报公式总分计算及年级、专业、班级组合排名配置',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_projects_tenant_id` (`workspace_id`, `id`),
  UNIQUE KEY `uk_projects_name_year` (`workspace_id`, `name`, `year`),
  CONSTRAINT `fk_projects_workspace_cascade` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `ck_projects_year` CHECK ((`year` between 1900 and 9999))
) ENGINE = InnoDB COMMENT ='学生工作评奖评优年度项目';

CREATE TABLE `project_detail_options`
(
  `workspace_id` char(16) NOT NULL COMMENT '工作空间编号',
  `project_id` bigint unsigned NOT NULL COMMENT '年度项目编号',
  `name` varchar(100) NOT NULL COMMENT '评分可选子项名称，最多 100 字符',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC，由数据库维护',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC，由数据库维护',
  PRIMARY KEY (`workspace_id`, `project_id`, `name`),
  CONSTRAINT `fk_project_detail_options_project_owned_cascade` FOREIGN KEY (`workspace_id`, `project_id`) REFERENCES `projects` (`workspace_id`, `id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE = InnoDB COMMENT = '项目评分子项配置，参与评分只能选择已配置名称';

CREATE TABLE `project_attachments`
(
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '项目附件编号',
  `workspace_id` char(16) NOT NULL COMMENT '工作空间编号',
  `project_id` bigint unsigned NOT NULL COMMENT '所属项目编号',
  `object_hash` char(64) NOT NULL COMMENT '对象键的 SHA-256',
  `object_key` varchar(512) NOT NULL COMMENT '私有 COS 原始对象键',
  `filename` varchar(255) NOT NULL COMMENT '原始文件名',
  `size` bigint NOT NULL COMMENT '文件字节数',
  `content_type` varchar(255) NOT NULL COMMENT 'STS 约束的声明媒体类型',
  `upload_actor` varchar(32) NOT NULL COMMENT '申请时的可信身份快照，用于确认上传',
  `state` varchar(16) NOT NULL COMMENT '存储流程状态 pending、ready、deleting',
  `revision` bigint unsigned NOT NULL DEFAULT '1' COMMENT '并发版本',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC，由数据库维护',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC，由数据库维护',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_project_attachments_object` (`object_hash`),
  KEY `idx_project_attachments_tenant` (`workspace_id`, `id`),
  KEY `idx_project_attachments_project` (`workspace_id`, `project_id`, `state`, `id`),
  CONSTRAINT `fk_project_attachments_project_cascade` FOREIGN KEY (`workspace_id`, `project_id`) REFERENCES `projects` (`workspace_id`, `id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `ck_project_attachments_state` CHECK ((`state` in (_utf8mb4'pending', _utf8mb4'ready', _utf8mb4'deleting')))
) ENGINE = InnoDB COMMENT ='项目私有附件，不存储签名和 URL';

CREATE TABLE `project_participations`
(
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '参与记录编号',
  `workspace_id` char(16) NOT NULL COMMENT '工作空间编号',
  `project_id` bigint unsigned NOT NULL COMMENT '项目编号，继承项目归属年份',
  `student_id` bigint unsigned NOT NULL COMMENT '学生编号',
  `rank` int DEFAULT NULL COMMENT '独立填写的学生排名，正整数，NULL 为未填写',
  `score` double DEFAULT NULL COMMENT '独立填写的双精度总分，NULL 为未填写，不汇总子项',
  `revision` bigint unsigned NOT NULL DEFAULT '1' COMMENT '总分和子项共用并发版本',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC，由数据库维护',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC，由数据库维护',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_project_participations_tenant` (`workspace_id`, `id`),
  UNIQUE KEY `uk_project_participations_student` (`workspace_id`, `project_id`, `student_id`),
  UNIQUE KEY `uk_project_participations_project` (`workspace_id`, `id`, `project_id`),
  KEY `idx_project_participations_student` (`workspace_id`, `student_id`),
  KEY `idx_project_participations_score` (`workspace_id`, `project_id`, `score` DESC, `id`),
  CONSTRAINT `fk_project_participations_project_cascade` FOREIGN KEY (`workspace_id`, `project_id`) REFERENCES `projects` (`workspace_id`, `id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `fk_project_participations_student_cascade` FOREIGN KEY (`workspace_id`, `student_id`) REFERENCES `students` (`workspace_id`, `id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `ck_project_participations_rank` CHECK (`rank` IS NULL OR `rank` > 0)
) ENGINE = InnoDB COMMENT ='学生项目参与及独立评分，硬删除后保留审计及身份快照';

CREATE TABLE `project_participation_details`
(
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '评分子项编号',
  `workspace_id` char(16) NOT NULL COMMENT '工作空间编号',
  `participation_id` bigint unsigned NOT NULL COMMENT '参与记录编号',
  `project_id` bigint unsigned NOT NULL COMMENT '所属年度项目编号',
  `sort_order` int unsigned NOT NULL COMMENT '子项录入顺序，从 0 开始',
  `name` varchar(100) NOT NULL COMMENT '只能选择所属项目配置的子项名称',
  `description` text NOT NULL COMMENT '子项说明，最多 10000 字符',
  `score` double DEFAULT NULL COMMENT '独立填写的双精度子项分数，NULL 表示未填写',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC，由数据库维护',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC，由数据库维护',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_project_details_position` (`workspace_id`, `participation_id`, `sort_order`),
  KEY `idx_project_details_name` (`workspace_id`, `name`),
  KEY `fk_project_details_project_participation` (`workspace_id`, `participation_id`, `project_id`),
  UNIQUE KEY `uk_participation_detail_name` (`workspace_id`, `participation_id`, `name`),
  CONSTRAINT `fk_project_details_participation_owned_cascade` FOREIGN KEY (`workspace_id`, `participation_id`, `project_id`) REFERENCES `project_participations` (`workspace_id`, `id`, `project_id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `fk_project_details_option_cascade` FOREIGN KEY (`workspace_id`, `project_id`, `name`) REFERENCES `project_detail_options` (`workspace_id`, `project_id`, `name`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE = InnoDB COMMENT ='独立评分子项表，与参与总分无必然联系，不自动累加';

CREATE TABLE `project_approval_steps`
(
  `workspace_id` char(16) NOT NULL COMMENT '所属工作空间',
  `project_id` bigint unsigned NOT NULL COMMENT '所属项目',
  `position` int unsigned NOT NULL COMMENT '审批顺序，从一级开始',
  `target_order` int unsigned NOT NULL COMMENT '同级处理人顺序，从零开始',
  `sign_mode` varchar(8) NOT NULL COMMENT 'any 或签、all 并签',
  `allow_edit` tinyint(1) NOT NULL DEFAULT 1 COMMENT '本级处理人是否允许编辑申报内容及附件',
  `kind` varchar(32) NOT NULL COMMENT 'applicant、class_position、student、user',
  `class_position` varchar(64) NOT NULL DEFAULT '' COMMENT '数据库枚举的独立班委职务',
  `student_id` bigint unsigned DEFAULT NULL COMMENT '指定学生编号',
  `user_id` bigint unsigned DEFAULT NULL COMMENT '指定组织账号编号',
  `reviewer_label` varchar(200) NOT NULL COMMENT '处理人身份显示快照',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC',
  PRIMARY KEY (`workspace_id`, `project_id`, `position`, `target_order`),
  KEY `idx_project_approval_student` (`workspace_id`, `student_id`),
  KEY `idx_project_approval_user` (`user_id`),
  CONSTRAINT `fk_project_approval_project` FOREIGN KEY (`workspace_id`, `project_id`) REFERENCES `projects` (`workspace_id`, `id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `fk_project_approval_student` FOREIGN KEY (`workspace_id`, `student_id`) REFERENCES `students` (`workspace_id`, `id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `fk_project_approval_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `ck_project_approval_kind` CHECK (`kind` IN ('applicant', 'class_position', 'student', 'user')),
  CONSTRAINT `ck_project_approval_sign_mode` CHECK (`sign_mode` IN ('any', 'all')),
  CONSTRAINT `ck_project_approval_allow_edit` CHECK (`allow_edit` IN (0, 1))
) ENGINE = InnoDB COMMENT = '项目的多级审批配置';

CREATE TABLE `project_applications`
(
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '申报编号',
  `workspace_id` char(16) NOT NULL COMMENT '所属工作空间',
  `project_id` bigint unsigned NOT NULL COMMENT '所属项目',
  `student_id` bigint unsigned NOT NULL COMMENT '申报学生',
  `statement` text NOT NULL COMMENT '申报说明，可留空，最多 10000 字符',
  `rank` int DEFAULT NULL COMMENT '申报排名，启用总分计算后由后台重排已通过申报',
  `score` double DEFAULT NULL COMMENT '申报总分，启用总分计算后仅由公式计算，排名时空值按 0 分',
  `status` varchar(16) NOT NULL COMMENT 'draft、pending、rejected、approved',
  `current_step` int unsigned NOT NULL COMMENT '当前节点，零为申报人',
  `revision` bigint unsigned NOT NULL DEFAULT 1 COMMENT '并发版本',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_project_application_tenant` (`workspace_id`, `id`),
  UNIQUE KEY `uk_project_application_project` (`workspace_id`, `id`, `project_id`),
  UNIQUE KEY `uk_project_application_student` (`workspace_id`, `project_id`, `student_id`),
  KEY `idx_project_application_status` (`workspace_id`, `status`, `id`),
  KEY `idx_project_application_mine` (`workspace_id`, `student_id`, `id`),
  KEY `idx_project_application_workflow` (`workspace_id`, `project_id`, `status`, `id`),
  CONSTRAINT `fk_project_application_project` FOREIGN KEY (`workspace_id`, `project_id`) REFERENCES `projects` (`workspace_id`, `id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `fk_project_application_student` FOREIGN KEY (`workspace_id`, `student_id`) REFERENCES `students` (`workspace_id`, `id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `ck_project_application_status` CHECK (`status` IN ('draft', 'pending', 'rejected', 'approved')),
  CONSTRAINT `ck_project_application_rank` CHECK (`rank` IS NULL OR `rank` > 0)
) ENGINE = InnoDB COMMENT = '学生评奖评优申报单';

CREATE TABLE `project_application_nodes`
(
  `workspace_id` char(16) NOT NULL COMMENT '所属工作空间',
  `application_id` bigint unsigned NOT NULL COMMENT '所属申报单',
  `position` int unsigned NOT NULL COMMENT '节点顺序，零为申报人',
  `target_order` int unsigned NOT NULL COMMENT '同级处理人顺序',
  `sign_mode` varchar(8) NOT NULL COMMENT 'any 或签、all 并签',
  `allow_edit` tinyint(1) NOT NULL DEFAULT 1 COMMENT '本级处理人是否允许编辑申报内容及附件',
  `kind` varchar(32) NOT NULL COMMENT 'applicant、class_position、student、user',
  `class_position` varchar(64) NOT NULL DEFAULT '' COMMENT '数据库枚举的独立班委职务',
  `student_id` bigint unsigned DEFAULT NULL COMMENT '指定学生编号',
  `user_id` bigint unsigned DEFAULT NULL COMMENT '指定组织账号编号',
  `reviewer_label` varchar(200) NOT NULL COMMENT '当前审批轮次的处理人身份显示快照',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC',
  PRIMARY KEY (`workspace_id`, `application_id`, `position`, `target_order`),
  KEY `idx_project_application_node_student` (`workspace_id`, `student_id`),
  KEY `idx_project_application_node_user` (`user_id`, `workspace_id`, `application_id`),
  CONSTRAINT `fk_project_application_node_application` FOREIGN KEY (`workspace_id`, `application_id`) REFERENCES `project_applications` (`workspace_id`, `id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `fk_project_application_node_student` FOREIGN KEY (`workspace_id`, `student_id`) REFERENCES `students` (`workspace_id`, `id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `fk_project_application_node_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `ck_project_application_node_kind` CHECK (`kind` IN ('applicant', 'class_position', 'student', 'user')),
  CONSTRAINT `ck_application_node_sign_mode` CHECK (`sign_mode` IN ('any', 'all')),
  CONSTRAINT `ck_application_node_allow_edit` CHECK (`allow_edit` IN (0, 1))
) ENGINE = InnoDB COMMENT = '申报审批流，未完成申报随项目流程变更重置，已通过申报保留最终流程';

CREATE TABLE `project_application_details`
(
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '申报子项编号',
  `workspace_id` char(16) NOT NULL COMMENT '所属工作空间',
  `application_id` bigint unsigned NOT NULL COMMENT '所属申报单',
  `project_id` bigint unsigned NOT NULL COMMENT '所属项目',
  `sort_order` int unsigned NOT NULL COMMENT '子项顺序，从零开始',
  `detail_key` char(32) NOT NULL COMMENT '客户端子项稳定标识',
  `name` varchar(100) NOT NULL COMMENT '项目配置的子项名称',
  `description` text NOT NULL COMMENT '子项说明',
  `score` double DEFAULT NULL COMMENT '独立子项分数',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_project_application_detail_tenant` (`workspace_id`, `id`),
  UNIQUE KEY `uk_application_detail_owner` (`workspace_id`, `application_id`, `id`),
  UNIQUE KEY `uk_project_application_detail_key` (`workspace_id`, `application_id`, `detail_key`),
  KEY `idx_project_application_detail_order` (`workspace_id`, `application_id`, `sort_order`),
  KEY `idx_project_application_detail_option` (`workspace_id`, `project_id`, `name`),
  UNIQUE KEY `uk_application_detail_name` (`workspace_id`, `application_id`, `name`),
  CONSTRAINT `fk_project_application_detail_application` FOREIGN KEY (`workspace_id`, `application_id`, `project_id`) REFERENCES `project_applications` (`workspace_id`, `id`, `project_id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `fk_project_application_detail_option` FOREIGN KEY (`workspace_id`, `project_id`, `name`) REFERENCES `project_detail_options` (`workspace_id`, `project_id`, `name`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE = InnoDB COMMENT = '申报评分子项，审批通过后保留，不自动生成获奖记录';

CREATE TABLE `project_application_history`
(
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '流转记录编号',
  `workspace_id` char(16) NOT NULL COMMENT '所属工作空间',
  `application_id` bigint unsigned NOT NULL COMMENT '所属申报单',
  `step` int unsigned NOT NULL COMMENT '操作时节点顺序',
  `operation` varchar(16) NOT NULL COMMENT 'draft、submit、update、approve、reject、reset',
  `actor_type` varchar(16) NOT NULL COMMENT 'student、user 或 system',
  `actor_id` bigint unsigned NOT NULL COMMENT '历史身份编号快照，不建立物理外键',
  `actor_name` varchar(200) NOT NULL COMMENT '历史身份名称快照',
  `comment` text NOT NULL COMMENT '审批说明，通过选填、驳回必填，最多 10000 字符',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '操作时间，UTC',
  PRIMARY KEY (`id`),
  KEY `idx_project_application_history` (`workspace_id`, `application_id`, `id`),
  CONSTRAINT `fk_project_application_history_application` FOREIGN KEY (`workspace_id`, `application_id`) REFERENCES `project_applications` (`workspace_id`, `id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `ck_project_application_history_operation` CHECK (`operation` IN ('draft', 'submit', 'update', 'approve', 'reject', 'reset')),
  CONSTRAINT `ck_project_application_history_actor` CHECK ((`actor_type` = 'system' AND `actor_id` = 0) OR (`actor_type` IN ('student', 'user') AND `actor_id` > 0))
) ENGINE = InnoDB COMMENT = '仅追加的申报流转历史，完整内容变更由操作审计留存';

CREATE TABLE `project_application_approvals`
(
  `workspace_id` char(16) NOT NULL COMMENT '工作空间编号',
  `application_id` bigint unsigned NOT NULL COMMENT '申报编号',
  `position` int unsigned NOT NULL COMMENT '审批级别',
  `actor_type` varchar(16) NOT NULL COMMENT 'student 或 user',
  `actor_id` bigint unsigned NOT NULL COMMENT '对应身份类型的编号',
  `student_id` bigint unsigned DEFAULT NULL COMMENT '学生编号',
  `user_id` bigint unsigned DEFAULT NULL COMMENT '组织用户编号',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC',
  PRIMARY KEY (`workspace_id`, `application_id`, `position`, `actor_type`, `actor_id`),
  KEY `idx_application_approval_student` (`workspace_id`, `student_id`),
  KEY `idx_application_approval_user` (`user_id`),
  CONSTRAINT `fk_application_approval_application` FOREIGN KEY (`workspace_id`, `application_id`) REFERENCES `project_applications` (`workspace_id`, `id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `fk_application_approval_student` FOREIGN KEY (`workspace_id`, `student_id`) REFERENCES `students` (`workspace_id`, `id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `fk_application_approval_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `ck_application_approval_actor` CHECK (`actor_type` IN ('student', 'user'))
) ENGINE = InnoDB COMMENT = '当前轮次有效的审批签署，修改及驳回后清空并保留审计';

CREATE TABLE `project_application_detail_attachments`
(
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '附件编号',
  `workspace_id` char(16) NOT NULL COMMENT '工作空间编号',
  `application_id` bigint unsigned NOT NULL COMMENT '申报编号',
  `detail_id` bigint unsigned NOT NULL COMMENT '申报评分子项编号',
  `object_hash` char(64) NOT NULL COMMENT '私有对象键摘要',
  `object_key` varchar(512) NOT NULL COMMENT '私有 COS 对象键',
  `filename` varchar(255) NOT NULL COMMENT '原始文件名',
  `size` bigint NOT NULL COMMENT '文件大小',
  `content_type` varchar(255) NOT NULL COMMENT 'STS 约束的声明媒体类型',
  `upload_actor` varchar(32) NOT NULL COMMENT '申请时的可信身份快照，用于确认上传',
  `state` varchar(16) NOT NULL COMMENT 'pending、ready、deleting',
  `revision` bigint unsigned NOT NULL DEFAULT 1 COMMENT '并发版本',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_application_detail_attachment_object` (`object_hash`),
  KEY `idx_application_detail_attachment` (`workspace_id`, `detail_id`, `state`, `id`),
  CONSTRAINT `fk_application_attachment_application` FOREIGN KEY (`workspace_id`, `application_id`) REFERENCES `project_applications` (`workspace_id`, `id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `fk_application_attachment_detail` FOREIGN KEY (`workspace_id`, `application_id`, `detail_id`) REFERENCES `project_application_details` (`workspace_id`, `application_id`, `id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `ck_application_attachment_state` CHECK (`state` IN ('pending', 'ready', 'deleting'))
) ENGINE = InnoDB COMMENT = '仅申报保留的子项私有附件，不转入获奖记录';

CREATE TABLE `student_off_campus_details`
(
  `workspace_id` char(16) NOT NULL COMMENT '工作空间编号',
  `student_id` bigint unsigned NOT NULL COMMENT '学生记录编号',
  `reason` varchar(1000) NOT NULL COMMENT '校外住宿原因',
  `start_date` date DEFAULT NULL COMMENT '校外住宿开始日期',
  `end_date` date DEFAULT NULL COMMENT '校外住宿结束日期',
  `category` varchar(64) NOT NULL COMMENT '校外住宿类别',
  `rental_method` varchar(64) NOT NULL COMMENT '租住方式',
  `landlord_name` varchar(50) NOT NULL COMMENT '房东姓名',
  `landlord_phone` varchar(32) NOT NULL COMMENT '房东联系电话',
  `roommate_name` varchar(50) NOT NULL COMMENT '合租人姓名',
  `roommate_unit` varchar(100) NOT NULL COMMENT '合租人单位',
  `roommate_phone` varchar(32) NOT NULL COMMENT '合租人联系电话',
  `address` varchar(255) NOT NULL COMMENT '校外居住地详细地址',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC，由数据库维护',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC，由数据库维护',
  PRIMARY KEY (`workspace_id`, `student_id`),
  CONSTRAINT `fk_student_off_campus_student_owned_cascade` FOREIGN KEY (`workspace_id`, `student_id`) REFERENCES `students` (`workspace_id`, `id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `ck_student_off_campus_dates` CHECK ((`start_date` <= `end_date`))
) ENGINE = InnoDB COMMENT ='每名校外住宿学生的独立住宿明细';

CREATE TABLE `user_bootstrap`
(
  `id` tinyint unsigned NOT NULL COMMENT '固定为 1 的初始化锁',
  `initialized` tinyint(1) NOT NULL DEFAULT '0' COMMENT '管理员初始化是否已完成',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC，由数据库维护',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC，由数据库维护',
  PRIMARY KEY (`id`),
  CONSTRAINT `ck_user_bootstrap_singleton` CHECK ((`id` = 1))
) ENGINE = InnoDB COMMENT ='管理员一次性初始化状态';

INSERT INTO user_bootstrap (id, initialized)
VALUES (1, FALSE);

CREATE TABLE `user_sessions`
(
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '会话业务编号，不用于认证',
  `token_hash` char(64) NOT NULL COMMENT '随机会话的 SHA-256 摘要',
  `user_id` bigint unsigned NOT NULL COMMENT '所属账号编号',
  `session_version` bigint unsigned NOT NULL DEFAULT '1' COMMENT '登录时的账号会话版本',
  `expires_at` datetime(6) NOT NULL COMMENT '绝对过期时间，UTC',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC，由数据库维护',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC，由数据库维护',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_sessions_token_hash` (`token_hash`),
  KEY `idx_user_sessions_user` (`user_id`),
  KEY `idx_user_sessions_expiry` (`expires_at`),
  CONSTRAINT `fk_user_sessions_user_cascade` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE = InnoDB COMMENT = '仅保存会话摘要，账号硬删除时级联清理';

CREATE TABLE `student_sessions`
(
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '学生会话业务编号',
  `token_hash` char(64) NOT NULL COMMENT '学生会话的 SHA-256 摘要',
  `workspace_id` char(16) NOT NULL COMMENT '绑定的工作空间编号',
  `student_id` bigint unsigned NOT NULL COMMENT '绑定的学生编号',
  `student_no` varchar(32) NOT NULL COMMENT '登录时的学号',
  `email` varchar(254) NOT NULL COMMENT '登录时的登记邮箱',
  `expires_at` datetime(6) NOT NULL COMMENT '绝对过期时间，UTC',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) COMMENT '创建时间，UTC，由数据库维护',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6) COMMENT '更新时间，UTC，由数据库维护',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_student_sessions_hash` (`token_hash`),
  KEY `idx_student_sessions_student` (`workspace_id`, `student_id`),
  KEY `idx_student_sessions_expiry` (`expires_at`),
  CONSTRAINT `fk_student_sessions_student_cascade` FOREIGN KEY (`workspace_id`, `student_id`) REFERENCES `students` (`workspace_id`, `id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE = InnoDB COMMENT ='绑定工作空间的学生登录会话，与管理账号隔离';

-- 标准业务审计不建立物理外键，业务硬删除后继续保留身份与业务快照。
CREATE TABLE `audit_events`
(
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '数据库分配的排序编号',
  `event_date` date NOT NULL COMMENT 'start_time 所属 UTC 日期',
  `event_id` char(32) NOT NULL COMMENT '随机审计事件编号',
  `workspace_id` varchar(16) NOT NULL COMMENT '操作所属工作空间，系统操作为 system',
  `event_content` text COMMENT '事件中文说明',
  `action_id` varchar(64) NOT NULL COMMENT '英文操作标识',
  `request_id` char(32) NOT NULL COMMENT '当前事件的 OpenTelemetry Trace ID',
  `user_id` bigint unsigned NOT NULL COMMENT '可信用户编号，组织为账号编号、学生为学生编号',
  `user_identify_type` tinyint unsigned NOT NULL COMMENT '身份类型，1 组织、2 学生',
  `user_identify_tenant_id` varchar(16) NOT NULL COMMENT '组织为 system，学生为所属空间编号',
  `start_time` bigint NOT NULL COMMENT '开始时间，UTC Unix 毫秒，用于计算分区日期',
  `end_time` bigint NOT NULL COMMENT '审计生成时间，UTC Unix 毫秒',
  `access_type` tinyint unsigned NOT NULL COMMENT '访问来源，1 web、2 mcp',
  `access_source_ip` varchar(45) NOT NULL COMMENT '客户端 IP',
  `access_user_agent` varchar(512) NOT NULL COMMENT '客户端 User Agent',
  `resource_type_id` varchar(64) NOT NULL COMMENT '业务对象类型',
  `instance_id` varchar(128) NOT NULL COMMENT '业务对象编号，保留字符串语义',
  `instance_name` varchar(255) DEFAULT NULL COMMENT '实例安全名称',
  `instance_data` json NOT NULL COMMENT '变更后完整普通快照，未涉及为 JSON null',
  `instance_origin_data` json NOT NULL COMMENT '变更前完整普通快照，未涉及为 JSON null',
  `result_code` int NOT NULL COMMENT '成功为 0，失败为数字 HTTP 状态或业务错误码',
  `result_content` varchar(255) NOT NULL COMMENT '可公开错误说明，成功时为空',
  `snapshot_user_info` json NOT NULL COMMENT '完整操作者身份快照',
  `extend_data` json NOT NULL COMMENT '仅保留 method 和 path',
  `sensitive_snapshot` longblob COMMENT 'AES-GCM 敏感快照密文，普通接口不可返回',
  `sensitive_key_id` varchar(64) NOT NULL COMMENT '敏感快照密钥编号，普通接口不可返回',
  `sensitive_context` json DEFAULT NULL COMMENT '敏感快照加密关联上下文六元组，私有且不可返回',
  `is_login_event` tinyint unsigned GENERATED ALWAYS AS (IFNULL(workspace_id = 'system' AND action_id = 'user.login' AND result_code = 0 AND resource_type_id = 'user_session' AND JSON_TYPE(instance_data) <> 'NULL', 0)) STORED COMMENT '是否为组织账号成功登录事件',
  PRIMARY KEY (`id`, `event_date`),
  UNIQUE KEY `uk_audit_events_event` (`event_id`, `event_date`),
  KEY `idx_audit_events_list` (`id` DESC, `start_time`, `user_identify_tenant_id`, `user_id`, `workspace_id`, `action_id`, `result_code`, `access_type`),
  KEY `idx_audit_events_filter_time` (`start_time`, `user_identify_tenant_id`, `user_id`, `workspace_id`, `action_id`, `result_code`, `access_type`),
  KEY `idx_audit_events_user` (`user_id`, `user_identify_tenant_id`, `start_time`, `id`, `workspace_id`, `action_id`, `result_code`, `access_type`),
  KEY `idx_audit_events_workspace` (`workspace_id`, `start_time`, `id`, `user_identify_tenant_id`, `user_id`, `action_id`, `result_code`, `access_type`),
  KEY `idx_audit_events_action` (`action_id`, `start_time`, `id`, `user_identify_tenant_id`, `user_id`, `workspace_id`, `result_code`, `access_type`),
  KEY `idx_audit_events_instance` (`instance_id`, `start_time`, `id`, `user_identify_tenant_id`, `user_id`, `workspace_id`, `action_id`, `result_code`, `access_type`, `request_id`),
  KEY `idx_audit_events_request` (`request_id`, `start_time`, `id`, `user_identify_tenant_id`, `user_id`, `workspace_id`, `action_id`, `result_code`, `access_type`, `instance_id`),
  KEY `idx_audit_events_login` (`is_login_event`, `user_id`, `end_time`),
  KEY `idx_audit_events_login_time` (`is_login_event`, `end_time`, `user_id`),
  CONSTRAINT `chk_audit_events_identity` CHECK (`user_identify_type` IN (1, 2)),
  CONSTRAINT `chk_audit_events_access` CHECK (`access_type` IN (1, 2)),
  CONSTRAINT `chk_audit_events_time` CHECK (`start_time` <= `end_time`)
) ENGINE = InnoDB ROW_FORMAT = DYNAMIC COMMENT ='标准业务审计，按开始时间的 UTC 日期分区';

-- 初始化从 UTC 当天起的 30 个每日分区，不设置兜底分区或删除历史。
WITH RECURSIVE partition_days AS (
  SELECT UTC_DATE() AS event_date, 0 AS day_offset
  UNION ALL
  SELECT DATE_ADD(event_date, INTERVAL 1 DAY), day_offset + 1
  FROM partition_days
  WHERE day_offset < 29
)
SELECT GROUP_CONCAT(
  CONCAT(
    'PARTITION p',
    DATE_FORMAT(event_date, '%Y%m%d'),
    ' VALUES IN (''',
    DATE_FORMAT(event_date, '%Y-%m-%d'),
    ''')'
  )
  ORDER BY event_date
  SEPARATOR ','
)
INTO @audit_events_partitions
FROM partition_days;

SET @audit_events_partition_ddl = CONCAT(
  'ALTER TABLE audit_events PARTITION BY LIST COLUMNS(event_date) (',
  @audit_events_partitions,
  ')'
);
PREPARE audit_events_partition_statement FROM @audit_events_partition_ddl;
EXECUTE audit_events_partition_statement;
DEALLOCATE PREPARE audit_events_partition_statement;

-- 分区仅由维护过程和数据库 Event 创建，应用运行账号仅需 DML。
DELIMITER $$
CREATE PROCEDURE maintain_audit_events_partitions()
SQL SECURITY DEFINER
BEGIN
  DECLARE partition_day DATE;
  DECLARE expected_name VARCHAR(16);
  DECLARE description_value TEXT;
  DECLARE existing_name INT DEFAULT 0;
  DECLARE existing_day INT DEFAULT 0;
  DECLARE operation_lock INT DEFAULT 0;
  DECLARE statement_prepared BOOLEAN DEFAULT FALSE;
  DECLARE lock_name VARCHAR(64);
  DECLARE EXIT HANDLER FOR SQLEXCEPTION
  BEGIN
    IF statement_prepared THEN
      DEALLOCATE PREPARE audit_events_add_partition_statement;
    END IF;
    IF operation_lock = 1 THEN
      DO RELEASE_LOCK(lock_name);
    END IF;
    RESIGNAL;
  END;

  SET lock_name = CONCAT(LEFT(SHA2(DATABASE(), 256), 32), ':audit_events');
  SET operation_lock = GET_LOCK(lock_name, 5);
  IF operation_lock IS NULL OR operation_lock <> 1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = '无法取得审计表分区维护锁';
  END IF;

  SET partition_day = UTC_DATE();
  WHILE partition_day < DATE_ADD(UTC_DATE(), INTERVAL 30 DAY) DO
    SET expected_name = CONCAT('p', DATE_FORMAT(partition_day, '%Y%m%d'));
    SELECT COUNT(*), MAX(PARTITION_DESCRIPTION)
    INTO existing_name, description_value
    FROM information_schema.PARTITIONS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'audit_events'
      AND PARTITION_NAME = expected_name;

    IF existing_name > 0
       AND description_value <> CONCAT('''', DATE_FORMAT(partition_day, '%Y-%m-%d'), '''') THEN
      SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = '审计表存在同名但日期不一致的分区';
    END IF;

    IF existing_name = 0 THEN
      SELECT COUNT(*)
      INTO existing_day
      FROM information_schema.PARTITIONS
      WHERE TABLE_SCHEMA = DATABASE()
        AND TABLE_NAME = 'audit_events'
        AND PARTITION_DESCRIPTION =
          CONCAT('''', DATE_FORMAT(partition_day, '%Y-%m-%d'), '''');
      IF existing_day > 0 THEN
        SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = '审计表存在日期相同但名称不一致的分区';
      END IF;

      SET @audit_events_add_partition_ddl = CONCAT(
        'ALTER TABLE audit_events ADD PARTITION (PARTITION ',
        expected_name,
        ' VALUES IN (''',
        DATE_FORMAT(partition_day, '%Y-%m-%d'),
        '''))'
      );
      PREPARE audit_events_add_partition_statement FROM @audit_events_add_partition_ddl;
      SET statement_prepared = TRUE;
      EXECUTE audit_events_add_partition_statement;
      DEALLOCATE PREPARE audit_events_add_partition_statement;
      SET statement_prepared = FALSE;
    END IF;
    SET partition_day = DATE_ADD(partition_day, INTERVAL 1 DAY);
  END WHILE;
  DO RELEASE_LOCK(lock_name);
END$$
DELIMITER ;

-- Event 的调度时区由上述 SESSION time_zone 保持为 UTC。
CREATE EVENT maintain_audit_events_partitions_daily
ON SCHEDULE EVERY 1 DAY
STARTS TIMESTAMP(UTC_DATE() + INTERVAL 1 DAY, '00:05:00')
ON COMPLETION PRESERVE ENABLE
DO CALL maintain_audit_events_partitions();
