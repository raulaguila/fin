"use client";

import { useEffect, useRef, useState } from "react";
import { usePathname } from "next/navigation";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Bot, Check, Loader2, RotateCcw, Send } from "lucide-react";
import { toast } from "sonner";
import { useFinanceStatus } from "@/components/finance";
import { Markdown } from "@/components/markdown";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { ApiError } from "@/lib/api/client";
import { finance, type ChatAction, type ChatMessage } from "@/lib/api/finance";
import { errorMessage } from "@/lib/errors";
import { fill, useFinanceFormat } from "@/lib/finance-format";
import type { MessageKey } from "@/lib/i18n/messages";
import { useI18n } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

interface Turn extends ChatMessage {
  actions?: ChatAction[];
  /** Indexes of the actions already applied. */
  applied?: number[];
}

const STORE = "fin.assistant";
const GENERAL: MessageKey[] = ["fin.ai.s1", "fin.ai.s2", "fin.ai.s3", "fin.ai.s4"];
// Questions that fit the screen the user is on come first.
const BY_PAGE: [string, MessageKey[]][] = [
  ["/budget", ["fin.ai.p.budget1", "fin.ai.p.budget2"]],
  ["/subscriptions", ["fin.ai.p.subs1", "fin.ai.p.subs2"]],
  ["/goals", ["fin.ai.p.goals1"]],
  ["/calendar", ["fin.ai.p.cal1"]],
  ["/transactions", ["fin.ai.p.tx1", "fin.ai.p.tx2"]],
  ["/report", ["fin.ai.p.report1"]],
  ["/people", ["fin.ai.p.people1"]],
  ["/categories", ["fin.ai.p.cat1"]],
];

function load(): Turn[] {
  try {
    const raw = sessionStorage.getItem(STORE);
    const turns = raw ? (JSON.parse(raw) as Turn[]) : [];
    return Array.isArray(turns) ? turns.filter((x) => typeof x?.content === "string" && x.content) : [];
  } catch {
    return [];
  }
}

function save(turns: Turn[]) {
  try {
    sessionStorage.setItem(STORE, JSON.stringify(turns));
  } catch {
    /* private mode or full: the chat still works for this page */
  }
}

