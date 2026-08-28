"use client";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useT } from "@/lib/i18n";
import type { EndpointForm } from "./shared";

export interface EndpointDialogProps {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  editingId: number | null;
  form: EndpointForm;
  setForm: React.Dispatch<React.SetStateAction<EndpointForm>>;
  userOptions: { id: number; name: string }[];
  isAdmin: boolean;
  saving: boolean;
  onSave: () => void;
}

/** 端点新建/编辑弹窗（admin 新建支持代建） */
export function EndpointDialog({
  open,
  onOpenChange,
  editingId,
  form,
  setForm,
  userOptions,
  isAdmin,
  saving,
  onSave,
}: EndpointDialogProps) {
  const t = useT();
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{editingId ? t("endpoints.edit") : t("endpoints.create")}</DialogTitle>
          <DialogDescription className="min-h-[2.5rem]">
            {editingId ? t("endpoints.edit_description") : t("endpoints.create_description")}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <div className="space-y-1">
            <Label htmlFor="ep-name">{t("endpoints.name")}</Label>
            <Input
              id="ep-name"
              placeholder={t("endpoints.name") + "..."}
              value={form.name}
              onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
            />
          </div>
          {!editingId && isAdmin && (
            <div className="space-y-1">
              <Label htmlFor="ep-owner">{t("upstream.owner")}</Label>
              <Select
                value={form.ownerUserID ? String(form.ownerUserID) : ""}
                onValueChange={(value) =>
                  setForm((f) => ({
                    ...f,
                    ownerUserID: value ? Number(value) : undefined,
                  }))
                }
              >
                <SelectTrigger id="ep-owner">
                  <SelectValue placeholder={t("upstream.owner_default")} />
                </SelectTrigger>
                <SelectContent>
                  {userOptions.map((u) => (
                    <SelectItem key={u.id} value={String(u.id)}>
                      {u.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className="text-[11px] text-muted-foreground">{t("upstream.owner_default")}</p>
            </div>
          )}
          <div className="space-y-1">
            <Label htmlFor="ep-openai-url">{t("endpoints.openai_base_url")}</Label>
            <Input
              id="ep-openai-url"
              placeholder="https://api.openai.com/v1"
              value={form.openaiBaseURL}
              onChange={(e) => setForm((f) => ({ ...f, openaiBaseURL: e.target.value }))}
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="ep-anthropic-url">{t("endpoints.anthropic_base_url")}</Label>
            <Input
              id="ep-anthropic-url"
              placeholder="https://api.anthropic.com/v1"
              value={form.anthropicBaseURL}
              onChange={(e) => setForm((f) => ({ ...f, anthropicBaseURL: e.target.value }))}
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="ep-apikey">{t("endpoints.api_key")}</Label>
            <Input
              id="ep-apikey"
              type="password"
              placeholder={editingId ? t("endpoints.keep_current") : t("endpoints.enter_api_key")}
              value={form.apiKey}
              onChange={(e) => setForm((f) => ({ ...f, apiKey: e.target.value }))}
            />
          </div>
          <div className="space-y-2">
            <Label>{t("endpoints.capabilities")}</Label>
            <div className="flex flex-col gap-2">
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={form.supportOpenAIChatCompletion}
                  onChange={(e) =>
                    setForm((f) => ({ ...f, supportOpenAIChatCompletion: e.target.checked }))
                  }
                  className="rounded"
                />
                {t("endpoints.openai_chat_label")}
              </label>
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={form.supportOpenAIResponse}
                  onChange={(e) =>
                    setForm((f) => ({ ...f, supportOpenAIResponse: e.target.checked }))
                  }
                  className="rounded"
                />
                {t("endpoints.openai_response_label")}
              </label>
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={form.supportAnthropicMessage}
                  onChange={(e) =>
                    setForm((f) => ({ ...f, supportAnthropicMessage: e.target.checked }))
                  }
                  className="rounded"
                />
                {t("endpoints.anthropic_messages_label")}
              </label>
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("common.cancel")}
          </Button>
          <Button onClick={onSave} disabled={!form.name.trim() || saving}>
            {saving
              ? t("common.saving")
              : editingId
                ? t("endpoints.update")
                : t("common.create")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
