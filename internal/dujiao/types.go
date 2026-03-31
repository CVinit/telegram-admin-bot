package dujiao

type Pagination struct {
	Page      int   `json:"page"`
	PageSize  int   `json:"page_size"`
	Total     int64 `json:"total"`
	TotalPage int64 `json:"total_page"`
}

type LoginResponse struct {
	Token     string         `json:"token"`
	User      LoginAdminUser `json:"user"`
	ExpiresAt string         `json:"expires_at"`
}

type LoginAdminUser struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

type AuthzPolicy struct {
	Subject string `json:"subject"`
	Object  string `json:"object"`
	Action  string `json:"action"`
}

type AuthzMeResponse struct {
	AdminID  uint          `json:"admin_id"`
	IsSuper  bool          `json:"is_super"`
	Roles    []string      `json:"roles"`
	Policies []AuthzPolicy `json:"policies"`
}

type DashboardOverviewResponse struct {
	Range    string               `json:"range"`
	From     string               `json:"from"`
	To       string               `json:"to"`
	Timezone string               `json:"timezone"`
	Currency string               `json:"currency,omitempty"`
	KPI      DashboardKPI         `json:"kpi"`
	Funnel   DashboardFunnel      `json:"funnel"`
	Alerts   []DashboardAlertItem `json:"alerts"`
}

type DashboardKPI struct {
	OrdersTotal          int64  `json:"orders_total"`
	PaidOrders           int64  `json:"paid_orders"`
	CompletedOrders      int64  `json:"completed_orders"`
	PendingPaymentOrders int64  `json:"pending_payment_orders"`
	ProcessingOrders     int64  `json:"processing_orders"`
	GMVPaid              string `json:"gmv_paid"`
	TotalCost            string `json:"total_cost"`
	TotalProfit          string `json:"total_profit"`
	ProfitMargin         string `json:"profit_margin"`
	PaymentsTotal        int64  `json:"payments_total"`
	PaymentsSuccess      int64  `json:"payments_success"`
	PaymentsFailed       int64  `json:"payments_failed"`
	PaymentSuccessRate   string `json:"payment_success_rate"`
	NewUsers             int64  `json:"new_users"`
	ActiveProducts       int64  `json:"active_products"`
	OutOfStockProducts   int64  `json:"out_of_stock_products"`
	LowStockProducts     int64  `json:"low_stock_products"`
	OutOfStockSKUs       int64  `json:"out_of_stock_skus"`
	LowStockSKUs         int64  `json:"low_stock_skus"`
	AutoAvailableSecrets int64  `json:"auto_available_secrets"`
	ManualAvailableUnits int64  `json:"manual_available_units"`
	TotalUserBalance     string `json:"total_user_balance"`
}

type DashboardFunnel struct {
	OrdersCreated         int64  `json:"orders_created"`
	PaymentsCreated       int64  `json:"payments_created"`
	PaymentsSuccess       int64  `json:"payments_success"`
	OrdersPaid            int64  `json:"orders_paid"`
	OrdersCompleted       int64  `json:"orders_completed"`
	PaymentConversionRate string `json:"payment_conversion_rate"`
	CompletionRate        string `json:"completion_rate"`
}

type DashboardAlertItem struct {
	Type  string `json:"type"`
	Level string `json:"level"`
	Value int64  `json:"value"`
}

type ListOrdersParams struct {
	Page           int
	PageSize       int
	Status         string
	UserID         uint
	UserKeyword    string
	OrderNo        string
	GuestEmail     string
	CreatedFrom    string
	CreatedTo      string
	ProductKeyword string
	SortBy         string
	SortOrder      string
}

type OrderListResponse struct {
	Items      []OrderListItem `json:"items"`
	Pagination Pagination      `json:"pagination"`
}

type OrderListItem struct {
	ID              uint   `json:"id"`
	OrderNo         string `json:"order_no"`
	UserID          uint   `json:"user_id"`
	GuestEmail      string `json:"guest_email"`
	Status          string `json:"status"`
	Currency        string `json:"currency"`
	TotalAmount     string `json:"total_amount"`
	UserEmail       string `json:"user_email"`
	UserDisplayName string `json:"user_display_name"`
	CreatedAt       string `json:"created_at"`
}

