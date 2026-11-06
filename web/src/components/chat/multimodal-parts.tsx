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
  | { status: "idle" | "loading" | "failed" | "no_preview" }
  | { status: "ready"; src: string; href?: string };

/**
 * 已外置图片：默认只显示「类型 · 大小」占位卡片，点击后才请求接口并加载图片。
 * 没有 SessionImageProvider（如分享页）时卡片不可点击。
 */
function PartBlobImage({ sha, mime, size }: { sha: string; mime?: string; size?: number }) {
  const t = useT();
  const sessionId = useSessionImageId();
  const [state, setState] = useState<BlobImageState>({ status: "idle" });

  const load = async () => {
    if (sessionId === null || state.status === "loading" || state.status === "no_preview") return;
    setState({ status: "loading" });
    try {
      const rsp = await api.getSessionImage(sessionId, sha);
      if (!rsp.image) {
        setState({ status: "failed" });
        return;
      }
      // 接口正常返回但既无 url 也无 dataUrl：服务端判定不可预览（如超过存储上限的原图）
      if (!rsp.image.url && !rsp.image.dataUrl) {
        setState({ status: "no_preview" });
        return;
      }
      const src = pickImageSrc(rsp.image);
      setState(src ? { status: "ready", src, href: rsp.image.url } : { status: "failed" });
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
          : state.status === "no_preview"
            ? t("chat.image_no_preview")
            : t("chat.image_click_to_view");
  const meta = formatImageMeta(mime, size) || t("chat.image_untitled");
  const clickable = sessionId !== null && state.status !== "no_preview";

  return (
    <Attachment>
      <AttachmentMedia>
        <ImageIcon className="size-4" />
      </AttachmentMedia>
      <AttachmentContent>
        <TooltipRoot>
          <TooltipTrigger render={<AttachmentTitle>{meta}</AttachmentTitle>} />
          <TooltipContent side="top" className="max-w-xs break-all">
            {meta}
          </TooltipContent>
        </TooltipRoot>
        <TooltipRoot>
          <TooltipTrigger render={<AttachmentDescription>{hint}</AttachmentDescription>} />
          <TooltipContent side="top" className="max-w-xs break-all">
            {hint}
          </TooltipContent>
        </TooltipRoot>
      </AttachmentContent>
      {clickable && (
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
