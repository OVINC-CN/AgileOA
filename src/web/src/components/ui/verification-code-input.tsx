import { useEffect, useId, useRef } from 'react';

import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';

export const verificationCodeLength = 6;

interface Props {
  value: readonly string[];
  onValueChange: (digits: string[]) => void;
  onComplete: (code: string) => void;
  label?: string;
  describedBy?: string;
  readOnly?: boolean;
}

export function VerificationCodeInput({
  value,
  onValueChange,
  onComplete,
  label = '验证码',
  describedBy,
  readOnly = false,
}: Props) {
  const id = useId();
  const inputs = useRef<(HTMLInputElement | null)[]>([]);

  useEffect(() => {
    if (value.every((digit) => digit === '')) inputs.current[0]?.focus();
  }, [value]);

  function focus(index: number) {
    const target = inputs.current[Math.max(0, Math.min(verificationCodeLength - 1, index))];
    target?.focus();
    target?.select();
  }

  function update(index: number, text: string) {
    if (readOnly) return;
    const digits = text.replace(/[^0-9]/g, '').slice(0, verificationCodeLength);
    if (text !== '' && digits === '') return;

    // 完整粘贴和系统自动填充从首格写入，部分粘贴从当前格继续。
    const start = digits.length === verificationCodeLength ? 0 : index;
    const next = [...value];
    if (digits === '') next[start] = '';
    else {
      for (
        let offset = 0;
        offset < digits.length && start + offset < verificationCodeLength;
        offset++
      ) {
        next[start + offset] = digits.charAt(offset);
      }
    }
    onValueChange(next);
    if (digits !== '') focus(start + digits.length);
    if (next.length === verificationCodeLength && next.every((digit) => /^[0-9]$/.test(digit))) {
      onComplete(next.join(''));
    }
  }

  return (
    <div
      role="group"
      aria-labelledby={`${id}-label`}
      aria-describedby={describedBy}
      className="space-y-2"
    >
      <Label id={`${id}-label`} htmlFor={`${id}-0`}>
        {label}
      </Label>
      <div className="grid grid-cols-6 gap-2">
        {Array.from({ length: verificationCodeLength }, (_, index) => (
          <Input
            key={index}
            ref={(element) => {
              inputs.current[index] = element;
            }}
            id={`${id}-${String(index)}`}
            type="text"
            inputMode="numeric"
            autoComplete={index === 0 ? 'one-time-code' : 'off'}
            pattern="[0-9]"
            maxLength={verificationCodeLength}
            spellCheck={false}
            readOnly={readOnly}
            className="h-10 px-0 text-center font-mono text-base tabular-nums"
            aria-label={`第 ${String(index + 1)} 位验证码`}
            aria-describedby={describedBy}
            value={value[index] ?? ''}
            onFocus={(event) => {
              event.currentTarget.select();
            }}
            onChange={(event) => {
              const native = event.nativeEvent;
              const text =
                native instanceof InputEvent &&
                native.inputType === 'insertText' &&
                native.data?.length === 1
                  ? native.data
                  : event.currentTarget.value;
              update(index, text);
            }}
            onPaste={(event) => {
              event.preventDefault();
              update(index, event.clipboardData.getData('text'));
            }}
            onKeyDown={(event) => {
              if (event.altKey || event.ctrlKey || event.metaKey) return;
              if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
                event.preventDefault();
                focus(index + (event.key === 'ArrowLeft' ? -1 : 1));
              } else if (event.key === 'Home' || event.key === 'End') {
                event.preventDefault();
                focus(event.key === 'Home' ? 0 : verificationCodeLength - 1);
              } else if (!readOnly && event.key === 'Backspace') {
                event.preventDefault();
                const target = value[index] === '' && index > 0 ? index - 1 : index;
                update(target, '');
                focus(target);
              } else if (!readOnly && event.key === 'Delete') {
                event.preventDefault();
                update(index, '');
              }
            }}
          />
        ))}
      </div>
    </div>
  );
}
