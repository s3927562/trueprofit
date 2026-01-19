package shopify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type GraphQLError struct {
	Message    string `json:"message"`
	Path       []any  `json:"path,omitempty"`
	Extensions struct {
		Code string `json:"code,omitempty"`
	} `json:"extensions,omitempty"`
}

type GraphQLResponse[T any] struct {
	Data   T              `json:"data"`
	Errors []GraphQLError `json:"errors"`
}

func PostGraphQL[T any](ctx context.Context, shopDomain, apiVersion, accessToken string, query string, variables any) (*GraphQLResponse[T], int, error) {
	endpoint := fmt.Sprintf("https://%s/admin/api/%s/graphql.json", shopDomain, apiVersion)

	body := map[string]any{
		"query":     query,
		"variables": variables,
	}
	b, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(b))
	req.Header.Set("content-type", "application/json")
	req.Header.Set("X-Shopify-Access-Token", accessToken)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)

	var out GraphQLResponse[T]
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, res.StatusCode, err
	}

	return &out, res.StatusCode, nil
}
