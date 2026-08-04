package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"restaurant-api/internal/services"

	"github.com/gin-gonic/gin"
)

type AccountInviteHandler struct {
	service *services.AccountInviteService
}

func NewAccountInviteHandler(service *services.AccountInviteService) *AccountInviteHandler {
	return &AccountInviteHandler{service: service}
}

func (h *AccountInviteHandler) CreateRequest(c *gin.Context) {
	var req services.CreateAccountRequestInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	invite, err := h.service.CreateRequest(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Account requested. Save your login ID — BillGenie will contact you with a register token.",
		"login_id": invite.LoginID,
		"invite":   invite,
	})
}

func (h *AccountInviteHandler) PreviewRegister(c *gin.Context) {
	var req struct {
		LoginID       string `json:"login_id"`
		RegisterToken string `json:"register_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	preview, err := h.service.PreviewRegister(req.LoginID, req.RegisterToken)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"invite": preview})
}

func (h *AccountInviteHandler) ListPlatformInvites(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	invites, total, err := h.service.List(
		c.Query("status"),
		c.Query("search"),
		limit,
		offset,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"invites": invites,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

func (h *AccountInviteHandler) GetPlatformInvite(c *gin.Context) {
	invite, err := h.service.GetByID(c.Param("invite_id"))
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"invite": invite})
}

func (h *AccountInviteHandler) SetDealAndIssueToken(c *gin.Context) {
	var req services.SetAccountInviteDealInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	actor, _ := c.Get("platform_actor")
	actorStr, _ := actor.(string)
	if actorStr == "" {
		actorStr = "platform"
	}

	invite, token, err := h.service.SetDealAndIssueToken(c.Param("invite_id"), req, actorStr)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "Deal saved. Share login ID + register token with the customer (token shown once).",
		"invite":         invite,
		"register_token": token,
		"login_id":       invite.LoginID,
	})
}
