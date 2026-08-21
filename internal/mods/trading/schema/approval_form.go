package schema

import (
	"strings"

	"github.com/LyricTian/gin-admin/v10/pkg/errors"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
)

type TradeApprovalQueryParam struct {
	util.PaginationParam
	OwnerUserID string `form:"ownerUserID"`
	TraderID    string `form:"traderID"`
	Status      string `form:"status"`
	Symbol      string `form:"symbol"`
}

type TradeApprovalQueryResult struct {
	Data       []*TradeApproval
	PageResult *util.PaginationResult
}

type TradeApprovalActForm struct {
	ConfirmationPhrase string `json:"confirmation_phrase"`
	Comment            string `json:"comment" binding:"max=500"`
}

func (a *TradeApprovalActForm) ValidateApprove() error {
	a.ConfirmationPhrase = strings.TrimSpace(a.ConfirmationPhrase)
	a.Comment = strings.TrimSpace(a.Comment)
	if a.ConfirmationPhrase != "APPROVE TRADE" {
		return errors.BadRequest("trading_approval_confirmation_required",
			"Approval requires the exact confirmation phrase APPROVE TRADE")
	}
	return nil
}

func (a *TradeApprovalActForm) ValidateReject() error {
	a.Comment = strings.TrimSpace(a.Comment)
	if a.Comment == "" {
		return errors.BadRequest("trading_approval_rejection_reason_required", "A rejection reason is required")
	}
	return nil
}
