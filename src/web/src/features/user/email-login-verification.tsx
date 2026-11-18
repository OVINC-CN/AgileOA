import { ArrowLeft, Mail } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { z } from 'zod';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { LoadingIcon } from '@/components/ui/loading-icon';
import {
  VerificationCodeInput,
  verificationCodeLength,
} from '@/components/ui/verification-code-input';
import type { EmailLoginChallenge } from '@/features/user/api';
import { showMessage } from '@/lib/messages';

export type ReceivedEmailChallenge = EmailLoginChallenge & { receivedAt: number };

interface Props {
  challenge: ReceivedEmailChallenge;
  email: string;
  requestedUsername: string;
  pending: boolean;
  verifying: boolean;
  resending: boolean;
  onVerify: (code: string, username?: string) => Promise<boolean | undefined>;
  onResend: () => void;
  onBack: () => void;
}

const usernameSchema = z
  .string()
  .trim()
  .regex(/^[a-z0-9][a-z0-9._-]{2,63}$/, '请输入 3 至 64 位小写英文、数字或 ._-');
const emptyCode = () => Array<string>(verificationCodeLength).fill('');

export function EmailLoginVerification({
  challenge,
  email,
  requestedUsername,
  pending,
  verifying,
  resending,
  onVerify,
  onResend,
  onBack,
}: Props) {
  const [now, setNow] = useState(Date.now);
  const [digits, setDigits] = useState(emptyCode);
  const [username, setUsername] = useState('');
  const [usernameRequired, setUsernameRequired] = useState(false);
  const [usernameInvalid, setUsernameInvalid] = useState(false);
  const usernameInput = useRef<HTMLInputElement>(null);
  const locked = useRef(false);
  const submitted = useRef<string | null>(null);
  const expiresAt = challenge.receivedAt + challenge.expires_in * 1000;
  const retryAt = challenge.receivedAt + challenge.retry_after * 1000;
  const retryAfter = Math.max(0, Math.ceil((retryAt - now) / 1000));
  const expired = now >= expiresAt;
  const code = digits.join('');

  useEffect(() => {
    const timer = window.setInterval(() => {
      setNow(Date.now());
    }, 1000);
    const update = () => {
      setNow(Date.now());
    };
    document.addEventListener('visibilitychange', update);
    return () => {
      window.clearInterval(timer);
      document.removeEventListener('visibilitychange', update);
    };
  }, []);

  useEffect(() => {
    if (usernameRequired) usernameInput.current?.focus();
  }, [usernameRequired]);

  function clearVerification() {
    setDigits(emptyCode());
    setUsername('');
    submitted.current = null;
  }

  async function verify(nextCode: string, nextUsername?: string) {
    // 输入事件中直接提交并同步加锁，避免重复填充或严格模式触发并发校验。
    if (pending || locked.current || !/^[0-9]{6}$/.test(nextCode)) return;
    if (Date.now() >= expiresAt) {
      clearVerification();
      showMessage('验证码已过期，请重新获取');
      return;
    }
    if (usernameRequired) {
      const parsed = usernameSchema.safeParse(nextUsername ?? '');
      if (!parsed.success) {
        clearVerification();
        setUsernameInvalid(true);
        showMessage(parsed.error.issues[0]?.message ?? '请输入用户名');
        return;
      }
      nextUsername = parsed.data;
    }
    const signature = `${nextCode}:${nextUsername ?? ''}`;
    if (submitted.current === signature) return;
    submitted.current = signature;
    locked.current = true;
    setUsernameInvalid(false);
    try {
      const required = await onVerify(nextCode, nextUsername);
      if (required) setUsernameRequired(true);
      else if (required === undefined) {
        // 校验失败后等待新的输入，不在副作用中自动重试或反复消耗错误次数。
        clearVerification();
        if (usernameRequired) setUsernameInvalid(true);
      }
    } finally {
      locked.current = false;
    }
  }

  function complete(nextCode: string) {
    if (usernameRequired && username.trim() !== requestedUsername) {
      usernameInput.current?.focus();
      return;
    }
    void verify(nextCode, usernameRequired ? username.trim() : undefined);
  }

  return (
    <div aria-busy={verifying}>
      <p role="status" className="mb-5 text-sm break-words text-muted-foreground">
        登录验证码已发送至 {email}，请查收。
      </p>
      <VerificationCodeInput
        value={digits}
        label="邮箱验证码"
        describedBy="email-code-help"
        readOnly={pending || expired}
        onValueChange={(next) => {
          setDigits(next);
          submitted.current = null;
        }}
        onComplete={complete}
      />
      <p id="email-code-help" className="mt-2 text-xs text-muted-foreground">
        {expired ? '验证码已过期，请重新获取。' : '填写 6 位数字后自动验证登录。'}
      </p>
      {verifying ? (
        <p role="status" className="mt-3 flex items-center gap-2 text-sm text-muted-foreground">
          <LoadingIcon loading className="size-4" />
          正在验证登录…
        </p>
      ) : null}
      {usernameRequired ? (
        <div className="mt-5 space-y-2">
          <Label htmlFor="email-login-username">用户名</Label>
          <Input
            ref={usernameInput}
            id="email-login-username"
            autoComplete="off"
            autoCapitalize="none"
            spellCheck={false}
            maxLength={64}
            required
            readOnly={pending || expired}
            className="h-10"
            aria-invalid={usernameInvalid}
            aria-describedby="email-username-help"
            value={username}
            onChange={(event) => {
              const next = event.currentTarget.value;
              setUsername(next);
              setUsernameInvalid(false);
              submitted.current = null;
              // 只比较本人重新输入的用户名，不自动回填申请时的值。
              if (next.trim() === requestedUsername) void verify(code, next.trim());
            }}
            onKeyDown={(event) => {
              if (event.key === 'Enter' && !event.nativeEvent.isComposing) {
                event.preventDefault();
                void verify(code, username.trim());
              }
            }}
          />
          <p id="email-username-help" className="text-xs text-muted-foreground">
            请输入本次登录使用的用户名。
          </p>
        </div>
      ) : null}
      <Button
        type="button"
        variant="ghost"
        className="mt-3 h-10 w-full text-muted-foreground"
        disabled={pending || retryAfter > 0}
        loading={resending}
        onClick={() => {
          if (Date.now() >= retryAt) onResend();
        }}
      >
        <Mail aria-hidden="true" />
        {retryAfter > 0 ? `重新发送验证码（${String(retryAfter)} 秒）` : '重新发送验证码'}
      </Button>
      <Button
        type="button"
        variant="ghost"
        className="mt-3 h-9 w-full text-muted-foreground"
        disabled={pending}
        onClick={onBack}
      >
        <ArrowLeft aria-hidden="true" />
        修改登录信息
      </Button>
    </div>
  );
}
