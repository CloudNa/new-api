package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
)

func setupUserDesktopControllerTestDB(t *testing.T) {
	t.Helper()
	setupDesktopControllerTestDB(t)

	originalRegisterEnabled := common.RegisterEnabled
	originalPasswordRegisterEnabled := common.PasswordRegisterEnabled
	originalEmailVerificationEnabled := common.EmailVerificationEnabled
	originalGenerateDefaultToken := constant.GenerateDefaultToken
	originalDefaultUseAutoGroup := setting.DefaultUseAutoGroup

	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false
	constant.GenerateDefaultToken = false
	setting.DefaultUseAutoGroup = false

	t.Cleanup(func() {
		common.RegisterEnabled = originalRegisterEnabled
		common.PasswordRegisterEnabled = originalPasswordRegisterEnabled
		common.EmailVerificationEnabled = originalEmailVerificationEnabled
		constant.GenerateDefaultToken = originalGenerateDefaultToken
		setting.DefaultUseAutoGroup = originalDefaultUseAutoGroup
	})
}

func TestRegisterCreatesDesktopDefaultToken(t *testing.T) {
	setupUserDesktopControllerTestDB(t)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/register", model.User{
		Username: "desktop-register",
		Password: "password123",
	}, 0)

	Register(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected register success, got message: %s", response.Message)
	}

	var user model.User
	if err := model.DB.Where("username = ?", "desktop-register").First(&user).Error; err != nil {
		t.Fatalf("failed to load registered user: %v", err)
	}

	token, err := model.GetDesktopDefaultToken(user.Id)
	if err != nil {
		t.Fatalf("expected desktop default token: %v", err)
	}
	if token.Name != model.DesktopDefaultTokenName {
		t.Fatalf("expected desktop token name %q, got %q", model.DesktopDefaultTokenName, token.Name)
	}
	if token.Group != "default" {
		t.Fatalf("expected desktop token group default, got %q", token.Group)
	}
	if _, err := model.ValidateUserToken(token.GetFullKey()); err != nil {
		t.Fatalf("expected registered desktop token to validate: %v", err)
	}
}
