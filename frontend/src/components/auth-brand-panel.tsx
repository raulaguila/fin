"use client";

import { BrandMark } from "@/components/brand-mark";
import { PublicControls } from "@/components/public-controls";
import { useI18n } from "@/lib/i18n/provider";

/** Desktop brand column follows the height of the authentication form. */
export function AuthBrandPanel() {
  const { t } = useI18n();
  return (
    <div className="relative hidden flex-col overflow-hidden bg-primary text-primary-foreground lg:flex [background-image:var(--gradient-brand)]">
      <div className="relative z-20 flex justify-end p-6">
        <PublicControls className="text-primary-foreground [&_button]:text-primary-foreground [&_button:hover]:bg-primary-foreground/10" />
      </div>
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0 flex items-center justify-center overflow-hidden"
      >
        <BrandMark size={320} className="opacity-[0.12]" />
      </div>
      <div className="relative z-10 mt-auto max-w-md space-y-3 p-10 pb-12 pr-12">
        <p className="text-sm font-medium uppercase tracking-[0.2em] text-primary-foreground">
          {t("auth.brand_eyebrow")}
        </p>
        <h2 className="font-heading text-3xl font-semibold leading-tight text-primary-foreground">
          {t("auth.brand_tagline")}
        </h2>
        <p className="text-sm leading-relaxed text-primary-foreground">
          {t("auth.brand_body")}
        </p>
      </div>
    </div>
  );
}
