import { useId, useState } from 'react';

import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Drawer } from '@/components/ui/drawer';
import { Label } from '@/components/ui/label';
import { type AgreementType, getAgreementContent } from '@/features/user/agreement-content';
import { getConfig } from '@/lib/config';

interface Props {
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  disabled?: boolean;
}

export function LoginAgreements({ checked, onCheckedChange, disabled = false }: Props) {
  const id = useId();
  const [opened, setOpened] = useState<AgreementType | null>(null);
  const { appName, contactEmail } = getConfig();
  const agreements = getAgreementContent(appName, contactEmail);
  const content = opened ? agreements[opened] : null;

  return (
    <>
      <div className="flex items-start gap-2">
        <Checkbox
          id={id}
          className="mt-1"
          checked={checked}
          onChange={(event) => {
            onCheckedChange(event.target.checked);
          }}
          disabled={disabled}
          required
          aria-label="我已阅读并同意《用户协议》和《隐私协议》"
        />
        <div className="min-w-0 text-xs leading-6 text-muted-foreground">
          <Label htmlFor={id} className="inline text-xs leading-6 font-normal">
            我已阅读并同意
          </Label>
          <Button
            type="button"
            variant="link"
            className="h-auto p-0 text-xs leading-6"
            onClick={() => {
              setOpened('user');
            }}
          >
            《用户协议》
          </Button>
          和
          <Button
            type="button"
            variant="link"
            className="h-auto p-0 text-xs leading-6"
            onClick={() => {
              setOpened('privacy');
            }}
          >
            《隐私协议》
          </Button>
        </div>
      </div>
      {content ? (
        <Drawer
          title={content.title}
          description="请仔细阅读并充分理解以下条款。"
          onClose={() => {
            setOpened(null);
          }}
          footer={
            <Button
              type="button"
              variant="cancel"
              onClick={() => {
                setOpened(null);
              }}
            >
              关闭
            </Button>
          }
        >
          <article className="space-y-6 p-6 text-sm leading-7 wrap-anywhere">
            <p>{content.introduction}</p>
            {content.sections.map((section) => (
              <section key={section.title} className="space-y-2">
                <h3 className="font-semibold">{section.title}</h3>
                {section.paragraphs.map((paragraph) => (
                  <p key={paragraph}>{paragraph}</p>
                ))}
                {section.subsections?.map((subsection) => (
                  <section key={subsection.title} className="space-y-2 pt-2">
                    <h4 className="font-medium">{subsection.title}</h4>
                    {subsection.paragraphs.map((paragraph) => (
                      <p key={paragraph}>{paragraph}</p>
                    ))}
                  </section>
                ))}
              </section>
            ))}
          </article>
        </Drawer>
      ) : null}
    </>
  );
}
