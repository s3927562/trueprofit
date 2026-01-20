# TrueProfit Frontend (React)

React + TypeScript version of the TrueProfit frontend.

## Setup

```bash
npm install
```

### Environment Variables

Create a `.env.local` file in the root directory with the following variables:

```env
# Cognito Configuration
VITE_COGNITO_DOMAIN=https://your-cognito-domain.auth.us-east-1.amazoncognito.com
VITE_COGNITO_CLIENT_ID=your-client-id

# Local Development URLs (using port 3000)
VITE_COGNITO_REDIRECT_URI=http://localhost:3000/callback
VITE_COGNITO_LOGOUT_URI=http://localhost:3000/login

# API Configuration
VITE_API_BASE_URL=https://your-api-gateway-url.execute-api.us-east-1.amazonaws.com/dev
```

**Important:** Make sure to add `http://localhost:3000/callback` and `http://localhost:3000/login` to your Cognito App Client's allowed callback and logout URLs.

## Development

```bash
npm run dev
```

The dev server will run on `http://localhost:3000`

## Build

```bash
npm run build
```

## Preview Production Build

```bash
npm run preview
```

The preview server will also run on `http://localhost:3000`
