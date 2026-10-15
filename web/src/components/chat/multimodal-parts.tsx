import { FileText, Music2, ShieldAlert } from "lucide-react";
import { Alert, AlertDescription } from "@/components/ui/alert";
import {
  Attachment,
  AttachmentContent,
  AttachmentDescription,
  AttachmentMedia,
  AttachmentTitle,
  AttachmentTrigger,
} from "@/components/ui/attachment";
import { useT } from "@/lib/i18n";
import { TooltipRoot, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import type { ContentPart } from "./content-extract";
import { imageURLOf } from "./content-extract";

function isSafeUrl(url: string): boolean {
  return url.startsWith("http://") || url.startsWith("https://");
}

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
