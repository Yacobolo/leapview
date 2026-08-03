// Package maliciousinstance provides the framework-independent hostile server
// used to verify LeapView Desktop's remote-content security boundary.
//
// The package deliberately serves permissive, adversarial web content and must
// never be mounted by the production LeapView application. Desktop framework
// adapters load the handler in an isolated test environment, drive the
// versioned attack manifest, and assert that native capabilities remain
// satisfied. Browser observations are intentionally restricted to bounded enums
// so the harness cannot become a credential or tenant-data collection path.
package maliciousinstance
