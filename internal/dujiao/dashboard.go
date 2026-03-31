package dujiao

import (
	"context"
	"net/url"
	"strconv"
	"strings"
)

type DashboardOverviewQuery struct {
	Range        string
	From         string
	To           string
	Timezone     string
	ForceRefresh *bool
}

func (c *Client) GetDashboardOverview(ctx context.Context, token string, query DashboardOverviewQuery) (*DashboardOverviewResponse, error) {
	values := url.Values{}
	if rangeKey := strings.TrimSpace(query.Range); rangeKey != "" {
		values.Set("range", rangeKey)
	}
	if from := strings.TrimSpace(query.From); from != "" {
		values.Set("from", from)
	}
	if to := strings.TrimSpace(query.To); to != "" {
		values.Set("to", to)
	}
	if timezone := strings.TrimSpace(query.Timezone); timezone != "" {
		values.Set("tz", timezone)
	}
	if query.ForceRefresh != nil {
		values.Set("force_refresh", strconv.FormatBool(*query.ForceRefresh))
	}

	path := "/admin/dashboard/overview"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var out DashboardOverviewResponse
	if err := c.doJSON(ctx, "GET", path, token, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
