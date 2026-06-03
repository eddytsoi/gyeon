package customers

import "testing"

func TestNormalizeRole(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		// installer_v2: canonical, WC underscore form, legacy condensed form,
		// and mixed case all collapse to the distinct installer_v2 tier.
		{"installer_v2", RoleInstallerV2},
		{"Installer_v2", RoleInstallerV2},
		{"installerv2", RoleInstallerV2},
		{"  INSTALLERV2  ", RoleInstallerV2},
		// installer stays installer (no longer absorbs the v2 variants).
		{"installer", RoleInstaller},
		{"Installer", RoleInstaller},
		// Default fallthrough for the base role, empties, and garbage.
		{"customer", RoleCustomer},
		{"", RoleCustomer},
		{"wholesale_customer", RoleCustomer},
		{"admin", RoleCustomer},
	}
	for _, c := range cases {
		if got := NormalizeRole(c.in); got != c.want {
			t.Errorf("NormalizeRole(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
