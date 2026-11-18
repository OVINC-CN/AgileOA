import { currentCalendarDate } from '@/lib/calendar';
import type { TextFieldOption } from '@/lib/text-field-options';

const previousGradeYears = 8;
const followingGradeYears = 1;
const guardianRelationships = [
  { value: 'father', label: '父亲' },
  { value: 'mother', label: '母亲' },
] as const;

// 表单建议与固定枚举分开维护，保留自由输入和后端历史值。
const fieldPresets: Readonly<Record<string, readonly TextFieldOption[]>> = {
  status: [
    { value: 'enrolled', label: '在籍' },
    { value: 'graduated', label: '毕业' },
  ],
  education_level: [
    { value: 'bachelor', label: '学士' },
    { value: 'master', label: '硕士' },
    { value: 'doctor', label: '博士' },
  ],
  class_position: [
    { value: 'monitor', label: '班长' },
    { value: 'league_secretary', label: '团支书' },
  ],
  admission_type: [
    { value: 'non_directed', label: '非定向' },
    { value: 'directed', label: '定向' },
    { value: 'directed_ethnic_minority_backbone', label: '定向（少数民族骨干计划）' },
  ],
  gender: [
    { value: 'male', label: '男' },
    { value: 'female', label: '女' },
  ],
  ethnicity: [
    { value: 'han', label: '汉族' },
    { value: 'mongol', label: '蒙古族' },
    { value: 'hui', label: '回族' },
    { value: 'tibetan', label: '藏族' },
    { value: 'uyghur', label: '维吾尔族' },
    { value: 'miao', label: '苗族' },
    { value: 'yi', label: '彝族' },
    { value: 'zhuang', label: '壮族' },
    { value: 'buyei', label: '布依族' },
    { value: 'korean', label: '朝鲜族' },
    { value: 'manchu', label: '满族' },
    { value: 'dong', label: '侗族' },
    { value: 'yao', label: '瑶族' },
    { value: 'bai', label: '白族' },
    { value: 'tujia', label: '土家族' },
    { value: 'hani', label: '哈尼族' },
    { value: 'kazakh', label: '哈萨克族' },
    { value: 'dai', label: '傣族' },
    { value: 'li', label: '黎族' },
    { value: 'lisu', label: '傈僳族' },
    { value: 'wa', label: '佤族' },
    { value: 'she', label: '畲族' },
    { value: 'gaoshan', label: '高山族' },
    { value: 'lahu', label: '拉祜族' },
    { value: 'shui', label: '水族' },
    { value: 'dongxiang', label: '东乡族' },
    { value: 'naxi', label: '纳西族' },
    { value: 'jingpo', label: '景颇族' },
    { value: 'kyrgyz', label: '柯尔克孜族' },
    { value: 'tu', label: '土族' },
    { value: 'daur', label: '达斡尔族' },
    { value: 'mulao', label: '仫佬族' },
    { value: 'qiang', label: '羌族' },
    { value: 'blang', label: '布朗族' },
    { value: 'salar', label: '撒拉族' },
    { value: 'maonan', label: '毛南族' },
    { value: 'gelao', label: '仡佬族' },
    { value: 'xibe', label: '锡伯族' },
    { value: 'achang', label: '阿昌族' },
    { value: 'pumi', label: '普米族' },
    { value: 'tajik', label: '塔吉克族' },
    { value: 'nu', label: '怒族' },
    { value: 'uzbek', label: '乌孜别克族' },
    { value: 'russian', label: '俄罗斯族' },
    { value: 'evenki', label: '鄂温克族' },
    { value: 'deang', label: '德昂族' },
    { value: 'bonan', label: '保安族' },
    { value: 'yugur', label: '裕固族' },
    { value: 'jing', label: '京族' },
    { value: 'tatar', label: '塔塔尔族' },
    { value: 'derung', label: '独龙族' },
    { value: 'oroqen', label: '鄂伦春族' },
    { value: 'hezhen', label: '赫哲族' },
    { value: 'monba', label: '门巴族' },
    { value: 'lhoba', label: '珞巴族' },
    { value: 'jino', label: '基诺族' },
  ],
  political_status: [
    { value: 'masses', label: '群众' },
    { value: 'league_member', label: '共青团员' },
    { value: 'probationary_member', label: '中共预备党员' },
    { value: 'full_member', label: '中共党员' },
  ],
  religion_category: [
    { value: 'buddhism', label: '佛教' },
    { value: 'taoism', label: '道教' },
    { value: 'islam', label: '伊斯兰教' },
    { value: 'catholicism', label: '天主教' },
    { value: 'protestantism', label: '基督教' },
    { value: 'other', label: '其他' },
  ],
  religion_identification: [
    { value: 'suspected', label: '疑似' },
    { value: 'confirmed', label: '已确认' },
  ],
  guardian1_relationship: guardianRelationships,
  guardian2_relationship: guardianRelationships,
};

export function studentFormPresets(field: string): readonly TextFieldOption[] {
  // 年级按配置的展示时区滚动生成，接口继续使用纯年份文本。
  if (field === 'grade') {
    const currentYear = Number(currentCalendarDate().slice(0, 4));
    return Array.from({ length: previousGradeYears + followingGradeYears + 1 }, (_, index) => {
      const year = String(currentYear + followingGradeYears - index);
      return { value: year, label: year };
    });
  }
  return fieldPresets[field] ?? [];
}
