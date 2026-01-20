import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import {
  Calendar,
  ChevronRight,
  DollarSign,
  LogOut,
  Plus,
  RefreshCw,
  Store,
  TrendingDown,
  TrendingUp
} from "lucide-react";
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { ApiError, createTransaction, listTransactions, type Transaction } from "../services/api";
import { logout, refreshIfNeeded } from "../services/auth";

function HomeView() {
  const navigate = useNavigate();

  /** Transactions */
  const [txLoading, setTxLoading] = useState(false);
  const [txLoadingMore, setTxLoadingMore] = useState(false);
  const [txError, setTxError] = useState<string | null>(null);
  const [txItems, setTxItems] = useState<Transaction[]>([]);
  const [txNextToken, setTxNextToken] = useState<string>(""); // empty => no more pages

  /** Create form */
  const [formAmount, setFormAmount] = useState<number>(0);
  const [formCurrency, setFormCurrency] = useState<string>("USD");
  const [formCategory, setFormCategory] = useState<string>("Sales");
  const [formNote, setFormNote] = useState<string>("");

  function handleAuthError(e: unknown): boolean {
    if (e instanceof ApiError && e.kind === "UNAUTHORIZED") {
      logout();
      navigate("/login", { replace: true });
      return true;
    }
    return false;
  }

  async function loadTxFirstPage() {
    setTxLoading(true);
    setTxError(null);
    try {
      await refreshIfNeeded();

      const res = await listTransactions({ limit: 20 });
      setTxItems(res.items ?? []);
      setTxNextToken(res.nextToken ?? "");
    } catch (e) {
      if (handleAuthError(e)) return;
      setTxError(e instanceof Error ? e.message : String(e));
    } finally {
      setTxLoading(false);
    }
  }

  async function loadTxMore() {
    if (!txNextToken) return;

    setTxLoadingMore(true);
    setTxError(null);
    try {
      await refreshIfNeeded();

      const res = await listTransactions({ limit: 20, nextToken: txNextToken });
      setTxItems([...txItems, ...(res.items ?? [])]);
      setTxNextToken(res.nextToken ?? "");
    } catch (e) {
      if (handleAuthError(e)) return;
      setTxError(e instanceof Error ? e.message : String(e));
    } finally {
      setTxLoadingMore(false);
    }
  }

  async function submitTx(e: React.FormEvent) {
    e.preventDefault();
    setTxError(null);
    try {
      await refreshIfNeeded();

      const created = await createTransaction({
        amount: Number(formAmount),
        currency: formCurrency,
        category: formCategory,
        note: formNote,
      });

      // Put newest on top; keep pagination token unchanged
      setTxItems([created, ...txItems]);
      setFormAmount(0);
      setFormNote("");
    } catch (e) {
      if (handleAuthError(e)) return;
      setTxError(e instanceof Error ? e.message : String(e));
    }
  }

  function doLogout() {
    logout();
  }

  useEffect(() => {
    loadTxFirstPage();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate-50 via-blue-50 to-indigo-50">
      {/* Header */}
      <header className="border-b bg-white/80 backdrop-blur-sm sticky top-0 z-10 shadow-sm">
        <div className="container mx-auto px-4 py-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="h-10 w-10 rounded-lg bg-gradient-to-br from-blue-600 to-indigo-600 flex items-center justify-center">
                <DollarSign className="h-6 w-6 text-white" />
              </div>
              <div>
                <h1 className="text-2xl font-bold bg-gradient-to-r from-blue-600 to-indigo-600 bg-clip-text text-transparent">
                  TrueProfit
                </h1>
                <p className="text-sm text-muted-foreground">Track your business finances</p>
              </div>
            </div>

            <div className="flex items-center gap-3">
              <Link to="/summary">
                <Button variant="outline" className="gap-2">
                  <Calendar className="h-4 w-4" />
                  Monthly Summary
                </Button>
              </Link>
              <Link to="/shopify">
                <Button variant="outline" className="gap-2">
                  <Store className="h-4 w-4" />
                  Shopify Shops
                </Button>
              </Link>
              <Button variant="destructive" onClick={doLogout} className="gap-2">
                <LogOut className="h-4 w-4" />
                Logout
              </Button>
            </div>
          </div>
        </div>
      </header>

      <main className="container mx-auto px-4 py-8 max-w-7xl">
        {/* Add Transaction Card */}
        <Card className="mb-8 border-2 shadow-lg">
          <CardHeader className="bg-gradient-to-r from-blue-50 to-indigo-50">
            <CardTitle className="flex items-center gap-2">
              <Plus className="h-5 w-5" />
              Add New Transaction
            </CardTitle>
            <CardDescription>
              Record income or expenses quickly
            </CardDescription>
          </CardHeader>
          <CardContent className="pt-6">
            <form onSubmit={submitTx} className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-4">
              <div className="space-y-2">
                <Label htmlFor="amount" className="text-xs font-semibold uppercase tracking-wide">
                  Amount
                </Label>
                <Input
                  id="amount"
                  value={formAmount}
                  onChange={(e) => setFormAmount(Number(e.target.value))}
                  type="number"
                  step="0.01"
                  placeholder="0.00"
                  required
                  className="h-10"
                />
                <p className="text-xs text-muted-foreground">+ income, - expense</p>
              </div>

              <div className="space-y-2">
                <Label htmlFor="currency" className="text-xs font-semibold uppercase tracking-wide">
                  Currency
                </Label>
                <Input
                  id="currency"
                  value={formCurrency}
                  onChange={(e) => setFormCurrency(e.target.value.toUpperCase())}
                  type="text"
                  maxLength={3}
                  placeholder="USD"
                  required
                  className="h-10"
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="category" className="text-xs font-semibold uppercase tracking-wide">
                  Category
                </Label>
                <Input
                  id="category"
                  value={formCategory}
                  onChange={(e) => setFormCategory(e.target.value)}
                  type="text"
                  placeholder="Sales"
                  required
                  className="h-10"
                />
              </div>

              <div className="space-y-2 lg:col-span-1">
                <Label htmlFor="note" className="text-xs font-semibold uppercase tracking-wide">
                  Note (optional)
                </Label>
                <Input
                  id="note"
                  value={formNote}
                  onChange={(e) => setFormNote(e.target.value)}
                  type="text"
                  placeholder="Details..."
                  className="h-10"
                />
              </div>

              <div className="flex items-center">
                <Button type="submit" className="w-full h-10 gap-2">
                  <Plus className="h-4 w-4" />
                  Add Transaction
                </Button>
              </div>
            </form>

            {txError && (
              <div className="mt-4 p-3 bg-red-50 border border-red-200 rounded-lg text-red-800 text-sm">
                {txError}
              </div>
            )}
          </CardContent>
        </Card>

        {/* Transactions List */}
        <Card className="shadow-lg">
          <CardHeader>
            <div className="flex items-center justify-between">
              <div>
                <CardTitle className="text-2xl flex items-center gap-2">
                  <DollarSign className="h-6 w-6" />
                  Transactions
                </CardTitle>
                <CardDescription className="mt-1">
                  View and manage your transaction history
                </CardDescription>
              </div>
              <Button 
                variant="outline" 
                onClick={loadTxFirstPage} 
                disabled={txLoading}
                className="gap-2"
              >
                <RefreshCw className={`h-4 w-4 ${txLoading ? 'animate-spin' : ''}`} />
                {txLoading ? "Refreshing..." : "Refresh"}
              </Button>
            </div>
          </CardHeader>
          <CardContent>
            {txLoading && (
              <div className="flex items-center justify-center py-12">
                <RefreshCw className="h-8 w-8 animate-spin text-blue-600" />
              </div>
            )}

            {!txLoading && txItems.length === 0 && (
              <div className="text-center py-12">
                <DollarSign className="h-12 w-12 mx-auto text-muted-foreground mb-3 opacity-50" />
                <p className="text-muted-foreground">No transactions yet.</p>
                <p className="text-sm text-muted-foreground mt-1">Add your first transaction above to get started.</p>
              </div>
            )}

            {txItems.length > 0 && (
              <div className="space-y-3">
                {txItems.map((t, idx) => (
                  <div key={t.id}>
                    <div className="flex items-start justify-between p-4 rounded-lg hover:bg-accent/50 transition-colors">
                      <div className="flex items-start gap-3 flex-1">
                        <div className={`mt-1 p-2 rounded-lg ${t.amount >= 0 ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
                          {t.amount >= 0 ? (
                            <TrendingUp className="h-4 w-4" />
                          ) : (
                            <TrendingDown className="h-4 w-4" />
                          )}
                        </div>
                        <div className="flex-1 min-w-0">
                          <div className="flex items-baseline gap-2 flex-wrap">
                            <span className={`text-lg font-bold ${t.amount >= 0 ? 'text-green-700' : 'text-red-700'}`}>
                              {t.amount >= 0 ? '+' : ''}{t.amount.toFixed(2)} {t.currency}
                            </span>
                            <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800">
                              {t.category}
                            </span>
                          </div>
                          {t.note && (
                            <p className="text-sm text-muted-foreground mt-1">{t.note}</p>
                          )}
                          <p className="text-xs text-muted-foreground mt-1 flex items-center gap-1">
                            <Calendar className="h-3 w-3" />
                            {new Date(t.createdAt).toLocaleString()}
                          </p>
                        </div>
                      </div>
                      <ChevronRight className="h-5 w-5 text-muted-foreground flex-shrink-0 ml-2" />
                    </div>
                    {idx < txItems.length - 1 && <Separator />}
                  </div>
                ))}

                <div className="flex items-center justify-center pt-4">
                  {txNextToken ? (
                    <Button 
                      variant="outline" 
                      onClick={loadTxMore} 
                      disabled={txLoadingMore}
                      className="gap-2"
                    >
                      {txLoadingMore ? (
                        <>
                          <RefreshCw className="h-4 w-4 animate-spin" />
                          Loading...
                        </>
                      ) : (
                        <>
                          Load more
                          <ChevronRight className="h-4 w-4" />
                        </>
                      )}
                    </Button>
                  ) : (
                    <p className="text-sm text-muted-foreground">
                      No more transactions to load
                    </p>
                  )}
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      </main>
    </div>
  );
}

export default HomeView;
