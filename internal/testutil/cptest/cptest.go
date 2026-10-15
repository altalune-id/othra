// Package cptest holds controlplane test helpers that would cycle inside fakes.
package cptest

import (
	"altalune.id/othra/internal/apiclient"
	"altalune.id/othra/internal/controlplane"
	"altalune.id/othra/internal/kernel/authn"
	"altalune.id/othra/internal/permission"
	"altalune.id/othra/internal/testutil/fakes"
)

// OperatorOptions makes a controlplane server accept fakes.OperatorToken as the system operator.
func OperatorOptions() controlplane.Options {
	return controlplane.Options{Authn: authn.InterceptorOptions{
		Chain:       authn.Chain{fakes.OperatorAuthn{}},
		KeyPrefix:   "othra_",
		Permissions: permission.Table(),
	}}
}

// OperatorCredential is the client side of OperatorOptions.
func OperatorCredential() apiclient.Credential {
	return apiclient.Credential{Token: fakes.OperatorToken}
}
