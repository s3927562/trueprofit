# TrueProfit User Guide

Welcome to **TrueProfit** - your all-in-one business finance tracking and analytics platform. This guide will help you get started and make the most of all available features.

---

## Table of Contents

1. [Getting Started](#getting-started)
2. [Dashboard & Transaction Management](#dashboard--transaction-management)
3. [Monthly Summary & Reports](#monthly-summary--reports)
4. [Shopify Integration](#shopify-integration)
5. [Ask Your Data (AI-Powered Queries)](#ask-your-data-ai-powered-queries)
6. [Troubleshooting](#troubleshooting)

---

## Getting Started

### Logging In

1. **Navigate to the login page** at `http://localhost:3000/login` (or your production URL)
2. Click the **"Login with Cognito"** button
3. You'll be redirected to AWS Cognito for secure authentication
4. Enter your credentials (username/email and password)
5. After successful authentication, you'll be redirected to the dashboard

### First Time Setup

- If you're a new user, an administrator will need to create your account in AWS Cognito
- Make sure you have your login credentials ready
- After first login, you may be prompted to change your password

---

## Dashboard & Transaction Management

The dashboard is your home base for managing financial transactions.

### Viewing Transactions

- **Automatic Loading**: When you first land on the dashboard, the most recent 20 transactions are loaded automatically
- **Transaction Details**: Each transaction displays:
  - Amount (color-coded: green for income, red for expenses)
  - Currency (e.g., USD, EUR)
  - Category (shown as a badge)
  - Optional note/description
  - Creation timestamp

### Adding a New Transaction

1. **Locate the "Add New Transaction" card** at the top of the dashboard
2. Fill in the form fields:
   - **Amount**: Enter the transaction amount
     - Use positive numbers for **income** (e.g., `1000`)
     - Use negative numbers for **expenses** (e.g., `-250`)
   - **Currency**: Enter the 3-letter currency code (e.g., `USD`, `EUR`, `GBP`)
   - **Category**: Enter a category name (e.g., `Sales`, `Marketing`, `Supplies`)
   - **Note** (optional): Add any additional details or description
3. Click **"Add Transaction"**
4. The new transaction will appear at the top of your transaction list

### Managing Transactions

- **Refresh List**: Click the "Refresh" button in the Transactions card to reload the latest data
- **Load More**: If you have more than 20 transactions, click "Load more" at the bottom to see additional items
- **Visual Indicators**:
  - 🟢 Green icon with upward arrow = Income
  - 🔴 Red icon with downward arrow = Expense

### Navigation

From the dashboard header, you can quickly access:
- **Ask Your Data** - AI-powered analytics (purple button)
- **Monthly Summary** - View financial reports
- **Shopify Shops** - Manage integrations
- **Logout** - Sign out of your account

---

## Monthly Summary & Reports

Get detailed insights into your financial performance with visual breakdowns by category.

### Accessing Monthly Summary

1. Click **"Monthly Summary"** in the dashboard header
2. You'll see a month selector and "Load Summary" button

### Viewing a Summary

1. **Select a Month**: Use the month picker to choose the period you want to analyze
   - Default is the current month
   - Format: `YYYY-MM` (e.g., `2026-01`)
2. Click **"Load Summary"**
3. The system will generate a comprehensive financial report

### Understanding the Summary

#### Key Metrics (Top Row)

1. **Income Card** (🟢 Green)
   - Total positive transactions for the month
   - Displays in your default currency

2. **Expense Card** (🔴 Red)
   - Total negative transactions for the month
   - Shows absolute value

3. **Net Profit Card** (💰 Blue/Red)
   - Income minus expenses
   - Color changes based on profit (blue) or loss (red)

4. **Transactions Card** (🔢 Purple)
   - Total number of transactions recorded

#### Category Breakdown

- **Visual Bars**: Each category shows a horizontal progress bar
  - Green bars = net income categories
  - Red bars = net expense categories
- **Percentage**: Shows relative contribution compared to the largest category
- **Amount**: Displays the net total for each category

#### Category Grid

- At the bottom, you'll see a visual grid comparing all categories
- Each card shows the category name and net amount
- Color-coded for quick identification

---

## Shopify Integration

Connect your Shopify stores to automatically sync transaction data.

### Connecting a New Shop

1. Navigate to **"Shopify Shops"** from the dashboard
2. Find the **"Connect a New Shop"** card
3. Enter your Shopify store domain:
   - Must be in format: `your-store.myshopify.com`
   - Example: `myawesomestore.myshopify.com`
4. Click **"Connect Shop"**
5. You'll be redirected to Shopify to authorize the connection
6. Grant the necessary permissions
7. After authorization, you'll be redirected back to TrueProfit
8. You'll see a success message confirming the connection

### Managing Connected Shops

#### View Connected Shops

- All your connected shops are listed in the "Connected Shops" section
- Each shop displays:
  - Shop domain name
  - OAuth scopes granted
  - Connection date and time

#### Syncing Shop Data

1. Find the shop you want to sync in the list
2. Click the **"Sync"** button
3. The system will start syncing orders and transactions from that shop
4. You'll see a confirmation message when sync starts

#### Disconnecting a Shop

1. Locate the shop you want to remove
2. Click the **"Disconnect"** button (red)
3. Confirm the action
4. The shop will be removed from your integrations
5. Historical data from that shop will remain in your transaction history

### Troubleshooting Shop Connections

- **Invalid domain error**: Make sure your shop URL ends with `.myshopify.com`
- **Connection failed**: Verify you have admin access to the Shopify store
- **Sync issues**: Try disconnecting and reconnecting the shop

---

## Ask Your Data (AI-Powered Queries)

Ask questions about your data in plain English, and let AI generate SQL queries and fetch results for you.

### How It Works

1. **Natural Language Input**: Type your question in everyday language
2. **AI Processing**: The system uses AI to understand your question and generate appropriate SQL
3. **Validation**: SQL is validated for security and correctness
4. **Execution**: Query runs against your data warehouse (Amazon Athena)
5. **Results**: You get formatted results with explanations

### Using the Ask Feature

1. Click **"Ask Your Data"** from the dashboard (purple sparkle button)
2. Type your question in the input field
3. Click **"Ask Question"** or press Enter
4. Wait for the AI to process (typically 5-15 seconds)

### Example Questions

Here are some questions you can ask:

#### Sales & Revenue
- "What are my total sales this month?"
- "Show me revenue by country"
- "What's my best-selling product?"
- "Which products generated the most revenue last quarter?"

#### Customer Analytics
- "Who are my top 10 customers by order value?"
- "How many new customers did I get this month?"
- "What's my average order value?"

#### Product Performance
- "What are my top selling products this week?"
- "Show me products with the highest profit margin"
- "Which products have the most returns?"

#### Time-Based Analysis
- "Compare this month's sales to last month"
- "Show daily revenue for the past 30 days"
- "What day of the week generates the most sales?"

### Understanding Results

#### Result Types

**1. Successful Query (🟢 Green)**
- Shows the generated SQL query
- Displays your data in a formatted table
- Includes query metadata:
  - Amount of data scanned
  - Execution time
  - Confidence level
  - Whether result was cached

**2. Need Clarification (🟡 Yellow)**
- The AI needs more information to answer accurately
- You'll see a clarifying question
- Revise your question with more details and try again

**3. SQL Rejected (🔴 Red)**
- The generated SQL didn't pass security validation
- You'll see the reason for rejection
- Try rephrasing your question in a different way

**4. Query Failed (🟠 Orange)**
- The query executed but encountered an error
- You'll see the error message and the SQL that was attempted
- This might happen if asking about data that doesn't exist

**5. No Shops Connected (⚪ Gray)**
- You need to connect at least one Shopify shop before querying data
- Click the "Connect a Shop" button to get started

### Best Practices

✅ **Do:**
- Be specific about time ranges ("last month", "this week", "in 2026")
- Use proper product/category names
- Ask one question at a time
- Include units when relevant ("top 10", "more than $1000")

❌ **Avoid:**
- Vague questions without context
- Multiple unrelated questions in one query
- Requesting data modifications (this is read-only)

### Query Metadata

Each successful query shows:
- **Confidence**: How confident the AI is about understanding your question (0-100%)
- **Assumptions**: What the AI assumed when interpreting your question
- **Data Scanned**: Amount of data processed (affects cost on AWS)
- **Execution Time**: How long the query took to run
- **Query ID**: Unique identifier for tracking

---

## Troubleshooting

### Authentication Issues

**Problem**: "Unauthorized" or "Session Expired" errors

**Solutions**:
- Click the "Logout" button and log back in
- Clear your browser cookies and cache
- Check that your Cognito credentials are still valid
- Contact your administrator if the issue persists

---

### Transaction Not Showing Up

**Problem**: Added a transaction but it doesn't appear in the list

**Solutions**:
- Click the "Refresh" button in the Transactions section
- Check that you filled in all required fields (Amount, Currency, Category)
- Verify there were no error messages when submitting
- Check your network connection

---

### Monthly Summary Shows No Data

**Problem**: Summary loads but shows zero or no categories

**Solutions**:
- Verify you selected the correct month
- Make sure you have transactions for that period
- Try selecting a different month
- Refresh the page and try again

---

### Shopify Connection Failed

**Problem**: Can't connect Shopify store

**Solutions**:
- Verify your shop URL format: `your-store.myshopify.com`
- Make sure you have admin access to the Shopify store
- Check that you clicked "Authorize" on the Shopify permission page
- Try disconnecting and reconnecting

---

### AI Query Returns Wrong Results

**Problem**: The "Ask Your Data" feature gives unexpected answers

**Solutions**:
- Be more specific in your question
- Include time ranges explicitly ("in January 2026")
- Check the "Assumptions" section to see what the AI interpreted
- Try rephrasing your question
- Look at the generated SQL to understand what was queried

---

### Slow Performance

**Problem**: Pages load slowly or time out

**Solutions**:
- Check your internet connection
- Try refreshing the page
- Clear browser cache
- Close other browser tabs
- Contact support if the issue persists

---

### Browser Compatibility

**Recommended Browsers**:
- ✅ Chrome (latest version)
- ✅ Firefox (latest version)  
- ✅ Safari (latest version)
- ✅ Edge (latest version)

**Not Recommended**:
- ❌ Internet Explorer (not supported)
- ❌ Older browser versions (< 2 years old)

---

## Tips & Best Practices

### Security
- 🔒 Always log out when using shared computers
- 🔒 Don't share your login credentials
- 🔒 Use a strong, unique password
- 🔒 Enable multi-factor authentication if available

### Data Management
- 📊 Add transactions regularly for accurate reporting
- 📊 Use consistent category names
- 📊 Include descriptive notes for better tracking
- 📊 Review monthly summaries to spot trends

### Shopify Integration
- 🛍️ Sync shops regularly to keep data current
- 🛍️ Connect all your shops for complete visibility
- 🛍️ Monitor sync status for any issues

### AI Queries
- 🤖 Start with simple questions, then get more complex
- 🤖 Review the generated SQL to learn patterns
- 🤖 Save your best questions for future reference
- 🤖 Check confidence scores - lower confidence means less certain results

---

## Support & Additional Help

### Getting Help

If you encounter issues not covered in this guide:

1. **Check the error message** - it often contains helpful information
2. **Try the troubleshooting section** above
3. **Contact your system administrator**
4. **Check for updates** - newer versions may fix known issues

### Keyboard Shortcuts

- **Escape**: Close modals or dialogs
- **Enter**: Submit forms (when in input fields)
- **Tab**: Navigate between form fields

---

## Appendix: Technical Details

### System Requirements
- Modern web browser (released within the last 2 years)
- Stable internet connection
- JavaScript enabled
- Cookies enabled

### Data Refresh Rates
- Transactions: Real-time
- Monthly summaries: Calculated on demand
- Shopify sync: On-demand (manual trigger)
- AI queries: Real-time

### Privacy & Data
- All data is encrypted in transit (HTTPS)
- Authentication handled by AWS Cognito
- Transactions are stored securely in AWS
- Data is accessible only to authenticated users

---

## Version Information

**Application**: TrueProfit Frontend (React)  
**Version**: 1.0.0  
**Last Updated**: January 2026

---

*Thank you for using TrueProfit! We hope this guide helps you manage your business finances effectively. Happy tracking! 🚀*
