import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import {
    AlertCircle,
    ArrowLeft,
    Calendar,
    CheckCircle2,
    Link as LinkIcon,
    Loader2,
    Plus,
    RefreshCw,
    Store,
    Unlink
} from "lucide-react";
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
    ApiError,
    disconnectShopifyShop,
    getShopifyAuthorizeUrl,
    listShopifyShops,
    syncShopifyShop,
    type ShopifyShop,
} from "../services/api";
import { logout } from "../services/auth";

function ShopifyView() {
  const navigate = useNavigate();

  const [shopInput, setShopInput] = useState<string>("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [items, setItems] = useState<ShopifyShop[]>([]);
  const [msg, setMsg] = useState<string | null>(null);

  function handleAuthError(e: unknown): boolean {
    if (e instanceof ApiError && e.kind === "UNAUTHORIZED") {
      logout();
      navigate("/login", { replace: true });
      return true;
    }
    return false;
  }

  async function refreshList() {
    setLoading(true);
    setError(null);
    setMsg(null);

    try {
      const result = await listShopifyShops();
      setItems(result);
    } catch (e) {
      if (handleAuthError(e)) return;
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }

  function normalizeShop(s: string): string {
    return s.trim().toLowerCase();
  }

  async function connectShop() {
    setMsg(null);
    setError(null);

    const shop = normalizeShop(shopInput);
    if (!shop.endsWith(".myshopify.com")) {
      setError("Shop must look like: your-store.myshopify.com");
      return;
    }

    try {
      const authorizeUrl = await getShopifyAuthorizeUrl(shop);
      window.location.assign(authorizeUrl); // now redirect to Shopify
    } catch (e) {
      if (handleAuthError(e)) return;
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  async function disconnect(shop: string) {
    setMsg(null);
    setError(null);
    try {
      await disconnectShopifyShop(shop);
      setMsg(`Disconnected: ${shop}`);
      await refreshList();
    } catch (e) {
      if (handleAuthError(e)) return;
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  async function sync(shop: string) {
    setMsg(null);
    setError(null);
    try {
      const res = await syncShopifyShop(shop);
      setMsg(`Sync started for ${res.shop}. ${res.note ?? ""}`.trim());
    } catch (e) {
      if (handleAuthError(e)) return;
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  useEffect(() => {
    // show callback message if present
    const p = new URLSearchParams(window.location.search);
    if (p.get("connected") === "1") {
      const s = p.get("shop");
      setMsg(s ? `Connected: ${s}` : "Shop connected!");
      // optional: clean URL
      navigate("/shopify", { replace: true });
    }

    refreshList();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate-50 via-purple-50 to-pink-50">
      {/* Header */}
      <header className="border-b bg-white/80 backdrop-blur-sm sticky top-0 z-10 shadow-sm">
        <div className="container mx-auto px-4 py-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <Link to="/">
                <Button variant="ghost" size="icon">
                  <ArrowLeft className="h-5 w-5" />
                </Button>
              </Link>
              <div className="flex items-center gap-3">
                <div className="h-10 w-10 rounded-lg bg-gradient-to-br from-purple-600 to-pink-600 flex items-center justify-center">
                  <Store className="h-6 w-6 text-white" />
                </div>
                <div>
                  <h1 className="text-2xl font-bold bg-gradient-to-r from-purple-600 to-pink-600 bg-clip-text text-transparent">
                    Shopify Integrations
                  </h1>
                  <p className="text-sm text-muted-foreground">Connect and manage your Shopify stores</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </header>

      <main className="container mx-auto px-4 py-8 max-w-6xl">
        {/* Success Message */}
        {msg && (
          <div className="mb-6 p-4 bg-green-50 border-2 border-green-200 rounded-lg flex items-start gap-3">
            <CheckCircle2 className="h-5 w-5 text-green-600 flex-shrink-0 mt-0.5" />
            <div className="text-green-800 flex-1">{msg}</div>
          </div>
        )}

        {/* Error Message */}
        {error && (
          <div className="mb-6 p-4 bg-red-50 border-2 border-red-200 rounded-lg flex items-start gap-3">
            <AlertCircle className="h-5 w-5 text-red-600 flex-shrink-0 mt-0.5" />
            <div className="text-red-800 flex-1">{error}</div>
          </div>
        )}

        {/* Connect New Shop */}
        <Card className="mb-8 border-2 shadow-lg">
          <CardHeader className="bg-gradient-to-r from-purple-50 to-pink-50">
            <CardTitle className="flex items-center gap-2">
              <Plus className="h-5 w-5" />
              Connect a New Shop
            </CardTitle>
            <CardDescription>
              You can connect multiple Shopify stores to this account
            </CardDescription>
          </CardHeader>
          <CardContent className="pt-6">
            <div className="flex flex-col sm:flex-row gap-4">
              <div className="flex-1 space-y-2">
                <Label htmlFor="shop-domain">Shop Domain</Label>
                <Input
                  id="shop-domain"
                  value={shopInput}
                  onChange={(e) => setShopInput(e.target.value)}
                  placeholder="your-store.myshopify.com"
                  className="h-10"
                />
                <p className="text-xs text-muted-foreground">
                  Enter your full Shopify store URL (e.g., mystore.myshopify.com)
                </p>
              </div>
              <div className="flex items-center">
                <Button onClick={connectShop} className="w-full sm:w-auto gap-2 h-10">
                  <LinkIcon className="h-4 w-4" />
                  Connect Shop
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Connected Shops */}
        <Card className="shadow-lg">
          <CardHeader>
            <div className="flex items-center justify-between">
              <div>
                <CardTitle className="text-2xl flex items-center gap-2">
                  <Store className="h-6 w-6" />
                  Connected Shops
                </CardTitle>
                <CardDescription className="mt-1">
                  {items.length === 0 
                    ? "No shops connected yet" 
                    : `${items.length} ${items.length === 1 ? 'shop' : 'shops'} connected`}
                </CardDescription>
              </div>
              <Button 
                variant="outline" 
                onClick={refreshList} 
                disabled={loading}
                className="gap-2"
              >
                <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
                Refresh
              </Button>
            </div>
          </CardHeader>
          <CardContent>
            {loading && (
              <div className="flex items-center justify-center py-12">
                <Loader2 className="h-8 w-8 animate-spin text-purple-600" />
              </div>
            )}

            {!loading && items.length === 0 && (
              <div className="text-center py-12">
                <Store className="h-12 w-12 mx-auto text-muted-foreground mb-3 opacity-50" />
                <p className="text-muted-foreground font-medium">No shops connected yet</p>
                <p className="text-sm text-muted-foreground mt-1">
                  Connect your first Shopify store above to start syncing transactions
                </p>
              </div>
            )}

            {items.length > 0 && (
              <div className="space-y-4">
                {items.map((shop, idx) => (
                  <div key={shop.shop}>
                    <Card className="border-2 hover:border-purple-200 transition-colors">
                      <CardContent className="p-6">
                        <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-4">
                          <div className="flex items-start gap-4 flex-1">
                            <div className="p-3 bg-gradient-to-br from-purple-100 to-pink-100 rounded-lg">
                              <Store className="h-6 w-6 text-purple-700" />
                            </div>
                            <div className="flex-1 min-w-0">
                              <h3 className="text-lg font-bold text-gray-900 break-all">
                                {shop.shop}
                              </h3>
                              <div className="mt-2 space-y-1">
                                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                                  <LinkIcon className="h-3.5 w-3.5" />
                                  <span className="font-mono text-xs">
                                    {shop.scope || "No scopes"}
                                  </span>
                                </div>
                                {shop.createdAt && (
                                  <div className="flex items-center gap-2 text-sm text-muted-foreground">
                                    <Calendar className="h-3.5 w-3.5" />
                                    <span>
                                      Connected {new Date(shop.createdAt).toLocaleString()}
                                    </span>
                                  </div>
                                )}
                              </div>
                            </div>
                          </div>

                          <div className="flex items-center gap-2 lg:flex-shrink-0">
                            <Button 
                              variant="outline" 
                              onClick={() => sync(shop.shop)}
                              className="gap-2 flex-1 lg:flex-none"
                            >
                              <RefreshCw className="h-4 w-4" />
                              Sync
                            </Button>
                            <Button 
                              variant="destructive" 
                              onClick={() => disconnect(shop.shop)}
                              className="gap-2 flex-1 lg:flex-none"
                            >
                              <Unlink className="h-4 w-4" />
                              Disconnect
                            </Button>
                          </div>
                        </div>
                      </CardContent>
                    </Card>
                    {idx < items.length - 1 && <Separator className="my-4" />}
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      </main>
    </div>
  );
}

export default ShopifyView;