/** The assistant floats over every screen; the chat opens in a side drawer and lasts while the tab is open. */
export function AssistantButton() {
  const { t } = useI18n();
  const f = useFinanceFormat();
  const pathname = usePathname();
  const client = useQueryClient();
  const status = useFinanceStatus();
  const [open, setOpen] = useState(false);
  const [turns, setTurns] = useState<Turn[]>([]);
  const [draft, setDraft] = useState("");
  const [live, setLive] = useState("");
  const [step, setStep] = useState("");
  const [error, setError] = useState("");
  const end = useRef<HTMLDivElement>(null);
  const abort = useRef<AbortController | null>(null);

  // sessionStorage exists only in the browser: read it after the first render.
  // eslint-disable-next-line react-hooks/set-state-in-effect -- restore once on mount
  useEffect(() => setTurns(load()), []);
  useEffect(() => {
    end.current?.scrollIntoView({ block: "end" });
  }, [turns, live, step, open]);

  const update = (next: Turn[]) => {
    setTurns(next);
    save(next);
  };

  const ask = useMutation({
    mutationFn: async (history: Turn[]) => {
      const controller = new AbortController();
      abort.current = controller;
      let text = "";
      let done: Turn | null = null;
      await finance.chat(history.map(({ role, content }) => ({ role, content })), (e) => {
        if (e.type === "delta") {
          text += e.text;
          setLive(text);
          setStep("");
        } else if (e.type === "reset") {
          text = "";
          setLive("");
        } else if (e.type === "status") setStep(e.text);
        else if (e.type === "done") done = { role: "assistant", content: e.answer ?? "", actions: e.actions };
        else if (e.type === "error") throw new ApiError(200, e.error);
      }, controller.signal);
      if (!done) throw new Error("stream ended without an answer");
      return { history, reply: done as Turn };
    },
    onSuccess: ({ history, reply }) => update([...history, reply]),
    onError: (err) => {
      if (!(err instanceof DOMException && err.name === "AbortError")) setError(errorMessage(err, "fin.ai.failed", t));
    },
    onSettled: () => {
      setLive("");
      setStep("");
    },
  });

  const send = (text: string) => {
    const content = text.trim();
    if (!content || ask.isPending) return;
    const history: Turn[] = [...turns, { role: "user", content }];
    update(history);
    setDraft("");
    setError("");
    ask.mutate(history);
  };

  const clear = () => {
    abort.current?.abort();
    update([]);
    setError("");
  };

  const apply = useMutation({
    mutationFn: async ({ action }: { turn: number; index: number; action: ChatAction }) => {
      if (action.kind === "rule") await finance.createRule(action.keyword, action.category_id ?? "");
      else await finance.createSubscription({ name: action.name ?? "", amount: action.amount ?? 0, day: action.day ?? 1, month: 0, keyword: action.keyword, category_id: "", account_id: "" });
    },
    onSuccess: (_, { turn, index }) => {
      update(turns.map((x, i) => (i === turn ? { ...x, applied: [...(x.applied ?? []), index] } : x)));
      void client.invalidateQueries({ queryKey: ["finance"] });
      toast.success(t("fin.ai.applied"));
    },
    onError: (err) => toast.error(errorMessage(err, "common.save_failed", t)),
  });

  const actionLabel = (a: ChatAction) =>
    a.kind === "rule"
      ? fill(t("fin.ai.action_rule"), { k: a.keyword, c: a.category_name ?? "" })
      : fill(t("fin.ai.action_sub"), { n: a.name ?? "", v: f.money(a.amount ?? 0), d: a.day ?? 1 });

  // No AI provider configured: nothing to offer, so no button.
  if (!status.data?.assistant) return null;

  const page = BY_PAGE.find(([prefix]) => pathname.startsWith(prefix))?.[1] ?? [];
  const suggestions = [...page, ...GENERAL].slice(0, 4);

  return (
    <>
      <Button
        aria-label={t("nav.assistant")}
        title={t("nav.assistant")}
        className="fixed right-4 bottom-4 z-30 size-12 rounded-full shadow-lg"
        onClick={() => setOpen(true)}
      >
        {ask.isPending ? <Loader2 className="size-5 animate-spin" /> : <Bot className="size-5" />}
      </Button>
      <Sheet open={open} onOpenChange={setOpen}>
        <SheetContent side="right" className="gap-0 data-[side=right]:w-full data-[side=right]:sm:max-w-md">
          <SheetHeader className="border-b pr-12">
            <SheetTitle>{t("nav.assistant")}</SheetTitle>
            <SheetDescription>{t("fin.ai.note")}</SheetDescription>
          </SheetHeader>
          <div className="min-h-0 flex-1 space-y-3 overflow-y-auto p-4" aria-live="polite">
            {turns.length === 0 && (
              <div className="flex flex-wrap gap-2">
                {suggestions.map((k) => (
                  <Button key={k} variant="outline" size="sm" className="h-auto whitespace-normal py-1.5 text-left" onClick={() => send(t(k))}>{t(k)}</Button>
                ))}
              </div>
            )}
            {turns.map((m, i) => (
              <div key={i} className={cn("flex flex-col gap-2", m.role === "user" ? "items-end" : "items-start")}>
                <Card className={cn("max-w-[90%] gap-0 px-4 py-2.5 text-sm leading-relaxed", m.role === "user" && "bg-primary text-primary-foreground whitespace-pre-wrap")}>
                  {m.role === "user" ? m.content : <Markdown text={m.content} />}
                </Card>
                {m.actions?.map((a, j) => {
                  const done = m.applied?.includes(j);
                  return (
                    <div key={j} className="flex max-w-[90%] items-center gap-3 rounded-lg border border-dashed px-3 py-2 text-sm">
                      <span className="min-w-0 flex-1">{actionLabel(a)}</span>
                      <Button size="sm" variant={done ? "ghost" : "outline"} disabled={done || apply.isPending}
                        onClick={() => apply.mutate({ turn: i, index: j, action: a })}>
                        {done ? <><Check className="size-4" />{t("fin.ai.done")}</> : t("fin.ai.apply")}
                      </Button>
                    </div>
                  );
                })}
              </div>
            ))}
            {live && (
              <Card className="max-w-[90%] gap-0 px-4 py-2.5 text-sm leading-relaxed"><Markdown text={live} /></Card>
            )}
            {ask.isPending && !live && (
              <p className="flex items-center gap-2 text-sm text-muted-foreground"><Loader2 className="size-4 animate-spin" />{step || t("fin.ai.thinking")}</p>
            )}
            {error && <p className="text-sm text-red-700 dark:text-red-400" role="alert">{error}</p>}
            <div ref={end} />
          </div>
          <form className="flex items-end gap-2 border-t p-4" onSubmit={(e) => { e.preventDefault(); send(draft); }}>
            {turns.length > 0 && (
              <Button type="button" variant="ghost" size="icon" aria-label={t("fin.ai.clear")} title={t("fin.ai.clear")} onClick={clear}>
                <RotateCcw className="size-4" />
              </Button>
            )}
            <textarea
              value={draft}
              rows={2}
              maxLength={4000}
              aria-label={t("fin.ai.placeholder")}
              placeholder={t("fin.ai.placeholder")}
              className="min-h-[var(--control-h)] flex-1 resize-none rounded-lg border bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
              onChange={(e) => setDraft(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !e.shiftKey) {
                  e.preventDefault();
                  send(draft);
                }
              }}
            />
            <Button type="submit" size="icon" aria-label={t("fin.ai.send")} disabled={!draft.trim() || ask.isPending}><Send className="size-4" /></Button>
          </form>
        </SheetContent>
      </Sheet>
    </>
  );
}
