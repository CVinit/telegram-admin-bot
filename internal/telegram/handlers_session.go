package telegram

import (
	"context"
	"strconv"
	"strings"
)

func (r *Router) handleSession(ctx context.Context, update IncomingUpdate) (*Response, error) {
	view, err := r.sessions.RequireSession(ctx, update.TelegramUser)
	if err != nil {
		return r.renderSessionRequired(err), nil
	}

	roles := "none"
	if len(view.Roles) > 0 {
		roles = strings.Join(view.Roles, ", ")
	}

	var builder strings.Builder
	builder.WriteString("当前会话\n")
	builder.WriteString("管理员: ")
	builder.WriteString(strings.TrimSpace(view.Username))
	builder.WriteString("\nAdmin ID: ")
	builder.WriteString(strconv.FormatUint(uint64(view.AdminID), 10))
	builder.WriteString("\nSuper: ")
	if view.IsSuper {
		builder.WriteString("yes")
	} else {
		builder.WriteString("no")
	}
	builder.WriteString("\nRoles: ")
	builder.WriteString(roles)

	return &Response{Text: builder.String()}, nil
}
