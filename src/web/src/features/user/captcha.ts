import type { CaptchaProof, LoginCaptcha } from '@/features/user/api';
import { getConfig } from '@/lib/config';

interface CaptchaResult {
  ret: number;
  ticket?: string | null;
  randstr?: string;
  errorCode?: number;
}

type TencentCaptcha = new (
  container: HTMLElement,
  appID: string,
  callback: (result: CaptchaResult) => void,
  options: {
    type: 'embed';
    userLanguage: string;
    aidEncrypted: string;
    aidEncryptedType: 'gcm';
  },
) => { show: () => void; destroy: () => void };

declare global {
  interface Window {
    TencentCaptcha?: TencentCaptcha;
  }
}

let sdkPromise: Promise<TencentCaptcha> | undefined;

function loadCaptchaSDK() {
  if (window.TencentCaptcha) return Promise.resolve(window.TencentCaptcha);
  sdkPromise ??= new Promise<TencentCaptcha>((resolve, reject) => {
    const script = document.createElement('script');
    script.src = '/extra-assets/js/tencent/TJCaptcha.js';
    script.async = true;

    function fail() {
      window.clearTimeout(timer);
      script.remove();
      reject(new Error('验证码加载失败，请重试'));
    }

    const timer = window.setTimeout(fail, getConfig().apiTimeoutMs);
    script.onerror = fail;
    script.onload = () => {
      if (!window.TencentCaptcha) {
        fail();
        return;
      }
      window.clearTimeout(timer);
      resolve(window.TencentCaptcha);
    };
    document.head.append(script);
  }).catch((error: unknown) => {
    sdkPromise = undefined;
    throw error;
  });
  return sdkPromise;
}

export async function solveLoginCaptcha(
  container: HTMLElement,
  challenge: LoginCaptcha,
  signal: AbortSignal,
): Promise<CaptchaProof | null> {
  if (signal.aborted) return null;

  let cancel: (() => void) | undefined;
  let captcha: InstanceType<TencentCaptcha> | undefined;
  let result: CaptchaResult | null;
  const target = document.createElement('div');
  try {
    result = await new Promise<CaptchaResult | null>((resolve, reject) => {
      cancel = () => {
        resolve(null);
      };
      signal.addEventListener('abort', cancel, { once: true });
      void loadCaptchaSDK()
        .then((Captcha) => {
          if (signal.aborted) return;
          container.append(target);
          captcha = new Captcha(target, challenge.app_id, resolve, {
            type: 'embed',
            userLanguage: 'zh-cn',
            aidEncrypted: challenge.aid_encrypted,
            aidEncryptedType: challenge.aid_encrypted_type,
          });
          captcha.show();
        })
        .catch(reject);
    });
  } catch {
    throw new Error('验证码加载失败，请重试');
  } finally {
    if (cancel) signal.removeEventListener('abort', cancel);
    // 移除本次验证的容器，隔离 SDK 在取消后的异步渲染。
    try {
      captcha?.destroy();
    } finally {
      target.remove();
    }
  }
  if (!result || result.ret === 2) return null;
  // 腾讯云在容灾时也返回 ret=0，应单独报告服务异常和错误码。
  if (result.errorCode || result.ticket?.startsWith('trerror')) {
    const detail = Number.isInteger(result.errorCode)
      ? `（错误码 ${String(result.errorCode)}）`
      : '';
    throw new Error(`验证码服务暂时不可用${detail}，请重试`);
  }
  if (result.ret !== 0 || !result.ticket || !result.randstr) {
    throw new Error('验证码未通过，请重试');
  }
  return { captcha_ticket: result.ticket, captcha_randstr: result.randstr };
}
