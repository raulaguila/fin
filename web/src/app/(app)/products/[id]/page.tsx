"use client";

import { useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { FormError, PageBody, PageHeader } from "@/components/bits";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { EmptyState } from "@/components/empty-state";
import { QueryErrorState } from "@/components/query-error-state";
import { ListSkeleton } from "@/components/skeletons";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { ApiError } from "@/lib/api/client";
import { products } from "@/lib/api/endpoints";
import type { Product } from "@/lib/api/types";
import { useMe } from "@/lib/context/me";
import { errorMessage } from "@/lib/errors";
import { useUnsavedGuard } from "@/lib/hooks/use-unsaved-guard";
import { useI18n } from "@/lib/i18n/provider";
import { can } from "@/lib/permissions";
import { useCrumb } from "@/lib/store/crumb";

/** Create when `product` is null, edit otherwise. Price stays a decimal string end to end. */
function ProductForm({ product, editable }: { product: Product | null; editable: boolean }) {
  const { t } = useI18n();
  const router = useRouter();
  const client = useQueryClient();
  const [name, setName] = useState(product?.name ?? "");
  const [price, setPrice] = useState(product?.price ?? "");
  const [enabled, setEnabled] = useState(product?.enabled ?? true);
  const [error, setError] = useState<string | null>(null);
  const dirty = product
    ? name.trim() !== product.name || price.trim() !== product.price || enabled !== product.enabled
    : name.trim() !== "" || price.trim() !== "";
  useUnsavedGuard(editable && dirty, t("roles.leave_unsaved"));
  const save = useMutation({
    mutationFn: () => {
      const input = { name: name.trim(), price: price.trim().replace(",", "."), enabled };
      return product ? products.update(product.id, input) : products.create(input);
    },
    onSuccess: (saved) => {
      setError(null);
      client.setQueryData(["products", "detail", saved.id], saved);
      void client.invalidateQueries({ queryKey: ["products"] });
      toast.success(t("common.saved"));
      router.push("/products");
    },
    onError: (err) => setError(errorMessage(err, "common.save_failed", t)),
  });

  return (
    <Card className="max-w-xl">
      <CardHeader><CardTitle>{t("products.details")}</CardTitle></CardHeader>
      <CardContent>
        <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); save.mutate(); }}>
          <FormError message={error} />
          <div className="space-y-2">
            <Label htmlFor="p-name">{t("auth.name")}</Label>
            <Input id="p-name" required autoFocus={!product} maxLength={120} disabled={!editable} value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="p-price">{t("products.price")}</Label>
            <Input id="p-price" required inputMode="decimal" pattern={"\\d+([.,]\\d{1,2})?"} placeholder="0.00" disabled={!editable} value={price} onChange={(e) => setPrice(e.target.value)} className="font-mono" />
          </div>
          <label className="flex items-center gap-3 text-sm">
            <Switch checked={enabled} disabled={!editable} onCheckedChange={setEnabled} aria-label={t("products.enabled")} />
            {t(enabled ? "roles.enabled" : "roles.disabled")}
          </label>
          {editable && (
            <div className="flex justify-end gap-2">
              <Button type="button" variant="ghost" disabled={save.isPending} onClick={() => router.push("/products")}>{t("common.cancel")}</Button>
              <Button type="submit" disabled={save.isPending || !dirty}>
                {save.isPending && <Loader2 className="size-4 animate-spin" />}
                {t(product ? "common.save" : "common.create")}
              </Button>
            </div>
          )}
        </form>
      </CardContent>
    </Card>
  );
}

export default function ProductPage() {
  const { t } = useI18n();
  const router = useRouter();
  const client = useQueryClient();
  const { user } = useMe();
  const { id } = useParams<{ id: string }>();
  const creating = id === "new";
  const detail = useQuery({ queryKey: ["products", "detail", id], queryFn: () => products.get(id), enabled: !creating });
  const product = detail.data;
  const [confirmDelete, setConfirmDelete] = useState(false);
  const editable = can(user, "products", creating ? "create" : "update");
  useCrumb(creating ? t("products.new") : product?.name);

  return (
    <>
      <PageHeader
        title={creating ? t("products.new") : (product?.name ?? t("nav.products"))}
        action={product && can(user, "products", "delete") && (
          <Button variant="destructive" onClick={() => setConfirmDelete(true)}>{t("common.delete")}</Button>
        )}
      />
      <PageBody>
        {creating ? (
          <ProductForm product={null} editable={editable} />
        ) : detail.isPending ? (
          <ListSkeleton rows={3} />
        ) : detail.error instanceof ApiError && detail.error.code === "NOT_FOUND" ? (
          <EmptyState title={t("error.not_found")} />
        ) : detail.isError || !product ? (
          <QueryErrorState error={detail.error} onRetry={() => void detail.refetch()} />
        ) : (
          <ProductForm key={product.id} product={product} editable={editable} />
        )}
      </PageBody>
      <ConfirmDialog
        open={confirmDelete}
        onOpenChange={setConfirmDelete}
        title={t("products.delete")}
        description={product?.name}
        destructive
        confirmLabel={t("common.delete")}
        onConfirm={async () => {
          await products.remove(id);
          router.push("/products");
          client.removeQueries({ queryKey: ["products", "detail", id] });
          void client.invalidateQueries({ queryKey: ["products"] });
        }}
      />
    </>
  );
}
