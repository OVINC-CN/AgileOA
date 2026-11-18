import { fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { StrictMode, useState } from 'react';
import { describe, expect, it, vi } from 'vitest';

import {
  VerificationCodeInput,
  verificationCodeLength,
} from '@/components/ui/verification-code-input';

function setup(readOnly = false) {
  const complete = vi.fn<(code: string) => void>();
  function Example() {
    const [digits, setDigits] = useState(() => Array<string>(verificationCodeLength).fill(''));
    return (
      <VerificationCodeInput
        value={digits}
        onValueChange={setDigits}
        onComplete={complete}
        readOnly={readOnly}
      />
    );
  }
  render(
    <StrictMode>
      <Example />
    </StrictMode>,
  );
  return {
    user: userEvent.setup(),
    complete,
    fields: screen.getAllByRole<HTMLInputElement>('textbox'),
  };
}

describe('六格验证码输入', () => {
  it('逐位输入自动前进，保留前导零，填满时仅通知一次', async () => {
    const { user, fields, complete } = setup();
    expect(fields).toHaveLength(6);
    expect(fields[0]).toHaveFocus();
    await user.keyboard('0');
    expect(fields[0]).toHaveValue('0');
    expect(fields[1]).toHaveFocus();
    await user.keyboard('1234');
    expect(fields[5]).toHaveFocus();
    expect(complete).not.toHaveBeenCalled();
    await user.keyboard('5');
    expect(fields.map((field) => field.value)).toEqual(['0', '1', '2', '3', '4', '5']);
    expect(complete).toHaveBeenCalledExactlyOnceWith('012345');
  });

  it('支持退格回删、方向键移动和覆盖已有数字，拒绝非数字', async () => {
    const { user, fields, complete } = setup();
    await user.keyboard('0123');
    expect(fields[4]).toHaveFocus();
    await user.keyboard('{Backspace}');
    expect(fields[3]).toHaveValue('');
    expect(fields[3]).toHaveFocus();
    await user.keyboard('{ArrowLeft}');
    expect(fields[2]).toHaveFocus();
    await user.keyboard('9');
    expect(fields[2]).toHaveValue('9');
    expect(fields[3]).toHaveFocus();
    await user.keyboard('a');
    expect(fields[3]).toHaveValue('');
    expect(fields[3]).toHaveFocus();
    await user.keyboard('{End}');
    expect(fields[5]).toHaveFocus();
    await user.keyboard('{Home}');
    expect(fields[0]).toHaveFocus();
    expect(complete).not.toHaveBeenCalled();
  });

  it('支持任意格粘贴完整验证码、分段粘贴和系统自动填充', async () => {
    const { user, fields, complete } = setup();
    const third = screen.getByLabelText('第 3 位验证码');
    await user.click(third);
    await user.paste('89');
    expect(fields.map((field) => field.value)).toEqual(['', '', '8', '9', '', '']);
    expect(fields[4]).toHaveFocus();
    expect(complete).not.toHaveBeenCalled();
    await user.paste(' 012 345 ');
    expect(fields.map((field) => field.value)).toEqual(['0', '1', '2', '3', '4', '5']);
    expect(complete).toHaveBeenCalledExactlyOnceWith('012345');
    fireEvent.change(screen.getByLabelText('第 1 位验证码'), { target: { value: '654321' } });
    expect(fields.map((field) => field.value)).toEqual(['6', '5', '4', '3', '2', '1']);
    expect(complete).toHaveBeenLastCalledWith('654321');
    expect(complete).toHaveBeenCalledTimes(2);
  });

  it('只读期间不能输入、粘贴或触发完成回调', async () => {
    const { user, fields, complete } = setup(true);
    await user.keyboard('012345');
    await user.paste('012345');
    expect(fields.every((field) => field.readOnly && field.value === '')).toBe(true);
    expect(complete).not.toHaveBeenCalled();
  });
});
