import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { AlertCircle, CheckCircle2, DollarSign, Loader2 } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { handleCallback } from "../services/auth";

function CallbackView() {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const hasRun = useRef(false);

  useEffect(() => {
    // Prevent double execution in React Strict Mode
    if (hasRun.current) return;
    hasRun.current = true;

    const performCallback = async () => {
      try {
        await handleCallback(window.location.search);
        navigate("/", { replace: true });
      } catch (e) {
        console.error(e);
        setError(e instanceof Error ? e.message : String(e));
      } finally {
        setLoading(false);
      }
    };

    performCallback();
  }, [navigate]);

  return (
    <div className="min-h-screen bg-gradient-to-br from-blue-50 via-indigo-50 to-purple-50 flex items-center justify-center p-4">
      <Card className="w-full max-w-md shadow-2xl border-2">
        <CardHeader className="space-y-3 text-center">
          <div className="mx-auto h-14 w-14 rounded-xl bg-gradient-to-br from-blue-600 to-indigo-600 flex items-center justify-center shadow-lg">
            <DollarSign className="h-8 w-8 text-white" />
          </div>
          <CardTitle className="text-2xl font-bold">
            {loading ? "Signing you in..." : error ? "Authentication Error" : "Success!"}
          </CardTitle>
          <CardDescription>
            {loading 
              ? "Please wait while we complete your authentication" 
              : error 
              ? "There was a problem signing you in" 
              : "Redirecting to your dashboard"}
          </CardDescription>
        </CardHeader>

        <CardContent className="flex flex-col items-center gap-4">
          {loading && (
            <div className="p-8">
              <Loader2 className="h-12 w-12 animate-spin text-blue-600" />
            </div>
          )}

          {error && (
            <div className="w-full p-4 bg-red-50 border-2 border-red-200 rounded-lg flex items-start gap-3">
              <AlertCircle className="h-5 w-5 text-red-600 shrink-0 mt-0.5" />
              <div className="text-sm text-red-800">{error}</div>
            </div>
          )}

          {!loading && !error && (
            <div className="p-8">
              <CheckCircle2 className="h-12 w-12 text-green-600" />
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

export default CallbackView;
