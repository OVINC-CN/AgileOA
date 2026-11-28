import { emptyValueLabel, formatValue } from '@/lib/format';

export interface StudentField {
  key: string;
  label: string;
  labelInSection?: string;
  fullWidthInForm?: boolean;
  group: 'basic' | 'sensitive';
  maxLength: number;
  required?: boolean;
  type?: 'month';
  enumerable: boolean;
  options: { value: string; label: string }[];
}

export const sensitiveAction = 'student.sensitive.handle';
export const politicalStatusLabel = '入学政治面貌';

export const studentFields: StudentField[] = [
  {
    key: 'student_no',
    required: true,
    label: '学号',
    group: 'basic',
    maxLength: 32,
    enumerable: false,
    options: [],
  },
  {
    key: 'name',
    required: true,
    label: '姓名',
    group: 'basic',
    maxLength: 50,
    enumerable: false,
    options: [],
  },
  {
    key: 'grade',
    required: true,
    label: '年级',
    group: 'basic',
    maxLength: 16,
    enumerable: true,
    options: [],
  },
  {
    key: 'status',
    required: true,
    label: '状态',
    group: 'basic',
    maxLength: 8,
    enumerable: true,
    options: [],
  },
  {
    key: 'education_level',
    required: true,
    label: '学历',
    group: 'basic',
    maxLength: 32,
    enumerable: true,
    options: [],
  },
  {
    key: 'major',
    required: true,
    label: '专业',
    group: 'basic',
    maxLength: 100,
    enumerable: true,
    options: [],
  },
  {
    key: 'class_name',
    required: true,
    label: '班级',
    group: 'basic',
    maxLength: 64,
    enumerable: true,
    options: [],
  },
  {
    key: 'class_position',
    label: '班内任职',
    group: 'basic',
    maxLength: 64,
    enumerable: true,
    options: [],
  },
  {
    key: 'admission_type',
    required: true,
    label: '录取类型',
    group: 'basic',
    maxLength: 32,
    enumerable: true,
    options: [],
  },
  {
    key: 'arrival_month',
    label: '来校时间',
    group: 'basic',
    maxLength: 7,
    type: 'month',
    enumerable: false,
    options: [],
  },
  {
    key: 'previous_school_or_employer',
    label: '毕（肄）业学校及单位',
    group: 'basic',
    maxLength: 255,
    enumerable: false,
    options: [],
  },
  {
    key: 'previous_graduation_month',
    label: '毕（肄）业时间',
    group: 'basic',
    maxLength: 7,
    type: 'month',
    enumerable: false,
    options: [],
  },
  {
    key: 'advisor',
    label: '导师姓名',
    group: 'basic',
    maxLength: 50,
    enumerable: true,
    options: [],
  },
  {
    key: 'advisor_no',
    label: '导师工号',
    group: 'basic',
    maxLength: 32,
    enumerable: false,
    options: [],
  },
  {
    key: 'advisor_phone',
    label: '导师电话',
    group: 'basic',
    maxLength: 32,
    enumerable: false,
    options: [],
  },
  {
    key: 'identity_number',
    required: true,
    label: '身份证号码',
    group: 'sensitive',
    maxLength: 18,
    enumerable: false,
    options: [],
  },
  {
    key: 'birth_date',
    label: '出生日期',
    group: 'basic',
    maxLength: 10,
    enumerable: false,
    options: [],
  },
  {
    key: 'gender',
    required: true,
    label: '性别',
    group: 'basic',
    maxLength: 16,
    enumerable: true,
    options: [],
  },
  {
    key: 'ethnicity',
    required: true,
    label: '民族',
    group: 'basic',
    maxLength: 32,
    enumerable: true,
    options: [],
  },
  {
    key: 'political_status',
    label: politicalStatusLabel,
    group: 'basic',
    maxLength: 32,
    enumerable: true,
    options: [],
  },
  {
    key: 'phone',
    label: '电话号码',
    group: 'basic',
    maxLength: 32,
    enumerable: false,
    options: [],
  },
  { key: 'qq', label: 'QQ号码', group: 'basic', maxLength: 20, enumerable: false, options: [] },
  { key: 'email', label: '邮箱', group: 'basic', maxLength: 254, enumerable: false, options: [] },
  {
    key: 'accommodation_type',
    required: true,
    label: '住宿类型',
    group: 'basic',
    maxLength: 16,
    enumerable: true,
    options: [
      { value: 'unknown', label: '未知' },
      { value: 'on_campus', label: '校内' },
      { value: 'off_campus', label: '校外' },
      { value: 'joint_abroad', label: '出国联培' },
    ],
  },
  {
    key: 'campus',
    label: '住宿园区',
    group: 'basic',
    maxLength: 64,
    enumerable: true,
    options: [],
  },
  {
    key: 'building',
    label: '宿舍楼',
    group: 'basic',
    maxLength: 32,
    enumerable: true,
    options: [],
  },
  { key: 'room', label: '宿舍号', group: 'basic', maxLength: 32, enumerable: true, options: [] },
  { key: 'bed', label: '床位号', group: 'basic', maxLength: 16, enumerable: false, options: [] },
  {
    key: 'origin',
    label: '户籍所在地（省市）',
    group: 'basic',
    maxLength: 128,
    enumerable: false,
    options: [],
  },
  {
    key: 'native_place',
    label: '籍贯',
    group: 'basic',
    maxLength: 128,
    enumerable: false,
    options: [],
  },
  {
    key: 'home_address',
    label: '家庭详细住址',
    group: 'sensitive',
    maxLength: 255,
    enumerable: false,
    options: [],
  },
  {
    key: 'guardian1_name',
    label: '家长1姓名',
    labelInSection: '姓名',
    group: 'basic',
    maxLength: 50,
    enumerable: false,
    options: [],
  },
  {
    key: 'guardian1_relationship',
    label: '家长1关系',
    labelInSection: '与学生关系',
    group: 'basic',
    maxLength: 32,
    enumerable: false,
    options: [],
  },
  {
    key: 'guardian1_phone',
    label: '家长1联系方式',
    labelInSection: '联系方式',
    fullWidthInForm: true,
    group: 'basic',
    maxLength: 32,
    enumerable: false,
    options: [],
  },
  {
    key: 'guardian2_name',
    label: '家长2姓名',
    labelInSection: '姓名',
    group: 'basic',
    maxLength: 50,
    enumerable: false,
    options: [],
  },
  {
    key: 'guardian2_relationship',
    label: '家长2关系',
    labelInSection: '与学生关系',
    group: 'basic',
    maxLength: 32,
    enumerable: false,
    options: [],
  },
  {
    key: 'guardian2_phone',
    label: '家长2联系方式',
    labelInSection: '联系方式',
    fullWidthInForm: true,
    group: 'basic',
    maxLength: 32,
    enumerable: false,
    options: [],
  },
  {
    key: 'only_child',
    label: '是否独生子女',
    group: 'sensitive',
    maxLength: 7,
    enumerable: true,
    options: [
      { value: 'unknown', label: '未填写／未知' },
      { value: 'yes', label: '是' },
      { value: 'no', label: '否' },
    ],
  },
  {
    key: 'single_parent',
    label: '是否单亲家庭',
    group: 'sensitive',
    maxLength: 7,
    enumerable: true,
    options: [
      { value: 'unknown', label: '未填写／未知' },
      { value: 'yes', label: '是' },
      { value: 'no', label: '否' },
    ],
  },
  {
    key: 'orphan',
    label: '是否孤儿',
    group: 'sensitive',
    maxLength: 7,
    enumerable: true,
    options: [
      { value: 'unknown', label: '未填写／未知' },
      { value: 'yes', label: '是' },
      { value: 'no', label: '否' },
    ],
  },
  {
    key: 'student_loan',
    label: '是否助学贷款',
    group: 'sensitive',
    maxLength: 7,
    enumerable: true,
    options: [
      { value: 'unknown', label: '未填写／未知' },
      { value: 'yes', label: '是' },
      { value: 'no', label: '否' },
    ],
  },
  {
    key: 'registered_poverty',
    label: '是否建档立卡生',
    group: 'sensitive',
    maxLength: 7,
    enumerable: true,
    options: [
      { value: 'unknown', label: '未填写／未知' },
      { value: 'yes', label: '是' },
      { value: 'no', label: '否' },
    ],
  },
  {
    key: 'has_religion',
    label: '是否有宗教信仰',
    group: 'sensitive',
    maxLength: 7,
    enumerable: true,
    options: [
      { value: 'unknown', label: '未填写／未知' },
      { value: 'yes', label: '是' },
      { value: 'no', label: '否' },
    ],
  },
  {
    key: 'religion_category',
    label: '宗教信仰类别',
    group: 'sensitive',
    maxLength: 32,
    enumerable: true,
    options: [],
  },
  {
    key: 'religion_identification',
    label: '宗教信仰标识',
    group: 'sensitive',
    maxLength: 16,
    enumerable: true,
    options: [],
  },
  {
    key: 'religion_start_year',
    label: '宗教信仰起始年份',
    group: 'sensitive',
    maxLength: 16,
    enumerable: false,
    options: [],
  },
  {
    key: 'family_religion',
    label: '家庭成员宗教信仰情况',
    group: 'sensitive',
    maxLength: 1000,
    enumerable: false,
    options: [],
  },
  {
    key: 'medical_history',
    label: '治疗中的心理疾病或既往病史',
    group: 'sensitive',
    maxLength: 2000,
    enumerable: false,
    options: [],
  },
  {
    key: 'special_circumstances',
    label: '其他本人或家庭特殊情况',
    group: 'sensitive',
    maxLength: 2000,
    enumerable: false,
    options: [],
  },
];

