package dujiao

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func (c *Client) ListOrders(ctx context.Context, token string, params ListOrdersParams) (*OrderListResponse, error) {
	values := url.Values{}
	if params.Page > 0 {
		values.Set("page", strconv.Itoa(params.Page))
	}
	if params.PageSize > 0 {
		values.Set("page_size", strconv.Itoa(params.PageSize))
	}
	if status := strings.TrimSpace(params.Status); status != "" {
		values.Set("status", status)
	}
	if params.UserID > 0 {
		values.Set("user_id", strconv.FormatUint(uint64(params.UserID), 10))
	}
	if keyword := strings.TrimSpace(params.UserKeyword); keyword != "" {
		values.Set("user_keyword", keyword)
	}
	if orderNo := strings.TrimSpace(params.OrderNo); orderNo != "" {
		values.Set("order_no", orderNo)
	}
	if email := strings.TrimSpace(params.GuestEmail); email != "" {
		values.Set("guest_email", email)
	}
	if createdFrom := strings.TrimSpace(params.CreatedFrom); createdFrom != "" {
		values.Set("created_from", createdFrom)
	}
	if createdTo := strings.TrimSpace(params.CreatedTo); createdTo != "" {
		values.Set("created_to", createdTo)
	}
	if productKeyword := strings.TrimSpace(params.ProductKeyword); productKeyword != "" {
		values.Set("product_keyword", productKeyword)
	}
	if sortBy := strings.TrimSpace(params.SortBy); sortBy != "" {
		values.Set("sort_by", sortBy)
	}
	if sortOrder := strings.TrimSpace(params.SortOrder); sortOrder != "" {
		values.Set("sort_order", sortOrder)
	}

	path := "/admin/orders"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var items []OrderListItem
	var pagination Pagination
	if err := c.doJSONPage(ctx, "GET", path, token, nil, &items, &pagination); err != nil {
		return nil, err
	}

	return &OrderListResponse{
		Items:      items,
		Pagination: pagination,
	}, nil
}

func (c *Client) GetOrder(ctx context.Context, token string, id uint) (*OrderDetail, error) {
	var out OrderDetail
	if err := c.doJSON(ctx, "GET", fmt.Sprintf("/admin/orders/%d", id), token, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreateFulfillment(ctx context.Context, token string, req CreateFulfillmentRequest) (*FulfillmentResponse, error) {
	var out FulfillmentResponse
	if err := c.doJSON(ctx, "POST", "/admin/fulfillments", token, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetUser(ctx context.Context, token string, id uint) (*AdminUserDetail, error) {
	var out AdminUserDetail
	if err := c.doJSON(ctx, "GET", fmt.Sprintf("/admin/users/%d", id), token, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
