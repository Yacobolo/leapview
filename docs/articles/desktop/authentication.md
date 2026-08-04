# Desktop authentication and sessions

LeapView Desktop authenticates through the system browser so the deployed instance and its configured identity provider remain in control. The desktop application never asks for an identity-provider password.

## Sign-in flow

1. LeapView opens a short-lived authorization request in the default browser.
2. The client binds an ephemeral callback on `127.0.0.1` only.
3. The request uses S256 PKCE and binds state, instance ID, profile ID, client ID, and the exact callback URI.
4. After browser authentication, the server returns a single-use authorization code to the loopback callback.
5. Electron redeems the code from the saved profile's isolated session.
6. The server creates a Secure, HttpOnly, SameSite session cookie. JavaScript receives no bearer token.

The callback accepts one bounded request. Provider rejection, cancellation, timeout, a duplicate callback, a port-bind failure, profile removal, closing every window, or application quit cancels the transaction. A retry creates new state, PKCE verifier, callback, and server-side claim.

## Session lifetime

Desktop sessions have a 30-minute idle timeout and an eight-hour absolute lifetime. Authorized activity advances the idle timestamp but never extends the absolute limit. Version one has no silent refresh endpoint.

After expiry, revocation, or sign-out, opening the profile starts a complete new browser authorization. A valid desktop session can reopen without prompting. Browser sessions and other desktop profiles are separate and remain unaffected.

## Sign-out, disconnect, and revocation

- **Sign out** in LeapView revokes the current server session and clears its HttpOnly cookie. The saved profile remains.
- **Disconnect** revokes that desktop session and clears the profile partition.
- **Remove** also deletes the non-secret local mapping.
- **Administrator revocation** invalidates only the chosen server session. The next authorized request or desktop status preflight observes the rejection and clears the matching partition before re-authentication.

LeapView does not automatically replay a cancelled authorization, interrupted command, POST request, or other user action.

## Identity-provider support

The desktop protocol is provider-neutral when a provider completes the server-owned OAuth flow correctly. Production support for a named external provider is established only after its warm-browser, cold-browser, cancellation, failure, and restart matrix has passed on the installed candidates. Until that evidence exists, consult your administrator's supported-provider statement rather than assuming every browser SSO configuration is qualified.