export function fieldLabel(field: StudentField, value: string) {
  const option = field.options.find((item) => item.value === value);
  if (option?.value === 'unknown') return emptyValueLabel;
  return option?.label ?? formatValue(value);
}

// 与数据库生成列保持一致，按十八位身份证号码中的有效日期生成出生日期。
export function birthDateFromIdentityNumber(value: string) {
  const identity = value.trim();
  if (!/^\d{6}[1-9]\d{3}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])\d{3}[\dXx]$/u.test(identity)) {
    return '';
  }
  const date = `${identity.slice(6, 10)}-${identity.slice(10, 12)}-${identity.slice(12, 14)}`;
  const parsed = new Date(`${date}T00:00:00Z`);
  return !Number.isNaN(parsed.getTime()) && parsed.toISOString().slice(0, 10) === date ? date : '';
}

interface StudentSection {
  id: string;
  label: string;
  fields: string[];
  detailTitleField?: string;
  formGridColumns?: string;
}

export const studentContactSections = ['contact', 'guardian-1', 'guardian-2'] as const;

export const studentRecordTabs = [
  {
    value: 'basic',
    label: '学籍资料',
    group: 'basic',
    sections: ['academic', 'previous-education', 'advisor', 'personal'],
  },
  {
    value: 'contact',
    label: '联系住宿',
    group: 'basic',
    sections: [...studentContactSections, 'accommodation'],
  },
  { value: 'sensitive', label: '敏感信息', group: 'sensitive', sections: [] },
] as const;

