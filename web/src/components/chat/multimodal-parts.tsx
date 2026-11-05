import { useState } from "react";
import { FileText, ImageIcon, Music2, ShieldAlert } from "lucide-react";
import { Alert, AlertDescription } from "@/components/ui/alert";
import {
  Attachment,
  AttachmentContent,
  AttachmentDescription,
  AttachmentMedia,
  AttachmentTitle,
  AttachmentTrigger,
} from "@/components/ui/attachment";
import { api } from "@/lib/api-client";
import { useT } from "@/lib/i18n";
import { isSafeUrl } from "@/lib/utils";
import { TooltipRoot, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import type { ContentPart } from "./content-extract";
import { imageURLOf } from "./content-extract";
import { formatImageMeta, parseBlobRef, pickImageSrc } from "./blob-image";
import { useSessionImageId } from "./session-image-context";

/** 图片附件：缩略图卡片，点击在新标签页打开原图（仅 http/https）。 */
function PartImage({ url }: { url: string }) {
  const t = useT();
  if (!isSafeUrl(url)) return null;
  return (
    <Attachment orientation="vertical">
      <AttachmentMedia variant="image">
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img src={url} alt="" />
      </AttachmentMedia>
      <AttachmentTrigger
        render={
          <a
            href={url}
            target="_blank"
            rel="noopener noreferrer"
            aria-label={t("chat.open_image")}
          />
        }
      />
    </Attachment>
  );
}

type BlobImageState =
  { status: "idle" | "loading" | "failed" } | { status: "ready"; src: string; href?: string };

/**
 * 已外置图片：默认只显示「类型 · 大小」占位卡片，点击后才请求接口并加载图片。
 * 没有 SessionImageProvider（如分享页）时卡片不可点击。
 */
function PartBlobImage({ sha, mime, size }: { sha: string; mime?: string; size?: number }) {
  const t = useT();
  const sessionId = useSessionImageId();
  const [state, setState] = useState<BlobImageState>({ status: "idle" });

  const load = async () => {
    if (sessionId === null || state.status === "loading") return;
    setState({ status: "loading" });
    try {
      const rsp = await api.getSessionImage(sessionId, sha);
      const src = rsp.image ? pickImageSrc(rsp.image) : null;
      setState(src ? { status: "ready", src, href: rsp.image?.url } : { status: "failed" });
    } catch {
      setState({ status: "failed" });
    }
  };

  if (state.status === "ready") {
    return (
      <Attachment orientation="vertical">
        <AttachmentMedia variant="image">
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={state.src} alt="" onError={() => setState({ status: "failed" })} />
        </AttachmentMedia>
        {state.href && (
          <AttachmentTrigger
            render={
              <a
                href={state.href}
                target="_blank"
                rel="noopener noreferrer"
                aria-label={t("chat.open_image")}
              />
            }
          />
        )}
      </Attachment>
    );
  }

  const hint =
    sessionId === null
      ? t("chat.image_unavailable")
      : state.status === "loading"
        ? t("chat.image_loading")
        : state.status === "failed"
          ? t("chat.image_load_failed")
          : t("chat.image_click_to_view");
  const meta = formatImageMeta(mime, size);

  return (
    <Attachment>
      <AttachmentMedia>
        <ImageIcon className="size-4" />
      </AttachmentMedia>
      <AttachmentContent>
        <TooltipRoot>
          <TooltipTrigger render={<AttachmentTitle>{meta || "Image"}</AttachmentTitle>} />
          <TooltipContent side="top" className="max-w-xs break-all">
            {meta || "Image"}
          </TooltipContent>
        </TooltipRoot>
        <TooltipRoot>
          <TooltipTrigger render={<AttachmentDescription>{hint}</AttachmentDescription>} />
          <TooltipContent side="top" className="max-w-xs break-all">
            {hint}
          </TooltipContent>
        </TooltipRoot>
      </AttachmentContent>
      {sessionId !== null && (
        <AttachmentTrigger onClick={load} disabled={state.status === "loading"} aria-label={hint} />
      )}
    </Attachment>
  );
}

function PartIconCard({
  icon,
  label,
  meta,
}: {
  icon: React.ReactNode;
  label: string;
  meta?: string;
}) {
  return (
    <Attachment>
      <AttachmentMedia>{icon}</AttachmentMedia>
      <AttachmentContent>
        <TooltipRoot>
          <TooltipTrigger render={<AttachmentTitle>{label}</AttachmentTitle>} />
          <TooltipContent side="top" className="max-w-xs break-all">
            {label}
          </TooltipContent>
        </TooltipRoot>
        {meta && (
          <TooltipRoot>
            <TooltipTrigger render={<AttachmentDescription>{meta}</AttachmentDescription>} />
            <TooltipContent side="top" className="max-w-xs break-all">
              {meta}
            </TooltipContent>
          </TooltipRoot>
        )}
      </AttachmentContent>
    </Attachment>
  );
}

function PartRefusal({ text }: { text: string }) {
  return (
    <Alert variant="destructive" className="my-2">
      <ShieldAlert />
      <AlertDescription className="whitespace-pre-wrap break-words">{text}</AlertDescription>
    </Alert>
  );
}

export function MultimodalParts({ parts }: { parts: ContentPart[] }) {
  if (parts.length === 0) return null;
  return (
    <div className="flex flex-wrap gap-2">
      {parts.map((part, i) => {
        switch (part.type) {
          case "image_url": {
            const url = imageURLOf(part);
            const sha = parseBlobRef(url);
            if (sha) {
              return (
                <PartBlobImage key={i} sha={sha} mime={part.image_mime} size={part.image_size} />
              );
            }
            return url ? <PartImage key={i} url={url} /> : null;
          }
          case "input_audio": {
            const fmt = part.input_audio?.format ?? part.audio_format ?? "audio";
            return (
              <PartIconCard
                key={i}
                icon={<Music2 className="size-4" />}
                label="Audio attachment"
                meta={String(fmt).toUpperCase()}
              />
            );
          }
          case "file": {
            const filename = part.file?.filename ?? part.filename ?? "file";
            const fileID = part.file?.file_id ?? part.file_id;
            return (
              <PartIconCard
                key={i}
                icon={<FileText className="size-4" />}
                label={filename}
                meta={fileID ? `id: ${fileID}` : undefined}
              />
            );
          }
          case "refusal":
            return part.refusal || part.text ? (
              <PartRefusal key={i} text={(part.refusal ?? part.text) as string} />
            ) : null;
          default:
            return null;
        }
      })}
    </div>
  );
}
