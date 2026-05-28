"use client";

/**
 * Share dialog — invoked from a session detail page. Calls
 * `POST /api/v1/session/share` to create a share link, then displays the
 * resulting public URL with a copy button and the expiry time.
 */

import { useCallback, useState } from "react";
import { Check, Copy, Loader2, Share2 } from "lucide-react";
import { toast } from "sonner";

import { api } from "@/lib/api-client";
import { ApiError } from "@/lib/api-client";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

export interface ShareDialogProps {
  sessionId: number;
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

export function ShareDialog({ sessionId, open, onOpenChange }: ShareDialogProps) {
  const [creating, setCreating] = useState(false);
  const [shareURL, setShareURL] = useState<string | null>(null);
  const [expiresAt, setExpiresAt] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const reset = useCallback(() => {
    setShareURL(null);
    setExpiresAt(null);
    setCopied(false);
  }, []);

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
      const rsp = await api.createShare({ sessionId });
      if (rsp.error) {
        if (rsp.error.code === 10004) {
          toast.error("This session is already shared. Revoke the existing link first.");
        } else {
          toast.error(rsp.error.message || "Failed to create share link");
        }
        return;
      }
      if (!rsp.shareId) {
        toast.error("Failed to create share link");
        return;
      }
      setShareURL(buildShareURL(rsp.shareId));
      setExpiresAt(rsp.expiresAt ?? null);
    } catch (err) {
      const msg =
        err instanceof ApiError
          ? `Failed to create share link (${err.status})`
          : err instanceof Error
            ? err.message
            : "Failed to create share link";
      toast.error(msg);
    } finally {
      setCreating(false);
    }
  }, [sessionId]);

  const handleCopy = useCallback(async () => {
    if (!shareURL) return;
    try {
      await navigator.clipboard.writeText(shareURL);
      setCopied(true);
      toast.success("Link copied to clipboard");
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      toast.error("Failed to copy link");
    }
  }, [shareURL]);

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Share2 className="size-4 text-primary" />
            Share session
          </DialogTitle>
          <DialogDescription>
            Generate a public link to this conversation. The link expires after
            24 hours and can be revoked anytime from the Shares page.
          </DialogDescription>
        </DialogHeader>

        {shareURL ? (
          <div className="space-y-3">
            <div className="flex items-stretch gap-2">
              <input
                readOnly
                value={shareURL}
                className="flex-1 rounded-md border border-input bg-muted/40 px-3 py-2 font-mono text-xs text-foreground/90 focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/40 focus-visible:outline-none"
                onFocus={(e) => e.currentTarget.select()}
              />
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={handleCopy}
                className="shrink-0 gap-1.5"
              >
                {copied ? (
                  <>
                    <Check className="size-3.5" />
                    Copied
                  </>
                ) : (
                  <>
                    <Copy className="size-3.5" />
                    Copy
                  </>
                )}
              </Button>
            </div>
            {expiresAt && (
              <p className="text-xs text-muted-foreground">
                Expires at{" "}
                <span className="font-medium text-foreground/80">
                  {new Date(expiresAt).toLocaleString()}
                </span>
              </p>
            )}
          </div>
        ) : (
          <div className="rounded-lg border border-dashed border-border/70 bg-muted/20 px-4 py-5 text-center text-sm text-muted-foreground">
            Anyone with the generated link will be able to read this session
            until it expires.
          </div>
        )}

        <DialogFooter>
          <DialogClose
            render={<Button variant="outline" type="button" />}
          >
            Close
          </DialogClose>
          {!shareURL && (
            <Button
              type="button"
              onClick={createShare}
              disabled={creating}
              className="gap-1.5"
            >
              {creating ? (
                <>
                  <Loader2 className="size-3.5 animate-spin" />
                  Creating...
                </>
              ) : (
                <>
                  <Share2 className="size-3.5" />
                  Create link
                </>
              )}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
