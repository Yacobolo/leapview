package main

import (
	"testing"

	"golang.org/x/sys/windows"
)

func TestValidateSecurityDescriptorRejectsUserWritableACL(t *testing.T) {
	tests := []struct {
		name    string
		sddl    string
		wantErr bool
	}{
		{
			name: "administrator controlled and user readable",
			sddl: "O:SYD:P(A;;GA;;;BA)(A;;GA;;;SY)(A;;GR;;;BU)",
		},
		{
			name:    "user writable",
			sddl:    "O:SYD:P(A;;GA;;;BA)(A;;GA;;;SY)(A;;GW;;;BU)",
			wantErr: true,
		},
		{
			name:    "unprotected inheritance",
			sddl:    "O:SYD:(A;;GA;;;BA)(A;;GA;;;SY)(A;;GR;;;BU)",
			wantErr: true,
		},
		{
			name:    "untrusted owner",
			sddl:    "O:BUD:P(A;;GA;;;BA)(A;;GA;;;SY)(A;;GR;;;BU)",
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			descriptor, err := windows.SecurityDescriptorFromString(test.sddl)
			if err != nil {
				t.Fatal(err)
			}
			err = validateSecurityDescriptor(descriptor)
			if test.wantErr && err == nil {
				t.Fatal("expected security descriptor to be rejected")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("expected security descriptor to be accepted: %v", err)
			}
		})
	}
}
