package capabilities_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"altalune.id/othra/internal/kernel/capabilities"
)

func TestFrom(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg  capabilities.Config
		want capabilities.Capabilities
	}{
		"selfhosted local": {
			cfg:  capabilities.Config{Selfhosted: true},
			want: capabilities.Capabilities{LocalLogin: true, PersonalKeys: true},
		},
		"selfhosted local with genesis": {
			cfg:  capabilities.Config{Selfhosted: true, EnvGenesis: true},
			want: capabilities.Capabilities{LocalLogin: true, PersonalKeys: true, EnvGenesis: true},
		},
		"selfhosted basic": {
			cfg:  capabilities.Config{Selfhosted: true, BasicAuth: true},
			want: capabilities.Capabilities{BasicAuth: true},
		},
		"selfhosted basic ignores genesis": {
			cfg:  capabilities.Config{Selfhosted: true, BasicAuth: true, EnvGenesis: true},
			want: capabilities.Capabilities{BasicAuth: true},
		},
		"cloud": {
			cfg:  capabilities.Config{Cloud: true},
			want: capabilities.Capabilities{LocalLogin: true, OrgCreation: true, PersonalKeys: true, OperatorOrgSupport: true},
		},
		"neither mode": {
			cfg:  capabilities.Config{},
			want: capabilities.Capabilities{PersonalKeys: true},
		},
		"selfhosted with passwords": {
			cfg:  capabilities.Config{Selfhosted: true, Passwords: true},
			want: capabilities.Capabilities{LocalLogin: true, PersonalKeys: true, Passwords: true},
		},
		"basic has no console passwords": {
			cfg:  capabilities.Config{Selfhosted: true, BasicAuth: true, Passwords: true},
			want: capabilities.Capabilities{BasicAuth: true},
		},
		"file storage": {
			cfg:  capabilities.Config{Selfhosted: true, FileStorage: true},
			want: capabilities.Capabilities{LocalLogin: true, PersonalKeys: true, FileUploads: true},
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, capabilities.From(tc.cfg))
		})
	}
}
