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
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from "@/components/ui/field";
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
        <FieldGroup className="gap-4">
          <Field>
            <FieldLabel htmlFor="ep-name">{t("endpoints.name")}</FieldLabel>
            <Input
              id="ep-name"
              placeholder={t("endpoints.name") + "..."}
              value={form.name}
              onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
            />
          </Field>
          {!editingId && isAdmin && (
            <Field>
              <FieldLabel htmlFor="ep-owner">{t("upstream.owner")}</FieldLabel>
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
              <FieldDescription>{t("upstream.owner_default")}</FieldDescription>
            </Field>
          )}
          <Field>
            <FieldLabel htmlFor="ep-openai-url">{t("endpoints.openai_base_url")}</FieldLabel>
            <Input
              id="ep-openai-url"
              placeholder="https://api.openai.com/v1"
              value={form.openaiBaseURL}
              onChange={(e) => setForm((f) => ({ ...f, openaiBaseURL: e.target.value }))}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="ep-anthropic-url">{t("endpoints.anthropic_base_url")}</FieldLabel>
            <Input
              id="ep-anthropic-url"
              placeholder="https://api.anthropic.com/v1"
              value={form.anthropicBaseURL}
              onChange={(e) => setForm((f) => ({ ...f, anthropicBaseURL: e.target.value }))}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="ep-apikey">{t("endpoints.api_key")}</FieldLabel>
            <Input
              id="ep-apikey"
              type="password"
              placeholder={editingId ? t("endpoints.keep_current") : t("endpoints.enter_api_key")}
              value={form.apiKey}
              onChange={(e) => setForm((f) => ({ ...f, apiKey: e.target.value }))}
            />
          </Field>
          <FieldSet>
            <FieldLegend variant="label">{t("endpoints.capabilities")}</FieldLegend>
            <FieldGroup className="gap-2">
              <Field orientation="horizontal">
                <Checkbox
                  id="ep-supportOpenAIChatCompletion"
                  checked={form.supportOpenAIChatCompletion}
                  onCheckedChange={(checked) =>
                    setForm((f) => ({ ...f, supportOpenAIChatCompletion: checked }))
                  }
                />
                <FieldLabel htmlFor="ep-supportOpenAIChatCompletion" className="font-normal">
                  {t("endpoints.openai_chat_label")}
                </FieldLabel>
              </Field>
              <Field orientation="horizontal">
                <Checkbox
                  id="ep-supportOpenAIResponse"
                  checked={form.supportOpenAIResponse}
                  onCheckedChange={(checked) =>
                    setForm((f) => ({ ...f, supportOpenAIResponse: checked }))
                  }
                />
                <FieldLabel htmlFor="ep-supportOpenAIResponse" className="font-normal">
                  {t("endpoints.openai_response_label")}
                </FieldLabel>
              </Field>
              <Field orientation="horizontal">
                <Checkbox
                  id="ep-supportAnthropicMessage"
                  checked={form.supportAnthropicMessage}
                  onCheckedChange={(checked) =>
                    setForm((f) => ({ ...f, supportAnthropicMessage: checked }))
                  }
                />
                <FieldLabel htmlFor="ep-supportAnthropicMessage" className="font-normal">
                  {t("endpoints.anthropic_messages_label")}
                </FieldLabel>
              </Field>
              <Field orientation="horizontal">
                <Checkbox
                  id="ep-supportOpenAIDecision"
                  checked={form.supportOpenAIDecision}
                  onCheckedChange={(checked) =>
                    setForm((f) => ({ ...f, supportOpenAIDecision: checked }))
                  }
                />
                <FieldLabel htmlFor="ep-supportOpenAIDecision" className="font-normal">
                  {t("endpoints.openai_decision_label")}
                </FieldLabel>
              </Field>
            </FieldGroup>
          </FieldSet>
        </FieldGroup>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("common.cancel")}
          </Button>
          <Button onClick={onSave} disabled={!form.name.trim() || saving}>
            {saving ? t("common.saving") : editingId ? t("endpoints.update") : t("common.create")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