type OrderDetail struct {
	ID              uint               `json:"id"`
	OrderNo         string             `json:"order_no"`
	UserID          uint               `json:"user_id"`
	GuestEmail      string             `json:"guest_email"`
	Status          string             `json:"status"`
	Currency        string             `json:"currency"`
	TotalAmount     string             `json:"total_amount"`
	UserEmail       string             `json:"user_email"`
	UserDisplayName string             `json:"user_display_name"`
	CouponCode      string             `json:"coupon_code"`
	PromotionName   string             `json:"promotion_name"`
	Items           []OrderItem        `json:"items"`
	Fulfillment     *OrderFulfillment  `json:"fulfillment,omitempty"`
	Children        []OrderChild       `json:"children,omitempty"`
	Payments        []AdminPaymentItem `json:"payments,omitempty"`
}

type OrderItem struct {
	ID              uint                   `json:"id"`
	OrderID         uint                   `json:"order_id"`
	ProductID       uint                   `json:"product_id"`
	SKUID           uint                   `json:"sku_id"`
	Title           map[string]any         `json:"title"`
	SKUSnapshot     map[string]any         `json:"sku_snapshot"`
	Quantity        int                    `json:"quantity"`
	FulfillmentType string                 `json:"fulfillment_type"`
	ManualFormData  map[string]interface{} `json:"manual_form_submission"`
}

type OrderChild struct {
	ID          uint              `json:"id"`
	OrderNo     string            `json:"order_no"`
	Status      string            `json:"status"`
	Items       []OrderItem       `json:"items"`
	Fulfillment *OrderFulfillment `json:"fulfillment,omitempty"`
}

type OrderFulfillment struct {
	ID           uint                   `json:"id"`
	OrderID      uint                   `json:"order_id"`
	Type         string                 `json:"type"`
	Status       string                 `json:"status"`
	Payload      string                 `json:"payload"`
	DeliveryData map[string]interface{} `json:"delivery_data"`
	DeliveredBy  *uint                  `json:"delivered_by,omitempty"`
	DeliveredAt  *string                `json:"delivered_at,omitempty"`
	PayloadLines int                    `json:"payload_line_count"`
}

type AdminPaymentItem struct {
	ID           uint   `json:"id"`
	OrderID      uint   `json:"order_id"`
	ChannelID    uint   `json:"channel_id"`
	ProviderType string `json:"provider_type"`
	ChannelType  string `json:"channel_type"`
	Status       string `json:"status"`
	Amount       string `json:"amount"`
	Currency     string `json:"currency"`
	ChannelName  string `json:"channel_name"`
}

type CreateFulfillmentRequest struct {
	OrderID      uint                   `json:"order_id"`
	Payload      string                 `json:"payload"`
	DeliveryData map[string]interface{} `json:"delivery_data,omitempty"`
}

type FulfillmentResponse struct {
	ID           uint                   `json:"id"`
	OrderID      uint                   `json:"order_id"`
	Type         string                 `json:"type"`
	Status       string                 `json:"status"`
	Payload      string                 `json:"payload"`
	DeliveryData map[string]interface{} `json:"delivery_data"`
	DeliveredBy  *uint                  `json:"delivered_by,omitempty"`
	DeliveredAt  *string                `json:"delivered_at,omitempty"`
}

type ListProductsParams struct {
	Page              int
	PageSize          int
	CategoryID        uint
	Search            string
	FulfillmentType   string
	ManualStockStatus string
}

type ProductListResponse struct {
	Items      []ProductSummary `json:"items"`
	Pagination Pagination       `json:"pagination"`
}

type ProductSummary struct {
	ID                 uint                `json:"id"`
	CategoryID         uint                `json:"category_id"`
	Slug               string              `json:"slug"`
	Title              map[string]any      `json:"title"`
	FulfillmentType    string              `json:"fulfillment_type"`
	ManualStockTotal   int                 `json:"manual_stock_total"`
	AutoStockAvailable int64               `json:"auto_stock_available"`
	IsActive           bool                `json:"is_active"`
	SKUs               []ProductSKUSummary `json:"skus,omitempty"`
}

