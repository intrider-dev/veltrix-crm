package tenancy

import (
	"testing"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
)

func TestValidateMemberTransitionProtectsLastOwner(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                              string
		actor, currentRole, currentStatus string
		nextRole, nextStatus              string
		owners                            int64
		want                              error
	}{
		{"last owner cannot be demoted", "owner", "owner", "active", "admin", "active", 1, ErrLastOwner},
		{"last owner cannot be disabled", "owner", "owner", "active", "owner", "disabled", 1, ErrLastOwner},
		{"owner can demote when another remains", "owner", "owner", "active", "admin", "active", 2, nil},
		{"admin cannot demote owner", "admin", "owner", "active", "admin", "active", 2, ErrCannotManageOwner},
		{"admin cannot promote owner", "admin", "admin", "active", "owner", "active", 2, ErrCannotManageOwner},
		{"admin can disable sales", "admin", "sales", "active", "sales", "disabled", 1, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ValidateMemberTransition(test.actor, test.currentRole, test.currentStatus, test.nextRole, test.nextStatus, test.owners)
			if got != test.want {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestResolveLocalePrecedenceAndValidation(t *testing.T) {
	t.Parallel()
	supported := []string{"en", "ru"}
	if got := ResolveLocale("ru", "en", "en", supported); got != "ru" {
		t.Fatalf("user preference = %q", got)
	}
	if got := ResolveLocale("unsupported", "ru", "en", supported); got != "ru" {
		t.Fatalf("workspace fallback = %q", got)
	}
	if got := ResolveLocale("", "unsupported", "en", supported); got != "en" {
		t.Fatalf("deployment fallback = %q", got)
	}
	if got := ResolveLocale("", "", "de", supported); got != "" {
		t.Fatalf("unsupported deployment locale accepted: %q", got)
	}
}

func TestInvitationTokenRoundTripAndTamper(t *testing.T) {
	t.Parallel()
	workspaceID := idsForTest(t)
	randomPart, wantHash, err := secureToken()
	if err != nil {
		t.Fatal(err)
	}
	gotWorkspace, gotHash, ok := parseInvitationToken(workspaceID.String() + "." + randomPart)
	if !ok || gotWorkspace != workspaceID || gotHash != wantHash {
		t.Fatal("valid invitation token did not round trip")
	}
	if _, _, ok := parseInvitationToken(workspaceID.String() + ".not-a-token"); ok {
		t.Fatal("malformed invitation token accepted")
	}
}

func idsForTest(t *testing.T) ids.UUID {
	t.Helper()
	return ids.UUID{1, 2, 3, 4, 5, 6, 0x70, 8, 0x80, 10, 11, 12, 13, 14, 15, 16}
}
