package maliciousinstance

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const ManifestVersion = "leapview.desktop.security/v2"

type Trigger string

const (
	TriggerAutomatic   Trigger = "automatic"
	TriggerUserGesture Trigger = "user-gesture"
	TriggerNavigation  Trigger = "navigation"
)

type Outcome string

const (
	OutcomeDenied      Outcome = "denied"
	OutcomeIsolated    Outcome = "isolated"
	OutcomeResponsive  Outcome = "responsive"
	OutcomeExposed     Outcome = "exposed"
	OutcomeUnsupported Outcome = "unsupported"
	OutcomeError       Outcome = "error"
)

type Attack struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Category string  `json:"category"`
	Path     string  `json:"path"`
	Trigger  Trigger `json:"trigger"`
	Expected Outcome `json:"expected"`
}

type Manifest struct {
	Version string   `json:"version"`
	Attacks []Attack `json:"attacks"`
}

var attackIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*)+$`)

func DefaultManifest() Manifest {
	return Manifest{
		Version: ManifestVersion,
		Attacks: []Attack{
			attack("native.renderer-authority", "Electron or Node renderer authority", "native-bridge", TriggerAutomatic, OutcomeIsolated),
			attack("navigation.cross-origin", "Cross-origin main-frame redirect", "navigation", TriggerNavigation),
			attack("navigation.javascript", "Top-level javascript URL", "navigation", TriggerUserGesture),
			attack("navigation.data", "Top-level data URL", "navigation", TriggerUserGesture),
			attack("navigation.blob", "Top-level blob URL", "navigation", TriggerUserGesture),
			attack("navigation.file", "Top-level file URL", "navigation", TriggerUserGesture),
			attack("popup.cross-origin", "Cross-origin popup", "popup", TriggerUserGesture),
			attack("frame.cross-origin", "Cross-origin child frame", "frame", TriggerAutomatic, OutcomeIsolated),
			attack("scheme.custom", "Untrusted custom protocol", "external-launch", TriggerUserGesture),
			attack("scheme.deep-link-injection", "Injected LeapView deep link", "external-launch", TriggerUserGesture),
			attack("permission.camera", "Camera permission request", "permission", TriggerUserGesture),
			attack("permission.microphone", "Microphone permission request", "permission", TriggerUserGesture),
			attack("permission.geolocation", "Geolocation permission request", "permission", TriggerUserGesture),
			attack("permission.notifications", "Notification permission request", "permission", TriggerUserGesture),
			attack("permission.clipboard-read", "Clipboard read request", "permission", TriggerUserGesture),
			attack("download.hostile-filename", "Download with a hostile suggested filename", "download", TriggerUserGesture),
			attack("storage.cross-profile", "Persistent cross-profile marker", "storage", TriggerAutomatic, OutcomeIsolated),
			attack("discovery.malformed", "Malformed instance discovery document", "discovery", TriggerNavigation),
			attack("discovery.oversized", "Oversized instance discovery document", "discovery", TriggerNavigation),
			attack("renderer.resource-exhaustion", "Bounded renderer resource exhaustion", "availability", TriggerUserGesture, OutcomeResponsive),
		},
	}
}

func attack(id, title, category string, trigger Trigger, outcomes ...Outcome) Attack {
	expected := OutcomeDenied
	if len(outcomes) == 1 {
		expected = outcomes[0]
	}
	return Attack{
		ID:       id,
		Title:    title,
		Category: category,
		Path:     "/attack/" + id,
		Trigger:  trigger,
		Expected: expected,
	}
}

func (m Manifest) Validate() error {
	if m.Version != ManifestVersion {
		return fmt.Errorf("manifest version %q does not match %q", m.Version, ManifestVersion)
	}
	if len(m.Attacks) == 0 {
		return fmt.Errorf("manifest must include at least one attack")
	}

	seen := make(map[string]struct{}, len(m.Attacks))
	for index, candidate := range m.Attacks {
		if !attackIDPattern.MatchString(candidate.ID) {
			return fmt.Errorf("attack %d has invalid ID %q", index, candidate.ID)
		}
		if _, duplicate := seen[candidate.ID]; duplicate {
			return fmt.Errorf("attack ID %q is duplicated", candidate.ID)
		}
		seen[candidate.ID] = struct{}{}
		if strings.TrimSpace(candidate.Title) == "" {
			return fmt.Errorf("attack %q is missing a title", candidate.ID)
		}
		if strings.TrimSpace(candidate.Category) == "" {
			return fmt.Errorf("attack %q is missing a category", candidate.ID)
		}
		path, err := url.Parse(candidate.Path)
		if err != nil || !strings.HasPrefix(candidate.Path, "/") || path.IsAbs() || path.Host != "" || path.RawQuery != "" || path.Fragment != "" {
			return fmt.Errorf("attack %q has invalid path %q", candidate.ID, candidate.Path)
		}
		if !validTrigger(candidate.Trigger) {
			return fmt.Errorf("attack %q has invalid trigger %q", candidate.ID, candidate.Trigger)
		}
		if !validOutcome(candidate.Expected) {
			return fmt.Errorf("attack %q has invalid expected outcome %q", candidate.ID, candidate.Expected)
		}
	}
	return nil
}

func validTrigger(trigger Trigger) bool {
	switch trigger {
	case TriggerAutomatic, TriggerUserGesture, TriggerNavigation:
		return true
	default:
		return false
	}
}

func validOutcome(outcome Outcome) bool {
	switch outcome {
	case OutcomeDenied, OutcomeIsolated, OutcomeResponsive, OutcomeExposed, OutcomeUnsupported, OutcomeError:
		return true
	default:
		return false
	}
}
