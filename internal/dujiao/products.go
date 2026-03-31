package dujiao

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func (c *Client) ListProducts(ctx context.Context, token string, params ListProductsParams) (*ProductListResponse, error) {
	values := url.Values{}
	if params.Page > 0 {
		values.Set("page", strconv.Itoa(params.Page))
	}
	if params.PageSize > 0 {
		values.Set("page_size", strconv.Itoa(params.PageSize))
	}
	if params.CategoryID > 0 {
		values.Set("category_id", strconv.FormatUint(uint64(params.CategoryID), 10))
	}
	if search := strings.TrimSpace(params.Search); search != "" {
		values.Set("search", search)
	}
	if fulfillmentType := strings.TrimSpace(params.FulfillmentType); fulfillmentType != "" {
		values.Set("fulfillment_type", fulfillmentType)
	}
	if stockStatus := strings.TrimSpace(params.ManualStockStatus); stockStatus != "" {
		values.Set("manual_stock_status", stockStatus)
	}

	path := "/admin/products"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var items []ProductSummary
	var pagination Pagination
	if err := c.doJSONPage(ctx, "GET", path, token, nil, &items, &pagination); err != nil {
		return nil, err
	}

	return &ProductListResponse{
		Items:      items,
		Pagination: pagination,
	}, nil
}

func (c *Client) GetProduct(ctx context.Context, token string, id uint) (*ProductDetail, error) {
	var out ProductDetail
	if err := c.doJSON(ctx, "GET", fmt.Sprintf("/admin/products/%d", id), token, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreateCardSecretBatch(ctx context.Context, token string, req CreateCardSecretBatchRequest) (*CreateCardSecretBatchResponse, error) {
	var out CreateCardSecretBatchResponse
	if err := c.doJSON(ctx, "POST", "/admin/card-secrets/batch", token, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
