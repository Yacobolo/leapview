package module

import "github.com/flidai/leapview/internal/deployment"

type ApprovalActor = deployment.ApprovalActor
type CredentialClass = deployment.CredentialClass

const (
	CredentialClassHuman    = deployment.CredentialClassHuman
	CredentialClassWorkload = deployment.CredentialClassWorkload
	CredentialClassAPIToken = deployment.CredentialClassAPIToken
	CredentialClassSession  = deployment.CredentialClassSession
)