type ProductSKUSummary struct {
	ID                 uint           `json:"id"`
	ProductID          uint           `json:"product_id"`
	SKUCode            string         `json:"sku_code"`
	SpecValues         map[string]any `json:"spec_values"`
	FulfillmentType    string         `json:"fulfillment_type,omitempty"`
	ManualStockTotal   int            `json:"manual_stock_total"`
	AutoStockAvailable int64          `json:"auto_stock_available"`
	IsActive           bool           `json:"is_active"`
}

type ProductDetail struct {
	ID                 uint                `json:"id"`
	CategoryID         uint                `json:"category_id"`
	Slug               string              `json:"slug"`
	Title              map[string]any      `json:"title"`
	Description        map[string]any      `json:"description"`
	Content            map[string]any      `json:"content"`
	FulfillmentType    string              `json:"fulfillment_type"`
	ManualStockTotal   int                 `json:"manual_stock_total"`
	AutoStockAvailable int64               `json:"auto_stock_available"`
	IsActive           bool                `json:"is_active"`
	SKUs               []ProductSKUSummary `json:"skus,omitempty"`
}

type CreateCardSecretBatchRequest struct {
	ProductID uint     `json:"product_id"`
	SKUID     uint     `json:"sku_id,omitempty"`
	Secrets   []string `json:"secrets"`
	BatchNo   string   `json:"batch_no,omitempty"`
	Note      string   `json:"note,omitempty"`
}

type CreateCardSecretBatchResponse struct {
	Created int    `json:"created"`
	BatchID uint   `json:"batch_id"`
	BatchNo string `json:"batch_no"`
}

type SMTPVerifyCodeSettings struct {
	ExpireMinutes       int `json:"expire_minutes"`
	SendIntervalSeconds int `json:"send_interval_seconds"`
	MaxAttempts         int `json:"max_attempts"`
	Length              int `json:"length"`
}

type SMTPSettings struct {
	Enabled     bool                   `json:"enabled"`
	Host        string                 `json:"host"`
	Port        int                    `json:"port"`
	Username    string                 `json:"username"`
	Password    string                 `json:"password"`
	HasPassword bool                   `json:"has_password"`
	From        string                 `json:"from"`
	FromName    string                 `json:"from_name"`
	UseTLS      bool                   `json:"use_tls"`
	UseSSL      bool                   `json:"use_ssl"`
	VerifyCode  SMTPVerifyCodeSettings `json:"verify_code"`
}

type TelegramBotRuntimeStatus struct {
	Connected        bool     `json:"connected"`
	LastSeenAt       string   `json:"last_seen_at"`
	BotVersion       string   `json:"bot_version"`
	WebhookStatus    string   `json:"webhook_status"`
	MachineCode      string   `json:"machine_code"`
	LicenseStatus    string   `json:"license_status"`
	LicenseExpiresAt string   `json:"license_expires_at"`
	Warnings         []string `json:"warnings"`
	ConfigVersion    int      `json:"config_version"`
	LastConfigSyncAt string   `json:"last_config_sync_at"`
}

type ChannelClient struct {
	ID            uint   `json:"id"`
	Name          string `json:"name"`
	ChannelType   string `json:"channel_type"`
	ChannelKey    string `json:"channel_key"`
	ChannelSecret string `json:"channel_secret"`
	BotToken      string `json:"bot_token"`
	BotTokenSet   bool   `json:"bot_token_set"`
	CallbackURL   string `json:"callback_url"`
	Description   string `json:"description"`
	Status        int    `json:"status"`
}

type AdminUserOAuthIdentity struct {
	ID             uint    `json:"id"`
	Provider       string  `json:"provider"`
	ProviderUserID string  `json:"provider_user_id"`
	Username       string  `json:"username"`
	AvatarURL      string  `json:"avatar_url"`
	AuthAt         *string `json:"auth_at,omitempty"`
	CreatedAt      string  `json:"created_at"`
}

type AdminUserDetail struct {
	ID              uint                     `json:"id"`
	Email           string                   `json:"email"`
	DisplayName     string                   `json:"display_name"`
	Status          string                   `json:"status"`
	Locale          string                   `json:"locale"`
	AdminNote       string                   `json:"admin_note"`
	LastLoginAt     *string                  `json:"last_login_at,omitempty"`
	CreatedAt       string                   `json:"created_at"`
	WalletBalance   string                   `json:"wallet_balance"`
	OAuthIdentities []AdminUserOAuthIdentity `json:"oauth_identities"`
}
