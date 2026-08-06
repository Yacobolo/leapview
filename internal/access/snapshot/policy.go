// Package snapshot defines the immutable access-policy projection installed
// when a serving state is activated.
package snapshot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	accesspolicy "github.com/flidai/leapview/internal/access/policy"
)

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
	ID             string                `json:"id"`
	Name           string                `json:"name"`
	Object         ObjectRef             `json:"object"`
	Subject        Subject               `json:"subject,omitempty"`
	PolicyType     string                `json:"policyType"`
	ExpressionJSON string                `json:"expressionJson"`
	Compiled       accesspolicy.Compiled `json:"-"`
}

func Decode(data []byte) (AccessPolicy, error) {
	var value AccessPolicy
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return AccessPolicy{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("contains multiple JSON values")
		}
		return AccessPolicy{}, err
	}
	for name, item := range value.DataPolicies {
		compiled, err := accesspolicy.Compile(firstPolicyID(item.ID, name), item.PolicyType, item.ExpressionJSON)
		if err != nil {
			return AccessPolicy{}, fmt.Errorf("data policy %q: %w", name, err)
		}
		item.Compiled = compiled
		value.DataPolicies[name] = item
	}
	return value, nil
}

func firstPolicyID(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return "unknown"
}
