import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import {
    BarChart3,
    CheckCircle2,
    DollarSign,
    LogIn,
    LogOut,
    Store,
    TrendingUp
} from "lucide-react";
import { isAuthed, logout, startLogin } from "../services/auth";

function LoginView() {
  const onLogin = () => startLogin();
  const onLogout = () => logout();

  return (
    <div className="min-h-screen bg-gradient-to-br from-blue-50 via-indigo-50 to-purple-50 flex items-center justify-center p-4">
      <div className="w-full max-w-6xl grid lg:grid-cols-2 gap-8 items-center">
        {/* Left side - Branding */}
        <div className="hidden lg:block">
          <div className="space-y-6">
            <div className="flex items-center gap-4">
              <div className="h-16 w-16 rounded-2xl bg-gradient-to-br from-blue-600 to-indigo-600 flex items-center justify-center shadow-xl">
                <DollarSign className="h-10 w-10 text-white" />
              </div>
              <div>
                <h1 className="text-4xl font-bold bg-gradient-to-r from-blue-600 to-indigo-600 bg-clip-text text-transparent">
                  TrueProfit
                </h1>
                <p className="text-lg text-muted-foreground">Track your business finances</p>
              </div>
            </div>

            <div className="space-y-4 pt-8">
              <div className="flex items-start gap-3">
                <div className="p-2 bg-green-100 rounded-lg">
                  <CheckCircle2 className="h-5 w-5 text-green-700" />
                </div>
                <div>
                  <h3 className="font-semibold text-lg">Track Transactions</h3>
                  <p className="text-muted-foreground">
                    Easily record income and expenses in one place
                  </p>
                </div>
              </div>

              <div className="flex items-start gap-3">
                <div className="p-2 bg-blue-100 rounded-lg">
                  <BarChart3 className="h-5 w-5 text-blue-700" />
                </div>
                <div>
                  <h3 className="font-semibold text-lg">Monthly Summaries</h3>
                  <p className="text-muted-foreground">
                    Get detailed insights with visual breakdowns by category
                  </p>
                </div>
              </div>

              <div className="flex items-start gap-3">
                <div className="p-2 bg-purple-100 rounded-lg">
                  <Store className="h-5 w-5 text-purple-700" />
                </div>
                <div>
                  <h3 className="font-semibold text-lg">Shopify Integration</h3>
                  <p className="text-muted-foreground">
                    Connect multiple Shopify stores and sync transactions automatically
                  </p>
                </div>
              </div>

              <div className="flex items-start gap-3">
                <div className="p-2 bg-indigo-100 rounded-lg">
                  <TrendingUp className="h-5 w-5 text-indigo-700" />
                </div>
                <div>
                  <h3 className="font-semibold text-lg">Real-time Analytics</h3>
                  <p className="text-muted-foreground">
                    Monitor your profit and loss with up-to-date financial data
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Right side - Login Card */}
        <div className="flex items-center justify-center">
          <Card className="w-full max-w-md shadow-2xl border-2">
            <CardHeader className="space-y-3 text-center pb-8">
              <div className="mx-auto h-14 w-14 lg:hidden rounded-xl bg-gradient-to-br from-blue-600 to-indigo-600 flex items-center justify-center shadow-lg">
                <DollarSign className="h-8 w-8 text-white" />
              </div>
              <CardTitle className="text-3xl font-bold">
                {isAuthed() ? "Welcome Back!" : "Welcome to TrueProfit"}
              </CardTitle>
              <CardDescription className="text-base">
                {isAuthed() 
                  ? "You are currently signed in to your account" 
                  : "Sign in to manage your business finances"}
              </CardDescription>
            </CardHeader>

            <CardContent className="space-y-6">
              {isAuthed() ? (
                <>
                  <div className="p-4 bg-green-50 border-2 border-green-200 rounded-lg flex items-center gap-3">
                    <CheckCircle2 className="h-5 w-5 text-green-600 flex-shrink-0" />
                    <div className="text-sm text-green-800">
                      You are successfully authenticated
                    </div>
                  </div>

                  <div className="space-y-3">
                    <Button 
                      onClick={() => window.location.href = '/'}
                      className="w-full h-12 text-base gap-2"
                      size="lg"
                    >
                      <DollarSign className="h-5 w-5" />
                      Go to Dashboard
                    </Button>

                    <Button 
                      onClick={onLogout}
                      variant="outline"
                      className="w-full h-12 text-base gap-2"
                      size="lg"
                    >
                      <LogOut className="h-5 w-5" />
                      Logout
                    </Button>
                  </div>
                </>
              ) : (
                <>
                  <div className="p-4 bg-blue-50 border-2 border-blue-200 rounded-lg">
                    <p className="text-sm text-blue-800 text-center">
                      Click the button below to sign in with AWS Cognito
                    </p>
                  </div>

                  <Button 
                    onClick={onLogin}
                    className="w-full h-12 text-base gap-2 shadow-lg hover:shadow-xl transition-shadow"
                    size="lg"
                  >
                    <LogIn className="h-5 w-5" />
                    Login with Cognito
                  </Button>

                  <div className="text-center text-xs text-muted-foreground pt-4">
                    <p>Secure authentication powered by AWS Cognito</p>
                  </div>
                </>
              )}
            </CardContent>
          </Card>
        </div>
      </div>

      {/* Footer */}
      <div className="absolute bottom-4 left-0 right-0 text-center">
        <p className="text-sm text-muted-foreground">
          © {new Date().getFullYear()} TrueProfit. Track, analyze, and grow your business.
        </p>
      </div>
    </div>
  );
}

export default LoginView;
