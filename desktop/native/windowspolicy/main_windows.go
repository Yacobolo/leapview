package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	schemaVersion = 1
	policyFile    = "desktop-policy.json"
)

type policyProbe struct {
	SchemaVersion int    `json:"schemaVersion"`
	PolicyPath    string `json:"policyPath"`
	Security      string `json:"security"`
}

func main() {
	if len(os.Args) != 1 {
		exitWithError(errors.New("this helper accepts no arguments"))
	}
	programData, err := windows.KnownFolderPath(
		windows.FOLDERID_ProgramData,
		windows.KF_FLAG_DEFAULT,
	)
	if err != nil {
		exitWithError(fmt.Errorf("resolve ProgramData: %w", err))
	}
	policyDirectory := filepath.Join(programData, "LeapView")
	policyPath := filepath.Join(policyDirectory, policyFile)
	security := "missing"
	if err := secureAdministratorPath(policyDirectory); err != nil {
		security = "insecure"
	} else {
		information, statError := os.Lstat(policyPath)
		switch {
		case errors.Is(statError, os.ErrNotExist):
		case statError != nil:
			security = "insecure"
		case information.Mode().IsRegular():
			if err := secureAdministratorPath(policyPath); err != nil {
				security = "insecure"
			} else {
				security = "secure"
			}
		default:
			security = "insecure"
		}
	}
	if err := json.NewEncoder(os.Stdout).Encode(policyProbe{
		SchemaVersion: schemaVersion,
		PolicyPath:    policyPath,
		Security:      security,
	}); err != nil {
		exitWithError(fmt.Errorf("encode policy probe: %w", err))
	}
}

func secureAdministratorPath(path string) error {
	information, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if information.Mode()&os.ModeSymlink != 0 {
		return errors.New("policy path must not be a symbolic link")
	}
	descriptor, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|
			windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		return fmt.Errorf("read security descriptor: %w", err)
	}
	owner, _, err := descriptor.Owner()
	if err != nil || !trustedAdministrator(owner) {
		return errors.New("policy owner is not Administrators or SYSTEM")
	}
	control, _, err := descriptor.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		return errors.New("policy DACL inherits permissions")
	}
	dacl, _, err := descriptor.DACL()
	if err != nil || dacl == nil {
		return errors.New("policy DACL is absent")
	}
	for index := uint16(0); index < dacl.AceCount; index++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, uint32(index), &ace); err != nil {
			return fmt.Errorf("read policy ACE: %w", err)
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			continue
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if hasWriteAuthority(ace.Mask) && !trustedAdministrator(sid) {
			return errors.New("non-administrator has policy write authority")
		}
	}
	return nil
}

func trustedAdministrator(sid *windows.SID) bool {
	return sid != nil &&
		(sid.IsWellKnown(windows.WinBuiltinAdministratorsSid) ||
			sid.IsWellKnown(windows.WinLocalSystemSid))
}

func hasWriteAuthority(mask windows.ACCESS_MASK) bool {
	const directoryWriteAuthority = windows.ACCESS_MASK(
		windows.GENERIC_ALL |
			windows.GENERIC_WRITE |
			windows.DELETE |
			windows.WRITE_DAC |
			windows.WRITE_OWNER |
			windows.FILE_WRITE_DATA |
			windows.FILE_APPEND_DATA |
			windows.FILE_WRITE_EA |
			windows.FILE_WRITE_ATTRIBUTES |
			0x00000040, // FILE_DELETE_CHILD
	)
	return mask&directoryWriteAuthority != 0
}

func exitWithError(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
