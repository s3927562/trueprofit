import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import {
  AlertCircle,
  ArrowLeft,
  CheckCircle2,
  Database,
  HelpCircle,
  Loader2,
  MessageSquare,
  Search,
  Sparkles,
  XCircle
} from "lucide-react";
import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { ApiError, askQuestion, type AskResponse } from "../services/api";
import { logout, refreshIfNeeded } from "../services/auth";

function AskView() {
  const navigate = useNavigate();

  const [question, setQuestion] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [response, setResponse] = useState<AskResponse | null>(null);

  function handleAuthError(e: unknown): boolean {
    if (e instanceof ApiError && e.kind === "UNAUTHORIZED") {
      logout();
      navigate("/login", { replace: true });
      return true;
    }
    return false;
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!question.trim()) return;

    setLoading(true);
    setError(null);
    setResponse(null);

    try {
      await refreshIfNeeded();
      const res = await askQuestion({ question: question.trim() });
      setResponse(res);
    } catch (e) {
      if (handleAuthError(e)) return;
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }

  function handleReset() {
    setQuestion("");
    setResponse(null);
    setError(null);
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-purple-50 via-pink-50 to-orange-50">
      {/* Header */}
      <header className="border-b bg-white/80 backdrop-blur-sm sticky top-0 z-10 shadow-sm">
        <div className="container mx-auto px-4 py-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <Link to="/">
                <Button variant="ghost" size="sm" className="gap-2">
                  <ArrowLeft className="h-4 w-4" />
                  Back
                </Button>
              </Link>
              <div className="h-10 w-10 rounded-lg bg-gradient-to-br from-purple-600 to-pink-600 flex items-center justify-center">
                <Sparkles className="h-6 w-6 text-white" />
              </div>
              <div>
                <h1 className="text-2xl font-bold bg-gradient-to-r from-purple-600 to-pink-600 bg-clip-text text-transparent">
                  Ask Your Data
                </h1>
                <p className="text-sm text-muted-foreground">Natural language queries powered by AI</p>
              </div>
            </div>
          </div>
        </div>
      </header>

      <main className="container mx-auto px-4 py-8 max-w-5xl">
        {/* Question Input Card */}
        <Card className="mb-6 border-2 shadow-lg">
          <CardHeader className="bg-gradient-to-r from-purple-50 to-pink-50">
            <CardTitle className="flex items-center gap-2">
              <MessageSquare className="h-5 w-5" />
              Ask a Question
            </CardTitle>
            <CardDescription>
              Ask questions about your data in plain English. The AI will generate SQL and run it for you.
            </CardDescription>
          </CardHeader>
          <CardContent className="pt-6">
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="question" className="text-sm font-semibold">
                  Your Question
                </Label>
                <Input
                  id="question"
                  value={question}
                  onChange={(e) => setQuestion(e.target.value)}
                  placeholder="e.g., What are my top selling products last month?"
                  className="h-12 text-base"
                  disabled={loading}
                  autoFocus
                />
                <p className="text-xs text-muted-foreground">
                  Examples: "Show sales by country", "What's my revenue this week?", "Top 10 customers by order value"
                </p>
              </div>

              <div className="flex gap-3">
                <Button 
                  type="submit" 
                  disabled={loading || !question.trim()} 
                  className="flex-1 gap-2 h-11"
                >
                  {loading ? (
                    <>
                      <Loader2 className="h-4 w-4 animate-spin" />
                      Processing...
                    </>
                  ) : (
                    <>
                      <Search className="h-4 w-4" />
                      Ask Question
                    </>
                  )}
                </Button>
                {(response || error) && (
                  <Button 
                    type="button" 
                    variant="outline" 
                    onClick={handleReset}
                    className="gap-2"
                  >
                    Clear
                  </Button>
                )}
              </div>
            </form>

            {error && (
              <div className="mt-4 p-4 bg-red-50 border border-red-200 rounded-lg flex items-start gap-3">
                <XCircle className="h-5 w-5 text-red-600 flex-shrink-0 mt-0.5" />
                <div>
                  <p className="font-semibold text-red-900">Error</p>
                  <p className="text-sm text-red-800 mt-1">{error}</p>
                </div>
              </div>
            )}
          </CardContent>
        </Card>

        {/* Response Display */}
        {response && (
          <>
            {/* Result Type: Success */}
            {response.type === "result" && (
              <Card className="shadow-lg border-2 border-green-200">
                <CardHeader className="bg-gradient-to-r from-green-50 to-emerald-50">
                  <div className="flex items-start justify-between">
                    <div className="flex items-start gap-3">
                      <CheckCircle2 className="h-6 w-6 text-green-600 flex-shrink-0 mt-1" />
                      <div>
                        <CardTitle className="text-green-900">Query Successful</CardTitle>
                        <CardDescription className="mt-1">
                          {response.cached && (
                            <span className="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-blue-100 text-blue-800 mr-2">
                              Cached Result
                            </span>
                          )}
                          <span className="text-xs text-muted-foreground">
                            Scanned {(response.scanned_bytes / 1024 / 1024).toFixed(2)} MB • 
                            Executed in {response.exec_ms}ms • 
                            Confidence: {(response.confidence * 100).toFixed(0)}%
                          </span>
                        </CardDescription>
                      </div>
                    </div>
                  </div>
                </CardHeader>
                <CardContent className="pt-6 space-y-6">
                  {/* Assumptions */}
                  {response.assumptions && response.assumptions.length > 0 && (
                    <div>
                      <h3 className="font-semibold text-sm mb-2 flex items-center gap-2">
                        <HelpCircle className="h-4 w-4" />
                        Assumptions Made
                      </h3>
                      <ul className="space-y-1">
                        {response.assumptions.map((assumption, idx) => (
                          <li key={idx} className="text-sm text-muted-foreground pl-6 relative before:content-['•'] before:absolute before:left-2">
                            {assumption}
                          </li>
                        ))}
                      </ul>
                    </div>
                  )}

                  <Separator />

                  {/* SQL Query */}
                  <div>
                    <h3 className="font-semibold text-sm mb-2 flex items-center gap-2">
                      <Database className="h-4 w-4" />
                      Generated SQL
                    </h3>
                    <pre className="bg-slate-900 text-slate-100 p-4 rounded-lg text-xs overflow-x-auto">
                      <code>{response.sql}</code>
                    </pre>
                  </div>

                  <Separator />

                  {/* Results Table */}
                  <div>
                    <h3 className="font-semibold text-sm mb-3">Results ({response.result.rows.length} rows)</h3>
                    {response.result.rows.length === 0 ? (
                      <p className="text-sm text-muted-foreground italic">No data returned</p>
                    ) : response.result.kind === "scalar" ? (
                      <div className="bg-slate-50 border rounded-lg p-6 text-center">
                        <p className="text-xs text-muted-foreground mb-2">Scalar Result:</p>
                        <p className="text-2xl font-bold text-slate-900">
                          {response.result.value === null || response.result.value === undefined ? (
                            <span className="text-slate-400 italic">null</span>
                          ) : (
                            String(response.result.value)
                          )}
                        </p>
                      </div>
                    ) : (
                      <div className="overflow-x-auto border rounded-lg">
                        <table className="w-full text-sm">
                          <thead className="bg-slate-50 border-b">
                            <tr>
                              {response.result.columns.map((col, idx) => (
                                <th key={idx} className="px-4 py-3 text-left font-semibold text-slate-700">
                                  {col}
                                </th>
                              ))}
                            </tr>
                          </thead>
                          <tbody>
                            {response.result.rows.map((row, rowIdx) => (
                              <tr key={rowIdx} className="border-b last:border-0 hover:bg-slate-50">
                                {response.result.columns.map((col, cellIdx) => (
                                  <td key={cellIdx} className="px-4 py-3 text-slate-700">
                                    {row[col] === null || row[col] === undefined ? (
                                      <span className="text-slate-400 italic">null</span>
                                    ) : (
                                      String(row[col])
                                    )}
                                  </td>
                                ))}
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    )}
                  </div>

                  <div className="text-xs text-muted-foreground">
                    Query ID: <code className="bg-slate-100 px-1 py-0.5 rounded">{response.query_id}</code>
                  </div>
                </CardContent>
              </Card>
            )}

            {/* Result Type: Clarification */}
            {response.type === "clarification" && (
              <Card className="shadow-lg border-2 border-yellow-200">
                <CardHeader className="bg-gradient-to-r from-yellow-50 to-amber-50">
                  <div className="flex items-start gap-3">
                    <HelpCircle className="h-6 w-6 text-yellow-600 flex-shrink-0 mt-1" />
                    <div>
                      <CardTitle className="text-yellow-900">Need More Information</CardTitle>
                      <CardDescription className="mt-1">
                        The AI needs clarification to answer your question accurately.
                      </CardDescription>
                    </div>
                  </div>
                </CardHeader>
                <CardContent className="pt-6 space-y-4">
                  <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-4">
                    <p className="font-semibold text-yellow-900 mb-2">Clarifying Question:</p>
                    <p className="text-sm text-yellow-800">{response.clarifying_question}</p>
                  </div>

                  {response.assumptions && response.assumptions.length > 0 && (
                    <div>
                      <h3 className="font-semibold text-sm mb-2">Assumptions:</h3>
                      <ul className="space-y-1">
                        {response.assumptions.map((assumption, idx) => (
                          <li key={idx} className="text-sm text-muted-foreground pl-6 relative before:content-['•'] before:absolute before:left-2">
                            {assumption}
                          </li>
                        ))}
                      </ul>
                    </div>
                  )}

                  <p className="text-xs text-muted-foreground">
                    Confidence: {(response.confidence * 100).toFixed(0)}%
                  </p>
                </CardContent>
              </Card>
            )}

            {/* Result Type: SQL Rejected */}
            {response.type === "sql_rejected" && (
              <Card className="shadow-lg border-2 border-red-200">
                <CardHeader className="bg-gradient-to-r from-red-50 to-rose-50">
                  <div className="flex items-start gap-3">
                    <XCircle className="h-6 w-6 text-red-600 flex-shrink-0 mt-1" />
                    <div>
                      <CardTitle className="text-red-900">SQL Validation Failed</CardTitle>
                      <CardDescription className="mt-1">
                        The generated SQL did not pass security validation.
                      </CardDescription>
                    </div>
                  </div>
                </CardHeader>
                <CardContent className="pt-6 space-y-4">
                  <div className="bg-red-50 border border-red-200 rounded-lg p-4">
                    <p className="font-semibold text-red-900 mb-2">Reason:</p>
                    <p className="text-sm text-red-800">{response.reason}</p>
                  </div>

                  {response.model_sql && (
                    <div>
                      <h3 className="font-semibold text-sm mb-2">Generated SQL (Rejected):</h3>
                      <pre className="bg-slate-900 text-slate-100 p-4 rounded-lg text-xs overflow-x-auto">
                        <code>{response.model_sql}</code>
                      </pre>
                    </div>
                  )}

                  {response.assumptions && response.assumptions.length > 0 && (
                    <div>
                      <h3 className="font-semibold text-sm mb-2">Assumptions:</h3>
                      <ul className="space-y-1">
                        {response.assumptions.map((assumption, idx) => (
                          <li key={idx} className="text-sm text-muted-foreground pl-6 relative before:content-['•'] before:absolute before:left-2">
                            {assumption}
                          </li>
                        ))}
                      </ul>
                    </div>
                  )}
                </CardContent>
              </Card>
            )}

            {/* Result Type: Athena Failed */}
            {response.type === "athena_failed" && (
              <Card className="shadow-lg border-2 border-orange-200">
                <CardHeader className="bg-gradient-to-r from-orange-50 to-amber-50">
                  <div className="flex items-start gap-3">
                    <AlertCircle className="h-6 w-6 text-orange-600 flex-shrink-0 mt-1" />
                    <div>
                      <CardTitle className="text-orange-900">Query Execution Failed</CardTitle>
                      <CardDescription className="mt-1">
                        The query could not be executed successfully.
                      </CardDescription>
                    </div>
                  </div>
                </CardHeader>
                <CardContent className="pt-6 space-y-4">
                  <div className="bg-orange-50 border border-orange-200 rounded-lg p-4">
                    <p className="font-semibold text-orange-900 mb-2">Error:</p>
                    <p className="text-sm text-orange-800">{response.error}</p>
                  </div>

                  {response.last_sql && (
                    <div>
                      <h3 className="font-semibold text-sm mb-2">Last Attempted SQL:</h3>
                      <pre className="bg-slate-900 text-slate-100 p-4 rounded-lg text-xs overflow-x-auto">
                        <code>{response.last_sql}</code>
                      </pre>
                    </div>
                  )}

                  {response.assumptions && response.assumptions.length > 0 && (
                    <div>
                      <h3 className="font-semibold text-sm mb-2">Assumptions:</h3>
                      <ul className="space-y-1">
                        {response.assumptions.map((assumption, idx) => (
                          <li key={idx} className="text-sm text-muted-foreground pl-6 relative before:content-['•'] before:absolute before:left-2">
                            {assumption}
                          </li>
                        ))}
                      </ul>
                    </div>
                  )}
                </CardContent>
              </Card>
            )}

            {/* Result Type: No Shops */}
            {response.type === "no_shops" && (
              <Card className="shadow-lg border-2 border-gray-200">
                <CardHeader className="bg-gradient-to-r from-gray-50 to-slate-50">
                  <div className="flex items-start gap-3">
                    <AlertCircle className="h-6 w-6 text-gray-600 flex-shrink-0 mt-1" />
                    <div>
                      <CardTitle className="text-gray-900">No Shops Connected</CardTitle>
                      <CardDescription className="mt-1">
                        You need to connect at least one shop to query your data.
                      </CardDescription>
                    </div>
                  </div>
                </CardHeader>
                <CardContent className="pt-6">
                  <p className="text-sm text-muted-foreground mb-4">{response.error}</p>
                  <Link to="/shopify">
                    <Button className="gap-2">
                      <Database className="h-4 w-4" />
                      Connect a Shop
                    </Button>
                  </Link>
                </CardContent>
              </Card>
            )}
          </>
        )}
      </main>
    </div>
  );
}

export default AskView;
