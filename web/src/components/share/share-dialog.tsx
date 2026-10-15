"use client";

import { useT } from "@/lib/i18n";

/**
 * Share dialog — invoked from a session detail page. Calls
 * `POST /api/v1/session/share` to create a share link, then displays the
 * resulting public URL with a copy button and the expiry time.
 */

import { useCallback, useState } from "react";
import { CalendarIcon, Check, Copy, Share2 } from "lucide-react";
import { format } from "date-fns";
import { toast } from "sonner";

import { api } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
import type { CreateShareReqBody } from "@/lib/types";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Calendar } from "@/components/ui/calendar";
import { Spinner } from "@/components/ui/spinner";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from "@/components/ui/input-group";
import { Label } from "@/components/ui/label";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { copyTextToClipboard } from "@/lib/clipboard";

export interface ShareDialogProps {
  sessionId: number;
  existingShareID?: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/**
 * Build the public-facing share URL. Mirrors the `basePath: "/web"` and
 * `trailingSlash: true` settings in `next.config.ts`, and uses the
 * `?id=<uuid>` query-string convention so the route stays compatible with
 * `output: "export"` (which cannot statically pre-render unknown UUIDs).
 *
 * The public share page lives at `/share/` (outside the `(dashboard)` route
 * group) so anonymous viewers are not redirected to /login by the auth guard.
 */
export function buildShareURL(shareID: string): string {
  if (typeof window === "undefined") return "";
  return `${window.location.origin}/web/share/?id=${encodeURIComponent(shareID)}`;
}

type ExpireOption = "1d" | "7d" | "30d" | "never" | "custom";

export function ShareDialog({ sessionId, existingShareID, open, onOpenChange }: ShareDialogProps) {
  const [creating, setCreating] = useState(false);
  const [shareURL, setShareURL] = useState<string | null>(
    existingShareID ? buildShareURL(existingShareID) : null,
  );
  const [expiresAt, setExpiresAt] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [expireOption, setExpireOption] = useState<ExpireOption>("1d");
  const [customDate, setCustomDate] = useState<Date | undefined>(undefined);

  const t = useT();

  const reset = useCallback(() => {
    setShareURL(existingShareID ? buildShareURL(existingShareID) : null);
    setExpiresAt(null);
    setCopied(false);
  }, [existingShareID]);

  const handleOpenChange = useCallback(
    (next: boolean) => {
      if (!next) reset();
      onOpenChange(next);
    },
    [onOpenChange, reset],
  );

  const createShare = useCallback(async () => {
    setCreating(true);
    try {
      const body: CreateShareReqBody = { sessionId, expiresIn: expireOption };
      if (expireOption === "custom" && customDate) {
        body.expiresAt = Math.floor(customDate.getTime() / 1000);
      }
      const rsp = await api.createShare(body);
      if (rsp.error) {
        if (rsp.error.code === 10004) {
          toast.error(t("share_dialog.already_shared"));
        } else {
          toast.error(rsp.error.message || t("share_dialog.create_failed"));
        }
        return;
      }
      if (!rsp.shareId) {
        toast.error(t("share_dialog.create_failed"));
        return;
      }
      setShareURL(buildShareURL(rsp.shareId));
      setExpiresAt(rsp.expiresAt ?? null);
    } catch (err) {
      showErrorToast(err, { title: t("share_dialog.create_failed") });
    } finally {
      setCreating(false);
    }
  }, [sessionId, expireOption, customDate, t]);

  const handleCopy = useCallback(async () => {
    if (!shareURL) return;
    if (await copyTextToClipboard(shareURL)) {
      setCopied(true);
      toast.success(t("share_dialog.copied_to_clipboard"));
      window.setTimeout(() => setCopied(false), 2000);
    } else {
      toast.error(t("share_dialog.copy_failed"));
    }
  }, [shareURL, t]);

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Share2 className="size-4 text-primary" />
            {t("share_dialog.title")}
          </DialogTitle>
          <DialogDescription className="min-h-[2.5rem]">
            {shareURL ? t("share_dialog.desc_created") : t("share_dialog.desc_create")}
          </DialogDescription>
        </DialogHeader>

        {shareURL ? (
          <div className="space-y-3">
            <InputGroup>
              <InputGroupInput
                readOnly
                value={shareURL}
                className="font-mono text-xs"
                onFocus={(e) => e.currentTarget.select()}
              />
              <InputGroupAddon align="inline-end">
                <InputGroupButton onClick={handleCopy}>
                  {copied ? <Check data-icon="inline-start" /> : <Copy data-icon="inline-start" />}
                  {copied ? t("share_dialog.copied") : t("share_dialog.copy")}
                </InputGroupButton>
              </InputGroupAddon>
            </InputGroup>
            {expiresAt && (
              <p className="text-xs text-muted-foreground">
                {t("share_dialog.expires_at")}{" "}
                <span className="font-medium text-foreground/80">
                  {new Date(expiresAt).toLocaleString()}
                </span>
              </p>
            )}
          </div>
        ) : (
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>{t("share_dialog.link_expiration")}</Label>
              <RadioGroup
                value={expireOption}
                onValueChange={(v) => setExpireOption(v as ExpireOption)}
                className="flex flex-wrap gap-2"
              >
                {[
                  { value: "1d", label: t("share_dialog.expire_1d") },
                  { value: "7d", label: t("share_dialog.expire_7d") },
                  { value: "30d", label: t("share_dialog.expire_30d") },
                  { value: "never", label: t("share_dialog.expire_never") },
                  { value: "custom", label: t("share_dialog.expire_custom") },
                ].map((opt) => (
                  <div key={opt.value} className="flex items-center gap-1.5">
                    <RadioGroupItem value={opt.value} id={`expire-${opt.value}`} />
                    <Label
                      htmlFor={`expire-${opt.value}`}
                      className="text-sm font-normal cursor-pointer"
                    >
                      {opt.label}
                    </Label>
                  </div>
                ))}
              </RadioGroup>
            </div>

            {expireOption === "custom" && (
              <Popover>
                <PopoverTrigger>
                  <button
                    type="button"
                    className={cn(
                      "flex w-full items-center rounded-md border border-input bg-background px-3 py-2 text-sm hover:bg-accent hover:text-accent-foreground",
                      !customDate && "text-muted-foreground",
                    )}
                  >
                    <CalendarIcon className="mr-2 size-4" />
                    {customDate ? format(customDate, "PPP") : t("share_dialog.pick_date")}
                  </button>
                </PopoverTrigger>
                <PopoverContent className="w-auto p-0" align="start">
                  <Calendar
                    mode="single"
                    selected={customDate}
                    onSelect={setCustomDate}
                    disabled={(date) => date < new Date()}
                  />
                </PopoverContent>
              </Popover>
            )}

            <div className="rounded-lg border border-dashed border-border/70 bg-muted/20 px-4 py-5 text-center text-sm text-muted-foreground">
              {t("share_dialog.warning")}
            </div>
          </div>
        )}

        <DialogFooter>
          <DialogClose render={<Button variant="outline" type="button" />}>
            {t("share_dialog.close")}
          </DialogClose>
          {!shareURL && (
            <Button type="button" onClick={createShare} disabled={creating}>
              {creating ? (
                <>
                  <Spinner data-icon="inline-start" />
                  {t("share_dialog.creating")}
                </>
              ) : (
                <>
                  <Share2 data-icon="inline-start" />
                  {t("share_dialog.create_link")}
                </>
              )}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
