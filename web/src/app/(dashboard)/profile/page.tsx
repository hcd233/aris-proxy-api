"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api-client";
import { useAuth } from "@/lib/auth-context";
import type { DetailedUser } from "@/lib/types";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { toast } from "sonner";

export default function ProfilePage() {
  const { user, isLoading } = useAuth();
  const [name, setName] = useState("");
  const [saving, setSaving] = useState(false);
  const [localUser, setLocalUser] = useState<DetailedUser | null>(null);

  /* eslint-disable react-hooks/set-state-in-effect -- Syncing local state from auth context requires setting state in effect */
  useEffect(() => {
    if (user) {
      setLocalUser(user);
      setName(user.name ?? "");
    }
  }, [user]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const handleSave = useCallback(async () => {
    setSaving(true);
    try {
      const rsp = await api.updateUser({ user: { name } });
      if (rsp.error) {
        toast.error(rsp.error.message);
        return;
      }
      if (rsp.user) {
        setLocalUser(rsp.user);
        toast.success("Profile updated");
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to update profile");
    } finally {
      setSaving(false);
    }
  }, [name]);

  if (isLoading) {
    return (
      <div className="space-y-4">
        <div className="h-8 w-40 animate-pulse rounded bg-muted" />
        <div className="h-48 w-full animate-pulse rounded bg-muted" />
      </div>
    );
  }

  if (!localUser) {
    return (
      <div className="flex items-center justify-center py-12">
        <p className="text-muted-foreground">Unable to load profile</p>
      </div>
    );
  }

  const initials = (localUser.name ?? localUser.email ?? "U")
    .split(" ")
    .map((n) => n[0])
    .join("")
    .toUpperCase()
    .slice(0, 2);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Profile</h1>
        <p className="text-sm text-muted-foreground">
          Manage your account settings
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Account Information</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-start gap-6">
            <Avatar size="lg">
              {localUser.avatar && (
                <AvatarImage src={localUser.avatar} alt={localUser.name ?? ""} />
              )}
              <AvatarFallback>{initials}</AvatarFallback>
            </Avatar>
            <div className="space-y-2">
              <div>
                <p className="font-medium">{localUser.name ?? "Unnamed"}</p>
                <p className="text-sm text-muted-foreground">{localUser.email ?? "—"}</p>
              </div>
              <Badge variant="secondary">{localUser.permission}</Badge>
              {localUser.createdAt && (
                <p className="text-xs text-muted-foreground">
                  Joined {new Date(localUser.createdAt).toLocaleDateString()}
                </p>
              )}
            </div>
          </div>
        </CardContent>
      </Card>

      <Separator />

      <Card>
        <CardHeader>
          <CardTitle>Edit Profile</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="max-w-sm space-y-4">
            <div className="space-y-1">
              <Label htmlFor="profile-name">Display Name</Label>
              <Input
                id="profile-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Enter your name"
              />
            </div>
            <div className="space-y-1">
              <Label>Email</Label>
              <Input value={localUser.email ?? ""} disabled className="opacity-60" />
              <p className="text-xs text-muted-foreground">
                Email is managed by your OAuth2 provider.
              </p>
            </div>
            <Button onClick={handleSave} disabled={saving || name === (localUser.name ?? "")}>
              {saving ? "Saving..." : "Save Changes"}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}