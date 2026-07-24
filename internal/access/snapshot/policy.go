// Package snapshot defines the immutable access-policy projection installed
// when a serving state is activated.
package snapshot

import "encoding/json"

type AccessPolicy struct {
	Groups       map[string]Group       `json:"groups,omitempty"`
	RoleBindings map[string]RoleBinding `json:"roleBindings,omitempty"`
	Grants       map[string]Grant       `json:"grants,omitempty"`
	DataPolicies map[string]DataPolicy  `json:"dataPolicies,omitempty"`
}

type Group struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Members     []GroupMember `json:"members,omitempty"`
}

type GroupMember struct {
	PrincipalID string `json:"principalId,omitempty"`
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
}

type RoleBinding struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Role    string  `json:"role"`
	Subject Subject `json:"subject"`
}

type Subject struct {
	Kind        string `json:"kind"`
	PrincipalID string `json:"principalId,omitempty"`
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Group       string `json:"group,omitempty"`
	Publication string `json:"publication,omitempty"`
}

type ObjectRef struct {
	Type string `json:"type"`
	ID   string `json:"id,omitempty"`
}

type Grant struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Object    ObjectRef `json:"object"`
	Subject   Subject   `json:"subject"`
	Privilege string    `json:"privilege"`
}

type DataPolicy struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Object         ObjectRef `json:"object"`
	Subject        Subject   `json:"subject,omitempty"`
	PolicyType     string    `json:"policyType"`
	ExpressionJSON string    `json:"expressionJson"`
}

func Decode(data []byte) (AccessPolicy, error) {
	var value AccessPolicy
	err := json.Unmarshal(data, &value)
	return value, err
}