export type StudentRecordTab = (typeof studentRecordTabs)[number];

// 展示类别仅组织详情与表单，不改变基本信息和敏感信息的权限边界。
export const studentSections: Record<StudentField['group'], StudentSection[]> = {
  basic: [
    {
      id: 'academic',
      label: '学籍信息',
      fields: [
        'student_no',
        'name',
        'grade',
        'status',
        'education_level',
        'major',
        'class_name',
        'class_position',
        'admission_type',
        'arrival_month',
      ],
    },
    {
      id: 'previous-education',
      label: '来校前经历',
      fields: ['previous_school_or_employer', 'previous_graduation_month'],
    },
    { id: 'advisor', label: '导师信息', fields: ['advisor', 'advisor_no', 'advisor_phone'] },
    {
      id: 'personal',
      label: '个人信息',
      fields: ['gender', 'ethnicity', 'political_status', 'birth_date', 'origin', 'native_place'],
    },
    { id: 'contact', label: '联系方式', fields: ['phone', 'qq', 'email'] },
    {
      id: 'accommodation',
      label: '住宿信息',
      fields: ['accommodation_type', 'campus', 'building', 'room', 'bed'],
    },
    {
      id: 'guardian-1',
      label: '家长 1',
      fields: ['guardian1_relationship', 'guardian1_name', 'guardian1_phone'],
      detailTitleField: 'guardian1_relationship',
      formGridColumns: 'grid-cols-2',
    },
    {
      id: 'guardian-2',
      label: '家长 2',
      fields: ['guardian2_relationship', 'guardian2_name', 'guardian2_phone'],
      detailTitleField: 'guardian2_relationship',
      formGridColumns: 'grid-cols-2',
    },
  ],
  sensitive: [
    { id: 'identity', label: '证件与住址', fields: ['identity_number', 'home_address'] },
    { id: 'family', label: '家庭情况', fields: ['only_child', 'single_parent', 'orphan'] },
    { id: 'financial', label: '资助情况', fields: ['student_loan', 'registered_poverty'] },
    {
      id: 'religion',
      label: '宗教信仰',
      fields: [
        'has_religion',
        'religion_category',
        'religion_identification',
        'religion_start_year',
        'family_religion',
      ],
    },
    {
      id: 'special',
      label: '个人专项',
      fields: ['medical_history', 'special_circumstances'],
    },
  ],
};
