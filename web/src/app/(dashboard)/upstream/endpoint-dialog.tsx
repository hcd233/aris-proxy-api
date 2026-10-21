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
import type { EndpointAuthType } from "@/lib/types";
import type { EndpointForm } from "./shared";
import { oauthCodeHintKey, type OAuthAuthorizeForm } from "./oauth-form";

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
  /** 编辑态目标为 OAuth 端点：连接字段服务端固定，仅名称可改 */
  editingOAuth: boolean;
  /** 新建时的认证类型选择（api_key 走原表单；oauth_* 走授权面板） */
  authSelection: EndpointAuthType;
  onSelectAuth: (v: EndpointAuthType) => void;
  /** OAuth 模式：null=API Key 表单；"create"=绑定订阅账号；"reauth"=重新授权 */
  oauthMode: "create" | "reauth" | null;
  oauthForm: OAuthAuthorizeForm;
  setOAuthForm: React.Dispatch<React.SetStateAction<OAuthAuthorizeForm>>;
  oauthAuthorizeURL: string | null;
  oauthBusy: boolean;
  onStartOAuth: () => void;
  onSubmitOAuth: () => void;
}

/** 端点新建/编辑弹窗（admin 新建支持代建；新建可选 API Key / 订阅账号两种接入方式） */
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
  editingOAuth,
  authSelection,
  onSelectAuth,
  oauthMode,
  oauthForm,
  setOAuthForm,
  oauthAuthorizeURL,
  oauthBusy,
  onStartOAuth,
  onSubmitOAuth,
}: EndpointDialogProps) {
  const t = useT();
  // Base UI Select 只有传 items 时，触发按钮才渲染选项文案而不是原始枚举值
  const ownerItems = userOptions.map((u) => ({ value: String(u.id), label: u.name }));
  const authTypeItems: { value: EndpointAuthType; label: string }[] = [
    { value: "api_key", label: t("endpoints.auth_api_key") },
    { value: "oauth_anthropic", label: t("oauth.badge_claude") },
    { value: "oauth_openai", label: t("oauth.badge_codex") },
  ];
  const visibilityItems: { value: "private" | "public"; label: string }[] = [
    { value: "private", label: t("endpoints.visibility_private") },
    { value: "public", label: t("endpoints.visibility_public") },
  ];
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
                items={ownerItems}
                onValueChange={(value) =>
                  setForm((f) => ({
                    ...f,
                    ownerUserID: value ? Number(value) : undefined,
                    // 代建给他人的端点只能是私有：避免出现「归属他人却公开」的端点。
                    visibility: value ? "private" : f.visibility,
                  }))
                }
              >
                <SelectTrigger id="ep-owner">
                  <SelectValue placeholder={t("upstream.owner_default")} />
                </SelectTrigger>
                <SelectContent>
                  {ownerItems.map((item) => (
                    <SelectItem key={item.value} value={item.value}>
                      {item.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <FieldDescription>{t("upstream.owner_default")}</FieldDescription>
            </Field>
          )}
          {isAdmin && (
            <Field>
              <FieldLabel htmlFor="ep-visibility">{t("endpoints.visibility")}</FieldLabel>
              <Select
                value={form.visibility}
                items={visibilityItems}
                disabled={Boolean(form.ownerUserID)}
                onValueChange={(v) =>
                  setForm((f) => ({ ...f, visibility: v as "private" | "public" }))
                }
              >
                <SelectTrigger id="ep-visibility">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {visibilityItems.map((item) => (
                    <SelectItem key={item.value} value={item.value}>
                      {item.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <FieldDescription>{t("endpoints.visibility_desc")}</FieldDescription>
            </Field>
          )}
          {!editingId && (
            <Field>
              <FieldLabel htmlFor="ep-auth-type">{t("endpoints.auth_type")}</FieldLabel>
              <Select
                value={authSelection}
                items={authTypeItems}
                onValueChange={(v) => onSelectAuth(v as EndpointAuthType)}
              >
                <SelectTrigger id="ep-auth-type">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {authTypeItems.map((item) => (
                    <SelectItem key={item.value} value={item.value}>
                      {item.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <FieldDescription>{t("endpoints.auth_type_desc")}</FieldDescription>
            </Field>
          )}
          {oauthMode !== null && (
            <>
              <Field>
                <FieldLabel htmlFor="ep-oauth-link">{t("oauth.authorize_link")}</FieldLabel>
                <div className="flex items-center gap-2">
                  <Input
                    id="ep-oauth-link"
                    readOnly
                    value={oauthAuthorizeURL ?? ""}
                    placeholder={oauthBusy ? t("common.loading") : ""}
                  />
                  <Button
                    type="button"
                    variant="outline"
                    disabled={oauthBusy || !oauthAuthorizeURL}
                    onClick={onStartOAuth}
                  >
                    {t("oauth.open_link")}
                  </Button>
                </div>
                <FieldDescription>{t("oauth.authorize_link_desc")}</FieldDescription>
              </Field>
              <Field>
                <FieldLabel htmlFor="ep-oauth-code">{t("oauth.code_label")}</FieldLabel>
                <Input
                  id="ep-oauth-code"
                  value={oauthForm.code}
                  onChange={(e) => setOAuthForm((f) => ({ ...f, code: e.target.value }))}
                  placeholder={t(oauthCodeHintKey(oauthForm.provider))}
                />
                <FieldDescription>{t(oauthCodeHintKey(oauthForm.provider))}</FieldDescription>
              </Field>
            </>
          )}
          {oauthMode === null && !editingOAuth && (
            <Field>
              <FieldLabel htmlFor="ep-openai-url">{t("endpoints.openai_base_url")}</FieldLabel>
              <Input
                id="ep-openai-url"
                placeholder="https://api.openai.com/v1"
                value={form.openaiBaseURL}
                onChange={(e) => setForm((f) => ({ ...f, openaiBaseURL: e.target.value }))}
              />
            </Field>
          )}
          {oauthMode === null && !editingOAuth && (
            <Field>
              <FieldLabel htmlFor="ep-anthropic-url">
                {t("endpoints.anthropic_base_url")}
              </FieldLabel>
              <Input
                id="ep-anthropic-url"
                placeholder="https://api.anthropic.com/v1"
                value={form.anthropicBaseURL}
                onChange={(e) => setForm((f) => ({ ...f, anthropicBaseURL: e.target.value }))}
              />
            </Field>
          )}
          {oauthMode === null && !editingOAuth && (
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
          )}
          {oauthMode === null && !editingOAuth && (
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
          )}
        </FieldGroup>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("common.cancel")}
          </Button>
          {oauthMode !== null ? (
            <Button
              onClick={onSubmitOAuth}
              disabled={!form.name.trim() || !oauthForm.code.trim() || oauthBusy || saving}
            >
              {oauthBusy || saving ? t("common.saving") : t("oauth.submit")}
            </Button>
          ) : (
            <Button onClick={onSave} disabled={!form.name.trim() || saving}>
              {saving ? t("common.saving") : editingId ? t("endpoints.update") : t("common.create")}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
