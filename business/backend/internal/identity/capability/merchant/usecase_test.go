package merchant

import (
	"context"
	"errors"
	"testing"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant/model"
)

type stubRepository struct{ values []model.Merchant }

func (s stubRepository) ListMerchants(context.Context) ([]model.Merchant, error) {
	return s.values, nil
}
func (stubRepository) ListMerchantPage(context.Context, model.Query) (model.Page, error) {
	return model.Page{}, nil
}
func (stubRepository) GetMerchant(context.Context, int64) (model.Record, error) {
	return model.Record{}, nil
}
func (stubRepository) CreateMerchant(context.Context, model.CreateCommand) (model.CreateResult, bool, error) {
	return model.CreateResult{}, false, nil
}
func (stubRepository) UpdateMerchant(context.Context, model.UpdateCommand) (model.Record, bool, error) {
	return model.Record{}, false, nil
}
func (stubRepository) UpdateProfile(context.Context, model.UpdateProfileCommand) (model.Record, bool, error) {
	return model.Record{}, false, nil
}
func (stubRepository) ResetOwnerPassword(context.Context, model.ResetPasswordCommand) (bool, error) {
	return false, nil
}
func (stubRepository) CloseMerchant(context.Context, model.CloseCommand) (model.Record, bool, error) {
	return model.Record{}, false, nil
}

func TestDirectoryRejectsInvalidRepositoryFacts(t *testing.T) {
	directory := NewDirectory(stubRepository{values: []model.Merchant{{ID: 1, Name: "", Status: model.StatusActive, Version: 1}}})
	if _, err := directory.List(context.Background()); !errors.Is(err, model.ErrInvalidMerchant) {
		t.Fatalf("error=%v", err)
	}
}

func TestCreateRejectsInvalidAccountAndPassword(t *testing.T) {
	directory := NewDirectory(stubRepository{})
	_, _, err := directory.Create(context.Background(), model.CreateCommand{CommandKey: "command-1", Account: "1bad", Password: "short", Name: "Demo"})
	if !errors.Is(err, model.ErrInvalid) {
		t.Fatalf("error=%v", err)
	}
	_, _, err = directory.Create(context.Background(), model.CreateCommand{CommandKey: "command-1", Account: "Lvtu@sufeipay.com", Password: "password1", Name: "Lvtu Open"})
	if err != nil {
		t.Fatalf("email account: %v", err)
	}
	_, _, err = directory.Create(context.Background(), model.CreateCommand{CommandKey: "command-1", Account: "owner", Password: "password1", Name: ""})
	if !errors.Is(err, model.ErrInvalid) {
		t.Fatalf("error=%v", err)
	}
}

func TestUpdateRejectsClosedStatusAndMissingVersion(t *testing.T) {
	directory := NewDirectory(stubRepository{})
	_, _, err := directory.Update(context.Background(), model.UpdateCommand{
		MerchantID: 2001, CommandKey: "command-2", ExpectedVersion: 1, Name: "Demo", Status: model.StatusClosed,
	})
	if !errors.Is(err, model.ErrInvalid) {
		t.Fatalf("error=%v", err)
	}
	_, _, err = directory.Update(context.Background(), model.UpdateCommand{
		MerchantID: 2001, CommandKey: "command-2", Name: "Demo", Status: model.StatusActive,
	})
	if !errors.Is(err, model.ErrInvalid) {
		t.Fatalf("error=%v", err)
	}
}

func TestCloseAndResetRequireStableCommandKey(t *testing.T) {
	directory := NewDirectory(stubRepository{})
	if _, err := directory.ResetPassword(context.Background(), model.ResetPasswordCommand{MerchantID: 2001, CommandKey: "short", Password: "password1"}); !errors.Is(err, model.ErrInvalid) {
		t.Fatalf("reset error=%v", err)
	}
	if _, _, err := directory.Close(context.Background(), model.CloseCommand{MerchantID: 2001, CommandKey: "command-3"}); !errors.Is(err, model.ErrInvalid) {
		t.Fatalf("close error=%v", err)
	}
}

func TestUpdateProfileRejectsMissingVersionAndOversizedFields(t *testing.T) {
	directory := NewDirectory(stubRepository{})
	_, _, err := directory.UpdateProfile(context.Background(), model.UpdateProfileCommand{
		MerchantID: 2001, CommandKey: "command-4", ContactName: "Ada",
	})
	if !errors.Is(err, model.ErrInvalid) {
		t.Fatalf("error=%v", err)
	}
	longID := make([]rune, 65)
	for i := range longID {
		longID[i] = 'a'
	}
	_, _, err = directory.UpdateProfile(context.Background(), model.UpdateProfileCommand{
		MerchantID: 2001, CommandKey: "command-4", ExpectedVersion: 1, ExternalID: string(longID),
	})
	if !errors.Is(err, model.ErrInvalid) {
		t.Fatalf("error=%v", err)
	}
}
