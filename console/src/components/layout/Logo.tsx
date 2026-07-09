import { PaperPlaneTilt } from "@phosphor-icons/react";

export function Logo() {
  return (
    <div aria-label="MailRelay" className="flex items-center gap-2.5 px-7 py-5">
      <PaperPlaneTilt weight="fill" className="size-7 text-primary" />
      <span className="text-[22px] font-semibold tracking-tight">
        Mail<span className="text-primary">Relay</span>
      </span>
    </div>
  );
}
