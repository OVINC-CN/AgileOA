import { queryOptions } from '@tanstack/react-query';
import type { z } from 'zod';

import { studentDetailSchema } from '@/features/student/api';
import { apiGet } from '@/lib/api';

export const studentProfileSchema = studentDetailSchema.pick({
  id: true,
  student_no: true,
  name: true,
  grade: true,
  status: true,
  education_level: true,
  major: true,
  class_name: true,
  class_position: true,
});
export type StudentProfile = z.infer<typeof studentProfileSchema>;

export const studentProfileOptions = (workspaceID: string, studentID: number) =>
  queryOptions({
    queryKey: ['workspace', workspaceID, 'student-service', 'profile', studentID],
    queryFn: ({ signal }) =>
      apiGet(
        `/v1/workspaces/${workspaceID}/student-service`,
        studentProfileSchema.refine((student) => student.id === studentID),
        signal,
      ),
    enabled: workspaceID !== '' && studentID > 0,
    retry: false,
  });
