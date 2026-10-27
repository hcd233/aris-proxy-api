"use client";

import { useCallback, useEffect, useState } from "react";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";

import { api } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
import { useT } from "@/lib/i18n";
import type { HarnessRuleItem } from "@/lib/types";
import { PageHeader } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";

interface RuleDraft {
  id?: number;
  pattern: string;
  name: string;
  sort: number;
}

const EMPTY_DRAFT: RuleDraft = { pattern: "", name: "", sort: 0 };

export default function HarnessPage() {
  const t = useT();
  const [rules, setRules] = useState<HarnessRuleItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [draft, setDraft] = useState<RuleDraft | null>(null);
  const [deleting, setDeleting] = useState<HarnessRuleItem | null>(null);
  const [previewUA, setPreviewUA] = useState("");
  const [previewResult, setPreviewResult] = useState<{
    harness: string;
    rule?: HarnessRuleItem;
  } | null>(null);
  const [previewing, setPreviewing] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const rsp = await api.listHarnessRules();
      setRules(rsp.rules ?? []);
    } catch (err) {
      showErrorToast(err);
    } finally {
      setLoading(false);
    }
  }, []);

  /* eslint-disable react-hooks/set-state-in-effect -- 首次挂载即拉取规则列表，属于异步取数的必然 setState */
  useEffect(() => {
    void load();
  }, [load]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const save = useCallback(async () => {
    if (!draft) return;
    setSaving(true);
    try {
      const body = { pattern: draft.pattern, name: draft.name, sort: draft.sort };
      if (draft.id) {
        await api.updateHarnessRule(draft.id, body);
      } else {
        await api.createHarnessRule(body);
      }
      toast.success(t("harness.saved"));
      setDraft(null);
      await load();
    } catch (err) {
      showErrorToast(err);
    } finally {
      setSaving(false);
    }
  }, [draft, load, t]);

  const remove = useCallback(async () => {
    if (!deleting) return;
    try {
      await api.deleteHarnessRule(deleting.id);
      toast.success(t("harness.deleted"));
      setDeleting(null);
      await load();
    } catch (err) {
      showErrorToast(err);
    }
  }, [deleting, load, t]);

  const preview = useCallback(async () => {
    if (!previewUA.trim()) return;
    setPreviewing(true);
    try {
      const rsp = await api.previewHarnessRule(previewUA.trim());
      setPreviewResult({ harness: rsp.harness, rule: rsp.rule });
    } catch (err) {
      showErrorToast(err);
    } finally {
      setPreviewing(false);
    }
  }, [previewUA]);

  return (
    <div className="space-y-6">
      <PageHeader
        title={t("harness.title")}
        description={t("harness.subtitle")}
        actions={
          <Button size="sm" className="gap-1.5" onClick={() => setDraft({ ...EMPTY_DRAFT })}>
            <Plus className="size-3.5" />
            {t("harness.add")}
          </Button>
        }
      />

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">{t("harness.preview")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex flex-col gap-2 sm:flex-row">
            <Input
              value={previewUA}
              onChange={(e) => setPreviewUA(e.target.value)}
              placeholder={t("harness.preview_placeholder")}
              className="font-mono text-xs"
            />
            <Button size="sm" onClick={() => void preview()} disabled={previewing}>
              {t("harness.preview_action")}
            </Button>
          </div>
          {previewResult && (
            <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
              <Badge variant={previewResult.harness === "Other" ? "outline" : "secondary"}>
                {previewResult.harness}
              </Badge>
              <span>
                {previewResult.rule
                  ? `${t("harness.preview_hit")} ${previewResult.rule.pattern} → ${previewResult.rule.name}`
                  : t("harness.preview_other")}
              </span>
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardContent className="pt-6">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("harness.pattern")}</TableHead>
                <TableHead>{t("harness.name")}</TableHead>
                <TableHead className="w-[100px]">{t("harness.sort")}</TableHead>
                <TableHead className="w-[104px] sr-only">{t("harness.actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rules.map((rule) => (
                <TableRow key={rule.id}>
                  <TableCell className="font-mono text-xs">{rule.pattern}</TableCell>
                  <TableCell>{rule.name}</TableCell>
                  <TableCell className="tabular-nums">{rule.sort}</TableCell>
                  <TableCell>
                    <div className="flex items-center gap-1">
                      <Button
                        size="icon"
                        variant="ghost"
                        aria-label={t("harness.edit")}
                        onClick={() =>
                          setDraft({
                            id: rule.id,
                            pattern: rule.pattern,
                            name: rule.name,
                            sort: rule.sort,
                          })
                        }
                      >
                        <Pencil className="size-4" />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        aria-label={t("harness.delete")}
                        onClick={() => setDeleting(rule)}
                      >
                        <Trash2 className="size-4" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
              {!loading && rules.length === 0 && (
                <TableRow>
                  <TableCell colSpan={4} className="text-center text-sm text-muted-foreground">
                    {t("harness.empty")}
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Dialog open={draft !== null} onOpenChange={(open) => !open && setDraft(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{draft?.id ? t("harness.edit") : t("harness.add")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <div className="space-y-1.5">
              <Label htmlFor="harness-pattern">{t("harness.pattern")}</Label>
              <Input
                id="harness-pattern"
                value={draft?.pattern ?? ""}
                onChange={(e) => setDraft((d) => (d ? { ...d, pattern: e.target.value } : d))}
                placeholder="deepseek-harness/"
                className="font-mono text-xs"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="harness-name">{t("harness.name")}</Label>
              <Input
                id="harness-name"
                value={draft?.name ?? ""}
                onChange={(e) => setDraft((d) => (d ? { ...d, name: e.target.value } : d))}
                placeholder="DSH"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="harness-sort">{t("harness.sort")}</Label>
              <Input
                id="harness-sort"
                type="number"
                value={draft?.sort ?? 0}
                onChange={(e) =>
                  setDraft((d) =>
                    d ? { ...d, sort: Number.parseInt(e.target.value || "0", 10) || 0 } : d,
                  )
                }
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setDraft(null)}>
              {t("harness.cancel")}
            </Button>
            <Button
              onClick={() => void save()}
              disabled={saving || !draft?.pattern || !draft?.name}
            >
              {t("harness.save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={deleting !== null} onOpenChange={(open) => !open && setDeleting(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("harness.delete")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("harness.delete_confirm").replace("{pattern}", deleting?.pattern ?? "")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("harness.cancel")}</AlertDialogCancel>
            <AlertDialogAction onClick={() => void remove()}>
              {t("harness.delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
