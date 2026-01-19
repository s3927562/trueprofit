package nlq

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type ValidateOptions struct {
	AllowedShopIDs  []string
	RequireDTFilter bool
	MaxDaysLookback int
	TodayISO        string // "YYYY-MM-DD" (server-side). If empty, uses UTC today.
}

// ValidateSQL enforces:
// - SELECT only
// - no semicolon, no comments
// - no dangerous keywords
// - must include dt predicate (partition pruning) AND bounded lookback
// - must include shop_id filter restricted to allowed shops
func ValidateSQL(sql string, opt ValidateOptions) error {
	s := strings.TrimSpace(sql)
	if s == "" {
		return fmt.Errorf("empty sql")
	}
	low := strings.ToLower(s)

	if strings.Contains(low, ";") {
		return fmt.Errorf("semicolon not allowed")
	}
	if strings.Contains(low, "--") || strings.Contains(low, "/*") || strings.Contains(low, "*/") {
		return fmt.Errorf("comments not allowed")
	}
	if !(strings.HasPrefix(strings.TrimSpace(low), "select") || strings.HasPrefix(strings.TrimSpace(low), "with")) {
		return fmt.Errorf("only SELECT queries are allowed")
	}

	block := []string{
		"insert ", "update ", "delete ", "merge ", "drop ", "alter ", "create ",
		"truncate ", "grant ", "revoke ", "call ", "execute ", "prepare ", "deallocate ",
	}
	for _, kw := range block {
		if strings.Contains(low, kw) {
			return fmt.Errorf("disallowed keyword: %s", strings.TrimSpace(kw))
		}
	}

	// dt predicate + bounded lookback
	if opt.RequireDTFilter {
		if opt.MaxDaysLookback <= 0 {
			opt.MaxDaysLookback = 90
		}
		today := opt.TodayISO
		if strings.TrimSpace(today) == "" {
			today = time.Now().UTC().Format("2006-01-02")
		}
		if err := requireBoundedDTPredicate(low, today, opt.MaxDaysLookback); err != nil {
			return err
		}
	}

	// shop_id scoping
	if len(opt.AllowedShopIDs) > 0 {
		if err := requireAllowedShopFilter(low, opt.AllowedShopIDs); err != nil {
			return err
		}
	} else {
		if !regexp.MustCompile(`\bshop_id\b`).MatchString(low) {
			return fmt.Errorf("missing required shop_id filter")
		}
	}

	return nil
}

// requireBoundedDTPredicate enforces dt is filtered and not older than maxDaysLookback.
// Accepts:
//
//	dt >= date 'YYYY-MM-DD'
//	dt >  date 'YYYY-MM-DD'   (treated as >= next day; MVP accept as is)
//	dt between date 'YYYY-MM-DD' and date 'YYYY-MM-DD'
//
// Also accepts ISO without date keyword:
//
//	dt >= 'YYYY-MM-DD'
//
// But rejects missing lower bound (e.g. only dt <= ...).
func requireBoundedDTPredicate(lowSQL, todayISO string, maxDays int) error {
	today, err := time.Parse("2006-01-02", todayISO)
	if err != nil {
		return fmt.Errorf("invalid TodayISO: %s", todayISO)
	}
	minAllowed := today.AddDate(0, 0, -maxDays)

	// BETWEEN pattern
	betweenRe := regexp.MustCompile(`\bdt\b\s+between\s+(date\s+)?'(\d{4}-\d{2}-\d{2})'\s+and\s+(date\s+)?'(\d{4}-\d{2}-\d{2})'`)
	if m := betweenRe.FindStringSubmatch(lowSQL); len(m) == 5 {
		start := m[2]
		// end := m[4]  // not needed for lookback bound, but could validate later
		startDate, err := time.Parse("2006-01-02", start)
		if err != nil {
			return fmt.Errorf("dt BETWEEN has invalid start date: %s", start)
		}
		if startDate.Before(minAllowed) {
			return fmt.Errorf("dt lookback too large: start=%s older than %d days", start, maxDays)
		}
		return nil
	}

	// >= or > lower bound pattern
	geRe := regexp.MustCompile(`\bdt\b\s*(>=|>)\s*(date\s+)?'(\d{4}-\d{2}-\d{2})'`)
	if m := geRe.FindStringSubmatch(lowSQL); len(m) == 4 {
		start := m[3]
		startDate, err := time.Parse("2006-01-02", start)
		if err != nil {
			return fmt.Errorf("dt lower bound invalid: %s", start)
		}
		if startDate.Before(minAllowed) {
			return fmt.Errorf("dt lookback too large: start=%s older than %d days", start, maxDays)
		}
		return nil
	}

	// If dt exists but no >=/between, reject (prevents dt <= only)
	if regexp.MustCompile(`\bdt\b`).MatchString(lowSQL) {
		return fmt.Errorf("dt filter must include a lower bound (dt >= ... or dt BETWEEN ...)")
	}
	return fmt.Errorf("missing required dt filter")
}

func requireAllowedShopFilter(lowSQL string, allowed []string) error {
	// Must mention shop_id somewhere
	if !regexp.MustCompile(`\bshop_id\b`).MatchString(lowSQL) {
		return fmt.Errorf("missing required shop_id filter")
	}

	// Must NOT reference a shop_id literal outside allowlist.
	// MVP approach: if query contains shop_id = 'X' or shop_id in ('X','Y'),
	// ensure all quoted values are subset of allowlist.
	allow := map[string]bool{}
	for _, v := range allowed {
		allow[strings.ToLower(strings.TrimSpace(v))] = true
	}

	// Extract quoted strings near shop_id comparisons (MVP heuristic)
	// Captures values in IN (...) or '=' contexts.
	re := regexp.MustCompile(`\bshop_id\b\s*(=|in)\s*\(([^)]*)\)|\bshop_id\b\s*=\s*'([^']*)'`)
	matches := re.FindAllStringSubmatch(lowSQL, -1)
	if len(matches) == 0 {
		// It has shop_id token but no detectable predicate
		return fmt.Errorf("shop_id filter must be equality or IN list")
	}

	// Parse any IN list values: 'a','b'
	inValRe := regexp.MustCompile(`'([^']*)'`)
	for _, m := range matches {
		// m[2] is inside (...) if IN was used
		if strings.TrimSpace(m[2]) != "" {
			valMatches := inValRe.FindAllStringSubmatch(m[2], -1)
			if len(valMatches) == 0 {
				return fmt.Errorf("shop_id IN list must contain quoted values")
			}
			for _, vm := range valMatches {
				v := strings.ToLower(strings.TrimSpace(vm[1]))
				if !allow[v] {
					return fmt.Errorf("shop_id value not allowed: %s", vm[1])
				}
			}
			return nil
		}
		// m[3] is direct equality value
		if strings.TrimSpace(m[3]) != "" {
			v := strings.ToLower(strings.TrimSpace(m[3]))
			if !allow[v] {
				return fmt.Errorf("shop_id value not allowed: %s", m[3])
			}
			return nil
		}
	}

	return fmt.Errorf("unable to validate shop_id predicate")
}
