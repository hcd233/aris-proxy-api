import { FileText, Music2, ShieldAlert } from "lucide-react";
import type { ContentPart } from "./content-extract";
import { imageURLOf } from "./content-extract";

function PartImage({ url }: { url: string }) {
  return (
    <div className="my-2 inline-block max-w-sm overflow-hidden rounded-lg border border-border/60 bg-muted/40">
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img src={url} alt="" className="block h-auto max-h-80 w-full object-contain" />
    </div>
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
    <div className="my-2 inline-flex items-center gap-2.5 rounded-lg border border-border/60 bg-muted/40 px-3 py-2">
      <div className="flex size-8 shrink-0 items-center justify-center rounded-md bg-background text-muted-foreground">
        {icon}
      </div>
      <div className="min-w-0">
        <p className="truncate text-sm font-medium text-foreground">{label}</p>
        {meta && <p className="truncate text-[11px] text-muted-foreground">{meta}</p>}
      </div>
    </div>
  );
}

function PartRefusal({ text }: { text: string }) {
  return (
    <div className="my-2 flex items-start gap-2 rounded-lg border border-destructive/40 bg-destructive/5 px-3 py-2 text-sm text-destructive">
      <ShieldAlert className="mt-0.5 size-4 shrink-0" />
      <span className="whitespace-pre-wrap break-words">{text}</span>
    </div>
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
