package nlq

func ShapeResult(columns []string, rows []map[string]any) map[string]any {
	out := map[string]any{
		"columns": columns,
		"rows":    rows,
	}
	if len(rows) == 1 && len(columns) == 1 {
		// scalar
		out["value"] = rows[0][columns[0]]
		out["kind"] = "scalar"
		return out
	}
	out["kind"] = "table"
	return out
}
