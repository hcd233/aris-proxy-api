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
    <div className="space-y-8">
      <div>
        <h1 className="font-display text-3xl font-semibold tracking-tight text-foreground">Profile</h1>
        <p className="mt-1.5 text-sm text-muted-foreground">
          Manage your account settings
        </p>
      </div>

      <div className="grid gap-6 lg:grid-cols-[1fr_1.5fr]">
        <Card>
          <CardHeader>
            <CardTitle className="font-display">Account Information</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex flex-col items-center text-center gap-4">
              <Avatar size="lg" className="size-20">
                {localUser.avatar && (
                  <AvatarImage src={localUser.avatar} alt={localUser.name ?? ""} />
                )}
                <AvatarFallback className="bg-secondary text-2xl font-medium">{initials}</AvatarFallback>
              </Avatar>
              <div className="space-y-2">
                <div>
                  <p className="font-display text-lg font-medium">{localUser.name ?? "Unnamed"}</p>
                  <p className="text-sm text-muted-foreground">{localUser.email ?? "—"}</p>
                </div>
                <Badge variant="secondary" className="text-xs">{localUser.permission}</Badge>
                {localUser.createdAt && (
                  <p className="text-xs text-muted-foreground">
                    Joined {new Date(localUser.createdAt).toLocaleDateString()}
                  </p>
                )}
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="font-display">Edit Profile</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="max-w-md space-y-5">
              <div className="space-y-1.5">
                <Label htmlFor="profile-name" className="text-sm font-medium">Display Name</Label>
                <Input
                  id="profile-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="Enter your name"
                />
              </div>
              <div className="space-y-1.5">
                <Label className="text-sm font-medium">Email</Label>
                <Input value={localUser.email ?? ""} disabled className="opacity-60 bg-muted/30" />
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
    </div>
  );
}