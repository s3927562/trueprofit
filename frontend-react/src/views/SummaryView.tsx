import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  ArrowLeft,
  BarChart3,
  Calendar,
  DollarSign,
  Hash,
  Loader2,
  TrendingDown,
  TrendingUp
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { ApiError, getMonthlySummary, type MonthlySummary } from "../services/api";
import { logout } from "../services/auth";

function SummaryView() {
  const navigate = useNavigate();

  const [month, setMonth] = useState<string>(new Date().toISOString().slice(0, 7)); // YYYY-MM
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [data, setData] = useState<MonthlySummary | null>(null);

  const categories = useMemo(() => {
    if (!data) return [];
    return Object.entries(data.byCategory).sort((a, b) => Math.abs(b[1]) - Math.abs(a[1]));
  }, [data]);

  const maxCategoryValue = useMemo(() => {
    if (categories.length === 0) return 1;
    return Math.max(...categories.map(([, val]) => Math.abs(val)));
  }, [categories]);

  async function load() {
    setLoading(true);
    setError(null);
    try {
      const result = await getMonthlySummary(month);
      setData(result);
    } catch (e) {
      if (e instanceof ApiError && e.kind === "UNAUTHORIZED") {
        logout();
        navigate("/login", { replace: true });
        return;
      }
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate-50 via-blue-50 to-indigo-50">
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
              <div>
                <h1 className="text-2xl font-bold bg-gradient-to-r from-blue-600 to-indigo-600 bg-clip-text text-transparent">
                  Monthly Summary
                </h1>
                <p className="text-sm text-muted-foreground">Financial overview for the selected month</p>
              </div>
            </div>
          </div>
        </div>
      </header>

      <main className="container mx-auto px-4 py-8 max-w-6xl">
        {/* Month Selector */}
        <Card className="mb-8 border-2 shadow-lg">
          <CardHeader className="bg-gradient-to-r from-blue-50 to-indigo-50">
            <CardTitle className="flex items-center gap-2">
              <Calendar className="h-5 w-5" />
              Select Month
            </CardTitle>
          </CardHeader>
          <CardContent className="pt-6">
            <div className="flex items-end gap-4">
              <div className="flex-1 max-w-xs space-y-2">
                <Label htmlFor="month">Month</Label>
                <Input
                  id="month"
                  value={month}
                  onChange={(e) => setMonth(e.target.value)}
                  type="month"
                  className="h-10"
                />
              </div>
              <Button onClick={load} disabled={loading} className="gap-2">
                {loading ? (
                  <>
                    <Loader2 className="h-4 w-4 animate-spin" />
                    Loading...
                  </>
                ) : (
                  <>
                    <BarChart3 className="h-4 w-4" />
                    Load Summary
                  </>
                )}
              </Button>
            </div>
          </CardContent>
        </Card>

        {error && (
          <div className="mb-8 p-4 bg-red-50 border border-red-200 rounded-lg text-red-800">
            {error}
          </div>
        )}

        {data && (
          <>
            {/* Summary Stats */}
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
              {/* Income Card */}
              <Card className="border-2 shadow-lg hover:shadow-xl transition-shadow">
                <CardHeader className="pb-3">
                  <CardDescription className="flex items-center gap-2">
                    <TrendingUp className="h-4 w-4 text-green-600" />
                    Income
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <div className="text-3xl font-bold text-green-700">
                    {data.income.toFixed(2)}
                  </div>
                  <p className="text-xs text-muted-foreground mt-1">{data.currency}</p>
                </CardContent>
              </Card>

              {/* Expense Card */}
              <Card className="border-2 shadow-lg hover:shadow-xl transition-shadow">
                <CardHeader className="pb-3">
                  <CardDescription className="flex items-center gap-2">
                    <TrendingDown className="h-4 w-4 text-red-600" />
                    Expense
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <div className="text-3xl font-bold text-red-700">
                    {data.expense.toFixed(2)}
                  </div>
                  <p className="text-xs text-muted-foreground mt-1">{data.currency}</p>
                </CardContent>
              </Card>

              {/* Net Card */}
              <Card className="border-2 shadow-lg hover:shadow-xl transition-shadow bg-gradient-to-br from-blue-50 to-indigo-50">
                <CardHeader className="pb-3">
                  <CardDescription className="flex items-center gap-2">
                    <DollarSign className="h-4 w-4 text-blue-600" />
                    Net Profit
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <div className={`text-3xl font-bold ${data.net >= 0 ? 'text-blue-700' : 'text-red-700'}`}>
                    {data.net >= 0 ? '+' : ''}{data.net.toFixed(2)}
                  </div>
                  <p className="text-xs text-muted-foreground mt-1">{data.currency}</p>
                </CardContent>
              </Card>

              {/* Count Card */}
              <Card className="border-2 shadow-lg hover:shadow-xl transition-shadow">
                <CardHeader className="pb-3">
                  <CardDescription className="flex items-center gap-2">
                    <Hash className="h-4 w-4 text-purple-600" />
                    Transactions
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <div className="text-3xl font-bold text-purple-700">
                    {data.count}
                  </div>
                  <p className="text-xs text-muted-foreground mt-1">Total count</p>
                </CardContent>
              </Card>
            </div>

            {/* Categories Breakdown */}
            <Card className="shadow-lg">
              <CardHeader>
                <CardTitle className="text-2xl flex items-center gap-2">
                  <BarChart3 className="h-6 w-6" />
                  By Category
                </CardTitle>
                <CardDescription>
                  Net contribution per category (positive = income, negative = expense)
                </CardDescription>
              </CardHeader>
              <CardContent>
                {categories.length === 0 ? (
                  <div className="text-center py-12">
                    <BarChart3 className="h-12 w-12 mx-auto text-muted-foreground mb-3 opacity-50" />
                    <p className="text-muted-foreground">No categories found for this month.</p>
                  </div>
                ) : (
                  <div className="space-y-6">
                    {categories.map(([cat, val]) => {
                      const percentage = (Math.abs(val) / maxCategoryValue) * 100;
                      const isPositive = val >= 0;

                      return (
                        <div key={cat} className="space-y-2">
                          <div className="flex items-center justify-between">
                            <div className="flex items-center gap-2">
                              <div className={`p-1.5 rounded-md ${isPositive ? 'bg-green-100' : 'bg-red-100'}`}>
                                {isPositive ? (
                                  <TrendingUp className="h-4 w-4 text-green-700" />
                                ) : (
                                  <TrendingDown className="h-4 w-4 text-red-700" />
                                )}
                              </div>
                              <span className="font-semibold">{cat}</span>
                            </div>
                            <span className={`font-bold ${isPositive ? 'text-green-700' : 'text-red-700'}`}>
                              {isPositive ? '+' : ''}{val.toFixed(2)} {data.currency}
                            </span>
                          </div>

                          {/* Progress Bar */}
                          <div className="relative h-3 bg-gray-100 rounded-full overflow-hidden">
                            <div
                              className={`h-full rounded-full transition-all duration-500 ${
                                isPositive 
                                  ? 'bg-gradient-to-r from-green-400 to-green-600' 
                                  : 'bg-gradient-to-r from-red-400 to-red-600'
                              }`}
                              style={{ width: `${percentage}%` }}
                            />
                          </div>

                          {/* Percentage indicator */}
                          <div className="flex justify-end">
                            <span className="text-xs text-muted-foreground">
                              {percentage.toFixed(1)}% of max category
                            </span>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                )}
              </CardContent>
            </Card>

            {/* Visual Summary */}
            {categories.length > 0 && (
              <Card className="mt-8 shadow-lg">
                <CardHeader>
                  <CardTitle className="text-2xl">Category Breakdown</CardTitle>
                  <CardDescription>
                    Visual comparison of all categories
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    {categories.map(([cat, val]) => (
                      <div
                        key={cat}
                        className={`p-4 rounded-lg border-2 ${
                          val >= 0 
                            ? 'bg-green-50 border-green-200' 
                            : 'bg-red-50 border-red-200'
                        }`}
                      >
                        <div className="flex items-center justify-between">
                          <span className="font-medium text-sm">{cat}</span>
                          <span className={`text-lg font-bold ${val >= 0 ? 'text-green-700' : 'text-red-700'}`}>
                            {val >= 0 ? '+' : ''}{val.toFixed(2)}
                          </span>
                        </div>
                      </div>
                    ))}
                  </div>
                </CardContent>
              </Card>
            )}
          </>
        )}

        {!data && !loading && (
          <Card className="shadow-lg">
            <CardContent className="py-12">
              <div className="text-center">
                <Calendar className="h-12 w-12 mx-auto text-muted-foreground mb-3 opacity-50" />
                <p className="text-muted-foreground">Select a month and click "Load Summary" to view data.</p>
              </div>
            </CardContent>
          </Card>
        )}
      </main>
    </div>
  );
}

export default SummaryView;
